package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/tiktok-clone/moderation-service/internal/config"
	"github.com/tiktok-clone/moderation-service/internal/models"
)

// ---- Rule-based signal weights -----------------------------------------

const (
	// Maximum fraction of tokens that can be hashtags before spam flag.
	hashtagRatioLimit = 0.40
	// Absolute count at which repeated hashtags are flagged.
	duplicateHashtagLimit = 3
	// A URL count above this triggers link-spam signals.
	linkSpamThreshold = 3
	// Jaccard similarity above this with known-spam corpus triggers the flag.
	jaccardSpamThreshold = 0.55
	// Maximum comment/caption length (chars) before it's treated as padding.
	textPaddingLimit = 2200
)

var (
	hashtagRe = regexp.MustCompile(`#\w+`)
	urlRe     = regexp.MustCompile(`https?://[^\s]+`)
	// Common spam phrases (case-insensitive match counts towards score).
	spamPhrases = []string{
		"follow for follow", "f4f", "like for like", "l4l",
		"sub4sub", "check my bio", "click link in bio",
		"dm for promo", "buy followers", "cheap followers",
		"free followers", "make money fast", "work from home",
	}
)

// mlSpamRequest is sent to the optional ML-based spam classifier.
type mlSpamRequest struct {
	Text        string `json:"text"`
	ContentType string `json:"content_type"`
}

// mlSpamResponse is returned by the ML-based spam classifier.
type mlSpamResponse struct {
	Score        float64 `json:"score"`
	Confidence   float64 `json:"confidence"`
	Labels       []string `json:"labels"`
	ModelVersion string  `json:"model_version"`
	ErrorMessage string  `json:"error,omitempty"`
}

// SpamDetector combines rule-based heuristics with an optional ML model.
type SpamDetector struct {
	cfg        config.AIConfig
	thresholds config.ThresholdConfig
	httpClient *http.Client

	// knownSpamTokens is a small corpus used for Jaccard similarity comparison.
	// In production this would be loaded from a database or file.
	knownSpamTokens []tokenSet
}

type tokenSet map[string]struct{}

