package workflow

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// readStableAt parses the cem/1.0 map at relative and requires its bytes to be
// the canonical stable encoding.
func readStableAt(t *testing.T, root, relative string) *wire.StableMap {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	if !wire.DeclaresStable(data) {
		t.Fatalf("%s does not declare cem/1.0: %.120s", relative, data)
	}
	document, err := wire.ParseStable(data)
	if err != nil {
		t.Fatalf("%s: %v", relative, err)
	}
	encoded, err := wire.EncodeStable(document)
	if err != nil || !bytes.Equal(encoded, data) {
		t.Fatalf("%s is not the canonical stable encoding (%v)", relative, err)
	}
	return document
}

// stableStatus reads one map through the Core stable status with the
// repository root as its artifact root and the full target object id.
func stableStatus(t *testing.T, root, relative, base, target string, limits PolicyLimits) map[string]any {
	t.Helper()
	result, err := openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: relative, ExpectedBase: base, Target: target, Limits: limits,
		ArtifactRoot: root, ArtifactsGiven: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func stableOutcome(result map[string]any) string {
	verification, _ := result["stableVerification"].(StableVerification)
	return verification.Result.Outcome
}

func intLimit(value int) *int { return &value }

// TestCEMV1140ProducerSpecSelection pins the --spec selection: an empty spec
// and cem/1.0 write a cem/1.0 producer map, a retired cem/0.x profile is
// refused with unsupported-spec and the migration fix, and any other value is
// refused as invalid arguments, before anything is written (CEM-V1-216).
func TestCEMV1140ProducerSpecSelection(t *testing.T) {
	t.Parallel()
	root, base, target := makeRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	readStableAt(t, root, wire.ExcludedCEMPath)
	beginPatch(t, root, base, target)
	if _, err := openSession(t, root).Begin(ctx(), BeginOptions{PatchPath: "exact.patch", Base: base, Output: "default.cem.json"}); err != nil {
		t.Fatal(err)
	}
	readStableAt(t, root, "default.cem.json")
	for _, c := range []struct{ spec, code, fragment string }{
		{"cem/0.1", cemcode.UnsupportedSpec, "fix: " + wire.RetiredSpecFix},
		{"cem/0.2", cemcode.UnsupportedSpec, "fix: " + wire.RetiredSpecFix},
		{"cem/0.3", cemcode.UnsupportedSpec, "fix: " + wire.RetiredSpecFix},
		{"cem/1.0 ", cemcode.InvalidArguments, "--spec must be cem/1.0"},
		{wire.CandidateSpec, cemcode.InvalidArguments, "--spec must be cem/1.0"},
	} {
		fresh, freshBase, freshTarget := makeRepo(t)
		_, err := openSession(t, fresh).Prepare(ctx(), PrepareOptions{Base: freshBase, Target: freshTarget, Spec: c.spec})
		if cemcode.CodeOf(err) != c.code || !strings.Contains(err.Error(), c.fragment) {
			t.Fatalf("prepare --spec %q: %v", c.spec, err)
		}
		if _, statErr := os.Lstat(filepath.Join(fresh, filepath.FromSlash(wire.ExcludedCEMPath))); !os.IsNotExist(statErr) {
			t.Fatalf("prepare --spec %q wrote a map (%v)", c.spec, statErr)
		}
		_, err = openSession(t, root).Begin(ctx(), BeginOptions{PatchPath: "exact.patch", Base: base, Output: "refused.cem.json", Spec: c.spec})
		if cemcode.CodeOf(err) != c.code || !strings.Contains(err.Error(), c.fragment) {
			t.Fatalf("begin --spec %q: %v", c.spec, err)
		}
		if _, statErr := os.Lstat(filepath.Join(root, "refused.cem.json")); !os.IsNotExist(statErr) {
			t.Fatalf("begin --spec %q wrote a map (%v)", c.spec, statErr)
		}
	}
}

// TestCEMV1141StablePrepareWritesResumesAndReplaces is the stable prepare
// positive path and its refusal controls.
func TestCEMV1141StablePrepareWritesResumesAndReplaces(t *testing.T) {
	t.Parallel()
	root, base, target := makeRepo(t)
	prepared, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, Spec: wire.StableSpec})
	if err != nil {
		t.Fatal(err)
	}
	document := readStableAt(t, root, wire.ExcludedCEMPath)
	if len(document.Change.Hunks) != 2 || document.Role != "" || len(document.Criteria) != 0 {
		t.Fatalf("stable producer map = %+v", document)
	}
	if prepared["resumed"] != false {
		t.Fatalf("first prepare resumed: %v", prepared)
	}
	actions, _ := prepared["nextActions"].([]any)
	if want := fmt.Sprint(stableNextAction(base, target, root)); len(actions) != 1 || fmt.Sprint(actions[0]) != want {
		t.Fatalf("nextActions = %v, want [%s]", actions, want)
	}
	if !strings.Contains(fmt.Sprint(actions[0]), "--artifacts "+root) {
		t.Fatalf("next action lacks the absolute artifact root: %v", actions[0])
	}
	if status := stableStatus(t, root, wire.ExcludedCEMPath, base, target, PolicyLimits{}); stableOutcome(status) != "ACCEPT" {
		t.Fatalf("prepared map does not verify: %v", status)
	}
	written, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath)))
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, Spec: wire.StableSpec})
	if err != nil || resumed["resumed"] != true {
		t.Fatalf("resume: %v %v", err, resumed)
	}
	requireUntouched(t, root, wire.ExcludedCEMPath, written)

	// A retired 0.x map at the sidecar is outdated by its spec until --replace.
	legacyRoot, legacyBase, legacyTarget := makeRepo(t)
	legacy := []byte(`{"spec":"cem/0.2","hunks":[]}` + "\n")
	writeFile(t, legacyRoot, wire.ExcludedCEMPath, string(legacy))
	_, err = openSession(t, legacyRoot).Prepare(ctx(), PrepareOptions{Base: legacyBase, Target: legacyTarget, Spec: wire.StableSpec})
	if cemcode.CodeOf(err) != cemcode.MapUnavailable || cemcode.GuidanceOf(err) != "the existing map records a different spec; pass --replace to regenerate" {
		t.Fatalf("0.x sidecar under a stable prepare: %v (guidance %q)", err, cemcode.GuidanceOf(err))
	}
	requireUntouched(t, legacyRoot, wire.ExcludedCEMPath, legacy)
	if _, err := openSession(t, legacyRoot).Prepare(ctx(), PrepareOptions{Base: legacyBase, Target: legacyTarget, Spec: wire.StableSpec, Replace: true}); err != nil {
		t.Fatal(err)
	}
	readStableAt(t, legacyRoot, wire.ExcludedCEMPath)

	// Bytes that are neither a cem/0.x nor a cem/1.0 map keep the invalid
	// document refusal, not the different-spec one.
	invalidRoot, invalidBase, invalidTarget := makeRepo(t)
	invalid := []byte(`{"spec":"cem/9.9"}` + "\n")
	writeFile(t, invalidRoot, wire.ExcludedCEMPath, string(invalid))
	_, err = openSession(t, invalidRoot).Prepare(ctx(), PrepareOptions{Base: invalidBase, Target: invalidTarget, Spec: wire.StableSpec})
	if cemcode.CodeOf(err) != cemcode.MapUnavailable || cemcode.GuidanceOf(err) != "the existing map is not a valid CEM document; pass --replace to regenerate" {
		t.Fatalf("invalid sidecar under a stable prepare: %v (guidance %q)", err, cemcode.GuidanceOf(err))
	}
	requireUntouched(t, invalidRoot, wire.ExcludedCEMPath, invalid)

	// A stable map carrying criteria, receipts, links or artifacts is never
	// replaced; an explicit legacy prepare never replaces any stable map.
	bound := stableMapBytes(t)
	for _, spec := range []string{wire.StableSpec, ""} {
		boundRoot, boundBase, boundTarget := makeRepo(t)
		writeFile(t, boundRoot, wire.ExcludedCEMPath, string(bound))
		_, err := openSession(t, boundRoot).Prepare(ctx(), PrepareOptions{Base: boundBase, Target: boundTarget, Spec: spec, Replace: true})
		requireStableRefusal(t, err, "prepare", boundRoot, wire.ExcludedCEMPath, bound)
	}
}

