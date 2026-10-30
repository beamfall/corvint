package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testacceptance"
)

// foldGolden is one group's recorded old-invocation behaviour (CTF-V0-002): the goldens are
// written only from the old companion binary, then every invocation is compared to them.
type foldGolden struct {
	Schema  string          `json:"schema"`
	Group   string          `json:"group"`
	Old     string          `json:"old"`
	New     string          `json:"new"`
	Request json.RawMessage `json:"request"`
	// Fixture is a repository committed at @DIR@/repo before the cases run; the request names
	// its commit, tree and file digests as @COMMIT@, @TREE@ and @SHA256:<path>@ (group 4).
	Fixture map[string]string `json:"fixture,omitempty"`
	Cases   []foldCase        `json:"cases"`
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
// foldRequest reads the golden's request document on standard input. foldDigest stands for the
// request digest of a run whose request binds that directory's absolute roots (group 4 `plan`).
const (
	foldDir     = "@DIR@"
	foldRequest = "@REQUEST@"
	foldDigest  = "@DIGEST@"
)

// foldFixture commits g.Fixture at dir/repo with a fixed identity and date, then substitutes the
// request's placeholders.
func foldFixture(t *testing.T, g foldGolden, dir string) []byte {
	t.Helper()
	request := strings.ReplaceAll(string(g.Request), foldDir, dir)
	if len(g.Fixture) == 0 {
		return []byte(request)
	}
	repo := filepath.Join(dir, "repo")
	for rel, text := range g.Fixture {
		path := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o755); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(text))
		request = strings.ReplaceAll(request, "@SHA256:"+rel+"@", "sha256:"+hex.EncodeToString(sum[:]))
	}
	gitOut := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_AUTHOR_NAME=fold", "GIT_AUTHOR_EMAIL=fold@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
			"GIT_COMMITTER_NAME=fold", "GIT_COMMITTER_EMAIL=fold@example.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	gitOut("init", "-q")
	gitOut("add", "-A")
	gitOut("commit", "-q", "-m", "fold fixture")
	request = strings.ReplaceAll(request, "@COMMIT@", gitOut("rev-parse", "HEAD"))
	return []byte(strings.ReplaceAll(request, "@TREE@", gitOut("rev-parse", "HEAD^{tree}")))
}

func buildCompanion(t *testing.T, pkg, name string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), name)
	if out, err := exec.Command("go", "build", "-o", bin, pkg).CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", pkg, err, out)
	}
	return bin
}

func runFoldCase(t *testing.T, argv []string, c foldCase, dir, digest string, request []byte) foldCase {
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
	placeholders := []string{dir, foldDir}
	if digest != "" {
		placeholders = append(placeholders, digest, foldDigest)
	}
	r := strings.NewReplacer(placeholders...)
	got.Stdout, got.Stderr = r.Replace(stdout.String()), r.Replace(stderr.String())
	return got
}

// CTF-V0-002, CTF-V0-003: the runner group's old invocation and `corvint-test runner` reproduce
// the goldens recorded from the pre-move `corvint-test-runner`: exit status, output documents byte
// for byte, and diagnostics with the invocation name substituted.
func TestCTFV0002RunnerFoldGoldens(t *testing.T) {
	foldGoldens(t, "runner", "../corvint-test-runner")
}

// CTF-V0-002, CTF-V0-003: the same for the falsify group, including the offline `verify-receipt`
// success path over a committed evidence receipt.
func TestCTFV0002FalsifyFoldGoldens(t *testing.T) {
	foldGoldens(t, "falsify", "../corvint-behavior-falsify")
}

// CTF-V0-002, CTF-V0-003: the same for the accept group, including the offline `plan` over a
// fixture repository committed per run; its digest binds that run's roots (@DIGEST@).
func TestCTFV0002AcceptFoldGoldens(t *testing.T) {
	foldGoldens(t, "accept", "../corvint-tests-accept")
}

