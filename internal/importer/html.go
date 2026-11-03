package importer

import (
	"html"
	"regexp"
	"strings"
)

var (
	reLink    = regexp.MustCompile(`(?is)<a\b[^>]*\bhref\s*=\s*["']([^"']*)["'][^>]*>(.*?)</a>`)
	reBreak   = regexp.MustCompile(`(?i)<br\s*/?>|</(li|tr)>`)
	rePara    = regexp.MustCompile(`(?i)</(p|div|h[1-6]|blockquote|pre|ul|ol)>`)
	reItem    = regexp.MustCompile(`(?i)<li\b[^>]*>`)
	reTag     = regexp.MustCompile(`(?s)<[^>]*>`)
	reBlanks  = regexp.MustCompile(`\n{3,}`)
	reTrailSp = regexp.MustCompile(`[ \t]+\n`)
)

// htmlToText reduces the HTML of a rich-text description to plain text:
// paragraphs are separated by blank lines, list items become "- " lines, links keep their
// URL. Text without tags is returned trimmed but otherwise unchanged.
func htmlToText(s string) string {
	s = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(s)
	if !strings.Contains(s, "<") {
		return strings.TrimSpace(s)
	}
	s = reLink.ReplaceAllStringFunc(s, func(m string) string {
		sub := reLink.FindStringSubmatch(m)
		href, text := sub[1], strings.TrimSpace(reTag.ReplaceAllString(sub[2], ""))
		if text == "" || html.UnescapeString(text) == html.UnescapeString(href) {
			return href
		}
		return text + " (" + href + ")"
	})
	s = reItem.ReplaceAllString(s, "- ")
	s = reBreak.ReplaceAllString(s, "\n")
	s = rePara.ReplaceAllString(s, "\n\n")
	s = reTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = reTrailSp.ReplaceAllString(s, "\n")
	s = reBlanks.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
