package queries

import (
	"context"
	"fmt"
	"strings"

	esclient "github.com/tiktok-clone/database/elasticsearch/client"
)

const (
	IndexUsers = "users"
)

// UserSearchService provides all user search operations.
type UserSearchService struct {
	client *esclient.SearchClient
}

// NewUserSearchService creates a UserSearchService.
func NewUserSearchService(client *esclient.SearchClient) *UserSearchService {
	return &UserSearchService{client: client}
}

// -----------------------------------------------------------------
// Request / Response types
// -----------------------------------------------------------------

// UserSearchRequest carries user search parameters.
type UserSearchRequest struct {
	Query        string
	Verified     *bool
	AccountType  string
	Country      string
	Language     string
	MinFollowers int64
	MaxFollowers int64
	MinVideos    int
	ExcludeIDs   []string
	Interests    []string
	Categories   []string
	From         int
	Size         int
	SortBy       string // relevance | followers | recent | score
	After        []interface{}
}

// AutocompleteRequest carries parameters for username/display-name autocomplete.
type AutocompleteRequest struct {
	Prefix   string
	Size     int
	Verified *bool
	Country  string
	Context  map[string]interface{} // completion context filters
}

// SuggestRequest parameters for completion suggester.
type SuggestRequest struct {
	Prefix  string
	Size    int
	Field   string // username.suggest | display_name (choose the field carrying a completion mapping)
	Contexts map[string][]string
}

// SuggestResult holds one completion suggestion.
type SuggestResult struct {
	Text  string
	Score float64
	ID    string
}

// UserResult is the typed representation of a user document.
type UserResult struct {
	UserID        string   `json:"user_id"`
	Username      string   `json:"username"`
	DisplayName   string   `json:"display_name"`
	Bio           string   `json:"bio"`
	AvatarURL     string   `json:"avatar_url"`
	Verified      bool     `json:"verified"`
	FollowerCount int64    `json:"follower_count"`
	VideoCount    int      `json:"video_count"`
	LikeCount     int64    `json:"like_count"`
	ProfileScore  float64  `json:"profile_score"`
	Interests     []string `json:"interests"`
}

// SimilarUserRequest drives a vector-similarity user search.
type SimilarUserRequest struct {
	UserID        string
	Embedding     []float64
	FieldName     string // embedding | content_embedding
	NumCandidates int
	Size          int
	MinFollowers  int64
	ExcludeIDs    []string
}

// -----------------------------------------------------------------
// Full-text / fuzzy user search
// -----------------------------------------------------------------