// TestCEMV1141StablePrepareGuardsBackups: a stable prepare restores or removes
// an interrupted-publication backup only when it is a stable producer map (or
// 0.x); a bound stable backup or an unreadable one is refused before recovery
// or resume cleanup, and neither the backup nor the sidecar changes.
func TestCEMV1141StablePrepareGuardsBackups(t *testing.T) {
	t.Parallel()
	backup := wire.ExcludedCEMPath + ".bak-0123456789abcdef"
	bound := stableMapBytes(t)
	prepareStable := func(t *testing.T) (string, string, string, []byte) {
		t.Helper()
		root, base, target := makeRepo(t)
		if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, Spec: wire.StableSpec}); err != nil {
			t.Fatal(err)
		}
		sidecar, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath)))
		if err != nil {
			t.Fatal(err)
		}
		return root, base, target, sidecar
	}
	backupAbsent := func(t *testing.T, root string) {
		t.Helper()
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(backup))); !os.IsNotExist(err) {
			t.Fatalf("backup survived (%v)", err)
		}
	}

	t.Run("producer backup removed on resume", func(t *testing.T) {
		root, base, target, sidecar := prepareStable(t)
		writeFile(t, root, backup, string(sidecar))
		result, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, Spec: wire.StableSpec})
		if err != nil || result["resumed"] != true {
			t.Fatalf("resume over a producer backup: %v %v", err, result)
		}
		requireUntouched(t, root, wire.ExcludedCEMPath, sidecar)
		backupAbsent(t, root)
	})

	t.Run("producer backup restored", func(t *testing.T) {
		root, base, target, sidecar := prepareStable(t)
		full := filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))
		if err := os.Rename(full, filepath.Join(root, filepath.FromSlash(backup))); err != nil {
			t.Fatal(err)
		}
		result, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, Spec: wire.StableSpec})
		if err != nil || result["resumed"] != true {
			t.Fatalf("recovery of a producer backup: %v %v", err, result)
		}
		requireUntouched(t, root, wire.ExcludedCEMPath, sidecar)
		backupAbsent(t, root)
	})

	t.Run("bound backup beside a resumable sidecar", func(t *testing.T) {
		root, base, target, sidecar := prepareStable(t)
		writeFile(t, root, backup, string(bound))
		_, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, Spec: wire.StableSpec})
		requireStableRefusal(t, err, "prepare", root, backup, bound)
		requireUntouched(t, root, wire.ExcludedCEMPath, sidecar)
	})

	t.Run("bound backup with the sidecar missing", func(t *testing.T) {
		root, base, target := makeRepo(t)
		writeFile(t, root, backup, string(bound))
		_, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, Spec: wire.StableSpec, Replace: true})
		requireStableRefusal(t, err, "prepare", root, backup, bound)
		if _, statErr := os.Lstat(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))); !os.IsNotExist(statErr) {
			t.Fatalf("refusal restored the bound backup (%v)", statErr)
		}
	})

	t.Run("unreadable backup", func(t *testing.T) {
		root, base, target, sidecar := prepareStable(t)
		writeUnreadable(t, root, backup, bound)
		_, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, Spec: wire.StableSpec})
		requireUnresolvedRefusal(t, err, "prepare", root, backup, bound)
		requireUntouched(t, root, wire.ExcludedCEMPath, sidecar)
	})
}

