package httpcontrol_test

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/daaku/go.httpcontrol"
)

var theAnswer = []byte("42")

func sleepHandler(timeout time.Duration) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(timeout)
			w.Write(theAnswer)
		})
}

func errorHandler(timeout time.Duration) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(timeout)
			w.WriteHeader(500)
			w.Write(theAnswer)
		})
}

func assertResponse(req *http.Response, t *testing.T) {
	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, theAnswer) {
		t.Fatalf(`did not find expected bytes "%s" instead found "%s"`, theAnswer, b)
	}
}

func call(f func() error, t *testing.T) {
	if err := f(); err != nil {
		t.Fatal(err)
	}
}

func TestOkWithDefaults(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(sleepHandler(time.Millisecond))
	defer server.Close()
	transport := &httpcontrol.Transport{}
	call(transport.Start, t)
	defer call(transport.Stop, t)
	client := &http.Client{Transport: transport}
	res, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	assertResponse(res, t)
}

func TestHttpError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(errorHandler(time.Millisecond))
	defer server.Close()
	transport := &httpcontrol.Transport{}
	call(transport.Start, t)
	defer call(transport.Stop, t)
	client := &http.Client{Transport: transport}
	res, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	assertResponse(res, t)
	if res.StatusCode != 500 {
		t.Fatal("was expecting 500 got %d", res.StatusCode)
	}
}

func TestDialTimeout(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(sleepHandler(time.Millisecond))
	server.Close()
	transport := &httpcontrol.Transport{}
	call(transport.Start, t)
	defer call(transport.Stop, t)
	client := &http.Client{Transport: transport}
	res, err := client.Get(server.URL)
	if err == nil {
		t.Fatal("was expecting an error")
	}
	if res != nil {
		t.Fatal("was expecting nil response")
	}
	if !strings.Contains(err.Error(), "dial") {
		t.Fatal("was expecting dial related error")
	}
}

func TestResponseHeaderTimeout(t *testing.T) {
	t.Parallel()
}

func TestResponseTimeout(t *testing.T) {
	t.Parallel()
}

func TestSafeRetry(t *testing.T) {
	t.Parallel()
}

func TestUnsafeRetry(t *testing.T) {
	t.Parallel()
}

func TestRedirect(t *testing.T) {
	t.Parallel()
}
