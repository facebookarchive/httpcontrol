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

// Stats for a RoundTrip.
type Stats struct {
	// The RoundTrip request.
	Request *http.Request

	// May not always be available.
	Response *http.Response

	// Will be set if the RoundTrip resulted in an error.
	Error error

	// Each duration is independent and the sum of all of them is the total
	// request duration. One or more durations may be zero.
	Duration struct {
		Header, Body time.Duration
	}

	Retry struct {
		// Will be incremented for each retry. The initial request will have this set
		// to 0, and the first retry to 1 and so on.
		Count uint

		// Will be set if and only if an error was encountered and a retry is
		// pending.
		Pending bool
	}
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
	Stats                 func(*Stats)
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

func (t *Transport) tries(req *http.Request, try uint) (*http.Response, error) {
	startTime := time.Now()
	deadline := startTime.Add(t.RequestTimeout).UnixNano()
	item := &pqueue.Item{Value: req, Priority: deadline}
	t.pqMutex.Lock()
	heap.Push(&t.pq, item)
	t.pqMutex.Unlock()
	res, err := t.transport.RoundTrip(req)
	headerTime := time.Now()
	if err != nil {
		t.pqMutex.Lock()
		if item.Index != -1 {
			heap.Remove(&t.pq, item.Index)
		}
		t.pqMutex.Unlock()

		var stats *Stats
		if t.Stats != nil {
			stats = &Stats{
				Request:  req,
				Response: res,
				Error:    err,
			}
			stats.Duration.Header = headerTime.Sub(startTime)
			stats.Retry.Count = try
		}

		if try < t.MaxTries && req.Method == "GET" && shouldRetryError(err) {
			if t.Stats != nil {
				stats.Retry.Pending = true
				t.Stats(stats)
			}
			return t.tries(req, try+1)
		}

		if t.Stats != nil {
			t.Stats(stats)
		}
		return nil, err
	}

	res.Body = &bodyCloser{
		ReadCloser: res.Body,
		item:       item,
		transport:  t,
		startTime:  startTime,
		headerTime: headerTime,
	}
	return res, nil
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.Debug {
		log.Printf("httpcontrol: Request: %s", req.URL)
	}
	return t.tries(req, 0)
}

type bodyCloser struct {
	io.ReadCloser
	res        *http.Response
	item       *pqueue.Item
	transport  *Transport
	startTime  time.Time
	headerTime time.Time
}

func (b *bodyCloser) Close() error {
	err := b.ReadCloser.Close()
	closeTime := time.Now()
	b.transport.pqMutex.Lock()
	if b.item.Index != -1 {
		heap.Remove(&b.transport.pq, b.item.Index)
	}
	b.transport.pqMutex.Unlock()
	if b.transport.Stats != nil {
		stats := &Stats{
			Request:  b.res.Request,
			Response: b.res,
		}
		stats.Duration.Header = b.headerTime.Sub(b.startTime)
		stats.Duration.Body = closeTime.Sub(b.startTime) - stats.Duration.Header
		b.transport.Stats(stats)
	}
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
