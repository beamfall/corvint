package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestCEMV1636StatusNamesThePolicyVerdict checks CEM-V1-636: with ceilings,
// status names the ceiling verdict as policyState beside state, which stays
// the structural verifier verdict; without ceilings no policyState appears.
func TestCEMV1636StatusNamesThePolicyVerdict(t *testing.T) {
	f := materializeStableRead(t)
	exit, envelope, refusal := runCoreRead(t, f.repo, coreStableArguments("status", f, f.artifacts)...)
	if exit != 0 || refusal != nil {
		t.Fatalf("baseline: exit %d refusal %v", exit, refusal)
	}
	if _, present := envelope["policyState"]; present {
		t.Fatalf("a status without ceilings carries policyState: %v", envelope["policyState"])
	}
	for _, c := range []struct {
		name   string
		extra  []string
		exit   int
		ok     bool
		policy string
	}{
		{"within", []string{"--max-unknown", "0", "--max-mechanical", "1"}, 0, true, "within-ceilings"},
		{"exceeded", []string{"--max-mechanical", "0"}, 1, false, "ceiling-exceeded"},
	} {
		exit, envelope, refusal := runCoreRead(t, f.repo, append(coreStableArguments("status", f, f.artifacts), c.extra...)...)
		if exit != c.exit || refusal != nil || envelope["ok"] != c.ok {
			t.Fatalf("%s: exit %d ok %v refusal %v", c.name, exit, envelope["ok"], refusal)
		}
		if envelope["state"] != "accepted" || envelope["policyState"] != c.policy {
			t.Fatalf("%s: state %v policyState %v, want accepted and %s", c.name, envelope["state"], envelope["policyState"], c.policy)
		}
	}
}

// TestCEMV1637ReportAppliesCeilings checks CEM-V1-637: cem report admits the
// status ceilings in every format. An exceeded ceiling makes the envelope ok
// false and exits 1 with the counts, policyIssues and policyState of status,
// and the embedded verifier result is unmodified; verify still refuses.
func TestCEMV1637ReportAppliesCeilings(t *testing.T) {
	f := materializeStableRead(t)
	_, plain, _ := runCoreRead(t, f.repo, append(coreStableArguments("report", f, f.artifacts), "--format", "json")...)
	report := func(extra ...string) []string {
		return append(coreStableArguments("report", f, f.artifacts), extra...)
	}
	for _, c := range []struct {
		name      string
		arguments []string
		exit      int
		policy    string
	}{
		{"json within", report("--format", "json", "--max-unknown", "0", "--max-mechanical", "1"), 0, "within-ceilings"},
		{"json exceeded", report("--format", "json", "--max-mechanical", "0"), 1, "ceiling-exceeded"},
		{"markdown exceeded", report("--max-mechanical", "0"), 1, "ceiling-exceeded"},
	} {
		exit, envelope, refusal := runCoreRead(t, f.repo, c.arguments...)
		if exit != c.exit || refusal != nil || envelope["ok"] != (c.exit == 0) || envelope["policyState"] != c.policy {
			t.Fatalf("%s: exit %d envelope %v refusal %v", c.name, exit, envelope, refusal)
		}
		counts, _ := envelope["counts"].(map[string]any)
		if counts["mechanical"] != float64(1) || counts["unknown"] != float64(0) {
			t.Fatalf("%s: counts = %v", c.name, counts)
		}
		if issues, _ := envelope["policyIssues"].([]any); (len(issues) != 0) != (c.exit != 0) {
			t.Fatalf("%s: policyIssues = %v", c.name, issues)
		}
		if !reflect.DeepEqual(envelope["stableVerification"], plain["stableVerification"]) {
			t.Fatalf("%s: ceilings changed the embedded verifier result", c.name)
		}
	}
	// The exceeded markdown report still publishes the full review.
	if _, err := os.Stat(filepath.Join(f.repo, ".git", "corvint", "cem-review.md")); err != nil {
		t.Fatalf("exceeded markdown report did not publish: %v", err)
	}
	exit, envelope, refusal := runCoreRead(t, f.repo, append(coreStableArguments("verify", f, f.artifacts), "--max-unknown", "0")...)
	if exit != 2 || envelope != nil || refusal == nil {
		t.Fatalf("verify ceiling: exit %d envelope %v refusal %v", exit, envelope, refusal)
	}
}

// TestCCPV0010StatusAndReportAlwaysEmitCounts checks CCP-V0-010: status and a
// JSON or Markdown report without ceilings carry the map's per-disposition
// counts beside the unmodified verifier result, with no policyIssues or
// policyState and the verifier's own exit; verify carries no counts.
func TestCCPV0010StatusAndReportAlwaysEmitCounts(t *testing.T) {
	f := materializeStableRead(t)
	_, verified, _ := runCoreRead(t, f.repo, coreStableArguments("verify", f, f.artifacts)...)
	if _, present := verified["counts"]; present {
		t.Fatalf("verify carries counts: %v", verified["counts"])
	}
	for _, c := range []struct {
		name      string
		arguments []string
	}{
		{"status", coreStableArguments("status", f, f.artifacts)},
		{"report json", append(coreStableArguments("report", f, f.artifacts), "--format", "json")},
		{"report markdown", coreStableArguments("report", f, f.artifacts)},
	} {
		exit, envelope, refusal := runCoreRead(t, f.repo, c.arguments...)
		if exit != 0 || refusal != nil || envelope["ok"] != true {
			t.Fatalf("%s: exit %d ok %v refusal %v", c.name, exit, envelope["ok"], refusal)
		}
		want := map[string]any{"total": float64(2), "supported": float64(1), "unknown": float64(0), "mechanical": float64(1)}
		if !reflect.DeepEqual(envelope["counts"], want) {
			t.Fatalf("%s: counts = %v, want %v", c.name, envelope["counts"], want)
		}
		for _, key := range []string{"policyIssues", "policyState"} {
			if _, present := envelope[key]; present {
				t.Fatalf("%s: a read without ceilings carries %s", c.name, key)
			}
		}
		if !reflect.DeepEqual(envelope["stableVerification"], verified["stableVerification"]) {
			t.Fatalf("%s: counts changed the embedded verifier result", c.name)
		}
	}
}
