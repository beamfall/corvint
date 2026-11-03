package jstestprovider

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/testplatform"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// hostAbs returns the POSIX-absolute fixture path p as an absolute path under
// the running host's path rules. Receipts carry paths the reporter observed on
// the same host, and production checks them with filepath.IsAbs, so a fixture
// path must be host-absolute: p unchanged off Windows, p on the C: volume there
// (ASD-V0-2233).
func hostAbs(p string) string {
	if runtime.GOOS == "windows" {
		return "C:" + filepath.FromSlash(p)
	}
	return p
}

// hostBundledUse makes the bundled headless-shell executablePath of a project
// use fixture host-absolute while keeping its forward-slash qualified suffix,
// the exact shape qualifiedBundledPlaywrightBrowser checks. It is the identity
// off Windows (ASD-V0-2233).
func hostBundledUse(use string) string {
	if runtime.GOOS != "windows" {
		return use
	}
	return strings.ReplaceAll(use, `"executablePath":"/portable/cache/`, `"executablePath":"C:/portable/cache/`)
}

// skipWindowsProcessRefusal skips t on native Windows only when procgroup.Run,
// through which every jstestprovider subprocess (Git observation, attestation
// provider, runner) starts, returns its listed platform refusal (decision 0559;
// procgroup's TestRunPlatformRefusalIsTheSentinel covers the refusal). Off
// Windows it runs nothing and never skips (ASD-V0-2234).
func skipWindowsProcessRefusal(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		return
	}
	testplatform.SkipWindowsRefusal(t, procgroup.Run(context.Background(), procgroup.Spec{}).Err)
}

// requireFreshCurrent requires a complete freshness fixture to project
// CURRENT. On native Windows, unsupported by decision 0559, no dependency
// manifest can be complete: every path must equal its filepath.Clean form,
// which uses backslashes there, yet each file must lie below root+"/".
// Dependency observation also refuses (TestPTFV0DependencyManifestBound), so
// there it requires the projection to fail closed instead of certifying
// currency (ASD-V0-2235).
func requireFreshCurrent(t *testing.T, got, what string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		if got == testvalidity.FreshnessCurrent {
			t.Fatalf("%s: native Windows certified freshness currency (decision 0559 leaves it unsupported)", what)
		}
		return
	}
	if got != testvalidity.FreshnessCurrent {
		t.Fatalf("%s read %s, want %s", what, got, testvalidity.FreshnessCurrent)
	}
}

// ASD-V0-2235: on native Windows a complete freshness fixture fails closed,
// and its POSIX dependency manifest is not complete; off Windows the same
// fixture is CURRENT (TestPTFV0ObservedCurrency).
func TestASDV02235FreshnessFailsClosedOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows freshness refusal; TestPTFV0ObservedCurrency covers currency on supported hosts")
	}
	r := freshFixture(t)
	if got := ReceiptTestProjection(r, r.Tests[0]).Freshness.State; got == testvalidity.FreshnessCurrent {
		t.Fatalf("native Windows projected %s", got)
	}
	if freshManifestComplete(r.Freshness.Dependencies) {
		t.Fatal("native Windows admitted a POSIX dependency manifest")
	}
}
