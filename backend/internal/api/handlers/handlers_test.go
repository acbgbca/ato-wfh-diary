package handlers_test

import (
	"ato-wfh-diary/internal/api/handlers"
	"ato-wfh-diary/internal/db"
	"ato-wfh-diary/migrations"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testAuthHeader = "X-Test-User"

// newTestServer starts a real HTTP test server backed by an in-memory SQLite
// database with all migrations applied. It is torn down automatically when
// the test ends.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	database, err := db.Open(":memory:", migrations.FS)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	store := db.NewStore(database)
	h := handlers.New(store)
	router := handlers.NewRouter(h, testAuthHeader)

	srv := httptest.NewServer(router)
	t.Cleanup(func() {
		srv.Close()
		database.Close()
	})
	return srv
}

// do performs an HTTP request against srv, setting the auth header to
// username. Pass a nil body for requests without a body.
func do(t *testing.T, srv *httptest.Server, method, path, username string, body any) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, srv.URL+path, bodyReader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if username != "" {
		req.Header.Set(testAuthHeader, username)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

// decodeJSON decodes the response body into v.
func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// mustCreateUser calls GET /api/me as username and returns the user ID.
func mustCreateUser(t *testing.T, srv *httptest.Server, username string) int64 {
	t.Helper()
	resp := do(t, srv, http.MethodGet, "/api/me", username, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mustCreateUser %q: status %d", username, resp.StatusCode)
	}
	var user struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, resp, &user)
	return user.ID
}
