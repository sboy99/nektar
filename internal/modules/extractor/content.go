package extractor

import (
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
)

// ParsedContent is the body extracted from an email RawBody.
type ParsedContent struct {
	HTML      string
	PlainText string
	IsHTML    bool
}

// ParseContent extracts HTML and/or plain text from an email body.
// Handles MIME multipart messages, HTML documents, and plain text.
func ParseContent(raw string) ParsedContent {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParsedContent{}
	}

	if looksLikeMIME(raw) {
		if parsed, ok := parseMIME(raw); ok {
			return parsed
		}
	}

	if looksLikeHTML(raw) {
		return ParsedContent{HTML: raw, IsHTML: true}
	}

	return ParsedContent{PlainText: raw, IsHTML: false}
}

func looksLikeMIME(raw string) bool {
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "content-type:") {
		return true
	}
	if strings.Contains(lower, "\ncontent-type:") &&
		(strings.Contains(lower, "multipart/") || strings.Contains(lower, "boundary=")) {
		return true
	}
	// RFC 822 style message starting with headers.
	if idx := strings.Index(raw, "\n\n"); idx > 0 {
		headers := strings.ToLower(raw[:idx])
		return strings.Contains(headers, "content-type:") &&
			(strings.Contains(headers, "multipart/") || strings.Contains(headers, "text/"))
	}
	return false
}

func looksLikeHTML(raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html") {
		return true
	}
	// Common email HTML fragments.
	tags := []string{"<div", "<p>", "<p ", "<br", "<table", "<h1", "<h2", "<span", "<a "}
	for _, tag := range tags {
		if strings.Contains(lower, tag) {
			return true
		}
	}
	return false
}

func parseMIME(raw string) (ParsedContent, bool) {
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		return ParsedContent{}, false
	}

	ct := msg.Header.Get("Content-Type")
	if ct == "" {
		ct = "text/plain"
	}
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return ParsedContent{}, false
	}

	htmlBody, plainBody := extractMIMEParts(mediaType, params, msg.Body)
	if htmlBody == "" && plainBody == "" {
		return ParsedContent{}, false
	}
	if htmlBody != "" {
		return ParsedContent{HTML: htmlBody, PlainText: plainBody, IsHTML: true}, true
	}
	return ParsedContent{PlainText: plainBody, IsHTML: false}, true
}

func extractMIMEParts(mediaType string, params map[string]string, body io.Reader) (htmlBody, plainBody string) {
	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return "", ""
		}
		mr := multipart.NewReader(body, boundary)
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			partCT := part.Header.Get("Content-Type")
			if partCT == "" {
				partCT = "text/plain"
			}
			partMedia, partParams, err := mime.ParseMediaType(partCT)
			if err != nil {
				continue
			}
			h, p := extractMIMEParts(partMedia, partParams, part)
			if h != "" && htmlBody == "" {
				htmlBody = h
			}
			if p != "" && plainBody == "" {
				plainBody = p
			}
		}
		return htmlBody, plainBody
	}

	data, err := io.ReadAll(body)
	if err != nil {
		return "", ""
	}
	content := string(data)
	switch {
	case strings.EqualFold(mediaType, "text/html"):
		return content, ""
	case strings.EqualFold(mediaType, "text/plain"):
		return "", content
	default:
		return "", ""
	}
}

// EnsureHTML returns HTML suitable for normalization. Plain text is wrapped in <pre>.
func EnsureHTML(parsed ParsedContent) string {
	if parsed.IsHTML && parsed.HTML != "" {
		return parsed.HTML
	}
	if parsed.PlainText != "" {
		escaped := escapeHTML(parsed.PlainText)
		return "<html><body><pre>" + escaped + "</pre></body></html>"
	}
	if parsed.HTML != "" {
		return parsed.HTML
	}
	return ""
}

func escapeHTML(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	)
	return replacer.Replace(s)
}
