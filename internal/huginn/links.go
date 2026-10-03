package huginn

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	linkRe = regexp.MustCompile("https?://[^\\s<>()\\[\\]{}\"'`|]+")
	// codeRe is code in an answer, whose addresses are examples, not links.
	codeRe = regexp.MustCompile("(?s)```.*?(?:```|$)|`[^`\n]*`")
	// numericHostRe is an IPv4 or IPv6 address.
	numericHostRe = regexp.MustCompile(`^[\d.]+$|:`)
)

// Links are the web addresses an answer links to, outside its code, each
// once, in order.
func Links(answer string) []string {
	var out []string
	seen := map[string]bool{}
	for _, raw := range linkRe.FindAllString(codeRe.ReplaceAllString(answer, " "), -1) {
		u := strings.TrimRight(raw, ".,;:!?*_~")
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

// UngroundedLinks are the answer's links to pages that no source gave: an
// address the model wrote from memory, which is often wrong or gone. A link
// is grounded when it appears in the evidence or in what the person wrote.
// A site's home page, an example address, and one on this computer or
// network are taken as they are.
func UngroundedLinks(answer string, sources ...string) []string {
	var known strings.Builder
	for _, s := range sources {
		known.WriteString(sourceKey(s))
		known.WriteString("\n")
	}
	text := known.String()
	var out []string
	for _, raw := range Links(answer) {
		if linkTrusted(raw) || strings.Contains(text, linkKey(raw)) {
			continue
		}
		out = append(out, raw)
	}
	return out
}

// schemeless drops schemes and www. from addresses, for comparison.
var schemeless = strings.NewReplacer("https://", "", "http://", "", "www.", "")

// sourceKey writes a source's addresses the way linkKey does.
func sourceKey(s string) string {
	return schemeless.Replace(strings.ToLower(s))
}

// linkKey writes an address one way for comparison: lower case, without
// scheme, www., fragment, or trailing slash.
func linkKey(s string) string {
	s, _, _ = strings.Cut(sourceKey(s), "#")
	return strings.TrimRight(s, "/")
}

func linkTrusted(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return true
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case (u.Path == "" || u.Path == "/") && u.RawQuery == "":
		return true
	case host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".test") || strings.HasSuffix(host, ".example") || strings.HasSuffix(host, ".internal"):
		return true
	case host == "example.com" || host == "example.org" || host == "example.net" || strings.HasSuffix(host, ".example.com"):
		return true
	case !strings.Contains(host, "."):
		return true
	}
	// An address by number is on this computer or a network it can see.
	return numericHostRe.MatchString(host)
}

// DescribeLinks lists ungrounded links for the model, one per line.
func DescribeLinks(links []string) string {
	var b strings.Builder
	for _, l := range links {
		b.WriteString("- " + l + "\n")
	}
	return b.String()
}
