package jstestprovider

import (
	"context"
	"errors"
	"net/url"
)

// LoopbackGet performs one GET of address within one second and returns its
// status; without followRedirects a redirect's own status is returned.
type LoopbackGet func(ctx context.Context, address string, followRedirects bool) (int, error)

// loopbackGet is the client the readiness and preflight probes use. It lives
// in internal/jstestprovider/loopbackhttp so the core binary, which links this
// package for its report types, links no net/http client (V1-2185, MTV-V0-950).
var loopbackGet LoopbackGet

// errNoLoopbackGet fails a probe closed in a binary that never imported the client.
var errNoLoopbackGet = errors.New("no loopback HTTP client is linked: import internal/jstestprovider/loopbackhttp")

// RegisterLoopbackGet installs the probes' HTTP client; loopbackhttp calls it from init.
func RegisterLoopbackGet(get LoopbackGet) { loopbackGet = get }

// probeAddress refuses a probe before its first GET when no client is linked
// or address cannot form a request, as http.NewRequest would.
func probeAddress(address string) error {
	if loopbackGet == nil {
		return errNoLoopbackGet
	}
	_, err := url.Parse(address)
	return err
}
