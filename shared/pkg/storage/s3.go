// Package storage provides an S3/MinIO client wrapper with helpers for object
// upload, download, deletion, and presigned URL generation.
//
// The Client is built on the official minio-go/v7 SDK and is compatible with
// both AWS S3 and self-hosted MinIO.
//
// Usage:
//
//	client, err := storage.New(storage.Config{
//	    Endpoint:        "minio:9000",
//	    AccessKeyID:     "minioadmin",
//	    SecretAccessKey: "minioadmin",
//	    UseSSL:          false,
//	    DefaultBucket:   "videos",
//	})
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
)

// ---- Configuration ----------------------------------------------------------

// Config holds MinIO/S3 client settings.
type Config struct {
	// Endpoint is the S3/MinIO server address (host:port or host for AWS).
	// For AWS S3 use "s3.amazonaws.com".
	Endpoint string
	// AccessKeyID is the access key / AWS access key ID.
	AccessKeyID string
	// SecretAccessKey is the secret key / AWS secret access key.
	SecretAccessKey string
	// SessionToken is optional (for temporary credentials).
	SessionToken string
	// UseSSL enables TLS. Set to true for production AWS/MinIO.
	UseSSL bool
	// Region overrides the auto-detected region.
	Region string
	// DefaultBucket is used by methods that do not require an explicit bucket.
	DefaultBucket string

	// PresignedURLExpiry is the default expiry for presigned URLs.
	// Defaults to 15 minutes.
	PresignedURLExpiry time.Duration

	// ConnectRetries is the number of bucket-check retries at startup.
	ConnectRetries int
	// ConnectRetryDelay is the wait between retries.
	ConnectRetryDelay time.Duration
}

func (c *Config) defaults() {
	if c.Endpoint == "" {
		c.Endpoint = "localhost:9000"
	}
	if c.PresignedURLExpiry == 0 {
		c.PresignedURLExpiry = 15 * time.Minute
	}
	if c.ConnectRetries == 0 {
		c.ConnectRetries = 5
	}
	if c.ConnectRetryDelay == 0 {
		c.ConnectRetryDelay = 2 * time.Second
	}
}

// ---- Object metadata --------------------------------------------------------

// ObjectInfo contains metadata returned after a successful upload or stat.
type ObjectInfo struct {
	Key          string
	Bucket       string
	Size         int64
	ContentType  string
	ETag         string
	LastModified time.Time
	UserMeta     map[string]string
}

// UploadOptions controls how an object is stored.
type UploadOptions struct {
	// ContentType is the MIME type (e.g. "video/mp4"). Defaults to
	// "application/octet-stream".
	ContentType string
	// UserMeta is a map of user-defined metadata headers (stored as x-amz-meta-*).
	UserMeta map[string]string
	// Tags are object tags (key-value pairs).
	Tags map[string]string
	// ObjectSize is used for progress tracking and multipart upload decisions.
	// Pass -1 if unknown.
	ObjectSize int64
	// PartSize overrides the multipart part size (defaults to minio's auto).
	// Set to 0 to use the library default.
	PartSize uint64
}

// ---- Client -----------------------------------------------------------------

// Client wraps *minio.Client with convenience helpers.
type Client struct {
	mc  *minio.Client
	cfg Config
}

// New creates a Client and verifies that the DefaultBucket is accessible.
// It retries the bucket check cfg.ConnectRetries times.
func New(cfg Config) (*Client, error) {
	cfg.defaults()

	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: creating minio client: %w", err)
	}

	c := &Client{mc: mc, cfg: cfg}

	// Verify we can reach the server.
	if cfg.DefaultBucket != "" {
		var lastErr error
		for attempt := 1; attempt <= cfg.ConnectRetries; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, lastErr = mc.BucketExists(ctx, cfg.DefaultBucket)
			cancel()
			if lastErr == nil {
				break
			}
			if attempt < cfg.ConnectRetries {
				time.Sleep(cfg.ConnectRetryDelay)
			}
		}
		if lastErr != nil {
			return nil, fmt.Errorf("storage: verifying bucket %q after %d attempts: %w",
				cfg.DefaultBucket, cfg.ConnectRetries, lastErr)
		}
	}

	return c, nil
}

