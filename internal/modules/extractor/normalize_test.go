package extractor

import (
	"strings"
	"testing"
)

func TestNormalizeHTMLRemovesJunk(t *testing.T) {
	raw := `
<html><body>
<nav>Home About</nav>
<div class="content">
  <h1>Main Story</h1>
  <p>This is the important newsletter content that should remain.</p>
</div>
<div class="ad advert">Buy now!</div>
<footer>
  <a href="https://example.com/unsubscribe">Unsubscribe</a>
</footer>
<img src="https://track.example/pixel.gif" width="1" height="1">
</body></html>`

	out, err := NormalizeHTML(raw)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(out)
	if !strings.Contains(out, "Main Story") || !strings.Contains(out, "important newsletter") {
		t.Fatalf("main content missing: %s", out)
	}
	for _, junk := range []string{"home about", "buy now", "unsubscribe", "pixel.gif"} {
		if strings.Contains(lower, junk) {
			t.Fatalf("junk still present (%s): %s", junk, out)
		}
	}
}

func TestToMarkdownAndReadingTime(t *testing.T) {
	html := `<h1>Title</h1><p>One two three four five.</p>`
	md, err := ToMarkdown(html)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "Title") || !strings.Contains(md, "One two") {
		t.Fatalf("markdown unexpected: %s", md)
	}
	plain := PlainTextFromHTML(html)
	if rt := ReadingTimeMinutes(plain, 200); rt < 1 {
		t.Fatalf("reading time: %d", rt)
	}
}