// Search performs a keyword search over username and display_name.
func (s *UserSearchService) Search(ctx context.Context, req UserSearchRequest) (*esclient.SearchResponse, error) {
	if req.Size <= 0 {
		req.Size = 20
	}
	if req.Size > 100 {
		req.Size = 100
	}

	must := []interface{}{}
	filter := []interface{}{
		map[string]interface{}{"term": map[string]interface{}{"is_active": true}},
		map[string]interface{}{"term": map[string]interface{}{"is_banned": false}},
	}

	if req.Query != "" {
		// Clean query string — strip leading @
		q := strings.TrimPrefix(strings.TrimSpace(req.Query), "@")

		must = append(must, map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []interface{}{
					// Exact-ish username match gets the highest boost
					map[string]interface{}{
						"term": map[string]interface{}{
							"username.keyword": map[string]interface{}{
								"value": strings.ToLower(q),
								"boost": 10.0,
							},
						},
					},
					// Prefix match on the autocomplete field
					map[string]interface{}{
						"match": map[string]interface{}{
							"username.autocomplete": map[string]interface{}{
								"query":  q,
								"boost":  5.0,
							},
						},
					},
					// Fuzzy full-text on raw username
					map[string]interface{}{
						"match": map[string]interface{}{
							"username": map[string]interface{}{
								"query":          q,
								"fuzziness":      "AUTO",
								"prefix_length":  1,
								"boost":          3.0,
							},
						},
					},
					// Display name search
					map[string]interface{}{
						"match": map[string]interface{}{
							"display_name": map[string]interface{}{
								"query":  q,
								"boost":  2.0,
							},
						},
					},
					// Display name autocomplete
					map[string]interface{}{
						"match": map[string]interface{}{
							"display_name.autocomplete": map[string]interface{}{
								"query": q,
								"boost": 1.5,
							},
						},
					},
					// Bio keyword in text
					map[string]interface{}{
						"match": map[string]interface{}{
							"bio": map[string]interface{}{
								"query": q,
								"boost": 0.5,
							},
						},
					},
				},
				"minimum_should_match": 1,
			},
		})
	} else {
		must = append(must, map[string]interface{}{"match_all": map[string]interface{}{}})
	}

	// Categorical filters
	if req.Verified != nil {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"verified": *req.Verified}})
	}
	if req.AccountType != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"account_type": req.AccountType}})
	}
	if req.Country != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"country_code": req.Country}})
	}
	if req.Language != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"language": req.Language}})
	}
	if len(req.Interests) > 0 {
		filter = append(filter, map[string]interface{}{"terms": map[string]interface{}{"interests": req.Interests}})
	}
	if len(req.Categories) > 0 {
		filter = append(filter, map[string]interface{}{"terms": map[string]interface{}{"profile_categories": req.Categories}})
	}

	// Follower range
	if req.MinFollowers > 0 || req.MaxFollowers > 0 {
		fr := map[string]interface{}{}
		if req.MinFollowers > 0 {
			fr["gte"] = req.MinFollowers
		}
		if req.MaxFollowers > 0 {
			fr["lte"] = req.MaxFollowers
		}
		filter = append(filter, map[string]interface{}{"range": map[string]interface{}{"follower_count": fr}})
	}

	if req.MinVideos > 0 {
		filter = append(filter, map[string]interface{}{
			"range": map[string]interface{}{"video_count": map[string]interface{}{"gte": req.MinVideos}},
		})
	}

	mustNot := []interface{}{}
	if len(req.ExcludeIDs) > 0 {
		mustNot = append(mustNot, map[string]interface{}{
			"ids": map[string]interface{}{"values": req.ExcludeIDs},
		})
	}

	boolQuery := map[string]interface{}{
		"bool": map[string]interface{}{
			"must":     must,
			"filter":   filter,
			"must_not": mustNot,
		},
	}

	// Blend BM25 with follower count signal
	query := map[string]interface{}{
		"function_score": map[string]interface{}{
			"query": boolQuery,
			"functions": []map[string]interface{}{
				{
					"field_value_factor": map[string]interface{}{
						"field":    "follower_count",
						"factor":   0.0001,
						"modifier": "log1p",
						"missing":  0,
					},
					"weight": 1.5,
				},
				{
					"field_value_factor": map[string]interface{}{
						"field":    "profile_score",
						"factor":   0.01,
						"modifier": "log1p",
						"missing":  0,
					},
					"weight": 1.0,
				},
				{
					// Verified users receive a modest score boost
					"filter": map[string]interface{}{
						"term": map[string]interface{}{"verified": true},
					},
					"weight": 1.5,
				},
			},
			"score_mode": "sum",
			"boost_mode": "sum",
		},
	}

	sort := buildUserSort(req.SortBy)

	// Aggregations for search facets
	aggs := map[string]interface{}{
		"by_country": map[string]interface{}{
			"terms": map[string]interface{}{"field": "country_code", "size": 20},
		},
		"by_account_type": map[string]interface{}{
			"terms": map[string]interface{}{"field": "account_type", "size": 10},
		},
		"verified_count": map[string]interface{}{
			"filter": map[string]interface{}{"term": map[string]interface{}{"verified": true}},
		},
		"follower_ranges": map[string]interface{}{
			"range": map[string]interface{}{
				"field": "follower_count",
				"ranges": []map[string]interface{}{
					{"to": 1000},
					{"from": 1000, "to": 10000},
					{"from": 10000, "to": 100000},
					{"from": 100000, "to": 1000000},
					{"from": 1000000},
				},
			},
		},
		"top_interests": map[string]interface{}{
			"terms": map[string]interface{}{"field": "interests", "size": 20},
		},
	}

	return s.client.Search(ctx, esclient.SearchRequest{
		Index:          []string{IndexUsers},
		Query:          query,
		Sort:           sort,
		Aggregations:   aggs,
		From:           req.From,
		Size:           req.Size,
		TrackTotalHits: true,
		SearchAfter:    req.After,
		Highlight:      buildUserHighlight(),
		Source: []string{
			"user_id", "username", "display_name", "bio", "avatar_url",
			"verified", "follower_count", "video_count", "like_count",
			"profile_score", "account_type", "country_code", "interests",
		},
	})
}

