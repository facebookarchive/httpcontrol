package httpcontrol

import (
	"math"
	"net"
	"net/http"
	"strings"
	"time"
)

type Retriable func(*http.Response, error) bool

type waitTime func(try uint) time.Duration

/*
type RetryPolicy interface {
	Retriable(*http.Response, error) bool
	Wait(uint)
}
*/

type RetryPolicy struct {
	retriables []Retriable
	/*
		wait       Wait
		Retriable  func(*http.Response, error) bool
	*/
}

func (rp *RetryPolicy) CanRetry(resp *http.Response, err error) bool {
	for _, retriable := range rp.retriables {
		if err != nil {
			if !retriable(nil, err) {
				return false
			}
		}
		if resp != nil {
			if !retriable(resp, err) {
				return false
			}
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
	if neterr, ok := err.(net.Error); ok {
		if neterr.Temporary() {
			return true
		}
	}
	return false
}

func NetworkError(req *http.Request, resp *http.Response, err error) bool {
	s := err.Error()
	for _, suffix := range knownFailureSuffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}

func RetryOnGet(req *http.Request, res *http.Response, err error) bool {
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
