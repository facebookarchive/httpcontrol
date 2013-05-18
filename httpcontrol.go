// Package httpcontrol allows for HTTP transport level control around
// timeouts and retries.
package httpcontrol

import (
	"container/heap"
	"crypto/tls"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/daaku/go.pqueue"
)

// Provides the ability to collect stats for interesting events.
type Stats interface {
	// This is called when a request is retried with the original request, the
	// failed response (if any), try count and error.
	Retry(req *http.Request, res *http.Response, try uint, err error)
}

// Look at http.Transport for the meaning of most of the fields here.
type Transport struct {
	Proxy                 func(*http.Request) (*url.URL, error)
	TLSClientConfig       *tls.Config
	DisableKeepAlives     bool
	DisableCompression    bool
	MaxIdleConnsPerHost   int
	DialTimeout           time.Duration
	ResponseHeaderTimeout time.Duration
	RequestTimeout        time.Duration
	MaxTries              uint // Max retries for known safe failures.
	Stats                 Stats
	Debug                 bool // Verbose logging of request & response
	transport             *http.Transport
	closeMonitor          chan bool
	pqMutex               sync.Mutex
	pq                    pqueue.PriorityQueue
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

func (t *Transport) tries(req *http.Request, try uint) (*http.Response, error) {
	resp, err := t.transport.RoundTrip(req)
	if err != nil && try < t.MaxTries && req.Method == "GET" && shouldRetryError(err) {
		if t.Stats != nil {
			t.Stats.Retry(req, resp, try, err)
		}
		return t.tries(req, try+1)
	}
	return resp, err
}

// Start the Transport.
func (t *Transport) Start() error {
	if t.Debug {
		log.Println("httpcontrol: Start")
	}
	dialer := &net.Dialer{Timeout: t.DialTimeout}
	t.transport = &http.Transport{
		Dial:                dialer.Dial,
		Proxy:               t.Proxy,
		TLSClientConfig:     t.TLSClientConfig,
		DisableKeepAlives:   t.DisableKeepAlives,
		DisableCompression:  t.DisableCompression,
		MaxIdleConnsPerHost: t.MaxIdleConnsPerHost,
	}
	t.closeMonitor = make(chan bool)
	t.pq = pqueue.New(16)
	go t.monitor()
	return nil
}

// Close the Transport.
func (t *Transport) Close() error {
	t.transport.CloseIdleConnections()
	t.closeMonitor <- true
	<-t.closeMonitor
	if t.Debug {
		log.Println("httpcontrol: Close")
	}
	return nil
}

func (t *Transport) monitor() {
	ticker := time.NewTicker(25 * time.Millisecond)
	for {
		select {
		case <-t.closeMonitor:
			ticker.Stop()
			close(t.closeMonitor)
			return
		case n := <-ticker.C:
			now := n.UnixNano()
			for {
				t.pqMutex.Lock()
				item, _ := t.pq.PeekAndShift(now)
				t.pqMutex.Unlock()

				if item == nil {
					break
				}

				req := item.Value.(*http.Request)
				if t.Debug {
					log.Printf("httpcontrol: Request Timeout: %s", req.URL)
				}
				t.CancelRequest(req)
			}
		}
	}
}

func (t *Transport) CancelRequest(req *http.Request) {
	t.transport.CancelRequest(req)
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.Debug {
		log.Printf("httpcontrol: Request: %s", req.URL)
	}
	deadline := time.Now().Add(t.RequestTimeout).UnixNano()
	item := &pqueue.Item{Value: req, Priority: deadline}
	t.pqMutex.Lock()
	heap.Push(&t.pq, item)
	t.pqMutex.Unlock()
	res, err := t.tries(req, 0)
	if err != nil {
		t.pqMutex.Lock()
		if item.Index != -1 {
			heap.Remove(&t.pq, item.Index)
		}
		t.pqMutex.Unlock()
		return nil, err
	}
	res.Body = &bodyCloser{
		ReadCloser: res.Body,
		item:       item,
		transport:  t,
	}
	return res, nil
}

type bodyCloser struct {
	io.ReadCloser
	item      *pqueue.Item
	transport *Transport
}

func (b *bodyCloser) Close() error {
	err := b.ReadCloser.Close()
	b.transport.pqMutex.Lock()
	if b.item.Index != -1 {
		heap.Remove(&b.transport.pq, b.item.Index)
	}
	b.transport.pqMutex.Unlock()
	return err
}

// A Flag configured Transport instance.
func TransportFlag(name string) *Transport {
	t := &Transport{TLSClientConfig: &tls.Config{}}
	flag.BoolVar(
		&t.TLSClientConfig.InsecureSkipVerify,
		name+".insecure-tls",
		false,
		name+" skip tls certificate verification",
	)
	flag.BoolVar(
		&t.DisableKeepAlives,
		name+".disable-keepalive",
		false,
		name+" disable keep-alives",
	)
	flag.BoolVar(
		&t.DisableCompression,
		name+".disable-compression",
		false,
		name+" disable compression",
	)
	flag.IntVar(
		&t.MaxIdleConnsPerHost,
		name+".max-idle-conns-per-host",
		http.DefaultMaxIdleConnsPerHost,
		name+" max idle connections per host",
	)
	flag.DurationVar(
		&t.DialTimeout,
		name+".dial-timeout",
		2*time.Second,
		name+" dial timeout",
	)
	flag.DurationVar(
		&t.ResponseHeaderTimeout,
		name+".response-header-timeout",
		3*time.Second,
		name+" response header timeout",
	)
	flag.DurationVar(
		&t.RequestTimeout,
		name+".request-timeout",
		30*time.Second,
		name+" request timeout",
	)
	flag.UintVar(
		&t.MaxTries,
		name+".max-tries",
		0,
		name+" max retries for known safe failures",
	)
	flag.BoolVar(
		&t.Debug,
		name+".debug",
		false,
		name+" debug logging",
	)
	return t
}
