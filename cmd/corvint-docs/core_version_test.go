// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// besideCore puts a corvint stand-in beside the running test executable, where Locate looks
// first, printing versionLine for --version and recording any other argument vector; it is
// removed when the test ends.
func besideCore(t *testing.T, versionLine string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	sibling := filepath.Join(filepath.Dir(self), "corvint")
	if _, err := os.Lstat(sibling); err == nil {
		t.Fatalf("%s already exists", sibling)
	}
	capture := filepath.Join(t.TempDir(), "capture")
	script := "#!/bin/sh\nif [ \"$1\" = --version ]; then printf '%s\\n' '" + versionLine + "'; exit 0; fi\nprintf '%s\\n' \"$@\" > \"$CORVINT_TEST_CAPTURE\"\nprintf '{\"ok\": true}\\n'\n"
	if err := os.WriteFile(sibling, []byte(script), 0o755); err != nil {
		t.Skipf("cannot write beside the test executable: %v", err)
	}
	t.Cleanup(func() { os.Remove(sibling) })
	t.Setenv("CORVINT_TEST_CAPTURE", capture)
	previous := locateCore
	locateCore = func() (string, error) { return sibling, nil }
	t.Cleanup(func() { locateCore = previous })
	return capture
}

// CMS-V0-002: a corvint installed beside corvint-docs at another version is refused with the
// named version-mismatch code before the core read runs; one at this build's version runs.
func TestCoreReadRefusesSiblingCoreAtAnotherVersion(t *testing.T) {
	capture := besideCore(t, "Corvint 9.9.9 (build 1, commit "+strings.Repeat("9", 40)+")")
	var stdout, stderr bytes.Buffer
	code := runCoreRead(context.Background(), []string{"--root", t.TempDir(), "impact", "x.go"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code": "version-mismatch"`) || !strings.Contains(stderr.String(), "Corvint is version 9.9.9") {
		t.Fatalf("exit %d stdout %q stderr %q", code, &stdout, &stderr)
	}
	if _, err := os.Stat(capture); err == nil {
		t.Fatal("a mismatched corvint was still run")
	}
}

func TestCoreReadRunsSiblingCoreAtThisVersion(t *testing.T) {
	capture := besideCore(t, "Corvint "+versionSpec().Line().Version)
	var stdout, stderr bytes.Buffer
	if code := runCoreRead(context.Background(), []string{"impact", "x.go"}, strings.NewReader(""), &stdout, &stderr); code != 0 || stderr.Len() != 0 || stdout.String() != "{\"ok\": true}\n" {
		t.Fatalf("exit %d stdout %q stderr %q", code, &stdout, &stderr)
	}
	if raw, err := os.ReadFile(capture); err != nil || string(raw) != "impact\nx.go\n" {
		t.Fatalf("forwarded argv %q, %v", raw, err)
	}
}