// -----------------------------------------------------------------
// Autocomplete
// -----------------------------------------------------------------

// Autocomplete returns prefix-matched users for a search-as-you-type experience.
func (s *UserSearchService) Autocomplete(ctx context.Context, req AutocompleteRequest) (*esclient.SearchResponse, error) {
	if req.Size <= 0 {
		req.Size = 10
	}
	if req.Size > 30 {
		req.Size = 30
	}

	prefix := strings.TrimPrefix(strings.TrimSpace(req.Prefix), "@")
	if prefix == "" {
		return nil, fmt.Errorf("autocomplete prefix must not be empty")
	}

	// Build bool query using edge-ngram field
	should := []interface{}{
		// Exact prefix on keyword for highest confidence
		map[string]interface{}{
			"prefix": map[string]interface{}{
				"username.keyword": map[string]interface{}{
					"value": strings.ToLower(prefix),
					"boost": 6.0,
				},
			},
		},
		// Edge-ngram autocomplete field
		map[string]interface{}{
			"match": map[string]interface{}{
				"username.autocomplete": map[string]interface{}{
					"query": prefix,
					"boost": 4.0,
				},
			},
		},
		// Display name autocomplete
		map[string]interface{}{
			"match": map[string]interface{}{
				"display_name.autocomplete": map[string]interface{}{
					"query": prefix,
					"boost": 2.0,
				},
			},
		},
	}

	filter := []interface{}{
		map[string]interface{}{"term": map[string]interface{}{"is_active": true}},
		map[string]interface{}{"term": map[string]interface{}{"is_banned": false}},
	}
	if req.Verified != nil {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"verified": *req.Verified}})
	}
	if req.Country != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"country_code": req.Country}})
	}

	query := map[string]interface{}{
		"function_score": map[string]interface{}{
			"query": map[string]interface{}{
				"bool": map[string]interface{}{
					"should":               should,
					"filter":               filter,
					"minimum_should_match": 1,
				},
			},
			"functions": []map[string]interface{}{
				{
					"field_value_factor": map[string]interface{}{
						"field":    "follower_count",
						"factor":   0.00005,
						"modifier": "log1p",
						"missing":  0,
					},
					"weight": 1.2,
				},
				{
					"filter": map[string]interface{}{
						"term": map[string]interface{}{"verified": true},
					},
					"weight": 2.0,
				},
			},
			"score_mode": "sum",
			"boost_mode": "sum",
		},
	}

	return s.client.Search(ctx, esclient.SearchRequest{
		Index:          []string{IndexUsers},
		Query:          query,
		Sort:           []map[string]interface{}{{"_score": map[string]interface{}{"order": "desc"}}},
		Size:           req.Size,
		TrackTotalHits: false,
		Source: []string{
			"user_id", "username", "display_name", "avatar_url",
			"verified", "follower_count",
		},
	})
}

// -----------------------------------------------------------------
// Completion suggester
// -----------------------------------------------------------------

