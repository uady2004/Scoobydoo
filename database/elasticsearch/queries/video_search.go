package queries

import (
	"context"
	"fmt"
	"math"
	"time"

	esclient "github.com/tiktok-clone/database/elasticsearch/client"
)

const (
	IndexVideos = "videos"
)

// VideoSearchService provides all video search operations.
type VideoSearchService struct {
	client *esclient.SearchClient
}

// NewVideoSearchService creates a VideoSearchService.
func NewVideoSearchService(client *esclient.SearchClient) *VideoSearchService {
	return &VideoSearchService{client: client}
}

// -----------------------------------------------------------------
// Request / Response types
// -----------------------------------------------------------------

// VideoSearchRequest is the common search parameters struct.
type VideoSearchRequest struct {
	Query         string
	Hashtags      []string
	AuthorID      string
	AuthorIDs     []string
	Language      string
	Country       string
	Category      string
	MinDuration   float64
	MaxDuration   float64
	MinViews      int64
	MinLikes      int64
	Privacy       string
	ExcludeIDs    []string
	ProductIDs    []string
	From          int
	Size          int
	SortBy        string // relevance | trending | recent | views | likes
	After         []interface{}
}

// GeoSearchRequest extends VideoSearchRequest with location parameters.
type GeoSearchRequest struct {
	VideoSearchRequest
	Lat          float64
	Lon          float64
	RadiusKm     float64
	Unit         string // km | mi
}

// TrendingRequest parameters for the trending feed.
type TrendingRequest struct {
	Country     string
	Language    string
	Category    string
	MinScore    float64
	HoursBack   int
	ExcludeIDs  []string
	From        int
	Size        int
}

// SimilarVideoRequest for vector similarity search.
type SimilarVideoRequest struct {
	VideoID      string
	Embedding    []float64
	FieldName    string // embedding | visual_embedding | audio_embedding
	NumCandidates int
	Size          int
	MinScore      *float64
	Filters       map[string]interface{}
}

