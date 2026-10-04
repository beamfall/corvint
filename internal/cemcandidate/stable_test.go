package cemcandidate

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	cw "github.com/Beamfall/corvint/internal/cem/wire"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

func stableFixture(t *testing.T, format string) (StableRequest, string) {
	t.Helper()
	r, out := fixture(t, format)
	// The reviewed stable prototype admits literal generated configurations in
	// one section order. Keep that narrower fixture qualification explicit.
	if format == "sha256" {
		p := filepath.Join(r.Repository, ".git/config")
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		prefix := "[extensions]\n\tobjectformat = sha256\n"
		if strings.HasPrefix(string(b), prefix) {
			if e := os.WriteFile(p, []byte(strings.TrimPrefix(string(b), prefix)+prefix), 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
	snapshot := save(t, filepath.Join(realTemp(t), "snapshot.json"), []byte("{\"profile\":\"manufactured-snapshot-not-native-authority\"}\n"))
	return StableRequest{Profile: StableAssemblyProfile, Repository: r.Repository, ExpectedBase: r.ExpectedBase, Target: r.Target, TicketID: r.TicketID, AttemptID: r.AttemptID, SourcePrefix: r.SourcePrefix, SourceMap: r.SourceMap, Capture: r.Capture, Verification: r.Verification, SnapshotHeadArtifact: snapshot, RunnerPlan: r.RunnerPlan, RunnerReceipt: r.RunnerReceipt, Links: r.Links}, out
}

func stableAssemblyDebug(result StableAssemblyResult) string {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func TestStableAssemblySeparateSnapshotIdentityAndFullEnvelope(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			r, out := stableFixture(t, format)
			result, e := AssembleStable(t.Context(), r, out)
			if e != nil {
				config, _ := os.ReadFile(filepath.Join(r.Repository, ".git/config"))
				t.Fatalf("%v; verification=%s config=%s", e, stableAssemblyDebug(result), config)
			}
			if result.Verification.Outcome != "ACCEPT" || result.Verification.Spec != cw.StableSpec || result.DeclaredInputGitBinding != "DECLARED_INPUT_BYTES_MATCH_TARGET" {
				t.Fatalf("result %+v", result)
			}
			for _, s := range []string{result.NativeCaptureSemanticVerification, result.ReceiptAuthority, result.SourceInventoryCompleteness, result.ExecutionAtCommit} {
				if s != "NOT_OBSERVED" {
					t.Fatal("promoted native authority")
				}
			}
			raw, e := Read(result.StablePath)
			if e != nil {
				t.Fatal(e)
			}
			d, e := cw.ParseStable(raw)
			if e != nil {
				t.Fatal(e)
			}
			c := d.Criteria[0]
			if c.SnapshotHeadArtifactSha256 != r.SnapshotHeadArtifact.Sha256 || c.SnapshotHeadReceiptSha256 != strings.Repeat("b", 64) || c.SnapshotHeadArtifactSha256 == c.SnapshotHeadReceiptSha256 {
				t.Fatalf("aliased snapshot identity %+v", c)
			}
			// Compute the complete identity independently, including both snapshot fields.
			var document map[string]json.RawMessage
			_ = json.Unmarshal(raw, &document)
			var criteria []map[string]any
			_ = json.Unmarshal(document["criterionBindings"], &criteria)
			delete(criteria[0], "id")
			canonical, _ := json.Marshal(criteria[0])
			want := cw.CandidateCriterionPrefix + tr.Digest(canonical)
			if c.ID != want || d.Links[0].CriterionID != want {
				t.Fatal("wrong complete criterion identity or link")
			}
			delete(criteria[0], "snapshotHeadArtifactSha256")
			oldCanonical, _ := json.Marshal(criteria[0])
			if c.ID == cw.CandidateCriterionPrefix+tr.Digest(oldCanonical) {
				t.Fatal("old candidate identity reused")
			}
			if len(d.Artifacts) != 6 || len(result.Verification.ArtifactChecks) != 6 {
				t.Fatal("six-role closure lost")
			}
			for _, pin := range r.inputs()[1:] {
				found := false
				for _, a := range d.Artifacts {
					if a.Sha256 == pin.Sha256 {
						b, e := Read(filepath.Join(out, a.Path))
						if e != nil || !bytes.Equal(b, mustReadStable(t, pin.Path)) {
							t.Fatal("original artifact bytes lost")
						}
						found = true
					}
				}
				if !found {
					t.Fatal("pinned artifact omitted")
				}
			}
			b, _ := json.Marshal(result.Verification)
			var full map[string]json.RawMessage
			_ = json.Unmarshal(b, &full)
			if len(full) != 21 {
				t.Fatalf("verification keys=%d", len(full))
			}
			for _, key := range []string{"nativeAuthority", "historicalValidity", "currentApplicability", "sourceGitBinding", "sourceInventoryCompleteness", "executionAtCommit", "runnerReceiptSemantics", "runnerExecution", "authentication", "dependencyClosure", "criterionAdequacy", "criterionDiscrimination", "externalInteroperability"} {
				if result.Verification.Axes[key] != "NOT_OBSERVED" {
					t.Fatalf("promoted %s", key)
				}
			}
		})
	}
}
func mustReadStable(t *testing.T, p string) []byte {
	t.Helper()
	b, e := Read(p)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

// These are manufactured 0.3 witness fixtures, not claims that the retained
// historical runner executed coverage or mutation experiments.
func TestStableAssemblyPreservesFull03Witnesses(t *testing.T) {
	for _, state := range []string{"observed", "not-run"} {
		t.Run(state, func(t *testing.T) {
			r, out := stableFixture(t, "sha1")
			var source map[string]any
			if e := json.Unmarshal(mustReadStable(t, r.SourceMap.Path), &source); e != nil {
				t.Fatal(e)
			}
			source["spec"] = cw.Spec03
			h := source["hunks"].([]any)[0].(map[string]any)
			coverage := map[string]any{"profileSha256": strings.Repeat("3", 64), "testRun": "manufactured fixture; execution NOT_OBSERVED", "mode": "set", "state": "covered", "covered": []any{map[string]any{"start": 1, "count": 1}}}
			discrimination := map[string]any{"treeRevision": r.Target, "selectionSha256": strings.Repeat("4", 64), "mutants": 3, "killed": 1, "survived": 1, "survivors": []any{map[string]any{"operator": "literal", "line": 1, "description": "one survivor and one uncompiled residual"}}, "bounds": map[string]any{"maxHunks": 2, "maxMutants": 3, "wallTimeSeconds": 10}, "state": "survived", "detail": "fixture only"}
			if state == "not-run" {
				coverage["state"] = "uncovered"
				coverage["covered"] = []any{}
				discrimination["mutants"] = 0
				discrimination["killed"] = 0
				discrimination["survived"] = 0
				discrimination["survivors"] = []any{}
				discrimination["state"] = "not-run"
			}
			h["coverage"] = coverage
			h["discriminates"] = discrimination
			r.SourceMap = save(t, r.SourceMap.Path, encode(t, source))
			before, e := cw.ParseMap(mustReadStable(t, r.SourceMap.Path))
			if e != nil {
				t.Fatal(e)
			}
			result, e := AssembleStable(t.Context(), r, out)
			if e != nil {
				t.Fatal(e)
			}
			d, e := cw.ParseStable(mustReadStable(t, result.StablePath))
			if e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(before.Hunks, d.Change.Hunks) || !reflect.DeepEqual(before.Evidence, d.Change.Evidence) || before.Spec != cw.Spec03 {
				t.Fatal("source vocabulary or witnesses changed")
			}
			if result.Verification.Axes["coverageValidation"] != "WIRE_VALIDATED_ONLY" || result.Verification.Axes["discriminationValidation"] != "WIRE_VALIDATED_ONLY" {
				t.Fatal("witness validation not retained")
			}
		})
	}
}
func TestStableAssemblyRetainsNativeFailureAndRetryBytes(t *testing.T) {
	r, out := stableFixture(t, "sha1")
	var receipt tr.ReceiptDocument
	if e := Decode(mustReadStable(t, r.RunnerReceipt.Path), &receipt); e != nil {
		t.Fatal(e)
	}
	receipt.Observation.Complete = false
	receipt.Observation.Tests = []tr.Test{{ID: "failed", State: tr.Failed, FailureKind: tr.Infrastructure, Attempts: []tr.Attempt{{State: tr.Failed, FailureKind: tr.BuildFailure, Message: "original attempt"}}}, {ID: "skipped", State: tr.Skipped}, {ID: "flaky", State: tr.Flaky}, {ID: "timeout", State: tr.TimedOut}, {ID: "interrupted", State: tr.Interrupted}}
	raw := encode(t, receipt)
	r.RunnerReceipt = save(t, r.RunnerReceipt.Path, raw)
	result, e := AssembleStable(t.Context(), r, out)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(raw, mustReadStable(t, filepath.Join(out, "artifacts/runner-receipt.json"))) {
		t.Fatal("failed/incomplete native bytes changed")
	}
	if result.Verification.Axes["criterionDiscrimination"] != "NOT_OBSERVED" || result.Verification.Axes["runnerExecution"] != "NOT_OBSERVED" {
		t.Fatal("reference became execution or discrimination")
	}
}
func TestStableAssemblyRefusesChangedOrUnsupportedInputs(t *testing.T) {
	for _, kind := range []string{"snapshot-pin", "snapshot-missing", "snapshot-symlink", "snapshot-git-path", "source-pin", "duplicate-link", "unresolved-hunk", "wrong-profile", "target-sidecar", "existing-output", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			r, out := stableFixture(t, "sha1")
			ctx := t.Context()
			switch kind {
			case "snapshot-pin":
				r.SnapshotHeadArtifact.Sha256 = strings.Repeat("0", 64)
			case "snapshot-missing":
				r.SnapshotHeadArtifact.Path = filepath.Join(realTemp(t), "missing")
			case "snapshot-symlink":
				p := filepath.Join(realTemp(t), "link")
				if e := os.Symlink(r.SnapshotHeadArtifact.Path, p); e != nil {
					t.Fatal(e)
				}
				r.SnapshotHeadArtifact.Path = p
			case "snapshot-git-path":
				dir := filepath.Join(realTemp(t), ".GiT")
				if e := os.Mkdir(dir, 0700); e != nil {
					t.Fatal(e)
				}
				r.SnapshotHeadArtifact = save(t, filepath.Join(dir, "receipt"), mustReadStable(t, r.SnapshotHeadArtifact.Path))
			case "source-pin":
				r.SourceMap.Sha256 = strings.Repeat("0", 64)
			case "duplicate-link":
				r.Links = append(r.Links, r.Links[0])
			case "unresolved-hunk":
				r.Links[0].HunkIDs = []string{cw.HunkPrefix + strings.Repeat("0", 64)}
			case "wrong-profile":
				r.Profile = Profile
			case "target-sidecar":
				p := filepath.Join(r.Repository, cw.ExcludedCEMPath)
				if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
					t.Fatal(e)
				}
				save(t, p, []byte("conflict"))
				git(t, r.Repository, "add", ".")
				git(t, r.Repository, "commit", "-qm", "sidecar conflict")
				r.Target = git(t, r.Repository, "rev-parse", "HEAD")
			case "existing-output":
				if e := os.Mkdir(out, 0700); e != nil {
					t.Fatal(e)
				}
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			result, e := AssembleStable(ctx, r, out)
			if e == nil || result.StablePath != "" || result.StableSha256 != "" {
				t.Fatalf("accepted invalid %s %s %v", kind, stableAssemblyDebug(result), e)
			}
			if _, e := os.Stat(filepath.Join(out, "stable.json")); !os.IsNotExist(e) {
				t.Fatal("invalid assembly published stable map")
			}
		})
	}
}

func TestStableAssemblyRetainsNarrowRepositoryEnvelope(t *testing.T) {
	r, out := stableFixture(t, "sha256")
	p := filepath.Join(r.Repository, ".git/config")
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	section := "[extensions]\n\tobjectformat = sha256\n"
	if !strings.HasSuffix(string(b), section) {
		t.Fatal("unexpected fixture config")
	}
	if e := os.WriteFile(p, []byte(section+strings.TrimSuffix(string(b), section)), 0600); e != nil {
		t.Fatal(e)
	}
	result, e := AssembleStable(t.Context(), r, out)
	if e == nil || result.StablePath != "" || result.Verification.Stage != "repository" || result.Verification.Code == nil || *result.Verification.Code != "unsupported-repository-envelope" {
		t.Fatalf("broadened qualification %s %v", stableAssemblyDebug(result), e)
	}
}
