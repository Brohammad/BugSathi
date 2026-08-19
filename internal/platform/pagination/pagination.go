package pagination

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Order controls keyset comparison direction for stable cursor paging.
type Order int

const (
	Desc Order = iota
	Asc
)

// Page is a single list request window.
type Page struct {
	Limit  int
	Cursor string
	Order  Order
}

// Result is a page of items plus an opaque cursor for the next page.
type Result[T any] struct {
	Items      []T
	NextCursor string
}

// Limits configures default and maximum page sizes.
type Limits struct {
	Default int
	Max     int
}

func (l Limits) withDefaults() Limits {
	if l.Default <= 0 {
		l.Default = 50
	}
	if l.Max <= 0 {
		l.Max = 100
	}
	if l.Default > l.Max {
		l.Default = l.Max
	}
	return l
}

// Parse reads limit and cursor query params.
func Parse(r *http.Request, limits Limits, order Order) Page {
	limits = limits.withDefaults()
	limit := limits.Default
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	if limit > limits.Max {
		limit = limits.Max
	}
	if limit < 1 {
		limit = 1
	}
	return Page{
		Limit:  limit,
		Cursor: strings.TrimSpace(r.URL.Query().Get("cursor")),
		Order:  order,
	}
}

// EncodeCursor builds an opaque cursor from a sort key.
func EncodeCursor(at time.Time, id uuid.UUID) string {
	raw := at.UTC().Format(time.RFC3339Nano) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor parses a cursor produced by EncodeCursor.
func DecodeCursor(cursor string) (time.Time, uuid.UUID, error) {
	if cursor == "" {
		return time.Time{}, uuid.Nil, fmt.Errorf("empty cursor")
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor decode: %w", err)
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor format")
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor time: %w", err)
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor id: %w", err)
	}
	return at, id, nil
}

// TrimPage returns up to page.Limit items and next_cursor when more exist.
func TrimPage[T any](page Page, items []T, cursor func(T) (time.Time, uuid.UUID)) Result[T] {
	if len(items) <= page.Limit {
		return Result[T]{Items: items}
	}
	items = items[:page.Limit]
	at, id := cursor(items[len(items)-1])
	return Result[T]{Items: items, NextCursor: EncodeCursor(at, id)}
}
