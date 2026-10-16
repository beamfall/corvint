package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCEMV1241PointerProblems proves the dangling-pointer refusal on a
// synthetic protocol tree: a sibling-digest pointer and a "../" path pointer
// must resolve to a file with the pinned SHA-256, a CEM map's artifact paths
// are not pointers, and a recorded dangling pointer is accepted only while it
// still dangles.
func TestCEMV1241PointerProblems(t *testing.T) {
	write := func(t *testing.T, root, rel, body string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	plan := "{}\n"
	planSum := digest([]byte(plan))
	cases := []struct {
		name     string
		doc      string
		withPlan bool
		recorded map[string]string
		want     string
	}{
		{"resolves", `{"parentPlan":"../plan.json","parentPlanSHA256":"` + planSum + `"}`, true, nil, ""},
		{"dangling", `{"parentPlan":"../plan.json","parentPlanSHA256":"` + planSum + `"}`, false, nil, "dangling pointer"},
		{"digest", `{"parentPlan":"../plan.json","parentPlanSHA256":"00"}`, true, nil, "does not match its pinned SHA-256"},
		{"path-dangling", `{"review":{"path":"../plan.json","sha256":"` + planSum + `"}}`, false, nil, "dangling pointer"},
		{"escape", `{"parentPlan":"../../../../x.json","parentPlanSHA256":"00"}`, false, nil, "leaves the repository"},
		{"cem-map", `{"spec":"cem/1.0","artifacts":[{"path":"../outside.json","sha256":"00"}]}`, false, nil, ""},
		{"recorded", `{"parentPlan":"../plan.json","parentPlanSHA256":"` + planSum + `"}`, false,
			map[string]string{"protocol/cem-1.0/s/m.json#/parentPlan": "../plan.json"}, ""},
		{"recorded-resolves", `{"parentPlan":"../plan.json","parentPlanSHA256":"` + planSum + `"}`, true,
			map[string]string{"protocol/cem-1.0/s/m.json#/parentPlan": "../plan.json"}, "now resolves"},
		{"recorded-gone", `{}`, false,
			map[string]string{"protocol/cem-1.0/s/m.json#/parentPlan": "../plan.json"}, "no longer present"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "protocol/cem-1.0/s/m.json", tc.doc)
			if tc.withPlan {
				write(t, root, "protocol/cem-1.0/plan.json", plan)
			}
			got := strings.Join(pointerProblems(root, tc.recorded), "\n")
			if tc.want == "" && got != "" || tc.want != "" && !strings.Contains(got, tc.want) {
				t.Fatalf("pointerProblems = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCEMV1241RepositoryPointersRecorded pins the repository's known dangling
// pointers: every one is recorded (decision 0531) and nothing else dangles.
func TestCEMV1241RepositoryPointersRecorded(t *testing.T) {
	if problems := pointerProblems(repoRoot, recordedDanglingPointers); len(problems) != 0 {
		t.Fatalf("protocol pointers: %v", problems)
	}
	if problems := pointerProblems(repoRoot, nil); len(problems) != len(recordedDanglingPointers) {
		t.Fatalf("unrecorded scan found %d dangling pointers, want the %d recorded: %v", len(problems), len(recordedDanglingPointers), problems)
	}
}
