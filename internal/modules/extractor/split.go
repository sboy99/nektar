package extractor

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ArticleCandidate is a split content unit ready to persist as a domain.Article.
type ArticleCandidate struct {
	Title     string
	RawHTML   string
	Markdown  string
	PlainText string
	URL       string
	Position  int
}

// SplitConfig controls article splitting heuristics.
type SplitConfig struct {
	MinArticleChars int
	WordsPerMinute  int
	FallbackTitle   string
}

// SplitArticles detects multiple articles in cleaned HTML, or returns a single article.
func SplitArticles(html string, cfg SplitConfig) ([]ArticleCandidate, error) {
	if cfg.MinArticleChars <= 0 {
		cfg.MinArticleChars = 100
	}
	if cfg.WordsPerMinute <= 0 {
		cfg.WordsPerMinute = 200
	}

	html = strings.TrimSpace(html)
	if html == "" {
		return nil, nil
	}

	sections, err := splitSections(html, cfg.MinArticleChars)
	if err != nil {
		return nil, err
	}
	if len(sections) == 0 {
		return singleArticle(html, cfg.FallbackTitle)
	}

	out := make([]ArticleCandidate, 0, len(sections))
	for i, sec := range sections {
		md, err := ToMarkdown(sec.HTML)
		if err != nil {
			return nil, err
		}
		plain := PlainTextFromHTML(sec.HTML)
		title := sec.Title
		if strings.TrimSpace(title) == "" {
			title = cfg.FallbackTitle
		}
		if strings.TrimSpace(title) == "" {
			title = "Untitled"
		}
		out = append(out, ArticleCandidate{
			Title:     strings.TrimSpace(title),
			RawHTML:   sec.HTML,
			Markdown:  md,
			PlainText: plain,
			URL:       sec.URL,
			Position:  i,
		})
	}
	return out, nil
}

type section struct {
	Title string
	HTML  string
	URL   string
}

func singleArticle(html, fallbackTitle string) ([]ArticleCandidate, error) {
	md, err := ToMarkdown(html)
	if err != nil {
		return nil, err
	}
	plain := PlainTextFromHTML(html)
	title := strings.TrimSpace(fallbackTitle)
	if title == "" {
		title = "Untitled"
	}
	return []ArticleCandidate{{
		Title:     title,
		RawHTML:   html,
		Markdown:  md,
		PlainText: plain,
		URL:       firstAbsoluteURL(html),
		Position:  0,
	}}, nil
}

func splitSections(html string, minChars int) ([]section, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div id=\"root\">" + html + "</div>"))
	if err != nil {
		return nil, err
	}
	root := doc.Find("#root")
	if root.Length() == 0 {
		return nil, nil
	}

	// Prefer heading-based sections when multiple substantial h1/h2 blocks exist.
	if sections := splitByHeadings(root, minChars); len(sections) >= 2 {
		return sections, nil
	}

	// Fall back to <hr>-separated blocks.
	if sections := splitByHR(html, minChars); len(sections) >= 2 {
		return sections, nil
	}

	return nil, nil
}

func splitByHeadings(root *goquery.Selection, minChars int) []section {
	headings := root.Find("h1, h2")
	if headings.Length() < 2 {
		return nil
	}

	var sections []section
	headings.Each(func(_ int, h *goquery.Selection) {
		title := strings.TrimSpace(h.Text())
		var parts []string
		for sib := h.Next(); sib.Length() > 0; sib = sib.Next() {
			name := goquery.NodeName(sib)
			if name == "h1" || name == "h2" {
				break
			}
			html, err := goquery.OuterHtml(sib)
			if err != nil || strings.TrimSpace(html) == "" {
				continue
			}
			parts = append(parts, html)
		}
		body := strings.TrimSpace(strings.Join(parts, "\n"))
		combined := title + "\n" + body
		if len(stripTagsFallback(combined)) < minChars {
			return
		}
		sectionHTML := strings.TrimSpace(goqueryOuterOrEmpty(h) + "\n" + body)
		sections = append(sections, section{
			Title: title,
			HTML:  sectionHTML,
			URL:   firstAbsoluteURL(sectionHTML),
		})
	})

	if len(sections) < 2 {
		return nil
	}
	return sections
}

func goqueryOuterOrEmpty(s *goquery.Selection) string {
	html, err := goquery.OuterHtml(s)
	if err != nil {
		return ""
	}
	return html
}

func splitByHR(html string, minChars int) []section {
	// Case-insensitive split on <hr>, <hr/>, <hr ...>.
	lower := strings.ToLower(html)
	var parts []string
	start := 0
	for {
		idx := strings.Index(lower[start:], "<hr")
		if idx < 0 {
			parts = append(parts, html[start:])
			break
		}
		idx += start
		parts = append(parts, html[start:idx])
		end := strings.Index(lower[idx:], ">")
		if end < 0 {
			parts = append(parts, html[idx:])
			break
		}
		start = idx + end + 1
	}

	var sections []section
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		plain := PlainTextFromHTML(part)
		if len(plain) < minChars {
			continue
		}
		title := firstHeadingTitle(part)
		sections = append(sections, section{
			Title: title,
			HTML:  part,
			URL:   firstAbsoluteURL(part),
		})
	}
	if len(sections) < 2 {
		return nil
	}
	return sections
}

func firstHeadingTitle(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}
	h := doc.Find("h1, h2, h3").First()
	return strings.TrimSpace(h.Text())
}

func firstAbsoluteURL(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}
	var url string
	doc.Find("a[href]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		href, _ := s.Attr("href")
		href = strings.TrimSpace(href)
		if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
			// Skip unsubscribe-ish links.
			if unsubPattern.MatchString(href + " " + s.Text()) {
				return true
			}
			url = href
			return false
		}
		return true
	})
	return url
}
