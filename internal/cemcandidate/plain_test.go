package cemcandidate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	cw "github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// neutralCase is the public no-Tasks case the plain producer re-produces.
const neutralCase = "protocol/cem-1.0/stable/neutral-criterion"

func neutralCaseRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test file path")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", neutralCase)
}

// fixtureGit runs Git with the recipe identity of the public case.
func fixtureGit(t *testing.T, dir, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@invalid", "GIT_AUTHOR_DATE=2026-10-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-10-01T00:00:00Z")
	cmd.Stdin = strings.NewReader(stdin)
	b, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v: %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}

// plainFixture builds the public case's recipe repository, which has no
// .taskman/ and no Corvint file, runs the cem/1.0 producer (prepare, then one
// cite per hunk) and returns a plain assembly request over its output and the
// case's runner artifacts.
func plainFixture(t *testing.T) (PlainRequest, string, string) {
	t.Helper()
	caseRoot := neutralCaseRoot(t)
	var expected struct {
		BaseRevision   string `json:"baseRevision"`
		TargetRevision string `json:"targetRevision"`
	}
	readJSON(t, filepath.Join(caseRoot, "EXPECTED.json"), &expected)
	temp, repo, base, parent := recipeRepository(t, caseRoot)
	if base != expected.BaseRevision || parent != expected.TargetRevision {
		t.Fatalf("recipe commits %s..%s, want %s..%s", base, parent, expected.BaseRevision, expected.TargetRevision)
	}
	source, produced := producerMap(t, repo, base, parent, map[string]string{"1": "6:6", "2": "6:6"})
	if len(source.Change.Evidence) != 1 || len(source.Change.Hunks) != 2 {
		t.Fatalf("producer map has %d evidence and %d hunks, want 1 and 2", len(source.Change.Evidence), len(source.Change.Hunks))
	}
	inputs := filepath.Join(temp, "inputs")
	if e := os.Mkdir(inputs, 0o700); e != nil {
		t.Fatal(e)
	}
	ref := func(name string, raw []byte) FileRef { return save(t, filepath.Join(inputs, name), raw) }
	artifact := func(name string) []byte {
		b, e := os.ReadFile(filepath.Join(caseRoot, "artifacts", name))
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	evidence := source.Change.Evidence[0].ID
	r := PlainRequest{Profile: PlainAssemblyProfile, Repository: repo, ExpectedBase: base, Target: parent,
		SourceMap: ref("source.json", produced), RunnerPlan: ref("runner-plan.json", artifact("runner-plan.json")), RunnerReceipt: ref("runner-receipt.json", artifact("runner-receipt.json")),
		Criteria: []PlainCriterion{{CriterionEvidenceID: evidence, HunkIDs: []string{source.Change.Hunks[0].ID, source.Change.Hunks[1].ID}, EvidenceIDs: []string{evidence}}}}
	return r, filepath.Join(temp, "bundle"), caseRoot
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if e := json.Unmarshal(b, v); e != nil {
		t.Fatal(e)
	}
}

// recipeRepository builds a case's flat-tree recipe repository in a new
// temporary directory and returns that directory, the repository, and its
// base and target commits.
func recipeRepository(t *testing.T, caseRoot string) (string, string, string, string) {
	t.Helper()
	var recipe struct {
		Identity struct{ Name, Email, Date string } `json:"identity"`
		Commits  []struct {
			Name    string            `json:"name"`
			Message string            `json:"message"`
			Files   map[string]string `json:"files"`
		} `json:"commits"`
	}
	readJSON(t, filepath.Join(caseRoot, "repository.recipe.json"), &recipe)
	if recipe.Identity.Name != "Fixture" || recipe.Identity.Email != "fixture@invalid" || recipe.Identity.Date != "2026-10-01T00:00:00Z" {
		t.Fatalf("recipe identity %+v differs from fixtureGit", recipe.Identity)
	}
	temp := realTemp(t)
	repo := filepath.Join(temp, "repo")
	if e := os.Mkdir(repo, 0o755); e != nil {
		t.Fatal(e)
	}
	fixtureGit(t, repo, "", "init", "-q", "--object-format=sha1")
	base, parent := "", ""
	for _, commit := range recipe.Commits {
		names := make([]string, 0, len(commit.Files))
		for name := range commit.Files {
			names = append(names, name)
		}
		sort.Strings(names)
		var tree strings.Builder
		for _, name := range names {
			tree.WriteString("100644 blob " + fixtureGit(t, repo, commit.Files[name], "hash-object", "-w", "--stdin") + "\t" + name + "\n")
		}
		args := []string{"commit-tree", fixtureGit(t, repo, tree.String(), "mktree"), "-m", commit.Message}
		if parent != "" {
			args = append(args, "-p", parent)
		}
		parent = fixtureGit(t, repo, "", args...)
		if commit.Name == "base" {
			base = parent
		}
	}
	fixtureGit(t, repo, "", "update-ref", "refs/heads/main", parent)
	fixtureGit(t, repo, "", "symbolic-ref", "HEAD", "refs/heads/main")
	return temp, repo, base, parent
}

// producerMap runs the cem/1.0 producer over base..target: prepare, then one
// cite of SPEC.md per hunk at the given lines, and returns the producer map.
func producerMap(t *testing.T, repo, base, target string, cites map[string]string) (*cw.StableMap, []byte) {
	t.Helper()
	session, e := workflow.Open(repo)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	if _, e := session.Prepare(ctx, workflow.PrepareOptions{Base: base, Target: target}); e != nil {
		t.Fatalf("prepare: %v", e)
	}
	hunks := make([]string, 0, len(cites))
	for hunk := range cites {
		hunks = append(hunks, hunk)
	}
	sort.Strings(hunks)
	for _, hunk := range hunks {
		if _, e := session.Cite(ctx, workflow.CiteOptions{MapPath: cw.ExcludedCEMPath, Hunk: hunk, EvidencePath: "SPEC.md", Lines: cites[hunk], Relation: "specification"}); e != nil {
			t.Fatalf("cite hunk %s: %v", hunk, e)
		}
	}
	produced, e := os.ReadFile(filepath.Join(repo, cw.ExcludedCEMPath))
	if e != nil {
		t.Fatal(e)
	}
	source, e := cw.ParseStable(produced)
	if e != nil {
		t.Fatal(e)
	}
	return source, produced
}

// TestCEMV1240PlainAssemblyReproducesNeutralCriterionCase proves the plain
// producer, from the cem/1.0 producer map of a repository with no .taskman/,
// emits a map whose only criterion is a plain binding, that the native stable
// verifier ACCEPTs it, and that it and its artifacts equal the hand-assembled
// public case byte for byte (CEM-V1-240), except that the unadmitted receipt
// record is labelled opaque/0 where the public case keeps its legacy label
// (CEM-V1-310).
func TestCEMV1240PlainAssemblyReproducesNeutralCriterionCase(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	r, out, caseRoot := plainFixture(t)
	result, e := AssemblePlain(context.Background(), r, out)
	if e != nil {
		t.Fatalf("AssemblePlain: %v", e)
	}
	if result.Verification.Outcome != "ACCEPT" || result.ReceiptAuthority != "NOT_OBSERVED" || result.ExecutionAtCommit != "NOT_OBSERVED" || result.StablePath != filepath.Join(out, "stable.json") {
		t.Fatalf("result %+v", result)
	}
	for _, name := range []string{"stable.json", "artifacts/runner-plan.json", "artifacts/runner-receipt.json"} {
		got, e := os.ReadFile(filepath.Join(out, name))
		if e != nil {
			t.Fatal(e)
		}
		want, e := os.ReadFile(filepath.Join(caseRoot, name))
		if e != nil {
			t.Fatal(e)
		}
		if name == "stable.json" {
			want = opaqueLabelled(t, want)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s differs from the public case:\n%s\nwant:\n%s", name, got, want)
		}
	}
	if _, e := os.Lstat(filepath.Join(r.Repository, ".taskman")); !os.IsNotExist(e) {
		t.Fatalf(".taskman lstat = %v, want absent", e)
	}
}

// TestCEMV1240PlainAssemblyRefusals proves each request, source, graph,
// bundle and verifier refusal names its reason and writes no map.
func TestCEMV1240PlainAssemblyRefusals(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	r, out, caseRoot := plainFixture(t)
	stable, e := os.ReadFile(filepath.Join(caseRoot, "stable.json"))
	if e != nil {
		t.Fatal(e)
	}
	inputs := filepath.Dir(r.SourceMap.Path)
	type refusal struct {
		mutate func(*PlainRequest, *string)
		want   string
	}
	cases := map[string]refusal{
		"wrong-profile": {func(p *PlainRequest, _ *string) { p.Profile = StableAssemblyProfile }, "incomplete plain stable assembly request"},
		"no-criteria":   {func(p *PlainRequest, _ *string) { p.Criteria = nil }, "incomplete plain stable assembly request"},
		"no-link-hunks": {func(p *PlainRequest, _ *string) { p.Criteria[0].HunkIDs = nil }, "invalid plain criterion link"},
		"relative-pin":  {func(p *PlainRequest, _ *string) { p.RunnerPlan.Path = "runner-plan.json" }, "absolute pinned input required"},
		"changed-pin":   {func(p *PlainRequest, _ *string) { p.RunnerReceipt.Sha256 = strings.Repeat("0", 64) }, "input digest changed"},
		"assembled-source": {func(p *PlainRequest, _ *string) {
			p.SourceMap = save(t, filepath.Join(inputs, "assembled.json"), stable)
		}, "requires a cem/1.0 producer map source"},
		"unknown-criterion": {func(p *PlainRequest, _ *string) {
			p.Criteria[0].CriterionEvidenceID = "evidence:sha256:" + strings.Repeat("0", 64)
		}, "criterion evidence is not an evidence record"},
		"duplicate-criterion": {func(p *PlainRequest, _ *string) { p.Criteria = append(p.Criteria, p.Criteria[0]) }, "duplicate-reference"},
		"basis-outside-hunks": {func(p *PlainRequest, _ *string) {
			p.Criteria[0].EvidenceIDs = []string{"evidence:sha256:" + strings.Repeat("1", 64)}
		}, "unresolved-reference"},
		"bundle-in-repository":  {func(p *PlainRequest, o *string) { *o = filepath.Join(p.Repository, "bundle") }, "bundle must be outside repository"},
		"bundle-already-exists": {func(_ *PlainRequest, o *string) { *o = inputs }, "file exists"},
		"verifier-refusal": {func(p *PlainRequest, _ *string) {
			p.ExpectedBase, p.Target = p.Target, p.ExpectedBase
		}, "stable verification refused at"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := r
			p.Criteria = append([]PlainCriterion(nil), r.Criteria...)
			p.Criteria[0].HunkIDs = append([]string(nil), r.Criteria[0].HunkIDs...)
			o := out + "-" + name
			c.mutate(&p, &o)
			result, e := AssemblePlain(context.Background(), p, o)
			if e == nil || !strings.Contains(e.Error(), c.want) || result.StablePath != "" {
				t.Fatalf("AssemblePlain = %+v, %v; want refusal %q", result, e, c.want)
			}
			if _, e := os.Lstat(filepath.Join(o, "stable.json")); !os.IsNotExist(e) {
				t.Fatalf("refusal wrote stable.json: %v", e)
			}
		})
	}
}

