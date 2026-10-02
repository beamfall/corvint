package verify

import (
	"errors"
	"slices"
	"testing"

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

func TestStableSimulationCreateOverBaseDirectoryDelegatesToBaseSource(t *testing.T) {
	refusal := errors.New("base path is not a regular blob")
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
		errs:  map[string]error{"p": refusal},
	}
	source := &stableSimulationSource{source: base, authenticatedCreateAbsent: stableAuthenticatedCreateAbsences(parsed)}
	if err := sim.Simulate(parsed, source); !errors.Is(err, refusal) {
		t.Fatalf("simulate err = %v, want inherited base-source refusal", err)
	}
	if !slices.Contains(base.calls, "p") {
		t.Fatalf("base source calls = %v, want destination p delegated", base.calls)
	}
}

func TestStableSimulationCreateOverBaseGitlinkDelegatesToBaseSource(t *testing.T) {
	path := "p"
	parsed := &patch.Patch{Groups: []*patch.Group{
		{Kind: patch.KindCreate, NewPath: &path, Mode: "100644"},
		{Kind: patch.KindDelete, OldPath: &path, Mode: "160000"},
	}}
	if stableAuthenticatedCreateAbsences(parsed)[path] {
		t.Fatal("base-side exact path must prevent authenticated create absence")
	}
	refusal := errors.New("base path is not a regular blob")
	base := &stableSimulationTestSource{errs: map[string]error{path: refusal}}
	source := &stableSimulationSource{source: base, authenticatedCreateAbsent: stableAuthenticatedCreateAbsences(parsed)}
	if _, _, _, err := source.BaseBlob(path); !errors.Is(err, refusal) {
		t.Fatalf("BaseBlob err = %v, want inherited base-source refusal", err)
	}
	if got := base.calls; len(got) != 1 || got[0] != path {
		t.Fatalf("base source calls = %v, want [%s]", got, path)
	}
}

func TestStableSimulationPlainCreateDoesNotCallBaseSource(t *testing.T) {
	parsed := parseStableSimulationPatch(t, ""+
		"diff --git a/p b/p\n"+
		"new file mode 100644\n"+
		"--- /dev/null\n"+
		"+++ b/p\n"+
		"@@ -0,0 +1 @@\n"+
		"+new\n")
	base := &stableSimulationTestSource{}
	source := &stableSimulationSource{source: base, authenticatedCreateAbsent: stableAuthenticatedCreateAbsences(parsed)}
	if err := sim.Simulate(parsed, source); err != nil {
		t.Fatal(err)
	}
	if len(base.calls) != 0 {
		t.Fatalf("base source calls = %v, want none", base.calls)
	}
}