// TestCEMV1142StableBeginWritesProducerMap is the stable begin positive path:
// the out-of-band patch yields a canonical cem/1.0 producer map that verifies,
// and a stable producer destination may be rewritten by a stable begin.
func TestCEMV1142StableBeginWritesProducerMap(t *testing.T) {
	t.Parallel()
	root, base, target := makeRepo(t)
	beginPatch(t, root, base, target)
	for range 2 {
		result, err := openSession(t, root).Begin(ctx(), BeginOptions{PatchPath: "exact.patch", Base: base, Output: "begin.cem.json", Spec: wire.StableSpec})
		if err != nil || result["ok"] != true {
			t.Fatalf("stable begin: %v %v", err, result)
		}
	}
	document := readStableAt(t, root, "begin.cem.json")
	if document.Change.BaseRevision != base || len(document.Change.Hunks) != 2 {
		t.Fatalf("stable begin map = %+v", document.Change)
	}
	if status := stableStatus(t, root, "begin.cem.json", base, target, PolicyLimits{}); stableOutcome(status) != "ACCEPT" {
		t.Fatalf("begin map does not verify: %v", status)
	}
	// Refusal control: a retired-profile begin never replaces the stable
	// producer map (CEM-V1-216).
	written, _ := os.ReadFile(filepath.Join(root, "begin.cem.json"))
	_, err := openSession(t, root).Begin(ctx(), BeginOptions{PatchPath: "exact.patch", Base: base, Output: "begin.cem.json", Spec: "cem/0.1"})
	if got, _ := os.ReadFile(filepath.Join(root, "begin.cem.json")); cemcode.CodeOf(err) != cemcode.UnsupportedSpec || !bytes.Equal(got, written) {
		t.Fatalf("retired begin over a stable map: %v", err)
	}
}

