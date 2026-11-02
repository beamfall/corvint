package gitauth

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/testplatform"
)

// DCR-V1-985 (V1-2745): N LookupTreeEntry calls inside one object session
// start one `git cat-file` process, and every answer and the remaining
// operation budget equal the one-shot lookups'.
func TestDCRV1985SessionLookupsShareOneCatFile(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the counting host-git is a #!/bin/sh script")
	root, base, target := makeRepo(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	log := filepath.Join(outside, "calls")
	binary := filepath.Join(outside, "host-git")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> %s\nexec %s \"$@\"\n", quote(log), quote(realGit))
	if err := execfile.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	paths := []string{"docs/rule.txt", "docs/none", "docs/none/deeper/x", "f.go", "f.go/x", "none"}
	lookups := func(scoped bool) ([]string, int) {
		budget := gitrun.NewDefaultBudget()
		repository, err := OpenPinned(root, budget, binary)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(log, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if scoped {
			defer repository.BeginObjectSession()()
		}
		var got []string
		for range 10 {
			for _, revision := range []string{base, target} {
				for _, path := range paths {
					entry, exists, err := repository.LookupTreeEntry(context.Background(), revision, path)
					got = append(got, fmt.Sprintf("%s %s %v %v|%s|%v", revision, path, entry, exists, cemcode.CodeOf(err), err))
				}
			}
		}
		data, err := os.ReadFile(log)
		if err != nil {
			t.Fatal(err)
		}
		spawns := strings.Count(string(data), "cat-file")
		remaining := 0
		for ; ; remaining++ {
			if _, err := budget.ReserveOperation(0); err != nil {
				break
			}
		}
		return append(got, fmt.Sprint("remaining operations ", remaining)), spawns
	}
	oneShot, oneShotSpawns := lookups(false)
	scoped, scopedSpawns := lookups(true)
	if !slices.Equal(oneShot, scoped) {
		t.Fatalf("scoped lookups differ\none-shot: %q\nscoped:   %q", oneShot, scoped)
	}
	if want := 10 * 2 * len(paths); oneShotSpawns != want || scopedSpawns != 1 {
		t.Fatalf("cat-file spawns one-shot=%d (want %d) scoped=%d (want 1)", oneShotSpawns, want, scopedSpawns)
	}
}
