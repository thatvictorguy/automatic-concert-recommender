package spotify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/thatvictorguy/automatic-concert-recommender/domain"
)

const defaultBaseURL = "https://api.spotify.com"

// Client implements domain.MusicSource using the Spotify Web API.
// AccessToken must be a valid OAuth 2.0 Bearer token with the
// user-top-read scope granted.
type Client struct {
	AccessToken string
	BaseURL     string
	HTTP        *http.Client
}

// New returns a Client ready for production use.
func New(accessToken string) *Client {
	return &Client{
		AccessToken: accessToken,
		BaseURL:     defaultBaseURL,
		HTTP:        &http.Client{Timeout: 10 * time.Second},
	}
}

// --- Spotify API response types ---

type topArtistsResponse struct {
	Items []spArtist `json:"items"`
}

type spArtist struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Genres []string `json:"genres"`
}

// TopArtists fetches the current user's top artists from Spotify.
// Requires the user-top-read OAuth scope.
func (c *Client) TopArtists() ([]domain.Artist, error) {
	endpoint := c.BaseURL + "/v1/me/top/artists?limit=50&time_range=medium_term"

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("spotify: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("spotify: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify: unexpected status %d", resp.StatusCode)
	}

	var body topArtistsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("spotify: decode response: %w", err)
	}

	return toArtists(body.Items), nil
}

// toArtists maps Spotify API artist objects to domain.Artist values.
func toArtists(items []spArtist) []domain.Artist {
	artists := make([]domain.Artist, len(items))
	for i, item := range items {
		artists[i] = domain.Artist{
			ID:     item.ID,
			Name:   item.Name,
			Genres: item.Genres,
		}
	}
	return artists
}
