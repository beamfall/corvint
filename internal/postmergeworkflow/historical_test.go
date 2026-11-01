package postmergeworkflow

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	connector "github.com/Beamfall/corvint/internal/postmergeconnector"
)

// historicalAdapter is the changed-path reference adapter of PMR-V0-943. It is a
// deliberately simple, deterministic rule, not a product detector:
//   - a source file is a .go, .mjs, .js or .sh file that is not a test file;
//   - affected flows are the directories of changed source files;
//   - a test gap is a directory with a changed source file and no changed test file;
//   - it never reports documentation targets or defects;
//   - when it reports test gaps and the manifest names a follow-up commit, it
//     authors the follow-up's test files as the tests draft (PMR-V0-940).
//
// Directory identifiers replace "/" with ":" and the repository root with "root".
func historicalAdapter(request Request, args []string) {
	if len(args) < 1 {
		os.Exit(20)
	}
	root := args[0]
	gitOut := func(a ...string) string {
		out, err := exec.Command("git", append([]string{"-C", root}, a...)...).Output()
		if err != nil {
			os.Exit(21)
		}
		return string(out)
	}
	source, tested := map[string]bool{}, map[string]bool{}
	for _, p := range strings.Fields(gitOut("diff", "--no-renames", "--name-only", request.Binding.Base, request.Binding.Merge, "--")) {
		dir := historicalDir(p)
		switch {
		case testPath(p):
			tested[dir] = true
		case slices.Contains([]string{".go", ".mjs", ".js", ".sh"}, path.Ext(p)):
			source[dir] = true
		}
	}
	flows, gaps := []string{}, []string{}
	for dir := range source {
		flows = append(flows, dir)
		if !tested[dir] {
			gaps = append(gaps, dir)
		}
	}
	r := Result{Request: request, AffectedFlows: flows, DocumentationTargets: []string{}, TestGaps: gaps, Defects: []connector.Finding{}, Input: connector.Input{Binding: request.Binding, SourceItem: request.Binding.Change, Counts: connector.Counts{Tests: len(gaps)}}}
	for _, name := range []string{"trigger", "intake", "delta", "followup", "findings", "metrics"} {
		r.Stages = append(r.Stages, Stage{name, "observed", SHA256([]byte(name))})
	}
	for _, name := range []string{"docs-author", "docs-scope", "docs-validation", "tests-author", "tests-scope", "tests-validation", "draft-requests"} {
		status := "not-applicable"
		if len(gaps) > 0 && !strings.HasPrefix(name, "docs-") {
			status = "deferred"
		}
		r.Stages = append(r.Stages, Stage{name, status, ""})
	}
	if len(gaps) > 0 && len(args) > 1 {
		followup := args[1]
		files := []AuthoredFile{}
		for _, p := range strings.Fields(gitOut("diff", "--no-renames", "--name-only", "--diff-filter=AM", followup+"^", followup, "--")) {
			if testPath(p) {
				files = append(files, AuthoredFile{Path: p, Content: gitOut("show", followup+":"+p)})
			}
		}
		if len(files) > 0 {
			r.Authored = []Authored{{Kind: "tests", Files: files}}
		}
	}
	if json.NewEncoder(os.Stdout).Encode(r) != nil {
		os.Exit(22)
	}
	os.Exit(0)
}

func historicalDir(p string) string {
	if d := path.Dir(p); d != "." {
		return strings.ReplaceAll(d, "/", ":")
	}
	return "root"
}

type historicalEntry struct {
	ID             string `json:"id"`
	Change         string `json:"change"`
	Base           string `json:"base"`
	CreatedAt      string `json:"created_at"`
	MergedAt       string `json:"merged_at"`
	Title          string `json:"title"`
	FollowupCommit string `json:"followup_commit,omitempty"`
	Provenance     string `json:"provenance"`
	Uncertainty    string `json:"uncertainty,omitempty"`
	Expected       struct {
		AffectedFlows []string            `json:"affected_flows"`
		Followup      bool                `json:"followup"`
		TestGaps      []string            `json:"test_gaps"`
		Defects       []connector.Finding `json:"defects"`
	} `json:"expected"`
	Recorded historicalOutcome `json:"recorded"`
}

type historicalOutcome struct {
	Status         string   `json:"status"`
	MismatchFields []string `json:"mismatch_fields"`
	DraftRequests  int      `json:"draft_requests"`
}

type historicalManifest struct {
	Profile          string            `json:"profile"`
	Source           string            `json:"source"`
	Adapter          string            `json:"adapter"`
	LabelDefinitions map[string]string `json:"label_definitions"`
	Entries          []historicalEntry `json:"entries"`
	Campaign         json.RawMessage   `json:"campaign"`
}

