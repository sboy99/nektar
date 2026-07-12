package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
)

func main() {
	clientID := flag.String("client-id", os.Getenv("NEKTAR_GMAIL_CLIENT_ID"), "OAuth client ID")
	clientSecret := flag.String("client-secret", os.Getenv("NEKTAR_GMAIL_CLIENT_SECRET"), "OAuth client secret")
	ref := flag.String("ref", "default", "refresh_tokens map key to print in the YAML snippet")
	addr := flag.String("addr", "localhost:8085", "local redirect listen address")
	flag.Parse()

	if *clientID == "" || *clientSecret == "" {
		fmt.Fprintln(os.Stderr, "usage: nektar-gmail-auth -client-id=... -client-secret=... [-ref=alice]")
		fmt.Fprintln(os.Stderr, "or set NEKTAR_GMAIL_CLIENT_ID and NEKTAR_GMAIL_CLIENT_SECRET")
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

	fmt.Println("Refresh token obtained. Add to .env:")
	fmt.Println()
	fmt.Printf("NEKTAR_GMAIL_CLIENT_ID=%s\n", *clientID)
	fmt.Printf("NEKTAR_GMAIL_CLIENT_SECRET=%s\n", *clientSecret)
	fmt.Printf("NEKTAR_GMAIL_REFRESH_TOKENS={\"%s\":%q}\n", *ref, token.RefreshToken)
	fmt.Println()
	fmt.Println("Then seed a user + gmail_sync row with refresh_token_ref matching that key.")
}
