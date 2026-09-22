package doccompiler

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

const (
	guideText = "# Guide\n\nExisting accepted prose.\n"
	noLFText  = "# No final LF\nlast line"
)

// plannedFixture extends admissionFixture with documentation targets and the
// tracked tree that proves which targets are absent.
func plannedFixture() *contextindex.Index {
	index := admissionFixture()
	pin(index, "docs/guide.md", guideText)
	pin(index, "docs/nolf.md", noLFText)
	index.Tracked = map[string]struct{}{"docs/unpinned.md": {}, "docs/file": {}, "docs/dir.md/child.md": {}}
	for path := range index.Sources {
		index.Tracked[path] = struct{}{}
	}
	return index
}

func pin(index *contextindex.Index, path, text string) {
	digest := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(text), text)))
	index.Sources[path] = contextindex.Source{Path: path, BlobHash: hex.EncodeToString(digest[:]), Data: []byte(text), Mode: "100644"}
}

func plannedRequest(index *contextindex.Index) AdmittedPlanRequest {
	return AdmittedPlanRequest{
		Clauses: []Clause{
			{ID: "c-intent", State: ClauseSupported, Kind: KindPrescriptive, Text: "Plans never write the repository.", Anchors: []Anchor{anchorAt(index, "docs/spec.md", 2, 2, AuthorityAcceptedIntent)}},
			{ID: "c-test", State: ClauseSupported, Kind: KindDescriptive, Text: "Every plan is tested.", Anchors: []Anchor{anchorAt(index, "internal/plan/plan_test.go", 3, 3, AuthorityPinnedTest)}},
		},
		Documents: []PlanDocument{
			{ID: "op-new", Target: "docs/new.md", ClauseIDs: []string{"c-intent"}, Reason: "the intent names a new page"},
			{ID: "op-guide", Target: "docs/guide.md", ClauseIDs: []string{"c-test", "c-intent"}, Reason: "the guide answers the question"},
			{ID: "op-nolf", Target: "docs/nolf.md", ClauseIDs: []string{"c-test"}, Reason: "the page lacks a final LF"},
		},
	}
}

func compilePlanned(t *testing.T, index *contextindex.Index) ([]byte, []byte, AdmittedPlan) {
	t.Helper()
	raw, patch, err := CompileAdmittedPlan(index, plannedRequest(index))
	if err != nil {
		t.Fatalf("CompileAdmittedPlan = %v", err)
	}
	plan, err := VerifyAdmittedPlan(index, raw, patch)
	if err != nil {
		t.Fatalf("VerifyAdmittedPlan(compiled) = %v\n%s", err, raw)
	}
	return raw, patch, plan
}

