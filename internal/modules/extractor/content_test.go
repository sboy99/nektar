package extractor

import (
	"strings"
	"testing"
)

func TestParseContentHTML(t *testing.T) {
	raw := `<html><body><p>Hello <b>world</b></p></body></html>`
	parsed := ParseContent(raw)
	if !parsed.IsHTML || !strings.Contains(parsed.HTML, "Hello") {
		t.Fatalf("expected HTML parse, got %+v", parsed)
	}
}

func TestParseContentPlain(t *testing.T) {
	raw := "Just a plain newsletter paragraph with no tags."
	parsed := ParseContent(raw)
	if parsed.IsHTML || parsed.PlainText != raw {
		t.Fatalf("expected plain text, got %+v", parsed)
	}
}

func TestParseContentMIME(t *testing.T) {
	raw := "Content-Type: multipart/alternative; boundary=bound123\r\n" +
		"\r\n" +
		"--bound123\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Plain version of the story.\r\n" +
		"--bound123\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<html><body><p>HTML version of the story.</p></body></html>\r\n" +
		"--bound123--\r\n"

	parsed := ParseContent(raw)
	if !parsed.IsHTML {
		t.Fatalf("expected HTML preferred, got %+v", parsed)
	}
	if !strings.Contains(parsed.HTML, "HTML version") {
		t.Fatalf("missing HTML body: %+v", parsed)
	}
	if !strings.Contains(parsed.PlainText, "Plain version") {
		t.Fatalf("missing plain body: %+v", parsed)
	}
}

func TestEnsureHTMLWrapsPlain(t *testing.T) {
	html := EnsureHTML(ParsedContent{PlainText: "hello <world>", IsHTML: false})
	if !strings.Contains(html, "<pre>") || !strings.Contains(html, "&lt;world&gt;") {
		t.Fatalf("unexpected wrap: %s", html)
	}
}
