package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// coreHTTPImporters names every module package in the core binary's import
// closure that imports net/http directly, with the reason its HTTP client
// stays unreachable from every core command (V1-1670, AGENTS.md invariant 7;
// docs/build-log/2026-10-21-core-net-http-reachability.md).
var coreHTTPImporters = map[string]string{
	// Loopback readiness probes in RunE2E/RunFreshE2E. Core links the package
	// for its report types only; RunE2E is called from changee2e,
	// playwrightminimize, testacceptance and corvint-js-test-provider, none of
	// which cmd/corvint links.
	"github.com/Beamfall/corvint/internal/jstestprovider": "RunE2E callers are outside core",
	// Loopback WebDriver session DELETE in device cleanup. Core reaches
	// testrunner.Execute only through liveverify/rbmutate, whose request is
	// always the rspec runner, so no invocation carries a DeviceCleanup step.
	"github.com/Beamfall/corvint/internal/testrunner": "device cleanup is mobile/android only",
}

// TestCoreHTTPImportersAreAllowlisted fails when a package in the core
// binary's closure gains a direct net/http import that is not reviewed above,
// or when an allowlisted importer leaves the closure (so the list stays exact).
func TestCoreHTTPImportersAreAllowlisted(t *testing.T) {
	t.Parallel()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "list", "-deps", "-f", "{{.ImportPath}}{{range .Imports}} {{.}}{{end}}", "./cmd/corvint")
	command.Dir = root
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off")
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	found := []string{}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.HasPrefix(fields[0], "github.com/Beamfall/corvint/") {
			continue
		}
		for _, imported := range fields[1:] {
			if imported == "net/http" {
				found = append(found, fields[0])
				break
			}
		}
	}
	sort.Strings(found)
	want := make([]string, 0, len(coreHTTPImporters))
	for path := range coreHTTPImporters {
		want = append(want, path)
	}
	sort.Strings(want)
	if strings.Join(found, "\n") != strings.Join(want, "\n") {
		t.Fatalf("core net/http importers = %v, reviewed allowlist = %v; a new importer needs a reachability review (invariant 7)", found, want)
	}
}
