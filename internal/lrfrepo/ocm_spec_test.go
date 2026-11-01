package lrfrepo

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Enumeration proves visibility, not implementation or execution of these clauses.
func TestWorkQueueSpecEnumeratesEveryObligation(t *testing.T) {
	path := "docs/specs/work-queue-observation-v0.md"
	data, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	_, requirements, _, err := requirementsFromBlob(path, "fixture-blob", data)
	if err != nil {
		t.Fatal(err)
	}
	want := indexedRequirements(t, path)
	if len(want) == 0 || !slices.Equal(slices.Sorted(slices.Values(requirements)), want) {
		t.Fatalf("OCM enumerates %v; REQUIREMENTS.tsv lists %v", requirements, want)
	}
}

// The expected SOL-V0 clauses come from docs/specs/REQUIREMENTS.tsv, which
// script/gen-spec-requirements.sh derives from the spec independently of the
// OCM parser (V1-2153): a new clause and its regenerated index row pass with no
// count to edit, and a clause the OCM no longer enumerates still fails.
func TestSelfObservationSpecEnumeratesEveryObligation(t *testing.T) {
	path := "docs/specs/self-observation-ledger-v0.md"
	data, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	_, requirements, _, err := requirementsFromBlob(path, "fixture-blob", data)
	if err != nil {
		t.Fatal(err)
	}
	want := indexedRequirements(t, path)
	if len(want) == 0 || !slices.Equal(slices.Sorted(slices.Values(requirements)), want) {
		t.Fatalf("OCM enumerates %v; REQUIREMENTS.tsv lists %v", requirements, want)
	}
	for _, id := range want {
		t.Run("registry lists "+id, func(t *testing.T) {
			if !slices.Contains(requirements, id) {
				t.Errorf("OCM does not enumerate %s", id)
			}
		})
	}
}

// indexedRequirements returns, sorted, the requirement IDs that
// docs/specs/REQUIREMENTS.tsv defines in path.
func indexedRequirements(t *testing.T, path string) []string {
	t.Helper()
	index, err := os.ReadFile("../../docs/specs/REQUIREMENTS.tsv")
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, row := range strings.Split(string(index), "\n")[1:] {
		if fields := strings.Split(row, "\t"); len(fields) > 1 && fields[1] == path {
			ids = append(ids, fields[0])
		}
	}
	slices.Sort(ids)
	return ids
}

func TestImprovementLoopSpecsEnumerateEveryObligation(t *testing.T) {
	cases := []struct {
		path string
		want []string
	}{
		{"docs/specs/retrieval-bench-diagnostics-v0.md", []string{"RBD-V0-001", "RBD-V0-002", "RBD-V0-003", "RBD-V0-004", "RBD-V0-005", "RBD-V0-006", "RBD-V0-007"}},
		{"docs/specs/source-documentation-draft-v0.md", []string{"SDD-V0-001", "SDD-V0-002", "SDD-V0-003", "SDD-V0-004", "SDD-V0-005", "SDD-V0-006", "SDD-V0-007", "SDD-V0-008", "SDD-V0-009", "SDD-V0-010", "SDD-V0-011", "SDD-V0-012", "SDD-V0-013", "SDD-V0-014", "SDD-V0-015", "SDD-V0-018", "SDD-V0-019", "SDD-V0-630", "SDD-V0-631", "SDD-V0-632", "SDD-V0-633", "SDD-V0-634"}},
		{"docs/specs/rust-analyzer-candidate-v0.md", []string{"RAC-001", "RAC-002", "RAC-003", "RAC-004", "RAC-005", "RAC-006", "RAC-007", "RAC-008", "RAC-009", "RAC-010", "RAC-011"}},
	}
	for _, test := range cases {
		t.Run(test.path, func(t *testing.T) {
			data, err := os.ReadFile("../../" + test.path)
			if err != nil {
				t.Fatal(err)
			}
			_, requirements, _, err := requirementsFromBlob(test.path, "fixture-blob", data)
			if err != nil {
				t.Fatal(err)
			}
			if len(requirements) != len(test.want) {
				t.Fatalf("OCM enumerates %v; want %v", requirements, test.want)
			}
			for i, want := range test.want {
				if requirements[i] != want {
					t.Errorf("obligation %d = %s, want %s", i, requirements[i], want)
				}
			}
		})
	}
}

func TestOCMSpecFrozenLineSyntaxMatchesEnumeration(t *testing.T) {
	t.Run("OCM-V0-001 the frozen requirement-line regex selects exactly the enumerated lines", func(t *testing.T) {
		path := "docs/specs/ocm-v0-dogfood.md"
		data, err := os.ReadFile("../../" + path)
		if err != nil {
			t.Fatal(err)
		}
		_, fence, found := strings.Cut(string(data), "Frozen requirement-line syntax (inline-code, bold-colon, or bold-period):\n\n```regex\n")
		if !found {
			t.Fatal("frozen requirement-line syntax block is absent")
		}
		pattern := regexp.MustCompile(strings.SplitN(fence, "\n", 2)[0])
		_, requirements, scope, err := requirementsFromBlob(path, "fixture-blob", data)
		if err != nil {
			t.Fatal(err)
		}
		matched := []string{}
		for _, line := range strings.Split(string(scope), "\n") {
			if groups := pattern.FindStringSubmatch(line); groups != nil {
				matched = append(matched, groups[1]+groups[2])
			}
		}
		if !slices.Equal(matched, requirements) {
			t.Fatalf("spec regex selects %v; parser enumerates %v", matched, requirements)
		}
	})
}

