// SPDX-License-Identifier: Apache-2.0

package authority

import (
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PUR-V0-1920 (V1-4261): this test binary was compiled from a checkout, so its guard names
// that checkout's common directory, exports it for children, and refuses it, before any lock
// file is opened. The live store is never locked here: only the pure guard is called on it.
func TestPURV01920_TestBinaryGuardsItsOwnCheckout(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	want, err := intent.ResolveCommonDir(wd)
	if err != nil {
		t.Skipf("package directory is not in a Git checkout: %v", err)
	}
	if got := testSourceCommonDir(); !sameDirectory(got, want) {
		t.Fatalf("test source common dir = %q, want this checkout's %q", got, want)
	}
	if got := os.Getenv(ForbidCommonDirEnv); got == "" {
		t.Fatalf("%s is unset: children of a test binary would not inherit the guard", ForbidCommonDirEnv)
	}
	if err := refuseForbiddenStore(want); wire.CodeOf(err) != wire.CodeOutOfScope {
		t.Fatalf("guard on the checkout's own store = %v, want OUT_OF_SCOPE", err)
	}
	if err := refuseForbiddenStore(t.TempDir()); err != nil {
		t.Fatalf("guard refused an unrelated directory: %v", err)
	}
}

func TestPURV01920_IsTestBinary(t *testing.T) {
	for argv0, want := range map[string]bool{
		"/tmp/go-build1/b001/authority.test":   true,
		`C:\go-build\b001\authority.test.exe`:  true,
		"/home/u/.local/bin/corvint-tasks":     false,
		"/usr/bin/test":                        false,
		"":                                     false,
		"/tmp/x/corvint-tasks.test-fixture.sh": false,
	} {
		if got := isTestBinary([]string{argv0, "-test.timeout=30m"}); got != want {
			t.Errorf("isTestBinary(%q) = %v, want %v", argv0, got, want)
		}
	}
	// PUR-V0-1920: a production binary may have either test-looking name.
	for _, name := range []string{"/tmp/corvint-tasks.test", "/tmp/corvint-tasks.test.exe"} {
		if isTestBinary([]string{name}) {
			t.Errorf("ordinary production binary %q classified as a test", name)
		}
	}
	if isTestBinary(nil) {
		t.Error("isTestBinary(nil) = true")
	}
}
