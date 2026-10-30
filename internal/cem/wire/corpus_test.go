package wire

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// committedChanges is the repository's sealed CEM corpus.
var committedChanges = filepath.Join("..", "..", "..", ".corvint", "changes")

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
	for _, spec := range []string{"cem/0.1", "cem/0.2", "cem/0.3"} {
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
