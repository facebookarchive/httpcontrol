go.httpcontrol [![Build Status](https://secure.travis-ci.org/daaku/go.httpcontrol.png)](http://travis-ci.org/daaku/go.httpcontrol)
==============

    import "github.com/daaku/go.httpcontrol"

Package httpcontrol allows a HTTP transport supporting connection pooling,
timeouts & retries.

This Transport is built on top of the standard library transport and augments it
with additional features. Using it can be as simple as:

    client := &http.Client{
        Transport: &httpcontrol.Transport{
            RequestTimeout: time.Minute,
            MaxTries: 3,
        },
    }
    res, err := client.Get("http://example.com/")

## Usage

#### type Logger

```go
type Logger interface {
	Print(v ...interface{})
}
```

For logging of unstructured data. This can be satisfied by log.Logger for
example.

#### type Stats

```go
type Stats struct {
	// The RoundTrip request.
	Request *http.Request

	// May not always be available.
	Response *http.Response

	// Will be set if the RoundTrip resulted in an error. Note that these are
	// RoundTrip errors and we do not care about the HTTP Status.
	Error error

	// Each duration is independent and the sum of all of them is the total
	// request duration. One or more durations may be zero.
	Duration struct {
		Header, Body time.Duration
	}

	Retry struct {
		// Will be incremented for each retry. The initial request will have this
		// set to 0, and the first retry to 1 and so on.
		Count uint

		// Will be set if and only if an error was encountered and a retry is
		// pending.
		Pending bool
	}
}
```

Stats for a RoundTrip.

#### func (*Stats) String

```go
func (s *Stats) String() string
```
A human readable representation often useful for debugging.

#### type Transport

```go
type Transport struct {

	// Proxy specifies a function to return a proxy for a given
	// *http.Request. If the function returns a non-nil error, the
	// request is aborted with the provided error.
	// If Proxy is nil or returns a nil *url.URL, no proxy is used.
	Proxy func(*http.Request) (*url.URL, error)

	// TLSClientConfig specifies the TLS configuration to use with
	// tls.Client. If nil, the default configuration is used.
	TLSClientConfig *tls.Config

	// DisableKeepAlives, if true, prevents re-use of TCP connections
	// between different HTTP requests.
	DisableKeepAlives bool

	// DisableCompression, if true, prevents the Transport from
	// requesting compression with an "Accept-Encoding: gzip"
	// request header when the Request contains no existing
	// Accept-Encoding value. If the Transport requests gzip on
	// its own and gets a gzipped response, it's transparently
	// decoded in the Response.Body. However, if the user
	// explicitly requested gzip it is not automatically
	// uncompressed.
	DisableCompression bool

	// MaxIdleConnsPerHost, if non-zero, controls the maximum idle
	// (keep-alive) to keep per-host.  If zero,
	// http.DefaultMaxIdleConnsPerHost is used.
	MaxIdleConnsPerHost int

	// Timeout is the maximum amount of time a dial will wait for
	// a connect to complete.
	//
	// The default is no timeout.
	//
	// With or without a timeout, the operating system may impose
	// its own earlier timeout. For instance, TCP timeouts are
	// often around 3 minutes.
	DialTimeout time.Duration

	// ResponseHeaderTimeout, if non-zero, specifies the amount of
	// time to wait for a server's response headers after fully
	// writing the request (including its body, if any). This
	// time does not include the time to read the response body.
	ResponseHeaderTimeout time.Duration

	// RequestTimeout, if non-zero, specifies the amount of time for the entire
	// request. This includes dialing (if necessary), the response header as well
	// as the entire body.
	RequestTimeout time.Duration

	// MaxTries, if non-zero, specifies the number of times we will retry on
	// failure. Retries are only attempted for temporary network errors or known
	// safe failures.
	MaxTries uint

	// Stats allows for capturing the result of a request and is useful for
	// monitoring purposes.
	Stats func(*Stats)

	// DebugLogger if provided will trigger logging of interesting events to aid
	// in debugging the request flow.
	DebugLogger Logger
}
```

Transport is an implementation of RoundTripper that supports http, https, and
http proxies (for either http or https with CONNECT). Transport can cache
connections for future re-use, provides various timeouts, retry logic and the
ability to track request statistics.

#### func  TransportFlag

```go
func TransportFlag(name string) *Transport
```
A Flag configured Transport instance.

#### func (*Transport) CancelRequest

```go
func (t *Transport) CancelRequest(req *http.Request)
```
CancelRequest cancels an in-flight request by closing its connection.

#### func (*Transport) Close

```go
func (t *Transport) Close() error
```
Close the Transport.

#### func (*Transport) RoundTrip

```go
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error)
```
RoundTrip implements the RoundTripper interface.

#### func (*Transport) Start

```go
func (t *Transport) Start() error
```
Start the Transport.