// CTF-V0-002, CTF-V0-003: the same for the minimize group (refusals and help; its plan reads
// pinned live evidence, so no success path runs offline).
func TestCTFV0002MinimizeFoldGoldens(t *testing.T) {
	foldGoldens(t, "minimize", "../corvint-playwright-minimize")
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
	// The accept group refuses a root reached through a symbolic link (macOS /var).
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	request := foldFixture(t, g, dir)
	if err := os.WriteFile(filepath.Join(dir, "req.json"), request, 0o600); err != nil {
		t.Fatal(err)
	}
	// A request binding the run's absolute roots has a per-run digest: the golden holds
	// foldDigest, and the output must carry exactly the digest of this run's request.
	digest := ""
	if len(g.Fixture) != 0 {
		var r testacceptance.Request
		if err := testacceptance.Decode(request, &r); err != nil {
			t.Fatal(err)
		}
		digest = testacceptance.Digest(r)
	}
	old := []string{buildCompanion(t, oldPkg, g.Old)}
	if os.Getenv("CTF_UPDATE_GOLDEN") == "1" {
		for i, c := range g.Cases {
			g.Cases[i] = runFoldCase(t, old, c, dir, digest, request)
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
			if inv.name == g.New && slices.ContainsFunc(c.Args, isHelpFlag) {
				// The folded group's help is the corvint-test command-tree help (CAP-V0-042);
				// only the old invocation keeps its recorded help.
				var help strings.Builder
				if handled, err := helpRequest(append([]string{newName}, c.Args...), &help); !handled || err != nil {
					t.Fatalf("%s %s: help not answered: %v", inv.name, c.Name, err)
				}
				want.Exit, want.Stdout, want.Stderr = 0, help.String(), ""
			} else if c.Text {
				want.Stdout = strings.ReplaceAll(c.Stdout, g.Old, inv.name)
				want.Stderr = strings.ReplaceAll(c.Stderr, g.Old, inv.name)
			}
			if got := runFoldCase(t, inv.argv, c, dir, digest, request); got.Exit != want.Exit || got.Stdout != want.Stdout || got.Stderr != want.Stderr {
				t.Errorf("%s %s:\n got exit %d stdout %q stderr %q\nwant exit %d stdout %q stderr %q",
					inv.name, c.Name, got.Exit, got.Stdout, got.Stderr, want.Exit, want.Stdout, want.Stderr)
			}
		}
	}
}

// CTF-V0-007, CTF-V0-010: the folded group entries are not linked into core `corvint`, each group
// is named in the companion usage, and its sources never use the bare `corvint-test:` prefix of
// core `corvint test` diagnostics nor name `corvint` as an executable.
func TestCTFV0007FoldedGroupBoundary(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "../corvint").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if strings.HasSuffix(dep, "/internal/testrunner/cli") || strings.HasSuffix(dep, "/internal/behaviorfalsify/cli") ||
			strings.HasSuffix(dep, "/internal/playwrightminimize/cli") || strings.HasSuffix(dep, "/internal/testacceptance/cli") {
			t.Fatalf("core corvint imports %s", dep)
		}
	}
	for _, group := range []string{"corvint-test runner <command>", "corvint-test falsify <plan|", "corvint-test minimize <plan|", "corvint-test accept plan|accept"} {
		if !strings.Contains(usage, group) {
			t.Fatalf("usage does not name %q: %q", group, usage)
		}
	}
	srcs, _ := filepath.Glob("../../internal/testrunner/cli/*.go")
	for _, dir := range []string{"behaviorfalsify", "playwrightminimize", "testacceptance"} {
		more, _ := filepath.Glob("../../internal/" + dir + "/cli/*.go")
		srcs = append(srcs, more...)
	}
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

// CTF-V0-006: an approved `corvint-test accept accept` re-enters the built corvint-test binary as
// `corvint-test accept worker`. Every worker returns a run report from the group code; a re-entry
// that reached anything else would leave its run `worker-incomplete`.
func TestCTFV0006AcceptWorkerReentersCorvintTest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "fold", "accept.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g foldGolden
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	request := foldFixture(t, g, dir)
	plan := filepath.Join(dir, "req.json")
	if err := os.WriteFile(plan, request, 0o600); err != nil {
		t.Fatal(err)
	}
	var r testacceptance.Request
	if err := testacceptance.Decode(request, &r); err != nil {
		t.Fatal(err)
	}
	// The executable re-enters itself by a path without symbolic links (self-worker-invalid).
	bin := filepath.Join(dir, "corvint-test")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, "accept", "accept", "--plan", plan,
		"--approve-plan", testacceptance.Digest(r), "--repeat", "2", "--environment", r.Environment, "--new", r.Tests[0].File)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("accept: %v\n%s", err, stderr.String())
	}
	var report testacceptance.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(report.Build, "corvint-test accept experimental/0 ") || len(report.Runs) == 0 {
		t.Fatalf("report build %q, %d runs", report.Build, len(report.Runs))
	}
	for _, run := range report.Runs {
		if slices.Contains(run.Reasons, "worker-incomplete") || slices.Contains(run.Reasons, "worker-report-invalid") {
			t.Fatalf("worker re-entry did not reach the accept group: %s %v", run.Kind, run.Reasons)
		}
	}
}
