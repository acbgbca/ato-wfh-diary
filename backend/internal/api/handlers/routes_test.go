package handlers_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"ato-wfh-diary/internal/api/handlers"
	"ato-wfh-diary/internal/db"
	"ato-wfh-diary/migrations"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// newTestServerWithFrontend starts a test server with a fake frontend FS and the given build hash.
func newTestServerWithFrontend(t *testing.T, buildHash string) *httptest.Server {
	t.Helper()
	database, err := db.Open(":memory:", migrations.FS)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	store := db.NewStore(database)
	_, publicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("generate vapid keys: %v", err)
	}
	h := handlers.NewWithConfig(store, publicKey, "Australia/Melbourne")

	fakeFS := fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte(`<html><head><link rel="stylesheet" href="/css/app.css?v={{.BuildHash}}"></head><body><script src="/js/app.js?v={{.BuildHash}}"></script></body></html>`),
		},
		"js/app.js":  &fstest.MapFile{Data: []byte(`console.log("app")`)},
		"css/app.css": &fstest.MapFile{Data: []byte(`body {}`)},
	}

	router := handlers.NewRouter(h, testAuthHeader, fakeFS, buildHash)
	srv := httptest.NewServer(router)
	t.Cleanup(func() {
		srv.Close()
		database.Close()
	})
	return srv
}

func TestIndexHTML_CacheControlNoCache(t *testing.T) {
	srv := newTestServerWithFrontend(t, "abc123")

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	cc := resp.Header.Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("expected Cache-Control: no-cache, got %q", cc)
	}
}

func TestIndexHTML_BuildHashSubstituted(t *testing.T) {
	srv := newTestServerWithFrontend(t, "abc123")

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "?v=abc123") {
		t.Errorf("expected build hash abc123 in response body, got:\n%s", bodyStr)
	}
	if strings.Contains(bodyStr, "{{.BuildHash}}") {
		t.Errorf("template placeholder was not substituted in response body")
	}
}

func TestJSAsset_LongCacheHeader(t *testing.T) {
	srv := newTestServerWithFrontend(t, "abc123")

	resp, err := http.Get(srv.URL + "/js/app.js")
	if err != nil {
		t.Fatalf("GET /js/app.js: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	cc := resp.Header.Get("Cache-Control")
	if cc != "max-age=31536000, immutable" {
		t.Errorf("expected long cache header, got %q", cc)
	}
}

func TestCSSAsset_LongCacheHeader(t *testing.T) {
	srv := newTestServerWithFrontend(t, "abc123")

	resp, err := http.Get(srv.URL + "/css/app.css")
	if err != nil {
		t.Fatalf("GET /css/app.css: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	cc := resp.Header.Get("Cache-Control")
	if cc != "max-age=31536000, immutable" {
		t.Errorf("expected long cache header, got %q", cc)
	}
}
