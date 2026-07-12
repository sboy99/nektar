package digest

import (
	"fmt"
	"strconv"
	"strings"
)

// topicSection is one rendered topic block for the digest body.
type topicSection struct {
	Heading string
	Summary string
	Sources []sourceLink
}

type sourceLink struct {
	Title string
	URL   string
}

func renderDigestMarkdown(tmpl string, title, intro string, sections []topicSection, readingTime int) string {
	var body strings.Builder
	for i, sec := range sections {
		if i > 0 {
			body.WriteString("\n\n")
		}
		body.WriteString("## ")
		body.WriteString(sec.Heading)
		body.WriteString("\n\n")
		body.WriteString(strings.TrimSpace(sec.Summary))
		if len(sec.Sources) > 0 {
			body.WriteString("\n\n**Sources**\n")
			for _, src := range sec.Sources {
				if src.URL != "" {
					body.WriteString(fmt.Sprintf("- [%s](%s)\n", src.Title, src.URL))
				} else {
					body.WriteString(fmt.Sprintf("- %s\n", src.Title))
				}
			}
		}
	}

	return renderPrompt(tmpl, map[string]string{
		"title":         title,
		"intro":         intro,
		"sections":      strings.TrimSpace(body.String()),
		"reading_time":  strconv.Itoa(readingTime),
	})
}

func buildIntro(day string, topicCount, articleCount int) string {
	return fmt.Sprintf(
		"Your technical digest for %s covering %d topic(s) and %d article(s).",
		day, topicCount, articleCount,
	)
}

func dailyTitle(day string) string {
	return "Daily Digest — " + day
}

func cleanHeading(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'`")
	s = strings.Split(s, "\n")[0]
	return strings.TrimSpace(s)
}