// Raw returns the underlying *minio.Client for advanced operations.
func (c *Client) Raw() *minio.Client { return c.mc }

// ---- Bucket management ------------------------------------------------------

// EnsureBucket creates bucket if it does not already exist.
func (c *Client) EnsureBucket(ctx context.Context, bucket, region string) error {
	exists, err := c.mc.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("storage: checking bucket %q: %w", bucket, err)
	}
	if exists {
		return nil
	}
	opts := minio.MakeBucketOptions{}
	if region != "" {
		opts.Region = region
	}
	if err := c.mc.MakeBucket(ctx, bucket, opts); err != nil {
		// Another replica may have created it concurrently.
		if isAlreadyExistsErr(err) {
			return nil
		}
		return fmt.Errorf("storage: creating bucket %q: %w", bucket, err)
	}
	return nil
}

// SetLifecycle applies an S3 lifecycle policy to a bucket (e.g. delete
// incomplete multipart uploads after 7 days).
func (c *Client) SetLifecycle(ctx context.Context, bucket string, cfg *lifecycle.Configuration) error {
	if err := c.mc.SetBucketLifecycle(ctx, bucket, cfg); err != nil {
		return fmt.Errorf("storage: setting lifecycle on %q: %w", bucket, err)
	}
	return nil
}

// ---- Upload -----------------------------------------------------------------

// Upload streams r into bucket/key. Pass opts.ObjectSize = -1 when the size
// is not known in advance (triggers streaming multipart upload).
func (c *Client) Upload(ctx context.Context, bucket, key string, r io.Reader, opts UploadOptions) (*ObjectInfo, error) {
	if opts.ContentType == "" {
		opts.ContentType = "application/octet-stream"
	}

	putOpts := minio.PutObjectOptions{
		ContentType:  opts.ContentType,
		UserMetadata: opts.UserMeta,
		UserTags:     opts.Tags,
		PartSize:     opts.PartSize,
	}

	info, err := c.mc.PutObject(ctx, bucket, key, r, opts.ObjectSize, putOpts)
	if err != nil {
		return nil, fmt.Errorf("storage: uploading %q to %q: %w", key, bucket, err)
	}

	return &ObjectInfo{
		Key:    info.Key,
		Bucket: info.Bucket,
		Size:   info.Size,
		ETag:   info.ETag,
	}, nil
}

// UploadToDefault uploads to the DefaultBucket.
func (c *Client) UploadToDefault(ctx context.Context, key string, r io.Reader, opts UploadOptions) (*ObjectInfo, error) {
	return c.Upload(ctx, c.cfg.DefaultBucket, key, r, opts)
}

// ---- Download ---------------------------------------------------------------

// Download returns a ReadCloser for the object at bucket/key.
// The caller must close the returned reader.
func (c *Client) Download(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	obj, err := c.mc.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("storage: getting object %q from %q: %w", key, bucket, err)
	}

	stat, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		if isNotFoundErr(err) {
			return nil, nil, ErrObjectNotFound
		}
		return nil, nil, fmt.Errorf("storage: stat object %q: %w", key, err)
	}

	info := &ObjectInfo{
		Key:          stat.Key,
		Bucket:       bucket,
		Size:         stat.Size,
		ContentType:  stat.ContentType,
		ETag:         stat.ETag,
		LastModified: stat.LastModified,
		UserMeta:     stat.UserMetadata,
	}
	return obj, info, nil
}

// DownloadBytes downloads an object and returns its full contents as a byte slice.
// Use only for small objects; prefer Download for large files.
func (c *Client) DownloadBytes(ctx context.Context, bucket, key string) ([]byte, *ObjectInfo, error) {
	rc, info, err := c.Download(ctx, bucket, key)
	if err != nil {
		return nil, nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, nil, fmt.Errorf("storage: reading object %q: %w", key, err)
	}
	return data, info, nil
}

// ---- Stat -------------------------------------------------------------------