// VideoResult is the typed search result for a video document.
type VideoResult struct {
	VideoID      string    `json:"video_id"`
	AuthorID     string    `json:"author_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Hashtags     []string  `json:"hashtags"`
	Duration     float64   `json:"duration"`
	ViewCount    int64     `json:"view_count"`
	LikeCount    int64     `json:"like_count"`
	CommentCount int64     `json:"comment_count"`
	ShareCount   int64     `json:"share_count"`
	TrendingScore float64  `json:"trending_score"`
	ThumbnailURL string    `json:"thumbnail_url"`
	CreatedAt    time.Time `json:"created_at"`
}

// -----------------------------------------------------------------
// Full-text search
// -----------------------------------------------------------------

// FullTextSearch performs a multi-field full-text video search.
func (s *VideoSearchService) FullTextSearch(ctx context.Context, req VideoSearchRequest) (*esclient.SearchResponse, error) {
	if req.Size <= 0 {
		req.Size = 20
	}
	if req.Size > 200 {
		req.Size = 200
	}

	// Build the must/filter clauses
	must := []interface{}{}
	filter := []interface{}{
		map[string]interface{}{"term": map[string]interface{}{"is_active": true}},
		map[string]interface{}{"term": map[string]interface{}{"is_deleted": false}},
	}

	if req.Query != "" {
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query": req.Query,
				"fields": []string{
					"title^4",
					"title.autocomplete^3",
					"description^2",
					"hashtags^3",
					"sounds.title^1",
					"sounds.artist^1",
					"location_name^0.5",
				},
				"type":                 "best_fields",
				"fuzziness":            "AUTO",
				"prefix_length":        2,
				"minimum_should_match": "75%",
				"tie_breaker":          0.3,
			},
		})
	} else {
		must = append(must, map[string]interface{}{"match_all": map[string]interface{}{}})
	}

	// Apply categorical filters
	if req.Language != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"language": req.Language}})
	}
	if req.Country != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"country_code": req.Country}})
	}
	if req.Category != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"category": req.Category}})
	}
	if req.AuthorID != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"author_id": req.AuthorID}})
	}
	if len(req.AuthorIDs) > 0 {
		filter = append(filter, map[string]interface{}{"terms": map[string]interface{}{"author_id": req.AuthorIDs}})
	}
	if req.Privacy != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"privacy_level": req.Privacy}})
	}
	if len(req.Hashtags) > 0 {
		filter = append(filter, map[string]interface{}{"terms": map[string]interface{}{"hashtags": req.Hashtags}})
	}
	if len(req.ProductIDs) > 0 {
		filter = append(filter, map[string]interface{}{"terms": map[string]interface{}{"product_ids": req.ProductIDs}})
	}

	// Duration range
	if req.MinDuration > 0 || req.MaxDuration > 0 {
		durationRange := map[string]interface{}{}
		if req.MinDuration > 0 {
			durationRange["gte"] = req.MinDuration
		}
		if req.MaxDuration > 0 {
			durationRange["lte"] = req.MaxDuration
		}
		filter = append(filter, map[string]interface{}{"range": map[string]interface{}{"duration": durationRange}})
	}

	// Count minimums
	if req.MinViews > 0 {
		filter = append(filter, map[string]interface{}{
			"range": map[string]interface{}{"view_count": map[string]interface{}{"gte": req.MinViews}},
		})
	}
	if req.MinLikes > 0 {
		filter = append(filter, map[string]interface{}{
			"range": map[string]interface{}{"like_count": map[string]interface{}{"gte": req.MinLikes}},
		})
	}

	// Exclude specific IDs
	mustNot := []interface{}{}
	if len(req.ExcludeIDs) > 0 {
		mustNot = append(mustNot, map[string]interface{}{
			"ids": map[string]interface{}{"values": req.ExcludeIDs},
		})
	}

	query := map[string]interface{}{
		"bool": map[string]interface{}{
			"must":     must,
			"filter":   filter,
			"must_not": mustNot,
		},
	}

	// Apply function_score to blend BM25 with engagement signals
	if req.Query != "" {
		query = applyFunctionScore(query, 1.0)
	}

	sort := buildVideoSort(req.SortBy)
	searchReq := esclient.SearchRequest{
		Index:          []string{IndexVideos},
		Query:          query,
		Sort:           sort,
		From:           req.From,
		Size:           req.Size,
		TrackTotalHits: true,
		SearchAfter:    req.After,
		Highlight:      buildVideoHighlight(),
		Source: []string{
			"video_id", "author_id", "author_username", "title", "description",
			"hashtags", "duration", "thumbnail_url", "view_count", "like_count",
			"comment_count", "share_count", "trending_score", "created_at",
		},
	}

	return s.client.Search(ctx, searchReq)
}

// -----------------------------------------------------------------
// Hashtag search
// -----------------------------------------------------------------

// SearchByHashtag finds videos matching one or more hashtags.
func (s *VideoSearchService) SearchByHashtag(ctx context.Context, req VideoSearchRequest) (*esclient.SearchResponse, error) {
	if req.Size <= 0 {
		req.Size = 20
	}
	if len(req.Hashtags) == 0 {
		return nil, fmt.Errorf("at least one hashtag is required")
	}

	var hashtagQuery interface{}
	if len(req.Hashtags) == 1 {
		hashtagQuery = map[string]interface{}{
			"term": map[string]interface{}{"hashtags": req.Hashtags[0]},
		}
	} else {
		hashtagQuery = map[string]interface{}{
			"terms": map[string]interface{}{
				"hashtags": req.Hashtags,
				"boost":    2.0,
			},
		}
	}

	filter := []interface{}{
		hashtagQuery,
		map[string]interface{}{"term": map[string]interface{}{"is_active": true}},
		map[string]interface{}{"term": map[string]interface{}{"is_deleted": false}},
	}

	if req.Language != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"language": req.Language}})
	}
	if req.Country != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"country_code": req.Country}})
	}

	// Additional full-text refinement within hashtag results
	must := []interface{}{map[string]interface{}{"match_all": map[string]interface{}{}}}
	if req.Query != "" {
		must = []interface{}{
			map[string]interface{}{
				"multi_match": map[string]interface{}{
					"query":  req.Query,
					"fields": []string{"title^3", "description^2"},
					"type":   "best_fields",
				},
			},
		}
	}

	query := map[string]interface{}{
		"bool": map[string]interface{}{
			"must":   must,
			"filter": filter,
		},
	}

	// Facet aggregations
	aggs := map[string]interface{}{
		"related_hashtags": map[string]interface{}{
			"terms": map[string]interface{}{
				"field": "hashtags",
				"size":  20,
				"exclude": req.Hashtags,
			},
		},
		"authors": map[string]interface{}{
			"terms": map[string]interface{}{
				"field": "author_id",
				"size":  10,
			},
		},
		"categories": map[string]interface{}{
			"terms": map[string]interface{}{
				"field": "category",
				"size":  15,
			},
		},
		"total_views": map[string]interface{}{
			"sum": map[string]interface{}{"field": "view_count"},
		},
	}

	return s.client.Search(ctx, esclient.SearchRequest{
		Index:          []string{IndexVideos},
		Query:          query,
		Sort:           buildVideoSort(req.SortBy),
		Aggregations:   aggs,
		From:           req.From,
		Size:           req.Size,
		TrackTotalHits: true,
		SearchAfter:    req.After,
		Source: []string{
			"video_id", "author_id", "author_username", "title", "hashtags",
			"thumbnail_url", "view_count", "like_count", "trending_score", "created_at",
		},
	})
}

// -----------------------------------------------------------------
// Location-based search
// -----------------------------------------------------------------

// SearchByLocation finds videos near a geographic coordinate.
func (s *VideoSearchService) SearchByLocation(ctx context.Context, req GeoSearchRequest) (*esclient.SearchResponse, error) {
	if req.Size <= 0 {
		req.Size = 20
	}
	if req.RadiusKm <= 0 {
		req.RadiusKm = 50
	}
	unit := req.Unit
	if unit == "" {
		unit = "km"
	}

	radius := fmt.Sprintf("%.0f%s", req.RadiusKm, unit)

	geoFilter := map[string]interface{}{
		"geo_distance": map[string]interface{}{
			"distance": radius,
			"location": map[string]interface{}{
				"lat": req.Lat,
				"lon": req.Lon,
			},
		},
	}

	filter := []interface{}{
		geoFilter,
		map[string]interface{}{"term": map[string]interface{}{"is_active": true}},
		map[string]interface{}{"term": map[string]interface{}{"is_deleted": false}},
	}

	if req.Language != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"language": req.Language}})
	}
	if req.Country != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"country_code": req.Country}})
	}

	must := []interface{}{map[string]interface{}{"match_all": map[string]interface{}{}}}
	if req.Query != "" {
		must = []interface{}{
			map[string]interface{}{
				"multi_match": map[string]interface{}{
					"query":     req.Query,
					"fields":    []string{"title^3", "description^2", "location_name^2"},
					"fuzziness": "AUTO",
				},
			},
		}
	}

	query := map[string]interface{}{
		"bool": map[string]interface{}{
			"must":   must,
			"filter": filter,
		},
	}

	// Sort by distance first, then trending score
	sort := []map[string]interface{}{
		{
			"_geo_distance": map[string]interface{}{
				"location": map[string]interface{}{
					"lat": req.Lat,
					"lon": req.Lon,
				},
				"order":           "asc",
				"unit":            unit,
				"distance_type":   "arc",
				"ignore_unmapped": true,
			},
		},
		{"trending_score": map[string]interface{}{"order": "desc"}},
		{"created_at": map[string]interface{}{"order": "desc"}},
	}

	// Geo-distance aggregation buckets
	aggs := map[string]interface{}{
		"by_distance": map[string]interface{}{
			"geo_distance": map[string]interface{}{
				"field":  "location",
				"origin": fmt.Sprintf("%f,%f", req.Lat, req.Lon),
				"unit":   unit,
				"ranges": []map[string]interface{}{
					{"to": 5},
					{"from": 5, "to": 20},
					{"from": 20, "to": 50},
					{"from": 50},
				},
			},
		},
		"countries": map[string]interface{}{
			"terms": map[string]interface{}{"field": "country_code", "size": 10},
		},
	}

	return s.client.Search(ctx, esclient.SearchRequest{
		Index:          []string{IndexVideos},
		Query:          query,
		Sort:           sort,
		Aggregations:   aggs,
		From:           req.From,
		Size:           req.Size,
		TrackTotalHits: true,
		SearchAfter:    req.After,
		Source: []string{
			"video_id", "author_id", "title", "thumbnail_url", "location",
			"location_name", "view_count", "like_count", "trending_score", "created_at",
		},
	})
}

// -----------------------------------------------------------------
// Trending videos
// -----------------------------------------------------------------

// GetTrending returns videos ranked by trending score within a time window.
func (s *VideoSearchService) GetTrending(ctx context.Context, req TrendingRequest) (*esclient.SearchResponse, error) {
	if req.Size <= 0 {
		req.Size = 20
	}
	if req.HoursBack <= 0 {
		req.HoursBack = 48
	}

	since := time.Now().UTC().Add(-time.Duration(req.HoursBack) * time.Hour)

	filter := []interface{}{
		map[string]interface{}{"term": map[string]interface{}{"is_active": true}},
		map[string]interface{}{"term": map[string]interface{}{"is_deleted": false}},
		map[string]interface{}{
			"range": map[string]interface{}{
				"created_at": map[string]interface{}{"gte": since.Format(time.RFC3339)},
			},
		},
	}

	if req.Country != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"country_code": req.Country}})
	}
	if req.Language != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"language": req.Language}})
	}
	if req.Category != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"category": req.Category}})
	}

	mustNot := []interface{}{}
	if len(req.ExcludeIDs) > 0 {
		mustNot = append(mustNot, map[string]interface{}{
			"ids": map[string]interface{}{"values": req.ExcludeIDs},
		})
	}

	// Blended trending: trending_score + recency decay
	query := map[string]interface{}{
		"function_score": map[string]interface{}{
			"query": map[string]interface{}{
				"bool": map[string]interface{}{
					"filter":   filter,
					"must_not": mustNot,
				},
			},
			"functions": []map[string]interface{}{
				{
					"field_value_factor": map[string]interface{}{
						"field":    "trending_score",
						"factor":   1.5,
						"modifier": "log1p",
						"missing":  0,
					},
				},
				{
					"field_value_factor": map[string]interface{}{
						"field":    "view_count",
						"factor":   0.0001,
						"modifier": "log1p",
						"missing":  0,
					},
				},
				{
					"gauss": map[string]interface{}{
						"created_at": map[string]interface{}{
							"origin": "now",
							"scale":  "12h",
							"offset": "1h",
							"decay":  0.5,
						},
					},
					"weight": 2.0,
				},
			},
			"score_mode": "sum",
			"boost_mode": "replace",
			"min_score":  req.MinScore,
		},
	}

	aggs := map[string]interface{}{
		"top_hashtags": map[string]interface{}{
			"terms": map[string]interface{}{"field": "hashtags", "size": 20},
		},
		"by_category": map[string]interface{}{
			"terms": map[string]interface{}{"field": "category", "size": 10},
		},
		"hourly_trend": map[string]interface{}{
			"date_histogram": map[string]interface{}{
				"field":             "created_at",
				"calendar_interval": "hour",
				"min_doc_count":     1,
			},
			"aggs": map[string]interface{}{
				"avg_trending": map[string]interface{}{
					"avg": map[string]interface{}{"field": "trending_score"},
				},
			},
		},
	}

	sort := []map[string]interface{}{
		{"_score": map[string]interface{}{"order": "desc"}},
		{"trending_score": map[string]interface{}{"order": "desc"}},
		{"created_at": map[string]interface{}{"order": "desc"}},
	}

	return s.client.Search(ctx, esclient.SearchRequest{
		Index:          []string{IndexVideos},
		Query:          query,
		Sort:           sort,
		Aggregations:   aggs,
		From:           req.From,
		Size:           req.Size,
		TrackTotalHits: true,
		SearchAfter:    req.After,
		Source: []string{
			"video_id", "author_id", "author_username", "title", "hashtags",
			"thumbnail_url", "view_count", "like_count", "comment_count",
			"trending_score", "engagement_rate", "created_at",
		},
	})
}

// -----------------------------------------------------------------
// Similarity / vector search
// -----------------------------------------------------------------

// FindSimilarVideos performs a kNN vector similarity search.
func (s *VideoSearchService) FindSimilarVideos(ctx context.Context, req SimilarVideoRequest) (*esclient.SearchResponse, error) {
	if req.Size <= 0 {
		req.Size = 10
	}
	if req.NumCandidates <= 0 {
		req.NumCandidates = req.Size * 10
	}
	if req.FieldName == "" {
		req.FieldName = "embedding"
	}

	filter := []interface{}{
		map[string]interface{}{"term": map[string]interface{}{"is_active": true}},
		map[string]interface{}{"term": map[string]interface{}{"is_deleted": false}},
	}

	// Exclude the source video from results
	if req.VideoID != "" {
		filter = append(filter, map[string]interface{}{
			"bool": map[string]interface{}{
				"must_not": map[string]interface{}{
					"term": map[string]interface{}{"video_id": req.VideoID},
				},
			},
		})
	}

	// Merge caller-supplied filters
	for field, value := range req.Filters {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{field: value},
		})
	}

	// kNN query using the dense_vector field
	query := map[string]interface{}{
		"bool": map[string]interface{}{
			"filter": filter,
			"must": []interface{}{
				map[string]interface{}{
					"knn": map[string]interface{}{
						"field":         req.FieldName,
						"query_vector":  req.Embedding,
						"k":             req.NumCandidates,
						"num_candidates": req.NumCandidates,
					},
				},
			},
		},
	}

	searchReq := esclient.SearchRequest{
		Index:          []string{IndexVideos},
		Query:          query,
		Size:           req.Size,
		MinScore:       req.MinScore,
		TrackTotalHits: false,
		Source: []string{
			"video_id", "author_id", "title", "hashtags",
			"thumbnail_url", "view_count", "like_count", "created_at",
		},
	}

	return s.client.Search(ctx, searchReq)
}

// -----------------------------------------------------------------
// Feed-for-you: personalised video recommendations
// -----------------------------------------------------------------

// PersonalizedFeedRequest drives the personalised feed query.
type PersonalizedFeedRequest struct {
	UserEmbedding []float64
	FollowedIDs   []string
	Hashtags      []string
	Language      string
	Country       string
	ExcludeIDs    []string
	From          int
	Size          int
}

// GetPersonalizedFeed builds a hybrid BM25 + kNN feed query.
func (s *VideoSearchService) GetPersonalizedFeed(ctx context.Context, req PersonalizedFeedRequest) (*esclient.SearchResponse, error) {
	if req.Size <= 0 {
		req.Size = 20
	}

	baseFilter := []interface{}{
		map[string]interface{}{"term": map[string]interface{}{"is_active": true}},
		map[string]interface{}{"term": map[string]interface{}{"is_deleted": false}},
		map[string]interface{}{"term": map[string]interface{}{"privacy_level": "public"}},
	}
	if req.Language != "" {
		baseFilter = append(baseFilter, map[string]interface{}{"term": map[string]interface{}{"language": req.Language}})
	}
	if req.Country != "" {
		baseFilter = append(baseFilter, map[string]interface{}{"term": map[string]interface{}{"country_code": req.Country}})
	}

	mustNot := []interface{}{}
	if len(req.ExcludeIDs) > 0 {
		mustNot = append(mustNot, map[string]interface{}{
			"ids": map[string]interface{}{"values": req.ExcludeIDs},
		})
	}

	should := []interface{}{}
	// Boost videos from followed authors
	if len(req.FollowedIDs) > 0 {
		should = append(should, map[string]interface{}{
			"terms": map[string]interface{}{
				"author_id": req.FollowedIDs,
				"boost":     3.0,
			},
		})
	}
	// Boost by user interest hashtags
	if len(req.Hashtags) > 0 {
		should = append(should, map[string]interface{}{
			"terms": map[string]interface{}{
				"hashtags": req.Hashtags,
				"boost":    2.0,
			},
		})
	}

	boolQuery := map[string]interface{}{
		"bool": map[string]interface{}{
			"filter":               baseFilter,
			"must_not":             mustNot,
			"should":               should,
			"minimum_should_match": 0,
		},
	}

	// Blend semantic similarity with engagement signals
	query := applyFunctionScore(boolQuery, 2.0)

	// Add kNN component for semantic similarity when embedding is available
	if len(req.UserEmbedding) > 0 {
		query = map[string]interface{}{
			"function_score": map[string]interface{}{
				"query": map[string]interface{}{
					"bool": map[string]interface{}{
						"must": []interface{}{
							map[string]interface{}{
								"knn": map[string]interface{}{
									"field":         "embedding",
									"query_vector":  req.UserEmbedding,
									"k":             req.Size * 5,
									"num_candidates": req.Size * 10,
									"boost":         1.5,
								},
							},
						},
						"filter":   baseFilter,
						"should":   should,
						"must_not": mustNot,
					},
				},
				"functions": engagementFunctions(),
				"score_mode": "sum",
				"boost_mode": "sum",
			},
		}
	}

	sort := []map[string]interface{}{
		{"_score": map[string]interface{}{"order": "desc"}},
		{"trending_score": map[string]interface{}{"order": "desc"}},
	}

	return s.client.Search(ctx, esclient.SearchRequest{
		Index:          []string{IndexVideos},
		Query:          query,
		Sort:           sort,
		From:           req.From,
		Size:           req.Size,
		TrackTotalHits: false,
		Source: []string{
			"video_id", "author_id", "author_username", "title", "hashtags",
			"thumbnail_url", "duration", "view_count", "like_count",
			"trending_score", "engagement_rate", "created_at",
		},
	})
}

// -----------------------------------------------------------------
// Multi-search: combined search across video dimensions
// -----------------------------------------------------------------

// CombinedSearchResponse packages results from a multi-query search.
type CombinedSearchResponse struct {
	FullText  *esclient.SearchResponse
	Trending  *esclient.SearchResponse
	NearYou   *esclient.SearchResponse
	TotalHits int64
}

// CombinedSearch runs a full-text search, trending search, and geo search in parallel.
func (s *VideoSearchService) CombinedSearch(
	ctx context.Context,
	textReq VideoSearchRequest,
	trendReq TrendingRequest,
	geoReq GeoSearchRequest,
) (*CombinedSearchResponse, error) {
	// We need all three queries built first, then fan out as a multi-search
	// (simplified here as separate search requests merged into an msearch call)

	textBody := buildFullTextQuery(textReq)
	trendBody := buildTrendingQuery(trendReq)
	geoBody := buildGeoQuery(geoReq)

	mReq := esclient.MultiSearchRequest{
		Requests: []esclient.SearchRequest{
			{
				Index: []string{IndexVideos},
				Query: textBody,
				Sort:  buildVideoSort(textReq.SortBy),
				From:  textReq.From,
				Size:  textReq.Size,
			},
			{
				Index: []string{IndexVideos},
				Query: trendBody,
				Sort:  []map[string]interface{}{{"_score": map[string]interface{}{"order": "desc"}}},
				From:  trendReq.From,
				Size:  trendReq.Size,
			},
			{
				Index: []string{IndexVideos},
				Query: geoBody,
				Sort: []map[string]interface{}{
					{"_geo_distance": map[string]interface{}{
						"location": map[string]interface{}{"lat": geoReq.Lat, "lon": geoReq.Lon},
						"order": "asc", "unit": "km",
					}},
				},
				From: geoReq.From,
				Size: geoReq.Size,
			},
		},
	}

	mRes, err := s.client.MultiSearch(ctx, mReq)
	if err != nil {
		return nil, fmt.Errorf("combined multi-search: %w", err)
	}

	resp := &CombinedSearchResponse{}
	if len(mRes.Responses) > 0 && mRes.Errors[0] == nil {
		resp.FullText = mRes.Responses[0]
		resp.TotalHits += resp.FullText.Total
	}
	if len(mRes.Responses) > 1 && mRes.Errors[1] == nil {
		resp.Trending = mRes.Responses[1]
	}
	if len(mRes.Responses) > 2 && mRes.Errors[2] == nil {
		resp.NearYou = mRes.Responses[2]
	}

	return resp, nil
}

// -----------------------------------------------------------------
// Helper / builder functions
// -----------------------------------------------------------------

func buildVideoSort(sortBy string) []map[string]interface{} {
	switch sortBy {
	case "recent":
		return []map[string]interface{}{
			{"created_at": map[string]interface{}{"order": "desc"}},
			{"_score": map[string]interface{}{"order": "desc"}},
		}
	case "views":
		return []map[string]interface{}{
			{"view_count": map[string]interface{}{"order": "desc"}},
			{"_score": map[string]interface{}{"order": "desc"}},
		}
	case "likes":
		return []map[string]interface{}{
			{"like_count": map[string]interface{}{"order": "desc"}},
			{"_score": map[string]interface{}{"order": "desc"}},
		}
	case "trending":
		return []map[string]interface{}{
			{"trending_score": map[string]interface{}{"order": "desc"}},
			{"_score": map[string]interface{}{"order": "desc"}},
		}
	default: // "relevance"
		return []map[string]interface{}{
			{"_score": map[string]interface{}{"order": "desc"}},
			{"trending_score": map[string]interface{}{"order": "desc"}},
			{"created_at": map[string]interface{}{"order": "desc"}},
		}
	}
}

func buildVideoHighlight() map[string]interface{} {
	return map[string]interface{}{
		"pre_tags":  []string{"<mark>"},
		"post_tags": []string{"</mark>"},
		"fields": map[string]interface{}{
			"title":       map[string]interface{}{"number_of_fragments": 1, "fragment_size": 120},
			"description": map[string]interface{}{"number_of_fragments": 2, "fragment_size": 200},
		},
	}
}

func applyFunctionScore(query map[string]interface{}, queryWeight float64) map[string]interface{} {
	return map[string]interface{}{
		"function_score": map[string]interface{}{
			"query":      query,
			"functions":  engagementFunctions(),
			"score_mode": "sum",
			"boost_mode": "sum",
			"boost":      queryWeight,
		},
	}
}

func engagementFunctions() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"field_value_factor": map[string]interface{}{
				"field":    "view_count",
				"factor":   0.00001,
				"modifier": "log1p",
				"missing":  0,
			},
			"weight": 1.0,
		},
		{
			"field_value_factor": map[string]interface{}{
				"field":    "like_count",
				"factor":   0.0001,
				"modifier": "log1p",
				"missing":  0,
			},
			"weight": 1.5,
		},
		{
			"field_value_factor": map[string]interface{}{
				"field":    "engagement_rate",
				"factor":   100,
				"modifier": "log1p",
				"missing":  0,
			},
			"weight": 2.0,
		},
		{
			"gauss": map[string]interface{}{
				"created_at": map[string]interface{}{
					"origin": "now",
					"scale":  "24h",
					"offset": "2h",
					"decay":  0.4,
				},
			},
			"weight": 1.5,
		},
	}
}

func buildFullTextQuery(req VideoSearchRequest) map[string]interface{} {
	must := []interface{}{}
	filter := []interface{}{
		map[string]interface{}{"term": map[string]interface{}{"is_active": true}},
		map[string]interface{}{"term": map[string]interface{}{"is_deleted": false}},
	}

	if req.Query != "" {
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  req.Query,
				"fields": []string{"title^4", "description^2", "hashtags^3"},
				"type":   "best_fields",
			},
		})
	} else {
		must = append(must, map[string]interface{}{"match_all": map[string]interface{}{}})
	}

	return map[string]interface{}{
		"bool": map[string]interface{}{
			"must":   must,
			"filter": filter,
		},
	}
}

func buildTrendingQuery(req TrendingRequest) map[string]interface{} {
	since := time.Now().UTC().Add(-time.Duration(req.HoursBack) * time.Hour)
	filter := []interface{}{
		map[string]interface{}{"term": map[string]interface{}{"is_active": true}},
		map[string]interface{}{"term": map[string]interface{}{"is_deleted": false}},
		map[string]interface{}{
			"range": map[string]interface{}{
				"created_at": map[string]interface{}{"gte": since.Format(time.RFC3339)},
			},
		},
	}
	if req.Country != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"country_code": req.Country}})
	}
	return map[string]interface{}{
		"function_score": map[string]interface{}{
			"query":     map[string]interface{}{"bool": map[string]interface{}{"filter": filter}},
			"functions": engagementFunctions(),
			"score_mode": "sum",
			"boost_mode": "replace",
		},
	}
}

func buildGeoQuery(req GeoSearchRequest) map[string]interface{} {
	radius := fmt.Sprintf("%.0fkm", req.RadiusKm)
	if req.RadiusKm <= 0 {
		radius = "50km"
	}
	return map[string]interface{}{
		"bool": map[string]interface{}{
			"filter": []interface{}{
				map[string]interface{}{
					"geo_distance": map[string]interface{}{
						"distance": radius,
						"location": map[string]interface{}{"lat": req.Lat, "lon": req.Lon},
					},
				},
				map[string]interface{}{"term": map[string]interface{}{"is_active": true}},
			},
		},
	}
}

// DecayScore computes a time-decay multiplier (0–1) for a given age.
// Uses an exponential decay with the provided half-life in hours.
func DecayScore(age time.Duration, halfLifeHours float64) float64 {
	hours := age.Hours()
	if hours < 0 {
		return 1.0
	}
	return math.Exp(-0.693 * hours / halfLifeHours)
}
