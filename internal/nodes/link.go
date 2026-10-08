package nodes

import (
	"crypto/tls"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/yeixio/toskar-core/internal/auth"
)

// Bifrost over TLS (#175). A computer serves HTTPS and plain HTTP on the
// same port, so computers that haven't updated still reach it. A client
// speaks TLS first, checked against the key stored when the computers
// paired; it uses plain HTTP only with a computer that answers without TLS
// and never has spoken it, so an attacker can't push a computer that has
// back to plain HTTP.

// Link is how a paired computer is reached: the fingerprint of its key, and
// whether it has spoken TLS before.
type Link struct {
	// Pin is the computer's key fingerprint (auth.KeyFingerprint form);
	// empty before pairing, when TLS still encrypts but doesn't check.
	Pin string
	// TLSSeen means it spoke TLS before, so plain HTTP is refused.
	TLSSeen bool
	// MarkTLS records the first time it speaks TLS.
	MarkTLS func()
}

// ErrPlainAfterTLS is a computer that spoke TLS before and now answers
// without it.
var ErrPlainAfterTLS = errors.New("this computer answered without encryption, though it used encryption before; it may not be the computer you paired with")

// plainFor is how long a computer that answered without TLS is reached over
// plain HTTP before TLS is tried again, so an update there is noticed.
const plainFor = 10 * time.Minute

// links remembers, by computer, which way it was last reached.
var links = struct {
	sync.Mutex
	plainUntil map[string]time.Time
	used       map[string]string
}{plainUntil: map[string]time.Time{}, used: map[string]string{}}

// Transport is the way a paired computer was last reached: "tls", "plain",
// or "" before it was.
func Transport(key string) string {
	links.Lock()
	defer links.Unlock()
	return links.used[key]
}

func noteTransport(key, how string) {
	links.Lock()
	defer links.Unlock()
	links.used[key] = how
	if how == "tls" {
		delete(links.plainUntil, key)
	}
}

func plainNow(key string) bool {
	links.Lock()
	defer links.Unlock()
	return time.Now().Before(links.plainUntil[key])
}

func notePlain(key string) {
	links.Lock()
	defer links.Unlock()
	links.plainUntil[key] = time.Now().Add(plainFor)
	links.used[key] = "plain"
}

// linkTransport sends http:// requests to a computer over TLS, and falls
// back to plain HTTP as Link allows.
type linkTransport struct {
	key   string
	link  Link
	tls   http.RoundTripper
	plain http.RoundTripper
}

func newLinkTransport(key string, link Link) *linkTransport {
	secure := http.DefaultTransport.(*http.Transport).Clone()
	secure.TLSClientConfig = auth.PinnedTLSConfig(link.Pin)
	return &linkTransport{key: key, link: link, tls: secure, plain: http.DefaultTransport}
}

func (t *linkTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "http" {
		return t.tls.RoundTrip(req)
	}
	if t.link.TLSSeen || !plainNow(t.key) {
		secure := req.Clone(req.Context())
		u := *req.URL
		u.Scheme = "https"
		secure.URL = &u
		resp, err := t.tls.RoundTrip(secure)
		if err == nil {
			noteTransport(t.key, "tls")
			if !t.link.TLSSeen && t.link.MarkTLS != nil {
				t.link.MarkTLS()
				t.link.TLSSeen = true
			}
			return resp, nil
		}
		var notTLS tls.RecordHeaderError
		if !errors.As(err, &notTLS) {
			return nil, err
		}
		if t.link.TLSSeen {
			return nil, ErrPlainAfterTLS
		}
		notePlain(t.key)
		// The request is sent again, plainly; one whose body can't be
		// read twice fails this once, and the next goes plainly at once.
		if req.Body != nil && req.Body != http.NoBody {
			if req.GetBody == nil {
				return nil, err
			}
			body, berr := req.GetBody()
			if berr != nil {
				return nil, err
			}
			req = req.Clone(req.Context())
			req.Body = body
		}
	}
	return t.plain.RoundTrip(req)
}
