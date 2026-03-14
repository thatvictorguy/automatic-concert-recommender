package discord_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func artists(names ...string) []domain.Artist {
	result := make([]domain.Artist, len(names))
	for i, n := range names {
		result[i] = domain.Artist{Name: n}
	}
	return result
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

// TestNotify_Empty_AlwaysSends verifies that even with no concerts a message is
// still sent listing the artists in the section.
func TestNotify_Empty_AlwaysSends(t *testing.T) {
	srv, payload := capturePayload(t)
	defer srv.Close()

	n := discord.New(srv.URL)
	if err := n.Notify("Past Month Top Artists", artists("Cornelius", "Fishmans"), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := payload()["content"].(string)
	if content == "" {
		t.Fatal("expected a content message even with no concerts")
	}
	if !strings.Contains(content, "Cornelius") || !strings.Contains(content, "Fishmans") {
		t.Errorf("expected artist names in content, got: %q", content)
	}
}

// TestNotify_Empty_NoEmbeds verifies that the no-concerts message has no embeds.
func TestNotify_Empty_NoEmbeds(t *testing.T) {
	srv, payload := capturePayload(t)
	defer srv.Close()

	n := discord.New(srv.URL)
	_ = n.Notify("Past Month Top Artists", artists("Ado"), nil)

	embeds, _ := payload()["embeds"].([]any)
	if len(embeds) != 0 {
		t.Errorf("expected 0 embeds for no-concert message, got %d", len(embeds))
	}
}

// TestNotify_Empty_NilArtists_NoHTTPCall verifies that if there are truly
// nothing to report (no artists, no concerts) no message is sent.
func TestNotify_Empty_NilArtists_NoHTTPCall(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	n := discord.New(srv.URL)
	if err := n.Notify("All-Time Top Artists", nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("expected no HTTP call when both artists and concerts are nil")
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
	if err := n.Notify("All-Time Top Artists", artists("Band"), []domain.Concert{concert("Band", "Zepp Tokyo", "Tokyo", "")}); err != nil {
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
	if err := n.Notify("All-Time Top Artists", artists("YOASOBI"), []domain.Concert{concert("YOASOBI", "Zepp Tokyo", "Tokyo", "")}); err != nil {
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

func TestNotify_SectionInContent(t *testing.T) {
	srv, payload := capturePayload(t)
	defer srv.Close()

	n := discord.New(srv.URL)
	if err := n.Notify("Past Month Top Artists", artists("Band"), []domain.Concert{concert("Band", "Venue", "Tokyo", "")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := payload()["content"].(string)
	if !strings.Contains(content, "Past Month Top Artists") {
		t.Errorf("expected section label in content, got: %q", content)
	}
}

func TestNotify_EmbedURL_WhenTicketURLPresent(t *testing.T) {
	srv, payload := capturePayload(t)
	defer srv.Close()

	n := discord.New(srv.URL)
	c := concert("YOASOBI", "Zepp Tokyo", "Tokyo", "https://eplus.jp/ticket/123")
	if err := n.Notify("All-Time Top Artists", artists("YOASOBI"), []domain.Concert{c}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	embeds, _ := payload()["embeds"].([]any)
	url, _ := embeds[0].(map[string]any)["url"].(string)
	if url != "https://eplus.jp/ticket/123" {
		t.Errorf("embed url = %q, want ticket URL", url)
	}
}

// TestNotify_EmbedURL_FallbackSearchWhenNoTicket verifies that a ticket search URL
// is generated for concerts that have no TicketURL.
func TestNotify_EmbedURL_FallbackSearchWhenNoTicket(t *testing.T) {
	srv, payload := capturePayload(t)
	defer srv.Close()

	n := discord.New(srv.URL)
	if err := n.Notify("All-Time Top Artists", artists("YOASOBI"), []domain.Concert{concert("YOASOBI", "Zepp Tokyo", "Tokyo", "")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	embeds, _ := payload()["embeds"].([]any)
	url, _ := embeds[0].(map[string]any)["url"].(string)
	if url == "" {
		t.Error("expected a fallback ticket search URL, got empty string")
	}
	if !strings.Contains(url, "YOASOBI") {
		t.Errorf("expected artist name in fallback URL, got: %q", url)
	}
}

func TestNotify_EmbedColor(t *testing.T) {
	srv, payload := capturePayload(t)
	defer srv.Close()

	n := discord.New(srv.URL)
	if err := n.Notify("All-Time Top Artists", artists("Band"), []domain.Concert{concert("Band", "Venue", "Tokyo", "")}); err != nil {
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
	err := n.Notify("All-Time Top Artists", artists("Band"), []domain.Concert{concert("Band", "Venue", "Tokyo", "")})
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
	err := n.Notify("All-Time Top Artists", artists("Band"), []domain.Concert{concert("Band", "Venue", "Tokyo", "")})
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
	if err := n.Notify("All-Time Top Artists", artists("Band"), concerts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestCount != 2 {
		t.Errorf("expected 2 requests for 11 concerts, got %d", requestCount)
	}
}
