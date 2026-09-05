package pagination

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultPage    = 1
	DefaultPerPage = 20
	MaxPerPage     = 100
	MaxPage        = 1_000_000
)

// Cursor is the stable ordering position used by keyset pagination.
// CreatedAt and ID together make the cursor deterministic when timestamps tie.
type Cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

// EncodeCursor returns an opaque, URL-safe cursor for a created-at/ID pair.
func EncodeCursor(createdAt time.Time, id uuid.UUID) (string, error) {
	if id == uuid.Nil || createdAt.IsZero() {
		return "", fmt.Errorf("cursor position is incomplete")
	}
	payload, err := json.Marshal(Cursor{CreatedAt: createdAt.UTC(), ID: id})
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

// DecodeCursor validates and decodes an opaque keyset cursor.
func DecodeCursor(value string) (Cursor, error) {
	if value == "" {
		return Cursor{}, fmt.Errorf("cursor is empty")
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return Cursor{}, fmt.Errorf("cursor is invalid")
	}
	var cursor Cursor
	if err := json.Unmarshal(payload, &cursor); err != nil || cursor.ID == uuid.Nil || cursor.CreatedAt.IsZero() {
		return Cursor{}, fmt.Errorf("cursor is invalid")
	}
	cursor.CreatedAt = cursor.CreatedAt.UTC()
	return cursor, nil
}

// Params holds pagination parameters.
type Params struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

// Offset returns the SQL offset value.
func (p Params) Offset() int {
	return (p.Page - 1) * p.PerPage
}

// Limit returns the SQL limit value.
func (p Params) Limit() int {
	return p.PerPage
}

// Result holds a paginated result set.
type Result[T any] struct {
	Data       []T `json:"data"`
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalCount int `json:"total_count"`
	TotalPages int `json:"total_pages"`
}

// NewResult creates a paginated result.
func NewResult[T any](data []T, params Params, totalCount int) Result[T] {
	totalPages := totalCount / params.PerPage
	if totalCount%params.PerPage != 0 {
		totalPages++
	}
	return Result[T]{
		Data:       data,
		Page:       params.Page,
		PerPage:    params.PerPage,
		TotalCount: totalCount,
		TotalPages: totalPages,
	}
}

// FromRequest extracts pagination params from an HTTP request.
func FromRequest(r *http.Request) Params {
	page := queryInt(r, "page", DefaultPage)
	perPage := queryInt(r, "per_page", DefaultPerPage)

	if page < 1 {
		page = DefaultPage
	}
	if page > MaxPage {
		page = MaxPage
	}
	if perPage < 1 {
		perPage = DefaultPerPage
	}
	if perPage > MaxPerPage {
		perPage = MaxPerPage
	}

	return Params{Page: page, PerPage: perPage}
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	intVal, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return intVal
}