// Suggest uses the ES completion suggester for low-latency type-ahead.
func (s *UserSearchService) Suggest(ctx context.Context, req SuggestRequest) ([]SuggestResult, error) {
	if req.Size <= 0 {
		req.Size = 8
	}
	if req.Field == "" {
		req.Field = "username.suggest"
	}

	suggesterBody := map[string]interface{}{
		"prefix": strings.TrimPrefix(strings.TrimSpace(req.Prefix), "@"),
		"completion": map[string]interface{}{
			"field": req.Field,
			"size":  req.Size,
			"skip_duplicates": true,
			"fuzzy": map[string]interface{}{
				"fuzziness":      1,
				"prefix_length":  1,
				"min_length":     3,
				"transpositions": true,
			},
		},
	}

	if len(req.Contexts) > 0 {
		contexts := map[string]interface{}{}
		for k, v := range req.Contexts {
			contexts[k] = v
		}
		suggesterBody["completion"].(map[string]interface{})["contexts"] = contexts
	}

	body := map[string]interface{}{
		"_source": []string{"user_id", "username", "display_name", "avatar_url", "verified", "follower_count"},
		"suggest": map[string]interface{}{
			"user_suggest": suggesterBody,
		},
	}

	// Suggest calls require a raw body sent via sc.Client() directly using the
	// native ES completion suggester API. Here we provide an autocomplete-query
	// fallback so the public API remains uniform across all callers. To use the
	// native completion suggester, call sc.Client().Search() with a body of the
	// form: {"suggest":{"user_suggest":{...}}} against the users index.

	// Fallback: run an autocomplete query
	autoResp, err := s.Autocomplete(ctx, AutocompleteRequest{
		Prefix: req.Prefix,
		Size:   req.Size,
	})
	if err != nil {
		return nil, fmt.Errorf("suggest (autocomplete fallback): %w", err)
	}

	results := make([]SuggestResult, 0, len(autoResp.Hits))
	for _, hit := range autoResp.Hits {
		score := 0.0
		if hit.Score != nil {
			score = *hit.Score
		}
		results = append(results, SuggestResult{
			ID:    hit.ID,
			Score: score,
		})
	}

	return results, nil
}

// -----------------------------------------------------------------
// Fuzzy matching
// -----------------------------------------------------------------

// FuzzySearch finds users whose username is similar to the query term
// using Levenshtein distance (fuzziness AUTO).
func (s *UserSearchService) FuzzySearch(ctx context.Context, query string, size int) (*esclient.SearchResponse, error) {
	if size <= 0 {
		size = 10
	}
	q := strings.TrimPrefix(strings.TrimSpace(query), "@")
	if q == "" {
		return nil, fmt.Errorf("fuzzy search query must not be empty")
	}

	esQuery := map[string]interface{}{
		"bool": map[string]interface{}{
			"should": []interface{}{
				map[string]interface{}{
					"fuzzy": map[string]interface{}{
						"username.keyword": map[string]interface{}{
							"value":          strings.ToLower(q),
							"fuzziness":      "AUTO",
							"prefix_length":  1,
							"max_expansions": 50,
							"transpositions": true,
							"boost":          3.0,
						},
					},
				},
				map[string]interface{}{
					"match": map[string]interface{}{
						"display_name": map[string]interface{}{
							"query":     q,
							"fuzziness": "AUTO",
							"boost":     1.5,
						},
					},
				},
			},
			"filter": []interface{}{
				map[string]interface{}{"term": map[string]interface{}{"is_active": true}},
				map[string]interface{}{"term": map[string]interface{}{"is_banned": false}},
			},
			"minimum_should_match": 1,
		},
	}

	return s.client.Search(ctx, esclient.SearchRequest{
		Index:          []string{IndexUsers},
		Query:          esQuery,
		Sort:           []map[string]interface{}{{"_score": map[string]interface{}{"order": "desc"}}},
		Size:           size,
		TrackTotalHits: false,
		Source: []string{
			"user_id", "username", "display_name", "avatar_url", "verified", "follower_count",
		},
	})
}

// -----------------------------------------------------------------
// Similar users (kNN vector search)
// -----------------------------------------------------------------

// FindSimilarUsers uses kNN search to find users with similar profiles.
func (s *UserSearchService) FindSimilarUsers(ctx context.Context, req SimilarUserRequest) (*esclient.SearchResponse, error) {
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
		map[string]interface{}{"term": map[string]interface{}{"is_banned": false}},
	}
	if req.UserID != "" {
		filter = append(filter, map[string]interface{}{
			"bool": map[string]interface{}{
				"must_not": map[string]interface{}{
					"term": map[string]interface{}{"user_id": req.UserID},
				},
			},
		})
	}
	if len(req.ExcludeIDs) > 0 {
		filter = append(filter, map[string]interface{}{
			"bool": map[string]interface{}{
				"must_not": map[string]interface{}{
					"ids": map[string]interface{}{"values": req.ExcludeIDs},
				},
			},
		})
	}
	if req.MinFollowers > 0 {
		filter = append(filter, map[string]interface{}{
			"range": map[string]interface{}{"follower_count": map[string]interface{}{"gte": req.MinFollowers}},
		})
	}

	query := map[string]interface{}{
		"bool": map[string]interface{}{
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
			"filter": filter,
		},
	}

	return s.client.Search(ctx, esclient.SearchRequest{
		Index:          []string{IndexUsers},
		Query:          query,
		Size:           req.Size,
		TrackTotalHits: false,
		Source: []string{
			"user_id", "username", "display_name", "avatar_url",
			"verified", "follower_count", "profile_score", "interests",
		},
	})
}

