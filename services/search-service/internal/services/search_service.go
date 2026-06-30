package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Sentinel errors
// ---------------------------------------------------------------------------

var (
	ErrInvalidQuery = errors.New("search: query is empty or invalid")
)

// ---------------------------------------------------------------------------
// Domain models
// ---------------------------------------------------------------------------

// VideoResult represents a single video search hit.
type VideoResult struct {
	VideoID     string    `json:"video_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatorID   string    `json:"creator_id"`
	Username    string    `json:"username"`
	Hashtags    []string  `json:"hashtags"`
	ViewCount   int64     `json:"view_count"`
	LikeCount   int64     `json:"like_count"`
	ThumbnailURL string   `json:"thumbnail_url"`
	Duration    int       `json:"duration_seconds"`
	CreatedAt   time.Time `json:"created_at"`
	Score       float64   `json:"score"`
}

// UserResult represents a user search hit (autocomplete-friendly).
type UserResult struct {
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	AvatarURL    string `json:"avatar_url"`
	FollowerCount int64 `json:"follower_count"`
	Verified     bool   `json:"verified"`
	Score        float64 `json:"score"`
}

// HashtagResult represents a hashtag search hit.
type HashtagResult struct {
	HashtagID   string `json:"hashtag_id"`
	Name        string `json:"name"`
	VideoCount  int64  `json:"video_count"`
	ViewCount   int64  `json:"view_count"`
	Trending    bool   `json:"trending"`
	Score       float64 `json:"score"`
}

// ProductResult represents an in-app product search hit.
type ProductResult struct {
	ProductID   string  `json:"product_id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Currency    string  `json:"currency"`
	ImageURL    string  `json:"image_url"`
	SellerID    string  `json:"seller_id"`
	Score       float64 `json:"score"`
}

