package esclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"go.uber.org/zap"
)

// Config holds the Elasticsearch client configuration.
type Config struct {
	Addresses         []string
	Username          string
	Password          string
	APIKey            string
	CloudID           string
	CACert            []byte
	MaxRetries        int
	RetryBackoff      time.Duration
	DialTimeout       time.Duration
	ResponseTimeout   time.Duration
	MaxIdleConns      int
	MaxConnsPerHost   int
	IdleConnTimeout   time.Duration
	CompressRequests  bool
	EnableMetrics     bool
	EnableDebugLogger bool
}

// DefaultConfig returns a production-ready default configuration.
func DefaultConfig() Config {
	return Config{
		Addresses:        []string{"http://localhost:9200"},
		MaxRetries:       3,
		RetryBackoff:     100 * time.Millisecond,
		DialTimeout:      5 * time.Second,
		ResponseTimeout:  30 * time.Second,
		MaxIdleConns:     100,
		MaxConnsPerHost:  10,
		IdleConnTimeout:  90 * time.Second,
		CompressRequests: true,
	}
}

// SearchClient wraps the official ES8 client with higher-level abstractions.
type SearchClient struct {
	es     *elasticsearch.Client
	cfg    Config
	logger *zap.Logger
	mu     sync.RWMutex
}

// NewSearchClient constructs and returns a fully configured SearchClient.
func NewSearchClient(cfg Config, logger *zap.Logger) (*SearchClient, error) {
	transport := &http.Transport{
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxConnsPerHost,
		IdleConnTimeout:     cfg.IdleConnTimeout,
		DisableCompression:  !cfg.CompressRequests,
		ForceAttemptHTTP2:   true,
	}

	esCfg := elasticsearch.Config{
		Addresses:     cfg.Addresses,
		Username:      cfg.Username,
		Password:      cfg.Password,
		APIKey:        cfg.APIKey,
		CloudID:       cfg.CloudID,
		CACert:        cfg.CACert,
		MaxRetries:    cfg.MaxRetries,
		RetryBackoff:  func(i int) time.Duration { return cfg.RetryBackoff * time.Duration(i) },
		Transport:     transport,
		CompressRequestBody: cfg.CompressRequests,
	}

	es, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	sc := &SearchClient{
		es:     es,
		cfg:    cfg,
		logger: logger,
	}

	if err := sc.ping(context.Background()); err != nil {
		return nil, fmt.Errorf("elasticsearch ping failed: %w", err)
	}

	return sc, nil
}

// ping verifies connectivity to the cluster.
func (sc *SearchClient) ping(ctx context.Context) error {
	res, err := sc.es.Ping(
		sc.es.Ping.WithContext(ctx),
		sc.es.Ping.WithHuman(),
	)
	if err != nil {
		return fmt.Errorf("ping error: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("ping returned non-2xx: %s", res.Status())
	}
	return nil
}

// Client exposes the underlying ES8 client for advanced use.
func (sc *SearchClient) Client() *elasticsearch.Client {
	return sc.es
}

// -----------------------------------------------------------------
// SearchRequest / SearchResponse types
// -----------------------------------------------------------------

// SearchRequest is a structured search request envelope.
type SearchRequest struct {
	Index          []string
	Query          map[string]interface{}
	Sort           []map[string]interface{}
	Aggregations   map[string]interface{}
	Source         []string
	ExcludeSource  []string
	From           int
	Size           int
	TrackTotalHits interface{} // bool or int
	SearchAfter    []interface{}
	Highlight      map[string]interface{}
	MinScore       *float64
	Timeout        string
}

// SearchResponse is the decoded search response.
type SearchResponse struct {
	Took     int64
	TimedOut bool
	Total    int64
	Hits     []Hit
	Aggs     map[string]interface{}
}

// Hit represents a single search hit.
type Hit struct {
	Index     string
	ID        string
	Score     *float64
	Source    json.RawMessage
	Sort      []interface{}
	Highlight map[string][]string
}

// Search executes a search against one or more indices.
func (sc *SearchClient) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	body := map[string]interface{}{
		"query": req.Query,
		"from":  req.From,
		"size":  req.Size,
	}

	if len(req.Sort) > 0 {
		body["sort"] = req.Sort
	}
	if len(req.Aggregations) > 0 {
		body["aggs"] = req.Aggregations
	}
	if req.TrackTotalHits != nil {
		body["track_total_hits"] = req.TrackTotalHits
	}
	if len(req.SearchAfter) > 0 {
		body["search_after"] = req.SearchAfter
	}
	if len(req.Highlight) > 0 {
		body["highlight"] = req.Highlight
	}
	if req.MinScore != nil {
		body["min_score"] = *req.MinScore
	}

	sourceFilter := map[string]interface{}{}
	if len(req.Source) > 0 {
		sourceFilter["includes"] = req.Source
	}
	if len(req.ExcludeSource) > 0 {
		sourceFilter["excludes"] = req.ExcludeSource
	}
	if len(sourceFilter) > 0 {
		body["_source"] = sourceFilter
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal search body: %w", err)
	}

	opts := []func(*esapi.SearchRequest){
		sc.es.Search.WithContext(ctx),
		sc.es.Search.WithIndex(req.Index...),
		sc.es.Search.WithBody(bytes.NewReader(data)),
		sc.es.Search.WithTrackTotalHits(true),
	}
	if req.Timeout != "" {
		opts = append(opts, sc.es.Search.WithTimeout(req.Timeout))
	}

	res, err := sc.es.Search(opts...)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("search error [%s]: %s", res.Status(), string(body))
	}

	return decodeSearchResponse(res.Body)
}

