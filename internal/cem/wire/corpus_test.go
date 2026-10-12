package wire

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// committedChanges is the repository's sealed CEM corpus.
var committedChanges = filepath.Join("..", "..", "..", ".corvint", "changes")

// migrationCommits are the DCW-V0-050 migration commits that wrote every
// migrated map (internal/dogfoodflow cemMigrationCommits). The converter is
// gone, so no further migration commit can be written and the list is frozen.
var migrationCommits = []string{
	"0b6f1bc245b44a8620c5e64f91e87f6d08da08d0",
	"3b86121f4ced8b90212eaab6644ffbb4c491710d",
}

// corpusFailures checks every <bind>.cem.json map under dir: each must read
// as stable cem/1.0, and a migrated map's provenance must name its own bind
// commit. A retired 0.x map fails with the stable reader's refusal.
func corpusFailures(dir string) (int, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, nil, err
	}
	n, failures := 0, []string{}
	for _, e := range entries {
		bind, ok := strings.CutSuffix(e.Name(), ".cem.json")
		if !ok {
			continue
		}
		n++
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return n, failures, err
		}
		document, err := ParseStable(data)
		switch {
		case err != nil:
			failures = append(failures, fmt.Sprintf("%s: %s: %v", e.Name(), cemcode.CodeOf(err), err))
		case document.Provenance != nil && document.Provenance.BindCommit != bind:
			failures = append(failures, fmt.Sprintf("%s: provenance names bind commit %s", e.Name(), document.Provenance.BindCommit))
		}
	}
	return n, failures, nil
}

// CEM-V1-201: every sealed CEM the repository commits is a stable cem/1.0 map,
// so a pre-cutover branch that lands a 0.x seal fails here. It replaces the
// retired converter's corpus check (TestCEMV1100ConvertCorpusIdempotent).
func TestCEMV1201CommittedCorpusIsStable(t *testing.T) {
	n, failures, err := corpusFailures(committedChanges)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("no committed sealed CEMs read")
	}
	if len(failures) > 0 {
		t.Fatalf("%d of %d committed sealed CEMs are not stable cem/1.0:\n%s", len(failures), n, strings.Join(failures, "\n"))
	}
}

// CEM-V1-201 (CEM-V1-230/231 kept without the converter): a migrated map's
// provenance names the source blob by Git object ID and SHA-256 and the map
// equals the blob its migration commit wrote, and a stable map without
// provenance equals its bind commit's sidecar. Both read Git objects, so a
// checkout without them reports NOT_RUN.
func TestCEMV1201CommittedCorpusBinding(t *testing.T) {
	entries, err := os.ReadDir(committedChanges)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.Command("git", "-C", committedChanges, "rev-parse", "--git-dir").Output(); err != nil {
		t.Skipf("NOT_RUN: no Git repository: %v", err)
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
		document, err := ParseStable(data)
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		if p := document.Provenance; p != nil {
			source := gitObject(t, p.SourceBlobOid)
			sum := sha256.Sum256(source)
			if hex.EncodeToString(sum[:]) != p.SourceSha256 || !RetiredSpec(p.SourceSpec) {
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
	t.Logf("migrated=%d native=%d", migrated, native)
}

// migratedUnchanged reports whether data equals the blob one migration commit
// holds for the sealed map name. A migration commit the checkout lacks is
// NOT_RUN; a commit without that path is simply not the one that wrote it.
func migratedUnchanged(t *testing.T, name string, data []byte) bool {
	t.Helper()
	for _, commit := range migrationCommits {
		if err := exec.Command("git", "-C", committedChanges, "cat-file", "-e", commit+"^{commit}").Run(); err != nil {
			t.Skipf("NOT_RUN: migration commit %s unavailable: %v", commit, err)
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
		t.Skipf("NOT_RUN: object %s unavailable: %v", name, err)
	}
	return out
}

// CEM-V1-200, CEM-V1-201: a late 0.x seal fails the corpus check with the
// stable reader's unsupported-spec refusal, which names the merge recovery
// and never a rebase.
func TestCEMV1200RetiredSpecRefusedWithMergeFix(t *testing.T) {
	dir := t.TempDir()
	name := firstSealed(t)
	stable, err := os.ReadFile(filepath.Join(committedChanges, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), stable, 0o600); err != nil {
		t.Fatal(err)
	}
	if n, failures, err := corpusFailures(dir); err != nil || n != 1 || len(failures) != 0 {
		t.Fatalf("control: n=%d failures=%v err=%v", n, failures, err)
	}
	for _, spec := range []string{Spec01, Spec02, Spec03} {
		late := []byte(`{"spec":"` + spec + `","baseRevision":"` + strings.Repeat("b", 40) + `","hunks":[]}`)
		_, err := ParseStable(late)
		if cemcode.CodeOf(err) != cemcode.UnsupportedSpec || !strings.Contains(err.Error(), spec+" is retired") || !strings.Contains(err.Error(), "fix: "+RetiredSpecFix) {
			t.Fatalf("%s: %v", spec, err)
		}
		for _, banned := range []string{"rebase", "force-push", "--force"} {
			if strings.Contains(err.Error(), banned) {
				t.Fatalf("%s: refusal names %q: %v", spec, banned, err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, strings.Repeat("c", 40)+".cem.json"), late, 0o600); err != nil {
			t.Fatal(err)
		}
		if n, failures, _ := corpusFailures(dir); n != 2 || len(failures) != 1 || !strings.Contains(failures[0], "unsupported-spec") {
			t.Fatalf("%s: late seal not caught: n=%d failures=%v", spec, n, failures)
		}
		// CEM-V1-203's recovery removes the retired seal, which clears the scan.
		if err := os.Remove(filepath.Join(dir, strings.Repeat("c", 40)+".cem.json")); err != nil {
			t.Fatal(err)
		}
		if n, failures, err := corpusFailures(dir); err != nil || n != 1 || len(failures) != 0 {
			t.Fatalf("%s: recovery: n=%d failures=%v err=%v", spec, n, failures, err)
		}
	}
	// Other unsupported specs keep the plain stable refusal.
	if _, err := ParseStable([]byte(`{"spec":"cem/0.4"}`)); cemcode.CodeOf(err) != cemcode.UnsupportedSpec || strings.Contains(err.Error(), "retired") {
		t.Fatalf("cem/0.4: %v", err)
	}
}

func firstSealed(t *testing.T) string {
	t.Helper()
	entries, err := os.ReadDir(committedChanges)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".cem.json") {
			return e.Name()
		}
	}
	t.Fatal("no committed sealed CEM")
	return ""
}
