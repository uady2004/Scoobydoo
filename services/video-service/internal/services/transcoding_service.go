package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tiktok-clone/video-service/internal/config"
	"github.com/tiktok-clone/video-service/internal/models"
	"github.com/tiktok-clone/video-service/internal/repositories"
)

// TranscodingService orchestrates FFmpeg-based transcoding operations.
type TranscodingService struct {
	cfg      *config.Config
	repo     *repositories.VideoRepository
	s3Client *s3.Client
	logger   *zap.Logger
	sem      chan struct{} // limits concurrent transcode jobs
}

// NewTranscodingService creates a new TranscodingService.
func NewTranscodingService(
	cfg *config.Config,
	repo *repositories.VideoRepository,
	s3Client *s3.Client,
	logger *zap.Logger,
) *TranscodingService {
	concurrency := cfg.FFmpeg.Concurrency
	if concurrency <= 0 {
		concurrency = 2
	}
	return &TranscodingService{
		cfg:      cfg,
		repo:     repo,
		s3Client: s3Client,
		logger:   logger,
		sem:      make(chan struct{}, concurrency),
	}
}

// ProcessVideo is the main entry point called by the transcoding worker.
// It downloads the raw source file, runs all transcoding steps, and updates
// the database with the results.
func (t *TranscodingService) ProcessVideo(ctx context.Context, videoID, rawS3Key string) error {
	// Acquire semaphore slot.
	t.sem <- struct{}{}
	defer func() { <-t.sem }()

	// Create a dedicated work directory.
	workDir := filepath.Join(t.cfg.FFmpeg.TempDir, videoID)
	if err := os.MkdirAll(workDir, 0o750); err != nil {
		return fmt.Errorf("ProcessVideo mkdir: %w", err)
	}
	defer os.RemoveAll(workDir)

	// 1. Download the source file from S3.
	srcPath := filepath.Join(workDir, "source.mp4")
	if err := t.downloadFromS3(ctx, rawS3Key, srcPath); err != nil {
		return fmt.Errorf("ProcessVideo download: %w", err)
	}

	// 2. Extract video metadata.
	meta, err := t.GenerateMetadata(ctx, videoID, srcPath)
	if err != nil {
		return fmt.Errorf("ProcessVideo metadata: %w", err)
	}
	meta.VideoID = videoID
	if err := t.repo.SaveMetadata(ctx, meta); err != nil {
		return fmt.Errorf("ProcessVideo save metadata: %w", err)
	}

	// 3. Transcode to multiple qualities and generate HLS.
	hlsKey, hlsURL, qualities, err := t.TranscodeToHLS(ctx, videoID, srcPath, workDir)
	if err != nil {
		return fmt.Errorf("ProcessVideo transcode: %w", err)
	}

	// Update metadata with quality info.
	meta.Qualities = qualities
	if err := t.repo.UpdateMetadata(ctx, meta); err != nil {
		t.logger.Warn("failed to update metadata with qualities", zap.Error(err))
	}

	// 4. Extract thumbnail at 1-second mark.
	if err := t.ExtractThumbnail(ctx, videoID, srcPath, 1.0, true); err != nil {
		t.logger.Warn("thumbnail extraction failed", zap.String("video_id", videoID), zap.Error(err))
	}

	// 5. Generate subtitles via Whisper.
	if err := t.GenerateSubtitles(ctx, videoID, srcPath); err != nil {
		t.logger.Warn("subtitle generation failed", zap.String("video_id", videoID), zap.Error(err))
	}

	// 6. Finalise: set HLS key and mark video as ready.
	if err := t.repo.UpdateHLS(ctx, videoID, hlsKey, hlsURL); err != nil {
		return fmt.Errorf("ProcessVideo update HLS: %w", err)
	}

	t.logger.Info("video processing complete",
		zap.String("video_id", videoID),
		zap.String("hls_key", hlsKey),
	)
	return nil
}

