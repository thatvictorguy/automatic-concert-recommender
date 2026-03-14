package setlistfm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/thatvictorguy/automatic-concert-recommender/domain"
)

const (
	defaultBaseURL = "https://api.setlist.fm/rest/1.0"
	// Setlist.fm uses DD-MM-YYYY date format.
	dateLayout = "02-01-2006"
)

// Client implements domain.ConcertFinder using the Setlist.fm API.
// Get a free API key at https://www.setlist.fm/settings/api
type Client struct {
	APIKey         string
	BaseURL        string
	HTTP           *http.Client
	// RateLimitDelay is the pause between per-artist requests.
	// Defaults to 2s to stay within the free-tier rate limit. Set to 0 in tests.
	RateLimitDelay time.Duration
	// RetryDelay is how long to wait after a 429 before retrying.
	// Defaults to 5s. Set to 0 in tests.
	RetryDelay time.Duration
}

// New returns a Client ready for production use.
func New(apiKey string) *Client {
	return &Client{
		APIKey:         apiKey,
		BaseURL:        defaultBaseURL,
		HTTP:           &http.Client{Timeout: 10 * time.Second},
		RateLimitDelay: 2 * time.Second,
		RetryDelay:     5 * time.Second,
	}
}

// --- Setlist.fm API response types ---

type searchResponse struct {
	Setlist []slSetlist `json:"setlist"`
}

type slSetlist struct {
	ID        string   `json:"id"`
	EventDate string   `json:"eventDate"`
	Artist    slArtist `json:"artist"`
	Venue     slVenue  `json:"venue"`
	URL       string   `json:"url"`
}

type slArtist struct {
	Name string `json:"name"`
}

type slVenue struct {
	Name string `json:"name"`
	City slCity `json:"city"`
}

type slCity struct {
	Name    string    `json:"name"`
	Country slCountry `json:"country"`
}

type slCountry struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// FindConcerts searches Setlist.fm for upcoming Japan events for each artist.
func (c *Client) FindConcerts(artists []domain.Artist) ([]domain.Concert, error) {
	var all []domain.Concert
	for i, artist := range artists {
		if i > 0 && c.RateLimitDelay > 0 {
			time.Sleep(c.RateLimitDelay)
		}
		concerts, err := c.searchSetlists(artist)
		if err != nil {
			return nil, fmt.Errorf("setlistfm: search for %q: %w", artist.Name, err)
		}
		all = append(all, concerts...)
	}
	return all, nil
}

func (c *Client) searchSetlists(artist domain.Artist) ([]domain.Concert, error) {
	endpoint := fmt.Sprintf(
		"%s/search/setlists?artistName=%s&countryCode=JP",
		c.BaseURL,
		url.QueryEscape(artist.Name),
	)

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	// 404 means the artist was not found in Setlist.fm — treat as no results.
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	// 429 means rate limited — wait and retry once before giving up.
	if resp.StatusCode == http.StatusTooManyRequests {
		resp.Body.Close()
		if c.RetryDelay > 0 {
			time.Sleep(c.RetryDelay)
		}
		resp2, err2 := c.HTTP.Do(req.Clone(req.Context()))
		if err2 != nil {
			return nil, fmt.Errorf("request (retry): %w", err2)
		}
		defer resp2.Body.Close()
		if resp2.StatusCode == http.StatusTooManyRequests {
			// Still rate limited after retry — skip this artist silently.
			fmt.Printf("setlistfm: rate limited for %q after retry, skipping\n", artist.Name)
			return nil, nil
		}
		resp = resp2
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for artist %q", resp.StatusCode, artist.Name)
	}

	var result searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return toUpcomingConcerts(artist, result.Setlist), nil
}

// toUpcomingConcerts filters out past events and maps setlists to domain.Concert.
func toUpcomingConcerts(artist domain.Artist, setlists []slSetlist) []domain.Concert {
	today := time.Now().Truncate(24 * time.Hour)
	var concerts []domain.Concert
	for _, sl := range setlists {
		date, err := time.Parse(dateLayout, sl.EventDate)
		if err != nil {
			continue // skip unparseable dates rather than failing the batch
		}
		if date.Before(today) {
			continue
		}
		concerts = append(concerts, domain.Concert{
			ID:        sl.ID,
			Artist:    artist,
			Venue:     sl.Venue.Name,
			City:      sl.Venue.City.Name,
			Date:      date,
			TicketURL: sl.URL, // links to the setlist.fm event page
		})
	}
	return concerts
}
