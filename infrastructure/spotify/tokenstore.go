package spotify

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
)

// ErrNoToken is returned by TokenStore.Load when no token file exists yet.
// The user should run `concert-recommender auth` to authenticate.
var ErrNoToken = errors.New("no stored token found: run 'concert-recommender auth' first")

// TokenStore persists and loads OAuth tokens to/from a JSON file on disk.
type TokenStore struct {
	Path string
}

// NewTokenStore returns a TokenStore using the default path:
// ~/.concert-recommender/tokens.json
func NewTokenStore() (*TokenStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("tokenstore: resolve home dir: %w", err)
	}
	return &TokenStore{
		Path: filepath.Join(home, ".concert-recommender", "tokens.json"),
	}, nil
}

// Save writes the token to disk, creating parent directories if needed.
func (s *TokenStore) Save(t *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
		return fmt.Errorf("tokenstore: create dirs: %w", err)
	}
	f, err := os.Create(s.Path)
	if err != nil {
		return fmt.Errorf("tokenstore: create file: %w", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(t); err != nil {
		return fmt.Errorf("tokenstore: encode token: %w", err)
	}
	return nil
}

// Load reads the token from disk. Returns ErrNoToken if the file does not exist.
func (s *TokenStore) Load() (*oauth2.Token, error) {
	f, err := os.Open(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoToken
	}
	if err != nil {
		return nil, fmt.Errorf("tokenstore: open file: %w", err)
	}
	defer f.Close()

	var t oauth2.Token
	if err := json.NewDecoder(f).Decode(&t); err != nil {
		return nil, fmt.Errorf("tokenstore: decode token: %w", err)
	}
	return &t, nil
}