func TestAdmittedPlanWireIsClosedAndDeterministicHDCV0027(t *testing.T) {
	index := plannedFixture()
	raw, patch, plan := compilePlanned(t, index)
	again, againPatch, err := CompileAdmittedPlan(index, plannedRequest(index))
	if err != nil || !bytes.Equal(raw, again) || !bytes.Equal(patch, againPatch) {
		t.Fatalf("recompile differs: err=%v", err)
	}
	if plan.Profile != PlanProfile || plan.EnvironmentPin.Kind != PlanEnvironmentPlanOnly || plan.ConfigurationSnapshot.Status != PlanSnapshotNotRun {
		t.Fatalf("identity members = %q %+v %+v", plan.Profile, plan.EnvironmentPin, plan.ConfigurationSnapshot)
	}
	if plan.SourceIdentity.Revision != index.CommitRevision || plan.CandidatePatchSHA256 != sha256Hex(patch) {
		t.Fatalf("source revision or patch digest unbound: %+v", plan)
	}
	var paths []string
	for _, source := range plan.SourceIdentity.Sources {
		paths = append(paths, source.Path)
	}
	if strings.Join(paths, ",") != "docs/guide.md,docs/nolf.md,docs/spec.md,internal/plan/plan_test.go" {
		t.Fatalf("source identity paths = %v", paths)
	}
	if plan.Clauses[0].ID != "c-intent" || plan.Clauses[0].State != ClauseSupported || plan.Clauses[1].State != ClauseUnknown {
		t.Fatalf("clauses are not the admitted clauses: %+v", plan.Clauses)
	}
	if plan.LimitsConsumed != (PlanLimits{Clauses: 2, Anchors: 2, Operations: 3, SourceFiles: 4, SourceBytes: plannedSourceBytes(index, paths)}) {
		t.Fatalf("limits consumed = %+v", plan.LimitsConsumed)
	}
	if strings.Contains(string(raw), "build") || strings.Contains(string(raw), "offline") {
		t.Fatalf("plan binds receipt-only build or offline state:\n%s", raw)
	}

	document := func(change func(map[string]any)) []byte {
		var value map[string]any
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatal(err)
		}
		change(value)
		encoded, err := CanonicalJSON(value)
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	list := func(value map[string]any, member string) []any { return value[member].([]any) }
	operation := func(value map[string]any, position int) map[string]any {
		return list(value, "operations")[position].(map[string]any)
	}
	legacy, err := CanonicalJSON(PatchPlan{Profile: ExperimentalPatchPlanProfile})
	if err != nil {
		t.Fatal(err)
	}
	refused := []struct {
		name string
		raw  []byte
		code string
	}{
		{"unknown field", document(func(value map[string]any) { value["build"] = "PASS" }), "plan-unknown-field"},
		{"experimental PatchPlan", legacy, "plan-unknown-field"},
		{"profile", document(func(value map[string]any) { value["profile"] = "corvint-human-documentation-plan/1" }), "plan-profile"},
		{"duplicate key", bytes.Replace(raw, []byte(`{"candidate_patch_sha256"`), []byte(`{"candidate_patch_sha256":"x","candidate_patch_sha256"`), 1), "canonical-json-duplicate-key"},
		{"not canonical", append([]byte(" "), raw...), "canonical-json-not-canonical"},
		{"duplicate clause", document(func(value map[string]any) { list(value, "clauses")[1] = list(value, "clauses")[0] }), "duplicate-clause"},
		{"duplicate operation", document(func(value map[string]any) { operation(value, 1)["id"] = operation(value, 0)["id"] }), "duplicate-operation"},
		{"unordered sources", document(func(value map[string]any) {
			sources := value["source_identity"].(map[string]any)["sources"].([]any)
			sources[0], sources[1] = sources[1], sources[0]
		}), "unordered-set"},
		{"unordered operations", document(func(value map[string]any) {
			operations := list(value, "operations")
			operations[0], operations[1] = operations[1], operations[0]
		}), "unordered-set"},
		{"unordered clause IDs", document(func(value map[string]any) {
			ids := operation(value, 0)["clause_ids"].([]any)
			ids[0], ids[1] = ids[1], ids[0]
		}), "unordered-set"},
		{"execution environment", document(func(value map[string]any) {
			value["environment_pin"] = map[string]any{"kind": "linux-docker-capsule"}
		}), "plan-environment-unsupported"},
		{"configuration snapshot", document(func(value map[string]any) {
			value["configuration_snapshot"] = map[string]any{"status": "PASS"}
		}), "plan-snapshot-unsupported"},
		{"edit_nav", document(func(value map[string]any) { operation(value, 0)["kind"] = OperationEditNav }), "nav-authority-required"},
		{"null set", document(func(value map[string]any) { value["uncertainty"] = nil }), "plan-not-reproducible"},
		{"forged patch digest", document(func(value map[string]any) { value["candidate_patch_sha256"] = strings.Repeat("0", 64) }), "plan-not-reproducible"},
	}
	for _, test := range refused {
		if _, err := VerifyAdmittedPlan(index, test.raw, patch); errorCode(err) != test.code {
			t.Errorf("%s: VerifyAdmittedPlan = %v, want %s", test.name, err, test.code)
		}
	}
	if _, err := VerifyAdmittedPlan(index, raw, append([]byte("\n"), patch...)); errorCode(err) != "plan-not-reproducible" {
		t.Errorf("tampered patch: VerifyAdmittedPlan = %v", err)
	}
}

func plannedSourceBytes(index *contextindex.Index, paths []string) int {
	total := 0
	for _, path := range paths {
		total += len(index.Sources[path].Data)
	}
	return total
}

