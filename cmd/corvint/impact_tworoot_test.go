package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// twoRootImpactRepos splits the internal/appmap fixture into an application repository (routers,
// flows and the manifest, whose tests block names alias e2e) and a test repository (e2e/ and
// package.json), and returns both with their HEAD commits.
func twoRootImpactRepos(t *testing.T) (app, appBase, e2e, e2eRev string) {
	t.Helper()
	app = appmapCLIRepo(t)
	raw, err := os.ReadFile(filepath.Join(app, "appmap.json"))
	if err != nil {
		t.Fatal(err)
	}
	e2e = t.TempDir()
	if err = os.CopyFS(filepath.Join(e2e, "e2e"), os.DirFS(filepath.Join(app, "e2e"))); err != nil {
		t.Fatal(err)
	}
	pkg, err := os.ReadFile(filepath.Join(app, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(e2e, "package.json"), pkg, 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, app, "rm", "-rq", "e2e", "package.json")
	manifest := strings.Replace(string(raw), `"tests": {`, `"tests": {"repo": "e2e",`, 1)
	if err = os.WriteFile(filepath.Join(app, "appmap.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, e2e, "init", "-q")
	return app, twoRootCommit(t, app, "move tests out"), e2e, twoRootCommit(t, e2e, "e2e suite")
}

func twoRootCommit(t *testing.T, dir, message string) string {
	t.Helper()
	affectedGit(t, dir, "add", "-A")
	affectedGit(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", message)
	return strings.TrimSpace(affectedGit(t, dir, "rev-parse", "HEAD"))
}

func twoRootWrite(t *testing.T, dir, rel, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runTwoRootImpactCLI(t *testing.T, args ...string) (impactTwoRootReport, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), append([]string{"impact"}, args...), strings.NewReader(""), &stdout, &stderr)
	var r impactTwoRootReport
	if code == 0 {
		if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
		}
	} else if stdout.Len() != 0 {
		t.Fatalf("refusal wrote stdout %q", stdout.String())
	}
	return r, stderr.String(), code
}

// ANL-V0-064 ANL-V0-065: two-root impact takes the diff in --app-root, reads the tests from the
// --test-root alias at its HEAD, records both pins, and answers with test-repository spec files;
// an application path the map does not place, or an uncommitted application path, returns
// full-suite-required with explicit unknown lines instead of an empty selection.
func TestANLV0064TwoRootImpact(t *testing.T) {
	app, appBase, e2e, e2eRev := twoRootImpactRepos(t)
	routes, err := os.ReadFile(filepath.Join(app, "app", "routes.js"))
	if err != nil {
		t.Fatal(err)
	}
	twoRootWrite(t, app, "app/routes.js", string(routes)+"\n")
	appHead := twoRootCommit(t, app, "routes")
	args := []string{"--app-root", app, "--test-root", "e2e=" + e2e, "--manifest", "appmap.json", "--base", appBase}
	r, stderr, code := runTwoRootImpactCLI(t, args...)
	if code != 0 {
		t.Fatal(stderr)
	}
	if r.Schema != impactTwoRootSchema || r.Verdict != appmap.VerdictTargeted || len(r.SpecFiles) == 0 || len(r.Reasons) != 0 || len(r.Unknown) != 0 ||
		r.AppRoot.Base != appBase || r.AppRoot.Head != appHead || r.AppRoot.Stated || r.TestRoot.Repo != "e2e" || r.TestRoot.Revision != e2eRev || r.TestRoot.Stated {
		t.Fatalf("targeted two-root impact %+v", r)
	}
	for _, f := range r.SpecFiles {
		if _, err := os.Stat(filepath.Join(e2e, filepath.FromSlash(f))); err != nil {
			t.Fatalf("spec file %s is not in the test repository: %v", f, err)
		}
	}

	// An uncommitted application path read at HEAD is not covered by the committed diff.
	twoRootWrite(t, app, "notes.txt", "x\n")
	if r, stderr, code = runTwoRootImpactCLI(t, args...); code != 0 || r.Verdict != appmap.VerdictFullSuite || r.AppRoot.Dirty != 1 ||
		!slices.Contains(r.Reasons, "app-worktree-not-covered") || len(r.Unknown) == 0 {
		t.Fatalf("dirty application root: code %d %s %+v", code, stderr, r)
	}
	if err := os.Remove(filepath.Join(app, "notes.txt")); err != nil {
		t.Fatal(err)
	}

	// An application path the map does not place never yields an empty selection.
	twoRootWrite(t, app, "handler.go", "package server\n")
	twoRootCommit(t, app, "backend")
	if r, stderr, code = runTwoRootImpactCLI(t, args...); code != 0 || r.Verdict != appmap.VerdictFullSuite ||
		!(slices.Contains(r.Reasons, "impact-ignored-paths") || slices.Contains(r.Reasons, "impact-unknown-paths")) ||
		!slices.ContainsFunc(r.Unknown, func(u string) bool { return strings.HasPrefix(u, affectedAppNavigationUnknown+": ") }) {
		t.Fatalf("unplaced application path: code %d %s %+v", code, stderr, r)
	}
}

// ANL-V0-066: --at reads the application at a stated revision from committed objects only, so an
// uncommitted or later application checkout does not change the answer; --test-at does the same
// for the test root, and a dirty test root read at its HEAD refuses.
func TestANLV0066TwoRootImpactAtRevision(t *testing.T) {
	app, appBase, e2e, e2eRev := twoRootImpactRepos(t)
	routes, err := os.ReadFile(filepath.Join(app, "app", "routes.js"))
	if err != nil {
		t.Fatal(err)
	}
	twoRootWrite(t, app, "app/routes.js", string(routes)+"\n")
	at := twoRootCommit(t, app, "routes")
	twoRootWrite(t, app, "later.txt", "later\n")
	twoRootCommit(t, app, "later")
	twoRootWrite(t, app, "notes.txt", "x\n")
	twoRootWrite(t, e2e, "later.txt", "later\n")
	twoRootCommit(t, e2e, "later")
	twoRootWrite(t, e2e, "e2e/specs/scratch.spec.ts", "// uncommitted\n")
	args := []string{"--app-root", app, "--test-root", "e2e=" + e2e, "--manifest", "appmap.json", "--base", appBase}
	if _, stderr, code := runTwoRootImpactCLI(t, args...); code == 0 || !strings.Contains(stderr, "appmap-root-dirty") {
		t.Fatalf("dirty test root at HEAD: code %d %s", code, stderr)
	}
	r, stderr, code := runTwoRootImpactCLI(t, append(args, "--at", at, "--test-at", e2eRev)...)
	if code != 0 {
		t.Fatal(stderr)
	}
	if r.Verdict != appmap.VerdictTargeted || r.AppRoot.Head != at || !r.AppRoot.Stated || r.AppRoot.Dirty != 0 ||
		r.TestRoot.Revision != e2eRev || !r.TestRoot.Stated || len(r.SpecFiles) == 0 {
		t.Fatalf("stated revisions %+v", r)
	}
	if _, stderr, code = runTwoRootImpactCLI(t, append(args, "--at", "no-such-revision", "--test-at", e2eRev)...); code == 0 || !strings.Contains(stderr, "appmap-invalid-revision") {
		t.Fatalf("unknown --at: code %d %s", code, stderr)
	}
}

// ANL-V0-063 ANL-V0-064: an incomplete or mixed two-root layout refuses typed before any output.
func TestANLV0064TwoRootImpactRefusesInvalidLayout(t *testing.T) {
	app, appBase, e2e, _ := twoRootImpactRepos(t)
	full := []string{"--app-root", app, "--test-root", "e2e=" + e2e, "--manifest", "appmap.json", "--base", appBase}
	for _, args := range [][]string{
		{"--app-root", app, "--manifest", "appmap.json", "--base", appBase},
		{"--test-root", "e2e=" + e2e, "--manifest", "appmap.json", "--base", appBase},
		{"--app-root", app, "--test-root", e2e, "--manifest", "appmap.json", "--base", appBase},
		append(append([]string{}, full...), "--revision", "HEAD"),
		append(append([]string{}, full...), "--manifest-repo", "other"),
		append(append([]string{}, full...), "a.go"),
	} {
		if _, stderr, code := runTwoRootImpactCLI(t, args...); code == 0 || !strings.Contains(stderr, "unsupported-impact-layout") || !strings.Contains(stderr, "--test-root ALIAS=ABSOLUTE_ROOT") {
			t.Fatalf("%v: code %d %s", args, code, stderr)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := runContext(context.Background(), append([]string{"--root", app, "impact"}, full...), strings.NewReader(""), &stdout, &stderr); code == 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "unsupported-impact-layout") {
		t.Fatalf("--root with two-root impact: code %d %s", code, stderr.String())
	}
	// A test root under another alias than the manifest's tests.repo is undeclared.
	other := append(append([]string{}, full[:2]...), append([]string{"--test-root", "other=" + e2e}, full[4:]...)...)
	if _, stderr, code := runTwoRootImpactCLI(t, other...); code == 0 || !strings.Contains(stderr, "appmap-root-unavailable") {
		t.Fatalf("undeclared tests alias: code %d %s", code, stderr)
	}
}

// ANL-V0-065: a reported change that selected no spec file is never an empty targeted answer.
func TestANLV0065TwoRootImpactNeverEmptyForAChange(t *testing.T) {
	pin := &appmap.RootPin{Repo: "e2e", Revision: "r"}
	r := twoRootImpactVerdict(appmap.ImpactReport{App: "admin", Verdict: appmap.VerdictTargeted, Changed: 1, TestRoot: pin}, 0, false)
	if r.Verdict != appmap.VerdictFullSuite || !slices.Contains(r.Reasons, "change-selected-no-spec-files") || len(r.Unknown) != 1 || len(r.SpecFiles) != 0 {
		t.Fatalf("empty targeted %+v", r)
	}
	r = twoRootImpactVerdict(appmap.ImpactReport{App: "admin", Verdict: appmap.VerdictNoNavigation, TestRoot: pin}, 0, false)
	if r.Verdict != appmap.VerdictNoNavigation || len(r.Reasons) != 0 || len(r.Unknown) != 0 {
		t.Fatalf("no change %+v", r)
	}
}

// ANL-V0-072: an unknown path's line names the file and line, or the bound, behind its reason.
func TestANLV0072TwoRootUnknownNamesItsDetail(t *testing.T) {
	pin := &appmap.RootPin{Repo: "e2e", Revision: "r"}
	r := twoRootImpactVerdict(appmap.ImpactReport{App: "admin", Verdict: appmap.VerdictFullSuite, Changed: 2, TestRoot: pin, Unknown: []appmap.ImpactUnknown{
		{Path: "app/a.html", Reason: "unresolved-reference", Detail: "app/b.js:7"},
		{Path: "app/c.html", Reason: "non-ui-path"},
	}}, 0, false)
	if r.Verdict != appmap.VerdictFullSuite || !slices.Contains(r.Unknown, "UNBOUND_APPLICATION_PATH: app/a.html: unresolved-reference: app/b.js:7") || !slices.Contains(r.Unknown, "UNBOUND_APPLICATION_PATH: app/c.html: non-ui-path") {
		t.Fatalf("%+v", r)
	}
}

// ANL-V0-066 ANL-V0-067: an application-map-impact/0 report carries the test-root pin of its map;
// `affected` in the test repository fails closed when that pin is not its HEAD, and a bound
// checkout that moves after the report was bound refuses when the receipt is written.
func TestANLV0067AffectedChecksTestRootPin(t *testing.T) {
	app, appBase, e2e, e2eRev := twoRootImpactRepos(t)
	dir := t.TempDir()
	code, mapJSON, diag := runFlowsCLI(app, "appmap", "build", "--manifest", "appmap.json", "--repo", "e2e="+e2e)
	if code != 0 {
		t.Fatal(diag)
	}
	mapFile := filepath.Join(dir, "map.json")
	if err := os.WriteFile(mapFile, []byte(mapJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	routes, err := os.ReadFile(filepath.Join(app, "app", "routes.js"))
	if err != nil {
		t.Fatal(err)
	}
	twoRootWrite(t, app, "app/routes.js", string(routes)+"\n")
	twoRootCommit(t, app, "routes")
	code, report, diag := runFlowsCLI(app, "appmap", "impact", "--map", mapFile, "--base", appBase, "--full")
	if code != 0 {
		t.Fatal(diag)
	}
	reportFile := filepath.Join(dir, "impact.json")
	if err := os.WriteFile(reportFile, []byte(report), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := appmap.LoadImpactReport(reportFile)
	if err != nil || loaded.TestRoot == nil || loaded.TestRoot.Repo != "e2e" || loaded.TestRoot.Revision != e2eRev {
		t.Fatalf("report test root %+v %v", loaded, err)
	}
	declared := []string{"--appmap-impact", reportFile, "--repository", "web=" + app, "--repository-base", "web=" + appBase}
	row := func(testBase string) map[string]any {
		t.Helper()
		receipt, _, stderr, code := runAffectedCLI(t, e2e, append([]string{"--base", testBase}, declared...)...)
		if code != 0 {
			t.Fatal(stderr)
		}
		return receipt["advice"].(map[string]any)["app_navigation"].([]any)[0].(map[string]any)
	}
	reasons := func(row map[string]any) []string {
		out := []string{}
		for _, r := range row["reasons"].([]any) {
			out = append(out, r.(string))
		}
		return out
	}
	if r := row(e2eRev); r["verdict"] != "targeted" || slices.Contains(reasons(r), "test-root-not-at-head") {
		t.Fatalf("pinned test root at HEAD %v", r)
	}
	twoRootWrite(t, e2e, "later.txt", "later\n")
	later := twoRootCommit(t, e2e, "later")
	if r := row(later); r["verdict"] != "full-suite-required" || !slices.Contains(reasons(r), "test-root-not-at-head") {
		t.Fatalf("pinned test root behind HEAD %v", r)
	}

	// The recheck the receipt runs after compiling refuses a bound checkout that moved.
	invocation := affectedInvocation{Root: e2e, Base: later, AppmapImpacts: []string{reportFile},
		Checkouts: []extevidence.Checkout{{ID: "web", Source: app}}, RepositoryBases: map[string]string{"web": appBase}}
	var advice affectedAdvice
	recheck, err := affectedAppNavigations(context.Background(), "git", invocation, later, nil, &advice)
	if err != nil {
		t.Fatal(err)
	}
	if err = recheck(); err != nil {
		t.Fatalf("unchanged checkout: %v", err)
	}
	twoRootWrite(t, app, "moved.txt", "moved\n")
	twoRootCommit(t, app, "moved")
	if err = recheck(); err == nil || !strings.Contains(err.Error(), "repository web changed while the plan was compiled") {
		t.Fatalf("moved checkout: %v", err)
	}
}

// ANL-V0-066: the final recheck refuses a test-root worktree that changed after the map was built,
// and a stated --at whose commit is not the one the map and the diff read.
func TestANLV0066TwoRootImpactDriftRecheck(t *testing.T) {
	app, appBase, e2e, e2eRev := twoRootImpactRepos(t)
	ctx := context.Background()
	pinned := func() *appmap.ImpactReport {
		return &appmap.ImpactReport{MapRevision: appBase, Head: appBase, TestRoot: &appmap.RootPin{Repo: "e2e", Revision: e2eRev}}
	}
	if err := twoRootImpactDrift(ctx, app, appBase, "HEAD", nil, true, pinned(), e2e, nil); err != nil {
		t.Fatalf("unchanged roots: %v", err)
	}
	if err := twoRootImpactDrift(ctx, app, appBase, "0000000000000000000000000000000000000000", nil, false, pinned(), e2e, nil); !strings.Contains(twoRootErrorCode(err), "impact-root-drift") {
		t.Fatalf("stated --at differing from the map revision: %v", err)
	}
	twoRootWrite(t, e2e, "e2e/specs/late.spec.ts", "// written after the map\n")
	if err := twoRootImpactDrift(ctx, app, appBase, "HEAD", nil, true, pinned(), e2e, nil); !strings.Contains(twoRootErrorCode(err), "impact-root-drift") {
		t.Fatalf("test-root worktree drift: %v", err)
	}
}

func twoRootErrorCode(err error) string {
	var k *gokernel.Error
	if errors.As(err, &k) {
		return k.Code
	}
	return ""
}
