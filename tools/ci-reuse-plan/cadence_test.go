// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/testplatform"
)

const cadenceScript = "../../.github/ci-cadence/classify.sh"

// cadenceRepo is a throwaway repository with one base commit.
type cadenceRepo struct {
	t   *testing.T
	dir string
}

func newCadenceRepo(t *testing.T) *cadenceRepo {
	t.Helper()
	r := &cadenceRepo{t: t, dir: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	r.write("go.mod", "module example\n")
	r.write("internal/foo/foo.go", "package foo\n")
	r.write("internal/foo/foo_test.go", "package foo\n")
	r.write("internal/versionline/v_test.go", "package versionline\n")
	r.write("docs/build-log/2026-01-01-a.md", "# a\n")
	r.write("README.md", "# r\n")
	r.commit("base")
	return r
}

func (r *cadenceRepo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *cadenceRepo) write(name, body string) {
	r.t.Helper()
	p := filepath.Join(r.dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *cadenceRepo) commit(msg string) string {
	r.git("add", "-A")
	r.git("commit", "-q", "--allow-empty", "-m", msg)
	return r.git("rev-parse", "HEAD")
}

// pr applies EDIT on a head branch and returns base, head and GitHub's two-parent test merge.
func (r *cadenceRepo) pr(edit func()) (base, head, target string) {
	r.t.Helper()
	base = r.git("rev-parse", "main")
	r.git("checkout", "-q", "-b", "head", base)
	edit()
	head = r.commit("head")
	r.git("checkout", "-q", "--detach", base)
	r.git("merge", "-q", "--no-ff", "-m", "merge", head)
	target = r.git("rev-parse", "HEAD")
	return base, head, target
}

func runCadence(t *testing.T, env []string, args ...string) map[string]string {
	t.Helper()
	cmd := exec.Command("bash", append([]string{"--noprofile", "--norc", cadenceScript}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("classify.sh %v: %v\n%s", args, err, out)
	}
	got := map[string]string{}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	keys := []string{"profile", "cadence", "class", "base", "head", "target", "helper", "reason"}
	if len(lines) != len(keys) {
		t.Fatalf("classify.sh printed %q, want the fixed %d-line schema", out, len(keys))
	}
	for i, line := range lines {
		k, v, ok := strings.Cut(line, "=")
		if !ok || k != keys[i] {
			t.Fatalf("line %d %q, want key %s", i, line, keys[i])
		}
		got[k] = v
	}
	if got["profile"] != "corvint-ci-cadence/0" || !strings.HasPrefix(got["helper"], "sha256:") || len(got["helper"]) != 71 {
		t.Fatalf("schema fields %v", got)
	}
	return got
}

// CIN-V0-4750, CIN-V0-4751 (V1-4750): only build-log prose and ordinary Go tests DEFER, from a
// verified pull-request merge or a queue range; everything else, and every refusal, is FULL.
func TestCINV04750CadenceClassifier(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the classifier is a bash script run with git")
	cases := []struct {
		name, cadence, class, reason string
		edit                         func(r *cadenceRepo)
	}{
		{"build-log prose", "DEFER", "DOCS", "routine-change", func(r *cadenceRepo) {
			r.write("docs/build-log/2026-11-06-b.md", "# b\n")
			r.write("docs/build-log/2026-01-01-a.md", "# a2\n")
		}},
		{"ordinary go test", "DEFER", "TESTS", "routine-change", func(r *cadenceRepo) {
			r.write("internal/foo/foo_test.go", "package foo\n\nfunc TestX(t *testing.T) {}\n")
			r.write("internal/foo/more_test.go", "package foo\n")
		}},
		{"mixed routine", "DEFER", "DOCS+TESTS", "routine-change", func(r *cadenceRepo) {
			r.write("docs/build-log/2026-11-06-b.md", "# b\n")
			r.write("internal/foo/more_test.go", "package foo\n")
		}},
		{"production source", "FULL", "NONE", "unclassified:internal/foo/foo.go", func(r *cadenceRepo) {
			r.write("internal/foo/foo.go", "package foo\n\nvar X = 1\n")
		}},
		{"spec prose", "FULL", "NONE", "unclassified:docs/specs/x.md", func(r *cadenceRepo) { r.write("docs/specs/x.md", "x\n") }},
		{"nested build-log", "FULL", "NONE", "unclassified:docs/build-log/sub/x.md", func(r *cadenceRepo) { r.write("docs/build-log/sub/x.md", "x\n") }},
		{"archive-relevant test", "FULL", "NONE", "unclassified:internal/versionline/v_test.go", func(r *cadenceRepo) {
			r.write("internal/versionline/v_test.go", "package versionline\n\n")
		}},
		{"selection test", "FULL", "NONE", "unclassified:tools/selection-baseline/s_test.go", func(r *cadenceRepo) {
			r.write("tools/selection-baseline/s_test.go", "package main\n")
		}},
		{"testdata", "FULL", "NONE", "unclassified:internal/foo/testdata/x_test.go", func(r *cadenceRepo) {
			r.write("internal/foo/testdata/x_test.go", "package x\n")
		}},
		{"platform suffix", "FULL", "NONE", "unclassified:internal/foo/foo_darwin_test.go", func(r *cadenceRepo) {
			r.write("internal/foo/foo_darwin_test.go", "package foo\n")
		}},
		{"arch suffix", "FULL", "NONE", "unclassified:internal/foo/foo_arm64_test.go", func(r *cadenceRepo) {
			r.write("internal/foo/foo_arm64_test.go", "package foo\n")
		}},
		{"build constraint", "FULL", "NONE", "unclassified:internal/foo/c_test.go", func(r *cadenceRepo) {
			r.write("internal/foo/c_test.go", "//go:build linux\n\npackage foo\n")
		}},
		{"legacy constraint", "FULL", "NONE", "unclassified:internal/foo/c_test.go", func(r *cadenceRepo) {
			r.write("internal/foo/c_test.go", "// +build linux\n\npackage foo\n")
		}},
		{"darwin witness", "FULL", "NONE", "unclassified:internal/foo/c_test.go", func(r *cadenceRepo) {
			r.write("internal/foo/c_test.go", "package foo\n\nvar _ = runtime.GOOS == \"darwin\"\n")
		}},
		{"embed", "FULL", "NONE", "unclassified:internal/foo/c_test.go", func(r *cadenceRepo) {
			r.write("internal/foo/c_test.go", "package foo\n\n//go:embed x\nvar x string\n")
		}},
		{"embedded directory", "FULL", "NONE", "unclassified:internal/foo/assets/x_test.go", func(r *cadenceRepo) {
			r.write("internal/foo/embed.go", "package foo\n\n//go:embed assets/*\nvar assets embed.FS\n")
			r.write("internal/foo/assets/x_test.go", "package assets\n")
		}},
		{"byte-order mark", "FULL", "NONE", "unclassified:internal/foo/c_test.go", func(r *cadenceRepo) {
			r.write("internal/foo/c_test.go", "\ufeff//go:build darwin\n\npackage foo\n")
		}},
		{"cgo", "FULL", "NONE", "unclassified:internal/foo/c_test.go", func(r *cadenceRepo) {
			r.write("internal/foo/c_test.go", "package foo\n\nimport \"C\"\n")
		}},
		{"testmain", "FULL", "NONE", "unclassified:internal/foo/c_test.go", func(r *cadenceRepo) {
			r.write("internal/foo/c_test.go", "package foo\n\nfunc TestMain(m *testing.M) {}\n")
		}},
		{"workflow", "FULL", "NONE", "unclassified:.github/workflows/x.yml", func(r *cadenceRepo) { r.write(".github/workflows/x.yml", "x\n") }},
		{"delete", "FULL", "NONE", "delete:internal/foo/foo_test.go", func(r *cadenceRepo) { r.git("rm", "-q", "internal/foo/foo_test.go") }},
		{"rename", "FULL", "NONE", "rename-or-copy:internal/foo/foo_test.go", func(r *cadenceRepo) {
			r.git("mv", "internal/foo/foo_test.go", "internal/foo/bar_test.go")
		}},
		{"executable", "FULL", "NONE", "mode-100755:internal/foo/foo_test.go", func(r *cadenceRepo) {
			if err := os.Chmod(filepath.Join(r.dir, "internal/foo/foo_test.go"), 0o755); err != nil {
				r.t.Fatal(err)
			}
		}},
		{"empty", "FULL", "NONE", "empty-diff", func(r *cadenceRepo) {}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newCadenceRepo(t)
			base, head, target := r.pr(func() { c.edit(r) })
			got := runCadence(t, nil, "--root", r.dir, "--event", "pull_request", "--base", base, "--head", head, "--target", target)
			if got["cadence"] != c.cadence || got["class"] != c.class || got["reason"] != c.reason {
				t.Fatalf("got %s/%s %q, want %s/%s %q", got["cadence"], got["class"], got["reason"], c.cadence, c.class, c.reason)
			}
			if got["base"] != base || got["head"] != head || got["target"] != target {
				t.Fatalf("echoed commits %v", got)
			}
		})
	}
}

// CIN-V0-4752 (V1-4750): an unverifiable input or the rollback switch is FULL, never DEFER, and
// the queue classifies its whole base..head range.
func TestCINV04752CadenceRefusals(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the classifier is a bash script run with git")
	r := newCadenceRepo(t)
	base, head, target := r.pr(func() { r.write("docs/build-log/2026-11-06-b.md", "# b\n") })
	args := func(event, b, h, tg string) []string {
		return []string{"--root", r.dir, "--event", event, "--base", b, "--head", h, "--target", tg}
	}
	if got := runCadence(t, nil, args("pull_request", base, head, target)...); got["cadence"] != "DEFER" {
		t.Fatalf("control case %v", got)
	}
	for _, c := range []struct {
		name, reason string
		env          []string
		args         []string
	}{
		{"forced full", "forced-full", []string{"CI_CADENCE_FORCE=FULL"}, args("pull_request", base, head, target)},
		{"head is not the merge", "merge-topology-mismatch", nil, args("pull_request", base, head, head)},
		{"swapped parents", "merge-topology-mismatch", nil, args("pull_request", head, base, target)},
		{"short oid", "invalid-oid", nil, args("pull_request", base[:12], head, target)},
		{"unknown commit", "commit-missing", nil, args("pull_request", strings.Repeat("a", 40), head, target)},
		{"push event", "unsupported-event", nil, args("push", base, head, target)},
		{"queue head mismatch", "queue-head-mismatch", nil, args("merge_group", base, head, target)},
		{"queue base not ancestor", "queue-base-not-ancestor", nil, args("merge_group", target, base, base)},
		{"missing root", "root-missing", nil, []string{"--root", filepath.Join(r.dir, "absent"), "--event", "pull_request", "--base", base, "--head", head, "--target", target}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := runCadence(t, c.env, c.args...)
			if got["cadence"] != "FULL" || got["reason"] != c.reason {
				t.Fatalf("got %s %q, want FULL %q", got["cadence"], got["reason"], c.reason)
			}
		})
	}
	// A queue entry whose range holds a routine change after a FULL one is FULL as a whole.
	r.git("checkout", "-q", "main")
	r.write("internal/foo/foo.go", "package foo\n\nvar Y = 2\n")
	r.commit("source")
	r.write("docs/build-log/2026-11-06-c.md", "# c\n")
	tip := r.commit("prose")
	got := runCadence(t, nil, args("merge_group", base, tip, tip)...)
	if got["cadence"] != "FULL" || got["reason"] != "unclassified:internal/foo/foo.go" {
		t.Fatalf("queue range %v", got)
	}
	parent := r.git("rev-parse", tip+"^")
	if got := runCadence(t, nil, args("merge_group", parent, tip, tip)...); got["cadence"] != "DEFER" {
		t.Fatalf("queue routine range %v", got)
	}
}

// runPlan runs run.sh from DIR (a copy of all or part of .github/ci-cadence) in a workspace whose
// ./source is repository R, and returns the GITHUB_OUTPUT text and the step summary.
func runPlan(t *testing.T, dir string, r *cadenceRepo, env ...string) (output, summary string) {
	t.Helper()
	ws := t.TempDir()
	if err := os.Symlink(r.dir, filepath.Join(ws, "source")); err != nil {
		t.Fatal(err)
	}
	out, sum := filepath.Join(ws, "output"), filepath.Join(ws, "summary")
	cmd := exec.Command("bash", "--noprofile", "--norc", filepath.Join(dir, "run.sh"))
	cmd.Dir = ws
	cmd.Env = append(os.Environ(), append([]string{"CI_CADENCE_FORCE=", "GITHUB_OUTPUT=" + out,
		"GITHUB_STEP_SUMMARY=" + sum, "RUNNER_TEMP=" + ws}, env...)...)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run.sh must always exit zero: %v\n%s", err, b)
	}
	o, _ := os.ReadFile(out)
	s, _ := os.ReadFile(sum)
	return string(o), string(s)
}

