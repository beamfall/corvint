package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dogfoodflow"
	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/localcompletion"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

func transportEvidenceBytes(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	result := map[string][32]byte{}
	for _, prefix := range []string{".git/corvint/local-completion", ".corvint"} {
		err := filepath.WalkDir(filepath.Join(root, prefix), func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, _ := filepath.Rel(root, path)
			result[relative] = sha256.Sum256(raw)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func transportFileSHA(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func transportBuild(t *testing.T, path string, args ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", append(append([]string{"build"}, args...), "-buildvcs=false", "-o", path, ".")...)
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOCACHE="+filepath.Join(os.TempDir(), "corvint-go-build-cache"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v %s", path, err, output)
	}
}

// ALO-V0-023 to ALO-V0-026: the recovery-only aggregate finish runs explicitly
// admitted, independent BASE and TREE verifiers through the worker startup
// route, refuses every unadmitted, drifting or fixed identity before any
// evidence mutation, and leaves ordinary finish unchanged.
func TestTransportAdaptedRecoveryNative(t *testing.T) {
	if !groupreap.OwnerAvailable() {
		t.Skip("native Owner unavailable")
	}
	verifierA := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	// Over the legacy recorder's admitted limit, so aggregate is required.
	for i := 0; i < 200; i++ {
		cemWrite(t, root, fmt.Sprintf("aggregate/f%03d.go", i), fmt.Sprintf("package aggregate\nconst V%d = %d\n", i, i))
	}
	cemGit(t, root, "add", "aggregate")
	cemGit(t, root, "commit", "-qm", "aggregate source")
	key := localcompletion.HashSession(t.Name())
	plan, _ := json.Marshal(localcompletion.Plan{Base: base, Intents: []string{"intent.md"}, Checks: []localcompletion.Check{{ID: "actual-test", Argv: []string{"go", "test", "./fixture", "./aggregate"}, TimeoutSeconds: 60}}})
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planPath, plan, 0600); err != nil {
		t.Fatal(err)
	}
	head := cemGit(t, root, "rev-parse", "HEAD")
	headTree := cemGit(t, root, "rev-parse", "HEAD^{tree}")
	tmp := t.TempDir()
	// The HELD target is the commit after the CEM binding, so the recovery
	// binary's test admissions are linked only once that target exists.
	verifierA.ok(t, root, "dogfood", "begin", "--plan", planPath, "--session-key", key)
	verifierA.ok(t, root, "cem", "prepare", "--base", base, "--target", "HEAD")
	for i := 1; i <= 201; i++ {
		verifierA.ok(t, root, "cem", "cite", "--map", ".corvint/change.cem.json", "--hunk", fmt.Sprint(i), "--evidence-path", "intent.md", "--lines", "1:5", "--relation", "specification")
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "bind evidence")
	held := cemGit(t, root, "rev-parse", "HEAD")
	heldTree := cemGit(t, root, "rev-parse", "HEAD^{tree}")
	baseTree := cemGit(t, root, "rev-parse", base+"^{tree}")
	var saved struct {
		PlanDigest string `json:"planDigest"`
	}
	stateRaw, err := os.ReadFile(filepath.Join(root, ".git/corvint/local-completion", key, "state.json"))
	if err != nil || json.Unmarshal(stateRaw, &saved) != nil || len(saved.PlanDigest) != 64 {
		t.Fatalf("state: %v %s", err, stateRaw)
	}
	basePatch := strings.Repeat("1", 64)
	heldPatch := strings.Repeat("2", 64)
	// The verifier binaries exist before the recovery binary links its test
	// admissions, because each row pins its roles' executable digests.
	verifierB := filepath.Join(tmp, "b", "corvint")
	transportBuild(t, verifierB)
	disagree := filepath.Join(tmp, "d", "corvint")
	if err = os.MkdirAll(filepath.Dir(disagree), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(disagree, []byte("#!/bin/sh\n\""+verifierA.binary+"\" \"$@\"\nstatus=$?\necho transport-disagreement\nexit $status\n"), 0755); err != nil {
		t.Fatal(err)
	}
	shaA, shaB, shaD := transportFileSHA(t, verifierA.binary), transportFileSHA(t, verifierB), transportFileSHA(t, disagree)
	pin := func(sha string) string { return strings.TrimPrefix(sha, "sha256:") }
	rows := []string{
		strings.Join([]string{key, saved.PlanDigest, base, baseTree, basePatch, pin(shaA), held, heldTree, heldPatch, pin(shaB)}, ","),
		// The same provenance admitting the disagreeing TREE executable.
		strings.Join([]string{key, saved.PlanDigest, base, baseTree, basePatch, pin(shaA), held, heldTree, heldPatch, pin(shaD)}, ","),
		// Admitted but naming a BASE other than the plan's base.
		strings.Join([]string{key, saved.PlanDigest, head, headTree, strings.Repeat("3", 64), pin(shaA), held, heldTree, heldPatch, pin(shaB)}, ","),
		// Admitted but naming a HELD other than the current target.
		strings.Join([]string{key, saved.PlanDigest, base, baseTree, basePatch, pin(shaA), base, baseTree, strings.Repeat("4", 64), pin(shaB)}, ","),
	}
	recovery := portableRun{binary: filepath.Join(tmp, "recovery", "corvint"), env: verifierA.env}
	transportBuild(t, recovery.binary, "-trimpath", "-ldflags=-X github.com/Beamfall/corvint/internal/localcompletion.transportRecoveryTestAdmissions="+strings.Join(rows, ";"))
	fixed := filepath.Join(tmp, "fixed", "corvint")
	workCopyExecutable(t, recovery.binary, fixed)
	shaFixed := transportFileSHA(t, fixed)
	if len(map[string]bool{shaA: true, shaB: true, shaD: true, shaFixed: true}) != 4 {
		t.Fatal("verifier identities are not distinct")
	}

	recovery.ok(t, root, "dogfood", "verify", "--check", "actual-test", "--session-key", key)
	code, out, stderr := recovery.exec(t, root, nil, "dogfood", "finish", "--session-key", key)
	var evaluation struct {
		Policy struct {
			ReportSetDigest string `json:"reportSetDigest"`
			Satisfied       bool   `json:"satisfied"`
		} `json:"policy"`
		TransportAdaptedRecovery *localcompletion.TransportRecoveryProvenance `json:"transportAdaptedRecovery"`
	}
	if err = json.Unmarshal([]byte(out), &evaluation); err != nil || code != 1 || evaluation.Policy.ReportSetDigest == "" {
		t.Fatalf("legacy finish: %d %s %s %v", code, out, stderr, err)
	}
	recovery.ok(t, root, "dogfood", "review", "--report-set", evaluation.Policy.ReportSetDigest, "--session-key", key)
	if code, out, stderr = recovery.exec(t, root, nil, "dogfood", "finish", "--session-key", key); code == 0 {
		t.Fatalf("legacy recorder unexpectedly accepted aggregate: %s %s", out, stderr)
	}

	request := func(baseRole, treeRole localcompletion.TransportRecoveryVerifier, mutate func(*localcompletion.TransportRecoveryRequest)) string {
		value := localcompletion.TransportRecoveryRequest{Base: baseRole, Tree: treeRole, PlanDigest: saved.PlanDigest, Profile: localcompletion.TransportAdaptedRecoveryProfile, Qualification: localcompletion.TransportAdaptedQualification, Session: key}
		if mutate != nil {
			mutate(&value)
		}
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "request.json")
		if err = os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	role := func(path, sha, revision, tree, patch string) localcompletion.TransportRecoveryVerifier {
		return localcompletion.TransportRecoveryVerifier{AdapterPatchSHA256: "sha256:" + patch, HistoricalRevision: revision, HistoricalTree: tree, Path: path, SHA256: sha}
	}
	roleA := role(verifierA.binary, shaA, base, baseTree, basePatch)
	roleB := role(verifierB, shaB, held, heldTree, heldPatch)
	good := request(roleA, roleB, nil)
	recoverRun := func(path string, extra ...string) (int, string, string) {
		return recovery.exec(t, root, nil, append([]string{"dogfood", "finish", "--session-key", key, "--transport-adapted-recovery", path}, extra...)...)
	}
	profile := []string{"--aggregate-outcome-profile", tracerecordrepo.AggregateOutcomeProfile}

	nonCanonical := filepath.Join(t.TempDir(), "request.json")
	raw, _ := os.ReadFile(good)
	if err = os.WriteFile(nonCanonical, append([]byte(" "), raw...), 0600); err != nil {
		t.Fatal(err)
	}
	refusals := []struct {
		name, path, reason string
		extra              []string
	}{
		{"profile-flag-required", good, "invalid-local-completion-option", nil},
		{"non-canonical", nonCanonical, "transport-recovery-request-invalid", profile},
		{"unknown-qualification", request(roleA, roleB, func(r *localcompletion.TransportRecoveryRequest) { r.Qualification = "PRISTINE_HISTORICAL" }), "transport-recovery-request-invalid", profile},
		{"same-path", request(roleA, role(verifierA.binary, shaB, held, heldTree, heldPatch), nil), "transport-recovery-fixed-verifier", profile},
		{"enrollment", request(roleA, roleB, func(r *localcompletion.TransportRecoveryRequest) { r.PlanDigest = strings.Repeat("e", 64) }), "transport-recovery-enrollment-mismatch", profile},
		{"not-admitted", request(roleA, role(verifierB, shaB, held, heldTree, strings.Repeat("9", 64)), nil), "transport-recovery-not-admitted", profile},
		{"base-mismatch", request(role(verifierA.binary, shaA, head, headTree, strings.Repeat("3", 64)), roleB, nil), "transport-recovery-base-mismatch", profile},
		{"held-mismatch", request(roleA, role(verifierB, shaB, base, baseTree, strings.Repeat("4", 64)), nil), "transport-recovery-held-mismatch", profile},
		// No row can pin the running binary, whose bytes embed the rows; the
		// unit test covers the running-executable refusal itself.
		{"fixed-running-binary", request(role(fixed, shaFixed, base, baseTree, basePatch), roleB, nil), "transport-recovery-verifier-not-admitted", profile},
		{"wrong-binary-digest", request(role(disagree, shaD, base, baseTree, basePatch), roleB, nil), "transport-recovery-verifier-not-admitted", profile},
		{"identity-drift", request(role(verifierB, shaA, base, baseTree, basePatch), role(verifierA.binary, shaB, held, heldTree, heldPatch), nil), "transport-recovery-identity-drift", profile},
	}
	before := transportEvidenceBytes(t, root)
	for _, refusal := range refusals {
		code, out, stderr = recoverRun(refusal.path, refusal.extra...)
		if code != 2 || !strings.Contains(stderr, refusal.reason) {
			t.Fatalf("%s: exit=%d stdout=%s stderr=%s", refusal.name, code, out, stderr)
		}
		if after := transportEvidenceBytes(t, root); !reflect.DeepEqual(before, after) {
			t.Fatalf("%s mutated enrollment or evidence", refusal.name)
		}
	}

	// A disagreeing TREE verifier fails the strict check and leaves the
	// aggregate pending; the public check cannot treat it as complete.
	code, out, stderr = recoverRun(request(roleA, role(disagree, shaD, held, heldTree, heldPatch), nil), profile...)
	if code == 0 || !strings.Contains(stderr, "final-check-failed") {
		t.Fatalf("disagreement: exit=%d stdout=%s stderr=%s", code, out, stderr)
	}
	if code, out, stderr = recovery.exec(t, root, nil, "dogfood", "check", base); code == 0 {
		t.Fatalf("public check accepted pending aggregate: %s %s", out, stderr)
	}

	code, out, stderr = recoverRun(good, profile...)
	if err = json.Unmarshal([]byte(out), &evaluation); err != nil || code != 0 || !evaluation.Policy.Satisfied || evaluation.TransportAdaptedRecovery == nil {
		t.Fatalf("recovery: exit=%d stdout=%s stderr=%s %v", code, out, stderr, err)
	}
	if got := *evaluation.TransportAdaptedRecovery; got.Qualification != "TRANSPORT_ADAPTED_HISTORICAL" || got.BaseSHA256 != shaA || got.TreeSHA256 != shaB || got.RequestSHA256 != transportFileSHA(t, good) {
		t.Fatalf("provenance: %+v", got)
	}
	report, err := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := dogfoodflow.ParseAggregateReport(report)
	if err != nil || parsed.DogfoodCheck == nil || parsed.DogfoodCheck.BaseVerifierSHA256 != shaA || parsed.DogfoodCheck.TreeVerifierSHA256 != shaB || parsed.DogfoodCheck.OverrideVerifierSHA256 != nil || !parsed.DogfoodCheck.OutputsAgree {
		t.Fatalf("report verifiers: %v %s", err, report)
	}

	// Recovery is pending-only; ordinary aggregate finish keeps its committed
	// no-op and the public check passes against the committed outcome.
	before = transportEvidenceBytes(t, root)
	if code, out, stderr = recoverRun(good, profile...); code != 2 || !strings.Contains(stderr, "transport-recovery-not-pending") {
		t.Fatalf("committed recovery: exit=%d stdout=%s stderr=%s", code, out, stderr)
	}
	recovery.ok(t, root, append([]string{"dogfood", "finish", "--session-key", key}, profile...)...)
	if after := transportEvidenceBytes(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("committed recovery refusal or ordinary finish mutated evidence")
	}
}
