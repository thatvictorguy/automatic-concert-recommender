package bandsintown

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/thatvictorguy/automatic-concert-recommender/domain"
)

const defaultBaseURL = "https://rest.bandsintown.com"

// Client implements domain.ConcertFinder using the Bandsintown API.
// Register a free app_id at https://www.bandsintown.com/partner/signup.
type Client struct {
	AppID   string
	BaseURL string
	HTTP    *http.Client
}

// New returns a Client ready for production use.
func New(appID string) *Client {
	return &Client{
		AppID:   appID,
		BaseURL: defaultBaseURL,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

// --- Bandsintown API response types ---

type biEvent struct {
	ID       string   `json:"id"`
	DateTime string   `json:"datetime"`
	Venue    biVenue  `json:"venue"`
	Offers   []biOffer `json:"offers"`
}

type biVenue struct {
	Name    string `json:"name"`
	City    string `json:"city"`
	Country string `json:"country"`
}

type biOffer struct {
	Type   string `json:"type"`
	URL    string `json:"url"`
	Status string `json:"status"`
}

// FindConcerts fetches upcoming Japan events for each artist from the Bandsintown API.
func (c *Client) FindConcerts(artists []domain.Artist) ([]domain.Concert, error) {
	var all []domain.Concert
	for _, artist := range artists {
		concerts, err := c.fetchEvents(artist)
		if err != nil {
			return nil, fmt.Errorf("bandsintown: fetch events for %q: %w", artist.Name, err)
		}
		all = append(all, concerts...)
	}
	return all, nil
}

func (c *Client) fetchEvents(artist domain.Artist) ([]domain.Concert, error) {
	endpoint := fmt.Sprintf(
		"%s/artists/%s/events?app_id=%s",
		c.BaseURL,
		url.PathEscape(artist.Name),
		url.QueryEscape(c.AppID),
	)

	resp, err := c.HTTP.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bandsintown: unexpected status %d for artist %q", resp.StatusCode, artist.Name)
	}

	var events []biEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil, fmt.Errorf("bandsintown: decode response: %w", err)
	}

	return toJapanConcerts(artist, events), nil
}

// toJapanConcerts filters events to Japan only and maps them to domain.Concert.
func toJapanConcerts(artist domain.Artist, events []biEvent) []domain.Concert {
	var concerts []domain.Concert
	for _, e := range events {
		if e.Venue.Country != "Japan" {
			continue
		}
		date, err := time.Parse("2006-01-02T15:04:05", e.DateTime)
		if err != nil {
			// Skip events with unparseable dates rather than failing the whole batch
			continue
		}
		concerts = append(concerts, domain.Concert{
			ID:        e.ID,
			Artist:    artist,
			Venue:     e.Venue.Name,
			City:      e.Venue.City,
			Date:      date,
			TicketURL: firstTicketURL(e.Offers),
		})
	}
	return concerts
}

// firstTicketURL returns the URL of the first offer of type "Tickets", or empty string.
func firstTicketURL(offers []biOffer) string {
	for _, o := range offers {
		if o.Type == "Tickets" {
			return o.URL
		}
	}
	return ""
}
