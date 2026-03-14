package setlistfm_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thatvictorguy/automatic-concert-recommender/domain"
	"github.com/thatvictorguy/automatic-concert-recommender/infrastructure/setlistfm"
)

// --- helpers ---

func futureDate() string { return time.Now().Add(48 * time.Hour).Format("02-01-2006") }
func pastDate() string   { return time.Now().Add(-48 * time.Hour).Format("02-01-2006") }

func slResponse(setlists []map[string]any) []byte {
	b, _ := json.Marshal(map[string]any{
		"type":         "setlists",
		"itemsPerPage": 20,
		"page":         1,
		"total":        len(setlists),
		"setlist":      setlists,
	})
	return b
}

func slEvent(id, artist, date, venue, city string) map[string]any {
	return map[string]any{
		"id":        id,
		"eventDate": date,
		"artist":    map[string]any{"name": artist},
		"venue": map[string]any{
			"name": venue,
			"city": map[string]any{
				"name":    city,
				"country": map[string]any{"code": "JP", "name": "Japan"},
			},
		},
		"url": "https://www.setlist.fm/setlist/test/2026/venue-" + id + ".html",
	}
}

func newTestClient(t *testing.T, handler http.HandlerFunc) (*setlistfm.Client, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c := setlistfm.New("test-api-key")
	c.BaseURL = srv.URL
	c.RateLimitDelay = 0
	c.RetryDelay = 0
	return c, srv.Close
}

// --- tests ---

func TestFindConcerts_ReturnsUpcomingEvents(t *testing.T) {
	c, close := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(slResponse([]map[string]any{
			slEvent("abc1", "YOASOBI", futureDate(), "Zepp Shinjuku", "Tokyo"),
		}))
	})
	defer close()

	concerts, err := c.FindConcerts([]domain.Artist{{Name: "YOASOBI"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(concerts) != 1 {
		t.Fatalf("expected 1 concert, got %d", len(concerts))
	}
	if concerts[0].Venue != "Zepp Shinjuku" {
		t.Errorf("Venue = %q, want %q", concerts[0].Venue, "Zepp Shinjuku")
	}
	if concerts[0].City != "Tokyo" {
		t.Errorf("City = %q, want %q", concerts[0].City, "Tokyo")
	}
	if concerts[0].Artist.Name != "YOASOBI" {
		t.Errorf("Artist = %q, want %q", concerts[0].Artist.Name, "YOASOBI")
	}
}

func TestFindConcerts_FiltersPastEvents(t *testing.T) {
	c, close := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(slResponse([]map[string]any{
			slEvent("past1", "YOASOBI", pastDate(), "Budokan", "Tokyo"),
			slEvent("fut1", "YOASOBI", futureDate(), "Zepp Shinjuku", "Tokyo"),
		}))
	})
	defer close()

	concerts, err := c.FindConcerts([]domain.Artist{{Name: "YOASOBI"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(concerts) != 1 {
		t.Fatalf("expected 1 concert (future only), got %d", len(concerts))
	}
	if concerts[0].ID != "fut1" {
		t.Errorf("expected future event, got ID %q", concerts[0].ID)
	}
}

func TestFindConcerts_EmptyResponse(t *testing.T) {
	c, close := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(slResponse([]map[string]any{}))
	})
	defer close()

	concerts, err := c.FindConcerts([]domain.Artist{{Name: "Unknown Artist"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(concerts) != 0 {
		t.Errorf("expected 0 concerts, got %d", len(concerts))
	}
}

func TestFindConcerts_404TreatedAsNoResults(t *testing.T) {
	// Setlist.fm returns 404 when an artist is not found — treat as empty, not error.
	c, close := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer close()

	concerts, err := c.FindConcerts([]domain.Artist{{Name: "Nonexistent Artist"}})
	if err != nil {
		t.Fatalf("expected no error for 404, got: %v", err)
	}
	if len(concerts) != 0 {
		t.Errorf("expected 0 concerts for 404, got %d", len(concerts))
	}
}

func TestFindConcerts_SetsRequiredHeaders(t *testing.T) {
	var gotAPIKey, gotAccept string
	c, close := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("x-api-key")
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		w.Write(slResponse([]map[string]any{}))
	})
	defer close()

	c.FindConcerts([]domain.Artist{{Name: "Test"}})

	if gotAPIKey != "test-api-key" {
		t.Errorf("x-api-key = %q, want %q", gotAPIKey, "test-api-key")
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want %q", gotAccept, "application/json")
	}
}

func TestFindConcerts_SearchesJapanOnly(t *testing.T) {
	var gotQuery string
	c, close := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write(slResponse([]map[string]any{}))
	})
	defer close()

	c.FindConcerts([]domain.Artist{{Name: "Test Artist"}})

	if gotQuery == "" {
		t.Fatal("expected query string, got empty")
	}
	if r := r(gotQuery, "countryCode=JP"); !r {
		t.Errorf("query %q does not contain countryCode=JP", gotQuery)
	}
}

