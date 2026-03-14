package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	spotifyendpoint "golang.org/x/oauth2/spotify"

	"github.com/thatvictorguy/automatic-concert-recommender/application"
	"github.com/thatvictorguy/automatic-concert-recommender/infrastructure/discord"
	"github.com/thatvictorguy/automatic-concert-recommender/infrastructure/setlistfm"
	"github.com/thatvictorguy/automatic-concert-recommender/infrastructure/spotify"
)

var recommendCmd = &cobra.Command{
	Use:   "recommend",
	Short: "Fetch your top Spotify artists and email upcoming Japan concerts",
	RunE:  runRecommend,
}

func init() {
	rootCmd.AddCommand(recommendCmd)
}

func runRecommend(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	accessToken, err := resolveSpotifyToken(cfg)
	if err != nil {
		return err
	}

	uc := application.RecommendUseCase{
		Music:    spotify.New(accessToken),
		Concerts: setlistfm.New(cfg.SetlistFMAPIKey),
		Notifier: discord.New(cfg.DiscordWebhookURL),
	}

	fmt.Println("Fetching your top artists from Spotify...")
	fmt.Println("Searching for upcoming concerts in Japan...")

	if err := uc.Run(); err != nil {
		return fmt.Errorf("recommendation failed: %w", err)
	}

	fmt.Println("Done! Concert recommendations posted to Discord.")
	return nil
}

// resolveSpotifyToken returns a valid Spotify access token by checking, in order:
//  1. SPOTIFY_ACCESS_TOKEN env var (manual override)
//  2. SPOTIFY_REFRESH_TOKEN env var (GitHub Actions path — no local store)
//  3. Stored token on disk — auto-refreshed via oauth2.TokenSource if expired
func resolveSpotifyToken(cfg config) (string, error) {
	// 1. Explicit access token takes precedence.
	if cfg.SpotifyToken != "" {
		return cfg.SpotifyToken, nil
	}

	// 2. CI path: refresh token supplied via env var (GitHub Actions).
	if cfg.SpotifyRefreshToken != "" {
		return resolveFromRefreshToken(cfg)
	}

	// 3. Local dev path: load from on-disk token store.
	return resolveFromTokenStore(cfg)
}

// resolveFromRefreshToken exchanges a refresh token for a new access token.
// If Spotify rotates the refresh token, the new value is pushed to GitHub Secrets.
func resolveFromRefreshToken(cfg config) (string, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return "", fmt.Errorf(
			"SPOTIFY_REFRESH_TOKEN requires SPOTIFY_CLIENT_ID and SPOTIFY_CLIENT_SECRET",
		)
	}
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     spotifyendpoint.Endpoint,
	}
	// Seed an expired token so TokenSource is forced to refresh immediately.
	seed := &oauth2.Token{RefreshToken: cfg.SpotifyRefreshToken}
	refreshed, err := oauthCfg.TokenSource(context.Background(), seed).Token()
	if err != nil {
		return "", fmt.Errorf("resolve token: refresh: %w", err)
	}

	// If Spotify issued a new refresh token, rotate it in GitHub Secrets.
	if refreshed.RefreshToken != "" && refreshed.RefreshToken != cfg.SpotifyRefreshToken {
		fmt.Println("Spotify issued a new refresh token — rotating SPOTIFY_REFRESH_TOKEN in GitHub Secrets...")
		if err := rotateGitHubSecret("SPOTIFY_REFRESH_TOKEN", refreshed.RefreshToken); err != nil {
			fmt.Printf("Warning: could not rotate SPOTIFY_REFRESH_TOKEN: %v\n", err)
		} else {
			fmt.Println("SPOTIFY_REFRESH_TOKEN rotated successfully.")
		}
	}

	return refreshed.AccessToken, nil
}

// resolveFromTokenStore loads a token from disk and refreshes it if expired.
func resolveFromTokenStore(cfg config) (string, error) {
	store, err := spotify.NewTokenStore()
	if err != nil {
		return "", fmt.Errorf("resolve token: %w", err)
	}
	token, err := store.Load()
	if errors.Is(err, spotify.ErrNoToken) {
		return "", fmt.Errorf("not authenticated: run 'concert-recommender auth' first")
	}
	if err != nil {
		return "", fmt.Errorf("resolve token: load: %w", err)
	}
	if !token.Valid() {
		if cfg.ClientID == "" || cfg.ClientSecret == "" {
			return "", fmt.Errorf(
				"token expired — set SPOTIFY_CLIENT_ID and SPOTIFY_CLIENT_SECRET " +
					"or run 'concert-recommender auth' to re-authenticate",
			)
		}
		oauthCfg := &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     spotifyendpoint.Endpoint,
		}
		refreshed, err := oauthCfg.TokenSource(context.Background(), token).Token()
		if err != nil {
			return "", fmt.Errorf("resolve token: refresh: %w", err)
		}
		if err := store.Save(refreshed); err != nil {
			return "", fmt.Errorf("resolve token: save refreshed: %w", err)
		}
		fmt.Println("Access token refreshed.")
		return refreshed.AccessToken, nil
	}
	return token.AccessToken, nil
}

// rotateGitHubSecret updates a secret in the current GitHub repo using the gh CLI.
// Requires GH_TOKEN and GH_REPO env vars to be set (both available in GitHub Actions by default).
func rotateGitHubSecret(name, value string) error {
	repo := os.Getenv("GH_REPO")
	if repo == "" {
		return fmt.Errorf("GH_REPO not set")
	}
	out, err := exec.Command("gh", "secret", "set", name, "--body", value, "--repo", repo).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}
