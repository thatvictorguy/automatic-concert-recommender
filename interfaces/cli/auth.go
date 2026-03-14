package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	spotifyendpoint "golang.org/x/oauth2/spotify"

	"github.com/thatvictorguy/automatic-concert-recommender/infrastructure/spotify"
)

const (
	callbackPort = "8080"
	callbackPath = "/callback"
	redirectURI  = "http://127.0.0.1:" + callbackPort + callbackPath
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Spotify and store credentials locally",
	RunE:  runAuth,
}

func init() {
	rootCmd.AddCommand(authCmd)
}

func runAuth(cmd *cobra.Command, args []string) error {
	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf(
			"missing required environment variables:\n  SPOTIFY_CLIENT_ID\n  SPOTIFY_CLIENT_SECRET\n\n" +
				"Register your app at https://developer.spotify.com/dashboard\n" +
				"and set the redirect URI to " + redirectURI,
		)
	}

	oauthCfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURI,
		Scopes:       []string{"user-top-read"},
		Endpoint:     spotifyendpoint.Endpoint,
	}

	state, err := randomState()
	if err != nil {
		return fmt.Errorf("auth: generate state: %w", err)
	}

	// Channel receives the authorization code once the callback fires.
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("state"); got != state {
			errCh <- fmt.Errorf("auth: state mismatch (possible CSRF)")
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			errCh <- fmt.Errorf("auth: Spotify returned error: %s", errParam)
			fmt.Fprintf(w, "Authentication failed: %s. You can close this tab.", errParam)
			return
		}
		code := r.URL.Query().Get("code")
		fmt.Fprintln(w, "Authentication successful! You can close this tab and return to your terminal.")
		codeCh <- code
	})

	srv := &http.Server{Addr: ":" + callbackPort, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("auth: callback server: %w", err)
		}
	}()

	// AccessTypeOffline requests a refresh token so we can renew without re-auth.
	authURL := oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
	fmt.Printf("\nOpen the following URL in your browser to connect your Spotify account:\n\n")
	fmt.Println(" ", authURL)
	fmt.Println("\nWaiting for Spotify to redirect back...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var code string
	select {
	case code = <-codeCh:
	case err = <-errCh:
		srv.Shutdown(ctx)
		return err
	case <-ctx.Done():
		srv.Shutdown(ctx)
		return fmt.Errorf("auth: timed out waiting for Spotify callback (5 min)")
	}

	srv.Shutdown(ctx)

	fmt.Println("\nExchanging authorization code for tokens...")
	token, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("auth: token exchange: %w", err)
	}

	store, err := spotify.NewTokenStore()
	if err != nil {
		return fmt.Errorf("auth: init token store: %w", err)
	}
	if err := store.Save(token); err != nil {
		return fmt.Errorf("auth: save token: %w", err)
	}

	fmt.Printf("\nAuthenticated! Credentials saved to %s\n", store.Path)
	fmt.Println("You can now run: concert-recommender recommend")
	fmt.Println()
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Println("To run automatically via GitHub Actions, push these once:")
	fmt.Println()
	fmt.Println("  # Secrets (sensitive values):")
	fmt.Printf("  gh secret set SPOTIFY_CLIENT_SECRET --body %q\n", clientSecret)
	fmt.Printf("  gh secret set SPOTIFY_REFRESH_TOKEN --body %q\n", token.RefreshToken)
	fmt.Println(`  gh secret set DISCORD_WEBHOOK_URL --body "<your_discord_webhook_url>"`)
	fmt.Println()
	fmt.Println("  # Variables (non-sensitive config):")
	fmt.Printf("  gh variable set SPOTIFY_CLIENT_ID --body %q\n", clientID)
	fmt.Println(`  gh variable set BANDSINTOWN_APP_ID --body "concert-recommender"`)
	fmt.Println("─────────────────────────────────────────────────────────")
	return nil
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
