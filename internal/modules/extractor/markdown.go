package extractor

import (
	"strings"
	"unicode"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
)

// ToMarkdown converts cleaned HTML into Markdown.
func ToMarkdown(html string) (string, error) {
	html = strings.TrimSpace(html)
	if html == "" {
		return "", nil
	}
	md, err := htmltomarkdown.ConvertString(html)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(md), nil
}

// PlainTextFromHTML extracts readable plain text from HTML.
func PlainTextFromHTML(html string) string {
	html = strings.TrimSpace(html)
	if html == "" {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return stripTagsFallback(html)
	}
	text := doc.Find("body").Text()
	if strings.TrimSpace(text) == "" {
		text = doc.Text()
	}
	return collapsePlainWhitespace(text)
}

// ReadingTimeMinutes estimates reading time from plain text.
func ReadingTimeMinutes(plain string, wordsPerMinute int) int {
	if wordsPerMinute <= 0 {
		wordsPerMinute = 200
	}
	words := countWords(plain)
	if words == 0 {
		return 1
	}
	minutes := (words + wordsPerMinute - 1) / wordsPerMinute
	if minutes < 1 {
		return 1
	}
	return minutes
}

func countWords(s string) int {
	count := 0
	inWord := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			inWord = false
			continue
		}
		if !inWord {
			count++
			inWord = true
		}
	}
	return count
}

func collapsePlainWhitespace(s string) string {
	return strings.TrimSpace(spacePattern.ReplaceAllString(s, " "))
}

func stripTagsFallback(html string) string {
	var b strings.Builder
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return collapsePlainWhitespace(b.String())
}
