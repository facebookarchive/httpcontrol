package httpcontrol

import (
	"math"
	"net"
	"net/http"
	"strings"
	"time"
)

type Retriable func(*http.Request, *http.Response, error) bool

type Wait func(try uint) time.Duration

var knownFailureSuffixes = []string{
	"connection refused",
	"connection reset by peer.",
	"connection timed out.",
	"no such host.",
	"remote error: handshake failure",
	"unexpected EOF.",
}

func shouldRetryDefault(req *http.Request, res *http.Response, err error) bool {
	if err == nil {
		return false
	}
	if req.Method != "GET" {
		return false
	}

	if neterr, ok := err.(net.Error); ok {
		if neterr.Temporary() {
			return true
		}
	}

	s := err.Error()
	for _, suffix := range knownFailureSuffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}

func RetryOn4xx(req *http.Request, res *http.Response, err error) bool {
	if res != nil {
		return 500 > res.StatusCode && res.StatusCode >= 400
	}
	return false
}

func ExpBackoff(try uint) {
	time.Sleep(time.Second * time.Duration(math.Exp2(2)))
}

func LinearBackoff(try uint) {
	time.Sleep(time.Second * time.Duration(try))
}

func NoWait(uint) {
}
