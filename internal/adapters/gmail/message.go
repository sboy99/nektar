package gmail

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/gmail/v1"

	"github.com/sboy99/nektar/shared/domain"
)

func mapMessage(userID string, msg *gmail.Message) (*domain.Email, error) {
	if msg == nil {
		return nil, fmt.Errorf("nil message")
	}

	headers := headerMap(msg)
	subject := headers["Subject"]
	from := headers["From"]
	receivedAt := time.UnixMilli(msg.InternalDate).UTC()
	if msg.InternalDate == 0 {
		receivedAt = time.Now().UTC()
	}

	email, err := domain.NewEmail(userID, msg.Id, subject, from, receivedAt)
	if err != nil {
		return nil, err
	}
	email.ThreadID = msg.ThreadId
	email.Headers = headers
	email.RawBody = extractBody(msg.Payload)
	email.ListID = headers["List-Id"]
	if email.ListID == "" {
		email.ListID = headers["List-ID"]
	}
	email.ListUnsubscribe = headers["List-Unsubscribe"]
	return email, nil
}

func headerMap(msg *gmail.Message) map[string]string {
	out := make(map[string]string)
	if msg.Payload == nil {
		return out
	}
	for _, h := range msg.Payload.Headers {
		if h == nil || h.Name == "" {
			continue
		}
		out[h.Name] = h.Value
	}
	return out
}

func extractBody(payload *gmail.MessagePart) string {
	if payload == nil {
		return ""
	}
	if html := findPartBody(payload, "text/html"); html != "" {
		return html
	}
	return findPartBody(payload, "text/plain")
}

func findPartBody(part *gmail.MessagePart, mimeType string) string {
	if part == nil {
		return ""
	}
	if strings.EqualFold(part.MimeType, mimeType) && part.Body != nil && part.Body.Data != "" {
		return decodeBody(part.Body.Data)
	}
	for _, child := range part.Parts {
		if body := findPartBody(child, mimeType); body != "" {
			return body
		}
	}
	return ""
}

func decodeBody(data string) string {
	// Gmail uses URL-safe base64 without padding.
	decoded, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(data)
		if err != nil {
			return data
		}
	}
	return string(decoded)
}
