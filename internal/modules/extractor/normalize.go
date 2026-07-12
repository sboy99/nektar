package extractor

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	unsubPattern = regexp.MustCompile(`(?i)unsubscribe|manage\s+(your\s+)?preferences|opt[-\s]?out|email\s+preferences|update\s+your\s+preferences`)
	spacePattern = regexp.MustCompile(`[ \t\x0a\x0d]{2,}`)
)

// NormalizeHTML removes navigation, footers, ads, unsubscribe blocks, and collapses whitespace.
func NormalizeHTML(html string) (string, error) {
	html = strings.TrimSpace(html)
	if html == "" {
		return "", nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}

	removeJunk(doc)
	collapseWhitespace(doc)

	body := doc.Find("body")
	if body.Length() > 0 {
		out, err := body.Html()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(out), nil
	}

	out, err := doc.Html()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func removeJunk(doc *goquery.Document) {
	selectors := []string{
		"nav",
		"footer",
		"header",
		"[role=navigation]",
		"[role=banner]",
		"[role=contentinfo]",
		".footer",
		"#footer",
		".nav",
		"#nav",
		".navigation",
		"#navigation",
		".navbar",
		".site-footer",
		".email-footer",
		".ad",
		".ads",
		".advert",
		".advertisement",
		"[class*=advert]",
		"[id*=advert]",
		"[class*=promo]",
		"script",
		"style",
		"noscript",
	}
	for _, sel := range selectors {
		doc.Find(sel).Remove()
	}

	// Tracking pixels.
	doc.Find("img").Each(func(_ int, s *goquery.Selection) {
		w, _ := s.Attr("width")
		h, _ := s.Attr("height")
		if (w == "1" || w == "0") && (h == "1" || h == "0") {
			s.Remove()
		}
	})

	// Unsubscribe / preference links and their containing blocks.
	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		text := s.Text()
		combined := href + " " + text
		if unsubPattern.MatchString(combined) {
			// Prefer removing a nearby block if it looks like a footer row.
			parent := s.Parent()
			if parent.Is("p, div, td, span, li") && len(strings.TrimSpace(parent.Text())) < 200 {
				parent.Remove()
				return
			}
			s.Remove()
		}
	})

	doc.Find("p, div, td, span").Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" && unsubPattern.MatchString(text) && len(text) < 200 {
			s.Remove()
		}
	})
}

func collapseWhitespace(doc *goquery.Document) {
	doc.Find("*").Contents().Each(func(_ int, s *goquery.Selection) {
		if goquery.NodeName(s) != "#text" {
			return
		}
		node := s.Get(0)
		if node == nil {
			return
		}
		node.Data = spacePattern.ReplaceAllString(node.Data, " ")
	})
}
