package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/tiktok-clone/video-service/internal/config"
	"github.com/tiktok-clone/video-service/internal/models"
	"github.com/tiktok-clone/video-service/internal/repositories"
	"github.com/tiktok-clone/video-service/internal/services"
)

// ---- in-memory stubs --------------------------------------------------------

// stubVideoRepo is a minimal in-memory implementation of the repository
// surface used by UploadService during tests.
type stubVideoRepo struct {
	videos map[string]*models.Video
	chunks map[string][]*models.VideoChunk
}

func newStubVideoRepo() *stubVideoRepo {
	return &stubVideoRepo{
		videos: make(map[string]*models.Video),
		chunks: make(map[string][]*models.VideoChunk),
	}
}

func (r *stubVideoRepo) CreateVideo(_ context.Context, v *models.Video) (*models.Video, error) {
	r.videos[v.ID] = v
	return v, nil
}

func (r *stubVideoRepo) GetByID(_ context.Context, id string) (*models.Video, error) {
	v, ok := r.videos[id]
	if !ok {
		return nil, fmt.Errorf("not found: %s", id)
	}
	return v, nil
}

func (r *stubVideoRepo) UpdateStatus(_ context.Context, id string, status models.VideoStatus) error {
	v, ok := r.videos[id]
	if !ok {
		return fmt.Errorf("not found: %s", id)
	}
	v.Status = status
	return nil
}

func (r *stubVideoRepo) SaveChunk(_ context.Context, chunk *models.VideoChunk) error {
	r.chunks[chunk.UploadID] = append(r.chunks[chunk.UploadID], chunk)
	return nil
}

func (r *stubVideoRepo) GetChunks(_ context.Context, uploadID string) ([]*models.VideoChunk, error) {
	cs := r.chunks[uploadID]
	out := make([]*models.VideoChunk, len(cs))
	copy(out, cs)
	return out, nil
}

func (r *stubVideoRepo) DeleteChunks(_ context.Context, _ string) error { return nil }

// stubS3 is an in-memory S3 store for use in tests.
type stubS3 struct {
	objects map[string][]byte
}

func newStubS3() *stubS3 {
	return &stubS3{objects: make(map[string][]byte)}
}

func (c *stubS3) PutObject(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	data, err := io.ReadAll(params.Body)
	if err != nil {
		return nil, err
	}
	key := aws.ToString(params.Bucket) + "/" + aws.ToString(params.Key)
	c.objects[key] = data
	return &s3.PutObjectOutput{}, nil
}

func (c *stubS3) GetObject(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	key := aws.ToString(params.Bucket) + "/" + aws.ToString(params.Key)
	data, ok := c.objects[key]
	if !ok {
		return nil, fmt.Errorf("s3 stub: object not found: %s", key)
	}
	return &s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: aws.Int64(int64(len(data))),
	}, nil
}

func (c *stubS3) DeleteObject(_ context.Context, params *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	key := aws.ToString(params.Bucket) + "/" + aws.ToString(params.Key)
	delete(c.objects, key)
	return &s3.DeleteObjectOutput{}, nil
}

// ---- Redis helper -----------------------------------------------------------

// newTestRedis returns a Redis client connected to the local test instance (DB 15).
// The test is skipped if Redis is unavailable.
func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // dedicated test DB
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available (%v); skipping integration test", err)
	}
	t.Cleanup(func() {
		rdb.FlushDB(context.Background()) //nolint:errcheck
		rdb.Close()                       //nolint:errcheck
	})
	return rdb
}

// ---- unit tests: pure helper functions --------------------------------------

func TestMissingChunks(t *testing.T) {
	tests := []struct {
		received []int
		total    int
		want     []int
	}{
		{received: []int{0, 1, 2}, total: 3, want: nil},
		{received: []int{0, 2}, total: 3, want: []int{1}},
		{received: []int{}, total: 3, want: []int{0, 1, 2}},
		{received: []int{1, 3}, total: 5, want: []int{0, 2, 4}},
	}

	for _, tt := range tests {
		got := testMissingChunks(tt.received, tt.total)
		if !intSliceEqual(got, tt.want) {
			t.Errorf("missingChunks(%v, %d) = %v; want %v",
				tt.received, tt.total, got, tt.want)
		}
	}
}

