package cli

import (
	"errors"
	"os"
)

type config struct {
	// SpotifyToken is optional — if empty, token is loaded from the store.
	SpotifyToken string
	// SpotifyRefreshToken is used in CI (GitHub Actions) instead of a local token store.
	SpotifyRefreshToken string
	ClientID            string
	ClientSecret        string
	SetlistFMAPIKey   string
	DiscordWebhookURL string
}

// loadConfig reads configuration from environment variables.
// SPOTIFY_ACCESS_TOKEN is optional: if unset, the stored token is used.
// Returns a descriptive error listing all missing required variables.
func loadConfig() (config, error) {
	var missing []string

	require := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg := config{
		SpotifyToken:        os.Getenv("SPOTIFY_ACCESS_TOKEN"),  // optional
		SpotifyRefreshToken: os.Getenv("SPOTIFY_REFRESH_TOKEN"), // optional, CI path
		ClientID:            os.Getenv("SPOTIFY_CLIENT_ID"),     // optional (needed for refresh)
		ClientSecret:        os.Getenv("SPOTIFY_CLIENT_SECRET"), // optional (needed for refresh)
		SetlistFMAPIKey:     require("SETLISTFM_API_KEY"),
		DiscordWebhookURL:   require("DISCORD_WEBHOOK_URL"),
	}

	if len(missing) > 0 {
		msg := "missing required environment variables:"
		for _, k := range missing {
			msg += "\n  " + k
		}
		return config{}, errors.New(msg)
	}

	return cfg, nil
}
