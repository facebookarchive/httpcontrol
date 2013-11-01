package httpcontrol

import (
	"math"
	"net"
	"net/http"
	"strings"
	"time"
)

type Retriable func(*http.Request, *http.Response, error) bool

type waitTime func(try uint) time.Duration

type RetryPolicy struct {
	retriables []Retriable
}

func (rp *RetryPolicy) CanRetry(req *http.Request, resp *http.Response, err error) bool {
	for _, retriable := range rp.retriables {
		if !retriable(req, resp, err) {
			return false
		}
	}
	return true
}

var knownFailureSuffixes = []string{
	"connection refused",
	"connection reset by peer.",
	"connection timed out.",
	"no such host.",
	"remote error: handshake failure",
	"unexpected EOF.",
}

func TemporaryError(req *http.Request, resp *http.Response, err error) bool {
	if err == nil {
		return true
	}
	if neterr, ok := err.(net.Error); ok {
		if neterr.Temporary() {
			return true
		}
	}
	return false
}

func NetworkError(req *http.Request, resp *http.Response, err error) bool {
	if err == nil {
		return true
	}
	s := err.Error()
	for _, suffix := range knownFailureSuffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}

func RetryOnGet(req *http.Request, res *http.Response, err error) bool {
	if req == nil {
		return true
	}
	if req.Method == "GET" {
		return true
	}
	return false
}

func ExpBackoff(try uint) {
	time.Sleep(time.Second * time.Duration(math.Exp2(2)))
}

func LinearBackoff(try uint) {
	time.Sleep(time.Second * time.Duration(try))
}
