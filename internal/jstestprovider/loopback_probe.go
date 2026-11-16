package jstestprovider

import (
	"context"
	"errors"
	"net/url"
)

// LoopbackGet performs one GET of address within one second and returns its
// status; without followRedirects a redirect's own status is returned.
type LoopbackGet func(ctx context.Context, address string, followRedirects bool) (int, error)

// errNoLoopbackGet fails a probe closed in a binary that never imported the client.
var errNoLoopbackGet = errors.New("no loopback HTTP client is linked: import internal/jstestprovider/loopbackhttp")

// probeAddress refuses a probe before its first GET when no client is linked
// or address cannot form a request, as http.NewRequest would.
func probeAddress(address string, loopbackGet LoopbackGet) error {
	if loopbackGet == nil {
		return errNoLoopbackGet
	}
	_, err := url.Parse(address)
	return err
}