// TestCEMV1143StableProducerMutationsStayStable cites, marks, covers and
// discriminates a stable producer map: each write stays canonical cem/1.0,
// keeps the producer role, and the result verifies; a bound stable map is
// still refused (CEM-V1-143, CEM-V1-144).
func TestCEMV1143StableProducerMutationsStayStable(t *testing.T) {
	t.Parallel()
	root, base, target := makeGoRepo(t, importReorderTarget, whisperTarget)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, Spec: wire.StableSpec}); err != nil {
		t.Fatal(err)
	}
	// A producer map may carry a role; Core keeps it through every write.
	document := readStableAt(t, root, wire.ExcludedCEMPath)
	document.Role = "bind-chore"
	encoded, err := wire.EncodeStable(document)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, wire.ExcludedCEMPath, string(encoded))
	if _, err := openSession(t, root).Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "pkg/a.go", Lines: "1:2", Relation: "test-claim",
	}); err != nil {
		t.Fatal(err)
	}
	cited := readStableAt(t, root, wire.ExcludedCEMPath)
	if cited.Role != "bind-chore" || cited.Change.Hunks[0].Disposition != "supported" {
		t.Fatalf("cited stable map = role %q hunk %+v", cited.Role, cited.Change.Hunks[0])
	}
	writeFile(t, root, "cover.out", "mode: set\nexample.com/m/pkg/a.go:9.41,9.72 1 1\nexample.com/m/pkg/a.go:12.43,12.71 1 1\n")
	if _, err := openSession(t, root).Cover(ctx(), CoverOptions{
		MapPath: wire.ExcludedCEMPath, Coverprofile: "cover.out", TestRun: "go test ./pkg/...",
	}); err != nil {
		t.Fatal(err)
	}
	covered := readStableAt(t, root, wire.ExcludedCEMPath)
	if covered.Role != "bind-chore" || covered.Change.Hunks[0].Coverage == nil {
		t.Fatalf("covered stable map = role %q coverage %+v", covered.Role, covered.Change.Hunks[0].Coverage)
	}
	if _, err := openSession(t, root).Discriminate(ctx(), DiscriminateOptions{MapPath: wire.ExcludedCEMPath, Target: "HEAD"}); err != nil {
		t.Fatal(err)
	}
	discriminated := readStableAt(t, root, wire.ExcludedCEMPath)
	if discriminated.Role != "bind-chore" || discriminated.Change.Hunks[0].Discriminates == nil {
		t.Fatalf("discriminated stable map = role %q witness %+v", discriminated.Role, discriminated.Change.Hunks[0].Discriminates)
	}
	if status := stableStatus(t, root, wire.ExcludedCEMPath, base, target, PolicyLimits{}); stableOutcome(status) != "ACCEPT" {
		t.Fatalf("mutated stable map does not verify: %v", status)
	}

	// Refusal control: the bound public map is refused by every writer.
	bound := stableMapBytes(t)
	writeFile(t, root, "bound.cem.json", string(bound))
	for tool, mutate := range stableMutators() {
		requireStableRefusal(t, mutate(openSession(t, root), "bound.cem.json", ""), tool, root, "bound.cem.json", bound)
	}
}

// TestCEMV1144StableStructuralMark marks a structural reason on a stable
// producer map: the map stays cem/1.0 (no cem/0.3 upgrade), the verifier
// proves the import reorder, and a wrong-class claim is rejected
// unproven-mechanical by the stable status rather than accepted.
func TestCEMV1144StableStructuralMark(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		reason, outcome string
		exit            int
	}{
		{"import-reorder", "ACCEPT", 0},
		{"rename", "REJECT", 1},
	} {
		root, base, target := makeGoRepo(t, importReorderBase, importReorderTarget)
		if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, Spec: wire.StableSpec}); err != nil {
			t.Fatal(err)
		}
		if _, err := openSession(t, root).Mark(ctx(), MarkOptions{
			MapPath: wire.ExcludedCEMPath, Hunk: "1", Disposition: "mechanical", Reason: c.reason, Output: witnessMap,
		}); err != nil {
			t.Fatalf("%s: %v", c.reason, err)
		}
		marked := readStableAt(t, root, witnessMap)
		if hunk := marked.Change.Hunks[0]; hunk.Disposition != "mechanical" || hunk.Reason != c.reason {
			t.Fatalf("%s: marked hunk %+v", c.reason, hunk)
		}
		status := stableStatus(t, root, witnessMap, base, target, PolicyLimits{MaxUnknown: intLimit(0), MaxMechanical: intLimit(1)})
		verification, _ := status["stableVerification"].(StableVerification)
		if verification.Result.Outcome != c.outcome || verification.Exit != c.exit {
			t.Fatalf("%s: outcome %s exit %d, want %s %d", c.reason, verification.Result.Outcome, verification.Exit, c.outcome, c.exit)
		}
		if c.exit != 0 && (status["ok"] != false || fmt.Sprint(verification.Result.IssueCodes) != "["+cemcode.UnprovenMechanical+"]") {
			t.Fatalf("%s: wrong-class claim not rejected: ok %v issues %v", c.reason, status["ok"], verification.Result.IssueCodes)
		}
	}
}