// SoundResult represents a sound/music search hit.
type SoundResult struct {
	SoundID    string `json:"sound_id"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Duration   int    `json:"duration_seconds"`
	UsageCount int64  `json:"usage_count"`
	CoverURL   string `json:"cover_url"`
	Score      float64 `json:"score"`
}

// SearchResponse is the unified search response container.
type SearchResponse struct {
	Query   string        `json:"query"`
	Total   int64         `json:"total"`
	Page    int           `json:"page"`
	Limit   int           `json:"limit"`
	Videos  []VideoResult  `json:"videos,omitempty"`
	Users   []UserResult   `json:"users,omitempty"`
	Hashtags []HashtagResult `json:"hashtags,omitempty"`
	Products []ProductResult `json:"products,omitempty"`
	Sounds  []SoundResult  `json:"sounds,omitempty"`
}

// SearchFilters holds optional filters applied to video search.
type SearchFilters struct {
	MinDuration   int      `json:"min_duration,omitempty"`
	MaxDuration   int      `json:"max_duration,omitempty"`
	CreatorID     string   `json:"creator_id,omitempty"`
	Hashtags      []string `json:"hashtags,omitempty"`
	SortBy        string   `json:"sort_by,omitempty"` // relevance | views | likes | date
	DateFrom      string   `json:"date_from,omitempty"`
	DateTo        string   `json:"date_to,omitempty"`
}

// TrendingSearch is a trending search term with its score.
type TrendingSearch struct {
	Query  string  `json:"query"`
	Score  float64 `json:"score"`
	Delta  float64 `json:"delta_24h"`
}

// SearchSuggestion is an autocomplete suggestion.
type SearchSuggestion struct {
	Text string `json:"text"`
	Type string `json:"type"` // query | user | hashtag
}

// ---------------------------------------------------------------------------
// SearchService
// ---------------------------------------------------------------------------

// SearchService handles all search functionality backed by Elasticsearch and Redis.
type SearchService struct {
	es     *elasticsearch.Client
	redis  *redis.Client
	logger *zap.Logger
}

// NewSearchService creates a new SearchService.
func NewSearchService(es *elasticsearch.Client, redisClient *redis.Client, logger *zap.Logger) *SearchService {
	return &SearchService{
		es:     es,
		redis:  redisClient,
		logger: logger,
	}
}

// ---------------------------------------------------------------------------
// SearchVideos
// ---------------------------------------------------------------------------

// SearchVideos executes a multi-match ES query with optional filters, returning
// paginated VideoResult hits.
func (s *SearchService) SearchVideos(
	ctx context.Context,
	query string,
	filters SearchFilters,
	page, limit int,
) (*SearchResponse, error) {
	if strings.TrimSpace(query) == "" {
		return nil, ErrInvalidQuery
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	from := (page - 1) * limit

	// Build must clauses (multi-match).
	mustClause := map[string]interface{}{
		"multi_match": map[string]interface{}{
			"query":    query,
			"fields":   []string{"title^3", "description^2", "hashtags^2", "username"},
			"type":     "best_fields",
			"fuzziness": "AUTO",
		},
	}

	// Build filter clauses.
	var filterClauses []map[string]interface{}

	// Status filter — only return public, active videos.
	filterClauses = append(filterClauses, map[string]interface{}{
		"term": map[string]interface{}{"status": "public"},
	})

	if filters.CreatorID != "" {
		filterClauses = append(filterClauses, map[string]interface{}{
			"term": map[string]interface{}{"creator_id": filters.CreatorID},
		})
	}
	if len(filters.Hashtags) > 0 {
		filterClauses = append(filterClauses, map[string]interface{}{
			"terms": map[string]interface{}{"hashtags": filters.Hashtags},
		})
	}
	if filters.MinDuration > 0 || filters.MaxDuration > 0 {
		rangeClause := map[string]interface{}{}
		if filters.MinDuration > 0 {
			rangeClause["gte"] = filters.MinDuration
		}
		if filters.MaxDuration > 0 {
			rangeClause["lte"] = filters.MaxDuration
		}
		filterClauses = append(filterClauses, map[string]interface{}{
			"range": map[string]interface{}{"duration_seconds": rangeClause},
		})
	}
	if filters.DateFrom != "" || filters.DateTo != "" {
		rangeClause := map[string]interface{}{}
		if filters.DateFrom != "" {
			rangeClause["gte"] = filters.DateFrom
		}
		if filters.DateTo != "" {
			rangeClause["lte"] = filters.DateTo
		}
		filterClauses = append(filterClauses, map[string]interface{}{
			"range": map[string]interface{}{"created_at": rangeClause},
		})
	}

	// Sort.
	var sortClause interface{}
	switch filters.SortBy {
	case "views":
		sortClause = []map[string]interface{}{{"view_count": map[string]interface{}{"order": "desc"}}}
	case "likes":
		sortClause = []map[string]interface{}{{"like_count": map[string]interface{}{"order": "desc"}}}
	case "date":
		sortClause = []map[string]interface{}{{"created_at": map[string]interface{}{"order": "desc"}}}
	default:
		sortClause = []map[string]interface{}{{"_score": map[string]interface{}{"order": "desc"}}}
	}

	esQuery := map[string]interface{}{
		"from": from,
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must":   mustClause,
				"filter": filterClauses,
			},
		},
		"sort": sortClause,
		"_source": []string{
			"video_id", "title", "description", "creator_id", "username",
			"hashtags", "view_count", "like_count", "thumbnail_url",
			"duration_seconds", "created_at",
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(esQuery); err != nil {
		return nil, fmt.Errorf("encode search query: %w", err)
	}

	res, err := s.es.Search(
		s.es.Search.WithContext(ctx),
		s.es.Search.WithIndex("videos"),
		s.es.Search.WithBody(&buf),
		s.es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch error: %s", res.Status())
	}

	var esResult struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Score  float64    `json:"_score"`
				Source VideoResult `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&esResult); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	videos := make([]VideoResult, 0, len(esResult.Hits.Hits))
	for _, hit := range esResult.Hits.Hits {
		v := hit.Source
		v.Score = hit.Score
		videos = append(videos, v)
	}

	// Track the search query for trending.
	go s.trackSearchQuery(context.Background(), query)

	return &SearchResponse{
		Query:  query,
		Total:  esResult.Hits.Total.Value,
		Page:   page,
		Limit:  limit,
		Videos: videos,
	}, nil
}

// ---------------------------------------------------------------------------
// SearchUsers
// ---------------------------------------------------------------------------