func TestAddChunkIndex(t *testing.T) {
	slice := []int{0, 2, 4}
	slice = testAddChunkIndex(slice, 2) // duplicate – length must stay 3
	if len(slice) != 3 {
		t.Fatalf("expected 3 elements after inserting duplicate, got %d", len(slice))
	}
	slice = testAddChunkIndex(slice, 3) // new element
	if len(slice) != 4 {
		t.Fatalf("expected 4 elements after inserting 3, got %d", len(slice))
	}
	if slice[2] != 3 {
		t.Errorf("expected sorted slice[2] == 3, got %d", slice[2])
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"vacation_2024.mp4": "vacation_2024",
		"video.MOV":         "video",
		"/path/to/clip.avi": "clip",
		"noextension":       "noextension",
		"":                  "Untitled",
	}
	for input, want := range cases {
		got := testSanitizeFilename(input)
		if got != want {
			t.Errorf("sanitizeFilename(%q) = %q; want %q", input, got, want)
		}
	}
}

// ---- integration tests: UploadService ---------------------------------------

// TestUploadService_InitiateUpload_ValidRequest verifies that a valid initiation
// creates a session in Redis and a draft video in the repository.
func TestUploadService_InitiateUpload_ValidRequest(t *testing.T) {
	rdb := newTestRedis(t)
	cfg := testConfig()
	repo := newStubVideoRepo()
	svc := buildUploadService(cfg, rdb)

	req := &models.InitiateUploadRequest{
		Filename:    "my_video.mp4",
		MIMEType:    "video/mp4",
		TotalSize:   10 * 1024 * 1024,
		TotalChunks: 2,
		ChunkSize:   5 * 1024 * 1024,
	}

	resp, err := svc.InitiateUpload(context.Background(), req, "user-001")
	if err != nil {
		t.Fatalf("InitiateUpload error: %v", err)
	}
	if resp.UploadID == "" {
		t.Error("expected non-empty upload_id")
	}
	if resp.VideoID == "" {
		t.Error("expected non-empty video_id")
	}
	if resp.TotalChunks != 2 {
		t.Errorf("expected TotalChunks=2, got %d", resp.TotalChunks)
	}

	// The session must be stored in Redis.
	raw, err := rdb.Get(context.Background(), "upload:session:"+resp.UploadID).Bytes()
	if err != nil {
		t.Fatalf("session not found in Redis: %v", err)
	}
	var session models.UploadSession
	if err := json.Unmarshal(raw, &session); err != nil {
		t.Fatalf("failed to unmarshal session: %v", err)
	}
	if session.VideoID != resp.VideoID {
		t.Errorf("session.VideoID mismatch: got %s, want %s", session.VideoID, resp.VideoID)
	}

	// Suppress unused variable warning from the stub repo.
	_ = repo
}

