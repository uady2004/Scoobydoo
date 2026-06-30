package services

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nareix/joy4/av"
	"go.uber.org/zap"

	"github.com/tiktok-clone/livestream-service/internal/config"
	"github.com/tiktok-clone/livestream-service/internal/models"
)

// RTMPFrame carries either a codec header (on first frame) or a media packet.
type RTMPFrame struct {
	Streams []av.CodecData
	Packet  *av.Packet
}

// TranscoderInputCh is the channel type for sending AV frames to FFmpeg.
type TranscoderInputCh chan RTMPFrame

// HLSService manages FFmpeg transcoding and HLS playlist serving.
type HLSService interface {
	// StartTranscoder launches FFmpeg for the given stream and returns:
	//   - A channel to write RTMPFrames into.
	//   - A stop function to gracefully shut down the transcoder.
	//   - An error if startup failed.
	StartTranscoder(ctx context.Context, stream *models.LiveStream) (TranscoderInputCh, func(), error)

	// GetPlaylistURL returns the master M3U8 URL for a stream.
	GetPlaylistURL(streamID string) string

	// GetOutputDir returns the local filesystem path for a stream's HLS files.
	GetOutputDir(streamID string) string

	// CleanupStream removes all HLS files for a stream after it has ended.
	CleanupStream(streamID string) error

	// ListActiveTranscoders returns the IDs of streams currently being transcoded.
	ListActiveTranscoders() []string
}

type hlsService struct {
	cfg        *config.Config
	streamSvc  StreamService
	logger     *zap.Logger
	mu         sync.Mutex
	transcoders map[string]*transcoderJob // keyed by streamID
}

type transcoderJob struct {
	streamID string
	cmds     []*exec.Cmd    // one cmd per rendition
	done     chan struct{}
}

// NewHLSService creates an HLSService backed by local FFmpeg processes.
func NewHLSService(cfg *config.Config, streamSvc StreamService, logger *zap.Logger) HLSService {
	return &hlsService{
		cfg:         cfg,
		streamSvc:   streamSvc,
		logger:      logger,
		transcoders: make(map[string]*transcoderJob),
	}
}

// StartTranscoder starts one FFmpeg process per HLS rendition. The input is
// fed via a named pipe (FIFO) so the RTMP service can push raw AV data without
// needing a second RTMP connection.
//
// For simplicity in this implementation the RTMP input is re-ingested by
// FFmpeg directly from the RTMP server using the internal stream key URL.
// This avoids the complexity of muxing AV packets back into RTMP over a pipe.
func (h *hlsService) StartTranscoder(ctx context.Context, stream *models.LiveStream) (TranscoderInputCh, func(), error) {
	outputDir := h.GetOutputDir(stream.ID)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("creating HLS output dir: %w", err)
	}

	// Input RTMP URL that FFmpeg will read from (loopback to the RTMP server).
	inputURL := fmt.Sprintf("rtmp://127.0.0.1:%d/%s/%s",
		h.cfg.RTMP.Port, h.cfg.RTMP.AppName, stream.RTMPKey,
	)

	job := &transcoderJob{
		streamID: stream.ID,
		done:     make(chan struct{}),
	}

	transCtx, cancelTranscoder := context.WithCancel(ctx)

	var wg sync.WaitGroup
	errCh := make(chan error, len(h.cfg.HLS.Renditions))

	for _, rendition := range h.cfg.HLS.Renditions {
		wg.Add(1)
		rendition := rendition // capture
		go func() {
			defer wg.Done()
			if err := h.runFFmpegForRendition(transCtx, stream.ID, inputURL, outputDir, rendition); err != nil {
				h.logger.Warn("FFmpeg rendition exited",
					zap.String("stream_id", stream.ID),
					zap.String("rendition", rendition.Name),
					zap.Error(err),
				)
				errCh <- err
			}
		}()
	}

	// Generate the master playlist once we know the renditions.
	masterURL, err := h.writeMasterPlaylist(stream.ID, outputDir)
	if err != nil {
		cancelTranscoder()
		return nil, nil, fmt.Errorf("writing master playlist: %w", err)
	}

	// Tell the stream service what the HLS URL is so clients can start watching.
	go func() {
		// Small delay to allow FFmpeg to write the first segment.
		time.Sleep(3 * time.Second)
		if err := h.streamSvc.UpdateHLSPlaylistURL(context.Background(), stream.ID, masterURL); err != nil {
			h.logger.Warn("failed to update HLS playlist URL", zap.Error(err))
		}
	}()

	h.mu.Lock()
	h.transcoders[stream.ID] = job
	h.mu.Unlock()

	// Wait goroutine: close job.done when all renditions finish.
	go func() {
		wg.Wait()
		close(job.done)
		h.mu.Lock()
		delete(h.transcoders, stream.ID)
		h.mu.Unlock()
		h.logger.Info("all transcoder jobs completed", zap.String("stream_id", stream.ID))
	}()

	stopFn := func() {
		cancelTranscoder()
		select {
		case <-job.done:
		case <-time.After(10 * time.Second):
			h.logger.Warn("transcoder stop timeout", zap.String("stream_id", stream.ID))
		}
	}

	// The input channel is consumed by the RTMP service but in this
	// architecture FFmpeg reads directly from the loopback RTMP port,
	// so we return a no-op drain channel.
	inputCh := make(TranscoderInputCh, 256)
	go func() {
		for range inputCh {
			// Packets are not forwarded in this architecture; FFmpeg
			// connects directly to the RTMP ingest port.
		}
	}()

	h.logger.Info("transcoder started",
		zap.String("stream_id", stream.ID),
		zap.String("master_playlist", masterURL),
	)
	return inputCh, stopFn, nil
}

