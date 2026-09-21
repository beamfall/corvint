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
	want := make([]string, 50)
	for i := range want {
		want[i] = fmt.Sprintf("WQO-V0-%03d", i+1)
	}
	slices.Sort(requirements)
	if !slices.Equal(requirements, want) {
		t.Fatalf("OCM enumerates %v; want exactly %v", requirements, want)
	}
}

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
	cases := []struct{ name string }{
		{name: "SOL-V0-001"}, {name: "SOL-V0-002"}, {name: "SOL-V0-003"},
		{name: "SOL-V0-004"}, {name: "SOL-V0-005"}, {name: "SOL-V0-006"},
		{name: "SOL-V0-007"}, {name: "SOL-V0-008"}, {name: "SOL-V0-009"},
		{name: "SOL-V0-010"},
	}
	if len(requirements) != len(cases) {
		t.Fatalf("OCM enumerates %v; want all ten SOL-V0 clauses", requirements)
	}
	for i, test := range cases {
		t.Run("registry lists "+test.name, func(t *testing.T) {
			if requirements[i] != test.name {
				t.Errorf("obligation %d = %s, want %s", i, requirements[i], test.name)
			}
		})
	}
}

func TestImprovementLoopSpecsEnumerateEveryObligation(t *testing.T) {
	cases := []struct {
		path string
		want []string
	}{
		{"docs/specs/retrieval-bench-diagnostics-v0.md", []string{"RBD-V0-001", "RBD-V0-002", "RBD-V0-003", "RBD-V0-004", "RBD-V0-005", "RBD-V0-006", "RBD-V0-007"}},
		{"docs/specs/source-documentation-draft-v0.md", []string{"SDD-V0-001", "SDD-V0-002", "SDD-V0-003", "SDD-V0-004", "SDD-V0-005", "SDD-V0-006", "SDD-V0-007", "SDD-V0-008", "SDD-V0-009", "SDD-V0-010"}},
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
