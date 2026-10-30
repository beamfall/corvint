//go:build darwin || linux

package taskman

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/stablefixture"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/testrunner"
)

// runnerFixtureOptions vary the one runner-bound change below.
type runnerFixtureOptions struct {
	inputs   func(map[string]string) // edits the plan's inputFiles
	failTest bool                    // the receipt records one FAILED test
	opaque   bool                    // the receipts row is opaque/0
	unlinked []string                // paths whose hunks no criterionLink lists
	foreign  string                  // "plan" or "receipt": that document carries a foreign profile
	// failedFirst adds artifacts/runner-receipt-1.json, a FAILED run of the
	// same plan whose receipts row precedes the passing one; the link names
	// both, so no input is anchored (CEM-V1-606).
	failedFirst bool
	// brokenTest adds lib::TestBroken to the plan's expectedTests and the
	// receipt records it FAILED next to the passing lib::TestHelper; without
	// it the plan's selection leaves that test out (V1-2599).
	brokenTest bool
}

// newRunnerFixture is the #1703 shape in miniature: one change edits
// lib/util.txt, adds lib/fresh.txt, deletes lib/gone.txt and commits one
// runner plan and receipt under artifacts/. Every hunk cites docs/accept.txt,
// whose span names none of those paths, so every hunk recomputes as cited.
// The bound map carries one plain criterion over that span, one runnerReceipts
// row and one link naming it.
func newRunnerFixture(t *testing.T, opt runnerFixtureOptions) verdictFixture {
	t.Helper()
	stablefixture.RequireStableHost(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		command.Dir = root
		command.Env = append(os.Environ(),
			"GIT_CONFIG_NOSYSTEM=1", "HOME="+home, "XDG_CONFIG_HOME="+home,
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
			"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	put := func(path string, data []byte) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	put("docs/accept.txt", []byte("the helper passes its own test\n"))
	put("lib/util.txt", []byte("one\n"))
	put("lib/gone.txt", []byte("old\n"))
	run("add", ".")
	run("commit", "-qm", "base")
	f := verdictFixture{root: root, base: run("rev-parse", "HEAD"), git: run, portable: portableConsumer(t)}
	util, fresh := []byte("one\ntwo\n"), []byte("fresh\n")
	put("lib/util.txt", util)
	put("lib/fresh.txt", fresh)
	if err := os.Remove(filepath.Join(root, "lib", "gone.txt")); err != nil {
		t.Fatal(err)
	}
	inputs := map[string]string{"lib/util.txt": sum(util), "lib/fresh.txt": sum(fresh)}
	if opt.inputs != nil {
		opt.inputs(inputs)
	}
	plan := testrunner.PlanDocument{Profile: "corvint-test-runner-plan/0", Request: testrunner.Request{ExpectedTests: []string{"lib::TestHelper"}, InputFiles: inputs, Runner: "go-test", TimeoutSeconds: 60}}
	if opt.brokenTest {
		plan.Request.ExpectedTests = append(plan.Request.ExpectedTests, "lib::TestBroken")
	}
	if opt.foreign == "plan" {
		plan.Profile = "corvint-test-runner-plan/9"
	}
	planBytes := runnerDocument(t, plan)
	receiptFor := func(state string) []byte {
		receipt := testrunner.ReceiptDocument{Profile: wire.StableRunnerReceiptProfile, PlanSha256: testrunner.Identity(plan), Observation: testrunner.Observation{Runner: "go-test", Tests: []testrunner.Test{{ID: "lib::TestHelper", Name: "TestHelper", State: state}}, Complete: true}}
		if opt.brokenTest {
			receipt.Observation.Tests = append(receipt.Observation.Tests, testrunner.Test{ID: "lib::TestBroken", Name: "TestBroken", State: "FAILED"})
		}
		if opt.foreign == "receipt" {
			receipt.Profile = "corvint-test-runner-receipt/9"
		}
		return runnerDocument(t, receipt)
	}
	state := testrunner.Passed
	if opt.failTest {
		state = "FAILED"
	}
	receiptBytes := receiptFor(state)
	put("artifacts/runner-plan-0.json", planBytes)
	put("artifacts/runner-receipt-0.json", receiptBytes)
	var failedBytes []byte
	hunks := 5
	if opt.failedFirst {
		failedBytes, hunks = receiptFor("FAILED"), 6
		put("artifacts/runner-receipt-1.json", failedBytes)
	}
	run("add", "-A", ".")
	run("commit", "-qm", "change")
	f.change = run("rev-parse", "HEAD")
	ctx := context.Background()
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Prepare(ctx, workflow.PrepareOptions{Base: f.base, Target: f.change}); err != nil {
		t.Fatal(err)
	}
	sidecar := filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))
	prepared, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	var shape struct {
		Hunks []struct{ ID, Path string } `json:"hunks"`
	}
	if err := json.Unmarshal(prepared, &shape); err != nil || len(shape.Hunks) != hunks {
		t.Fatalf("fixture expects %d hunks: %v %s", hunks, err, prepared)
	}
	for _, hunk := range shape.Hunks {
		if session, err = workflow.Open(root); err != nil {
			t.Fatal(err)
		}
		if _, err := session.Cite(ctx, workflow.CiteOptions{MapPath: wire.ExcludedCEMPath, Hunk: hunk.ID, EvidencePath: "docs/accept.txt", Lines: "1:1", Relation: "specification"}); err != nil {
			t.Fatal(err)
		}
	}
	cited, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	document, err := wire.ParseStable(cited)
	if err != nil {
		t.Fatal(err)
	}
	criterion := wire.StableCriterion{CriterionIndex: 0}
	for _, ev := range document.Change.Evidence {
		if ev.Path == "docs/accept.txt" {
			criterion.EvidenceID, criterion.CriterionSha256 = ev.ID, ev.SpanSha256
		}
	}
	value, err := wire.Parse([]byte(`{"criterionIndex":0,"criterionSha256":"` + criterion.CriterionSha256 + `","evidenceId":"` + criterion.EvidenceID + `"}`))
	if err != nil {
		t.Fatal(err)
	}
	criterion.ID = wire.CandidateCriterionPrefix + sum(wire.CanonicalValue(value))
	link := wire.StableLink{CriterionID: criterion.ID, EvidenceIDs: []string{criterion.EvidenceID}, RunnerReceiptSha256s: []string{sum(receiptBytes)}}
	for _, hunk := range document.Change.Hunks {
		if !containsString(opt.unlinked, hunk.Path) {
			link.HunkIDs = append(link.HunkIDs, hunk.ID)
		}
	}
	profile := wire.StableRunnerReceiptProfile
	if opt.opaque {
		profile = wire.StableOpaqueReceiptProfile
	}
	document.Criteria = []wire.StableCriterion{criterion}
	document.Receipts = []wire.StableReceipt{{Sha256: sum(receiptBytes), Profile: profile, PlanSha256: sum(planBytes), SourceGitBinding: "NOT_OBSERVED", ExecutionAuthority: "CALLER_OBSERVED", DependencyClosure: "NOT_OBSERVED"}}
	document.Links = []wire.StableLink{link}
	document.Artifacts = []wire.StableArtifact{{Kind: "runner-plan", Path: "artifacts/runner-plan-0.json", Sha256: sum(planBytes)}, {Kind: "runner-receipt", Path: "artifacts/runner-receipt-0.json", Sha256: sum(receiptBytes)}}
	if opt.failedFirst {
		failed := document.Receipts[0]
		failed.Sha256 = sum(failedBytes)
		document.Receipts = append([]wire.StableReceipt{failed}, document.Receipts...)
		document.Links[0].RunnerReceiptSha256s = append(document.Links[0].RunnerReceiptSha256s, failed.Sha256)
		document.Artifacts = append(document.Artifacts, wire.StableArtifact{Kind: "runner-receipt", Path: "artifacts/runner-receipt-1.json", Sha256: sum(failedBytes)})
	}
	bound, err := wire.EncodeStable(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecar, bound, 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-f", wire.ExcludedCEMPath)
	run("commit", "-qm", "chore: bind change evidence")
	f.bind = run("rev-parse", "HEAD")
	f.sealed = sealedMapDir + f.bind + ".cem.json"
	if err := os.MkdirAll(filepath.Join(root, ".corvint", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	run("mv", wire.ExcludedCEMPath, f.sealed)
	run("commit", "-qm", "chore: seal change evidence")
	f.seal = run("rev-parse", "HEAD")
	return f
}

func runnerDocument(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

// governedByPath indexes a verdict's governed hunks by path; the fixture has
// one hunk per path.
func governedByPath(t *testing.T, view verdictView) map[string]cemVerdictHunk {
	t.Helper()
	out := map[string]cemVerdictHunk{}
	for _, h := range view.Governed {
		if _, dup := out[h.Path]; dup {
			t.Fatalf("fixture expects one hunk per path: %+v", view.Governed)
		}
		out[h.Path] = h
	}
	return out
}

var runnerTouch = []string{"artifacts/", "lib/fresh.txt", "lib/util.txt"}

func TestCEMV1606RunnerInputAnchorsNewAndEditedFiles(t *testing.T) {
	f := newRunnerFixture(t, runnerFixtureOptions{})
	in := f.input(PredicateTouchedAnchored, runnerTouch...)
	result, view := evaluateVerdict(t, in)
	if !result.Granted || view.Outcome != "GRANT" {
		t.Fatalf("runner-bound change must grant:\n%s", result.Document)
	}
	byPath := governedByPath(t, view)
	for _, path := range []string{"lib/util.txt", "lib/fresh.txt"} {
		if h := byPath[path]; h.Tier != verify.TierCited || !h.Satisfied || h.Anchor != runnerAnchorInput || h.Reason != "" {
			t.Fatalf("%s must stay cited and be satisfied on runner-input: %+v", path, h)
		}
	}
	// The tier is never raised: every governed tier is the recomputed one.
	var citation verify.Citation
	if err := json.Unmarshal(view.Citation, &citation); err != nil {
		t.Fatal(err)
	}
	tiers := map[string]string{}
	for _, row := range citation.Hunks {
		tiers[row.HunkID] = row.Tier
	}
	for _, h := range view.Governed {
		if tiers[h.HunkID] != h.Tier {
			t.Fatalf("governed tier differs from the citation: %+v", h)
		}
	}
	for _, p := range view.Paths {
		if !p.Satisfied || p.Tier != verify.TierCited {
			t.Fatalf("path rows keep the minimum tier and report satisfaction: %+v", view.Paths)
		}
	}
	// Pinned evidence reverifies byte for byte.
	again, err := ReverifyCEMVerdictGate(context.Background(), in, result.Document)
	if err != nil || !again.Granted || !bytes.Equal(again.Document, result.Document) {
		t.Fatalf("runner-bound grant must reverify: %v %+v", err, again)
	}
}

func TestCEMV1607RunnerArtifactAnchorsCommittedRunnerArtifacts(t *testing.T) {
	f := newRunnerFixture(t, runnerFixtureOptions{})
	result, view := evaluateVerdict(t, f.input(PredicateTouchedAnchored, "artifacts/"))
	if !result.Granted {
		t.Fatalf("committed runner artifacts must grant:\n%s", result.Document)
	}
	byPath := governedByPath(t, view)
	for _, path := range []string{"artifacts/runner-plan-0.json", "artifacts/runner-receipt-0.json"} {
		if h := byPath[path]; h.Tier != verify.TierCited || !h.Satisfied || h.Anchor != runnerAnchorArtifact {
			t.Fatalf("%s must be satisfied on runner-artifact: %+v", path, h)
		}
	}
	// A plan bound by a failed run and by a later passing run is anchored by
	// the passing one; the failed run's receipt is not.
	both := newRunnerFixture(t, runnerFixtureOptions{failedFirst: true})
	result, view = evaluateVerdict(t, both.input(PredicateTouchedAnchored, "artifacts/runner-plan-0.json"))
	if h := governedByPath(t, view)["artifacts/runner-plan-0.json"]; !result.Granted || !h.Satisfied || h.Anchor != runnerAnchorArtifact {
		t.Fatalf("a passing receipt after a failed one must anchor the plan: %+v\n%s", h, result.Document)
	}
	view = requireVerdictRefusal(t, both.input(PredicateTouchedAnchored, "artifacts/runner-receipt-1.json"), CEMVerdictPredicateFailed, "tier-below-anchored")
	if len(view.Governed) != 1 || view.Governed[0].Satisfied || view.Governed[0].Reason != runnerReasonNotPassed {
		t.Fatalf("the failed receipt anchors nothing: %+v", view.Governed)
	}
	// An artifact bound by a failed run is not anchored.
	failed := newRunnerFixture(t, runnerFixtureOptions{failTest: true})
	view = requireVerdictRefusal(t, failed.input(PredicateTouchedAnchored, "artifacts/"), CEMVerdictPredicateFailed, "tier-below-anchored")
	for _, h := range view.Governed {
		if h.Satisfied || h.Anchor != "" || h.Reason != runnerReasonNotPassed {
			t.Fatalf("a failed run anchors nothing: %+v", h)
		}
	}
}

func TestCEMV1608UnanchorablePathsRefuseWithANamedReason(t *testing.T) {
	cases := []struct {
		name   string
		opt    runnerFixtureOptions
		path   string
		reason string
	}{
		{"deleted file", runnerFixtureOptions{}, "lib/gone.txt", runnerReasonNoPostImage},
		{"input not in plan", runnerFixtureOptions{inputs: func(m map[string]string) { delete(m, "lib/fresh.txt") }}, "lib/fresh.txt", runnerReasonNoBinding},
		{"stale input digest", runnerFixtureOptions{inputs: func(m map[string]string) { m["lib/util.txt"] = sum([]byte("one\n")) }}, "lib/util.txt", runnerReasonInputMismatch},
		{"failed run", runnerFixtureOptions{failTest: true}, "lib/util.txt", runnerReasonNotPassed},
		{"opaque receipt", runnerFixtureOptions{opaque: true}, "lib/fresh.txt", runnerReasonNoBinding},
		{"foreign plan profile", runnerFixtureOptions{foreign: "plan"}, "lib/util.txt", runnerReasonArtifactMismatch},
		{"foreign receipt profile", runnerFixtureOptions{foreign: "receipt"}, "artifacts/runner-plan-0.json", runnerReasonArtifactMismatch},
		{"unlinked hunk", runnerFixtureOptions{unlinked: []string{"lib/fresh.txt"}}, "lib/fresh.txt", runnerReasonNoBinding},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newRunnerFixture(t, c.opt)
			view := requireVerdictRefusal(t, f.input(PredicateTouchedAnchored, c.path), CEMVerdictPredicateFailed, "tier-below-anchored")
			if len(view.Governed) != 1 || view.Governed[0].Satisfied || view.Governed[0].Anchor != "" || view.Governed[0].Reason != c.reason || view.Governed[0].Tier != verify.TierCited {
				t.Fatalf("want one unsatisfied cited hunk naming %s: %+v", c.reason, view.Governed)
			}
			if len(view.Paths) != 1 || view.Paths[0].Satisfied {
				t.Fatalf("path row must be unsatisfied: %+v", view.Paths)
			}
		})
	}
	// all-anchored governs the deletion too, so the whole change refuses.
	f := newRunnerFixture(t, runnerFixtureOptions{})
	requireVerdictRefusal(t, f.input(PredicateAllAnchored), CEMVerdictPredicateFailed, "tier-below-anchored")
}

func TestCEMV1608RunnerAnchorReasonsAndReadFailure(t *testing.T) {
	doc := &wire.StableMap{}
	b := newRunnerBinding(func(...string) ([]byte, error) { t.Fatal("an uncited hunk reads nothing"); return nil, nil }, "c", doc)
	if anchor, reason, err := b.anchor(wire.Hunk{ID: "h", Path: "a"}, verify.TierUnknown); err != nil || anchor != "" || reason != runnerReasonUncited {
		t.Fatalf("uncited: %q %q %v", anchor, reason, err)
	}
	failing := newRunnerBinding(func(...string) ([]byte, error) { return nil, errors.New("boom") }, "c", doc)
	g := &verdictGate{doc: cemVerdictDocument{Governed: []cemVerdictHunk{}, Paths: []cemVerdictPath{}}}
	citation := &verify.Citation{Hunks: []verify.CitationHunk{{HunkID: "h", Tier: verify.TierCited}}}
	g.predicate(PredicateAllAnchored, nil, []wire.Hunk{{ID: "h", Path: "a"}}, citation, failing.anchor)
	if g.doc.Outcome == "GRANT" || g.doc.Code == nil || *g.doc.Code != CEMVerdictUnverifiable || *g.doc.Reason != "git-read-failed" || len(g.doc.Governed) != 0 {
		t.Fatalf("a failed runner read is unverifiable: %+v", g.doc)
	}
}

func TestCEMV1608AnchoredGrantEncodesWithoutRunnerMembers(t *testing.T) {
	f := newVerdictFixture(t, nil)
	result, _ := evaluateVerdict(t, f.input(PredicateTouchedAnchored, "src/"))
	if !result.Granted || bytes.Contains(result.Document, []byte(`"anchor"`)) || bytes.Contains(result.Document, []byte(`"reason":"`)) {
		t.Fatalf("a tier-satisfied grant must encode exactly as before:\n%s", result.Document)
	}
}
