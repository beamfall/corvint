// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// fakeDocsApp puts a corvint-docs stand-in on PATH that records its argument vector in the file
// CORVINT_TEST_CAPTURE names, echoes stdin to stdout after a marker, writes one stderr line and
// exits 3. PATH holds nothing else, so an unforwarded invocation cannot reach any real binary.
func fakeDocsApp(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	dir := t.TempDir()
	// Builtins only: PATH holds nothing but this directory.
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$CORVINT_TEST_CAPTURE\"\nprintf 'forwarded:'\nwhile IFS= read -r line || [ -n \"$line\" ]; do printf '%s' \"$line\"; done\nprintf 'app stderr\\n' >&2\nexit 3\n"
	if err := execfile.WriteFile(filepath.Join(dir, "corvint-docs"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(dir, "capture")
	t.Setenv("CORVINT_TEST_CAPTURE", capture)
	t.Setenv("PATH", dir)
	return capture
}

// CMS-V0-003: `corvint docs ...` and `--corpus=FILE` reads are exec-only shims onto corvint-docs
// with the argument vector, stdin, stdout, stderr and exit status passed through unchanged.
func TestDocsShimForwardsToCorvintDocs(t *testing.T) {
	capture := fakeDocsApp(t)
	for _, args := range [][]string{
		{"docs", "corpus", "info", "--artifact", "a.json"},
		{"--root", "/some/where", "docs", "draft", "--source", "o.md", "--package", "p"},
		{"--root=/some/where", "docs"},
		{"--root", "r", "impact", "--corpus=corpus.json", "cache/demux.go"},
		{"affected", "--corpus=corpus.json", "--corpus-depth=2"},
		{"impact", "--corpus-depth", "2"},
	} {
		_ = os.Remove(capture)
		var stdout, stderr bytes.Buffer
		code := run(args, strings.NewReader("stdin bytes"), &stdout, &stderr)
		recorded, err := os.ReadFile(capture)
		if err != nil {
			t.Fatalf("%q was not forwarded: exit %d stderr %q", args, code, &stderr)
		}
		if got, want := string(recorded), strings.Join(args, "\n")+"\n"; got != want {
			t.Errorf("%q forwarded as %q", args, got)
		}
		if code != 3 || stdout.String() != "forwarded:stdin bytes" || stderr.String() != "app stderr\n" {
			t.Errorf("%q: exit %d stdout %q stderr %q; want the app's own", args, code, &stdout, &stderr)
		}
	}
}

// Help for the docs group is the shim's own page and never starts the app; an unvalued --corpus
// stays a native flag; mcp owns every argument after its verb (AHI-373).
func TestDocsShimDoesNotForwardHelpOrNativeFlags(t *testing.T) {
	capture := fakeDocsApp(t)
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"docs", "--help"}, docsHelp},
		{[]string{"docs", "corpus", "search", "--help"}, docsHelp},
		{[]string{"help", "docs"}, docsHelp},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(test.args, &forbiddenHelpReader{}, &stdout, &stderr); code != 0 || stdout.String() != helpText("docs") || !strings.HasPrefix(stdout.String(), test.want) {
			t.Errorf("%q: exit %d stdout matches=%v stderr %q", test.args, code, stdout.String() == helpText("docs"), &stderr)
		}
	}
	for _, args := range [][]string{{"impact", "--corpus", "x.json"}, {"mcp", "--corpus=x.json"}} {
		var stdout, stderr bytes.Buffer
		run(args, strings.NewReader(""), &stdout, &stderr)
		if _, err := os.Stat(capture); err == nil {
			t.Errorf("%q reached corvint-docs", args)
		}
	}
}

// A missing corvint-docs is the named NOT_INSTALLED gap, exit 2, on both forwarded shapes, and
// names the form the user ran and the command that installs the core set (CMS-V0-930).
func TestDocsShimReportsNotInstalled(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, test := range []struct {
		args  []string
		names string
	}{
		{[]string{"docs", "corpus", "info"}, "corvint docs is served"},
		{[]string{"impact", "--corpus=corpus.json", "a.go"}, "corvint --corpus is served"},
		{[]string{"affected", "--corpus-depth", "2"}, "corvint --corpus-depth is served"},
		{[]string{"flows", "map"}, "corvint flows is served"},
		{[]string{"how", "x"}, "corvint how is served"},
	} {
		args := test.args
		var stdout, stderr bytes.Buffer
		code := run(args, strings.NewReader(""), &stdout, &stderr)
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code": "not-installed"`) || !strings.Contains(stderr.String(), "NOT_INSTALLED") || !strings.Contains(stderr.String(), "corvint-docs") || !strings.Contains(stderr.String(), test.names) || !strings.Contains(stderr.String(), "corvint-update apply --allow-network --component core --bin-dir") {
			t.Errorf("%q: exit %d stdout %q stderr %q", args, code, &stdout, &stderr)
		}
	}
}

// DIRTY-CACHE-016: the docs group refuses --require-clean in the shim, before it looks for
// corvint-docs, and names the flag; an option value after `--` is not the flag.
func TestDocsShimRefusesRequireClean(t *testing.T) {
	capture := fakeDocsApp(t)
	for _, args := range [][]string{{"docs", "nav", "--require-clean"}, {"--root", "r", "docs", "corpus", "info", "--require-clean=true"}} {
		var stdout, stderr bytes.Buffer
		code := run(args, strings.NewReader(""), &stdout, &stderr)
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "--require-clean") {
			t.Errorf("%q: exit %d stdout %q stderr %q", args, code, &stdout, &stderr)
		}
		if _, err := os.Stat(capture); err == nil {
			t.Errorf("%q reached corvint-docs", args)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"docs", "nav", "--", "--require-clean"}, strings.NewReader(""), &stdout, &stderr); code != 3 {
		t.Errorf("an operand after -- was refused: exit %d stderr %q", code, &stderr)
	}
}

