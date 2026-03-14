package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thatvictorguy/automatic-concert-recommender/domain"
)

// SpotifyGreen is exported so tests can assert the correct embed colour.
const SpotifyGreen = 0x1DB954 // #1DB954

const maxEmbedsPerMessage = 10

// Notifier sends concert recommendations to a Discord channel via a webhook.
type Notifier struct {
	WebhookURL string
	HTTP       *http.Client
}

// New returns a Notifier ready for production use.
func New(webhookURL string) *Notifier {
	return &Notifier{
		WebhookURL: webhookURL,
		HTTP:       &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify posts concert recommendations to Discord under the given section label.
// When concerts is empty but artists is non-empty, a "no upcoming concerts" message
// listing the artists is sent instead. No HTTP call is made only when both are nil/empty.
// Discord limits 10 embeds per message, so large lists are split across multiple requests.
func (n *Notifier) Notify(section string, artists []domain.Artist, concerts []domain.Concert) error {
	if len(concerts) == 0 {
		if len(artists) == 0 {
			return nil
		}
		return n.postNoResults(section, artists)
	}
	for i := 0; i < len(concerts); i += maxEmbedsPerMessage {
		end := i + maxEmbedsPerMessage
		if end > len(concerts) {
			end = len(concerts)
		}
		chunk := concerts[i:end]
		isFirst := i == 0
		if err := n.post(section, chunk, len(concerts), isFirst); err != nil {
			return err
		}
	}
	return nil
}

func (n *Notifier) postNoResults(section string, artists []domain.Artist) error {
	names := make([]string, len(artists))
	for i, a := range artists {
		names[i] = "• " + a.Name
	}
	content := fmt.Sprintf("🎵 **%s** — No upcoming concerts in the next 3 weeks\n%s",
		section, strings.Join(names, "\n"))

	p := webhookPayload{Content: content, Embeds: []embed{}}
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("discord: marshal payload: %w", err)
	}
	return n.send(body)
}

func (n *Notifier) post(section string, concerts []domain.Concert, total int, includeHeader bool) error {
	p := buildPayload(section, concerts, total, includeHeader)
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("discord: marshal payload: %w", err)
	}
	return n.send(body)
}

func (n *Notifier) send(body []byte) error {
	resp, err := n.HTTP.Post(n.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discord: post webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord: webhook returned status %d", resp.StatusCode)
	}
	return nil
}

type webhookPayload struct {
	Content string  `json:"content,omitempty"`
	Embeds  []embed `json:"embeds"`
}

type embed struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
	Color       int    `json:"color"`
}

func buildPayload(section string, concerts []domain.Concert, total int, includeHeader bool) webhookPayload {
	embeds := make([]embed, len(concerts))
	for i, c := range concerts {
		e := embed{
			Title:       fmt.Sprintf("%s @ %s", c.Artist.Name, c.Venue),
			Description: fmt.Sprintf("📍 %s\n🗓 %s", c.City, c.Date.Format("Mon Jan 2, 2006 · 15:04")),
			Color:       SpotifyGreen,
			URL:         resolveTicketURL(c),
		}
		embeds[i] = e
	}

	p := webhookPayload{Embeds: embeds}
	if includeHeader {
		p.Content = fmt.Sprintf("🎵 **%s** — %d concert(s) found", section, total)
	}
	return p
}

// resolveTicketURL returns the concert's own TicketURL if set, otherwise a
// fallback eplus.jp search URL for the artist so the user can find tickets.
func resolveTicketURL(c domain.Concert) string {
	if c.TicketURL != "" {
		return c.TicketURL
	}
	return "https://eplus.jp/sf/search?keyword=" + url.QueryEscape(c.Artist.Name)
}