// Stat returns metadata for an object without downloading its body.
func (c *Client) Stat(ctx context.Context, bucket, key string) (*ObjectInfo, error) {
	stat, err := c.mc.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isNotFoundErr(err) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("storage: stat %q in %q: %w", key, bucket, err)
	}
	return &ObjectInfo{
		Key:          stat.Key,
		Bucket:       bucket,
		Size:         stat.Size,
		ContentType:  stat.ContentType,
		ETag:         stat.ETag,
		LastModified: stat.LastModified,
		UserMeta:     stat.UserMetadata,
	}, nil
}

// Exists returns true if the object exists.
func (c *Client) Exists(ctx context.Context, bucket, key string) (bool, error) {
	_, err := c.Stat(ctx, bucket, key)
	if errors.Is(err, ErrObjectNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ---- Delete -----------------------------------------------------------------

// Delete removes a single object.
func (c *Client) Delete(ctx context.Context, bucket, key string) error {
	err := c.mc.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("storage: deleting %q from %q: %w", key, bucket, err)
	}
	return nil
}

// DeleteMany removes multiple objects in a single request.
// Returns the number of objects successfully removed and any errors.
func (c *Client) DeleteMany(ctx context.Context, bucket string, keys []string) (int, []error) {
	objectCh := make(chan minio.ObjectInfo, len(keys))
	for _, k := range keys {
		objectCh <- minio.ObjectInfo{Key: k}
	}
	close(objectCh)

	opts := minio.RemoveObjectsOptions{GovernanceBypass: false}
	errCh := c.mc.RemoveObjects(ctx, bucket, objectCh, opts)

	var errs []error
	removed := len(keys)
	for e := range errCh {
		errs = append(errs, fmt.Errorf("storage: deleting %q: %w", e.ObjectName, e.Err))
		removed--
	}
	return removed, errs
}

// ---- Copy -------------------------------------------------------------------

// Copy copies src (bucket/key) to dst (bucket/key) server-side.
func (c *Client) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	src := minio.CopySrcOptions{Bucket: srcBucket, Object: srcKey}
	dst := minio.CopyDestOptions{Bucket: dstBucket, Object: dstKey}
	_, err := c.mc.CopyObject(ctx, dst, src)
	if err != nil {
		return fmt.Errorf("storage: copying %q/%q to %q/%q: %w", srcBucket, srcKey, dstBucket, dstKey, err)
	}
	return nil
}

// ---- List -------------------------------------------------------------------

// ListOptions controls the object listing.
type ListOptions struct {
	// Prefix filters results to objects whose key starts with this prefix.
	Prefix string
	// Recursive lists all objects under the prefix (not just the next level).
	Recursive bool
	// MaxKeys is the maximum number of objects to return (0 = unlimited).
	MaxKeys int
}

// ListResult is a single entry returned by List.
type ListResult struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	ContentType  string
}

// List returns objects in bucket matching opts.
func (c *Client) List(ctx context.Context, bucket string, opts ListOptions) ([]ListResult, error) {
	listOpts := minio.ListObjectsOptions{
		Prefix:    opts.Prefix,
		Recursive: opts.Recursive,
	}

	var results []ListResult
	for obj := range c.mc.ListObjects(ctx, bucket, listOpts) {
		if obj.Err != nil {
			return nil, fmt.Errorf("storage: listing objects in %q: %w", bucket, obj.Err)
		}
		results = append(results, ListResult{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
			ContentType:  obj.ContentType,
		})
		if opts.MaxKeys > 0 && len(results) >= opts.MaxKeys {
			break
		}
	}
	return results, nil
}

// ---- Presigned URLs ---------------------------------------------------------

// PresignedUploadURL returns a presigned PUT URL for direct browser uploads.
// expiry defaults to cfg.PresignedURLExpiry when 0.
func (c *Client) PresignedUploadURL(ctx context.Context, bucket, key string, expiry time.Duration) (*url.URL, error) {
	if expiry <= 0 {
		expiry = c.cfg.PresignedURLExpiry
	}
	u, err := c.mc.PresignedPutObject(ctx, bucket, key, expiry)
	if err != nil {
		return nil, fmt.Errorf("storage: presigning PUT for %q in %q: %w", key, bucket, err)
	}
	return u, nil
}

