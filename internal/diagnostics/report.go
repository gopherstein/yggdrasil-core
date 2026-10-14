package diagnostics

import (
	"encoding/json"
	"net"
	"regexp"
	"strings"
)

// Report is what a "Report this" sends with a problem (#588): enough to
// find it, nothing that says who or where the person is.
type Report struct {
	Version  string `json:"version"`
	Commit   string `json:"commit,omitempty"`
	Contract string `json:"contract"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	CPU      string `json:"cpu,omitempty"`
	MemoryGB int    `json:"memory_gb,omitempty"`
	// GPUs names each graphics card or chip, such as "Apple M3 Pro".
	GPUs []string `json:"gpus,omitempty"`
	// Log is Core's last warnings and errors, scrubbed, for an Admin.
	Log []string `json:"log,omitempty"`
}

var (
	emailRe = regexp.MustCompile(`[\w.+-]+@[\w-]+(?:\.[\w-]+)+`)
	// A key, token, or session: Toskar's own prefixes, a bearer header,
	// and any long run of letters and digits that a word or ID wouldn't be.
	keyRe    = regexp.MustCompile(`\b(?:ygg|tsk|toskar)_[\w-]+`)
	bearerRe = regexp.MustCompile(`(?i)\b(bearer\s+|(?:token|secret|password|key)"?\s*[=:]\s*"?)[^\s",}<][^\s",}]*`)
	longRe   = regexp.MustCompile(`\b[A-Za-z0-9+/_-]{32,}={0,2}`)
	// A home folder on each kind of computer, up to the person's name.
	homeRe = regexp.MustCompile(`(?i)(/Users/|/home/|[A-Z]:\\Users\\)[^/\\\s"]+`)
	ipv4Re = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`)
	// Something that may be an IPv6 address; addressKind decides.
	ipv6Re = regexp.MustCompile(`(?i)\b[0-9a-f]{0,4}(?::[0-9a-f]{0,4}){2,7}\b`)
	uuidRe = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
)

// Scrub takes out of text what could identify the person or let someone
// in: email addresses, keys and tokens, the name in a home folder, the
// computer's name, and internet addresses, which are replaced by their
// kind. Addresses on the home network stay: they help and give nothing
// away. IDs (UUIDs) stay too, to match a report to a log.
func Scrub(text, hostname string) string {
	text = emailRe.ReplaceAllString(text, "<email>")
	text = keyRe.ReplaceAllString(text, "<key>")
	text = bearerRe.ReplaceAllString(text, "${1}<hidden>")
	text = longRe.ReplaceAllStringFunc(text, func(m string) string {
		if uuidRe.MatchString(m) || !hasDigitAndLetter(m) {
			return m
		}
		return "<hidden>"
	})
	text = homeRe.ReplaceAllString(text, "${1}<you>")
	if h := strings.TrimSpace(hostname); len(h) >= 3 {
		text = replaceFold(text, strings.TrimSuffix(h, ".local"), "<computer>")
	}
	text = ipv4Re.ReplaceAllStringFunc(text, addressKind)
	text = ipv6Re.ReplaceAllStringFunc(text, func(m string) string {
		// A time such as 18:16:04 isn't an address: an IPv6 one has :: or a hex letter.
		if !strings.Contains(m, "::") && !strings.ContainsAny(strings.ToLower(m), "abcdef") {
			return m
		}
		if uuidRe.MatchString(m) {
			return m
		}
		return addressKind(m)
	})
	return text
}

// addressKind is an address on the home network as it is, and any other
// as its kind.
func addressKind(m string) string {
	ip := net.ParseIP(m)
	switch {
	case ip == nil:
		return m
	case ip.IsLoopback(), ip.IsPrivate(), ip.IsLinkLocalUnicast(), ip.IsUnspecified():
		return m
	case ip.To4() != nil:
		if ip[12] == 100 && ip[13] >= 64 && ip[13] <= 127 {
			return "<carrier-grade NAT address>"
		}
		return "<public IPv4>"
	}
	return "<public IPv6>"
}

func hasDigitAndLetter(s string) bool {
	return strings.ContainsAny(s, "0123456789") && strings.IndexFunc(s, func(r rune) bool {
		return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
	}) >= 0
}

// replaceFold replaces old in s whatever its case.
func replaceFold(s, old, repl string) string {
	if old == "" {
		return s
	}
	re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(old))
	return re.ReplaceAllString(s, repl)
}

// reportFields are the parts of a log line a report keeps. The others can
// hold what someone wrote (a prompt, a memory, a file name).
var reportFields = []string{"error", "err", "code", "status", "model", "runtime", "backend", "route"}

// ReportLog is the last n warnings and errors in a JSON-lines log, each as
// one line of time, level, message, and the fields above, scrubbed.
func ReportLog(content, hostname string, n int) []string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		var rec map[string]any
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		level, _ := rec["level"].(string)
		if level != "WARN" && level != "ERROR" {
			continue
		}
		t, _ := rec["time"].(string)
		msg, _ := rec["msg"].(string)
		parts := []string{t, level, msg}
		for _, k := range reportFields {
			if v, ok := rec[k]; ok {
				b, _ := json.Marshal(v)
				parts = append(parts, k+"="+string(b))
			}
		}
		out = append(out, Scrub(strings.Join(parts, " "), hostname))
	}
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return out
}
