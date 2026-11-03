//go:build darwin || linux

package externallive_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"

	// The live external runs probe readiness through the client the E2E
	// binaries link (MTV-V0-950); without it every probe fails closed with
	// server-not-ready before Playwright starts (V1-2390).
	_ "github.com/Beamfall/corvint/internal/jstestprovider/loopbackhttp"
)

// TestExternalLiveLinksLoopbackClient runs no Playwright and starts no
// process: the readiness URL answers 503, so a linked client probes it until
// the short readiness limit and fails with the deadline. An unlinked client
// would refuse at once with the unlinked-client detail instead.
func TestExternalLiveLinksLoopbackClient(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "playwright.config.cjs")
	spec := filepath.Join(root, "a.spec.cjs")
	for _, path := range []string{config, spec} {
		if err := os.WriteFile(path, []byte("module.exports = {};\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var probes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		probes.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	receipt, err := jstestprovider.RunE2E(context.Background(), jstestprovider.E2EConfig{
		Config: jstestprovider.Config{
			Dir: root, ConfigFile: config, TestFiles: []string{spec},
			RunnerName: "playwright", RunnerVersion: "1.64.0", Timeout: 10 * time.Second,
		},
		ExternalServer: true, AppIdentity: "loopback-link", ServerReadyURL: server.URL,
		ServerReadyLimit: 200 * time.Millisecond, TestArgv: []string{"a.spec.cjs"},
	})
	if err != nil {
		t.Fatalf("RunE2E refused before the readiness probe: %v", err)
	}
	if receipt.Infrastructure == nil || receipt.Infrastructure.Reason != "server-not-ready" {
		t.Fatalf("want server-not-ready from the 503 readiness URL, got %+v", receipt.Infrastructure)
	}
	if strings.Contains(receipt.Infrastructure.Detail, "no loopback HTTP client") {
		t.Fatalf("externallive links no loopback client: %+v", receipt.Infrastructure)
	}
	if probes.Load() == 0 {
		t.Fatal("the readiness URL was never probed")
	}
}