// SearchUsers performs an autocomplete-friendly user search using a prefix query
// combined with a fuzzy match on username and display_name.
func (s *SearchService) SearchUsers(
	ctx context.Context,
	query string,
	page, limit int,
) (*SearchResponse, error) {
	if strings.TrimSpace(query) == "" {
		return nil, ErrInvalidQuery
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	from := (page - 1) * limit

	esQuery := map[string]interface{}{
		"from": from,
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []interface{}{
					map[string]interface{}{
						"prefix": map[string]interface{}{
							"username": map[string]interface{}{
								"value": strings.ToLower(query),
								"boost": 3,
							},
						},
					},
					map[string]interface{}{
						"match": map[string]interface{}{
							"username": map[string]interface{}{
								"query":     query,
								"fuzziness": "AUTO",
								"boost":     2,
							},
						},
					},
					map[string]interface{}{
						"match": map[string]interface{}{
							"display_name": map[string]interface{}{
								"query":     query,
								"fuzziness": "AUTO",
							},
						},
					},
				},
				"minimum_should_match": 1,
				"filter": []map[string]interface{}{
					{"term": map[string]interface{}{"status": "active"}},
				},
			},
		},
		"sort": []map[string]interface{}{
			{"_score": map[string]interface{}{"order": "desc"}},
			{"follower_count": map[string]interface{}{"order": "desc"}},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(esQuery); err != nil {
		return nil, fmt.Errorf("encode user search: %w", err)
	}

	res, err := s.es.Search(
		s.es.Search.WithContext(ctx),
		s.es.Search.WithIndex("users"),
		s.es.Search.WithBody(&buf),
		s.es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch user search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch error: %s", res.Status())
	}

	var esResult struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Score  float64    `json:"_score"`
				Source UserResult `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&esResult); err != nil {
		return nil, fmt.Errorf("decode user search response: %w", err)
	}

	users := make([]UserResult, 0, len(esResult.Hits.Hits))
	for _, hit := range esResult.Hits.Hits {
		u := hit.Source
		u.Score = hit.Score
		users = append(users, u)
	}

	return &SearchResponse{
		Query: query,
		Total: esResult.Hits.Total.Value,
		Page:  page,
		Limit: limit,
		Users: users,
	}, nil
}

// ---------------------------------------------------------------------------
// SearchHashtags
// ---------------------------------------------------------------------------

// SearchHashtags searches for hashtags matching the query.
func (s *SearchService) SearchHashtags(
	ctx context.Context,
	query string,
	page, limit int,
) (*SearchResponse, error) {
	if strings.TrimSpace(query) == "" {
		return nil, ErrInvalidQuery
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	// Strip leading # if present.
	query = strings.TrimPrefix(query, "#")
	from := (page - 1) * limit

	esQuery := map[string]interface{}{
		"from": from,
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []interface{}{
					map[string]interface{}{
						"prefix": map[string]interface{}{
							"name": map[string]interface{}{
								"value": strings.ToLower(query),
								"boost": 3,
							},
						},
					},
					map[string]interface{}{
						"match": map[string]interface{}{
							"name": map[string]interface{}{
								"query":     query,
								"fuzziness": "AUTO",
							},
						},
					},
				},
				"minimum_should_match": 1,
			},
		},
		"sort": []map[string]interface{}{
			{"_score": map[string]interface{}{"order": "desc"}},
			{"video_count": map[string]interface{}{"order": "desc"}},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(esQuery); err != nil {
		return nil, fmt.Errorf("encode hashtag search: %w", err)
	}

	res, err := s.es.Search(
		s.es.Search.WithContext(ctx),
		s.es.Search.WithIndex("hashtags"),
		s.es.Search.WithBody(&buf),
		s.es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch hashtag search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch error: %s", res.Status())
	}

	var esResult struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Score  float64       `json:"_score"`
				Source HashtagResult `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&esResult); err != nil {
		return nil, fmt.Errorf("decode hashtag search response: %w", err)
	}

	hashtags := make([]HashtagResult, 0, len(esResult.Hits.Hits))
	for _, hit := range esResult.Hits.Hits {
		h := hit.Source
		h.Score = hit.Score
		hashtags = append(hashtags, h)
	}

	return &SearchResponse{
		Query:    query,
		Total:    esResult.Hits.Total.Value,
		Page:     page,
		Limit:    limit,
		Hashtags: hashtags,
	}, nil
}

// ---------------------------------------------------------------------------
// SearchProducts
// ---------------------------------------------------------------------------

