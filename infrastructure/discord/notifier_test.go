package discord_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thatvictorguy/automatic-concert-recommender/domain"
	"github.com/thatvictorguy/automatic-concert-recommender/infrastructure/discord"
)

func concert(artist, venue, city, ticketURL string) domain.Concert {
	return domain.Concert{
		Artist:    domain.Artist{Name: artist},
		Venue:     venue,
		City:      city,
		Date:      time.Date(2026, 4, 3, 19, 0, 0, 0, time.UTC),
		TicketURL: ticketURL,
	}
}

// capturePayload starts a test server that captures the first request body.
func capturePayload(t *testing.T) (*httptest.Server, func() map[string]any) {
	t.Helper()
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	return srv, func() map[string]any { return captured }
}

func TestNotify_Empty(t *testing.T) {
	// No concerts → no HTTP call should be made.
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	n := discord.New(srv.URL)
	if err := n.Notify(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("expected no HTTP call for empty concert list")
	}
}

func TestNotify_SendsCorrectContentType(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	n := discord.New(srv.URL)
	if err := n.Notify([]domain.Concert{concert("Band", "Zepp Tokyo", "Tokyo", "")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotContentType, "application/json")
	}
}

func TestNotify_EmbedTitle(t *testing.T) {
	srv, payload := capturePayload(t)
	defer srv.Close()

	n := discord.New(srv.URL)
	if err := n.Notify([]domain.Concert{concert("YOASOBI", "Zepp Tokyo", "Tokyo", "")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	embeds, _ := payload()["embeds"].([]any)
	if len(embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(embeds))
	}
	title, _ := embeds[0].(map[string]any)["title"].(string)
	if title != "YOASOBI @ Zepp Tokyo" {
		t.Errorf("embed title = %q, want %q", title, "YOASOBI @ Zepp Tokyo")
	}
}

func TestNotify_EmbedURL_WhenTicketURLPresent(t *testing.T) {
	srv, payload := capturePayload(t)
	defer srv.Close()

	n := discord.New(srv.URL)
	c := concert("YOASOBI", "Zepp Tokyo", "Tokyo", "https://eplus.jp/ticket/123")
	if err := n.Notify([]domain.Concert{c}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	embeds, _ := payload()["embeds"].([]any)
	url, _ := embeds[0].(map[string]any)["url"].(string)
	if url != "https://eplus.jp/ticket/123" {
		t.Errorf("embed url = %q, want ticket URL", url)
	}
}

func TestNotify_EmbedURL_AbsentWhenNoTicket(t *testing.T) {
	srv, payload := capturePayload(t)
	defer srv.Close()

	n := discord.New(srv.URL)
	if err := n.Notify([]domain.Concert{concert("Band", "Venue", "Tokyo", "")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	embeds, _ := payload()["embeds"].([]any)
	_, hasURL := embeds[0].(map[string]any)["url"]
	if hasURL {
		t.Error("expected no url field when TicketURL is empty")
	}
}

func TestNotify_EmbedColor(t *testing.T) {
	srv, payload := capturePayload(t)
	defer srv.Close()

	n := discord.New(srv.URL)
	if err := n.Notify([]domain.Concert{concert("Band", "Venue", "Tokyo", "")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	embeds, _ := payload()["embeds"].([]any)
	color, _ := embeds[0].(map[string]any)["color"].(float64)
	if int(color) != discord.SpotifyGreen {
		t.Errorf("embed color = %d, want %d (Spotify green)", int(color), discord.SpotifyGreen)
	}
}

func TestNotify_HTTPError(t *testing.T) {
	n := discord.New("http://127.0.0.1:1") // nothing listening on port 1
	err := n.Notify([]domain.Concert{concert("Band", "Venue", "Tokyo", "")})
	if err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}
}

func TestNotify_Non2xxStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	n := discord.New(srv.URL)
	err := n.Notify([]domain.Concert{concert("Band", "Venue", "Tokyo", "")})
	if err == nil {
		t.Fatal("expected error for non-2xx status, got nil")
	}
}

func TestNotify_ChunksOver10Concerts(t *testing.T) {
	// Discord allows max 10 embeds per message. 11 concerts should send 2 requests.
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	concerts := make([]domain.Concert, 11)
	for i := range concerts {
		concerts[i] = concert("Band", "Venue", "Tokyo", "")
	}

	n := discord.New(srv.URL)
	if err := n.Notify(concerts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestCount != 2 {
		t.Errorf("expected 2 requests for 11 concerts, got %d", requestCount)
	}
}
