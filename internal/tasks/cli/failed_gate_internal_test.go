package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-322: missing evidence yields uncertainty, never a guessed tail. An
// attempt whose gate result record is absent reports NOT_OBSERVED; a failed
// result whose output blob is absent or differs from its recorded digest
// reports output.observed NOT_OBSERVED with null tail and failing line; a
// PASSED result is never the last failed gate.
func TestCALV0322_LastFailedGateUnknowns(t *testing.T) {
	repo := &intent.Repository{StateDir: t.TempDir()}
	put := func(raw []byte) wire.Digest {
		t.Helper()
		d := wire.Sum(raw)
		p := filepath.Join(repo.StateDir, "evidence", string(d))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		return d
	}
	output := []byte("compiling\nFAIL: x\n")
	tree, exit, clean := strings.Repeat("a", 40), wire.Count("1"), true
	record := func(state string, out []byte) wire.Digest {
		t.Helper()
		exit := exit
		if state == "PASSED" {
			exit = "0"
		}
		g := snapshot.GateResult{GateID: "docs", AttemptID: "attempt:acme:main:0123456789abcdef0123456789abcdef", Generation: "1", TicketRevision: "1", DefinitionSha256: wire.Sum(nil), CandidateTreeOid: tree, ExecutedTreeOid: &tree, PorcelainClean: &clean, InputsSha256: wire.Sum(nil), EnvironmentSha256: wire.Sum(nil), PolicySha256: wire.Sum(nil), ConfigSha256: wire.Sum(nil), StartedAt: "2026-10-23T00:00:00Z", EndedAt: "2026-10-23T00:00:01Z", State: state, OutcomeClass: "EXIT", ExitCode: &exit, Evidence: []snapshot.GateEvidence{{Label: "output", Sha256: wire.Sum(out), Bytes: wire.SizeOf(uint64(len(out)))}}}
		raw, err := g.Encode()
		if err != nil {
			t.Fatal(err)
		}
		return put(raw)
	}
	failed, passed := record("FAILED", output), record("PASSED", nil)
	show := func(results ...wire.Digest) wire.Value {
		a := &snapshot.Attempt{AttemptID: "attempt:acme:main:0123456789abcdef0123456789abcdef"}
		for _, d := range results {
			a.GateResults = append(a.GateResults, string(d))
		}
		return attemptLastFailedGate(repo, a)
	}
	if v := show(passed); v.Kind != wire.KindNull {
		t.Fatalf("a PASSED result is reported as failed: %s", wire.EncodeFile(v))
	}
	if v := show(failed, wire.Sum([]byte("absent record"))); v.Str != "NOT_OBSERVED" {
		t.Fatalf("an absent newer record: %s", wire.EncodeFile(v))
	}
	observed := func(v wire.Value) string {
		o, _ := v.Obj.Get("output")
		s, _ := o.Obj.Get("observed")
		return s.Str
	}
	v := show(failed, passed)
	if observed(v) != "NOT_OBSERVED" {
		t.Fatalf("an absent output blob: %s", wire.EncodeFile(v))
	}
	if tail, _ := v.Obj.Get("tail"); tail.Kind != wire.KindNull {
		t.Fatalf("an absent output blob quotes a tail: %s", wire.EncodeFile(v))
	}
	d := put(output)
	if v := show(failed, passed); observed(v) != "OBSERVED" {
		t.Fatalf("a retained output blob: %s", wire.EncodeFile(v))
	} else if line, _ := v.Obj.Get("failingLine"); line.Str != "FAIL: x" {
		t.Fatalf("failingLine %s", wire.EncodeFile(line))
	}
	if err := os.WriteFile(filepath.Join(repo.StateDir, "evidence", string(d)), []byte("altered!!!!!!!!!!!\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if v := show(failed); observed(v) != "NOT_OBSERVED" {
		t.Fatalf("an output blob that differs from its digest: %s", wire.EncodeFile(v))
	}
}
