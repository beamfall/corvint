package verify

import (
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/sim"
)

type stableSimulationTestSource struct {
	blobs map[string][]byte
	modes map[string]string
	errs  map[string]error
	calls []string
}

func (s *stableSimulationTestSource) BaseBlob(path string) ([]byte, string, bool, error) {
	s.calls = append(s.calls, path)
	if err := s.errs[path]; err != nil {
		return nil, "", false, err
	}
	data, ok := s.blobs[path]
	if !ok {
		return nil, "", false, nil
	}
	mode := s.modes[path]
	if mode == "" {
		mode = "100644"
	}
	return data, mode, true, nil
}

func parseStableSimulationPatch(t *testing.T, raw string) *patch.Patch {
	t.Helper()
	parsed, err := patch.Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestStableSimulationCreateOverBaseDirectoryRejectsFromProof(t *testing.T) {
	parsed := parseStableSimulationPatch(t, ""+
		"diff --git a/p/old.txt b/p/old.txt\n"+
		"deleted file mode 100644\n"+
		"--- a/p/old.txt\n"+
		"+++ /dev/null\n"+
		"@@ -1 +0,0 @@\n"+
		"-old\n"+
		"diff --git a/p b/p\n"+
		"new file mode 100644\n"+
		"--- /dev/null\n"+
		"+++ b/p\n"+
		"@@ -0,0 +1 @@\n"+
		"+new\n")
	base := &stableSimulationTestSource{
		blobs: map[string][]byte{"p/old.txt": []byte("old\n")},
	}
	createAbsent, createBlocked, err := stableAuthenticatedCreateAbsences(parsed, map[string]gitauth.TreeEntry{"p": {OID: "tree", Type: "tree"}})
	if err != nil {
		t.Fatal(err)
	}
	source := &stableSimulationSource{source: base, authenticatedCreateAbsent: createAbsent, authenticatedCreateBlocked: createBlocked}
	if err := sim.Simulate(parsed, source); cemcode.CodeOf(err) != cemcode.InvalidField {
		t.Fatalf("simulate err = %v, want invalid-field", err)
	}
	if slices.Contains(base.calls, "p") {
		t.Fatalf("base source calls = %v, did not want destination p delegated", base.calls)
	}
}

func TestStableSimulationCreateOverBaseGitlinkRejectsFromProof(t *testing.T) {
	path := "p"
	parsed := &patch.Patch{Groups: []*patch.Group{
		{Kind: patch.KindCreate, NewPath: &path, Mode: "100644"},
		{Kind: patch.KindDelete, OldPath: &path, Mode: "160000"},
	}}
	base := &stableSimulationTestSource{}
	createAbsent, createBlocked, err := stableAuthenticatedCreateAbsences(parsed, map[string]gitauth.TreeEntry{path: {OID: "commit", Type: "commit"}})
	if err != nil {
		t.Fatal(err)
	}
	source := &stableSimulationSource{source: base, authenticatedCreateAbsent: createAbsent, authenticatedCreateBlocked: createBlocked}
	if _, _, _, err := source.BaseBlob(path); cemcode.CodeOf(err) != cemcode.InvalidField {
		t.Fatalf("BaseBlob err = %v, want invalid-field", err)
	}
	if len(base.calls) != 0 {
		t.Fatalf("base source calls = %v, want none", base.calls)
	}
}

func TestStableSimulationCreateDestinationsAuthenticateBaseEntries(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"empty-directory", "p"},
		{"nonempty-directory", "p"},
		{"gitlink", "p"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			parsed := &patch.Patch{Groups: []*patch.Group{
				{Kind: patch.KindCreate, NewPath: &testCase.path, Mode: "100644"},
			}}
			base := &stableSimulationTestSource{}
			createAbsent, createBlocked, err := stableAuthenticatedCreateAbsences(parsed, map[string]gitauth.TreeEntry{testCase.path: {OID: "base", Type: "tree"}})
			if err != nil {
				t.Fatal(err)
			}
			source := &stableSimulationSource{source: base, authenticatedCreateAbsent: createAbsent, authenticatedCreateBlocked: createBlocked}
			if _, _, _, err := source.BaseBlob(testCase.path); cemcode.CodeOf(err) != cemcode.InvalidField {
				t.Fatalf("BaseBlob err = %v, want invalid-field", err)
			}
			if len(base.calls) != 0 {
				t.Fatalf("base source calls = %v, want none", base.calls)
			}
		})
	}
}

func TestStableSimulationPlainAndNestedCreatesAuthenticateAbsence(t *testing.T) {
	cases := []string{"p", ".corvint/change.cem.json", "nested/p"}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			parsed := parseStableSimulationPatch(t, ""+
				"diff --git a/"+path+" b/"+path+"\n"+
				"new file mode 100644\n"+
				"--- /dev/null\n"+
				"+++ b/"+path+"\n"+
				"@@ -0,0 +1 @@\n"+
				"+new\n")
			base := &stableSimulationTestSource{}
			createAbsent, createBlocked, err := stableAuthenticatedCreateAbsences(parsed, map[string]gitauth.TreeEntry{path: {}})
			if err != nil {
				t.Fatal(err)
			}
			source := &stableSimulationSource{source: base, authenticatedCreateAbsent: createAbsent, authenticatedCreateBlocked: createBlocked}
			if err := sim.Simulate(parsed, source); err != nil {
				t.Fatal(err)
			}
			if len(base.calls) != 0 {
				t.Fatalf("base source calls = %v, want none", base.calls)
			}
		})
	}
}

func TestStableSimulationCreateMissingProofRefuses(t *testing.T) {
	path := "p"
	parsed := &patch.Patch{Groups: []*patch.Group{
		{Kind: patch.KindCreate, NewPath: &path, Mode: "100644"},
	}}
	if _, _, err := stableAuthenticatedCreateAbsences(parsed, nil); cemcode.CodeOf(err) != "unsupported-patch-inventory" {
		t.Fatalf("stableAuthenticatedCreateAbsences err = %v, want unsupported-patch-inventory", err)
	}
}

func TestStableSimulationCreateOverExistingBlobRejectsInvalidField(t *testing.T) {
	path := "p"
	parsed := parseStableSimulationPatch(t, ""+
		"diff --git a/p b/p\n"+
		"new file mode 100644\n"+
		"--- /dev/null\n"+
		"+++ b/p\n"+
		"@@ -0,0 +1 @@\n"+
		"+new\n")
	base := &stableSimulationTestSource{}
	createAbsent, createBlocked, err := stableAuthenticatedCreateAbsences(parsed, map[string]gitauth.TreeEntry{path: {OID: "blob", Type: "blob"}})
	if err != nil {
		t.Fatal(err)
	}
	source := &stableSimulationSource{source: base, authenticatedCreateAbsent: createAbsent, authenticatedCreateBlocked: createBlocked}
	if err := sim.Simulate(parsed, source); cemcode.CodeOf(err) != cemcode.InvalidField {
		t.Fatalf("simulate err = %v, want invalid-field", err)
	}
	if len(base.calls) != 0 {
		t.Fatalf("base source calls = %v, want none", base.calls)
	}
}
