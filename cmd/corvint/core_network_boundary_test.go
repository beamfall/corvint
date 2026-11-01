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
// https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-core-net-http-reachability.md).
var coreHTTPImporters = map[string]string{
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
// jstestprovider's E2E probe client moved to jstestprovider/loopbackhttp,
// which neither closure links (V1-2185, MTV-V0-950), so the set is empty.
var testValidityToolNetworkImporters = map[string]string{}

// networkGuardConfigs are the build configurations each closure guard lists
// explicitly, so a platform-only importer (a _unix.go or _windows.go file) is
// visible on every host and the result does not depend on the host platform.
var networkGuardConfigs = []string{"darwin/arm64", "linux/amd64", "windows/amd64"}

// coreHTTPImportersByConfig is the exact set of reviewed importers expected in
// each configuration. testrunner's net/http import is in
// device_cleanup_unix.go, so Windows builds do not link it.
var coreHTTPImportersByConfig = map[string][]string{
	"darwin/arm64":  {"github.com/Beamfall/corvint/internal/testrunner"},
	"linux/amd64":   {"github.com/Beamfall/corvint/internal/testrunner"},
	"windows/amd64": {},
}

var testValidityToolNetworkImportersByConfig = map[string][]string{
	"darwin/arm64":  {},
	"linux/amd64":   {},
	"windows/amd64": {},
}

// moduleImporters lists the module packages in target's import closure for
// config (GOOS/GOARCH) that directly import a package matched by imported,
// sorted.
func moduleImporters(t *testing.T, config, target string, imported func(string) bool) []string {
	t.Helper()
	goos, goarch, ok := strings.Cut(config, "/")
	if !ok {
		t.Fatalf("config %q is not GOOS/GOARCH", config)
	}
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
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off", "GOFLAGS=", "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
	output, err := goListOutput(command)
	if err != nil {
		t.Fatalf("go list %s for %s: %v", target, config, err)
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

// checkImporterConfigs compares each configuration's importers with its exact
// expected set and requires every expected importer to carry a review reason.
func checkImporterConfigs(t *testing.T, target, rule string, imported func(string) bool, reasons map[string]string, byConfig map[string][]string) {
	t.Helper()
	reviewed := map[string]bool{}
	for _, config := range networkGuardConfigs {
		want, ok := byConfig[config]
		if !ok {
			t.Fatalf("%s: no expectation for %s", rule, config)
		}
		want = append([]string(nil), want...)
		sort.Strings(want)
		for _, pkg := range want {
			if reasons[pkg] == "" {
				t.Fatalf("%s: %s expected for %s has no review reason", rule, pkg, config)
			}
			reviewed[pkg] = true
		}
		if found := moduleImporters(t, config, target, imported); strings.Join(found, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s: %s importers = %v, reviewed expectation = %v; a new importer needs a reachability review", rule, config, found, want)
		}
	}
	for _, pkg := range allowlistKeys(reasons) {
		if !reviewed[pkg] {
			t.Errorf("%s: reviewed importer %s is expected in no configuration; remove it", rule, pkg)
		}
	}
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
// or when an expected importer leaves a configuration's closure (so each
// per-configuration list stays exact).
func TestCoreHTTPImportersAreAllowlisted(t *testing.T) {
	t.Parallel()
	checkImporterConfigs(t, "./cmd/corvint", "invariant 7", func(path string) bool { return path == "net/http" }, coreHTTPImporters, coreHTTPImportersByConfig)
}

// TestTestValidityToolNetworkImportersAreAllowlisted (MTV-V0-010): the MCP
// test-validity tool path's transitive closure reaches a connection-making
// network package only through reviewed importers, in every guarded build
// configuration. Parsing-only packages (net/url, net/netip, net/mail,
// net/textproto) are not network access.
func TestTestValidityToolNetworkImportersAreAllowlisted(t *testing.T) {
	t.Parallel()
	dialing := func(path string) bool {
		return path == "net" || path == "net/http" || strings.HasPrefix(path, "net/http/") || path == "net/rpc" || path == "net/smtp"
	}
	checkImporterConfigs(t, "./internal/mcp/testvaliditybridge", "MTV-V0-010", dialing, testValidityToolNetworkImporters, testValidityToolNetworkImportersByConfig)
}
