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
		// Verify the Authorization header is set correctly
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

	// Wrong token — should get an error
	badClient := &spotify.Client{
		AccessToken: "wrong-token",
		BaseURL:     srv.URL,
		HTTP:        &http.Client{Timeout: 5 * time.Second},
	}
	if _, err := badClient.TopArtists(); err == nil {
		t.Fatal("expected error for wrong token, got nil")
	}

	// Correct token — should succeed
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

// TestTopArtists_NonOKStatus verifies that a non-200 response (e.g. 401 expired
// token, 429 rate limit) is surfaced as an error.
func TestTopArtists_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	// Bypass auth check in this test by hitting a raw handler
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
