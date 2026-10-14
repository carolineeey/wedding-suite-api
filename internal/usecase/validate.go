package usecase

import (
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	maxNameChars      = 100
	maxMessageChars   = 1000
	maxShortTextChars = 200
	maxOpeningChars   = 2000
	maxStoryChars     = 5000
	maxURLChars       = 2000
	// maxSlugChars is the DNS label limit: the slug is meant to become a path
	// segment or subdomain once a second wedding exists.
	maxSlugChars = 63
)

func tooLong(s string, maxChars int) bool {
	return utf8.RuneCountInString(s) > maxChars
}

// slugPattern allows lowercase alphanumeric segments joined by single
// hyphens, so a slug is always safe in a URL: no leading, trailing, or
// doubled hyphens.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// normalizeSlug lets an admin send "Caroline-Rafi " and get the canonical
// form stored, the same way invite codes are normalized on lookup.
func normalizeSlug(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}

// isWebURL reports whether s is an absolute http(s) URL. Links admins enter
// are rendered as hrefs on the guest page, so anything else (javascript:,
// data:) is refused rather than trusted.
func isWebURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}