// runFFmpegForRendition builds and executes the FFmpeg command for a single
// HLS rendition. The 2-second segment duration is enforced via -hls_time.
func (h *hlsService) runFFmpegForRendition(
	ctx context.Context,
	streamID, inputURL, outputDir string,
	r config.HLSRendition,
) error {
	segDir := filepath.Join(outputDir, r.Name)
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		return fmt.Errorf("creating segment dir: %w", err)
	}

	playlistPath := filepath.Join(segDir, "index.m3u8")
	segPattern := filepath.Join(segDir, "seg_%05d.ts")

	args := h.buildFFmpegArgs(inputURL, r, playlistPath, segPattern)

	h.logger.Debug("starting FFmpeg",
		zap.String("stream_id", streamID),
		zap.String("rendition", r.Name),
		zap.Strings("args", args),
	)

	cmd := exec.CommandContext(ctx, h.cfg.FFmpeg.BinaryPath, args...)
	cmd.Env = os.Environ()

	// Capture FFmpeg stderr for logging.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting ffmpeg: %w", err)
	}

	// Log FFmpeg output asynchronously.
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			h.logger.Debug("ffmpeg",
				zap.String("stream_id", streamID),
				zap.String("rendition", r.Name),
				zap.String("line", scanner.Text()),
			)
		}
	}()

	return cmd.Wait()
}

// buildFFmpegArgs constructs the argument slice for an HLS transcode job.
// Produces 2-second HLS segments with the configured video/audio codecs.
func (h *hlsService) buildFFmpegArgs(
	inputURL string,
	r config.HLSRendition,
	playlistPath, segPattern string,
) []string {
	cfg := h.cfg.FFmpeg
	hlsCfg := h.cfg.HLS

	args := []string{
		"-loglevel", cfg.LogLevel,
		"-re",   // read input at native frame rate (live mode)
		"-i", inputURL,
	}

	// Hardware acceleration (optional).
	if cfg.HWAccel != "" {
		args = append(args, "-hwaccel", cfg.HWAccel)
	}

	args = append(args,
		// Video
		"-vf", fmt.Sprintf("scale=%d:%d", r.Width, r.Height),
		"-c:v", cfg.VideoCodec,
		"-preset", cfg.Preset,
		"-crf", fmt.Sprintf("%d", cfg.CRF),
		"-b:v", r.Bitrate,
		"-maxrate", r.MaxBitrate,
		"-bufsize", doubleRate(r.MaxBitrate),
		"-g", fmt.Sprintf("%d", hlsCfg.SegmentDuration*r.FramerateOrDefault(30)*2), // GOP = 2 * segment duration
		"-sc_threshold", "0",
		"-keyint_min", fmt.Sprintf("%d", hlsCfg.SegmentDuration*r.FramerateOrDefault(30)),
		"-threads", fmt.Sprintf("%d", cfg.Threads),

		// Audio
		"-c:a", cfg.AudioCodec,
		"-b:a", cfg.AudioBitrate,
		"-ar", "44100",
		"-ac", "2",

		// HLS muxer options
		"-f", "hls",
		"-hls_time", fmt.Sprintf("%d", hlsCfg.SegmentDuration), // 2-second segments
		"-hls_list_size", fmt.Sprintf("%d", hlsCfg.PlaylistLength),
		"-hls_flags", "delete_segments+append_list",
		"-hls_segment_type", "mpegts",
		"-hls_segment_filename", segPattern,
		playlistPath,
	)

	return args
}

