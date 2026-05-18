package handlers_test

import (
	"fmt"
	"net/http"
	"testing"
)

func TestGetProfile_NotFound(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodGet, "/api/me/profile", "alice", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGetProfile_Found(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	profile := map[string]any{
		"default_hours": 7.5,
		"mon_type":      "wfh",
		"tue_type":      "wfh",
		"wed_type":      "office",
		"thu_type":      "wfh",
		"fri_type":      "wfh",
		"sat_type":      "weekend",
		"sun_type":      "weekend",
	}
	putResp := do(t, srv, http.MethodPut, "/api/me/profile", "alice", profile)
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status: got %d, want %d", putResp.StatusCode, http.StatusOK)
	}

	resp := do(t, srv, http.MethodGet, "/api/me/profile", "alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got map[string]any
	decodeJSON(t, resp, &got)
	if got["default_hours"] != 7.5 {
		t.Errorf("default_hours: got %v, want 7.5", got["default_hours"])
	}
	if got["mon_type"] != "wfh" {
		t.Errorf("mon_type: got %v, want wfh", got["mon_type"])
	}
	if got["wed_type"] != "office" {
		t.Errorf("wed_type: got %v, want office", got["wed_type"])
	}
	if got["sat_type"] != "weekend" {
		t.Errorf("sat_type: got %v, want weekend", got["sat_type"])
	}
}

func TestPutProfile_Create(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	profile := map[string]any{
		"default_hours": 8.0,
		"mon_type":      "wfh",
		"tue_type":      "wfh",
		"wed_type":      "wfh",
		"thu_type":      "wfh",
		"fri_type":      "wfh",
		"sat_type":      "weekend",
		"sun_type":      "weekend",
	}
	resp := do(t, srv, http.MethodPut, "/api/me/profile", "alice", profile)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestPutProfile_Update(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	first := map[string]any{
		"default_hours": 8.0,
		"mon_type":      "wfh",
		"tue_type":      "wfh",
		"wed_type":      "wfh",
		"thu_type":      "wfh",
		"fri_type":      "wfh",
		"sat_type":      "weekend",
		"sun_type":      "weekend",
	}
	do(t, srv, http.MethodPut, "/api/me/profile", "alice", first)

	second := map[string]any{
		"default_hours": 6.0,
		"mon_type":      "wfh",
		"tue_type":      "office",
		"wed_type":      "wfh",
		"thu_type":      "office",
		"fri_type":      "wfh",
		"sat_type":      "weekend",
		"sun_type":      "weekend",
	}
	resp := do(t, srv, http.MethodPut, "/api/me/profile", "alice", second)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	getResp := do(t, srv, http.MethodGet, "/api/me/profile", "alice", nil)
	var got map[string]any
	decodeJSON(t, getResp, &got)
	if got["default_hours"] != 6.0 {
		t.Errorf("default_hours: got %v, want 6.0", got["default_hours"])
	}
	if got["tue_type"] != "office" {
		t.Errorf("tue_type: got %v, want office", got["tue_type"])
	}
}

func TestGetProfile_Unauthorized(t *testing.T) {
	srv := newTestServer(t)

	resp := do(t, srv, http.MethodGet, "/api/me/profile", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestPutProfile_Unauthorized(t *testing.T) {
	srv := newTestServer(t)

	profile := map[string]any{
		"default_hours": 8.0,
		"mon_type":      "wfh",
		"tue_type":      "wfh",
		"wed_type":      "wfh",
		"thu_type":      "wfh",
		"fri_type":      "wfh",
		"sat_type":      "weekend",
		"sun_type":      "weekend",
	}
	resp := do(t, srv, http.MethodPut, "/api/me/profile", "", profile)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestPutProfile_InvalidDayType(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	profile := map[string]any{
		"default_hours": 8.0,
		"mon_type":      "nap", // invalid
		"tue_type":      "wfh",
		"wed_type":      "wfh",
		"thu_type":      "wfh",
		"fri_type":      "wfh",
		"sat_type":      "weekend",
		"sun_type":      "weekend",
	}
	resp := do(t, srv, http.MethodPut, "/api/me/profile", "alice", profile)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPutProfile_InvalidHours(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	profile := map[string]any{
		"default_hours": -1.0, // invalid
		"mon_type":      "wfh",
		"tue_type":      "wfh",
		"wed_type":      "wfh",
		"thu_type":      "wfh",
		"fri_type":      "wfh",
		"sat_type":      "weekend",
		"sun_type":      "weekend",
	}
	resp := do(t, srv, http.MethodPut, "/api/me/profile", "alice", profile)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// --- /api/users/{id}/profile endpoint tests ---

func TestGetUserProfile_Found(t *testing.T) {
	srv := newTestServer(t)
	aliceID := mustCreateUser(t, srv, "alice")

	// Create a profile for alice via /api/me/profile
	profile := map[string]any{
		"default_hours": 7.5,
		"mon_type":      "wfh",
		"tue_type":      "wfh",
		"wed_type":      "office",
		"thu_type":      "wfh",
		"fri_type":      "wfh",
		"sat_type":      "weekend",
		"sun_type":      "weekend",
	}
	do(t, srv, http.MethodPut, "/api/me/profile", "alice", profile)

	// Fetch via /api/users/{id}/profile (bob fetching alice's profile)
	mustCreateUser(t, srv, "bob")
	resp := do(t, srv, http.MethodGet, fmt.Sprintf("/api/users/%d/profile", aliceID), "bob", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got map[string]any
	decodeJSON(t, resp, &got)
	if got["default_hours"] != 7.5 {
		t.Errorf("default_hours: got %v, want 7.5", got["default_hours"])
	}
	if got["mon_type"] != "wfh" {
		t.Errorf("mon_type: got %v, want wfh", got["mon_type"])
	}
	if got["wed_type"] != "office" {
		t.Errorf("wed_type: got %v, want office", got["wed_type"])
	}
}

func TestGetUserProfile_NotFound(t *testing.T) {
	srv := newTestServer(t)
	aliceID := mustCreateUser(t, srv, "alice")

	// Alice has no profile yet
	resp := do(t, srv, http.MethodGet, fmt.Sprintf("/api/users/%d/profile", aliceID), "alice", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGetUserProfile_InvalidID(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodGet, "/api/users/abc/profile", "alice", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestGetUserProfile_NonexistentUser(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodGet, "/api/users/9999/profile", "alice", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestPutUserProfile_Create(t *testing.T) {
	srv := newTestServer(t)
	aliceID := mustCreateUser(t, srv, "alice")
	mustCreateUser(t, srv, "bob")

	profile := map[string]any{
		"default_hours": 8.0,
		"mon_type":      "wfh",
		"tue_type":      "wfh",
		"wed_type":      "wfh",
		"thu_type":      "wfh",
		"fri_type":      "wfh",
		"sat_type":      "weekend",
		"sun_type":      "weekend",
	}
	// Bob creates alice's profile
	resp := do(t, srv, http.MethodPut, fmt.Sprintf("/api/users/%d/profile", aliceID), "bob", profile)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Verify via GET
	getResp := do(t, srv, http.MethodGet, fmt.Sprintf("/api/users/%d/profile", aliceID), "bob", nil)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET status: got %d, want %d", getResp.StatusCode, http.StatusOK)
	}
	var got map[string]any
	decodeJSON(t, getResp, &got)
	if got["default_hours"] != 8.0 {
		t.Errorf("default_hours: got %v, want 8.0", got["default_hours"])
	}
}

func TestPutUserProfile_Update(t *testing.T) {
	srv := newTestServer(t)
	aliceID := mustCreateUser(t, srv, "alice")

	first := map[string]any{
		"default_hours": 8.0,
		"mon_type":      "wfh",
		"tue_type":      "wfh",
		"wed_type":      "wfh",
		"thu_type":      "wfh",
		"fri_type":      "wfh",
		"sat_type":      "weekend",
		"sun_type":      "weekend",
	}
	do(t, srv, http.MethodPut, fmt.Sprintf("/api/users/%d/profile", aliceID), "alice", first)

	second := map[string]any{
		"default_hours": 6.0,
		"mon_type":      "wfh",
		"tue_type":      "office",
		"wed_type":      "wfh",
		"thu_type":      "office",
		"fri_type":      "wfh",
		"sat_type":      "weekend",
		"sun_type":      "weekend",
	}
	resp := do(t, srv, http.MethodPut, fmt.Sprintf("/api/users/%d/profile", aliceID), "alice", second)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got map[string]any
	decodeJSON(t, resp, &got)
	if got["default_hours"] != 6.0 {
		t.Errorf("default_hours: got %v, want 6.0", got["default_hours"])
	}
	if got["tue_type"] != "office" {
		t.Errorf("tue_type: got %v, want office", got["tue_type"])
	}
}

func TestPutUserProfile_InvalidDayType(t *testing.T) {
	srv := newTestServer(t)
	aliceID := mustCreateUser(t, srv, "alice")

	profile := map[string]any{
		"default_hours": 8.0,
		"mon_type":      "nap",
		"tue_type":      "wfh",
		"wed_type":      "wfh",
		"thu_type":      "wfh",
		"fri_type":      "wfh",
		"sat_type":      "weekend",
		"sun_type":      "weekend",
	}
	resp := do(t, srv, http.MethodPut, fmt.Sprintf("/api/users/%d/profile", aliceID), "alice", profile)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPutUserProfile_InvalidHours(t *testing.T) {
	srv := newTestServer(t)
	aliceID := mustCreateUser(t, srv, "alice")

	profile := map[string]any{
		"default_hours": -1.0,
		"mon_type":      "wfh",
		"tue_type":      "wfh",
		"wed_type":      "wfh",
		"thu_type":      "wfh",
		"fri_type":      "wfh",
		"sat_type":      "weekend",
		"sun_type":      "weekend",
	}
	resp := do(t, srv, http.MethodPut, fmt.Sprintf("/api/users/%d/profile", aliceID), "alice", profile)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPutUserProfile_InvalidID(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	profile := map[string]any{
		"default_hours": 8.0,
		"mon_type":      "wfh",
		"tue_type":      "wfh",
		"wed_type":      "wfh",
		"thu_type":      "wfh",
		"fri_type":      "wfh",
		"sat_type":      "weekend",
		"sun_type":      "weekend",
	}
	resp := do(t, srv, http.MethodPut, "/api/users/abc/profile", "alice", profile)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPutUserProfile_NonexistentUser(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	profile := map[string]any{
		"default_hours": 8.0,
		"mon_type":      "wfh",
		"tue_type":      "wfh",
		"wed_type":      "wfh",
		"thu_type":      "wfh",
		"fri_type":      "wfh",
		"sat_type":      "weekend",
		"sun_type":      "weekend",
	}
	resp := do(t, srv, http.MethodPut, "/api/users/9999/profile", "alice", profile)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestPutProfile_IsolatedByUser(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")
	mustCreateUser(t, srv, "bob")

	aliceProfile := map[string]any{
		"default_hours": 7.5,
		"mon_type":      "wfh",
		"tue_type":      "wfh",
		"wed_type":      "wfh",
		"thu_type":      "wfh",
		"fri_type":      "wfh",
		"sat_type":      "weekend",
		"sun_type":      "weekend",
	}
	do(t, srv, http.MethodPut, "/api/me/profile", "alice", aliceProfile)

	// Bob has no profile
	resp := do(t, srv, http.MethodGet, "/api/me/profile", "bob", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("bob profile: status %d, want 404", resp.StatusCode)
	}
}