// TestCEMV1240PlainAssemblyRechecksPinsAfterVerify proves each pinned input
// that changes after the stable verifier ACCEPTs, but before the map is
// written, is refused and leaves no stable.json.
func TestCEMV1240PlainAssemblyRechecksPinsAfterVerify(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	r, out, _ := plainFixture(t)
	recheckPinsAfterVerify(t, r, out)
}

// recheckPinsAfterVerify changes each pinned input of r in turn after the
// verifier ACCEPTs and requires the producer to refuse it without a map.
func recheckPinsAfterVerify(t *testing.T, r PlainRequest, out string) {
	t.Helper()
	t.Cleanup(func() { afterPlainVerify = nil })
	for i, pin := range r.inputs() {
		original, e := os.ReadFile(pin.Path)
		if e != nil {
			t.Fatal(e)
		}
		reached := false
		afterPlainVerify = func() {
			reached = true
			if e := os.WriteFile(pin.Path, []byte("{}\n"), 0o600); e != nil {
				t.Error(e)
			}
		}
		o := fmt.Sprintf("%s-%d", out, i)
		result, e := AssemblePlain(context.Background(), r, o)
		if !reached || e == nil || !strings.Contains(e.Error(), "input digest changed: "+pin.Path) || result.StablePath != "" || result.Verification.Outcome != "ACCEPT" {
			t.Fatalf("%s: reached=%v AssemblePlain = %+v, %v; want ACCEPT then refusal", pin.Path, reached, result, e)
		}
		if _, e := os.Lstat(filepath.Join(o, "stable.json")); !os.IsNotExist(e) {
			t.Fatalf("%s: refusal wrote stable.json: %v", pin.Path, e)
		}
		if e := os.WriteFile(pin.Path, original, 0o600); e != nil {
			t.Fatal(e)
		}
	}
}

