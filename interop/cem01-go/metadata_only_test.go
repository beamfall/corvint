package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestMetadataOnlyChangeRefusalVectors pins CEM-V1-300..302 for the portable
// consumer over the vectors shared with internal/cem/patch: a headerless
// empty-file creation, empty-file deletion or mode-only group, alone or mixed
// with a text change, is the operational metadata-only-change-unsupported
// naming the kind and path; near misses keep their legacy code; and the frozen
// stable profile still reports the pre-V1-0609 issue code.
func TestMetadataOnlyChangeRefusalVectors(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("testdata", "metadata-only")
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases []struct {
			Name                string  `json:"name"`
			Patch               string  `json:"patch"`
			NativeCode          string  `json:"nativeCode"`
			PortableCode        string  `json:"portableCode"`
			PortableStableIssue string  `json:"portableStableIssue"`
			Kind                *string `json:"kind"`
			Path                *string `json:"path"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	refused := 0
	for _, tc := range manifest.Cases {
		patch, err := os.ReadFile(filepath.Join(dir, tc.Patch))
		if err != nil {
			t.Fatal(err)
		}
		_, e := parsePatch(patch)
		if e == nil {
			t.Fatalf("%s: accepted", tc.Name)
		}
		if e.code != tc.PortableCode {
			t.Fatalf("%s: code = %q, want %q", tc.Name, e.code, tc.PortableCode)
		}
		if got := e.stable().code; got != tc.PortableStableIssue {
			t.Fatalf("%s: stable code = %q, want %q", tc.Name, got, tc.PortableStableIssue)
		}
		if tc.PortableCode != "metadata-only-change-unsupported" {
			if e.detail != "" || e.stableCode != "" || tc.Kind != nil {
				t.Fatalf("%s: legacy refusal carries metadata-only detail %q", tc.Name, e.detail)
			}
			continue
		}
		refused++
		if tc.NativeCode != tc.PortableCode || !e.operational {
			t.Fatalf("%s: native %q, operational %v", tc.Name, tc.NativeCode, e.operational)
		}
		if want := *tc.Kind + " " + strconv.Quote(*tc.Path); e.detail != want {
			t.Fatalf("%s: detail = %q, want %q", tc.Name, e.detail, want)
		}
	}
	if refused < 5 {
		t.Fatalf("only %d metadata-only refusal vectors", refused)
	}
}