// -----------------------------------------------------------------
// People you may know
// -----------------------------------------------------------------

// PeopleYouMayKnowRequest drives mutual-interest user discovery.
type PeopleYouMayKnowRequest struct {
	UserID      string
	Interests   []string
	Hashtags    []string
	Country     string
	Language    string
	FollowedIDs []string
	ExcludeIDs  []string
	Size        int
}

// PeopleYouMayKnow returns users sharing interests/hashtags with the caller.
func (s *UserSearchService) PeopleYouMayKnow(ctx context.Context, req PeopleYouMayKnowRequest) (*esclient.SearchResponse, error) {
	if req.Size <= 0 {
		req.Size = 20
	}

	filter := []interface{}{
		map[string]interface{}{"term": map[string]interface{}{"is_active": true}},
		map[string]interface{}{"term": map[string]interface{}{"is_banned": false}},
	}
	if req.Country != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"country_code": req.Country}})
	}
	if req.Language != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"language": req.Language}})
	}

	mustNot := []interface{}{}
	if req.UserID != "" {
		mustNot = append(mustNot, map[string]interface{}{
			"term": map[string]interface{}{"user_id": req.UserID},
		})
	}
	// Exclude already-followed users
	if len(req.FollowedIDs) > 0 {
		mustNot = append(mustNot, map[string]interface{}{
			"ids": map[string]interface{}{"values": req.FollowedIDs},
		})
	}
	if len(req.ExcludeIDs) > 0 {
		mustNot = append(mustNot, map[string]interface{}{
			"ids": map[string]interface{}{"values": req.ExcludeIDs},
		})
	}

	should := []interface{}{}
	if len(req.Interests) > 0 {
		should = append(should, map[string]interface{}{
			"terms": map[string]interface{}{
				"interests":      req.Interests,
				"boost":          2.0,
			},
		})
	}

	query := map[string]interface{}{
		"function_score": map[string]interface{}{
			"query": map[string]interface{}{
				"bool": map[string]interface{}{
					"filter":               filter,
					"should":               should,
					"must_not":             mustNot,
					"minimum_should_match": 0,
				},
			},
			"functions": []map[string]interface{}{
				{
					"field_value_factor": map[string]interface{}{
						"field":    "follower_count",
						"factor":   0.00005,
						"modifier": "log1p",
						"missing":  0,
					},
					"weight": 1.0,
				},
				{
					"filter": map[string]interface{}{
						"term": map[string]interface{}{"verified": true},
					},
					"weight": 1.5,
				},
			},
			"score_mode": "sum",
			"boost_mode": "sum",
		},
	}

	return s.client.Search(ctx, esclient.SearchRequest{
		Index:          []string{IndexUsers},
		Query:          query,
		Sort:           []map[string]interface{}{{"_score": map[string]interface{}{"order": "desc"}}},
		Size:           req.Size,
		TrackTotalHits: false,
		Source: []string{
			"user_id", "username", "display_name", "avatar_url",
			"verified", "follower_count", "interests",
		},
	})
}

// -----------------------------------------------------------------
// Trending creators
// -----------------------------------------------------------------

// TrendingCreatorsRequest parameters for the rising creators query.
type TrendingCreatorsRequest struct {
	Country    string
	Language   string
	Category   string
	Verified   *bool
	MinVideos  int
	From       int
	Size       int
}