// A corvint-docs killed by a signal exits 128+N with a diagnostic, not 255.
func TestDocsShimMapsSignalExit(t *testing.T) {
	capture := fakeDocsApp(t)
	script := filepath.Join(filepath.Dir(capture), "corvint-docs")
	if err := execfile.WriteFile(script, []byte("#!/bin/sh\nkill -TERM $$\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"docs", "nav"}, strings.NewReader(""), &stdout, &stderr); code != 128+15 || !strings.Contains(stderr.String(), "killed by signal") {
		t.Errorf("exit %d stderr %q; want 143 and a signal diagnostic", code, &stderr)
	}
}

// PRB-V0-005: a profile knob without {pid} names the file this corvint process owns, so the
// forwarded corvint-docs does not inherit it; a knob with the token, and every other variable,
// pass through unchanged.
func TestPRBV0005ForwardedDocsChildKeepsOnlyPIDKnobs(t *testing.T) {
	capture := fakeDocsApp(t)
	script := filepath.Join(filepath.Dir(capture), "corvint-docs")
	body := "#!/bin/sh\nprintf '%s|%s|%s\\n' \"${CORVINT_CPUPROFILE-unset}\" \"${CORVINT_MEMPROFILE-unset}\" \"$CORVINT_TEST_KEEP\" > \"$CORVINT_TEST_CAPTURE\"\n"
	if err := execfile.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CORVINT_TEST_KEEP", "kept")
	for _, test := range []struct{ cpu, mem, want string }{
		{"/profiles/cpu.prof", "/profiles/mem-{pid}.prof", "unset|/profiles/mem-{pid}.prof|kept\n"},
		{"/profiles/cpu-{pid}.prof", "/profiles/mem.prof", "/profiles/cpu-{pid}.prof|unset|kept\n"},
	} {
		t.Setenv("CORVINT_CPUPROFILE", test.cpu)
		t.Setenv("CORVINT_MEMPROFILE", test.mem)
		var stdout, stderr bytes.Buffer
		if code := run([]string{"docs", "nav"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("exit %d stderr %q", code, &stderr)
		}
		if recorded, err := os.ReadFile(capture); err != nil || string(recorded) != test.want {
			t.Errorf("child environment %q (%v); want %q", recorded, err, test.want)
		}
	}
}

// besideDocsApp puts a corvint-docs stand-in beside the running test executable, where Locate
// looks first, printing versionLine for --version and recording any other argument vector; PATH
// holds nothing. It is removed when the test ends so the PATH-based tests see no sibling.
func besideDocsApp(t *testing.T, versionLine string) string {
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
	sibling := filepath.Join(filepath.Dir(self), "corvint-docs")
	if _, err := os.Lstat(sibling); err == nil {
		t.Fatalf("%s already exists", sibling)
	}
	capture := filepath.Join(t.TempDir(), "capture")
	script := "#!/bin/sh\nif [ \"$1\" = --version ]; then printf '%s\\n' '" + versionLine + "'; exit 0; fi\nprintf '%s\\n' \"$@\" > \"$CORVINT_TEST_CAPTURE\"\nexit 0\n"
	if err := execfile.WriteFile(sibling, []byte(script), 0o755); err != nil {
		t.Skipf("cannot write beside the test executable: %v", err)
	}
	t.Cleanup(func() { os.Remove(sibling) })
	t.Setenv("CORVINT_TEST_CAPTURE", capture)
	t.Setenv("PATH", t.TempDir())
	return capture
}

// CMS-V0-002: a corvint-docs installed beside corvint at another version is refused with the
// named version-mismatch code before anything is forwarded; one that states this build's version
// is forwarded as before.
func TestDocsShimRefusesSiblingAtAnotherVersion(t *testing.T) {
	capture := besideDocsApp(t, "corvint-docs 9.9.9 (build 1, commit "+strings.Repeat("9", 40)+")")
	var stdout, stderr bytes.Buffer
	code := run([]string{"docs", "nav"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code": "version-mismatch"`) || !strings.Contains(stderr.String(), "corvint-docs is version 9.9.9") || !strings.Contains(stderr.String(), "one release: install the core set with: corvint-update apply --allow-network --component core") {
		t.Fatalf("exit %d stdout %q stderr %q", code, &stdout, &stderr)
	}
	if _, err := os.Stat(capture); err == nil {
		t.Fatal("a mismatched corvint-docs was still run")
	}
}

func TestDocsShimForwardsToSiblingAtThisVersion(t *testing.T) {
	self := selfVersionLine()
	if self.Version == "" {
		t.Fatal("this executable has no version line")
	}
	capture := besideDocsApp(t, "corvint-docs "+self.Version)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"docs", "nav"}, strings.NewReader(""), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit %d stderr %q", code, &stderr)
	}
	if raw, err := os.ReadFile(capture); err != nil || string(raw) != "docs\nnav\n" {
		t.Fatalf("forwarded argv %q, %v", raw, err)
	}
}