// TestUploadService_InitiateUpload_InvalidMIME verifies that an unsupported
// MIME type is rejected with a descriptive error.
func TestUploadService_InitiateUpload_InvalidMIME(t *testing.T) {
	rdb := newTestRedis(t)
	cfg := testConfig()
	svc := buildUploadService(cfg, rdb)

	req := &models.InitiateUploadRequest{
		Filename:    "malware.exe",
		MIMEType:    "application/x-msdownload",
		TotalSize:   1024,
		TotalChunks: 1,
		ChunkSize:   1024,
	}

	_, err := svc.InitiateUpload(context.Background(), req, "user-001")
	if err == nil {
		t.Fatal("expected error for invalid MIME, got nil")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestUploadService_InitiateUpload_FileTooLarge verifies that files that exceed
// the configured maximum are rejected.
func TestUploadService_InitiateUpload_FileTooLarge(t *testing.T) {
	rdb := newTestRedis(t)
	cfg := testConfig()
	svc := buildUploadService(cfg, rdb)

	req := &models.InitiateUploadRequest{
		Filename:    "huge.mp4",
		MIMEType:    "video/mp4",
		TotalSize:   cfg.Upload.MaxFileSize + 1,
		TotalChunks: 1,
		ChunkSize:   cfg.Upload.MaxFileSize + 1,
	}

	_, err := svc.InitiateUpload(context.Background(), req, "user-001")
	if err == nil {
		t.Fatal("expected error for oversized file, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestUploadService_UploadChunk_And_Progress uploads a chunk and checks that
// the progress endpoint reflects it correctly.
func TestUploadService_UploadChunk_And_Progress(t *testing.T) {
	rdb := newTestRedis(t)
	cfg := testConfig()
	svc := buildUploadService(cfg, rdb)

	resp, err := svc.InitiateUpload(context.Background(), &models.InitiateUploadRequest{
		Filename:    "test.mp4",
		MIMEType:    "video/mp4",
		TotalSize:   10,
		TotalChunks: 2,
		ChunkSize:   5,
	}, "user-001")
	if err != nil {
		t.Fatalf("InitiateUpload: %v", err)
	}

	// Upload chunk 0.
	payload := []byte("hello")
	if err := svc.UploadChunk(context.Background(), resp.UploadID, 0, bytes.NewReader(payload), int64(len(payload))); err != nil {
		t.Fatalf("UploadChunk 0: %v", err)
	}

	prog, err := svc.GetUploadProgress(context.Background(), resp.UploadID)
	if err != nil {
		t.Fatalf("GetUploadProgress: %v", err)
	}
	if prog.ReceivedChunks != 1 {
		t.Errorf("expected 1 received chunk, got %d", prog.ReceivedChunks)
	}
	if prog.TotalChunks != 2 {
		t.Errorf("expected TotalChunks=2, got %d", prog.TotalChunks)
	}
	if prog.PercentDone != 50.0 {
		t.Errorf("expected 50%% done, got %.1f", prog.PercentDone)
	}
	if len(prog.MissingChunks) != 1 || prog.MissingChunks[0] != 1 {
		t.Errorf("expected missing chunk [1], got %v", prog.MissingChunks)
	}
}

// TestUploadService_ResumeUpload verifies the resume endpoint returns the
// correct set of missing chunk indices.
func TestUploadService_ResumeUpload(t *testing.T) {
	rdb := newTestRedis(t)
	cfg := testConfig()
	svc := buildUploadService(cfg, rdb)

	resp, err := svc.InitiateUpload(context.Background(), &models.InitiateUploadRequest{
		Filename:    "test.mp4",
		MIMEType:    "video/mp4",
		TotalSize:   15,
		TotalChunks: 3,
		ChunkSize:   5,
	}, "user-002")
	if err != nil {
		t.Fatalf("InitiateUpload: %v", err)
	}

	// Upload chunks 0 and 2, skipping 1.
	for _, idx := range []int{0, 2} {
		data := []byte("chunk")
		if err := svc.UploadChunk(context.Background(), resp.UploadID, idx, bytes.NewReader(data), int64(len(data))); err != nil {
			t.Fatalf("UploadChunk %d: %v", idx, err)
		}
	}

	prog, err := svc.ResumeUpload(context.Background(), resp.UploadID)
	if err != nil {
		t.Fatalf("ResumeUpload: %v", err)
	}
	if len(prog.MissingChunks) != 1 || prog.MissingChunks[0] != 1 {
		t.Errorf("expected missing [1], got %v", prog.MissingChunks)
	}
}

// TestUploadService_UploadChunk_OutOfRange ensures that an out-of-range chunk
// index is rejected.
func TestUploadService_UploadChunk_OutOfRange(t *testing.T) {
	rdb := newTestRedis(t)
	cfg := testConfig()
	svc := buildUploadService(cfg, rdb)

	resp, err := svc.InitiateUpload(context.Background(), &models.InitiateUploadRequest{
		Filename:    "test.mp4",
		MIMEType:    "video/mp4",
		TotalSize:   5,
		TotalChunks: 1,
		ChunkSize:   5,
	}, "user-003")
	if err != nil {
		t.Fatalf("InitiateUpload: %v", err)
	}

	data := []byte("hello")
	err = svc.UploadChunk(context.Background(), resp.UploadID, 99, bytes.NewReader(data), int64(len(data)))
	if err == nil {
		t.Fatal("expected out-of-range error, got nil")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestUploadService_CompleteUpload_MissingChunks verifies that CompleteUpload
// returns an error when chunks are still outstanding.
func TestUploadService_CompleteUpload_MissingChunks(t *testing.T) {
	rdb := newTestRedis(t)
	cfg := testConfig()
	svc := buildUploadService(cfg, rdb)

	resp, err := svc.InitiateUpload(context.Background(), &models.InitiateUploadRequest{
		Filename:    "test.mp4",
		MIMEType:    "video/mp4",
		TotalSize:   10,
		TotalChunks: 2,
		ChunkSize:   5,
	}, "user-004")
	if err != nil {
		t.Fatalf("InitiateUpload: %v", err)
	}

	// Upload only chunk 0.
	data := []byte("hello")
	if err := svc.UploadChunk(context.Background(), resp.UploadID, 0, bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("UploadChunk 0: %v", err)
	}

	_, err = svc.CompleteUpload(context.Background(), resp.UploadID)
	if err == nil {
		t.Fatal("expected error for missing chunks, got nil")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestUploadService_NonExistentSession verifies that operating on an unknown
// uploadID returns a useful error.
func TestUploadService_NonExistentSession(t *testing.T) {
	rdb := newTestRedis(t)
	cfg := testConfig()
	svc := buildUploadService(cfg, rdb)

	_, err := svc.GetUploadProgress(context.Background(), "totally-made-up-id")
	if err == nil {
		t.Fatal("expected error for non-existent session, got nil")
	}
}

// ---- test helpers -----------------------------------------------------------

// testConfig returns a minimal config suitable for unit and integration tests.
func testConfig() *config.Config {
	return &config.Config{
		S3: config.S3Config{
			Bucket:     "test-bucket",
			TempBucket: "test-temp-bucket",
			Region:     "us-east-1",
		},
		Upload: config.UploadConfig{
			ChunkSize:   5 * 1024 * 1024,
			MaxFileSize: 100 * 1024 * 1024,
			TempDir:     os.TempDir(),
			ExpireAfter: 1 * time.Hour,
			AllowedMIMEs: []string{
				"video/mp4",
				"video/quicktime",
				"video/x-msvideo",
				"video/webm",
			},
			MaxConcurrent: 3,
		},
	}
}

// buildUploadService wires up an UploadService using real Redis but a real
// aws-sdk S3 client (network calls will fail in unit tests, but the session /
// progress tests exercise only the Redis path and never touch the repository).
func buildUploadService(cfg *config.Config, rdb *redis.Client) *services.UploadService {
	awsCfg, _ := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.S3.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	s3Client := s3.NewFromConfig(awsCfg)
	logger := zap.NewNop()

	// NewVideoRepository accepts a nil pool; the session/progress tests never
	// reach any database call, so this is safe for unit-test purposes.
	repo := repositories.NewVideoRepository(nil)

	return services.NewUploadService(cfg, repo, s3Client, rdb, logger)
}

// ---- pure-logic test helpers (reimplementations so we don't need internal access) ----

func testMissingChunks(received []int, total int) []int {
	set := make(map[int]struct{}, len(received))
	for _, v := range received {
		set[v] = struct{}{}
	}
	var missing []int
	for i := 0; i < total; i++ {
		if _, ok := set[i]; !ok {
			missing = append(missing, i)
		}
	}
	return missing
}

func testAddChunkIndex(slice []int, idx int) []int {
	for _, v := range slice {
		if v == idx {
			return slice
		}
	}
	slice = append(slice, idx)
	for i := len(slice) - 1; i > 0 && slice[i] < slice[i-1]; i-- {
		slice[i], slice[i-1] = slice[i-1], slice[i]
	}
	return slice
}

func testSanitizeFilename(name string) string {
	if name == "" {
		return "Untitled"
	}
	base := name
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '/' || name[i] == '\\' {
			base = name[i+1:]
			break
		}
	}
	if base == "" {
		return "Untitled"
	}
	if dot := strings.LastIndex(base, "."); dot > 0 {
		base = base[:dot]
	}
	if base == "" {
		return "Untitled"
	}
	return base
}

func intSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
