package corpusbinding

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// committedChanges is the repository's sealed CEM corpus. One literal names it,
// so the affected planner bounds this undeclared package's reads by path.
var committedChanges = filepath.FromSlash("../../../.corvint/changes")

// The converter is retired; DCW-V0-050's migration list is frozen.
var migrationCommits = []string{
	"0b6f1bc245b44a8620c5e64f91e87f6d08da08d0",
	"3b86121f4ced8b90212eaab6644ffbb4c491710d",
}

// CEM-V1-201/230/231: every map retains its original source/sidecar binding.
// Decision 0587's exact historical fixtures use committed original Git objects
// in an isolated repository. All other maps use the current repository only.
// Missing current proof still fails CI=true rather than counting a skip as a run.
func TestCEMV1201CommittedCorpusBinding(t *testing.T) {
	history, err := openHistoryFixture(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	counts, err := verifyCorpus(committedChanges, committedChanges, history)
	if err != nil {
		reportBindingError(t, err)
	}
	t.Logf("historical migrated=%d native=%d; current migrated=%d native=%d; historical binding grants no current-change coverage",
		counts.historicalMigrated, counts.historicalNative, counts.currentMigrated, counts.currentNative)
	if counts.currentMigrated+counts.currentNative == 0 {
		t.Log("current binding NOT_OBSERVED: no current-lineage maps")
	}
}

type bindingCounts struct {
	historicalMigrated, historicalNative, currentMigrated, currentNative int
}

func verifyCorpus(repo, changes string, history *historyFixture) (bindingCounts, error) {
	var counts bindingCounts
	retained, err := history.rootFixtures(repo)
	if err != nil {
		return counts, err
	}
	// Enumerate the pinned root, not the mutable directory: deletion must not
	// shrink the historical set that this test verifies.
	for name, want := range retained {
		got, err := os.ReadFile(filepath.Join(changes, name))
		if err != nil {
			return counts, fmt.Errorf("historical fixture missing %s: %w", name, err)
		}
		if !bytes.Equal(got, want) {
			return counts, fmt.Errorf("historical fixture changed: %s", name)
		}
	}
	entries, err := os.ReadDir(changes)
	if err != nil {
		return counts, err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".cem.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(changes, e.Name()))
		if err != nil {
			return counts, err
		}
		objectRepo := repo
		_, historical := retained[e.Name()]
		if historical {
			objectRepo = history.repo
		}
		migrated, err := verifyBinding(objectRepo, e.Name(), data)
		if err != nil {
			return counts, err
		}
		switch {
		case historical && migrated:
			counts.historicalMigrated++
		case historical:
			counts.historicalNative++
		case migrated:
			counts.currentMigrated++
		default:
			counts.currentNative++
		}
	}
	return counts, nil
}

func verifyBinding(repo, name string, data []byte) (bool, error) {
	bind := strings.TrimSuffix(name, ".cem.json")
	document, err := wire.ParseStable(data)
	if err != nil {
		return false, fmt.Errorf("%s: %w", name, err)
	}
	if p := document.Provenance; p != nil {
		if p.BindCommit != bind {
			return false, fmt.Errorf("%s: provenance bindCommit differs from file name", name)
		}
		source, err := readGitBlob(repo, p.SourceBlobOid)
		if err != nil {
			return false, err
		}
		sum := sha256.Sum256(source)
		if hex.EncodeToString(sum[:]) != p.SourceSha256 || !wire.RetiredSpec(p.SourceSpec) {
			return false, fmt.Errorf("%s: provenance does not name its source blob", name)
		}
		for _, commit := range migrationCommits {
			if _, err := fixtureGit(repo, nil, 1024, "cat-file", "-e", commit+"^{commit}"); err != nil {
				return false, &proofUnavailable{fmt.Errorf("migration commit %s unavailable: %w", commit, err)}
			}
			blob, err := readGitBlob(repo, commit+":.corvint/changes/"+name)
			if err == nil && bytes.Equal(blob, data) {
				return true, nil
			}
		}
		return false, fmt.Errorf("%s: migrated map differs from every migration commit's blob", name)
	}
	sidecar, err := readGitBlob(repo, bind+":.corvint/change.cem.json")
	if err != nil {
		return false, err
	}
	if !bytes.Equal(sidecar, data) {
		return false, fmt.Errorf("%s: stable map without provenance differs from its bind commit's sidecar", name)
	}
	return false, nil
}

type proofUnavailable struct{ error }

func reportBindingError(t *testing.T, err error) {
	t.Helper()
	var absent *proofUnavailable
	if errors.As(err, &absent) {
		notRun(t, "%v", err)
	}
	t.Fatal(err)
}

// notRun preserves V1-1334: absent proof skips only outside CI.
func notRun(t *testing.T, format string, args ...any) {
	t.Helper()
	if os.Getenv("CI") == "true" {
		t.Fatalf("CI=true: "+format, args...)
	}
	t.Skipf("NOT_RUN: "+format, args...)
}
