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

// testValidityToolNetworkImporters names every module package in the MCP
// test-validity tool path's closure that imports a connection-making network
// package, with the reason it stays unreachable from the tool (MTV-V0-010,
// V1-1781). The guard lives here because the bridge package's tests run
// under a narrow declared read scope that cannot list the closure.
var testValidityToolNetworkImporters = map[string]string{
	"github.com/Beamfall/corvint/internal/jstestprovider": "RunE2E callers are outside the tool closure",
}

// moduleImporters lists the module packages in target's import closure that
// directly import a package matched by imported, sorted.
func moduleImporters(t *testing.T, target string, imported func(string) bool) []string {
	t.Helper()
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
	command := exec.CommandContext(ctx, "go", "list", "-deps", "-f", "{{.ImportPath}}{{range .Imports}} {{.}}{{end}}", target)
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
		for _, path := range fields[1:] {
			if imported(path) {
				found = append(found, fields[0])
				break
			}
		}
	}
	sort.Strings(found)
	return found
}

func allowlistKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// TestCoreHTTPImportersAreAllowlisted fails when a package in the core
// binary's closure gains a direct net/http import that is not reviewed above,
// or when an allowlisted importer leaves the closure (so the list stays exact).
func TestCoreHTTPImportersAreAllowlisted(t *testing.T) {
	t.Parallel()
	found := moduleImporters(t, "./cmd/corvint", func(path string) bool { return path == "net/http" })
	if want := allowlistKeys(coreHTTPImporters); strings.Join(found, "\n") != strings.Join(want, "\n") {
		t.Fatalf("core net/http importers = %v, reviewed allowlist = %v; a new importer needs a reachability review (invariant 7)", found, want)
	}
}

// TestTestValidityToolNetworkImportersAreAllowlisted (MTV-V0-010): the MCP
// test-validity tool path's transitive closure reaches a connection-making
// network package only through reviewed importers. Parsing-only packages
// (net/url, net/netip, net/mail, net/textproto) are not network access.
func TestTestValidityToolNetworkImportersAreAllowlisted(t *testing.T) {
	t.Parallel()
	dialing := func(path string) bool {
		return path == "net" || path == "net/http" || strings.HasPrefix(path, "net/http/") || path == "net/rpc" || path == "net/smtp"
	}
	found := moduleImporters(t, "./internal/mcp/testvaliditybridge", dialing)
	if want := allowlistKeys(testValidityToolNetworkImporters); strings.Join(found, "\n") != strings.Join(want, "\n") {
		t.Fatalf("test-validity tool network importers = %v, reviewed allowlist = %v; a new importer needs a reachability review (MTV-V0-010)", found, want)
	}
}