// TranscodeToHLS runs FFmpeg for each quality profile and generates an HLS
// master playlist. It returns the S3 key and CDN URL of the master playlist.
func (t *TranscodingService) TranscodeToHLS(
	ctx context.Context,
	videoID, srcPath, workDir string,
) (masterKey, masterURL string, qualities []models.VideoQuality, err error) {

	profiles := t.cfg.Transcode.Profiles
	segLen := t.cfg.Transcode.HLSSegmentLen
	if segLen <= 0 {
		segLen = 6
	}

	type result struct {
		quality models.VideoQuality
		err     error
	}

	results := make([]result, len(profiles))
	var wg sync.WaitGroup

	for i, profile := range profiles {
		wg.Add(1)
		go func(idx int, p config.TranscodeProfile) {
			defer wg.Done()
			q, transcodeErr := t.transcodeProfile(ctx, videoID, srcPath, workDir, p, segLen)
			results[idx] = result{quality: q, err: transcodeErr}
		}(i, profile)
	}
	wg.Wait()

	for _, res := range results {
		if res.err != nil {
			return "", "", nil, fmt.Errorf("transcode profile failed: %w", res.err)
		}
		qualities = append(qualities, res.quality)
	}

	// Build HLS master playlist.
	masterPlaylist := buildMasterPlaylist(qualities)
	masterKey = hlsMasterKey(t.cfg.Transcode.OutputDir, videoID)

	if uploadErr := t.uploadTextToS3(ctx, masterKey, masterPlaylist, "application/vnd.apple.mpegurl"); uploadErr != nil {
		return "", "", nil, fmt.Errorf("upload master playlist: %w", uploadErr)
	}

	if t.cfg.S3.CDNBaseURL != "" {
		masterURL = t.cfg.S3.CDNBaseURL + "/" + masterKey
	} else {
		masterURL = fmt.Sprintf("https://%s.s3.amazonaws.com/%s", t.cfg.S3.Bucket, masterKey)
	}

	return masterKey, masterURL, qualities, nil
}

// transcodeProfile runs FFmpeg for a single quality profile and uploads the
// resulting HLS sub-playlist and segments to S3.
func (t *TranscodingService) transcodeProfile(
	ctx context.Context,
	videoID, srcPath, workDir string,
	profile config.TranscodeProfile,
	segLen int,
) (models.VideoQuality, error) {

	profileDir := filepath.Join(workDir, profile.Name)
	if err := os.MkdirAll(profileDir, 0o750); err != nil {
		return models.VideoQuality{}, err
	}

	playlistPath := filepath.Join(profileDir, "playlist.m3u8")
	segPattern := filepath.Join(profileDir, "seg%05d.ts")

	// Build FFmpeg arguments.
	args := []string{
		"-y",
		"-i", srcPath,
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2",
			profile.Width, profile.Height, profile.Width, profile.Height),
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", strconv.Itoa(profile.CRF),
		"-maxrate", profile.VideoBitrate,
		"-bufsize", doubleBitrate(profile.VideoBitrate),
		"-c:a", "aac",
		"-b:a", profile.AudioBitrate,
		"-hls_time", strconv.Itoa(segLen),
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", segPattern,
		"-f", "hls",
		playlistPath,
	}

	cmd := exec.CommandContext(ctx, t.cfg.FFmpeg.BinaryPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return models.VideoQuality{}, fmt.Errorf("ffmpeg %s: %w — stderr: %s",
			profile.Name, err, stderr.String())
	}

	// Upload playlist and segments to S3.
	s3Prefix := hlsProfileKey(t.cfg.Transcode.OutputDir, videoID, profile.Name)
	playlistKey := s3Prefix + "/playlist.m3u8"

	// Rewrite playlist segment URLs to point at CDN/S3.
	if err := t.rewriteAndUploadPlaylist(ctx, playlistPath, playlistKey, s3Prefix); err != nil {
		return models.VideoQuality{}, fmt.Errorf("upload playlist %s: %w", profile.Name, err)
	}

	// Upload all .ts segment files.
	entries, err := os.ReadDir(profileDir)
	if err != nil {
		return models.VideoQuality{}, err
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".ts" {
			continue
		}
		segPath := filepath.Join(profileDir, e.Name())
		segKey := s3Prefix + "/" + e.Name()
		if err := t.uploadFileToS3(ctx, segKey, segPath, "video/MP2T"); err != nil {
			return models.VideoQuality{}, fmt.Errorf("upload segment %s: %w", e.Name(), err)
		}
	}

	var playlistURL string
	if t.cfg.S3.CDNBaseURL != "" {
		playlistURL = t.cfg.S3.CDNBaseURL + "/" + playlistKey
	} else {
		playlistURL = fmt.Sprintf("https://%s.s3.amazonaws.com/%s", t.cfg.S3.Bucket, playlistKey)
	}

	return models.VideoQuality{
		Name:         profile.Name,
		Width:        profile.Width,
		Height:       profile.Height,
		VideoBitrate: profile.VideoBitrate,
		AudioBitrate: profile.AudioBitrate,
		S3Key:        playlistKey,
		URL:          playlistURL,
	}, nil
}