func TestAdmittedPlanOperationsBindPreconditionsHDCV0028(t *testing.T) {
	index := plannedFixture()
	_, patch, plan := compilePlanned(t, index)
	byID := map[string]PlanOperation{}
	for _, operation := range plan.Operations {
		byID[operation.ID] = operation
	}
	created, guide := byID["op-new"], byID["op-guide"]
	if created.Kind != OperationCreateFile || created.TargetState != TargetAbsent || created.TargetBlob != "" || created.TargetSHA256 != "" || created.StartByte != 0 || created.EndByte != 0 {
		t.Fatalf("create_file precondition = %+v", created)
	}
	if guide.Kind != OperationInsertAfter || guide.TargetState != TargetPresent || guide.TargetBlob != index.Sources["docs/guide.md"].BlobHash || guide.TargetSHA256 != sha256Hex([]byte(guideText)) {
		t.Fatalf("insert_after precondition = %+v", guide)
	}
	if strings.Join(guide.ClauseIDs, ",") != "c-intent,c-test" || guide.Reason != "the guide answers the question" {
		t.Fatalf("insert_after clauses or reason = %+v", guide)
	}
	rendered, err := RenderAdmittedProse(index, plannedRequest(index).Clauses[:1])
	if err != nil || created.ReplacementSHA256 != sha256Hex(rendered) {
		t.Fatalf("create_file replacement digest does not bind admitted prose: %v", err)
	}

	root := t.TempDir()
	for path, text := range map[string]string{"docs/guide.md": guideText, "docs/nolf.md": noLFText} {
		if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	apply := exec.Command("git", "apply", "-")
	apply.Dir, apply.Stdin = root, bytes.NewReader(patch)
	if output, err := apply.CombinedOutput(); err != nil {
		t.Fatalf("git apply: %v\n%s\n%s", err, output, patch)
	}
	for _, operation := range plan.Operations {
		after, err := os.ReadFile(filepath.Join(root, operation.Target))
		if err != nil {
			t.Fatal(err)
		}
		original := index.Sources[operation.Target].Data
		if !bytes.HasPrefix(after, original) || sha256Hex(after[operation.StartByte:]) != operation.ReplacementSHA256 {
			t.Errorf("%s: applied bytes do not match the bound insertion", operation.ID)
		}
	}

	request := plannedRequest(index)
	refuse := func(change func(*AdmittedPlanRequest)) error {
		changed := plannedRequest(index)
		changed.Documents = append([]PlanDocument(nil), request.Documents...)
		change(&changed)
		_, _, err := CompileAdmittedPlan(index, changed)
		return err
	}
	withTarget := func(target string) func(*AdmittedPlanRequest) {
		return func(request *AdmittedPlanRequest) { request.Documents[0].Target = target }
	}
	refused := []struct {
		name   string
		change func(*AdmittedPlanRequest)
		code   string
	}{
		{"overlap", withTarget("docs/guide.md"), "overlapping-operation"},
		{"under another operation target", func(request *AdmittedPlanRequest) { request.Documents[2].Target = "docs/new.md/child.md" }, "overlapping-operation"},
		{"case alias of another operation target", func(request *AdmittedPlanRequest) { request.Documents[2].Target = "docs/New.md" }, "overlapping-operation"},
		{"tracked but unpinned", withTarget("docs/unpinned.md"), "unpinned-target"},
		{"under a tracked file", withTarget("docs/file/page.md"), "target-conflict"},
		{"over tracked paths", withTarget("docs/dir.md"), "target-conflict"},
		{"case alias of a tracked page", withTarget("docs/Guide.md"), "target-conflict"},
		{"under a case alias of a tracked directory", withTarget("DOCS/new.md"), "target-conflict"},
		{"escape", withTarget("../outside.md"), "path-escape"},
		{"not markdown", withTarget("docs/new.txt"), "invalid-target"},
		{"space", withTarget("docs/new page.md"), "invalid-target"},
		{"Git metadata in any case", withTarget("docs/.GIT/notes.md"), "invalid-target"},
		{"unknown clause", func(request *AdmittedPlanRequest) { request.Documents[0].ClauseIDs = []string{"c-missing"} }, "unknown-clause"},
		{"no clauses", func(request *AdmittedPlanRequest) { request.Documents[0].ClauseIDs = nil }, "invalid-operation"},
		{"control in reason", func(request *AdmittedPlanRequest) { request.Documents[0].Reason = "line\nbreak" }, "invalid-operation"},
		{"line separator in reason", func(request *AdmittedPlanRequest) { request.Documents[0].Reason = "line\u2028break" }, "invalid-operation"},
		{"zero-width space in reason", func(request *AdmittedPlanRequest) { request.Documents[0].Reason = "zero\u200bwidth" }, "invalid-operation"},
		{"bidi override in reason", func(request *AdmittedPlanRequest) { request.Documents[0].Reason = "bidi\u202eoverride" }, "invalid-operation"},
		{"reason too long", func(request *AdmittedPlanRequest) { request.Documents[0].Reason = strings.Repeat("r", 4097) }, "invalid-operation"},
		{"duplicate operation", func(request *AdmittedPlanRequest) { request.Documents[1].ID = "op-new" }, "duplicate-operation"},
		{"too many operations", func(request *AdmittedPlanRequest) { request.Documents = make([]PlanDocument, 2001) }, "plan-limit-exceeded"},
	}
	for _, test := range refused {
		if err := refuse(test.change); errorCode(err) != test.code {
			t.Errorf("%s: CompileAdmittedPlan = %v, want %s", test.name, err, test.code)
		}
	}
	malformed := plannedFixture()
	malformed.CommitRevision = "HEAD"
	if _, _, err := CompileAdmittedPlan(malformed, plannedRequest(malformed)); errorCode(err) != "invalid-source" {
		t.Errorf("malformed index revision: %v", err)
	}
	unproven := plannedFixture()
	unproven.Tracked = nil
	if _, _, err := CompileAdmittedPlan(unproven, plannedRequest(unproven)); errorCode(err) != "absence-unproven" {
		t.Errorf("absence without a tracked tree: %v", err)
	}
	corrupt := plannedFixture()
	source := corrupt.Sources["docs/guide.md"]
	source.Data = []byte("# Tampered\n")
	corrupt.Sources["docs/guide.md"] = source
	if _, _, err := CompileAdmittedPlan(corrupt, plannedRequest(corrupt)); errorCode(err) != "target-unverified" {
		t.Errorf("target bytes disagree with the pinned blob: %v", err)
	}
}

func TestAdmittedPlanInsertsWithoutReplacingAndRetainsStaleOperationsHDCV0029(t *testing.T) {
	index := plannedFixture()
	raw, patch, plan := compilePlanned(t, index)
	for _, operation := range plan.Operations {
		if operation.StartByte != operation.EndByte || operation.EndByte != len(index.Sources[operation.Target].Data) {
			t.Fatalf("%s replaces bytes %d..%d", operation.ID, operation.StartByte, operation.EndByte)
		}
	}
	if stale := StaleOperations(index, plan); len(stale) != 0 {
		t.Fatalf("fresh plan reports stale operations %v", stale)
	}

	edited := plannedFixture()
	pin(edited, "docs/guide.md", guideText+"Accepted edit.\n")
	pin(edited, "docs/new.md", "# Someone created it\n")
	edited.Tracked["docs/new.md"] = struct{}{}
	edited.CommitRevision = strings.Repeat("c", 40)
	if stale := StaleOperations(edited, plan); strings.Join(stale, ",") != "op-guide,op-new" {
		t.Fatalf("StaleOperations = %v, want op-guide,op-new", stale)
	}
	conflicted := plannedFixture()
	conflicted.Tracked["docs/new.md/child.md"] = struct{}{}
	if stale := StaleOperations(conflicted, plan); strings.Join(stale, ",") != "op-new" {
		t.Fatalf("StaleOperations with a tracked path under an absent target = %v, want op-new", stale)
	}
	if _, err := VerifyAdmittedPlan(edited, raw, patch); errorCode(err) != "stale-source" {
		t.Fatalf("VerifyAdmittedPlan at an edited revision = %v, want stale-source", err)
	}
	retained, err := VerifyAdmittedPlan(index, raw, patch)
	if err != nil || retained.Operations[0].Target != plan.Operations[0].Target || retained.Operations[0].ReplacementSHA256 != plan.Operations[0].ReplacementSHA256 {
		t.Fatalf("retained plan changed: %v", err)
	}
}
