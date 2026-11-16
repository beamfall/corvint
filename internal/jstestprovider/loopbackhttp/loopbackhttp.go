// Package loopbackhttp is the HTTP client behind jstestprovider's server
// readiness and preflight probes (V1-2185, MTV-V0-950). It is a separate package
// so the core binary, which links jstestprovider only for its report types,
// links no net/http client: a binary that runs E2E suites imports this package
// and passes Get in E2EConfig.
package loopbackhttp

import (
	"context"
	"net/http"
	"time"
)

// Get performs one GET of address within one second and returns its status.
// Without followRedirects a redirect's own status is returned.
func Get(ctx context.Context, address string, followRedirects bool) (int, error) {
	client := &http.Client{Timeout: time.Second}
	if !followRedirects {
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}
