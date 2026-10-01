package postmergeworkflow

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	connector "github.com/Beamfall/corvint/internal/postmergeconnector"
)

// This helper is a synthetic protocol adapter, never positive workflow evidence.
// It runs in a separate process so tests exercise real execution and recording.
func TestAdapterHelper(t *testing.T) {
	args := os.Args
	index := -1
	for i, a := range args {
		if a == "--replay-helper" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	mode := args[index+1]
	if os.Getenv("CORVINT_REPLAY_SECRET_SENTINEL") != "" {
		os.Exit(9)
	}
	var request Request
	if json.NewDecoder(os.Stdin).Decode(&request) != nil {
		os.Exit(10)
	}
	if mode == "hang" {
		child := exec.Command("/bin/sleep", "30")
		if child.Start() != nil {
			os.Exit(12)
		}
		if os.WriteFile(args[index+2], []byte(strconv.Itoa(child.Process.Pid)), 0600) != nil {
			os.Exit(13)
		}
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	r := Result{Request: request, AffectedFlows: []string{"flow-a"}, DocumentationTargets: []string{}, TestGaps: []string{}, Defects: []connector.Finding{{Class: "defect", Classification: "ordinary", Path: "a.txt", Line: 1}}, Input: connector.Input{Binding: request.Binding, SourceItem: "task"}}
	r.Input.Findings = r.Defects
	for _, name := range []string{"trigger", "intake", "delta", "followup", "findings", "metrics"} {
		r.Stages = append(r.Stages, Stage{name, "observed", SHA256([]byte(name))})
	}
	for _, name := range []string{"docs-author", "docs-scope", "docs-validation", "tests-author", "tests-scope", "tests-validation", "draft-requests"} {
		r.Stages = append(r.Stages, Stage{name, "not-applicable", ""})
	}
	switch mode {
	case "missing":
		r.Stages = r.Stages[1:]
	case "nonapplicable":
		r.Stages[0].Status = "not-applicable"
	case "binding":
		r.FixtureSHA256 = SHA256([]byte("wrong"))
	case "inconsistent":
		r.Input.Findings = nil
	case "gaps", "gaps-observed", "gaps-not-applicable":
		r.DocumentationTargets = []string{"docs/a.md"}
		r.TestGaps = []string{"gap-1"}
		r.Input.Counts = connector.Counts{Docs: 1, Tests: 1}
		status := map[string]string{"gaps": "deferred", "gaps-observed": "observed", "gaps-not-applicable": "not-applicable"}[mode]
		for i := range r.Stages {
			if r.Stages[i].Status == "not-applicable" {
				r.Stages[i].Status = status
				if status == "observed" {
					r.Stages[i].ArtifactSHA256 = SHA256([]byte(r.Stages[i].Name))
				}
			}
		}
	case "draft":
		r.Input.Drafts = []connector.Draft{{Kind: "docs", Revision: request.Binding.Merge, URL: "https://example.com/draft"}}
	case "nondeterministic":
		cwd, _ := os.Getwd()
		r.Stages[0].ArtifactSHA256 = SHA256([]byte(cwd))
	}
	if json.NewEncoder(os.Stdout).Encode(r) != nil {
		os.Exit(11)
	}
	os.Exit(0)
}

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func writeJSON(t *testing.T, name string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(name, b, 0600); err != nil {
		t.Fatal(err)
	}
}
func setup(t *testing.T, mode string) (string, string, string, Fixture, Policy) {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Replay")
	git(t, root, "config", "user.email", "replay@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "a.txt")
	git(t, root, "commit", "-qm", "base")
	base := git(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("new\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "commit", "-qam", "merge")
	merge := git(t, root, "rev-parse", "HEAD")
	binding := connector.Binding{Forge: "fileforge", Repository: "repo", Change: "change-1", Base: base, Merge: merge}
	labels := map[string]Label{}
	for _, field := range []string{"affected_flows", "followup", "test_gaps", "defects"} {
		labels[field] = Label{Basis: "generated", EvidenceSHA256: SHA256([]byte(field))}
	}
	f := Fixture{Profile: Profile, SourceItem: "task", Connector: connector.Fixture{Profile: connector.Profile, Tracker: connector.LocalTracker{Items: []connector.Item{{ID: "task", Level: "task", Parent: "epic"}, {ID: "epic", Level: "epic"}}}, Forge: connector.LocalForge{Change: connector.Change{Binding: binding, Author: "author", CreatedAt: "2026-09-29T01:00:00Z", MergedAt: "2026-09-29T02:00:00Z", Files: []connector.ChangedFile{{Path: "a.txt", Status: "M"}}, Title: "HOSTILE $(secret) raw title", Body: "raw body must never reach adapter"}}}, Expected: Expected{AffectedFlows: []string{"flow-a"}, Followup: true, TestGaps: []string{}, Defects: []connector.Finding{{Class: "defect", Classification: "ordinary", Path: "a.txt", Line: 1}}, Labels: labels}}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	p := Policy{Profile: Profile, Connector: connector.Policy{Profile: connector.Profile, Expected: binding, HierarchyLevel: "epic", AllowedClasses: []string{"defect"}, MaxFindings: 64, URLOrigins: []string{"https://example.com"}}, Executable: exe, ExecutableSHA256: SHA256(b), RuntimeSHA256: SHA256([]byte("helper-runtime")), Args: []string{"-test.run=^TestAdapterHelper$", "--", "--replay-helper", mode}}
	inputs := t.TempDir()
	ff := filepath.Join(inputs, "fixture.json")
	pf := filepath.Join(inputs, "policy.json")
	writeJSON(t, ff, f)
	writeJSON(t, pf, p)
	return root, ff, pf, f, p
}
func TestReplayActualRecording(t *testing.T) {
	root, ff, pf, _, _ := setup(t, "ok")
	t.Setenv("CORVINT_REPLAY_SECRET_SENTINEL", "must-not-be-inherited")
	before := git(t, root, "status", "--porcelain")
	r, err := Replay(context.Background(), root, ff, pf, "change-1")
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "MATCH" || r.WorkflowQualification != "NOT_OBSERVED" || len(r.HumanVerifiedMismatches)+len(r.GeneratedMismatches) != 0 || !strings.Contains(r.Recording, "upsert-followup") || !strings.Contains(r.Recording, "upsert-finding") {
		t.Fatalf("report: %+v", r)
	}
	if SHA256([]byte(r.Recording)) != r.RecordingSHA256 || strings.Contains(r.Recording, "HOSTILE") {
		t.Fatal("recording content/binding invalid")
	}
	if git(t, root, "status", "--porcelain") != before {
		t.Fatal("product changed")
	}
}
func TestReplayMismatchBasis(t *testing.T) {
	root, ff, pf, f, _ := setup(t, "ok")
	f.Expected.AffectedFlows = []string{"other-flow"}
	f.Expected.Followup = false
	writeJSON(t, ff, f)
	r, err := Replay(context.Background(), root, ff, pf, "change-1")
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "MISMATCH" || len(r.GeneratedMismatches) != 2 || len(r.HumanVerifiedMismatches) != 0 {
		t.Fatalf("mismatches: %+v", r)
	}
	for _, m := range r.GeneratedMismatches {
		if m.Basis != "generated" {
			t.Fatal(m)
		}
	}
}

// PMR-V0-007: the report separates mismatches against human-verified
// expectations from mismatches against generated ones.
func TestReplaySeparatesHumanVerifiedMismatches(t *testing.T) {
	root, ff, pf, f, p := setup(t, "ok")
	f.Expected.AffectedFlows = []string{"other-flow"}
	f.Expected.Followup = false
	label := Label{Basis: "human-verified", EvidenceSHA256: SHA256([]byte("review")), Human: "owner", Approval: "approval-1"}
	f.Expected.Labels["followup"] = label
	writeJSON(t, ff, f)
	p.Registry = filepath.Join(t.TempDir(), "registry.json")
	writeJSON(t, p.Registry, Registry{Profile: Profile, Approvals: []Approval{{Binding: p.Connector.Expected, Field: "followup", ValueSHA256: valueHash(false), Label: label}}})
	b, _ := os.ReadFile(p.Registry)
	p.RegistrySHA256 = SHA256(b)
	writeJSON(t, pf, p)
	r, err := Replay(context.Background(), root, ff, pf, "change-1")
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "MISMATCH" || len(r.HumanVerifiedMismatches) != 1 || r.HumanVerifiedMismatches[0].Field != "followup" || len(r.GeneratedMismatches) != 1 || r.GeneratedMismatches[0].Field != "affected_flows" {
		t.Fatalf("separation: %+v", r)
	}
}

// PMR-V0-006 and issue #395 acceptance: two separate replays of the same change
// produce identical recorded requests and identical reports.
func TestReplayTwiceIdenticalRecording(t *testing.T) {
	root, ff, pf, _, _ := setup(t, "ok")
	first, err := Replay(context.Background(), root, ff, pf, "change-1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Replay(context.Background(), root, ff, pf, "change-1")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if first.Recording == "" || first.Recording != second.Recording || string(a) != string(b) {
		t.Fatalf("replays differ:\n%s\n%s", a, b)
	}
}

// PMR-V0-005: detected documentation targets and test gaps are compared while
// author/scope/validation stages stay deferred; a driver cannot claim them.
func TestReplayDeferredAuthoringStages(t *testing.T) {
	root, ff, pf, f, _ := setup(t, "gaps")
	f.Expected.TestGaps = []string{"gap-1"}
	writeJSON(t, ff, f)
	r, err := Replay(context.Background(), root, ff, pf, "change-1")
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "MATCH" || len(r.DeferredStages) != 7 || !slices.Contains(r.Limits, "authoring-scope-validation-stages-deferred") || !strings.Contains(r.Recording, `Docs: 1\nTests: 1`) || strings.Contains(r.Recording, "upsert-draft-change") {
		t.Fatalf("report: %+v", r)
	}
	for mode, reason := range map[string]string{"gaps-observed": "draft-scope-validation-unavailable", "gaps-not-applicable": "stage-applicability-invalid"} {
		root, ff, pf, _, _ := setup(t, mode)
		if r, err := Replay(context.Background(), root, ff, pf, "change-1"); err == nil || err.Error() != reason || r.Recording != "" {
			t.Fatalf("%s: %+v %v", mode, r, err)
		}
	}
}
func TestReplayBlocks(t *testing.T) {
	for _, test := range []struct{ mode, reason string }{{"missing", "required-stage-missing"}, {"nonapplicable", "required-stage-blocked"}, {"binding", "driver-binding-mismatch"}, {"inconsistent", "driver-outcomes-inconsistent"}, {"draft", "draft-scope-validation-unavailable"}, {"nondeterministic", "replay-nondeterministic"}} {
		t.Run(test.mode, func(t *testing.T) {
			root, ff, pf, _, _ := setup(t, test.mode)
			r, err := Replay(context.Background(), root, ff, pf, "change-1")
			if err == nil || err.Error() != test.reason || r.Status != "BLOCKED" || r.Recording != "" {
				t.Fatalf("report %+v err %v", r, err)
			}
		})
	}
}
func TestHumanRegistryCannotSelfPromote(t *testing.T) {
	root, ff, pf, f, p := setup(t, "ok")
	label := f.Expected.Labels["followup"]
	label.Basis = "human-verified"
	label.Human = "owner"
	label.Approval = "approval-1"
	f.Expected.Labels["followup"] = label
	writeJSON(t, ff, f)
	if _, err := Replay(context.Background(), root, ff, pf, "change-1"); err == nil || err.Error() != "human-label-unverified" {
		t.Fatal(err)
	}
	registry := Registry{Profile: Profile, Approvals: []Approval{{Binding: p.Connector.Expected, Field: "followup", ValueSHA256: valueHash(true), Label: label}}}
	p.Registry = filepath.Join(t.TempDir(), "registry.json")
	writeJSON(t, p.Registry, registry)
	b, _ := os.ReadFile(p.Registry)
	p.RegistrySHA256 = SHA256(b)
	writeJSON(t, pf, p)
	r, err := Replay(context.Background(), root, ff, pf, "change-1")
	if err != nil || r.Status != "MATCH" {
		t.Fatalf("%+v %v", r, err)
	}
	registry.Approvals[0].Binding.Repository = "different-repository"
	writeJSON(t, p.Registry, registry)
	b, _ = os.ReadFile(p.Registry)
	p.RegistrySHA256 = SHA256(b)
	writeJSON(t, pf, p)
	if _, err = Replay(context.Background(), root, ff, pf, "change-1"); err == nil || err.Error() != "human-label-unverified" {
		t.Fatal(err)
	}
	registry.Approvals[0].Binding = p.Connector.Expected
	registry.Approvals[0].ValueSHA256 = valueHash(false)
	writeJSON(t, p.Registry, registry)
	b, _ = os.ReadFile(p.Registry)
	p.RegistrySHA256 = SHA256(b)
	writeJSON(t, pf, p)
	if _, err = Replay(context.Background(), root, ff, pf, "change-1"); err == nil || err.Error() != "human-label-unverified" {
		t.Fatal(err)
	}
}
func TestClosedJSON(t *testing.T) {
	for _, b := range []string{`{"profile":"a","profile":"b"}`, `{"profile":"a","PROFILE":"b"}`, `{"connector":{"PROFILE":"b"}}`, `{"unknown":true}`, `{} {}`, strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66)} {
		var f Fixture
		if Decode([]byte(b), &f) == nil {
			t.Fatalf("accepted %s", b)
		}
	}
}
func TestPinsAndSelector(t *testing.T) {
	root, ff, pf, f, p := setup(t, "ok")
	if _, err := Replay(context.Background(), root, ff, pf, "other"); err == nil || err.Error() != "change-selector-mismatch" {
		t.Fatal(err)
	}
	p.ExecutableSHA256 = SHA256([]byte("wrong"))
	writeJSON(t, pf, p)
	if _, err := Replay(context.Background(), root, ff, pf, "change-1"); err == nil || err.Error() != "executable-pin-mismatch" {
		t.Fatal(err)
	}
	f.Expected.TestGaps = []string{"../escape"}
	writeJSON(t, ff, f)
	if _, err := Replay(context.Background(), root, ff, pf, "change-1"); err == nil || err.Error() != "expectations-invalid" {
		t.Fatal(err)
	}
}

func TestCLIReplay(t *testing.T) {
	root, ff, pf, fixture, _ := setup(t, "ok")
	tool := filepath.Join(t.TempDir(), "corvint-postmerge-workflow")
	build := exec.Command("go", "build", "-o", tool, "../../cmd/corvint-postmerge-workflow")
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if b, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build companion: %v %s", err, b)
	}
	for _, mismatch := range []bool{false, true} {
		fixture.Expected.Followup = !mismatch
		writeJSON(t, ff, fixture)
		cmd := exec.Command(tool, "replay", "--change", "change-1", "--dry-run", "--fixture", ff, "--policy", pf)
		cmd.Dir = root // Exercise default --root=. against the actual product.
		b, err := cmd.Output()
		var report Report
		if Decode(b, &report) != nil {
			t.Fatalf("invalid CLI report: %s %v", b, err)
		}
		if !mismatch && (err != nil || report.Status != "MATCH") {
			t.Fatalf("match: %+v %v", report, err)
		}
		if mismatch {
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 1 || report.Status != "MISMATCH" {
				t.Fatalf("mismatch: %+v %v", report, err)
			}
		}
	}
	cmd := exec.Command(tool, "replay", "--change", "change-1", "--fixture", ff, "--policy", pf, "--root", root)
	if err := cmd.Run(); err == nil {
		t.Fatal("CLI accepted missing dry-run")
	}
}