// NewSpamDetector constructs a SpamDetector.
func NewSpamDetector(cfg config.AIConfig, thresholds config.ThresholdConfig) *SpamDetector {
	d := &SpamDetector{
		cfg:        cfg,
		thresholds: thresholds,
		httpClient: &http.Client{
			Timeout: cfg.ContentClassifierTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        50,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
	d.knownSpamTokens = buildSpamCorpus()
	return d
}

// Detect runs all spam-detection strategies and returns an aggregate score.
func (d *SpamDetector) Detect(ctx context.Context, req *models.ModerationRequest) (*models.DetectorScore, error) {
	start := time.Now()

	text := req.TextContent
	if text == "" && req.ContentType == models.ContentTypeVideo {
		// For videos we analyse the caption/description stored in TextContent.
		// If there's no text there's nothing rule-based to evaluate.
		return &models.DetectorScore{
			DetectorName: "spam_detector",
			Score:        0,
			Confidence:   0.5,
			Labels:       []string{"no_text"},
			LatencyMs:    time.Since(start).Milliseconds(),
			RanAt:        time.Now().UTC(),
		}, nil
	}

	signals := make(map[string]float64)
	var labels []string

	// -- Rule 1: Hashtag density ------------------------------------------
	hashtagScore, hashtagLabels := d.detectHashtagSpam(text)
	signals["hashtag_density"] = hashtagScore
	labels = append(labels, hashtagLabels...)

	// -- Rule 2: Link spam ---------------------------------------------------
	linkScore, linkLabels := d.detectLinkSpam(text)
	signals["link_spam"] = linkScore
	labels = append(labels, linkLabels...)

	// -- Rule 3: Known spam phrases ------------------------------------------
	phraseScore, phraseLabels := d.detectSpamPhrases(text)
	signals["spam_phrases"] = phraseScore
	labels = append(labels, phraseLabels...)

	// -- Rule 4: Text padding (excessive length with low information) ---------
	paddingScore := d.detectTextPadding(text)
	signals["text_padding"] = paddingScore
	if paddingScore > 0.5 {
		labels = append(labels, "text_padding")
	}

	// -- Rule 5: Copy-paste detection via Jaccard similarity -----------------
	jaccardScore, jaccardLabel := d.detectCopyPaste(text)
	signals["jaccard_similarity"] = jaccardScore
	if jaccardLabel != "" {
		labels = append(labels, jaccardLabel)
	}

	// Aggregate rule-based score (weighted average).
	ruleBasedScore := (signals["hashtag_density"]*0.25 +
		signals["link_spam"]*0.25 +
		signals["spam_phrases"]*0.30 +
		signals["text_padding"]*0.10 +
		signals["jaccard_similarity"]*0.10)

	// -- ML-based classifier (optional, best-effort) -------------------------
	mlScore, mlVersion, mlLabels, err := d.callMLClassifier(ctx, text, string(req.ContentType))
	if err != nil {
		// ML failure is non-fatal; we fall back to rule-based score only.
		mlScore = ruleBasedScore
	}
	labels = append(labels, mlLabels...)

	// Final score: blend rule-based (60%) and ML (40%) if ML was available.
	finalScore := ruleBasedScore*0.60 + mlScore*0.40
	if err != nil {
		finalScore = ruleBasedScore
	}
	finalScore = clamp(finalScore, 0, 1)

	return &models.DetectorScore{
		DetectorName: "spam_detector",
		Score:        finalScore,
		Confidence:   0.75,
		Labels:       deduplicate(labels),
		ModelVersion: mlVersion,
		LatencyMs:    time.Since(start).Milliseconds(),
		RanAt:        time.Now().UTC(),
	}, nil
}

// detectHashtagSpam flags excessive or duplicated hashtag usage.
func (d *SpamDetector) detectHashtagSpam(text string) (float64, []string) {
	tags := hashtagRe.FindAllString(strings.ToLower(text), -1)
	if len(tags) == 0 {
		return 0, nil
	}

	words := strings.Fields(text)
	totalTokens := len(words)
	if totalTokens == 0 {
		return 0, nil
	}

	// Count duplicates.
	counts := make(map[string]int, len(tags))
	for _, t := range tags {
		counts[t]++
	}
	maxDup := 0
	for _, c := range counts {
		if c > maxDup {
			maxDup = c
		}
	}

	var labels []string
	score := 0.0

	ratio := float64(len(tags)) / float64(totalTokens)
	if ratio > hashtagRatioLimit {
		score += 0.5
		labels = append(labels, fmt.Sprintf("hashtag_ratio:%.2f", ratio))
	}
	if maxDup >= duplicateHashtagLimit {
		score += 0.5
		labels = append(labels, fmt.Sprintf("duplicate_hashtags:%d", maxDup))
	}
	// Extra weight for egregious cases (> 30 hashtags).
	if len(tags) > 30 {
		score = math.Min(score+0.3, 1.0)
		labels = append(labels, "hashtag_flood")
	}

	return clamp(score, 0, 1), labels
}

// detectLinkSpam flags content with multiple URLs.
func (d *SpamDetector) detectLinkSpam(text string) (float64, []string) {
	urls := urlRe.FindAllString(text, -1)
	count := len(urls)
	if count == 0 {
		return 0, nil
	}
	if count < linkSpamThreshold {
		return float64(count) * 0.1, nil
	}
	score := math.Min(0.3+float64(count-linkSpamThreshold)*0.15, 1.0)
	return score, []string{fmt.Sprintf("link_spam:%d_urls", count)}
}

// detectSpamPhrases scans for common spam/scam phrases.
func (d *SpamDetector) detectSpamPhrases(text string) (float64, []string) {
	lower := strings.ToLower(text)
	var matched []string
	for _, phrase := range spamPhrases {
		if strings.Contains(lower, phrase) {
			matched = append(matched, "phrase:"+strings.ReplaceAll(phrase, " ", "_"))
		}
	}
	if len(matched) == 0 {
		return 0, nil
	}
	score := math.Min(float64(len(matched))*0.25, 1.0)
	return score, matched
}

// detectTextPadding flags text that is unusually long relative to its content.
func (d *SpamDetector) detectTextPadding(text string) float64 {
	runes := []rune(text)
	if len(runes) <= textPaddingLimit {
		return 0
	}
	// Count unique meaningful words.
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	uniqueWords := make(map[string]struct{}, len(words))
	for _, w := range words {
		uniqueWords[strings.ToLower(w)] = struct{}{}
	}
	if len(words) == 0 {
		return 0
	}
	diversityRatio := float64(len(uniqueWords)) / float64(len(words))
	// Low diversity in a long text is a strong padding signal.
	if diversityRatio < 0.2 {
		return 0.8
	}
	if diversityRatio < 0.35 {
		return 0.5
	}
	return 0.2
}

// detectCopyPaste uses Jaccard similarity to compare against a known-spam corpus.
// Returns a score in [0, 1] and a label if the similarity is high.
func (d *SpamDetector) detectCopyPaste(text string) (float64, string) {
	if len(d.knownSpamTokens) == 0 {
		return 0, ""
	}
	inputTokens := tokenize(text)
	if len(inputTokens) < 5 {
		// Too short to be meaningful.
		return 0, ""
	}

	maxSim := 0.0
	for _, spamTokens := range d.knownSpamTokens {
		sim := jaccardSimilarity(inputTokens, spamTokens)
		if sim > maxSim {
			maxSim = sim
		}
	}

	if maxSim >= jaccardSpamThreshold {
		return maxSim, fmt.Sprintf("jaccard_similarity:%.2f", maxSim)
	}
	return maxSim * 0.5, "" // partial score even below threshold
}

// callMLClassifier sends the text to the ML-based content classifier.
// Returns (score, modelVersion, labels, error).
func (d *SpamDetector) callMLClassifier(ctx context.Context, text, contentType string) (float64, string, []string, error) {
	if d.cfg.ContentClassifierEndpoint == "" || text == "" {
		return 0, "", nil, fmt.Errorf("spam_detector: ml classifier not configured or no text")
	}

	payload := mlSpamRequest{
		Text:        text,
		ContentType: contentType,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, "", nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, d.cfg.ContentClassifierEndpoint, bytes.NewReader(body))
	if err != nil {
		return 0, "", nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if d.cfg.ContentClassifierAPIKey != "" {
		httpReq.Header.Set("X-API-Key", d.cfg.ContentClassifierAPIKey)
	}

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return 0, "", nil, fmt.Errorf("spam_detector: ml classifier http: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return 0, "", nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, "", nil, fmt.Errorf("spam_detector: ml classifier status %d", resp.StatusCode)
	}

	var mlResp mlSpamResponse
	if err := json.Unmarshal(rawBody, &mlResp); err != nil {
		return 0, "", nil, err
	}
	if mlResp.ErrorMessage != "" {
		return 0, "", nil, fmt.Errorf("spam_detector: ml classifier error: %s", mlResp.ErrorMessage)
	}

	return clamp(mlResp.Score, 0, 1), mlResp.ModelVersion, mlResp.Labels, nil
}

// IsAutoReject returns true when the spam score exceeds the configured threshold.
func (d *SpamDetector) IsAutoReject(score float64) bool {
	return score >= d.thresholds.SpamAutoReject
}

// IsAutoApprove returns true when the spam score is safely low.
func (d *SpamDetector) IsAutoApprove(score float64) bool {
	return score < d.thresholds.SpamAutoApprove
}

// ---- helpers ---------------------------------------------------------------

// tokenize lowercases, removes punctuation, and splits into words.
func tokenize(text string) tokenSet {
	lower := strings.ToLower(text)
	words := strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	ts := make(tokenSet, len(words))
	for _, w := range words {
		if len(w) > 1 { // skip single characters
			ts[w] = struct{}{}
		}
	}
	return ts
}

// jaccardSimilarity computes |A ∩ B| / |A ∪ B|.
func jaccardSimilarity(a, b tokenSet) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	intersection := 0
	for k := range a {
		if _, ok := b[k]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// deduplicate removes duplicate strings from a slice.
func deduplicate(s []string) []string {
	seen := make(map[string]struct{}, len(s))
	out := make([]string, 0, len(s))
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

// buildSpamCorpus returns a small hard-coded set of known-spam token sets.
// In production, load this from a database or file.
func buildSpamCorpus() []tokenSet {
	samples := []string{
		"follow for follow f4f like for like l4l sub4sub check my bio free followers",
		"make money fast work from home earn dollars click link buy followers cheap",
		"dm for promo sponsored ad buy now limited offer discount coupon code promo",
		"free iphone giveaway enter now win prize click here limited time offer",
	}
	result := make([]tokenSet, len(samples))
	for i, s := range samples {
		result[i] = tokenize(s)
	}
	return result
}
