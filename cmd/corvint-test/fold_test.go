package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// foldGolden is one group's recorded old-invocation behaviour (CTF-V0-002): the goldens are
// written only from the old companion binary, then every invocation is compared to them.
type foldGolden struct {
	Schema  string          `json:"schema"`
	Group   string          `json:"group"`
	Old     string          `json:"old"`
	New     string          `json:"new"`
	Request json.RawMessage `json:"request"`
	Cases   []foldCase      `json:"cases"`
}

// foldCase is one invocation. Text marks a case whose stdout and stderr are diagnostics that name
// the invoking program; only those substitute the new invocation for the old name. Output
// documents are compared byte for byte.
type foldCase struct {
	Name   string   `json:"name"`
	Args   []string `json:"args"`
	Text   bool     `json:"text,omitempty"`
	Stdin  string   `json:"stdin,omitempty"`
	Exit   int      `json:"exit"`
	Stdout string   `json:"stdout"`
	Stderr string   `json:"stderr"`
}

// foldDir stands for the per-run directory holding the case request file; a case whose stdin is
// foldRequest reads the golden's request document on standard input.
const (
	foldDir     = "@DIR@"
	foldRequest = "@REQUEST@"
)

func buildCompanion(t *testing.T, pkg, name string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), name)
	if out, err := exec.Command("go", "build", "-o", bin, pkg).CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", pkg, err, out)
	}
	return bin
}

func runFoldCase(t *testing.T, argv []string, c foldCase, dir string, request []byte) foldCase {
	t.Helper()
	args := make([]string, 0, len(argv)+len(c.Args))
	args = append(args, argv[1:]...)
	for _, a := range c.Args {
		args = append(args, strings.ReplaceAll(a, foldDir, dir))
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(argv[0], args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if c.Stdin == foldRequest {
		cmd.Stdin = bytes.NewReader(request)
	} else {
		cmd.Stdin = strings.NewReader(c.Stdin)
	}
	got := foldCase{Name: c.Name, Args: c.Args, Text: c.Text, Stdin: c.Stdin}
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("%s: %v", c.Name, err)
		}
		got.Exit = ee.ExitCode()
	}
	got.Stdout = strings.ReplaceAll(stdout.String(), dir, foldDir)
	got.Stderr = strings.ReplaceAll(stderr.String(), dir, foldDir)
	return got
}

// CTF-V0-002, CTF-V0-003: the runner group's old invocation and `corvint-test runner` reproduce
// the goldens recorded from the pre-move `corvint-test-runner`: exit status, output documents byte
// for byte, and diagnostics with the invocation name substituted.
func TestCTFV0002RunnerFoldGoldens(t *testing.T) {
	foldGoldens(t, "runner", "../corvint-test-runner")
}

// foldGoldens compares every invocation of a group with testdata/fold/<group>.json. Set
// CTF_UPDATE_GOLDEN=1 to rewrite the goldens from the old binary only (CTF-V0-002).
func foldGoldens(t *testing.T, group, oldPkg string) {
	path := filepath.Join("testdata", "fold", group+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var g foldGolden
	if err := json.Unmarshal(data, &g); err != nil || g.Schema != "corvint-test-fold-golden/0" || g.Group != group || len(g.Cases) == 0 {
		t.Fatalf("golden %s: %v", path, err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "req.json"), g.Request, 0o600); err != nil {
		t.Fatal(err)
	}
	old := []string{buildCompanion(t, oldPkg, g.Old)}
	if os.Getenv("CTF_UPDATE_GOLDEN") == "1" {
		for i, c := range g.Cases {
			g.Cases[i] = runFoldCase(t, old, c, dir, g.Request)
		}
		out, err := json.MarshalIndent(g, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	newName := strings.TrimPrefix(g.New, "corvint-test ")
	for _, inv := range []struct {
		name string
		argv []string
	}{{g.Old, old}, {g.New, []string{buildCompanion(t, ".", "corvint-test"), newName}}} {
		for _, c := range g.Cases {
			want := c
			if c.Text {
				want.Stdout = strings.ReplaceAll(c.Stdout, g.Old, inv.name)
				want.Stderr = strings.ReplaceAll(c.Stderr, g.Old, inv.name)
			}
			if got := runFoldCase(t, inv.argv, c, dir, g.Request); got.Exit != want.Exit || got.Stdout != want.Stdout || got.Stderr != want.Stderr {
				t.Errorf("%s %s:\n got exit %d stdout %q stderr %q\nwant exit %d stdout %q stderr %q",
					inv.name, c.Name, got.Exit, got.Stdout, got.Stderr, want.Exit, want.Stdout, want.Stderr)
			}
		}
	}
}

// CTF-V0-007, CTF-V0-010: the folded runner entry is not linked into core `corvint`, the group
// is named in the companion usage, and its sources never use the bare `corvint-test:` prefix of
// core `corvint test` diagnostics nor name `corvint` as an executable.
func TestCTFV0007RunnerGroupBoundary(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "../corvint").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if strings.HasSuffix(dep, "/internal/testrunner/cli") {
			t.Fatalf("core corvint imports %s", dep)
		}
	}
	if !strings.Contains(usage, "corvint-test runner <command>") {
		t.Fatalf("usage does not name the runner group: %q", usage)
	}
	srcs, _ := filepath.Glob("../../internal/testrunner/cli/*.go")
	if len(srcs) == 0 {
		t.Fatal("no runner group sources")
	}
	corvintArgv := regexp.MustCompile(`"corvint"|"corvint\s|/corvint"`)
	for _, p := range srcs {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if corvintArgv.Match(data) || bytes.Contains(data, []byte(`"corvint-test:`)) {
			t.Fatalf("%s names the core executable or its diagnostic prefix", p)
		}
	}
}
