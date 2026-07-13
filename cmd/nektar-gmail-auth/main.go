package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/sboy99/nektar/internal/platform/config"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
)

func main() {
	if err := config.LoadDotEnv(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	clientID := flag.String("client-id", os.Getenv("NEKTAR_GMAIL_CLIENT_ID"), "OAuth client ID")
	clientSecret := flag.String("client-secret", os.Getenv("NEKTAR_GMAIL_CLIENT_SECRET"), "OAuth client secret")
	userID := flag.String("user-id", "user-1", "users.id / gmail_sync.user_id for the SQL seed")
	email := flag.String("email", "you@example.com", "user email for the SQL seed")
	name := flag.String("name", "You", "user display name for the SQL seed")
	query := flag.String("query", "newer_than:7d", "initial Gmail search query for gmail_sync")
	addr := flag.String("addr", "localhost:8085", "local redirect listen address")
	flag.Parse()

	if *clientID == "" || *clientSecret == "" {
		fmt.Fprintln(os.Stderr, "usage: nektar-gmail-auth -client-id=... -client-secret=... [-user-id=user-1]")
		fmt.Fprintln(os.Stderr, "or set NEKTAR_GMAIL_CLIENT_ID and NEKTAR_GMAIL_CLIENT_SECRET in the environment / .env")
		os.Exit(1)
	}

	redirectURL := "http://" + *addr + "/oauth2/callback"
	cfg := &oauth2.Config{
		ClientID:     *clientID,
		ClientSecret: *clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{gmail.GmailReadonlyScope},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/callback", func(w http.ResponseWriter, r *http.Request) {
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			errCh <- fmt.Errorf("oauth error: %s", errMsg)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("missing code")
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		fmt.Fprintln(w, "Authorization received. You can close this tab.")
		codeCh <- code
	})

	server := &http.Server{Addr: *addr, Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	authURL := cfg.AuthCodeURL("nektar", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Println("Open this URL in your browser:")
	fmt.Println(authURL)
	fmt.Println()

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	_ = server.Shutdown(context.Background())

	token, err := cfg.Exchange(context.Background(), code)
	if err != nil {
		fmt.Fprintf(os.Stderr, "token exchange failed: %v\n", err)
		os.Exit(1)
	}
	if token.RefreshToken == "" {
		fmt.Fprintln(os.Stderr, "no refresh token returned; revoke prior access and retry with consent")
		os.Exit(1)
	}

	fmt.Println("Refresh token obtained.")
	fmt.Println()
	fmt.Println("1) Keep app credentials in .env:")
	fmt.Println()
	fmt.Printf("NEKTAR_GMAIL_CLIENT_ID=%s\n", *clientID)
	fmt.Printf("NEKTAR_GMAIL_CLIENT_SECRET=%s\n", *clientSecret)
	fmt.Println()
	fmt.Println("2) Store the refresh token in Postgres (gmail_sync.refresh_token):")
	fmt.Println()
	fmt.Printf(`INSERT INTO users (id, email, name, created_at, updated_at)
VALUES (%s, %s, %s, now(), now())
ON CONFLICT (id) DO UPDATE SET
  email = EXCLUDED.email,
  name = EXCLUDED.name,
  updated_at = now();

INSERT INTO gmail_sync (user_id, history_id, last_synced_at, query, refresh_token)
VALUES (%s, '', now(), %s, %s)
ON CONFLICT (user_id) DO UPDATE SET
  refresh_token = EXCLUDED.refresh_token,
  query = EXCLUDED.query,
  last_synced_at = EXCLUDED.last_synced_at;
`,
		sqlQuote(*userID),
		sqlQuote(*email),
		sqlQuote(*name),
		sqlQuote(*userID),
		sqlQuote(*query),
		sqlQuote(token.RefreshToken),
	)
}

func sqlQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
