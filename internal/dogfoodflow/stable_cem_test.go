package dogfoodflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stableCEMFixture is the public sealed positive cem/1.0 map; its hunks carry
// supported and mechanical dispositions a loose 0.x decode would count.
const stableCEMFixture = "../../protocol/cem-1.0/stable/repository-envelope-packet/inherited/fixtures/maps/sha1-positive-sealed.json"

// CEM-V1-032: the bind loop reads no hunks from a map that declares cem/1.0,
// so a stable map never counts as cited or binds a citation plan.
func TestMapHunksRefusesStableMap(t *testing.T) {
	stable, err := os.ReadFile(stableCEMFixture)
	if err != nil {
		t.Fatal(err)
	}
	if got := mapHunks(stable); len(got) != 0 {
		t.Fatalf("mapHunks decoded %d hunks from a cem/1.0 map", len(got))
	}
	path := filepath.Join(t.TempDir(), "change.cem.json")
	if err := os.WriteFile(path, stable, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := citedHunks(path); got != 0 {
		t.Fatalf("citedHunks = %d for a cem/1.0 map; want 0", got)
	}
	// CEM-V1-033: only the exact declaration is refused; any other spec keeps
	// the earlier layout-agnostic decode.
	for _, spec := range []string{`"cem/1.0 "`, `"cem/1.0-experimental.1"`, `"cem/0.2"`, `10`} {
		data := []byte(`{"spec": ` + spec + `, "hunks": [{"disposition": "cited", "id": "hunk:a", "path": "a"}]}`)
		if got := mapHunks(data); len(got) != 1 || got[0]["id"] != "hunk:a" {
			t.Errorf("spec %s: mapHunks = %v; want one hunk", spec, got)
		}
	}
}

// CEM-V1-032: batch-check refuses a sealed cem/1.0 constituent with its own
// code before any verifier run, never as a malformed 0.x map.
func TestBatchCheckRefusesStableConstituent(t *testing.T) {
	repo := newBatchRepo(t)
	bind, seal := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", `{"spec": "cem/1.0", "baseRevision": "`+repo.base+`", "hunks": []}`+"\n")
	repo.batch(seal)
	code, _, stderr := repo.check()
	if code != 1 || !strings.Contains(stderr, "FAIL constituent-cem-stable-unsupported count=1\n  bind="+bind+"\n") ||
		strings.Contains(stderr, "constituent-cem-invalid") || len(repo.args) != 0 {
		t.Fatalf("cem/1.0 exit=%d stderr=%s argv=%q", code, stderr, repo.args)
	}
	if !strings.Contains(stderr, "fix: "+batchRecovery["constituent-cem-stable-unsupported"]+"\n") || batchRecovery["constituent-cem-stable-unsupported"] == "" {
		t.Fatalf("missing recovery line: %s", stderr)
	}
}
