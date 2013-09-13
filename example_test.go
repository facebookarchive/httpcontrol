package httpcontrol_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/daaku/go.httpcontrol"
)

func Example() {
	// This example doesn't use all available features, look at the API for
	// documentation on all options.
	transport := &httpcontrol.Transport{
		RequestTimeout: time.Minute,
		MaxTries:       3,
	}

	// The Transport needs to be started. This should be done once on application
	// startup.
	if err := transport.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// It can then be used as the Transport in a HTTP client (or used directly
	// via RoundTrip).
	client := &http.Client{Transport: transport}

	res, err := client.Get("http://graph.facebook.com/naitik?fields=name")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer res.Body.Close()

	// Just outputting the response for example purposes.
	if _, err := io.Copy(os.Stdout, res.Body); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// This isn't strictly necessary, but it will shutdown the background
	// goroutine monitoring for request timeouts.
	if err := transport.Close(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Output: {"name":"Naitik Shah","id":"5526183"}
}