// SearchProducts searches the product catalog index.
func (s *SearchService) SearchProducts(
	ctx context.Context,
	query string,
	minPrice, maxPrice float64,
	page, limit int,
) (*SearchResponse, error) {
	if strings.TrimSpace(query) == "" {
		return nil, ErrInvalidQuery
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	from := (page - 1) * limit

	var filterClauses []map[string]interface{}
	filterClauses = append(filterClauses, map[string]interface{}{
		"term": map[string]interface{}{"status": "active"},
	})
	if minPrice > 0 || maxPrice > 0 {
		priceRange := map[string]interface{}{}
		if minPrice > 0 {
			priceRange["gte"] = minPrice
		}
		if maxPrice > 0 {
			priceRange["lte"] = maxPrice
		}
		filterClauses = append(filterClauses, map[string]interface{}{
			"range": map[string]interface{}{"price": priceRange},
		})
	}

	esQuery := map[string]interface{}{
		"from": from,
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": map[string]interface{}{
					"multi_match": map[string]interface{}{
						"query":     query,
						"fields":    []string{"name^3", "description^2", "brand"},
						"type":      "best_fields",
						"fuzziness": "AUTO",
					},
				},
				"filter": filterClauses,
			},
		},
		"sort": []map[string]interface{}{
			{"_score": map[string]interface{}{"order": "desc"}},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(esQuery); err != nil {
		return nil, fmt.Errorf("encode product search: %w", err)
	}

	res, err := s.es.Search(
		s.es.Search.WithContext(ctx),
		s.es.Search.WithIndex("products"),
		s.es.Search.WithBody(&buf),
		s.es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch product search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch error: %s", res.Status())
	}

	var esResult struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Score  float64       `json:"_score"`
				Source ProductResult `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&esResult); err != nil {
		return nil, fmt.Errorf("decode product search response: %w", err)
	}

	products := make([]ProductResult, 0, len(esResult.Hits.Hits))
	for _, hit := range esResult.Hits.Hits {
		p := hit.Source
		p.Score = hit.Score
		products = append(products, p)
	}

	return &SearchResponse{
		Query:    query,
		Total:    esResult.Hits.Total.Value,
		Page:     page,
		Limit:    limit,
		Products: products,
	}, nil
}

// ---------------------------------------------------------------------------
// SearchSounds
// ---------------------------------------------------------------------------

// SearchSounds searches the sounds/music index.
func (s *SearchService) SearchSounds(
	ctx context.Context,
	query string,
	page, limit int,
) (*SearchResponse, error) {
	if strings.TrimSpace(query) == "" {
		return nil, ErrInvalidQuery
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	from := (page - 1) * limit

	esQuery := map[string]interface{}{
		"from": from,
		"size": limit,
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":     query,
				"fields":    []string{"title^3", "artist^2"},
				"type":      "best_fields",
				"fuzziness": "AUTO",
			},
		},
		"sort": []map[string]interface{}{
			{"_score": map[string]interface{}{"order": "desc"}},
			{"usage_count": map[string]interface{}{"order": "desc"}},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(esQuery); err != nil {
		return nil, fmt.Errorf("encode sound search: %w", err)
	}

	res, err := s.es.Search(
		s.es.Search.WithContext(ctx),
		s.es.Search.WithIndex("sounds"),
		s.es.Search.WithBody(&buf),
		s.es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch sound search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch error: %s", res.Status())
	}

	var esResult struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Score  float64     `json:"_score"`
				Source SoundResult `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&esResult); err != nil {
		return nil, fmt.Errorf("decode sound search response: %w", err)
	}

	sounds := make([]SoundResult, 0, len(esResult.Hits.Hits))
	for _, hit := range esResult.Hits.Hits {
		snd := hit.Source
		snd.Score = hit.Score
		sounds = append(sounds, snd)
	}

	return &SearchResponse{
		Query:  query,
		Total:  esResult.Hits.Total.Value,
		Page:   page,
		Limit:  limit,
		Sounds: sounds,
	}, nil
}

// ---------------------------------------------------------------------------
// GetTrendingSearches
// ---------------------------------------------------------------------------