// writeMasterPlaylist generates the HLS master (multi-variant) playlist.
// Returns the public URL of the master playlist.
func (h *hlsService) writeMasterPlaylist(streamID, outputDir string) (string, error) {
	masterPath := filepath.Join(outputDir, "master.m3u8")

	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")
	sb.WriteString("#EXT-X-VERSION:3\n\n")

	for _, r := range h.cfg.HLS.Renditions {
		bandwidth := bitrateToInt(r.Bitrate)
		maxBandwidth := bitrateToInt(r.MaxBitrate)
		sb.WriteString(fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,MAX-BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"avc1.42e01e,mp4a.40.2\",NAME=\"%s\"\n",
			bandwidth, maxBandwidth, r.Width, r.Height, r.Name,
		))

		// Use CDN base URL if configured; fall back to relative path.
		if h.cfg.HLS.BaseURL != "" {
			sb.WriteString(fmt.Sprintf("%s/%s/%s/index.m3u8\n", h.cfg.HLS.BaseURL, streamID, r.Name))
		} else {
			sb.WriteString(fmt.Sprintf("%s/index.m3u8\n", r.Name))
		}
	}

	if err := os.WriteFile(masterPath, []byte(sb.String()), 0o644); err != nil {
		return "", err
	}

	if h.cfg.HLS.BaseURL != "" {
		return fmt.Sprintf("%s/%s/master.m3u8", h.cfg.HLS.BaseURL, streamID), nil
	}
	return fmt.Sprintf("/hls/%s/master.m3u8", streamID), nil
}

func (h *hlsService) GetPlaylistURL(streamID string) string {
	if h.cfg.HLS.BaseURL != "" {
		return fmt.Sprintf("%s/%s/master.m3u8", h.cfg.HLS.BaseURL, streamID)
	}
	return fmt.Sprintf("/hls/%s/master.m3u8", streamID)
}

func (h *hlsService) GetOutputDir(streamID string) string {
	return filepath.Join(h.cfg.HLS.OutputPath, streamID)
}

func (h *hlsService) CleanupStream(streamID string) error {
	dir := h.GetOutputDir(streamID)
	h.logger.Info("cleaning up HLS files", zap.String("dir", dir))
	return os.RemoveAll(dir)
}

func (h *hlsService) ListActiveTranscoders() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	ids := make([]string, 0, len(h.transcoders))
	for id := range h.transcoders {
		ids = append(ids, id)
	}
	return ids
}

// ─── arithmetic helpers ───────────────────────────────────────────────────────

// bitrateToInt converts "800k" or "2800k" to bits-per-second integer.
func bitrateToInt(s string) int {
	s = strings.TrimSpace(s)
	multiplier := 1
	if strings.HasSuffix(s, "k") || strings.HasSuffix(s, "K") {
		multiplier = 1000
		s = s[:len(s)-1]
	} else if strings.HasSuffix(s, "m") || strings.HasSuffix(s, "M") {
		multiplier = 1_000_000
		s = s[:len(s)-1]
	}
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n * multiplier
}

// doubleRate doubles a bitrate string like "2800k" -> "5600k".
func doubleRate(s string) string {
	s = strings.TrimSpace(s)
	suffix := ""
	if strings.HasSuffix(s, "k") || strings.HasSuffix(s, "K") {
		suffix = s[len(s)-1:]
		s = s[:len(s)-1]
	} else if strings.HasSuffix(s, "m") || strings.HasSuffix(s, "M") {
		suffix = s[len(s)-1:]
		s = s[:len(s)-1]
	}
	var n int
	fmt.Sscanf(s, "%d", &n)
	return fmt.Sprintf("%d%s", n*2, suffix)
}
