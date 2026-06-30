// Package pagination provides cursor-based and offset-based pagination helpers
// for use across all TikTok-clone microservices.
package pagination

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

const (
	// DefaultPageSize is used when the caller does not specify a size.
	DefaultPageSize = 20
	// MaxPageSize is the upper bound on any single page request.
	MaxPageSize = 100
	// MinPageSize is the lower bound on any single page request.
	MinPageSize = 1
)

// ---- Offset pagination ------------------------------------------------------

// OffsetRequest carries parameters for a classic offset/limit query.
type OffsetRequest struct {
	Page int `json:"page" form:"page"`   // 1-based page index
	Size int `json:"size" form:"size"`   // items per page
}

// OffsetMeta is returned alongside a page of results.
type OffsetMeta struct {
	Page       int  `json:"page"`
	Size       int  `json:"size"`
	TotalItems int  `json:"total_items"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// Normalize clamps and defaults the request fields.
func (r *OffsetRequest) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.Size < MinPageSize {
		r.Size = DefaultPageSize
	}
	if r.Size > MaxPageSize {
		r.Size = MaxPageSize
	}
}

// Offset returns the SQL/NoSQL offset value (0-based).
func (r *OffsetRequest) Offset() int {
	r.Normalize()
	return (r.Page - 1) * r.Size
}

// Limit returns the number of rows to fetch.
func (r *OffsetRequest) Limit() int {
	r.Normalize()
	return r.Size
}

// BuildMeta constructs OffsetMeta given the total number of items.
func (r *OffsetRequest) BuildMeta(totalItems int) OffsetMeta {
	r.Normalize()
	totalPages := int(math.Ceil(float64(totalItems) / float64(r.Size)))
	if totalPages < 1 {
		totalPages = 1
	}
	return OffsetMeta{
		Page:       r.Page,
		Size:       r.Size,
		TotalItems: totalItems,
		TotalPages: totalPages,
		HasNext:    r.Page < totalPages,
		HasPrev:    r.Page > 1,
	}
}

// OffsetResponse wraps a slice of items with pagination metadata.
type OffsetResponse[T any] struct {
	Items []T        `json:"items"`
	Meta  OffsetMeta `json:"meta"`
}

// NewOffsetResponse creates an OffsetResponse from items and total count.
func NewOffsetResponse[T any](req *OffsetRequest, items []T, totalItems int) *OffsetResponse[T] {
	return &OffsetResponse[T]{
		Items: items,
		Meta:  req.BuildMeta(totalItems),
	}
}

// ---- Cursor pagination ------------------------------------------------------

// CursorType enumerates the supported cursor strategies.
type CursorType string

const (
	// CursorTypeID uses an opaque ID (e.g. UUID) as the cursor.
	CursorTypeID CursorType = "id"
	// CursorTypeTime uses a timestamp as the cursor (ideal for feeds).
	CursorTypeTime CursorType = "time"
	// CursorTypeComposite uses a combination of time + ID for stable ordering.
	CursorTypeComposite CursorType = "composite"
)

// cursor is the internal, serialisable cursor payload.
type cursor struct {
	Type      CursorType `json:"t"`
	ID        string     `json:"id,omitempty"`
	Timestamp *time.Time `json:"ts,omitempty"`
	// Offset carries additional ordering information for composite cursors.
	Offset string `json:"off,omitempty"`
}

// EncodeCursorID encodes an ID-based cursor to an opaque base64 string.
func EncodeCursorID(id string) (string, error) {
	return encodeCursor(cursor{Type: CursorTypeID, ID: id})
}

// EncodeCursorTime encodes a time-based cursor.
func EncodeCursorTime(ts time.Time) (string, error) {
	return encodeCursor(cursor{Type: CursorTypeTime, Timestamp: &ts})
}

// EncodeCursorComposite encodes a composite (time + id) cursor.
func EncodeCursorComposite(ts time.Time, id string) (string, error) {
	return encodeCursor(cursor{Type: CursorTypeComposite, Timestamp: &ts, ID: id})
}

// DecodeCursor decodes an opaque cursor string back into its components.
// Returns the cursor type, id, and timestamp (the latter two may be zero).
func DecodeCursor(token string) (CursorType, string, *time.Time, error) {
	if token == "" {
		return "", "", nil, nil
	}
	var c cursor
	if err := decodeCursor(token, &c); err != nil {
		return "", "", nil, fmt.Errorf("pagination: invalid cursor: %w", err)
	}
	return c.Type, c.ID, c.Timestamp, nil
}

// CursorRequest carries parameters for a cursor-based page request.
type CursorRequest struct {
	// Cursor is the opaque page token from the previous response.
	// Empty means "start from the beginning".
	Cursor string `json:"cursor" form:"cursor"`
	// Size is the number of items per page.
	Size int `json:"size" form:"size"`
	// Direction is "next" (default) or "prev".
	Direction string `json:"direction" form:"direction"`
}

// Normalize applies defaults.
func (r *CursorRequest) Normalize() {
	if r.Size < MinPageSize {
		r.Size = DefaultPageSize
	}
	if r.Size > MaxPageSize {
		r.Size = MaxPageSize
	}
	if r.Direction == "" {
		r.Direction = "next"
	}
}

// IsForward reports whether the request is paging forward (next).
func (r *CursorRequest) IsForward() bool {
	r.Normalize()
	return r.Direction != "prev"
}

// Limit returns the number of items to fetch.
func (r *CursorRequest) Limit() int {
	r.Normalize()
	return r.Size
}

// CursorMeta is returned alongside a cursor-paged result set.
type CursorMeta struct {
	// NextCursor is the cursor to pass for the next page.  Empty when there is
	// no next page.
	NextCursor string `json:"next_cursor,omitempty"`
	// PrevCursor is the cursor to pass for the previous page. Empty at the
	// start of the result set.
	PrevCursor string `json:"prev_cursor,omitempty"`
	// HasNext indicates that a subsequent page exists.
	HasNext bool `json:"has_next"`
	// HasPrev indicates that a previous page exists.
	HasPrev bool `json:"has_prev"`
	// Size is the number of items in the current page.
	Size int `json:"size"`
}

// CursorResponse wraps a page of items with cursor-based metadata.
type CursorResponse[T any] struct {
	Items []T        `json:"items"`
	Meta  CursorMeta `json:"meta"`
}

// CursorResponseBuilder helps assemble a CursorResponse from a raw result set.
//
// Typical usage:
//
//	// Fetch size+1 rows so we can detect whether a next page exists.
//	rows := db.Query("SELECT ... LIMIT $1", req.Limit()+1)
//	b := pagination.NewCursorResponseBuilder[MyItem](req)
//	for _, row := range rows {
//	    b.Add(row)
//	}
//	resp, err := b.Build(cursorFromItem)
type CursorResponseBuilder[T any] struct {
	req   *CursorRequest
	items []T
}

// NewCursorResponseBuilder creates a builder for the given request.
func NewCursorResponseBuilder[T any](req *CursorRequest) *CursorResponseBuilder[T] {
	req.Normalize()
	return &CursorResponseBuilder[T]{req: req}
}

// Add appends an item to the builder.
func (b *CursorResponseBuilder[T]) Add(item T) {
	b.items = append(b.items, item)
}

// AddAll appends a slice of items.
func (b *CursorResponseBuilder[T]) AddAll(items []T) {
	b.items = append(b.items, items...)
}

// CursorExtractor is a function that derives an opaque cursor from an item.
type CursorExtractor[T any] func(item T) (string, error)

// Build finalises the response. It expects that items contains up to size+1
// entries; the extra entry is used to detect whether a next page exists.
// cursorFn is called on the first and last retained items to generate cursors.
func (b *CursorResponseBuilder[T]) Build(cursorFn CursorExtractor[T]) (*CursorResponse[T], error) {
	size := b.req.Size
	hasMore := len(b.items) > size
	page := b.items
	if hasMore {
		page = page[:size]
	}

	var meta CursorMeta
	meta.Size = len(page)

	if len(page) > 0 {
		// Next cursor points after the last item in this page.
		if hasMore {
			nc, err := cursorFn(page[len(page)-1])
			if err != nil {
				return nil, fmt.Errorf("pagination: building next cursor: %w", err)
			}
			meta.NextCursor = nc
			meta.HasNext = true
		}
		// Prev cursor points before the first item in this page.
		if b.req.Cursor != "" {
			pc, err := cursorFn(page[0])
			if err != nil {
				return nil, fmt.Errorf("pagination: building prev cursor: %w", err)
			}
			meta.PrevCursor = pc
			meta.HasPrev = true
		}
	}

	return &CursorResponse[T]{Items: page, Meta: meta}, nil
}

// ---- internal helpers -------------------------------------------------------

func encodeCursor(c cursor) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeCursor(token string, c *cursor) error {
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, c)
}
