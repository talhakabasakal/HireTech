package pagination

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromRequest_Defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	params := FromRequest(r)

	assert.Equal(t, DefaultPage, params.Page)
	assert.Equal(t, DefaultPerPage, params.PerPage)
}

func TestCursorRoundTrip(t *testing.T) {
	id := uuid.New()
	createdAt := time.Date(2026, 9, 5, 12, 30, 0, 123000000, time.FixedZone("TR", 3*60*60))

	encoded, err := EncodeCursor(createdAt, id)
	require.NoError(t, err)
	decoded, err := DecodeCursor(encoded)
	require.NoError(t, err)

	assert.Equal(t, id, decoded.ID)
	assert.Equal(t, createdAt.UTC(), decoded.CreatedAt)
}

func TestDecodeCursorRejectsTamperingAndIncompleteValues(t *testing.T) {
	_, err := DecodeCursor("not-a-cursor")
	assert.Error(t, err)

	_, err = EncodeCursor(time.Time{}, uuid.New())
	assert.Error(t, err)
	_, err = EncodeCursor(time.Now(), uuid.Nil)
	assert.Error(t, err)
}

func TestFromRequest_CustomValues(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test?page=3&per_page=50", nil)
	params := FromRequest(r)

	assert.Equal(t, 3, params.Page)
	assert.Equal(t, 50, params.PerPage)
}

func TestFromRequest_MaxPerPage(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test?per_page=500", nil)
	params := FromRequest(r)

	assert.Equal(t, MaxPerPage, params.PerPage)
}

func TestFromRequest_MaxPage(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test?page=999999999", nil)
	params := FromRequest(r)

	assert.Equal(t, MaxPage, params.Page)
}

func TestParams_Offset(t *testing.T) {
	params := Params{Page: 3, PerPage: 20}
	assert.Equal(t, 40, params.Offset())
}

func TestNewResult(t *testing.T) {
	data := []string{"a", "b", "c"}
	params := Params{Page: 1, PerPage: 10}
	result := NewResult(data, params, 25)

	assert.Len(t, result.Data, 3)
	assert.Equal(t, 25, result.TotalCount)
	assert.Equal(t, 3, result.TotalPages)
}
