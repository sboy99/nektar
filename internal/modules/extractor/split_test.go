package extractor

import (
	"strings"
	"testing"
)

func TestSplitArticlesByHeadings(t *testing.T) {
	html := `
<h2>First Story</h2>
<p>This is the first article body with enough characters to pass the minimum length threshold for splitting.</p>
<p><a href="https://news.example/first">Read more</a></p>
<h2>Second Story</h2>
<p>This is the second article body with enough characters to pass the minimum length threshold for splitting.</p>
<p><a href="https://news.example/second">Read more</a></p>
`
	articles, err := SplitArticles(html, SplitConfig{
		MinArticleChars: 50,
		FallbackTitle:   "Weekly Digest",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 2 {
		t.Fatalf("expected 2 articles, got %d: %+v", len(articles), articles)
	}
	if articles[0].Title != "First Story" || articles[1].Title != "Second Story" {
		t.Fatalf("titles: %q %q", articles[0].Title, articles[1].Title)
	}
	if articles[0].URL != "https://news.example/first" {
		t.Fatalf("url: %q", articles[0].URL)
	}
	if articles[0].Position != 0 || articles[1].Position != 1 {
		t.Fatalf("positions: %d %d", articles[0].Position, articles[1].Position)
	}
	if articles[0].Markdown == "" || articles[0].PlainText == "" {
		t.Fatalf("missing markdown/plain: %+v", articles[0])
	}
}

func TestSplitArticlesSingleFallback(t *testing.T) {
	html := `<p>A single short newsletter body without multiple headings.</p>`
	articles, err := SplitArticles(html, SplitConfig{
		MinArticleChars: 100,
		FallbackTitle:   "Solo Subject",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}
	if articles[0].Title != "Solo Subject" {
		t.Fatalf("title: %q", articles[0].Title)
	}
}

func TestSplitArticlesByHR(t *testing.T) {
	html := `
<p>Block one has enough characters to qualify as its own article section in the newsletter digest.</p>
<hr>
<p>Block two has enough characters to qualify as its own article section in the newsletter digest.</p>
`
	articles, err := SplitArticles(html, SplitConfig{
		MinArticleChars: 40,
		FallbackTitle:   "HR Digest",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 2 {
		t.Fatalf("expected 2 hr articles, got %d (%v)", len(articles), titles(articles))
	}
	for _, a := range articles {
		if !strings.Contains(a.PlainText, "enough characters") {
			t.Fatalf("unexpected plain: %q", a.PlainText)
		}
	}
}

func titles(articles []ArticleCandidate) []string {
	out := make([]string, len(articles))
	for i, a := range articles {
		out[i] = a.Title
	}
	return out
}
