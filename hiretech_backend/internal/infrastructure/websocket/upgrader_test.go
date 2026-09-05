package websocket

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOriginCheckerFailsClosedWhenAllowListIsEmpty(t *testing.T) {
	check := originChecker(nil)

	request := httptest.NewRequest("GET", "http://localhost/api/v1/ws", nil)
	request.Header.Set("Origin", "https://unexpected.example")
	assert.False(t, check(request))

	assert.True(t, check(httptest.NewRequest("GET", "http://localhost/api/v1/ws", nil)))
}
