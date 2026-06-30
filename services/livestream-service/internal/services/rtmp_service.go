package services

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/nareix/joy4/av/pktque"
	"github.com/nareix/joy4/format"
	"github.com/nareix/joy4/format/rtmp"
	"go.uber.org/zap"

	"github.com/tiktok-clone/livestream-service/internal/config"
)

func init() {
	// Register all muxers/demuxers provided by joy4.
	format.RegisterAll()
}

// RTMPSession represents an active ingest connection from a broadcaster.
type RTMPSession struct {
	StreamID  string
	RTMPKey   string
	UserID    string
	StartedAt time.Time
	conn      *rtmp.Conn
	cancel    context.CancelFunc
}

// RTMPService handles RTMP ingest: it accepts connections, validates stream
// keys, and forwards the decoded AV stream to the HLS transcoder.
type RTMPService interface {
	// Start begins listening for RTMP connections. Blocks until ctx is cancelled.
	Start(ctx context.Context) error
	// ActiveSessionCount returns the number of live ingest sessions.
	ActiveSessionCount() int
	// TerminateSession forcibly ends an ingest session by stream ID.
	TerminateSession(streamID string) error
}

type rtmpService struct {
	cfg           *config.Config
	streamSvc     StreamService
	hlsSvc        HLSService
	logger        *zap.Logger
	mu            sync.RWMutex
	sessions      map[string]*RTMPSession // keyed by streamID
	sessionsByKey map[string]*RTMPSession // keyed by rtmpKey
}

// NewRTMPService creates a new RTMP ingest service.
func NewRTMPService(
	cfg *config.Config,
	streamSvc StreamService,
	hlsSvc HLSService,
	logger *zap.Logger,
) RTMPService {
	return &rtmpService{
		cfg:           cfg,
		streamSvc:     streamSvc,
		hlsSvc:        hlsSvc,
		logger:        logger,
		sessions:      make(map[string]*RTMPSession),
		sessionsByKey: make(map[string]*RTMPSession),
	}
}

// Start opens the TCP listener on the configured RTMP port and handles
// incoming connections in separate goroutines.
func (s *rtmpService) Start(ctx context.Context) error {
	server := &rtmp.Server{
		Addr: s.cfg.RTMP.Addr(),
	}

	// HandlePublish is called by the joy4 RTMP server for each incoming
	// publish connection.  We validate the stream key, register the session,
	// and pipe the AV stream to the HLS transcoder.
	server.HandlePublish = func(conn *rtmp.Conn) {
		s.handlePublish(ctx, conn)
	}

	// HandlePlay handles RTMP playback requests (re-stream use cases).
	server.HandlePlay = func(conn *rtmp.Conn) {
		s.handlePlay(ctx, conn)
	}

	s.logger.Info("RTMP server listening", zap.String("addr", s.cfg.RTMP.Addr()))

	// ListenAndServe blocks; run it in a goroutine so we can honour ctx.Done.
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("RTMP server shutting down")
		s.terminateAllSessions()
		return ctx.Err()
	case err := <-errCh:
		return fmt.Errorf("RTMP server error: %w", err)
	}
}