// decodeSearchResponse decodes the ES response body into SearchResponse.
func decodeSearchResponse(r io.Reader) (*SearchResponse, error) {
	var raw struct {
		Took     int  `json:"took"`
		TimedOut bool `json:"timed_out"`
		Hits     struct {
			Total struct {
				Value    int64  `json:"value"`
				Relation string `json:"relation"`
			} `json:"total"`
			Hits []struct {
				Index     string                 `json:"_index"`
				ID        string                 `json:"_id"`
				Score     *float64               `json:"_score"`
				Source    json.RawMessage        `json:"_source"`
				Sort      []interface{}          `json:"sort"`
				Highlight map[string][]string    `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
		Aggregations map[string]interface{} `json:"aggregations"`
	}

	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	resp := &SearchResponse{
		Took:     int64(raw.Took),
		TimedOut: raw.TimedOut,
		Total:    raw.Hits.Total.Value,
		Aggs:     raw.Aggregations,
	}
	for _, h := range raw.Hits.Hits {
		resp.Hits = append(resp.Hits, Hit{
			Index:     h.Index,
			ID:        h.ID,
			Score:     h.Score,
			Source:    h.Source,
			Sort:      h.Sort,
			Highlight: h.Highlight,
		})
	}
	return resp, nil
}

// -----------------------------------------------------------------
// Multi-search (msearch)
// -----------------------------------------------------------------

// MultiSearchRequest groups multiple search requests.
type MultiSearchRequest struct {
	Requests []SearchRequest
}

// MultiSearchResponse groups the responses in order.
type MultiSearchResponse struct {
	Responses []*SearchResponse
	Errors    []error
}

// MultiSearch executes several searches in a single HTTP round-trip.
func (sc *SearchClient) MultiSearch(ctx context.Context, mreq MultiSearchRequest) (*MultiSearchResponse, error) {
	var buf bytes.Buffer

	for _, req := range mreq.Requests {
		// Header line
		header := map[string]interface{}{
			"index": req.Index,
		}
		headerBytes, err := json.Marshal(header)
		if err != nil {
			return nil, fmt.Errorf("marshal msearch header: %w", err)
		}
		buf.Write(headerBytes)
		buf.WriteByte('\n')

		// Body line
		body := map[string]interface{}{
			"query": req.Query,
			"from":  req.From,
			"size":  req.Size,
		}
		if len(req.Sort) > 0 {
			body["sort"] = req.Sort
		}
		if len(req.Aggregations) > 0 {
			body["aggs"] = req.Aggregations
		}
		if req.TrackTotalHits != nil {
			body["track_total_hits"] = req.TrackTotalHits
		}

		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal msearch body: %w", err)
		}
		buf.Write(bodyBytes)
		buf.WriteByte('\n')
	}

	res, err := sc.es.Msearch(
		bytes.NewReader(buf.Bytes()),
		sc.es.Msearch.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("msearch request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("msearch error [%s]: %s", res.Status(), string(body))
	}

	var raw struct {
		Responses []json.RawMessage `json:"responses"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode msearch response: %w", err)
	}

	mresp := &MultiSearchResponse{
		Responses: make([]*SearchResponse, len(raw.Responses)),
		Errors:    make([]error, len(raw.Responses)),
	}

	for i, raw := range raw.Responses {
		resp, err := decodeSearchResponse(bytes.NewReader(raw))
		mresp.Responses[i] = resp
		mresp.Errors[i] = err
	}

	return mresp, nil
}

// -----------------------------------------------------------------
// Bulk indexing
// -----------------------------------------------------------------

// BulkAction defines the action type for a bulk operation.
type BulkAction string

const (
	BulkActionIndex  BulkAction = "index"
	BulkActionCreate BulkAction = "create"
	BulkActionUpdate BulkAction = "update"
	BulkActionDelete BulkAction = "delete"
)

// BulkItem represents a single item in a bulk operation.
type BulkItem struct {
	Action    BulkAction
	Index     string
	ID        string
	Routing   string
	Pipeline  string
	Document  interface{}
	RetryOnConflict *int
}

// BulkResponse summarizes the result of a bulk operation.
type BulkResponse struct {
	Took   int
	Errors bool
	Items  []BulkItemResult
}

// BulkItemResult holds the per-item result for a bulk action.
type BulkItemResult struct {
	Index  string
	ID     string
	Action BulkAction
	Status int
	Error  *BulkItemError
}

// BulkItemError is the error detail from a failed bulk item.
type BulkItemError struct {
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

// BulkIndex executes a bulk indexing operation.
// For best throughput call this with batches of 500–5000 documents.
func (sc *SearchClient) BulkIndex(ctx context.Context, items []BulkItem) (*BulkResponse, error) {
	if len(items) == 0 {
		return &BulkResponse{}, nil
	}

	var buf bytes.Buffer

	for _, item := range items {
		// Action line
		actionMeta := map[string]interface{}{
			"_index": item.Index,
			"_id":    item.ID,
		}
		if item.Routing != "" {
			actionMeta["routing"] = item.Routing
		}
		if item.Pipeline != "" {
			actionMeta["pipeline"] = item.Pipeline
		}
		if item.RetryOnConflict != nil && item.Action == BulkActionUpdate {
			actionMeta["retry_on_conflict"] = *item.RetryOnConflict
		}

		action := map[string]interface{}{
			string(item.Action): actionMeta,
		}
		actionBytes, err := json.Marshal(action)
		if err != nil {
			return nil, fmt.Errorf("marshal bulk action: %w", err)
		}
		buf.Write(actionBytes)
		buf.WriteByte('\n')

		// Document line (not needed for delete)
		if item.Action != BulkActionDelete && item.Document != nil {
			var docBytes []byte
			if item.Action == BulkActionUpdate {
				docBytes, err = json.Marshal(map[string]interface{}{"doc": item.Document, "doc_as_upsert": true})
			} else {
				docBytes, err = json.Marshal(item.Document)
			}
			if err != nil {
				return nil, fmt.Errorf("marshal bulk document: %w", err)
			}
			buf.Write(docBytes)
			buf.WriteByte('\n')
		}
	}

	res, err := sc.es.Bulk(
		bytes.NewReader(buf.Bytes()),
		sc.es.Bulk.WithContext(ctx),
		sc.es.Bulk.WithRefresh("false"),
	)
	if err != nil {
		return nil, fmt.Errorf("bulk request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("bulk error [%s]: %s", res.Status(), string(b))
	}

	return decodeBulkResponse(res.Body)
}

// decodeBulkResponse decodes the bulk API response.
func decodeBulkResponse(r io.Reader) (*BulkResponse, error) {
	var raw struct {
		Took   int  `json:"took"`
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Index  string         `json:"_index"`
			ID     string         `json:"_id"`
			Status int            `json:"status"`
			Error  *BulkItemError `json:"error"`
		} `json:"items"`
	}

	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode bulk response: %w", err)
	}

	resp := &BulkResponse{
		Took:   raw.Took,
		Errors: raw.Errors,
		Items:  make([]BulkItemResult, 0, len(raw.Items)),
	}

	for _, item := range raw.Items {
		for action, detail := range item {
			resp.Items = append(resp.Items, BulkItemResult{
				Index:  detail.Index,
				ID:     detail.ID,
				Action: BulkAction(action),
				Status: detail.Status,
				Error:  detail.Error,
			})
		}
	}

	return resp, nil
}

// -----------------------------------------------------------------
// Concurrent bulk indexer
// -----------------------------------------------------------------

// BulkIndexerConfig configures the concurrent bulk indexer.
type BulkIndexerConfig struct {
	Workers       int
	FlushBytes    int
	FlushInterval time.Duration
	OnError       func(ctx context.Context, err error)
	OnSuccess     func(ctx context.Context, item BulkItemResult)
	Pipeline      string
}

// ConcurrentBulkIndexer buffers documents and flushes them in batches using worker goroutines.
type ConcurrentBulkIndexer struct {
	client  *SearchClient
	cfg     BulkIndexerConfig
	queue   chan BulkItem
	wg      sync.WaitGroup
	buf     []BulkItem
	bufMu   sync.Mutex
	flushCh chan struct{}
	done    chan struct{}
	logger  *zap.Logger
}

// NewConcurrentBulkIndexer creates a ConcurrentBulkIndexer and starts its workers.
func NewConcurrentBulkIndexer(client *SearchClient, cfg BulkIndexerConfig) *ConcurrentBulkIndexer {
	if cfg.Workers <= 0 {
		cfg.Workers = runtime.NumCPU()
	}
	if cfg.FlushBytes <= 0 {
		cfg.FlushBytes = 5 * 1024 * 1024 // 5 MB
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}

	bi := &ConcurrentBulkIndexer{
		client:  client,
		cfg:     cfg,
		queue:   make(chan BulkItem, cfg.Workers*100),
		flushCh: make(chan struct{}, 1),
		done:    make(chan struct{}),
		logger:  client.logger,
	}

	bi.wg.Add(1)
	go bi.dispatcher()

	return bi
}

// Add enqueues a BulkItem for indexing.
func (bi *ConcurrentBulkIndexer) Add(ctx context.Context, item BulkItem) error {
	select {
	case bi.queue <- item:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close flushes remaining items and shuts down workers.
func (bi *ConcurrentBulkIndexer) Close(ctx context.Context) error {
	close(bi.done)
	bi.wg.Wait()
	return nil
}

func (bi *ConcurrentBulkIndexer) dispatcher() {
	defer bi.wg.Done()

	ticker := time.NewTicker(bi.cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case item, ok := <-bi.queue:
			if !ok {
				bi.flush(context.Background())
				return
			}
			bi.bufMu.Lock()
			bi.buf = append(bi.buf, item)
			shouldFlush := len(bi.buf) >= 500
			bi.bufMu.Unlock()
			if shouldFlush {
				bi.flush(context.Background())
			}

		case <-ticker.C:
			bi.flush(context.Background())

		case <-bi.done:
			bi.flush(context.Background())
			return
		}
	}
}

func (bi *ConcurrentBulkIndexer) flush(ctx context.Context) {
	bi.bufMu.Lock()
	if len(bi.buf) == 0 {
		bi.bufMu.Unlock()
		return
	}
	items := bi.buf
	bi.buf = nil
	bi.bufMu.Unlock()

	resp, err := bi.client.BulkIndex(ctx, items)
	if err != nil {
		if bi.cfg.OnError != nil {
			bi.cfg.OnError(ctx, err)
		}
		return
	}

	if bi.cfg.OnSuccess != nil {
		for _, item := range resp.Items {
			if item.Error == nil {
				bi.cfg.OnSuccess(ctx, item)
			}
		}
	}
	if bi.cfg.OnError != nil {
		for _, item := range resp.Items {
			if item.Error != nil {
				bi.cfg.OnError(ctx, fmt.Errorf("bulk item error [%s/%s]: %s - %s",
					item.Index, item.ID, item.Error.Type, item.Error.Reason))
			}
		}
	}
}

// -----------------------------------------------------------------
// Scroll API
// -----------------------------------------------------------------

// ScrollConfig configures a scroll search.
type ScrollConfig struct {
	Index    []string
	Query    map[string]interface{}
	Sort     []map[string]interface{}
	Size     int
	Source   []string
	KeepAlive string // e.g. "2m"
}

// ScrollResult is a single page of scroll results.
type ScrollResult struct {
	ScrollID string
	Total    int64
	Hits     []Hit
}

// Scroll initiates a scroll search and returns the first page.
func (sc *SearchClient) Scroll(ctx context.Context, cfg ScrollConfig) (*ScrollResult, error) {
	if cfg.Size <= 0 {
		cfg.Size = 1000
	}
	if cfg.KeepAlive == "" {
		cfg.KeepAlive = "2m"
	}

	body := map[string]interface{}{
		"query": cfg.Query,
		"size":  cfg.Size,
	}
	if len(cfg.Sort) > 0 {
		body["sort"] = cfg.Sort
	}
	if len(cfg.Source) > 0 {
		body["_source"] = cfg.Source
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal scroll body: %w", err)
	}

	res, err := sc.es.Search(
		sc.es.Search.WithContext(ctx),
		sc.es.Search.WithIndex(cfg.Index...),
		sc.es.Search.WithBody(bytes.NewReader(data)),
		sc.es.Search.WithScroll(cfg.KeepAlive),
		sc.es.Search.WithSize(cfg.Size),
	)
	if err != nil {
		return nil, fmt.Errorf("scroll init: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("scroll init error [%s]: %s", res.Status(), string(b))
	}

	return decodeScrollResponse(res.Body)
}

// ScrollNext fetches the next page of a scroll search.
func (sc *SearchClient) ScrollNext(ctx context.Context, scrollID, keepAlive string) (*ScrollResult, error) {
	if keepAlive == "" {
		keepAlive = "2m"
	}

	body := map[string]interface{}{
		"scroll":    keepAlive,
		"scroll_id": scrollID,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal scroll body: %w", err)
	}

	res, err := sc.es.Scroll(
		sc.es.Scroll.WithContext(ctx),
		sc.es.Scroll.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		return nil, fmt.Errorf("scroll next: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("scroll next error [%s]: %s", res.Status(), string(b))
	}

	return decodeScrollResponse(res.Body)
}

// ScrollClear releases a scroll context on the server.
func (sc *SearchClient) ScrollClear(ctx context.Context, scrollID string) error {
	body := map[string]interface{}{
		"scroll_id": []string{scrollID},
	}
	data, _ := json.Marshal(body)

	res, err := sc.es.ClearScroll(
		sc.es.ClearScroll.WithContext(ctx),
		sc.es.ClearScroll.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		return fmt.Errorf("clear scroll: %w", err)
	}
	defer res.Body.Close()
	return nil
}

// ScrollAll iterates all pages of a scroll search, calling fn for each page.
// Automatically clears the scroll context on completion or error.
func (sc *SearchClient) ScrollAll(ctx context.Context, cfg ScrollConfig, fn func(hits []Hit) error) error {
	first, err := sc.Scroll(ctx, cfg)
	if err != nil {
		return err
	}

	scrollID := first.ScrollID
	defer func() {
		if scrollID != "" {
			_ = sc.ScrollClear(context.Background(), scrollID)
		}
	}()

	if err := fn(first.Hits); err != nil {
		return err
	}

	for {
		page, err := sc.ScrollNext(ctx, scrollID, cfg.KeepAlive)
		if err != nil {
			return err
		}
		scrollID = page.ScrollID

		if len(page.Hits) == 0 {
			break
		}

		if err := fn(page.Hits); err != nil {
			return err
		}
	}

	return nil
}

func decodeScrollResponse(r io.Reader) (*ScrollResult, error) {
	var raw struct {
		ScrollID string `json:"_scroll_id"`
		Hits     struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Index  string          `json:"_index"`
				ID     string          `json:"_id"`
				Score  *float64        `json:"_score"`
				Source json.RawMessage `json:"_source"`
				Sort   []interface{}   `json:"sort"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode scroll response: %w", err)
	}

	result := &ScrollResult{
		ScrollID: raw.ScrollID,
		Total:    raw.Hits.Total.Value,
		Hits:     make([]Hit, 0, len(raw.Hits.Hits)),
	}
	for _, h := range raw.Hits.Hits {
		result.Hits = append(result.Hits, Hit{
			Index:  h.Index,
			ID:     h.ID,
			Score:  h.Score,
			Source: h.Source,
			Sort:   h.Sort,
		})
	}

	return result, nil
}

// -----------------------------------------------------------------
// Index management helpers
// -----------------------------------------------------------------

// IndexExists returns true if the given index exists.
func (sc *SearchClient) IndexExists(ctx context.Context, index string) (bool, error) {
	res, err := sc.es.Indices.Exists(
		[]string{index},
		sc.es.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		return false, fmt.Errorf("index exists check: %w", err)
	}
	defer res.Body.Close()
	return res.StatusCode == http.StatusOK, nil
}

// CreateIndex creates an index from a JSON settings/mappings body.
func (sc *SearchClient) CreateIndex(ctx context.Context, index string, body []byte) error {
	res, err := sc.es.Indices.Create(
		index,
		sc.es.Indices.Create.WithContext(ctx),
		sc.es.Indices.Create.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("create index error [%s]: %s", res.Status(), string(b))
	}
	return nil
}

// DeleteIndex removes an index.
func (sc *SearchClient) DeleteIndex(ctx context.Context, indices ...string) error {
	res, err := sc.es.Indices.Delete(
		indices,
		sc.es.Indices.Delete.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("delete index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("delete index error [%s]: %s", res.Status(), string(b))
	}
	return nil
}

// PutMapping updates the mapping for an existing index.
func (sc *SearchClient) PutMapping(ctx context.Context, index string, mapping []byte) error {
	res, err := sc.es.Indices.PutMapping(
		[]string{index},
		bytes.NewReader(mapping),
		sc.es.Indices.PutMapping.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("put mapping: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("put mapping error [%s]: %s", res.Status(), string(b))
	}
	return nil
}

// Refresh forces a refresh on the given indices.
func (sc *SearchClient) Refresh(ctx context.Context, indices ...string) error {
	res, err := sc.es.Indices.Refresh(
		sc.es.Indices.Refresh.WithContext(ctx),
		sc.es.Indices.Refresh.WithIndex(indices...),
	)
	if err != nil {
		return fmt.Errorf("refresh: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("refresh error [%s]: %s", res.Status(), string(b))
	}
	return nil
}

// IndexDocument indexes a single document.
func (sc *SearchClient) IndexDocument(ctx context.Context, index, id string, doc interface{}) error {
	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal document: %w", err)
	}

	opts := []func(*esapi.IndexRequest){
		sc.es.Index.WithContext(ctx),
	}
	if id != "" {
		opts = append(opts, sc.es.Index.WithDocumentID(id))
	}

	res, err := sc.es.Index(index, bytes.NewReader(data), opts...)
	if err != nil {
		return fmt.Errorf("index document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("index document error [%s]: %s", res.Status(), string(b))
	}
	return nil
}

// GetDocument retrieves a document by ID.
func (sc *SearchClient) GetDocument(ctx context.Context, index, id string) (json.RawMessage, error) {
	res, err := sc.es.Get(
		index, id,
		sc.es.Get.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("get document error [%s]: %s", res.Status(), string(b))
	}

	var raw struct {
		Source json.RawMessage `json:"_source"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode get response: %w", err)
	}
	return raw.Source, nil
}

// DeleteDocument removes a document by ID.
func (sc *SearchClient) DeleteDocument(ctx context.Context, index, id string) error {
	res, err := sc.es.Delete(
		index, id,
		sc.es.Delete.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() && res.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("delete document error [%s]: %s", res.Status(), string(b))
	}
	return nil
}

// UpdateDocument partially updates a document.
func (sc *SearchClient) UpdateDocument(ctx context.Context, index, id string, partial interface{}) error {
	data, err := json.Marshal(map[string]interface{}{"doc": partial})
	if err != nil {
		return fmt.Errorf("marshal update: %w", err)
	}

	res, err := sc.es.Update(
		index, id,
		bytes.NewReader(data),
		sc.es.Update.WithContext(ctx),
		sc.es.Update.WithRetryOnConflict(3),
	)
	if err != nil {
		return fmt.Errorf("update document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("update document error [%s]: %s", res.Status(), string(b))
	}
	return nil
}

// ClusterHealth returns the cluster health status.
func (sc *SearchClient) ClusterHealth(ctx context.Context) (string, error) {
	res, err := sc.es.Cluster.Health(
		sc.es.Cluster.Health.WithContext(ctx),
	)
	if err != nil {
		return "", fmt.Errorf("cluster health: %w", err)
	}
	defer res.Body.Close()

	var raw struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return "", fmt.Errorf("decode cluster health: %w", err)
	}
	return raw.Status, nil
}

// IndicesCSV joins index names into a comma-separated string for use in
// URL paths when calling the ES REST API directly.
func IndicesCSV(indices []string) string {
	return strings.Join(indices, ",")
}