func TestFindConcerts_HTTPError(t *testing.T) {
	c := setlistfm.New("test-key")
	c.BaseURL = "http://127.0.0.1:1" // nothing listening
	c.RateLimitDelay = 0

	_, err := c.FindConcerts([]domain.Artist{{Name: "Test"}})
	if err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}
}

func TestFindConcerts_Non200Status(t *testing.T) {
	// 500 internal server error should propagate as an error.
	c, close := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer close()

	_, err := c.FindConcerts([]domain.Artist{{Name: "Test"}})
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}

func TestFindConcerts_429RetriesAndSucceeds(t *testing.T) {
	// First request gets 429, retry succeeds with results.
	callCount := 0
	c, close := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(slResponse([]map[string]any{
			slEvent("r1", "Test", futureDate(), "Zepp Tokyo", "Tokyo"),
		}))
	})
	defer close()

	concerts, err := c.FindConcerts([]domain.Artist{{Name: "Test"}})
	if err != nil {
		t.Fatalf("unexpected error after 429 retry: %v", err)
	}
	if len(concerts) != 1 {
		t.Errorf("expected 1 concert after retry, got %d", len(concerts))
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls (429 + retry), got %d", callCount)
	}
}

func TestFindConcerts_429TwiceSkipsArtist(t *testing.T) {
	// Both attempts return 429 — artist is skipped, no error returned.
	c, close := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	defer close()

	concerts, err := c.FindConcerts([]domain.Artist{{Name: "Test"}})
	if err != nil {
		t.Fatalf("expected no error when rate limited twice, got: %v", err)
	}
	if len(concerts) != 0 {
		t.Errorf("expected 0 concerts when skipped, got %d", len(concerts))
	}
}

func TestFindConcerts_MultipleArtists(t *testing.T) {
	requestCount := 0
	c, close := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write(slResponse([]map[string]any{}))
	})
	defer close()

	c.FindConcerts([]domain.Artist{{Name: "Artist A"}, {Name: "Artist B"}, {Name: "Artist C"}})

	if requestCount != 3 {
		t.Errorf("expected 3 requests for 3 artists, got %d", requestCount)
	}
}

func TestFindConcerts_ParsesDateCorrectly(t *testing.T) {
	c, close := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Fixed future date in DD-MM-YYYY format
		w.Write(slResponse([]map[string]any{
			slEvent("d1", "Test", "15-08-2026", "Budokan", "Tokyo"),
		}))
	})
	defer close()

	concerts, err := c.FindConcerts([]domain.Artist{{Name: "Test"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(concerts) != 1 {
		t.Fatalf("expected 1 concert, got %d", len(concerts))
	}
	got := concerts[0].Date
	if got.Year() != 2026 || got.Month() != 8 || got.Day() != 15 {
		t.Errorf("date = %v, want 2026-08-15", got)
	}
}

func TestFindConcerts_TicketURLIsSetlistPage(t *testing.T) {
	c, close := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(slResponse([]map[string]any{
			slEvent("x1", "Test", futureDate(), "Venue", "Tokyo"),
		}))
	})
	defer close()

	concerts, err := c.FindConcerts([]domain.Artist{{Name: "Test"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://www.setlist.fm/setlist/test/2026/venue-x1.html"
	if concerts[0].TicketURL != want {
		t.Errorf("TicketURL = %q, want %q", concerts[0].TicketURL, want)
	}
}

// r is a tiny helper to check if a string contains a substring.
func r(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
