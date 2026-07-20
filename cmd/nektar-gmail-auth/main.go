package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sboy99/nektar/internal/adapters/postgres"
	"github.com/sboy99/nektar/internal/platform/config"
	"github.com/sboy99/nektar/internal/ports/repository"
	"github.com/sboy99/nektar/shared/domain"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	oauth2api "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if err := config.LoadDotEnv(); err != nil {
		return err
	}

	opts, err := parseOptions()
	if err != nil {
		return err
	}

	ctx := context.Background()
	oauthCfg := newOAuthConfig(opts)

	token, err := authorize(ctx, oauthCfg, opts.Addr)
	if err != nil {
		return err
	}
	if err := requireRefreshToken(token); err != nil {
		return err
	}

	profile, err := fetchGoogleProfile(ctx, oauthCfg, token)
	if err != nil {
		return err
	}

	storage, err := postgres.NewStorage(ctx, opts.DSN)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer storage.Close()

	user, sync, err := persistAccount(ctx, storage, profile, token.RefreshToken, opts)
	if err != nil {
		return err
	}

	printSavedAccount(user, sync, profile)
	return nil
}

type options struct {
	ClientID     string
	ClientSecret string
	UserID       string
	Query        string
	Addr         string
	DSN          string
}

func parseOptions() (options, error) {
	opts := options{}
	flag.StringVar(&opts.ClientID, "client-id", os.Getenv("NEKTAR_GMAIL_CLIENT_ID"), "OAuth client ID")
	flag.StringVar(&opts.ClientSecret, "client-secret", os.Getenv("NEKTAR_GMAIL_CLIENT_SECRET"), "OAuth client secret")
	flag.StringVar(&opts.UserID, "user-id", "", "optional users.id UUID override (default: existing email match, else new UUID)")
	flag.StringVar(&opts.Query, "query", "newer_than:7d", "initial Gmail search query for gmail_sync")
	flag.StringVar(&opts.Addr, "addr", "localhost:8085", "local redirect listen address")
	flag.StringVar(&opts.DSN, "dsn", os.Getenv("NEKTAR_POSTGRES_DSN"), "Postgres DSN (default: NEKTAR_POSTGRES_DSN)")
	flag.Parse()
	return opts, validateOptions(opts)
}

func validateOptions(opts options) error {
	if opts.ClientID == "" || opts.ClientSecret == "" {
		return fmt.Errorf("usage: nektar-gmail-auth [-user-id=...] [-query=newer_than:7d]\nrequires NEKTAR_GMAIL_CLIENT_ID, NEKTAR_GMAIL_CLIENT_SECRET, and NEKTAR_POSTGRES_DSN in .env")
	}
	if strings.TrimSpace(opts.DSN) == "" {
		return fmt.Errorf("NEKTAR_POSTGRES_DSN is required")
	}
	return nil
}

func newOAuthConfig(opts options) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     opts.ClientID,
		ClientSecret: opts.ClientSecret,
		RedirectURL:  "http://" + opts.Addr + "/oauth2/callback",
		Scopes: []string{
			gmail.GmailReadonlyScope,
			oauth2api.UserinfoEmailScope,
			oauth2api.UserinfoProfileScope,
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
	}
}

func authorize(ctx context.Context, cfg *oauth2.Config, addr string) (*oauth2.Token, error) {
	code, err := waitForAuthCode(ctx, cfg, addr)
	if err != nil {
		return nil, err
	}
	return exchangeAuthCode(ctx, cfg, code)
}

func waitForAuthCode(ctx context.Context, cfg *oauth2.Config, addr string) (string, error) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	server := newCallbackServer(addr, codeCh, errCh)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	defer server.Shutdown(ctx) //nolint:errcheck

	printAuthURL(cfg)

	select {
	case code := <-codeCh:
		return code, nil
	case err := <-errCh:
		return "", err
	}
}

func newCallbackServer(addr string, codeCh chan<- string, errCh chan<- error) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/callback", func(w http.ResponseWriter, r *http.Request) {
		handleOAuthCallback(w, r, codeCh, errCh)
	})
	return &http.Server{Addr: addr, Handler: mux}
}

func handleOAuthCallback(w http.ResponseWriter, r *http.Request, codeCh chan<- string, errCh chan<- error) {
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		errCh <- fmt.Errorf("oauth error: %s (if access_denied: add this Google account as an OAuth test user)", errMsg)
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
}

func printAuthURL(cfg *oauth2.Config) {
	authURL := cfg.AuthCodeURL("nektar", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Println("Open this URL in your browser:")
	fmt.Println(authURL)
	fmt.Println()
}

func exchangeAuthCode(ctx context.Context, cfg *oauth2.Config, code string) (*oauth2.Token, error) {
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	return token, nil
}

func requireRefreshToken(token *oauth2.Token) error {
	if token.RefreshToken == "" {
		return fmt.Errorf("no refresh token returned; revoke prior access at https://myaccount.google.com/permissions and retry")
	}
	return nil
}

type googleProfile struct {
	ID      string
	Email   string
	Name    string
	Picture string
	Locale  string
}

func fetchGoogleProfile(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*googleProfile, error) {
	client := cfg.Client(ctx, token)
	svc, err := oauth2api.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("create oauth2 service: %w", err)
	}
	info, err := svc.Userinfo.Get().Do()
	if err != nil {
		return nil, fmt.Errorf("fetch google userinfo: %w", err)
	}
	return mapUserinfo(info)
}

