// Package netx holds optional egress helpers shared by the transport and
// claim paths. Proxies are plain HTTP/HTTPS CONNECT endpoints supplied by the
// operator; nothing is bundled.
package netx

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// Rotator is a round-robin list of egress proxies.
type Rotator struct {
	list []*url.URL
	next atomic.Uint32
}

// FromEnv builds a rotator from ZCODE_PROXIES (comma/semicolon/space/newline
// separated, scheme optional and defaulting to http).
func FromEnv() *Rotator { return FromList(os.Getenv("ZCODE_PROXIES")) }

// FromList parses a proxy list; it returns nil when no valid entry exists.
func FromList(raw string) *Rotator {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == ' ' || r == '\t'
	})
	list := make([]*url.URL, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if !strings.Contains(f, "://") {
			f = "http://" + f
		}
		u, err := url.Parse(f)
		if err != nil || u.Host == "" {
			continue
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			continue
		}
		list = append(list, u)
	}
	if len(list) == 0 {
		return nil
	}
	return &Rotator{list: list}
}

// Len reports the number of configured proxies.
func (r *Rotator) Len() int {
	if r == nil {
		return 0
	}
	return len(r.list)
}

// At returns the proxy for one attempt. Negative attempts advance the shared
// counter, which gives callers a cheap "next proxy" without tracking state.
func (r *Rotator) At(attempt int) *url.URL {
	if r == nil || len(r.list) == 0 {
		return nil
	}
	if attempt < 0 {
		attempt = int(r.next.Add(1)) - 1
	}
	return r.list[attempt%len(r.list)]
}

// Client returns a client whose transport rides the proxy for attempt. The
// base client is never mutated. nil rotator/base returns something usable.
func (r *Rotator) Client(base *http.Client, attempt int, timeout time.Duration) *http.Client {
	var out *http.Client
	if base != nil {
		c := *base
		out = &c
	} else {
		out = &http.Client{}
	}
	if timeout > 0 {
		out.Timeout = timeout
	}
	p := r.At(attempt)
	if p == nil {
		return out
	}
	out.Transport = &http.Transport{
		Proxy:                 http.ProxyURL(p),
		ForceAttemptHTTP2:     true,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return out
}
