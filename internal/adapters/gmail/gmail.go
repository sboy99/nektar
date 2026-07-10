package gmail

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	portemail "github.com/sboy99/nektar/internal/ports/email"
	"github.com/sboy99/nektar/shared/domain"
)

// Provider implements the email port using the Gmail API.
type Provider struct {
	clientID     string
	clientSecret string
	defaultQuery string
	// client, when set, is used instead of building an OAuth client (tests).
	client *http.Client
	// endpoint overrides the Gmail API base URL (tests).
	endpoint string
}

// Config holds shared Gmail OAuth2 application credentials.
type Config struct {
	ClientID     string
	ClientSecret string
	DefaultQuery string
}

// NewProvider creates a Gmail email provider.
func NewProvider(cfg Config) *Provider {
	return &Provider{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		defaultQuery: cfg.DefaultQuery,
	}
}

// NewProviderWithClient creates a provider with a pre-built HTTP client (for testing).
func NewProviderWithClient(client *http.Client, defaultQuery, endpoint string) *Provider {
	return &Provider{client: client, defaultQuery: defaultQuery, endpoint: endpoint}
}

// Fetch retrieves emails for one user using History API or a query bootstrap.
func (p *Provider) Fetch(ctx context.Context, params portemail.FetchParams) (*portemail.FetchResult, error) {
	if params.RefreshToken == "" && p.client == nil {
		return nil, fmt.Errorf("gmail: refresh token is required")
	}

	svc, err := p.service(ctx, params.RefreshToken)
	if err != nil {
		return nil, err
	}

	query := params.Query
	if query == "" {
		query = p.defaultQuery
	}

	if params.HistoryID != "" {
		result, err := p.fetchViaHistory(ctx, svc, params.UserID, params.HistoryID)
		if err == nil {
			return result, nil
		}
		if !isHistoryExpired(err) {
			return nil, err
		}
	}

	return p.fetchViaList(ctx, svc, params.UserID, query)
}

func (p *Provider) service(ctx context.Context, refreshToken string) (*gmail.Service, error) {
	client := p.client
	if client == nil {
		token := &oauth2.Token{RefreshToken: refreshToken}
		oauthCfg := &oauth2.Config{
			ClientID:     p.clientID,
			ClientSecret: p.clientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.google.com/o/oauth2/auth",
				TokenURL: "https://oauth2.googleapis.com/token",
			},
			Scopes: []string{gmail.GmailReadonlyScope},
		}
		client = oauth2.NewClient(ctx, oauthCfg.TokenSource(ctx, token))
	}
	opts := []option.ClientOption{option.WithHTTPClient(client)}
	if p.endpoint != "" {
		opts = append(opts, option.WithEndpoint(p.endpoint))
	}
	return gmail.NewService(ctx, opts...)
}

func (p *Provider) fetchViaHistory(ctx context.Context, svc *gmail.Service, userID, historyID string) (*portemail.FetchResult, error) {
	startID, err := strconv.ParseUint(historyID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("gmail: invalid history id %q: %w", historyID, err)
	}

	seen := make(map[string]struct{})
	var messageIDs []string
	var newHistoryID string
	pageToken := ""

	for {
		call := svc.Users.History.List("me").StartHistoryId(startID).Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, err
		}

		newHistoryID = strconv.FormatUint(resp.HistoryId, 10)
		for _, h := range resp.History {
			for _, added := range h.MessagesAdded {
				if added.Message == nil || added.Message.Id == "" {
					continue
				}
				id := added.Message.Id
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				messageIDs = append(messageIDs, id)
			}
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	emails, err := p.getMessages(ctx, svc, userID, messageIDs)
	if err != nil {
		return nil, err
	}

	return &portemail.FetchResult{Emails: emails, NewHistoryID: newHistoryID}, nil
}

func (p *Provider) fetchViaList(ctx context.Context, svc *gmail.Service, userID, query string) (*portemail.FetchResult, error) {
	var messageIDs []string
	pageToken := ""

	for {
		call := svc.Users.Messages.List("me").Context(ctx)
		if query != "" {
			call = call.Q(query)
		}
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, err
		}
		for _, m := range resp.Messages {
			if m != nil && m.Id != "" {
				messageIDs = append(messageIDs, m.Id)
			}
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	emails, err := p.getMessages(ctx, svc, userID, messageIDs)
	if err != nil {
		return nil, err
	}

	profile, err := svc.Users.GetProfile("me").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("gmail: get profile: %w", err)
	}

	return &portemail.FetchResult{
		Emails:       emails,
		NewHistoryID: strconv.FormatUint(profile.HistoryId, 10),
	}, nil
}

func (p *Provider) getMessages(ctx context.Context, svc *gmail.Service, userID string, ids []string) ([]domain.Email, error) {
	emails := make([]domain.Email, 0, len(ids))
	for _, id := range ids {
		msg, err := svc.Users.Messages.Get("me", id).Format("full").Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("gmail: get message %s: %w", id, err)
		}
		email, err := mapMessage(userID, msg)
		if err != nil {
			return nil, fmt.Errorf("gmail: map message %s: %w", id, err)
		}
		emails = append(emails, *email)
	}
	return emails, nil
}

func isHistoryExpired(err error) bool {
	var gerr *googleapi.Error
	if !asGoogleAPIError(err, &gerr) {
		return false
	}
	if gerr.Code == http.StatusNotFound {
		return true
	}
	// Gmail returns 400 when startHistoryId is too old / invalid.
	if gerr.Code == http.StatusBadRequest && strings.Contains(strings.ToLower(gerr.Message), "history") {
		return true
	}
	return false
}

func asGoogleAPIError(err error, target **googleapi.Error) bool {
	for err != nil {
		if gerr, ok := err.(*googleapi.Error); ok {
			*target = gerr
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
