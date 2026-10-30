package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestCEMV1647MutatorsReadAnOutOfTreeMapIntoARelativeOutput pins CEM-V1-647:
// a mutator reads an absolute --map outside the repository and publishes only
// to the repository-relative --output named; without one, or with an absolute
// path inside the repository, it refuses invalid-arguments before writing,
// and the external map's bytes never change.
func TestCEMV1647MutatorsReadAnOutOfTreeMapIntoARelativeOutput(t *testing.T) {
	t.Parallel()
	root, base, target := makeCalcRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath)))
	if err != nil {
		t.Fatal(err)
	}
	outsideDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(outsideDir, "worker.cem.json")
	if err := os.WriteFile(outside, data, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := openSession(t, root).Mark(ctx(), MarkOptions{MapPath: outside, Hunk: "1", Disposition: "unknown", Reason: "insufficient-evidence", Output: "marked.cem.json"})
	if err != nil {
		t.Fatalf("mark over an out-of-tree map: %v", err)
	}
	if want := filepath.Join(root, "marked.cem.json"); result["map"] != want {
		t.Fatalf("mark wrote %v, want %s", result["map"], want)
	}
	if marked := readMapAt(t, root, "marked.cem.json"); marked.Hunks[0].Disposition != "unknown" {
		t.Fatalf("marked hunk %+v", marked.Hunks[0])
	}
	for name, run := range map[string]func() error{
		"mark in place": func() error {
			_, err := openSession(t, root).Mark(ctx(), MarkOptions{MapPath: outside, Hunk: "1", Disposition: "unknown", Reason: "insufficient-evidence"})
			return err
		},
		"discriminate in place": func() error {
			_, err := openSession(t, root).Discriminate(ctx(), DiscriminateOptions{MapPath: outside, Target: "HEAD"})
			return err
		},
		"inside the repository": func() error {
			_, err := openSession(t, root).Mark(ctx(), MarkOptions{MapPath: filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath)), Hunk: "1", Disposition: "unknown", Reason: "insufficient-evidence", Output: "again.cem.json"})
			return err
		},
	} {
		err := run()
		if err == nil || cemcode.CodeOf(err) != cemcode.InvalidArguments {
			t.Errorf("%s: error = %v, want invalid-arguments", name, err)
		}
		if name != "inside the repository" && !strings.Contains(err.Error(), "repository-relative --output") {
			t.Errorf("%s: error %q does not name the --output rule", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "again.cem.json")); !os.IsNotExist(err) {
		t.Fatalf("a refused mutator wrote its output: %v", err)
	}
	if after, err := os.ReadFile(outside); err != nil || string(after) != string(data) {
		t.Fatalf("the out-of-tree map changed: %v", err)
	}
}