// copyCadence copies the named files of .github/ci-cadence into a fresh directory.
func copyCadence(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join("../../.github/ci-cadence", n))
		if err != nil {
			t.Fatal(err)
		}
		if err := execfile.WriteFile(filepath.Join(dir, n), b, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// CIN-V0-4752: run.sh writes cadence exactly once, DEFER only for a receipt of the fixed schema that
// echoes this event's base, head and tested commit; every failure is FULL with exit zero.
func TestCINV04752CadenceRunWrapper(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the wrapper is a bash script run with git")
	r := newCadenceRepo(t)
	base, head, target := r.pr(func() { r.write("internal/foo/bar_test.go", "package foo\n") })
	ev := func(b, h, tg string) []string {
		return []string{"GITHUB_EVENT_NAME=pull_request", "BASE=" + b, "HEAD=" + h, "GITHUB_SHA=" + tg}
	}
	both := copyCadence(t, "run.sh", "classify.sh")
	if out, sum := runPlan(t, both, r, ev(base, head, target)...); out != "cadence=DEFER\n" ||
		!strings.Contains(sum, "reason=routine-change") || !strings.Contains(sum, "nightly") {
		t.Fatalf("routine change: output %q summary %q", out, sum)
	}
	// A classifier that claims DEFER for another tested commit is not believed.
	liar := copyCadence(t, "run.sh")
	fake := "#!/usr/bin/env bash\nprintf 'profile=corvint-ci-cadence/0\\ncadence=DEFER\\nclass=TESTS\\nbase=%s\\nhead=%s\\ntarget=%s\\nhelper=sha256:" +
		strings.Repeat("0", 64) + "\\nreason=routine-change\\n' " + base + " " + head + " " + head + "\n"
	if err := execfile.WriteFile(filepath.Join(liar, "classify.sh"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		dir string
		env []string
	}{
		"forced full":         {both, append(ev(base, head, target), "CI_CADENCE_FORCE=FULL")},
		"tested commit moved": {both, ev(base, head, head)},
		"classifier missing":  {copyCadence(t, "run.sh"), ev(base, head, target)},
		"mismatched receipt":  {liar, ev(base, head, target)},
		"push event":          {both, []string{"GITHUB_EVENT_NAME=push", "BASE=" + base, "HEAD=" + head, "GITHUB_SHA=" + target}},
	} {
		if out, sum := runPlan(t, c.dir, r, c.env...); out != "cadence=FULL\n" || !strings.Contains(sum, "CI cadence (CIN-V0-4752): FULL") {
			t.Errorf("%s: output %q summary %q", name, out, sum)
		}
	}
}

// CIN-V0-4753 to CIN-V0-4755: the workflows consume the cadence only to skip the named campaigns,
// and the nightly release gates run the deferred selection-baseline replay in full.
func TestCINV04753CadenceWorkflowWiring(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	plan := jobBlock(t, ci, "cadence-plan")
	for _, line := range []string{
		"    if: github.event_name == 'pull_request' || github.event_name == 'merge_group'",
		"    permissions:",
		"      contents: read",
		"          ref: ${{ github.event.pull_request.base.sha || github.event.merge_group.base_sha }}",
		"          sparse-checkout: .github/ci-cadence",
		"          CI_CADENCE_FORCE: ${{ vars.CI_CADENCE_FORCE }}",
		"          fetch-depth: ${{ github.event_name == 'pull_request' && 2 || 0 }}",
		// CIN-V0-4756: a trusted base without the classifier writes no cadence, so nothing defers.
		"          if [ -f trusted-cadence/.github/ci-cadence/run.sh ]; then",
		"            bash trusted-cadence/.github/ci-cadence/run.sh",
		"          else",
		"            echo 'Cadence FULL: the trusted event base has no cadence classifier.' >> \"$GITHUB_STEP_SUMMARY\"",
	} {
		wantLine(t, "cadence-plan", plan, line)
	}
	if strings.Count(plan, "persist-credentials: false") != 2 || strings.Contains(plan, "secrets.") || strings.Contains(plan, "github.token") {
		t.Error("cadence-plan must hold no credentials")
	}
	if strings.Contains(plan, "GITHUB_OUTPUT") || strings.Count(plan, "run.sh") != 2 {
		t.Error("only run.sh may write the cadence output")
	}
	// Only a verified pull-request DEFER skips the selection-baseline replay or artifact-integrity: the
	// main push reuses the queue's doc-gates and archive verdicts, so a queue entry runs both in full.
	docGates := jobBlock(t, ci, "doc-gates")
	wantLine(t, "doc-gates", docGates,
		"          CORVINT_CI_CADENCE: ${{ github.event_name == 'pull_request' && needs.cadence-plan.result == 'success' && needs.cadence-plan.outputs.cadence || 'FULL' }}")
	for _, l := range strings.Split(docGates, "\n") {
		if strings.Contains(l, "CORVINT_CI_CADENCE:") && !strings.Contains(l, "github.event_name == 'pull_request' && ") {
			t.Errorf("a merge-queue doc-gates run must never get CORVINT_CI_CADENCE=DEFER (its verdict is reused by the main push): %s", strings.TrimSpace(l))
		}
	}
	if n := strings.Count(ci, "CORVINT_CI_CADENCE:"); n != 1 {
		t.Errorf("ci.yml sets CORVINT_CI_CADENCE %d times, want only on the doc-gates step", n)
	}
	if !strings.Contains(jobBlock(t, ci, "doc-gates-record"), "&& (needs.cadence-plan.result != 'success' || needs.cadence-plan.outputs.cadence != 'DEFER') }}") {
		t.Error("doc-gates-record must be withheld by a verified DEFER")
	}
	if !strings.Contains(jobBlock(t, ci, "artifact-integrity"),
		"&& !(github.event_name == 'pull_request' && needs.cadence-plan.result == 'success' && needs.cadence-plan.outputs.cadence == 'DEFER') }}") {
		t.Error("artifact-integrity must be withheld only by a verified pull-request DEFER")
	}
	// go-macos runs the same classifier steps and starts no macOS runner under DEFER.
	mac := readWorkflow(t, "go-macos.yml")
	steps := func(b string) string { return b[strings.Index(b, "    steps:\n"):] }
	if steps(jobBlock(t, mac, "cadence-plan")) != steps(plan) {
		t.Error("go-macos cadence-plan steps diverged from ci.yml")
	}
	wantLine(t, "docs-plan", jobBlock(t, mac, "docs-plan"), "    if: ${{ !cancelled() && (needs.cadence-plan.result != 'success' || needs.cadence-plan.outputs.cadence != 'DEFER') }}")
	wantLine(t, "go-macos-shard", jobBlock(t, mac, "go-macos-shard"),
		"    if: ${{ always() && !cancelled() && github.event_name == 'pull_request' && (needs.cadence-plan.result != 'success' || needs.cadence-plan.outputs.cadence != 'DEFER') }}")
	if strings.Count(jobBlock(t, mac, "go-macos"), "        if: needs.cadence-plan.result != 'success' || needs.cadence-plan.outputs.cadence != 'DEFER'\n") != 2 {
		t.Error("the go-macos aggregator must check every shard unless the cadence verifiably deferred them")
	}
	// The Makefile guard reads only the step-level variable, and the nightly gates run the replay.
	mk, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mk), "selection-baseline-check: ## Check the test-selection baseline\n\t@if [ \"$${CORVINT_CI_CADENCE-}\" = DEFER ]; then echo") {
		t.Error("selection-baseline-check lost its CIN-V0-4753 DEFER guard")
	}
	rg := readWorkflow(t, "release-gates.yml")
	if !strings.Contains(rg, "          - gate: selection-baseline\n            os: ubuntu-24.04\n            timeout: 60\n            command: make selection-baseline-check\n") {
		t.Error("the nightly release gates must run selection-baseline-check (CIN-V0-4755)")
	}
}
