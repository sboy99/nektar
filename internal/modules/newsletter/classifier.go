package newsletter

import (
	"net/mail"
	"strings"

	"github.com/sboy99/nektar/shared/domain"
)

const (
	weightListID          = 0.40
	weightListUnsubscribe = 0.35
	weightPrecedence      = 0.20
	weightBulkHeaders     = 0.15
)

// ClassifierConfig holds scoring thresholds and sender lists.
type ClassifierConfig struct {
	ScoreThreshold float64
	Allowlist      []string
	Denylist       []string
}

// Result is the outcome of newsletter classification.
type Result struct {
	IsNewsletter bool
	Score        float64
	ListID       string
	Reason       string
}

// Classify scores an email using header and sender heuristics.
func Classify(email *domain.Email, cfg ClassifierConfig) Result {
	threshold := cfg.ScoreThreshold
	if threshold <= 0 {
		threshold = 0.5
	}

	fromAddr := extractAddress(email.From)
	headers := email.Headers
	if headers == nil {
		headers = map[string]string{}
	}

	if matchSender(fromAddr, cfg.Denylist) {
		return Result{IsNewsletter: false, Score: 0, Reason: "denylist"}
	}
	if isSpam(headers) {
		return Result{IsNewsletter: false, Score: 0, Reason: "spam"}
	}
	if matchSender(fromAddr, cfg.Allowlist) {
		listID := resolveListID(email, headers)
		return Result{IsNewsletter: true, Score: 1, ListID: listID, Reason: "allowlist"}
	}

	listID := resolveListID(email, headers)
	listUnsub := resolveListUnsubscribe(email, headers)

	var score float64
	if listID != "" {
		score += weightListID
	}
	if listUnsub != "" {
		score += weightListUnsubscribe
	}
	if hasPrecedenceBulk(headers) {
		score += weightPrecedence
	}
	if hasBulkCampaignHeaders(headers) {
		score += weightBulkHeaders
	}

	if score > 1 {
		score = 1
	}

	if score >= threshold {
		return Result{IsNewsletter: true, Score: score, ListID: listID, Reason: "score"}
	}
	return Result{IsNewsletter: false, Score: score, ListID: listID, Reason: "below_threshold"}
}

func resolveListID(email *domain.Email, headers map[string]string) string {
	raw := email.ListID
	if raw == "" {
		raw = headerValue(headers, "List-Id", "List-ID")
	}
	return parseListID(raw)
}

func resolveListUnsubscribe(email *domain.Email, headers map[string]string) string {
	if email.ListUnsubscribe != "" {
		return strings.TrimSpace(email.ListUnsubscribe)
	}
	return strings.TrimSpace(headerValue(headers, "List-Unsubscribe"))
}

// parseListID extracts the list identifier from a List-ID header value.
// Examples: "TLDR <tldr.tech>" → "tldr.tech", "<news.example.com>" → "news.example.com"
func parseListID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if start := strings.Index(raw, "<"); start >= 0 {
		end := strings.Index(raw[start:], ">")
		if end > 1 {
			return strings.TrimSpace(raw[start+1 : start+end])
		}
	}
	return raw
}

func headerValue(headers map[string]string, names ...string) string {
	for _, name := range names {
		if v, ok := headers[name]; ok && v != "" {
			return v
		}
		// Case-insensitive fallback.
		for k, v := range headers {
			if strings.EqualFold(k, name) && v != "" {
				return v
			}
		}
	}
	return ""
}

func extractAddress(from string) string {
	from = strings.TrimSpace(from)
	if from == "" {
		return ""
	}
	addr, err := mail.ParseAddress(from)
	if err == nil && addr.Address != "" {
		return strings.ToLower(addr.Address)
	}
	// Fallback: bare email or last token.
	from = strings.Trim(from, "<>")
	if at := strings.LastIndex(from, " "); at >= 0 {
		from = strings.Trim(from[at+1:], "<>")
	}
	return strings.ToLower(strings.TrimSpace(from))
}

func matchSender(addr string, list []string) bool {
	if addr == "" || len(list) == 0 {
		return false
	}
	addr = strings.ToLower(addr)
	domain := ""
	if at := strings.LastIndex(addr, "@"); at >= 0 {
		domain = addr[at:] // includes leading @
	}
	for _, entry := range list {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		if strings.HasPrefix(entry, "@") {
			if domain != "" && domain == entry {
				return true
			}
			continue
		}
		if addr == entry {
			return true
		}
	}
	return false
}

func isSpam(headers map[string]string) bool {
	flag := headerValue(headers, "X-Spam-Flag")
	if strings.EqualFold(strings.TrimSpace(flag), "YES") {
		return true
	}
	status := headerValue(headers, "X-Spam-Status")
	if status == "" {
		return false
	}
	// Common formats: "Yes, score=..." or "Yes"
	lower := strings.ToLower(strings.TrimSpace(status))
	return strings.HasPrefix(lower, "yes")
}

func hasPrecedenceBulk(headers map[string]string) bool {
	prec := strings.ToLower(strings.TrimSpace(headerValue(headers, "Precedence")))
	return prec == "bulk" || prec == "list"
}

var knownESPMailers = []string{
	"mailchimp",
	"sendgrid",
	"mandrill",
	"postmark",
	"amazon ses",
	"sparkpost",
	"convertkit",
	"klaviyo",
	"substack",
	"beehiiv",
	"buttondown",
	"campaign monitor",
	"constant contact",
}

func hasBulkCampaignHeaders(headers map[string]string) bool {
	if headerValue(headers, "List-Post") != "" || headerValue(headers, "List-Help") != "" {
		return true
	}
	for k := range headers {
		if strings.HasPrefix(strings.ToLower(k), "x-campaign") {
			return true
		}
	}
	mailer := strings.ToLower(headerValue(headers, "X-Mailer", "X-Mailer-Info"))
	if mailer == "" {
		return false
	}
	for _, esp := range knownESPMailers {
		if strings.Contains(mailer, esp) {
			return true
		}
	}
	return false
}
