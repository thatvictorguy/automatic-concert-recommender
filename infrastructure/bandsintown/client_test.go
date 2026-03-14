package bandsintown_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thatvictorguy/automatic-concert-recommender/domain"
	"github.com/thatvictorguy/automatic-concert-recommender/infrastructure/bandsintown"
)

// testServer spins up a mock Bandsintown API that returns the provided events
// for every artist request.
func testServer(t *testing.T, events []map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(events); err != nil {
			t.Errorf("testServer: encode: %v", err)
		}
	}))
}

func newTestClient(baseURL string) *bandsintown.Client {
	return &bandsintown.Client{
		AppID:   "test",
		BaseURL: baseURL,
		HTTP:    &http.Client{Timeout: 5 * time.Second},
	}
}

// TestFindConcerts_FiltersJapan verifies that only Japan events are returned
// when the API returns a mix of countries.
func TestFindConcerts_FiltersJapan(t *testing.T) {
	events := []map[string]any{
		{
			"id":       "1",
			"datetime": "2026-04-01T19:00:00",
			"url":      "https://bandsintown.com/e/1",
			"venue": map[string]any{
				"name":    "Budokan",
				"city":    "Tokyo",
				"country": "Japan",
			},
			"offers": []map[string]any{
				{"type": "Tickets", "url": "https://example.com/tickets/1", "status": "available"},
			},
		},
		{
			"id":       "2",
			"datetime": "2026-04-05T20:00:00",
			"url":      "https://bandsintown.com/e/2",
			"venue": map[string]any{
				"name":    "Madison Square Garden",
				"city":    "New York",
				"country": "United States",
			},
			"offers": []map[string]any{},
		},
	}

	srv := testServer(t, events)
	defer srv.Close()

	concerts, err := newTestClient(srv.URL).FindConcerts([]domain.Artist{{ID: "1", Name: "Radiohead"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(concerts) != 1 {
		t.Fatalf("expected 1 concert (Japan only), got %d", len(concerts))
	}
	if concerts[0].Venue != "Budokan" {
		t.Errorf("expected venue Budokan, got %q", concerts[0].Venue)
	}
	if concerts[0].City != "Tokyo" {
		t.Errorf("expected city Tokyo, got %q", concerts[0].City)
	}
}

// TestFindConcerts_ParsesDateAndTicketURL verifies date parsing and ticket URL extraction.
func TestFindConcerts_ParsesDateAndTicketURL(t *testing.T) {
	events := []map[string]any{
		{
			"id":       "1",
			"datetime": "2026-04-01T19:00:00",
			"url":      "https://bandsintown.com/e/1",
			"venue": map[string]any{
				"name":    "Zepp Tokyo",
				"city":    "Tokyo",
				"country": "Japan",
			},
			"offers": []map[string]any{
				{"type": "Tickets", "url": "https://eplus.jp/ticket/1", "status": "available"},
			},
		},
	}

	srv := testServer(t, events)
	defer srv.Close()

	concerts, err := newTestClient(srv.URL).FindConcerts([]domain.Artist{{ID: "1", Name: "Radiohead"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(concerts) == 0 {
		t.Fatal("expected 1 concert, got 0")
	}

	want := time.Date(2026, 4, 1, 19, 0, 0, 0, time.UTC)
	if !concerts[0].Date.Equal(want) {
		t.Errorf("expected date %v, got %v", want, concerts[0].Date)
	}
	if concerts[0].TicketURL != "https://eplus.jp/ticket/1" {
		t.Errorf("expected ticket URL %q, got %q", "https://eplus.jp/ticket/1", concerts[0].TicketURL)
	}
}

// TestFindConcerts_MultipleArtists verifies that events are fetched for each artist.
func TestFindConcerts_MultipleArtists(t *testing.T) {
	events := []map[string]any{
		{
			"id":       "1",
			"datetime": "2026-05-10T18:00:00",
			"url":      "https://bandsintown.com/e/1",
			"venue": map[string]any{
				"name":    "Liquid Room",
				"city":    "Tokyo",
				"country": "Japan",
			},
			"offers": []map[string]any{},
		},
	}

	srv := testServer(t, events)
	defer srv.Close()

	artists := []domain.Artist{
		{ID: "1", Name: "Cornelius"},
		{ID: "2", Name: "Fishmans"},
	}

	concerts, err := newTestClient(srv.URL).FindConcerts(artists)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// One event per artist from our mock server
	if len(concerts) != 2 {
		t.Fatalf("expected 2 concerts (one per artist), got %d", len(concerts))
	}
	if concerts[0].Artist.Name != "Cornelius" {
		t.Errorf("expected first artist Cornelius, got %q", concerts[0].Artist.Name)
	}
	if concerts[1].Artist.Name != "Fishmans" {
		t.Errorf("expected second artist Fishmans, got %q", concerts[1].Artist.Name)
	}
}

// TestFindConcerts_EmptyResponse verifies that an empty event list is handled gracefully.
func TestFindConcerts_EmptyResponse(t *testing.T) {
	srv := testServer(t, []map[string]any{})
	defer srv.Close()

	concerts, err := newTestClient(srv.URL).FindConcerts([]domain.Artist{{ID: "1", Name: "Unknown Artist"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(concerts) != 0 {
		t.Errorf("expected 0 concerts, got %d", len(concerts))
	}
}

// TestFindConcerts_HTTPError verifies that a network error is surfaced as an error.
func TestFindConcerts_HTTPError(t *testing.T) {
	client := newTestClient("http://localhost:0") // nothing listening
	_, err := client.FindConcerts([]domain.Artist{{ID: "1", Name: "Radiohead"}})
	if err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}
}

// TestFindConcerts_NonOKStatus verifies that a non-200 response is returned as an error.
func TestFindConcerts_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).FindConcerts([]domain.Artist{{ID: "1", Name: "Radiohead"}})
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
}