// handlePublish is the core ingest handler. It:
//  1. Extracts the stream key from the RTMP URL path.
//  2. Validates the key against the stream service.
//  3. Starts the HLS transcoder for the stream.
//  4. Reads AV packets from the RTMP connection and forwards them to FFmpeg.
//  5. Cleans up on disconnect.
func (s *rtmpService) handlePublish(ctx context.Context, conn *rtmp.Conn) {
	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Apply read deadline to detect stalled publishers.
	if tc, ok := conn.NetConn().(interface{ SetReadDeadline(time.Time) error }); ok {
		tc.SetReadDeadline(time.Now().Add(s.cfg.RTMP.ReadTimeout)) //nolint:errcheck
	}

	// The joy4 RTMP URL looks like rtmp://host/app/streamKey
	// conn.URL.Path is "/<app>/<streamKey>"
	rtmpKey := extractStreamKey(conn.URL.Path)
	if rtmpKey == "" {
		s.logger.Warn("empty stream key, closing connection",
			zap.String("remote", remoteAddr(conn)),
		)
		conn.Close()
		return
	}

	// Validate the key.
	stream, err := s.streamSvc.ValidateRTMPKey(connCtx, rtmpKey)
	if err != nil {
		s.logger.Warn("invalid stream key",
			zap.String("key", rtmpKey),
			zap.Error(err),
		)
		conn.Close()
		return
	}

	// Reject duplicate publishers for the same stream.
	if s.isStreamActive(stream.ID) {
		s.logger.Warn("duplicate publisher for stream",
			zap.String("stream_id", stream.ID),
		)
		conn.Close()
		return
	}

	// Register the session.
	sess := &RTMPSession{
		StreamID:  stream.ID,
		RTMPKey:   rtmpKey,
		UserID:    stream.UserID,
		StartedAt: time.Now().UTC(),
		conn:      conn,
		cancel:    cancel,
	}
	s.registerSession(sess)
	defer s.deregisterSession(sess)

	s.logger.Info("publisher connected",
		zap.String("stream_id", stream.ID),
		zap.String("user_id", stream.UserID),
		zap.String("remote", remoteAddr(conn)),
	)

	// Start the HLS transcoder, which returns a chan to receive packets.
	transcoderCh, stopTranscoder, err := s.hlsSvc.StartTranscoder(connCtx, stream)
	if err != nil {
		s.logger.Error("failed to start transcoder",
			zap.String("stream_id", stream.ID),
			zap.Error(err),
		)
		return
	}
	defer stopTranscoder()

	// Prepare the demuxer over the RTMP connection.
	demuxer := conn

	// Read the header (codec info) and send it to the transcoder.
	streams, err := demuxer.Streams()
	if err != nil {
		s.logger.Error("failed to read streams from RTMP",
			zap.String("stream_id", stream.ID),
			zap.Error(err),
		)
		return
	}

	// Forward stream headers to transcoder.
	select {
	case transcoderCh <- RTMPFrame{Streams: streams}:
	case <-connCtx.Done():
		return
	}

	// Packet read loop.
	filter := pktque.Filters{
		&pktque.FixTime{StartFromZero: true, MakeIncrement: true},
	}
	filteredDemux := &pktque.FilterDemuxer{Demuxer: demuxer, Filter: filter}

	for {
		// Refresh the read deadline on each packet so stalled connections
		// are caught.
		if tc, ok := conn.NetConn().(interface{ SetReadDeadline(time.Time) error }); ok {
			tc.SetReadDeadline(time.Now().Add(s.cfg.RTMP.ReadTimeout)) //nolint:errcheck
		}

		pkt, err := filteredDemux.ReadPacket()
		if err != nil {
			s.logger.Info("RTMP publisher disconnected",
				zap.String("stream_id", stream.ID),
				zap.Error(err),
			)
			break
		}

		select {
		case transcoderCh <- RTMPFrame{Packet: &pkt}:
		case <-connCtx.Done():
			return
		default:
			// Drop packet if transcoder is overwhelmed; prevents backpressure.
			s.logger.Debug("transcoder channel full, dropping packet",
				zap.String("stream_id", stream.ID),
			)
		}
	}

	// Notify the stream service that the stream has ended.
	if err := s.streamSvc.EndStream(context.Background(), stream.ID, stream.UserID); err != nil {
		s.logger.Warn("EndStream on RTMP disconnect failed", zap.Error(err))
	}
}

// handlePlay allows RTMP playback (re-stream or preview). In production this
// would be secured; here we log and close for unsupported clients.
func (s *rtmpService) handlePlay(ctx context.Context, conn *rtmp.Conn) {
	s.logger.Info("RTMP play request (not supported – use HLS)",
		zap.String("remote", remoteAddr(conn)),
		zap.String("path", conn.URL.Path),
	)
	conn.Close()
}

func (s *rtmpService) ActiveSessionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

func (s *rtmpService) TerminateSession(streamID string) error {
	s.mu.Lock()
	sess, ok := s.sessions[streamID]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("no active session for stream %s", streamID)
	}
	sess.cancel()
	sess.conn.Close()
	return nil
}

// ─── session management ───────────────────────────────────────────────────────

func (s *rtmpService) registerSession(sess *RTMPSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.StreamID] = sess
	s.sessionsByKey[sess.RTMPKey] = sess
}

func (s *rtmpService) deregisterSession(sess *RTMPSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sess.StreamID)
	delete(s.sessionsByKey, sess.RTMPKey)
}

func (s *rtmpService) isStreamActive(streamID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.sessions[streamID]
	return ok
}

func (s *rtmpService) terminateAllSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sess := range s.sessions {
		sess.cancel()
		sess.conn.Close()
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// extractStreamKey parses the last segment from a RTMP path like "/live/myKey".
func extractStreamKey(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			key := path[i+1:]
			if key != "" {
				return key
			}
		}
	}
	return ""
}

func remoteAddr(conn *rtmp.Conn) string {
	if nc := conn.NetConn(); nc != nil {
		if addr, ok := nc.RemoteAddr().(*net.TCPAddr); ok {
			return addr.String()
		}
	}
	return "unknown"
}