// GetTrendingCreators returns creators with high recent growth.
func (s *UserSearchService) GetTrendingCreators(ctx context.Context, req TrendingCreatorsRequest) (*esclient.SearchResponse, error) {
	if req.Size <= 0 {
		req.Size = 20
	}

	filter := []interface{}{
		map[string]interface{}{"term": map[string]interface{}{"is_active": true}},
		map[string]interface{}{"term": map[string]interface{}{"is_banned": false}},
	}
	if req.Country != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"country_code": req.Country}})
	}
	if req.Language != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"language": req.Language}})
	}
	if req.Category != "" {
		filter = append(filter, map[string]interface{}{"terms": map[string]interface{}{"profile_categories": []string{req.Category}}})
	}
	if req.Verified != nil {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"verified": *req.Verified}})
	}
	if req.MinVideos > 0 {
		filter = append(filter, map[string]interface{}{
			"range": map[string]interface{}{"video_count": map[string]interface{}{"gte": req.MinVideos}},
		})
	}

	query := map[string]interface{}{
		"function_score": map[string]interface{}{
			"query": map[string]interface{}{
				"bool": map[string]interface{}{
					"filter": filter,
				},
			},
			"functions": []map[string]interface{}{
				{
					"field_value_factor": map[string]interface{}{
						"field":    "follower_count",
						"factor":   0.0001,
						"modifier": "log1p",
						"missing":  0,
					},
					"weight": 1.5,
				},
				{
					"field_value_factor": map[string]interface{}{
						"field":    "like_count",
						"factor":   0.00001,
						"modifier": "log1p",
						"missing":  0,
					},
					"weight": 1.0,
				},
				{
					"field_value_factor": map[string]interface{}{
						"field":    "profile_score",
						"factor":   0.1,
						"modifier": "log1p",
						"missing":  0,
					},
					"weight": 2.0,
				},
				{
					"filter": map[string]interface{}{
						"term": map[string]interface{}{"verified": true},
					},
					"weight": 1.3,
				},
			},
			"score_mode": "sum",
			"boost_mode": "replace",
		},
	}

	sort := []map[string]interface{}{
		{"_score": map[string]interface{}{"order": "desc"}},
		{"follower_count": map[string]interface{}{"order": "desc"}},
	}

	aggs := map[string]interface{}{
		"by_category": map[string]interface{}{
			"terms": map[string]interface{}{"field": "profile_categories", "size": 15},
		},
		"by_country": map[string]interface{}{
			"terms": map[string]interface{}{"field": "country_code", "size": 20},
		},
		"follower_stats": map[string]interface{}{
			"stats": map[string]interface{}{"field": "follower_count"},
		},
	}

	return s.client.Search(ctx, esclient.SearchRequest{
		Index:          []string{IndexUsers},
		Query:          query,
		Sort:           sort,
		Aggregations:   aggs,
		From:           req.From,
		Size:           req.Size,
		TrackTotalHits: true,
		Source: []string{
			"user_id", "username", "display_name", "avatar_url",
			"verified", "follower_count", "video_count", "like_count",
			"profile_score", "profile_categories", "country_code",
		},
	})
}

// -----------------------------------------------------------------
// Helper functions
// -----------------------------------------------------------------

func buildUserSort(sortBy string) []map[string]interface{} {
	switch sortBy {
	case "followers":
		return []map[string]interface{}{
			{"follower_count": map[string]interface{}{"order": "desc"}},
			{"_score": map[string]interface{}{"order": "desc"}},
		}
	case "recent":
		return []map[string]interface{}{
			{"created_at": map[string]interface{}{"order": "desc"}},
			{"_score": map[string]interface{}{"order": "desc"}},
		}
	case "score":
		return []map[string]interface{}{
			{"profile_score": map[string]interface{}{"order": "desc"}},
			{"_score": map[string]interface{}{"order": "desc"}},
		}
	default: // "relevance"
		return []map[string]interface{}{
			{"_score": map[string]interface{}{"order": "desc"}},
			{"follower_count": map[string]interface{}{"order": "desc", "missing": 0}},
		}
	}
}

func buildUserHighlight() map[string]interface{} {
	return map[string]interface{}{
		"pre_tags":  []string{"<mark>"},
		"post_tags": []string{"</mark>"},
		"fields": map[string]interface{}{
			"username":     map[string]interface{}{"number_of_fragments": 1},
			"display_name": map[string]interface{}{"number_of_fragments": 1},
			"bio":          map[string]interface{}{"number_of_fragments": 1, "fragment_size": 150},
		},
	}
}

// NormalizeUsername lowercases and strips the leading @ from a username string.
func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(username), "@"))
}
