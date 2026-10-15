package corpusbinding

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// committedChanges is the repository's sealed CEM corpus. One literal names it,
// so the affected planner bounds this undeclared package's reads by path.
var committedChanges = filepath.FromSlash("../../../.corvint/changes")

// migrationCommits are the DCW-V0-050 migration commits that wrote every
// migrated map (internal/dogfoodflow cemMigrationCommits). The converter is
// gone, so no further migration commit can be written and the list is frozen.
var migrationCommits = []string{
	"0b6f1bc245b44a8620c5e64f91e87f6d08da08d0",
	"3b86121f4ced8b90212eaab6644ffbb4c491710d",
}

// CEM-V1-201 (CEM-V1-230/231 kept without the converter): a migrated map's
// provenance names the source blob by Git object ID and SHA-256 and the map
// equals the blob its migration commit wrote, and a stable map without
// provenance equals its bind commit's sidecar. Both read Git objects, so a
// checkout without them reports NOT_RUN, except under CI=true, where a missing
// object fails: CI evidence must not count a skip as a run (V1-1334).
func TestCEMV1201CommittedCorpusBinding(t *testing.T) {
	entries, err := os.ReadDir(committedChanges)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.Command("git", "-C", committedChanges, "rev-parse", "--git-dir").Output(); err != nil {
		notRun(t, "no Git repository: %v", err)
	}
	migrated, native := 0, 0
	for _, e := range entries {
		bind, ok := strings.CutSuffix(e.Name(), ".cem.json")
		if !ok {
			continue
		}
		data, err := os.ReadFile(filepath.Join(committedChanges, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		document, err := wire.ParseStable(data)
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		if p := document.Provenance; p != nil {
			source := gitObject(t, p.SourceBlobOid)
			sum := sha256.Sum256(source)
			if hex.EncodeToString(sum[:]) != p.SourceSha256 || !wire.RetiredSpec(p.SourceSpec) {
				t.Fatalf("%s: provenance does not name its source blob", e.Name())
			}
			if !migratedUnchanged(t, e.Name(), data) {
				t.Fatalf("%s: migrated map differs from the blob every migration commit wrote at its path", e.Name())
			}
			migrated++
		} else if sidecar := gitObject(t, bind+":.corvint/change.cem.json"); !bytes.Equal(sidecar, data) {
			t.Fatalf("%s: stable map without provenance differs from its bind commit's sidecar", e.Name())
		} else {
			native++
		}
	}
	if migrated+native == 0 {
		t.Fatal("no committed sealed CEMs read")
	}
	t.Logf("migrated=%d native=%d", migrated, native)
}

// notRun skips with NOT_RUN outside CI and fails under CI=true.
func notRun(t *testing.T, format string, args ...any) {
	t.Helper()
	if os.Getenv("CI") == "true" {
		t.Fatalf("CI=true: "+format, args...)
	}
	t.Skipf("NOT_RUN: "+format, args...)
}

// migratedUnchanged reports whether data equals the blob one migration commit
// holds for the sealed map name. A migration commit the checkout lacks is
// NOT_RUN; a commit without that path is simply not the one that wrote it.
func migratedUnchanged(t *testing.T, name string, data []byte) bool {
	t.Helper()
	for _, commit := range migrationCommits {
		if err := exec.Command("git", "-C", committedChanges, "cat-file", "-e", commit+"^{commit}").Run(); err != nil {
			notRun(t, "migration commit %s unavailable: %v", commit, err)
		}
		blob, err := exec.Command("git", "-C", committedChanges, "cat-file", "blob", commit+":.corvint/changes/"+name).Output()
		if err == nil && bytes.Equal(blob, data) {
			return true
		}
	}
	return false
}

func gitObject(t *testing.T, name string) []byte {
	t.Helper()
	out, err := exec.Command("git", "-C", committedChanges, "cat-file", "blob", name).Output()
	if err != nil {
		notRun(t, "object %s unavailable: %v", name, err)
	}
	return out
}
