// Package importer converts exports of other tools (Vikunja, Clockify) into
// a kala export that can be loaded with the regular JSON import.
package importer

import (
	"strings"
	"unicode"

	"kala/internal/tags"
)

// slug turns a free-form name into a valid tag segment: lowercase, with
// whitespace and the characters tags reject (or use as separators) replaced
// by dashes. It returns "" when nothing usable is left.
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune(`,"#@!:`, r) {
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
			continue
		}
		if r == '-' {
			if dash || b.Len() == 0 {
				continue
			}
			dash = true
		} else {
			dash = false
		}
		b.WriteRune(r)
	}
	out := strings.TrimRight(b.String(), "-")
	if out == "" {
		return ""
	}
	if _, err := tags.Normalize(out); err != nil {
		return ""
	}
	return out
}

// projectTag returns the project tag for a project name, or "" for none.
func projectTag(name string) string {
	s := slug(name)
	if s == "" {
		return ""
	}
	if _, err := tags.Normalize("project:" + s); err != nil {
		return ""
	}
	return "project:" + s
}
