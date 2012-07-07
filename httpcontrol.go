// Package httpcontrol allows for HTTP transport level control around
// timeouts and retries.
package httpcontrol

import (
	"github.com/daaku/go.stats"
	"net/http"
	"strings"
	"time"
)

type Control struct {
	Transport http.RoundTripper // The actual transport.
	Timeout   time.Duration     // Timeout before considering a failure.
	MaxTries  uint              // Max retries for known safe failures.
}

var knownFailureSuffixes = []string{
	"connection refused",
	"connection reset by peer.",
	"connection timed out.",
	"no such host.",
	"remote error: handshake failure",
	"unexpected EOF.",
}

func shouldRetryError(err error) bool {
	s := err.Error()
	for _, suffix := range knownFailureSuffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}

func (c *Control) tries(req *http.Request, tries uint) (*http.Response, error) {
	resp, err := c.Transport.RoundTrip(req)
	if err != nil && tries != 0 && req.Method == "GET" && shouldRetryError(err) {
		stats.Inc("httpcontrol retry")
		return c.tries(req, tries-1)
	}
	return resp, err
}

func (c *Control) RoundTrip(req *http.Request) (*http.Response, error) {
	return c.tries(req, c.MaxTries)
}