// V1-0521: the handoff slices once sat outside the selected section, so the
// session OCM counted 16 of the 22 registered clauses.
func TestSessionContextDividendSpecEnumeratesEveryObligation(t *testing.T) {
	path := "docs/specs/session-context-dividend-v0.md"
	data, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	_, requirements, _, err := requirementsFromBlob(path, "fixture-blob", data)
	if err != nil {
		t.Fatal(err)
	}
	want := make([]string, 22)
	for i := range want {
		want[i] = fmt.Sprintf("SESSION-V0-%03d", i+1)
	}
	if !slices.Equal(requirements, want) {
		t.Fatalf("OCM enumerates %v; want exactly %v", requirements, want)
	}
	registry, err := os.ReadFile("../../docs/specs/REQUIREMENTS.tsv")
	if err != nil {
		t.Fatal(err)
	}
	registered := []string{}
	for _, row := range strings.Split(string(registry), "\n") {
		if fields := strings.Split(row, "\t"); len(fields) >= 4 && fields[1] == path {
			registered = append(registered, fields[0])
		}
	}
	if !slices.Equal(registered, want) {
		t.Fatalf("registry locates %v in %s; want %v", registered, path, want)
	}
}

// CAL-V0-096 is defined under ## Requirements, so OCM enumerates it.
// Containment only: other agent-lease clauses are outside this check.
func TestAgentLeasesSpecEnumeratesCALV0096(t *testing.T) {
	path := "docs/specs/corvint-tasks-agent-leases-v0.md"
	data, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	_, requirements, _, err := requirementsFromBlob(path, "fixture-blob", data)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(requirements, "CAL-V0-096") {
		t.Fatalf("OCM enumerates %v; CAL-V0-096 missing", requirements)
	}
}

// CAL-V0-097 is defined inside the agent-leases Requirements section, so an
// OCM over that spec enumerates it.
func TestAgentLeasesSpecEnumeratesCALV0097(t *testing.T) {
	path := "docs/specs/corvint-tasks-agent-leases-v0.md"
	data, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	_, requirements, _, err := requirementsFromBlob(path, "fixture-blob", data)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(requirements, "CAL-V0-097") {
		t.Fatalf("OCM enumerates %v; want CAL-V0-097", requirements)
	}
}

// TestAgentLeasesSpecEnumeratesExecutionPrerequisites: the V1-0787
// amendment sits inside ## Requirements, so OCM enumerates CAL-V0-099.
// Enumeration proves visibility, not implementation.
func TestAgentLeasesSpecEnumeratesExecutionPrerequisites(t *testing.T) {
	path := "docs/specs/corvint-tasks-agent-leases-v0.md"
	data, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	_, requirements, _, err := requirementsFromBlob(path, "fixture-blob", data)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"CAL-V0-074", "CAL-V0-075", "CAL-V0-099"} {
		if !slices.Contains(requirements, id) {
			t.Errorf("OCM does not enumerate %s: %v", id, requirements)
		}
	}
}

// CAL-V0-098 (V1-0789) is defined under ## Requirements, so OCM enumerates it.
func TestAgentLeasesSpecEnumeratesCALV0098(t *testing.T) {
	path := "docs/specs/corvint-tasks-agent-leases-v0.md"
	data, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	_, requirements, _, err := requirementsFromBlob(path, "fixture-blob", data)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(requirements, "CAL-V0-098") {
		t.Fatalf("OCM enumerates %v; CAL-V0-098 missing", requirements)
	}
}

// CAL-V0-101 is defined inside the agent-leases Requirements section, so an
// OCM over that spec enumerates it.
func TestAgentLeasesSpecEnumeratesCALV0101(t *testing.T) {
	path := "docs/specs/corvint-tasks-agent-leases-v0.md"
	data, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	_, requirements, _, err := requirementsFromBlob(path, "fixture-blob", data)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(requirements, "CAL-V0-101") {
		t.Fatalf("OCM enumerates %v; want CAL-V0-101", requirements)
	}
}

// CAL-V0-102..103 (V1-0791) are defined inside the agent-leases
// Requirements section, so an OCM over that spec enumerates them.
func TestAgentLeasesSpecEnumeratesCALV0102(t *testing.T) {
	path := "docs/specs/corvint-tasks-agent-leases-v0.md"
	data, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	_, requirements, _, err := requirementsFromBlob(path, "fixture-blob", data)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"CAL-V0-102", "CAL-V0-103"} {
		if !slices.Contains(requirements, id) {
			t.Fatalf("OCM enumerates %v; want %s", requirements, id)
		}
	}
}

// CTW-V0-012..013 (V1-2165) are CAL-V0-073 and CAL-V0-106 re-homed into the
// intent worktree spec's Requirements section, which OCM enumerates; the
// agent-leases spec stays within the obligation bound without them.
func TestIntentWorktreeSpecEnumeratesRehomedCALV0073And106(t *testing.T) {
	path := "docs/specs/corvint-tasks-intent-worktree-v0.md"
	data, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	_, requirements, _, err := requirementsFromBlob(path, "fixture-blob", data)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"CTW-V0-012", "CTW-V0-013"} {
		if !slices.Contains(requirements, id) {
			t.Fatalf("OCM enumerates %v; want %s", requirements, id)
		}
	}
	leases := "docs/specs/corvint-tasks-agent-leases-v0.md"
	data, err = os.ReadFile("../../" + leases)
	if err != nil {
		t.Fatal(err)
	}
	if _, requirements, _, err = requirementsFromBlob(leases, "fixture-blob", data); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"CAL-V0-073", "CAL-V0-106"} {
		if slices.Contains(requirements, id) {
			t.Fatalf("OCM still enumerates retired alias %s in %s", id, leases)
		}
	}
}
