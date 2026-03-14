package cli

import (
	"testing"
)

// setEnv sets env vars for the duration of a test and restores them on cleanup.
func setEnv(t *testing.T, pairs map[string]string) {
	t.Helper()
	for k, v := range pairs {
		t.Setenv(k, v)
	}
}

func fullEnv() map[string]string {
	return map[string]string{
		"SPOTIFY_ACCESS_TOKEN":  "sp-token-123", // optional but included for coverage
		"SPOTIFY_CLIENT_ID":     "client-id",
		"SPOTIFY_CLIENT_SECRET": "client-secret",
		"SETLISTFM_API_KEY":     "slkey-abc",
		"DISCORD_WEBHOOK_URL":   "https://discord.com/api/webhooks/123/abc",
	}
}

// TestLoadConfig_AllPresent verifies all fields are correctly read when every
// env var is set.
func TestLoadConfig_AllPresent(t *testing.T) {
	setEnv(t, fullEnv())

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SpotifyToken != "sp-token-123" {
		t.Errorf("expected SpotifyToken sp-token-123, got %q", cfg.SpotifyToken)
	}
	if cfg.ClientID != "client-id" {
		t.Errorf("expected ClientID client-id, got %q", cfg.ClientID)
	}
	if cfg.SetlistFMAPIKey != "slkey-abc" {
		t.Errorf("expected SetlistFMAPIKey slkey-abc, got %q", cfg.SetlistFMAPIKey)
	}
	if cfg.DiscordWebhookURL != "https://discord.com/api/webhooks/123/abc" {
		t.Errorf("expected DiscordWebhookURL, got %q", cfg.DiscordWebhookURL)
	}
}

// TestLoadConfig_SpotifyTokenOptional verifies that omitting SPOTIFY_ACCESS_TOKEN
// does not cause an error — the token will be loaded from the store at runtime.
func TestLoadConfig_SpotifyTokenOptional(t *testing.T) {
	env := fullEnv()
	delete(env, "SPOTIFY_ACCESS_TOKEN")
	setEnv(t, env)

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("expected no error when SPOTIFY_ACCESS_TOKEN is unset, got: %v", err)
	}
	if cfg.SpotifyToken != "" {
		t.Errorf("expected empty SpotifyToken, got %q", cfg.SpotifyToken)
	}
}

// TestLoadConfig_MissingSetlistFMAPIKey verifies a missing SETLISTFM_API_KEY
// returns a descriptive error.
func TestLoadConfig_MissingSetlistFMAPIKey(t *testing.T) {
	env := fullEnv()
	delete(env, "SETLISTFM_API_KEY")
	setEnv(t, env)

	if _, err := loadConfig(); err == nil {
		t.Fatal("expected error for missing SETLISTFM_API_KEY, got nil")
	}
}

// TestLoadConfig_MissingDiscordWebhookURL verifies a missing DISCORD_WEBHOOK_URL
// returns a descriptive error.
func TestLoadConfig_MissingDiscordWebhookURL(t *testing.T) {
	env := fullEnv()
	delete(env, "DISCORD_WEBHOOK_URL")
	setEnv(t, env)

	if _, err := loadConfig(); err == nil {
		t.Fatal("expected error for missing DISCORD_WEBHOOK_URL, got nil")
	}
}