// opaqueLabelled returns the public case map with its one runner receipt
// label replaced by opaque/0, the label an unadmitted producer emits.
func opaqueLabelled(t *testing.T, raw []byte) []byte {
	t.Helper()
	legacy := []byte(`"profile":"` + cw.StableRunnerReceiptProfile + `"`)
	if bytes.Count(raw, legacy) != 1 {
		t.Fatalf("public case map carries %d runner receipt labels, want 1", bytes.Count(raw, legacy))
	}
	return bytes.Replace(raw, legacy, []byte(`"profile":"`+cw.StableOpaqueReceiptProfile+`"`), 1)
}

// CEM-V1-347 (V1-1471): a write, sync or close failure in writeNew leaves no
// partial file behind, so a failed plain assembly never leaves a stable.json
// a consumer could read.
func TestCEMV1347WriteNewFailureRemovesThePartialFile(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	previous := syncNew
	t.Cleanup(func() { syncNew = previous })
	injected := fmt.Errorf("injected sync failure")
	syncNew = func(*os.File) error { return injected }
	if err := writeNew(root, "stable.json", []byte("{}")); err != injected {
		t.Fatalf("writeNew returned %v, want the injected failure", err)
	}
	if _, err := root.Stat("stable.json"); !os.IsNotExist(err) {
		t.Fatalf("a failed write left stable.json behind: %v", err)
	}
	syncNew = previous
	if err := writeNew(root, "stable.json", []byte("{}")); err != nil {
		t.Fatalf("the name is not reusable after cleanup: %v", err)
	}
}
