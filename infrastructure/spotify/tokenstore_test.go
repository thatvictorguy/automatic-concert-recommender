package spotify_test

import (
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/thatvictorguy/automatic-concert-recommender/infrastructure/spotify"
)

func testStore(t *testing.T) *spotify.TokenStore {
	t.Helper()
	return &spotify.TokenStore{Path: filepath.Join(t.TempDir(), "tokens.json")}
}

// TestTokenStore_SaveAndLoad verifies a round-trip: save a token, load it back,
// and confirm all fields match.
func TestTokenStore_SaveAndLoad(t *testing.T) {
	store := testStore(t)
	want := &oauth2.Token{
		AccessToken:  "access-abc",
		RefreshToken: "refresh-xyz",
		Expiry:       time.Now().Add(time.Hour).Truncate(time.Second),
	}

	if err := store.Save(want); err != nil {
		t.Fatalf("Save: unexpected error: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}

	if got.AccessToken != want.AccessToken {
		t.Errorf("AccessToken: got %q, want %q", got.AccessToken, want.AccessToken)
	}
	if got.RefreshToken != want.RefreshToken {
		t.Errorf("RefreshToken: got %q, want %q", got.RefreshToken, want.RefreshToken)
	}
	if !got.Expiry.Equal(want.Expiry) {
		t.Errorf("Expiry: got %v, want %v", got.Expiry, want.Expiry)
	}
}

// TestTokenStore_LoadMissingFile verifies that loading from a non-existent path
// returns ErrNoToken.
func TestTokenStore_LoadMissingFile(t *testing.T) {
	store := testStore(t)

	_, err := store.Load()
	if err != spotify.ErrNoToken {
		t.Errorf("expected ErrNoToken, got %v", err)
	}
}

// TestTokenStore_SaveCreatesDirectories verifies that Save creates any missing
// parent directories (important for first-time setup).
func TestTokenStore_SaveCreatesDirectories(t *testing.T) {
	store := &spotify.TokenStore{
		Path: filepath.Join(t.TempDir(), "nested", "dir", "tokens.json"),
	}
	token := &oauth2.Token{
		AccessToken: "tok",
		Expiry:      time.Now().Add(time.Hour),
	}

	if err := store.Save(token); err != nil {
		t.Fatalf("expected Save to create parent dirs, got error: %v", err)
	}
	if _, err := store.Load(); err != nil {
		t.Fatalf("Load after Save with nested dirs failed: %v", err)
	}
}

// TestTokenStore_OverwritesExistingToken verifies that saving again replaces
// the previous token cleanly.
func TestTokenStore_OverwritesExistingToken(t *testing.T) {
	store := testStore(t)

	first := &oauth2.Token{AccessToken: "first", Expiry: time.Now().Add(time.Hour)}
	if err := store.Save(first); err != nil {
		t.Fatalf("Save first: %v", err)
	}

	second := &oauth2.Token{AccessToken: "second", Expiry: time.Now().Add(time.Hour)}
	if err := store.Save(second); err != nil {
		t.Fatalf("Save second: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.AccessToken != "second" {
		t.Errorf("expected second token, got %q", got.AccessToken)
	}
}