// PMR-V0-943 (V1-0805): replay the labelled historical set of real origin/main
// changes with the changed-path reference adapter and hold every outcome,
// mismatches included, to the outcome recorded in the manifest. An entry whose
// commits the checkout lacks reads NOT_RUN and is never counted as a match.
func TestHistoricalReplayCampaign(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "historical", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m historicalManifest
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if err = d.Decode(&m); err != nil || m.Profile != "postmerge-replay-historical/0" || len(m.Entries) == 0 {
		t.Fatalf("manifest: %v %+v", err, m.Profile)
	}
	top, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("NOT_RUN: every entry (%d): the package is not in a Git checkout", len(m.Entries))
	}
	if out, _ := exec.Command("git", "rev-parse", "--is-shallow-repository").Output(); strings.TrimSpace(string(out)) == "true" {
		t.Skipf("NOT_RUN: every entry (%d): the checkout is shallow, so ancestry reads are rewritten", len(m.Entries))
	}
	// A shared, unchecked-out clone is the product root, so replay never reads
	// or touches the developer's worktree, index or refs.
	product := filepath.Join(t.TempDir(), "product")
	git(t, filepath.Dir(product), "clone", "-q", "--shared", "--no-checkout", strings.TrimSpace(string(top)), product)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	exeBytes, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	ran := 0
	for _, e := range m.Entries {
		present := true
		for _, c := range []string{e.Base, e.Change, e.FollowupCommit} {
			if c != "" && exec.Command("git", "-C", product, "cat-file", "-e", c+"^{commit}").Run() != nil {
				present = false
			}
		}
		if !present {
			t.Logf("%-10s NOT_RUN commit-unavailable (expected recorded %s)", e.ID, e.Recorded.Status)
			continue
		}
		ran++
		got := replayHistorical(t, product, exe, SHA256(exeBytes), e)
		t.Logf("%-10s %-8s mismatches=%v drafts=%d", e.ID, got.Status, got.MismatchFields, got.DraftRequests)
		if got.Status != e.Recorded.Status || !slices.Equal(got.MismatchFields, e.Recorded.MismatchFields) || got.DraftRequests != e.Recorded.DraftRequests {
			t.Errorf("%s: outcome %+v, manifest records %+v", e.ID, got, e.Recorded)
		}
	}
	t.Logf("ran %d of %d entries; the rest read NOT_RUN", ran, len(m.Entries))
	if ran == 0 {
		t.Skipf("NOT_RUN: no entry's commits are in this checkout")
	}
}

func replayHistorical(t *testing.T, product, exe, exeSHA string, e historicalEntry) historicalOutcome {
	t.Helper()
	ctx := context.Background()
	binding := connector.Binding{Forge: "git", Repository: "corvint", Change: e.ID, Base: e.Base, Merge: e.Change}
	files, err := connector.ChangedFiles(ctx, product, binding)
	if err != nil {
		t.Fatalf("%s: changed files: %v", e.ID, err)
	}
	evidence := valueHash([]string{e.ID, e.Base, e.Change, e.FollowupCommit, e.Provenance})
	labels := map[string]Label{}
	for _, field := range []string{"affected_flows", "followup", "test_gaps", "defects"} {
		labels[field] = Label{Basis: "generated", EvidenceSHA256: evidence}
	}
	f := Fixture{Profile: Profile, SourceItem: e.ID,
		Connector: connector.Fixture{Profile: connector.Profile, Tracker: connector.LocalTracker{Items: []connector.Item{{ID: e.ID, Level: "task", Parent: "release"}, {ID: "release", Level: "epic"}}},
			Forge: connector.LocalForge{Change: connector.Change{Binding: binding, Author: "repository-owner", CreatedAt: e.CreatedAt, MergedAt: e.MergedAt, Files: files, Title: e.Title}}},
		Expected: Expected{AffectedFlows: e.Expected.AffectedFlows, Followup: e.Expected.Followup, TestGaps: e.Expected.TestGaps, Defects: e.Expected.Defects, Labels: labels}}
	args := []string{"-test.run=^TestAdapterHelper$", "--", "--replay-helper", "historical", product}
	if e.FollowupCommit != "" {
		args = append(args, e.FollowupCommit)
	}
	p := Policy{Profile: Profile, Connector: connector.Policy{Profile: connector.Profile, Expected: binding, HierarchyLevel: "epic", AllowedClasses: []string{"defect"}, MaxFindings: 64, URLOrigins: []string{"https://replay.example.invalid"}},
		Executable: exe, ExecutableSHA256: exeSHA, RuntimeSHA256: SHA256([]byte("historical-reference-adapter")), Args: args, DraftURL: "https://replay.example.invalid/draft"}
	inputs := t.TempDir()
	ff, pf := filepath.Join(inputs, "fixture.json"), filepath.Join(inputs, "policy.json")
	writeJSON(t, ff, f)
	writeJSON(t, pf, p)
	r, err := Replay(ctx, product, ff, pf, e.ID)
	if err != nil {
		t.Fatalf("%s: replay blocked: %v %v", e.ID, err, r.Reasons)
	}
	if r.WorkflowQualification != "NOT_OBSERVED" {
		t.Fatalf("%s: workflow qualification %q", e.ID, r.WorkflowQualification)
	}
	out := historicalOutcome{Status: r.Status, MismatchFields: []string{}, DraftRequests: strings.Count(r.Recording, `"upsert-draft-change"`)}
	for _, mm := range append(r.HumanVerifiedMismatches, r.GeneratedMismatches...) {
		out.MismatchFields = append(out.MismatchFields, mm.Field)
	}
	slices.Sort(out.MismatchFields)
	return out
}