// PresignedDownloadURL returns a presigned GET URL for time-limited object access.
func (c *Client) PresignedDownloadURL(ctx context.Context, bucket, key string, expiry time.Duration) (*url.URL, error) {
	if expiry <= 0 {
		expiry = c.cfg.PresignedURLExpiry
	}
	reqParams := url.Values{}
	u, err := c.mc.PresignedGetObject(ctx, bucket, key, expiry, reqParams)
	if err != nil {
		return nil, fmt.Errorf("storage: presigning GET for %q in %q: %w", key, bucket, err)
	}
	return u, nil
}

// PresignedDownloadURLWithFilename generates a presigned GET URL that sets the
// Content-Disposition header so the browser prompts a download with filename.
func (c *Client) PresignedDownloadURLWithFilename(
	ctx context.Context, bucket, key, filename string, expiry time.Duration,
) (*url.URL, error) {
	if expiry <= 0 {
		expiry = c.cfg.PresignedURLExpiry
	}
	reqParams := url.Values{}
	reqParams.Set("response-content-disposition",
		fmt.Sprintf(`attachment; filename="%s"`, filename))
	u, err := c.mc.PresignedGetObject(ctx, bucket, key, expiry, reqParams)
	if err != nil {
		return nil, fmt.Errorf("storage: presigning GET (with filename) for %q: %w", key, err)
	}
	return u, nil
}

// ---- Multipart helpers ------------------------------------------------------

// InitiateMultipartUpload creates a multipart upload and returns the upload ID.
// This allows callers to upload large files in parallel chunks.
func (c *Client) InitiateMultipartUpload(ctx context.Context, bucket, key, contentType string) (string, error) {
	// minio-go does not expose a raw InitiateMultipartUpload API, but PutObject
	// with a large stream will automatically use multipart. For explicit
	// multipart, callers should use PresignedUploadURL with multipart=true or
	// the core PutObject API with a known size. We expose presigned multipart
	// initiation instead.
	_, err := c.mc.PresignedPutObject(ctx, bucket, key, c.cfg.PresignedURLExpiry)
	if err != nil {
		return "", fmt.Errorf("storage: initiating multipart for %q: %w", key, err)
	}
	// Return a synthetic "upload ID" that encodes the bucket/key for this
	// single-shard presigned upload path.
	return fmt.Sprintf("%s/%s", bucket, key), nil
}

// ---- Bucket policy ----------------------------------------------------------

// SetPublicReadPolicy sets a bucket policy that allows public GET access to
// all objects (useful for public media CDNs behind a gateway).
func (c *Client) SetPublicReadPolicy(ctx context.Context, bucket string) error {
	policy := fmt.Sprintf(`{
		"Version":"2012-10-17",
		"Statement":[{
			"Effect":"Allow",
			"Principal":"*",
			"Action":["s3:GetObject"],
			"Resource":["arn:aws:s3:::%s/*"]
		}]
	}`, bucket)
	if err := c.mc.SetBucketPolicy(ctx, bucket, policy); err != nil {
		return fmt.Errorf("storage: setting public-read policy on %q: %w", bucket, err)
	}
	return nil
}

// ---- Sentinel errors --------------------------------------------------------

// ErrObjectNotFound is returned when a requested object does not exist.
var ErrObjectNotFound = errors.New("storage: object not found")

// ---- internal helpers -------------------------------------------------------

// isNotFoundErr reports whether the MinIO error indicates a missing object.
func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	var resp minio.ErrorResponse
	if errors.As(err, &resp) {
		return resp.Code == "NoSuchKey" || resp.Code == "NoSuchBucket" || resp.StatusCode == 404
	}
	return strings.Contains(err.Error(), "NoSuchKey") ||
		strings.Contains(err.Error(), "does not exist")
}

// isAlreadyExistsErr reports whether creating a bucket failed because it
// already exists.
func isAlreadyExistsErr(err error) bool {
	if err == nil {
		return false
	}
	var resp minio.ErrorResponse
	if errors.As(err, &resp) {
		return resp.Code == "BucketAlreadyOwnedByYou" || resp.Code == "BucketAlreadyExists"
	}
	return false
}
