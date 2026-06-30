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

// violenceRequest is the payload sent to the violence detection service.
type violenceRequest struct {
	ContentURL  string `json:"content_url"`
	ContentType string `json:"content_type"`
	// For video content, instruct the model to analyse at this frame rate.
	SampleRate int `json:"sample_rate,omitempty"`
}

// violenceCategory holds probability for each type of violent content.
type violenceCategory struct {
	Name        string  `json:"name"`        // e.g. "graphic_violence", "weapon", "blood"
	Probability float64 `json:"probability"` // [0.0, 1.0]
}

// violenceResponse is the response envelope from the violence detection service.
type violenceResponse struct {
	// Aggregate violence score. Range: [0.0, 1.0].
	Score      float64            `json:"score"`
	Confidence float64            `json:"confidence"`
	Categories []violenceCategory `json:"categories"`
	// Machine-readable labels (e.g. "graphic_violence", "weapon_present").
	Labels       []string `json:"labels"`
	ModelVersion string   `json:"model_version"`
	AnalysedAt   string   `json:"analysed_at"`
	ErrorMessage string   `json:"error,omitempty"`
}

// ViolenceDetector calls an external violence detection API.
type ViolenceDetector struct {
	cfg        config.AIConfig
	thresholds config.ThresholdConfig
	httpClient *http.Client
}

// NewViolenceDetector constructs a ViolenceDetector.
func NewViolenceDetector(cfg config.AIConfig, thresholds config.ThresholdConfig) *ViolenceDetector {
	return &ViolenceDetector{
		cfg:        cfg,
		thresholds: thresholds,
		httpClient: &http.Client{
			Timeout: cfg.ViolenceTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Detect sends content to the violence detection API and returns a DetectorScore.
func (d *ViolenceDetector) Detect(ctx context.Context, req *models.ModerationRequest) (*models.DetectorScore, error) {
	start := time.Now()

	payload := violenceRequest{
		ContentURL:  req.ContentURL,
		ContentType: string(req.ContentType),
		SampleRate:  d.cfg.VideoFrameSampleRate,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("violence_detector: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, d.cfg.ViolenceEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("violence_detector: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if d.cfg.ViolenceAPIKey != "" {
		httpReq.Header.Set("X-API-Key", d.cfg.ViolenceAPIKey)
	}

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("violence_detector: http call to %s: %w", d.cfg.ViolenceEndpoint, err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("violence_detector: read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("violence_detector: unexpected status %d: %s", resp.StatusCode, string(rawBody))
	}

	var vResp violenceResponse
	if err := json.Unmarshal(rawBody, &vResp); err != nil {
		return nil, fmt.Errorf("violence_detector: unmarshal response: %w", err)
	}
	if vResp.ErrorMessage != "" {
		return nil, fmt.Errorf("violence_detector: service error: %s", vResp.ErrorMessage)
	}

	// Build a rich label list that includes per-category breakdowns above 0.3.
	labels := vResp.Labels
	for _, cat := range vResp.Categories {
		if cat.Probability >= 0.30 {
			labels = appendIfMissing(labels, fmt.Sprintf("%s:%.2f", cat.Name, cat.Probability))
		}
	}

	return &models.DetectorScore{
		DetectorName: "violence_detector",
		Score:        clamp(vResp.Score, 0, 1),
		Confidence:   clamp(vResp.Confidence, 0, 1),
		Labels:       labels,
		ModelVersion: vResp.ModelVersion,
		LatencyMs:    time.Since(start).Milliseconds(),
		RanAt:        time.Now().UTC(),
	}, nil
}

// IsAutoReject returns true when the violence score exceeds the configured threshold.
func (d *ViolenceDetector) IsAutoReject(score float64) bool {
	return score >= d.thresholds.ViolenceAutoReject
}

// IsAutoApprove returns true when the violence score is safely low.
func (d *ViolenceDetector) IsAutoApprove(score float64) bool {
	return score < d.thresholds.ViolenceAutoApprove
}

// appendIfMissing adds s to slice only if it isn't already present.
func appendIfMissing(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}
