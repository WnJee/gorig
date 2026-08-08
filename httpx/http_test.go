package httpx

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetReturnsTransportError(t *testing.T) {
	_, err := Get("http://127.0.0.1:1", nil)
	if err == nil {
		t.Fatal("expected transport error")
	}
}

func TestBuildURLPreservesExistingQuery(t *testing.T) {
	value, err := buildURL("http://example.com/items?page=1", map[string]string{"q": "a b"})
	if err != nil || value != "http://example.com/items?page=1&q=a+b" {
		t.Fatalf("unexpected URL: %q, %v", value, err)
	}
}

func TestCORSRejectsUnknownCredentialedOrigin(t *testing.T) {
	SetAllowedOrigins("https://allowed.example")
	defer SetAllowedOrigins()
	r := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(r)
	c.Request = httptest.NewRequest(http.MethodOptions, "/", nil)
	c.Request.Header.Set("Origin", "https://unknown.example")
	CORS()(c)
	if r.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", r.Code)
	}
}
