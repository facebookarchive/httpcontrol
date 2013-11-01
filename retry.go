package httpcontrol

import (
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"strings"
	"time"
)

type Retriable func(*http.Request, *http.Response, error)

type Wait func(try uint) time.Duration

type RetryPolicy struct {
	Retriables []Retriable
}

// Proceed to the next filter
func (rp *RetryPolicy) next() Retriable {
}

func (rp *RetryPolicy) abort() {
}

func (rp *RetryPolicy) CanRetry(req *http.Request, resp *http.Response, err error) bool {
	if rp == nil || rp.Retriables == nil {
		return false
		if err != nil {
			return true
		} else {
			return false
		}
	}
	log.Println("Retrying")
	for _, retriable := range rp.Retriables {
		if !retriable(req, resp, err) {
			fmt.Println("False!")
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

func (rp *RetryPolicy) TemporaryError(req *http.Request, resp *http.Response, err error) {
	if err != nil {
		if neterr, ok := err.(net.Error); ok {
			if neterr.Temporary() {
				rp.next()
			}
		}
	}
}

func (rp *RetryPolicy) NetworkError(req *http.Request, resp *http.Response, err error) {
	if err != nil {
		s := err.Error()
		for _, suffix := range knownFailureSuffixes {
			if strings.HasSuffix(s, suffix) {
				rp.next()
			}
		}
	}
}

func (rp *RetryPolicy) RetryOnGet(req *http.Request, res *http.Response, err error) {
	if req != nil {
		if req.Method == "GET" {
			rp.next()
		}
	}
}

func (rp *RetryPolicy) RetryOn4xx(req *http.Request, res *http.Response, err error) {
	if res != nil {
		if 500 > res.StatusCode && res.StatusCode >= 400 {
			rp.next()
		}
	}
}

func (rp *RetryPolicy) AlwaysRetry(req *http.Request, resp *http.Response, err error) {
	rp.next()
}

func ExpBackoff(try uint) {
	time.Sleep(time.Second * time.Duration(math.Exp2(2)))
}

func LinearBackoff(try uint) {
	time.Sleep(time.Second * time.Duration(try))
}

func NoWait(uint) {
}
