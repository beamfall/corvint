// SPDX-License-Identifier: Apache-2.0

package docsenvelope

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/versionline"
)

// CMS-V0-002: two executables agree when their versions are equal and, when
// both carry a stamped commit, the commits are equal; a release never agrees
// with a development build, and a different commit of one version is a mismatch.
func TestCMSV0002MismatchComparesVersionThenStampedCommit(t *testing.T) {
	commitA, commitB := strings.Repeat("a", 40), strings.Repeat("b", 40)
	core := versionline.Line{Name: "Corvint", Version: "1.0.0-rc.3", Build: "183", Commit: commitA}
	cases := []struct {
		name  string
		other versionline.Line
		want  string
	}{
		{"same release", versionline.Line{Name: "corvint-docs", Version: "1.0.0-rc.3", Build: "183", Commit: commitA}, ""},
		{"other version", versionline.Line{Name: "corvint-docs", Version: "1.0.1"}, "corvint-docs is version 1.0.1, Corvint is version 1.0.0-rc.3"},
		{"development build", versionline.Line{Name: "corvint-docs", Version: "1.0.0-rc.3.dev"}, "corvint-docs is version 1.0.0-rc.3.dev, Corvint is version 1.0.0-rc.3"},
		{"other commit", versionline.Line{Name: "corvint-docs", Version: "1.0.0-rc.3", Build: "184", Commit: commitB}, "corvint-docs was built from commit " + commitB + ", Corvint from commit " + commitA},
		{"unstamped sibling of one version", versionline.Line{Name: "corvint-docs", Version: "1.0.0-rc.3"}, ""},
	}
	for _, test := range cases {
		if got := Mismatch(core, test.other); got != test.want {
			t.Errorf("%s: Mismatch = %q, want %q", test.name, got, test.want)
		}
	}
	dev := versionline.Line{Name: "Corvint", Version: "1.0.0-rc.3.dev", Commit: commitA}
	if got := Mismatch(dev, versionline.Line{Name: "corvint-docs", Version: "1.0.0-rc.3.dev"}); got != "" {
		t.Errorf("two development builds of one version disagree: %q", got)
	}
}

// ParseVersionLine reads both the PUB-V0-053 form and the plain unstamped companion form.
func TestCMSV0002ParseVersionLineReadsBothForms(t *testing.T) {
	line, err := ParseVersionLine("corvint-docs 1.0.0-rc.3 (build 183, commit " + strings.Repeat("c", 40) + ")\n")
	if err != nil || line != (versionline.Line{Name: "corvint-docs", Version: "1.0.0-rc.3", Build: "183", Commit: strings.Repeat("c", 40)}) {
		t.Fatalf("stamped line: %#v %v", line, err)
	}
	line, err = ParseVersionLine("corvint-docs 1.0.0-rc.3.dev\n")
	if err != nil || line != (versionline.Line{Name: "corvint-docs", Version: "1.0.0-rc.3.dev"}) {
		t.Fatalf("plain line: %#v %v", line, err)
	}
	for _, bad := range []string{"", "corvint-docs", "one two three", "corvint-docs 1.0.0 (build x)", "a b\nc d"} {
		if _, err := ParseVersionLine(bad); err == nil {
			t.Errorf("%q parsed as a version line", bad)
		}
	}
}

// ProbeVersion runs the sibling's --version alone and bounds what it reads;
// CheckBeside ignores a sibling that is not beside the running executable.
func TestCMSV0002ProbeVersionAndCheckBeside(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	dir := t.TempDir()
	sibling := filepath.Join(dir, "corvint-docs")
	if err := execfile.WriteFile(sibling, []byte("#!/bin/sh\n[ \"$1\" = --version ] || exit 9\nprintf 'corvint-docs 9.9.9\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	line, err := ProbeVersion(context.Background(), sibling)
	if err != nil || line.Version != "9.9.9" {
		t.Fatalf("probe: %#v %v", line, err)
	}
	self := versionline.Line{Name: "Corvint", Version: "1.0.0"}
	if Beside(sibling) {
		t.Fatal("a temporary directory counts as beside the test executable")
	}
	if reason := CheckBeside(context.Background(), sibling, self); reason != "" {
		t.Fatalf("a sibling on PATH, not beside, was checked: %q", reason)
	}
	if err := execfile.WriteFile(sibling, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ProbeVersion(context.Background(), sibling); err == nil {
		t.Fatal("a failing --version probed as a version")
	}
	if err := execfile.WriteFile(sibling, []byte("#!/bin/sh\nhead -c 100000 /dev/zero | tr '\\0' x\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ProbeVersion(context.Background(), sibling); err == nil {
		t.Fatal("an unbounded --version probed as a version")
	}
}