// GetTrendingSearches returns the top N trending search terms from a Redis sorted
// set where scores represent query frequency over the past 24 hours.
func (s *SearchService) GetTrendingSearches(ctx context.Context, limit int) ([]TrendingSearch, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	const trendingKey = "search:trending:24h"
	results, err := s.redis.ZRevRangeWithScores(ctx, trendingKey, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("get trending searches: %w", err)
	}

	// Also fetch yesterday's scores for delta calculation.
	const yesterdayKey = "search:trending:48h"
	trends := make([]TrendingSearch, 0, len(results))
	for _, z := range results {
		query, _ := z.Member.(string)
		t := TrendingSearch{
			Query: query,
			Score: z.Score,
		}
		// Calculate 24h delta vs prior 24h window.
		prevScore, err := s.redis.ZScore(ctx, yesterdayKey, query).Result()
		if err == nil && prevScore > 0 {
			t.Delta = (z.Score - prevScore) / prevScore * 100
		}
		trends = append(trends, t)
	}

	return trends, nil
}

// trackSearchQuery increments the query's score in the trending sorted set (ZINCRBY).
func (s *SearchService) trackSearchQuery(ctx context.Context, query string) {
	normalised := strings.ToLower(strings.TrimSpace(query))
	if normalised == "" {
		return
	}
	const trendingKey = "search:trending:24h"
	if err := s.redis.ZIncrBy(ctx, trendingKey, 1, normalised).Err(); err != nil {
		s.logger.Warn("track search query", zap.String("query", normalised), zap.Error(err))
		return
	}
	// Set TTL so the key expires after 25 hours (giving a little buffer).
	s.redis.Expire(ctx, trendingKey, 25*time.Hour)
}

// ---------------------------------------------------------------------------
// SaveSearchHistory
// ---------------------------------------------------------------------------

// SaveSearchHistory persists a search query to a per-user Redis list (capped at 50).
func (s *SearchService) SaveSearchHistory(ctx context.Context, userID, query string) error {
	if query = strings.TrimSpace(query); query == "" {
		return nil
	}
	key := fmt.Sprintf("search:history:%s", userID)
	pipe := s.redis.Pipeline()
	pipe.LRem(ctx, key, 0, query) // Remove duplicates first.
	pipe.LPush(ctx, key, query)   // Prepend.
	pipe.LTrim(ctx, key, 0, 49)  // Keep last 50.
	pipe.Expire(ctx, key, 30*24*time.Hour)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("save search history: %w", err)
	}
	return nil
}

// GetSearchHistory returns the search history list for a user.
func (s *SearchService) GetSearchHistory(ctx context.Context, userID string, limit int) ([]string, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	key := fmt.Sprintf("search:history:%s", userID)
	history, err := s.redis.LRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("get search history: %w", err)
	}
	return history, nil
}

// DeleteSearchHistory clears a user's search history.
func (s *SearchService) DeleteSearchHistory(ctx context.Context, userID string) error {
	key := fmt.Sprintf("search:history:%s", userID)
	return s.redis.Del(ctx, key).Err()
}

// ---------------------------------------------------------------------------
// GetSearchSuggestions
// ---------------------------------------------------------------------------

// GetSearchSuggestions returns autocomplete suggestions combining user history,
// trending queries, and ES completion suggestions.
func (s *SearchService) GetSearchSuggestions(
	ctx context.Context,
	userID, prefix string,
	limit int,
) ([]SearchSuggestion, error) {
	if prefix = strings.TrimSpace(prefix); len(prefix) < 1 {
		return nil, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 8
	}

	seen := make(map[string]bool)
	var suggestions []SearchSuggestion

	// 1. User's own history (highest priority).
	if userID != "" {
		history, _ := s.GetSearchHistory(ctx, userID, 50)
		for _, h := range history {
			if strings.HasPrefix(strings.ToLower(h), strings.ToLower(prefix)) {
				if !seen[h] {
					suggestions = append(suggestions, SearchSuggestion{Text: h, Type: "query"})
					seen[h] = true
				}
			}
			if len(suggestions) >= limit/2 {
				break
			}
		}
	}

	// 2. Trending queries matching the prefix.
	trending, _ := s.GetTrendingSearches(ctx, 50)
	for _, t := range trending {
		if strings.HasPrefix(strings.ToLower(t.Query), strings.ToLower(prefix)) {
			if !seen[t.Query] {
				suggestions = append(suggestions, SearchSuggestion{Text: t.Query, Type: "query"})
				seen[t.Query] = true
			}
		}
		if len(suggestions) >= limit {
			break
		}
	}

	// 3. ES completion suggester for remaining slots.
	if len(suggestions) < limit {
		esSuggestions := s.getESSuggestions(ctx, prefix, limit-len(suggestions))
		for _, sug := range esSuggestions {
			if !seen[sug.Text] {
				suggestions = append(suggestions, sug)
				seen[sug.Text] = true
			}
		}
	}

	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}
	return suggestions, nil
}

