package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tiktok-clone/moderation-service/internal/config"
	"github.com/tiktok-clone/moderation-service/internal/models"
)

// nsfwRequest is the payload sent to the NSFW detection microservice.
type nsfwRequest struct {
	// For video: provide the URL so the detector can fetch frames itself,
	// or provide base64-encoded frame data if self-contained.
	ContentURL   string   `json:"content_url"`
	ContentType  string   `json:"content_type"`
	FrameURLs    []string `json:"frame_urls,omitempty"`
	SampleRate   int      `json:"sample_rate,omitempty"` // frames per second to analyse
}

// nsfwCategory holds per-category probabilities returned by the detector.
type nsfwCategory struct {
	Name        string  `json:"name"`
	Probability float64 `json:"probability"`
}

// nsfwResponse is the response envelope from the NSFW detection service.
type nsfwResponse struct {
	// Aggregate score across all NSFW categories. Range: [0.0, 1.0].
	Score      float64        `json:"score"`
	Confidence float64        `json:"confidence"`
	Categories []nsfwCategory `json:"categories"`
	// Labels that triggered the score (e.g. "explicit_nudity", "suggestive").
	Labels       []string `json:"labels"`
	ModelVersion string   `json:"model_version"`
	AnalysedAt   string   `json:"analysed_at"`
	ErrorMessage string   `json:"error,omitempty"`
}

// NSFWDetector calls an external NSFW detection API and returns a DetectorScore.
type NSFWDetector struct {
	cfg        config.AIConfig
	thresholds config.ThresholdConfig
	httpClient *http.Client
}

// NewNSFWDetector constructs an NSFWDetector with a pre-configured HTTP client.
func NewNSFWDetector(cfg config.AIConfig, thresholds config.ThresholdConfig) *NSFWDetector {
	return &NSFWDetector{
		cfg:        cfg,
		thresholds: thresholds,
		httpClient: &http.Client{
			Timeout: cfg.NSFWTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Detect sends the content to the NSFW detection API and returns a DetectorScore.
// It returns an error only on transport failure; a high score is not an error.
func (d *NSFWDetector) Detect(ctx context.Context, req *models.ModerationRequest) (*models.DetectorScore, error) {
	start := time.Now()

	payload := nsfwRequest{
		ContentURL:  req.ContentURL,
		ContentType: string(req.ContentType),
		SampleRate:  d.cfg.VideoFrameSampleRate,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("nsfw_detector: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, d.cfg.NSFWEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("nsfw_detector: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if d.cfg.NSFWAPIKey != "" {
		httpReq.Header.Set("X-API-Key", d.cfg.NSFWAPIKey)
	}

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("nsfw_detector: http call to %s: %w", d.cfg.NSFWEndpoint, err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // cap at 1 MiB
	if err != nil {
		return nil, fmt.Errorf("nsfw_detector: read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nsfw_detector: unexpected status %d: %s", resp.StatusCode, string(rawBody))
	}

	var nsfwResp nsfwResponse
	if err := json.Unmarshal(rawBody, &nsfwResp); err != nil {
		return nil, fmt.Errorf("nsfw_detector: unmarshal response: %w", err)
	}
	if nsfwResp.ErrorMessage != "" {
		return nil, fmt.Errorf("nsfw_detector: service error: %s", nsfwResp.ErrorMessage)
	}

	// Clamp score to [0.0, 1.0] in case the model returns out-of-range values.
	score := clamp(nsfwResp.Score, 0, 1)

	return &models.DetectorScore{
		DetectorName: "nsfw_detector",
		Score:        score,
		Confidence:   clamp(nsfwResp.Confidence, 0, 1),
		Labels:       nsfwResp.Labels,
		ModelVersion: nsfwResp.ModelVersion,
		LatencyMs:    time.Since(start).Milliseconds(),
		RanAt:        time.Now().UTC(),
	}, nil
}

// IsAutoReject returns true when the score exceeds the configured threshold.
func (d *NSFWDetector) IsAutoReject(score float64) bool {
	return score >= d.thresholds.NSFWAutoReject
}

// IsAutoApprove returns true when the score is safely below the approve threshold.
func (d *NSFWDetector) IsAutoApprove(score float64) bool {
	return score < d.thresholds.NSFWAutoApprove
}

// clamp constrains v to [lo, hi].
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
