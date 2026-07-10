package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"

	portemail "github.com/sboy99/nektar/internal/ports/email"
)

func TestMapMessage(t *testing.T) {
	html := base64.URLEncoding.EncodeToString([]byte("<p>hello</p>"))
	msg := &gmail.Message{
		Id:           "msg-1",
		ThreadId:     "thread-1",
		InternalDate: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC).UnixMilli(),
		Payload: &gmail.MessagePart{
			Headers: []*gmail.MessagePartHeader{
				{Name: "Subject", Value: "Weekly Digest"},
				{Name: "From", Value: "news@example.com"},
				{Name: "List-Id", Value: "<news.example.com>"},
				{Name: "List-Unsubscribe", Value: "<mailto:unsub@example.com>"},
			},
			MimeType: "multipart/alternative",
			Parts: []*gmail.MessagePart{
				{
					MimeType: "text/plain",
					Body:     &gmail.MessagePartBody{Data: base64.URLEncoding.EncodeToString([]byte("plain"))},
				},
				{
					MimeType: "text/html",
					Body:     &gmail.MessagePartBody{Data: html},
				},
			},
		},
	}

	email, err := mapMessage("user-1", msg)
	if err != nil {
		t.Fatalf("mapMessage: %v", err)
	}
	if email.UserID != "user-1" || email.GmailMessageID != "msg-1" || email.ThreadID != "thread-1" {
		t.Fatalf("unexpected ids: %+v", email)
	}
	if email.Subject != "Weekly Digest" || email.From != "news@example.com" {
		t.Fatalf("unexpected headers: subject=%q from=%q", email.Subject, email.From)
	}
	if email.RawBody != "<p>hello</p>" {
		t.Fatalf("expected html body, got %q", email.RawBody)
	}
	if email.ListID != "<news.example.com>" || email.ListUnsubscribe != "<mailto:unsub@example.com>" {
		t.Fatalf("unexpected list headers: %+v", email)
	}
	if email.ID != "" {
		t.Fatalf("ID should be left empty for fetcher, got %q", email.ID)
	}
}

func TestExtractBodyPrefersHTML(t *testing.T) {
	payload := &gmail.MessagePart{
		MimeType: "multipart/alternative",
		Parts: []*gmail.MessagePart{
			{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: base64.RawURLEncoding.EncodeToString([]byte("plain"))}},
			{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: base64.RawURLEncoding.EncodeToString([]byte("<b>html</b>"))}},
		},
	}
	if got := extractBody(payload); got != "<b>html</b>" {
		t.Fatalf("got %q", got)
	}
}

func TestIsHistoryExpired(t *testing.T) {
	if !isHistoryExpired(&googleapi.Error{Code: 404, Message: "not found"}) {
		t.Fatal("404 should be expired")
	}
	if !isHistoryExpired(&googleapi.Error{Code: 400, Message: "Start history id is too old"}) {
		t.Fatal("400 history error should be expired")
	}
	if isHistoryExpired(&googleapi.Error{Code: 500, Message: "boom"}) {
		t.Fatal("500 should not be expired")
	}
}

func TestFetchViaList(t *testing.T) {
	html := base64.URLEncoding.EncodeToString([]byte("<p>body</p>"))
	mux := http.NewServeMux()
	mux.HandleFunc("/gmail/v1/users/me/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "newer_than:7d" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(&gmail.ListMessagesResponse{
			Messages: []*gmail.Message{{Id: "m1"}},
		})
	})
	mux.HandleFunc("/gmail/v1/users/me/messages/m1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&gmail.Message{
			Id:           "m1",
			ThreadId:     "t1",
			InternalDate: 1_700_000_000_000,
			Payload: &gmail.MessagePart{
				Headers: []*gmail.MessagePartHeader{
					{Name: "Subject", Value: "Hello"},
					{Name: "From", Value: "a@b.com"},
				},
				MimeType: "text/html",
				Body:     &gmail.MessagePartBody{Data: html},
			},
		})
	})
	mux.HandleFunc("/gmail/v1/users/me/profile", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(&gmail.Profile{HistoryId: 99})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	p := NewProviderWithClient(server.Client(), "newer_than:7d", server.URL+"/")
	result, err := p.Fetch(context.Background(), portemail.FetchParams{
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if result.NewHistoryID != "99" {
		t.Fatalf("history id: %q", result.NewHistoryID)
	}
	if len(result.Emails) != 1 || result.Emails[0].GmailMessageID != "m1" {
		t.Fatalf("emails: %+v", result.Emails)
	}
	if result.Emails[0].RawBody != "<p>body</p>" {
		t.Fatalf("body: %q", result.Emails[0].RawBody)
	}
}

func TestFetchHistoryFallbackOn404(t *testing.T) {
	html := base64.URLEncoding.EncodeToString([]byte("ok"))
	mux := http.NewServeMux()
	mux.HandleFunc("/gmail/v1/users/me/history", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(&googleapi.Error{Code: 404, Message: "notFound"})
	})
	mux.HandleFunc("/gmail/v1/users/me/messages", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/messages/") {
			return
		}
		_ = json.NewEncoder(w).Encode(&gmail.ListMessagesResponse{
			Messages: []*gmail.Message{{Id: "m2"}},
		})
	})
	mux.HandleFunc("/gmail/v1/users/me/messages/m2", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(&gmail.Message{
			Id:       "m2",
			ThreadId: "t2",
			Payload: &gmail.MessagePart{
				Headers: []*gmail.MessagePartHeader{
					{Name: "Subject", Value: "Fallback"},
					{Name: "From", Value: "x@y.com"},
				},
				MimeType: "text/plain",
				Body:     &gmail.MessagePartBody{Data: html},
			},
		})
	})
	mux.HandleFunc("/gmail/v1/users/me/profile", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(&gmail.Profile{HistoryId: 42})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	p := NewProviderWithClient(server.Client(), "label:newsletter", server.URL+"/")
	result, err := p.Fetch(context.Background(), portemail.FetchParams{
		UserID:    "user-1",
		HistoryID: "1",
		Query:     "label:newsletter",
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if result.NewHistoryID != "42" || len(result.Emails) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestFetchViaHistory(t *testing.T) {
	html := base64.URLEncoding.EncodeToString([]byte("hist"))
	mux := http.NewServeMux()
	mux.HandleFunc("/gmail/v1/users/me/history", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("startHistoryId") != "10" {
			t.Errorf("startHistoryId=%s", r.URL.Query().Get("startHistoryId"))
		}
		_ = json.NewEncoder(w).Encode(&gmail.ListHistoryResponse{
			HistoryId: 20,
			History: []*gmail.History{{
				MessagesAdded: []*gmail.HistoryMessageAdded{{
					Message: &gmail.Message{Id: "hm1"},
				}},
			}},
		})
	})
	mux.HandleFunc("/gmail/v1/users/me/messages/hm1", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(&gmail.Message{
			Id:       "hm1",
			ThreadId: "ht1",
			Payload: &gmail.MessagePart{
				Headers: []*gmail.MessagePartHeader{
					{Name: "Subject", Value: "Hist"},
					{Name: "From", Value: "h@e.com"},
				},
				MimeType: "text/plain",
				Body:     &gmail.MessagePartBody{Data: html},
			},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	p := NewProviderWithClient(server.Client(), "", server.URL+"/")
	result, err := p.Fetch(context.Background(), portemail.FetchParams{
		UserID:    "user-1",
		HistoryID: "10",
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if result.NewHistoryID != "20" || len(result.Emails) != 1 || result.Emails[0].GmailMessageID != "hm1" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
