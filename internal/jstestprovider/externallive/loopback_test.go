//go:build darwin || linux

package externallive_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"

	// The live external runs probe readiness through the client the E2E
	// binaries link (MTV-V0-950); without it every probe fails closed with
	// server-not-ready before Playwright starts (V1-2390).
	_ "github.com/Beamfall/corvint/internal/jstestprovider/loopbackhttp"
)

// TestExternalLiveLinksLoopbackClient runs no Playwright: an external-server
// run whose readiness URL is up must get past the readiness probe, so it can
// never fail with the unlinked-client refusal.
func TestExternalLiveLinksLoopbackClient(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "playwright.config.cjs")
	spec := filepath.Join(root, "a.spec.cjs")
	for _, path := range []string{config, spec} {
		if err := os.WriteFile(path, []byte("module.exports = {};\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	receipt, err := jstestprovider.RunE2E(context.Background(), jstestprovider.E2EConfig{
		Config: jstestprovider.Config{
			Dir: root, ConfigFile: config, TestFiles: []string{spec},
			RunnerName: "playwright", RunnerVersion: "1.64.0", Timeout: 10 * time.Second,
		},
		ExternalServer: true, AppIdentity: "loopback-link", ServerReadyURL: server.URL,
		TestArgv: []string{"a.spec.cjs"},
	})
	if receipt.Infrastructure != nil && strings.Contains(receipt.Infrastructure.Detail, "no loopback HTTP client") {
		t.Fatalf("externallive links no loopback client: %+v", receipt.Infrastructure)
	}
	if err != nil && strings.Contains(err.Error(), "no loopback HTTP client") {
		t.Fatalf("externallive links no loopback client: %v", err)
	}
}