// ExtractThumbnail grabs a single frame at offsetSecs using FFmpeg and uploads
// it to S3. If isCover is true, it is marked as the primary cover image.
func (t *TranscodingService) ExtractThumbnail(ctx context.Context, videoID, srcPath string, offsetSecs float64, isCover bool) error {
	tmpPath := filepath.Join(t.cfg.FFmpeg.TempDir, fmt.Sprintf("thumb-%s-%.1f.jpg", videoID, offsetSecs))
	defer os.Remove(tmpPath)

	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", offsetSecs),
		"-i", srcPath,
		"-vframes", "1",
		"-q:v", "2",
		"-vf", "scale=720:-2",
		tmpPath,
	}

	cmd := exec.CommandContext(ctx, t.cfg.FFmpeg.BinaryPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ExtractThumbnail ffmpeg: %w — %s", err, stderr.String())
	}

	// Get dimensions from the generated thumbnail.
	width, height, err := t.probeImageDimensions(ctx, tmpPath)
	if err != nil {
		t.logger.Warn("could not probe thumbnail dimensions", zap.Error(err))
		width, height = 720, 1280 // safe defaults
	}

	s3Key := thumbnailS3Key(videoID, offsetSecs)
	if err := t.uploadFileToS3(ctx, s3Key, tmpPath, "image/jpeg"); err != nil {
		return fmt.Errorf("ExtractThumbnail S3 upload: %w", err)
	}

	var thumbURL string
	if t.cfg.S3.CDNBaseURL != "" {
		thumbURL = t.cfg.S3.CDNBaseURL + "/" + s3Key
	} else {
		thumbURL = fmt.Sprintf("https://%s.s3.amazonaws.com/%s", t.cfg.S3.Bucket, s3Key)
	}

	thumb := &models.Thumbnail{
		VideoID:    videoID,
		S3Key:      s3Key,
		URL:        thumbURL,
		Width:      width,
		Height:     height,
		OffsetSecs: offsetSecs,
		IsCover:    isCover,
	}
	return t.repo.SaveThumbnail(ctx, thumb)
}

// GenerateMetadata uses ffprobe to extract technical metadata from the source file.
func (t *TranscodingService) GenerateMetadata(ctx context.Context, videoID, srcPath string) (*models.VideoMetadata, error) {
	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		srcPath,
	}

	cmd := exec.CommandContext(ctx, t.cfg.FFmpeg.ProbePath, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("GenerateMetadata ffprobe: %w", err)
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(out, &probe); err != nil {
		return nil, fmt.Errorf("GenerateMetadata unmarshal: %w", err)
	}

	meta := &models.VideoMetadata{
		ID:      uuid.New().String(),
		VideoID: videoID,
	}

	// Parse format-level info.
	if probe.Format.Duration != "" {
		meta.DurationSecs, _ = strconv.ParseFloat(probe.Format.Duration, 64)
	}
	if probe.Format.Size != "" {
		meta.FileSizeBytes, _ = strconv.ParseInt(probe.Format.Size, 10, 64)
	}

	// Parse stream-level info.
	for _, s := range probe.Streams {
		switch s.CodecType {
		case "video":
			meta.VideoCodec = s.CodecName
			meta.Width = s.Width
			meta.Height = s.Height
			meta.FrameRate = parseFrameRate(s.RFrameRate)
			if meta.Width > 0 && meta.Height > 0 {
				meta.AspectRatio = simplifyRatio(meta.Width, meta.Height)
			}
		case "audio":
			meta.AudioCodec = s.CodecName
		}
	}

	// Determine MIME from the format name.
	meta.MIMEType = formatToMIME(probe.Format.FormatName)

	return meta, nil
}

