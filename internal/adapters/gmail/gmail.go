package gmail

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"github.com/sboy99/nektar/internal/shared/errors"
	"github.com/sboy99/nektar/shared/domain"
)

// Provider implements the email port using the Gmail API.
type Provider struct {
	service *gmail.Service
}

// Config holds Gmail OAuth2 credentials.
type Config struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
}

// NewProvider creates a Gmail email provider.
func NewProvider(ctx context.Context, cfg Config) (*Provider, error) {
	token := &oauth2.Token{RefreshToken: cfg.RefreshToken}
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
		Scopes: []string{gmail.GmailReadonlyScope},
	}

	client := oauth2.NewClient(ctx, oauthCfg.TokenSource(ctx, token))
	svc, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	return &Provider{service: svc}, nil
}

// NewProviderWithClient creates a provider with a pre-built HTTP client (for testing).
func NewProviderWithClient(ctx context.Context, client *http.Client) (*Provider, error) {
	svc, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	return &Provider{service: svc}, nil
}

// Fetch retrieves newsletter emails from Gmail.
func (p *Provider) Fetch(ctx context.Context) ([]domain.Email, error) {
	_ = ctx
	_ = p.service
	return nil, errors.ErrNotImplemented
}