func mapUserinfo(info *oauth2api.Userinfo) (*googleProfile, error) {
	email := strings.TrimSpace(info.Email)
	if email == "" {
		return nil, fmt.Errorf("google account has no email")
	}
	return &googleProfile{
		ID:      strings.TrimSpace(info.Id),
		Email:   email,
		Name:    displayName(info, email),
		Picture: strings.TrimSpace(info.Picture),
		Locale:  strings.TrimSpace(info.Locale),
	}, nil
}

func displayName(info *oauth2api.Userinfo, email string) string {
	if name := strings.TrimSpace(info.Name); name != "" {
		return name
	}
	if name := strings.TrimSpace(info.GivenName + " " + info.FamilyName); name != "" {
		return name
	}
	return email
}

func persistAccount(
	ctx context.Context,
	storage *postgres.Storage,
	profile *googleProfile,
	refreshToken string,
	opts options,
) (*domain.User, *domain.GmailSync, error) {
	userID, err := resolveUserID(ctx, storage, profile, opts.UserID)
	if err != nil {
		return nil, nil, err
	}

	user, err := upsertUser(ctx, storage, userID, profile)
	if err != nil {
		return nil, nil, err
	}

	sync, err := upsertGmailSync(ctx, storage, userID, refreshToken, opts.Query)
	if err != nil {
		return nil, nil, err
	}

	return user, sync, nil
}

func resolveUserID(ctx context.Context, storage *postgres.Storage, profile *googleProfile, override string) (string, error) {
	if id := strings.TrimSpace(override); id != "" {
		if _, err := uuid.Parse(id); err != nil {
			return "", fmt.Errorf("user-id must be a UUID: %w", err)
		}
		return id, nil
	}
	if profile.ID != "" {
		existing, err := storage.Users().FindByGoogleID(ctx, profile.ID)
		if err == nil && existing != nil {
			return existing.ID, nil
		}
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return "", fmt.Errorf("find user by google id: %w", err)
		}
	}
	existing, err := storage.Users().FindByEmail(ctx, profile.Email)
	if err == nil && existing != nil {
		return existing.ID, nil
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return "", fmt.Errorf("find user by email: %w", err)
	}
	return uuid.NewString(), nil
}

func upsertUser(ctx context.Context, storage *postgres.Storage, userID string, profile *googleProfile) (*domain.User, error) {
	now := time.Now().UTC()
	user := &domain.User{
		ID:        userID,
		Email:     profile.Email,
		Name:      profile.Name,
		GoogleID:  profile.ID,
		AvatarURL: profile.Picture,
		CreatedAt: now,
		UpdatedAt: now,
	}

	existing, err := storage.Users().FindByID(ctx, userID)
	if err == nil && existing != nil {
		user.CreatedAt = existing.CreatedAt
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("find user: %w", err)
	}

	if err := storage.Users().Save(ctx, user); err != nil {
		return nil, fmt.Errorf("save user: %w", err)
	}
	return user, nil
}

func upsertGmailSync(ctx context.Context, storage *postgres.Storage, userID, refreshToken, query string) (*domain.GmailSync, error) {
	now := time.Now().UTC()
	sync := &domain.GmailSync{
		UserID:       userID,
		HistoryID:    "",
		LastSyncedAt: now,
		Query:        query,
		RefreshToken: refreshToken,
	}

	existing, err := storage.Users().GetGmailSync(ctx, userID)
	if err == nil && existing != nil {
		sync.HistoryID = existing.HistoryID
		sync.LastSyncedAt = existing.LastSyncedAt
		if strings.TrimSpace(query) == "" {
			sync.Query = existing.Query
		}
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("get gmail sync: %w", err)
	}

	if err := storage.Users().SaveGmailSync(ctx, sync); err != nil {
		return nil, fmt.Errorf("save gmail sync: %w", err)
	}
	return sync, nil
}

func printSavedAccount(user *domain.User, sync *domain.GmailSync, profile *googleProfile) {
	fmt.Println("Saved Google account to Postgres.")
	fmt.Printf("  user_id:    %s\n", user.ID)
	fmt.Printf("  email:      %s\n", user.Email)
	fmt.Printf("  name:       %s\n", user.Name)
	if user.GoogleID != "" {
		fmt.Printf("  google_id:  %s\n", user.GoogleID)
	}
	if user.AvatarURL != "" {
		fmt.Printf("  avatar_url: %s\n", user.AvatarURL)
	}
	if profile.Locale != "" {
		fmt.Printf("  locale:     %s\n", profile.Locale)
	}
	fmt.Printf("  query:      %s\n", sync.Query)
	fmt.Println("  refresh_token: stored in gmail_sync")
}
