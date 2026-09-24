package httpx

import (
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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

func TestCORSAllowAllModePermitsAnyOrigin(t *testing.T) {
	SetAllowedOrigins("*")
	defer SetAllowedOrigins()
	r := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(r)
	c.Request = httptest.NewRequest(http.MethodOptions, "/", nil)
	c.Request.Header.Set("Origin", "https://any.example")
	CORS()(c)
	if r.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for preflight in allow-all mode, got %d", r.Code)
	}
	if got := r.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected wildcard allow-origin, got %q", got)
	}
}

func TestResolveCorsOriginsPrecedence(t *testing.T) {
	if got := resolveCorsOrigins(true, nil); len(got) != 1 || got[0] != "*" {
		t.Fatalf("expected allow-all default, got %v", got)
	}
	if got := resolveCorsOrigins(true, []string{" https://a.example ", ""}); len(got) != 1 || got[0] != "https://a.example" {
		t.Fatalf("expected explicit origins to win and be trimmed, got %v", got)
	}
	if got := resolveCorsOrigins(false, []string{"a,b"}); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("expected comma splitting with allowAll disabled, got %v", got)
	}
	if got := resolveCorsOrigins(false, nil); got != nil {
		t.Fatalf("expected empty whitelist when disabled, got %v", got)
	}
}

func TestGetReturnsHTTPErrorForNon2xx(t *testing.T) {
	previous := client.Load()
	defer client.Store(previous)
	client.Store(&http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader("upstream failed")),
		}, nil
	})})

	body, err := Get("http://example.com", nil)
	if err == nil || body != "upstream failed" {
		t.Fatalf("expected non-2xx error with response body, got body=%q err=%v", body, err)
	}
}

func TestPostXMLEscapesValuesAndRejectsUnsafeNames(t *testing.T) {
	var requestBody string
	previous := client.Load()
	defer client.Store(previous)
	client.Store(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(r.Body)
		requestBody = string(data)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})})

	if _, err := PostXML("http://example.com", map[string]string{"name": "a&b"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(requestBody, "a&amp;b") {
		t.Fatalf("expected escaped XML value, got %q", requestBody)
	}
	if _, err := PostXML("http://example.com", map[string]string{"name><bad": "value"}); err == nil {
		t.Fatal("expected unsafe XML element name to be rejected")
	}
}