// GenerateSubtitles extracts audio from the video and calls the Whisper API
// to produce SRT and VTT subtitle files, which are then uploaded to S3.
func (t *TranscodingService) GenerateSubtitles(ctx context.Context, videoID, srcPath string) error {
	if t.cfg.Whisper.APIKey == "" {
		t.logger.Info("Whisper API key not configured; skipping subtitle generation",
			zap.String("video_id", videoID))
		return nil
	}

	// Extract audio track to a temporary WAV/MP3 file.
	audioPath := filepath.Join(t.cfg.FFmpeg.TempDir, fmt.Sprintf("audio-%s.mp3", videoID))
	defer os.Remove(audioPath)

	args := []string{
		"-y",
		"-i", srcPath,
		"-vn",
		"-acodec", "libmp3lame",
		"-ar", "16000",
		"-ac", "1",
		"-q:a", "4",
		audioPath,
	}
	cmd := exec.CommandContext(ctx, t.cfg.FFmpeg.BinaryPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("GenerateSubtitles audio extract: %w — %s", err, stderr.String())
	}

	// Call Whisper API.
	whisperResp, err := t.callWhisperAPI(ctx, audioPath)
	if err != nil {
		return fmt.Errorf("GenerateSubtitles Whisper call: %w", err)
	}

	// Produce SRT and VTT from the Whisper response.
	srtContent := whisperRespToSRT(whisperResp)
	vttContent := whisperRespToVTT(whisperResp)

	lang := t.cfg.Whisper.Language

	for _, pair := range []struct {
		format  models.SubtitleFormat
		content string
	}{
		{models.SubtitleSRT, srtContent},
		{models.SubtitleVTT, vttContent},
	} {
		s3Key := subtitleS3Key(videoID, lang, string(pair.format))
		if err := t.uploadTextToS3(ctx, s3Key, pair.content, subtitleMIME(pair.format)); err != nil {
			t.logger.Warn("subtitle S3 upload failed",
				zap.String("format", string(pair.format)),
				zap.Error(err))
			continue
		}
		var subURL string
		if t.cfg.S3.CDNBaseURL != "" {
			subURL = t.cfg.S3.CDNBaseURL + "/" + s3Key
		} else {
			subURL = fmt.Sprintf("https://%s.s3.amazonaws.com/%s", t.cfg.S3.Bucket, s3Key)
		}
		sub := &models.Subtitle{
			VideoID:       videoID,
			Language:      lang,
			Format:        pair.format,
			S3Key:         s3Key,
			URL:           subURL,
			AutoGenerated: true,
		}
		if err := t.repo.SaveSubtitle(ctx, sub); err != nil {
			t.logger.Warn("save subtitle record failed", zap.Error(err))
		}
	}
	return nil
}

