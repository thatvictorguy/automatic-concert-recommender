package spotify_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thatvictorguy/automatic-concert-recommender/infrastructure/spotify"
)

// testServer spins up a mock Spotify API returning the provided response body.
func testServer(t *testing.T, status int, body any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("testServer: encode: %v", err)
		}
	}))
}

// capturingServer spins up a mock that captures the incoming request for inspection.
func capturingServer(t *testing.T, body any) (*httptest.Server, func() *http.Request) {
	t.Helper()
	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("capturingServer: encode: %v", err)
		}
	}))
	return srv, func() *http.Request { return captured }
}

func newTestClient(baseURL string) *spotify.Client {
	return &spotify.Client{
		AccessToken: "test-token",
		BaseURL:     baseURL,
		HTTP:        &http.Client{Timeout: 5 * time.Second},
	}
}

// TestTopArtists_ReturnsArtists verifies the happy path — artists are correctly
// parsed from the Spotify response including ID, name, and genres.
func TestTopArtists_ReturnsArtists(t *testing.T) {
	body := map[string]any{
		"items": []map[string]any{
			{"id": "1", "name": "Cornelius", "genres": []string{"shibuya-kei", "experimental"}},
			{"id": "2", "name": "Fishmans", "genres": []string{"dub", "psychedelic rock"}},
		},
	}

	srv := testServer(t, http.StatusOK, body)
	defer srv.Close()

	artists, err := newTestClient(srv.URL).TopArtists()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(artists) != 2 {
		t.Fatalf("expected 2 artists, got %d", len(artists))
	}
	if artists[0].ID != "1" || artists[0].Name != "Cornelius" {
		t.Errorf("unexpected first artist: %+v", artists[0])
	}
	if len(artists[0].Genres) != 2 || artists[0].Genres[0] != "shibuya-kei" {
		t.Errorf("unexpected genres for first artist: %v", artists[0].Genres)
	}
	if artists[1].Name != "Fishmans" {
		t.Errorf("expected second artist Fishmans, got %q", artists[1].Name)
	}
}

// TestTopArtists_UsesLongTermTimeRange verifies TopArtists sends long_term and limit=5.
func TestTopArtists_UsesLongTermTimeRange(t *testing.T) {
	body := map[string]any{"items": []any{}}
	srv, getReq := capturingServer(t, body)
	defer srv.Close()

	if _, err := newTestClient(srv.URL).TopArtists(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	q := getReq().URL.Query()
	if q.Get("time_range") != "long_term" {
		t.Errorf("time_range = %q, want %q", q.Get("time_range"), "long_term")
	}
	if q.Get("limit") != "5" {
		t.Errorf("limit = %q, want %q", q.Get("limit"), "5")
	}
}

// TestTopArtists_SendsBearerToken verifies that a missing/wrong token results
// in a 401 being surfaced as an error (tested via our mock server's auth check).
func TestTopArtists_SendsBearerToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer correct-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
	}))
	defer srv.Close()

	badClient := &spotify.Client{
		AccessToken: "wrong-token",
		BaseURL:     srv.URL,
		HTTP:        &http.Client{Timeout: 5 * time.Second},
	}
	if _, err := badClient.TopArtists(); err == nil {
		t.Fatal("expected error for wrong token, got nil")
	}

	goodClient := &spotify.Client{
		AccessToken: "correct-token",
		BaseURL:     srv.URL,
		HTTP:        &http.Client{Timeout: 5 * time.Second},
	}
	if _, err := goodClient.TopArtists(); err != nil {
		t.Fatalf("unexpected error for correct token: %v", err)
	}
}

// TestTopArtists_EmptyItems verifies that a valid response with no artists
// returns an empty slice without error.
func TestTopArtists_EmptyItems(t *testing.T) {
	body := map[string]any{"items": []any{}}

	srv := testServer(t, http.StatusOK, body)
	defer srv.Close()

	artists, err := newTestClient(srv.URL).TopArtists()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(artists) != 0 {
		t.Errorf("expected 0 artists, got %d", len(artists))
	}
}

// TestTopArtists_NonOKStatus verifies that a non-200 response is surfaced as an error.
func TestTopArtists_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client := &spotify.Client{
		AccessToken: "any",
		BaseURL:     srv.URL,
		HTTP:        &http.Client{Timeout: 5 * time.Second},
	}
	if _, err := client.TopArtists(); err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
}

// TestTopArtists_HTTPError verifies that a network-level failure is surfaced as an error.
func TestTopArtists_HTTPError(t *testing.T) {
	client := newTestClient("http://localhost:0") // nothing listening
	if _, err := client.TopArtists(); err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}
}

// TestRecentTopArtists_ReturnsArtists verifies the happy path for recent artists.
func TestRecentTopArtists_ReturnsArtists(t *testing.T) {
	body := map[string]any{
		"items": []map[string]any{
			{"id": "3", "name": "Ado", "genres": []string{"j-pop"}},
		},
	}

	srv := testServer(t, http.StatusOK, body)
	defer srv.Close()

	artists, err := newTestClient(srv.URL).RecentTopArtists()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(artists) != 1 || artists[0].Name != "Ado" {
		t.Errorf("unexpected artists: %+v", artists)
	}
}

// TestRecentTopArtists_UsesShortTermTimeRange verifies RecentTopArtists sends short_term and limit=5.
func TestRecentTopArtists_UsesShortTermTimeRange(t *testing.T) {
	body := map[string]any{"items": []any{}}
	srv, getReq := capturingServer(t, body)
	defer srv.Close()

	if _, err := newTestClient(srv.URL).RecentTopArtists(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	q := getReq().URL.Query()
	if q.Get("time_range") != "short_term" {
		t.Errorf("time_range = %q, want %q", q.Get("time_range"), "short_term")
	}
	if q.Get("limit") != "5" {
		t.Errorf("limit = %q, want %q", q.Get("limit"), "5")
	}
}

// TestRecentTopArtists_HTTPError verifies network failures are surfaced as errors.
func TestRecentTopArtists_HTTPError(t *testing.T) {
	client := newTestClient("http://localhost:0")
	if _, err := client.RecentTopArtists(); err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}
}
