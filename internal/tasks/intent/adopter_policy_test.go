package intent_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func policyWith(t *testing.T, key, raw string) (*intent.Policy, error) {
	t.Helper()
	v := fixture.PolicyValue()
	x, err := wire.ParseInput([]byte(raw))
	if err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
	v.Obj.Set(key, x)
	return intent.DecodePolicy(wire.EncodeFile(v))
}

// TestKHNV0028_PolicyKnowHowScreen: knowHow.screen is optional, names a
// repository-relative values file and/or a sorted opt-in subset of EMAIL,
// PHONE and CARD, and refuses an empty, unknown or unsorted declaration.
func TestKHNV0028_PolicyKnowHowScreen(t *testing.T) {
	p, err := policyWith(t, "knowHow", `{"workerAdd":false}`)
	if err != nil || p.ValueScreen() != nil {
		t.Fatalf("no screen: %v %+v", err, p.ValueScreen())
	}
	p, err = policyWith(t, "knowHow", `{"workerAdd":false,"screen":{"denyValuesFile":"secrets/deny.env","classes":["CARD","EMAIL"]}}`)
	if err != nil || p.ValueScreen() == nil || *p.ValueScreen().DenyValuesFile != "secrets/deny.env" || len(p.ValueScreen().Classes) != 2 {
		t.Fatalf("screen: %v %+v", err, p)
	}
	p, err = policyWith(t, "knowHow", `{"workerAdd":false,"screen":{"denyValuesFile":null,"classes":["PHONE"]}}`)
	if err != nil || p.ValueScreen().DenyValuesFile != nil {
		t.Fatalf("classes only: %v", err)
	}
	for _, bad := range []string{
		`{"workerAdd":false,"screen":{"denyValuesFile":null,"classes":[]}}`,
		`{"workerAdd":false,"screen":{"denyValuesFile":null,"classes":["SSN"]}}`,
		`{"workerAdd":false,"screen":{"denyValuesFile":null,"classes":["PHONE","EMAIL"]}}`,
		`{"workerAdd":false,"screen":{"denyValuesFile":"/etc/deny","classes":[]}}`,
		`{"workerAdd":false,"screen":{"classes":["EMAIL"]}}`,
		`{"workerAdd":false,"screen":{"denyValuesFile":null,"classes":["EMAIL"],"values":["x"]}}`,
	} {
		if _, err := policyWith(t, "knowHow", bad); wire.CodeOf(err) != wire.CodeMalformed {
			t.Fatalf("%s: want MALFORMED, got %v", bad, err)
		}
	}
}

// TestTWTV0001_PolicyWorktrees: worktrees names a repository, a base ref and
// a branch pattern holding {ticket} exactly once; anything else refuses.
func TestTWTV0001_PolicyWorktrees(t *testing.T) {
	p, err := policyWith(t, "worktrees", `{"repository":".","base":"main","branchPattern":"ticket/{ticket}"}`)
	if err != nil || p.Worktrees == nil || p.Worktrees.BranchPattern != "ticket/{ticket}" || p.Worktrees.Repository != "." {
		t.Fatalf("worktrees: %v %+v", err, p)
	}
	if p, err = policyWith(t, "worktrees", `{"repository":"/srv/app","base":"origin/main","branchPattern":"claude/{ticket}-work"}`); err != nil || p.Worktrees.Repository != "/srv/app" {
		t.Fatalf("absolute repository: %v", err)
	}
	for _, bad := range []string{
		`{"repository":".","base":"main","branchPattern":"ticket/fixed"}`,
		`{"repository":".","base":"main","branchPattern":"{ticket}/{ticket}"}`,
		`{"repository":".","base":"main","branchPattern":"t/{other}"}`,
		`{"repository":".","base":"main","branchPattern":"-{ticket}"}`,
		`{"repository":".","base":"main","branchPattern":"a..b/{ticket}"}`,
		`{"repository":".","base":"a..b","branchPattern":"t/{ticket}"}`,
		`{"repository":".","base":"-x","branchPattern":"t/{ticket}"}`,
		`{"repository":"../up","base":"main","branchPattern":"t/{ticket}"}`,
		`{"repository":".","base":"main"}`,
	} {
		if _, err := policyWith(t, "worktrees", bad); wire.CodeOf(err) != wire.CodeMalformed {
			t.Fatalf("%s: want MALFORMED, got %v", bad, err)
		}
	}
}

// TestTDRV0001_PolicyDefects: defects is optional; prefix defaults to DEF and
// path to the private register; a bad prefix or path refuses.
func TestTDRV0001_PolicyDefects(t *testing.T) {
	p, err := intent.DecodePolicy(fixture.PolicyBytes())
	if err != nil || p.Defects != nil || p.DefectPrefix() != intent.DefaultDefectPrefix {
		t.Fatalf("default: %v", err)
	}
	if p, err = policyWith(t, "defects", `{}`); err != nil || p.Defects == nil || p.DefectPrefix() != "DEF" || p.Defects.Path != nil {
		t.Fatalf("empty defects: %v", err)
	}
	if p, err = policyWith(t, "defects", `{"prefix":"BUG","path":"qa/defects.jsonl"}`); err != nil || p.DefectPrefix() != "BUG" || *p.Defects.Path != "qa/defects.jsonl" {
		t.Fatalf("declared defects: %v", err)
	}
	for _, bad := range []string{`{"prefix":"bug"}`, `{"prefix":"B-1"}`, `{"path":"/abs"}`, `{"other":"1"}`} {
		if _, err := policyWith(t, "defects", bad); wire.CodeOf(err) != wire.CodeMalformed {
			t.Fatalf("%s: want MALFORMED, got %v", bad, err)
		}
	}
}

// TestNTPV0026_PolicyReservationsSharedPaths: reservations.sharedPaths is
// optional, a sorted unique non-empty list of repository paths, and refuses
// an empty, unsorted or unknown declaration.
func TestNTPV0026_PolicyReservationsSharedPaths(t *testing.T) {
	p, err := intent.DecodePolicy(wire.EncodeFile(fixture.PolicyValue()))
	if err != nil || p.SharedReservationPaths() != nil {
		t.Fatalf("absent: %v %v", err, p.SharedReservationPaths())
	}
	p, err = policyWith(t, "reservations", `{"sharedPaths":["docs/specs/INDEX.json","docs/specs/README.md","docs/specs/REQUIREMENTS.tsv"]}`)
	if err != nil || len(p.SharedReservationPaths()) != 3 {
		t.Fatalf("declared: %v %+v", err, p)
	}
	for _, raw := range []string{
		`{"sharedPaths":[]}`,
		`{"sharedPaths":["docs/specs/README.md","docs/specs/INDEX.json"]}`,
		`{"sharedPaths":["/abs"]}`,
		`{"sharedPaths":["a"],"extra":true}`,
		`{}`,
	} {
		if _, err := policyWith(t, "reservations", raw); err == nil {
			t.Errorf("%s accepted", raw)
		}
	}
}