// ---- FFprobe types ----------------------------------------------------------

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecName  string `json:"codec_name"`
	CodecType  string `json:"codec_type"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	RFrameRate string `json:"r_frame_rate"` // e.g. "30000/1001"
}

type ffprobeFormat struct {
	FormatName string `json:"format_name"`
	Duration   string `json:"duration"`
	Size       string `json:"size"`
}

// ---- Whisper API types ------------------------------------------------------

type whisperResponse struct {
	Task     string           `json:"task"`
	Language string           `json:"language"`
	Duration float64          `json:"duration"`
	Segments []whisperSegment `json:"segments"`
	Text     string           `json:"text"`
}

type whisperSegment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

func (t *TranscodingService) callWhisperAPI(ctx context.Context, audioPath string) (*whisperResponse, error) {
	f, err := os.Open(audioPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return nil, err
	}
	_ = mw.WriteField("model", t.cfg.Whisper.Model)
	_ = mw.WriteField("language", t.cfg.Whisper.Language)
	_ = mw.WriteField("response_format", "verbose_json")
	_ = mw.WriteField("timestamp_granularities[]", "segment")
	mw.Close()

	httpCtx, cancel := context.WithTimeout(ctx, t.cfg.Whisper.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, http.MethodPost, t.cfg.Whisper.Endpoint, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+t.cfg.Whisper.APIKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		rawBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Whisper API returned %d: %s", resp.StatusCode, string(rawBody))
	}

	var whisperResp whisperResponse
	if err := json.NewDecoder(resp.Body).Decode(&whisperResp); err != nil {
		return nil, err
	}
	return &whisperResp, nil
}

// ---- subtitle converters ----------------------------------------------------

func whisperRespToSRT(r *whisperResponse) string {
	var sb strings.Builder
	for i, seg := range r.Segments {
		sb.WriteString(strconv.Itoa(i + 1))
		sb.WriteString("\n")
		sb.WriteString(formatSRTTimestamp(seg.Start))
		sb.WriteString(" --> ")
		sb.WriteString(formatSRTTimestamp(seg.End))
		sb.WriteString("\n")
		sb.WriteString(strings.TrimSpace(seg.Text))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func whisperRespToVTT(r *whisperResponse) string {
	var sb strings.Builder
	sb.WriteString("WEBVTT\n\n")
	for _, seg := range r.Segments {
		sb.WriteString(formatVTTTimestamp(seg.Start))
		sb.WriteString(" --> ")
		sb.WriteString(formatVTTTimestamp(seg.End))
		sb.WriteString("\n")
		sb.WriteString(strings.TrimSpace(seg.Text))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func formatSRTTimestamp(secs float64) string {
	return formatTimestamp(secs, ",")
}

func formatVTTTimestamp(secs float64) string {
	return formatTimestamp(secs, ".")
}

func formatTimestamp(secs float64, msSep string) string {
	h := int(secs) / 3600
	m := (int(secs) % 3600) / 60
	s := int(secs) % 60
	ms := int(math.Round((secs-math.Floor(secs))*1000))
	return fmt.Sprintf("%02d:%02d:%02d%s%03d", h, m, s, msSep, ms)
}

// ---- S3 helpers -------------------------------------------------------------

func (t *TranscodingService) downloadFromS3(ctx context.Context, s3Key, destPath string) error {
	out, err := t.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(t.cfg.S3.Bucket),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return fmt.Errorf("S3 GetObject %s: %w", s3Key, err)
	}
	defer out.Body.Close()

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, out.Body)
	return err
}

func (t *TranscodingService) uploadFileToS3(ctx context.Context, s3Key, filePath, contentType string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	_, err = t.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(t.cfg.S3.Bucket),
		Key:           aws.String(s3Key),
		Body:          f,
		ContentLength: aws.Int64(stat.Size()),
		ContentType:   aws.String(contentType),
	})
	return err
}

func (t *TranscodingService) uploadTextToS3(ctx context.Context, s3Key, content, contentType string) error {
	data := []byte(content)
	_, err := t.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(t.cfg.S3.Bucket),
		Key:           aws.String(s3Key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   aws.String(contentType),
	})
	return err
}

func (t *TranscodingService) rewriteAndUploadPlaylist(ctx context.Context, localPath, s3Key, s3Prefix string) error {
	raw, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	// Replace bare segment filenames with full CDN/S3 URLs.
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, ".ts") && !strings.HasPrefix(trimmed, "http") {
			var segURL string
			segKey := s3Prefix + "/" + trimmed
			if t.cfg.S3.CDNBaseURL != "" {
				segURL = t.cfg.S3.CDNBaseURL + "/" + segKey
			} else {
				segURL = fmt.Sprintf("https://%s.s3.amazonaws.com/%s", t.cfg.S3.Bucket, segKey)
			}
			lines[i] = segURL
		}
	}
	rewritten := strings.Join(lines, "\n")
	return t.uploadTextToS3(ctx, s3Key, rewritten, "application/vnd.apple.mpegurl")
}

func (t *TranscodingService) probeImageDimensions(ctx context.Context, imgPath string) (width, height int, err error) {
	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		imgPath,
	}
	cmd := exec.CommandContext(ctx, t.cfg.FFmpeg.ProbePath, args...)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	var probe ffprobeOutput
	if err := json.Unmarshal(out, &probe); err != nil {
		return 0, 0, err
	}
	for _, s := range probe.Streams {
		if s.CodecType == "video" {
			return s.Width, s.Height, nil
		}
	}
	return 0, 0, fmt.Errorf("no video stream found in %s", imgPath)
}

// ---- HLS master playlist builder --------------------------------------------

func buildMasterPlaylist(qualities []models.VideoQuality) string {
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n\n")
	for _, q := range qualities {
		bandwidth := bitrateToKbps(q.VideoBitrate) * 1000
		sb.WriteString(fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,NAME=\"%s\"\n%s\n",
			bandwidth, q.Width, q.Height, q.Name, q.URL,
		))
	}
	return sb.String()
}

// ---- S3 key builders --------------------------------------------------------

func hlsMasterKey(outputDir, videoID string) string {
	return filepath.Join(outputDir, videoID, "master.m3u8")
}

func hlsProfileKey(outputDir, videoID, profile string) string {
	return filepath.Join(outputDir, videoID, profile)
}

func thumbnailS3Key(videoID string, offsetSecs float64) string {
	return fmt.Sprintf("thumbnails/%s/thumb-%.1f.jpg", videoID, offsetSecs)
}

func subtitleS3Key(videoID, lang, format string) string {
	return fmt.Sprintf("subtitles/%s/%s.%s", videoID, lang, format)
}

func subtitleMIME(f models.SubtitleFormat) string {
	switch f {
	case models.SubtitleVTT:
		return "text/vtt"
	case models.SubtitleJSON:
		return "application/json"
	default:
		return "application/x-subrip"
	}
}

// ---- misc helpers -----------------------------------------------------------

func parseFrameRate(r string) float64 {
	parts := strings.SplitN(r, "/", 2)
	if len(parts) != 2 {
		v, _ := strconv.ParseFloat(r, 64)
		return v
	}
	num, _ := strconv.ParseFloat(parts[0], 64)
	den, _ := strconv.ParseFloat(parts[1], 64)
	if den == 0 {
		return 0
	}
	return num / den
}

func simplifyRatio(w, h int) string {
	g := gcd(w, h)
	return fmt.Sprintf("%d:%d", w/g, h/g)
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func formatToMIME(format string) string {
	switch {
	case strings.Contains(format, "mp4"):
		return "video/mp4"
	case strings.Contains(format, "mov"):
		return "video/quicktime"
	case strings.Contains(format, "webm"):
		return "video/webm"
	case strings.Contains(format, "avi"):
		return "video/x-msvideo"
	default:
		return "video/mp4"
	}
}

// doubleBitrate returns a bitrate string doubled (for bufsize).
func doubleBitrate(bitrate string) string {
	kbps := bitrateToKbps(bitrate)
	return fmt.Sprintf("%dk", kbps*2)
}

func bitrateToKbps(bitrate string) int {
	bitrate = strings.ToLower(bitrate)
	bitrate = strings.TrimSuffix(bitrate, "k")
	v, _ := strconv.Atoi(bitrate)
	return v
}

// Ensure time import is used.
var _ = time.Second