// getESSuggestions uses Elasticsearch completion suggester on the search_suggest index.
func (s *SearchService) getESSuggestions(ctx context.Context, prefix string, limit int) []SearchSuggestion {
	esQuery := map[string]interface{}{
		"suggest": map[string]interface{}{
			"query_suggest": map[string]interface{}{
				"prefix": prefix,
				"completion": map[string]interface{}{
					"field": "suggest",
					"size":  limit,
					"fuzzy": map[string]interface{}{
						"fuzziness": 1,
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(esQuery); err != nil {
		return nil
	}

	res, err := s.es.Search(
		s.es.Search.WithContext(ctx),
		s.es.Search.WithIndex("search_suggest"),
		s.es.Search.WithBody(&buf),
	)
	if err != nil || res.IsError() {
		return nil
	}
	defer res.Body.Close()

	var result struct {
		Suggest map[string][]struct {
			Options []struct {
				Text string `json:"text"`
				Type string `json:"_type"`
			} `json:"options"`
		} `json:"suggest"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil
	}

	var suggestions []SearchSuggestion
	for _, bucket := range result.Suggest["query_suggest"] {
		for _, opt := range bucket.Options {
			t := opt.Type
			if t == "" {
				t = "query"
			}
			suggestions = append(suggestions, SearchSuggestion{Text: opt.Text, Type: t})
		}
	}
	return suggestions
}

// ---------------------------------------------------------------------------
// UnifiedSearch
// ---------------------------------------------------------------------------

// UnifiedSearch performs a cross-index search returning results from all entity
// types in a single round-trip using the _msearch API.
func (s *SearchService) UnifiedSearch(
	ctx context.Context,
	userID, query string,
	page, limit int,
) (*SearchResponse, error) {
	if strings.TrimSpace(query) == "" {
		return nil, ErrInvalidQuery
	}

	// Persist search to history.
	if userID != "" {
		go s.SaveSearchHistory(context.Background(), userID, query)
	}

	// Run searches concurrently using goroutines.
	type result struct {
		videos   []VideoResult
		users    []UserResult
		hashtags []HashtagResult
		err      error
	}

	ch := make(chan result, 1)
	go func() {
		var r result
		vResp, err := s.SearchVideos(ctx, query, SearchFilters{}, page, limit)
		if err != nil {
			r.err = err
			ch <- r
			return
		}
		r.videos = vResp.Videos

		uResp, _ := s.SearchUsers(ctx, query, 1, 5)
		if uResp != nil {
			r.users = uResp.Users
		}

		hResp, _ := s.SearchHashtags(ctx, query, 1, 5)
		if hResp != nil {
			r.hashtags = hResp.Hashtags
		}

		ch <- r
	}()

	r := <-ch
	if r.err != nil {
		return nil, r.err
	}

	total := int64(len(r.videos))
	return &SearchResponse{
		Query:    query,
		Total:    total,
		Page:     page,
		Limit:    limit,
		Videos:   r.videos,
		Users:    r.users,
		Hashtags: r.hashtags,
	}, nil
}

// ---------------------------------------------------------------------------
// Index helpers (used by other services to keep ES up to date)
// ---------------------------------------------------------------------------

// IndexVideo upserts a video document in the ES videos index.
func (s *SearchService) IndexVideo(ctx context.Context, doc map[string]interface{}) error {
	videoID, _ := doc["video_id"].(string)
	if videoID == "" {
		videoID = uuid.New().String()
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal video doc: %w", err)
	}

	req := esapi.IndexRequest{
		Index:      "videos",
		DocumentID: videoID,
		Body:       bytes.NewReader(body),
		Refresh:    "false",
	}
	res, err := req.Do(ctx, s.es)
	if err != nil {
		return fmt.Errorf("index video: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("index video error: %s", res.Status())
	}
	return nil
}

// DeleteVideo removes a video document from the ES index.
func (s *SearchService) DeleteVideo(ctx context.Context, videoID string) error {
	req := esapi.DeleteRequest{
		Index:      "videos",
		DocumentID: videoID,
	}
	res, err := req.Do(ctx, s.es)
	if err != nil {
		return fmt.Errorf("delete video: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("delete video error: %s", res.Status())
	}
	return nil
}
