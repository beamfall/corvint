package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/observations"
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

// CEM-V1-032: the cite step refuses a cem/1.0 map explicitly. It does not read
// it as a map that owes no hunk, so it never runs cem cite, writes no
// plan binding or cite log, and leaves the map unchanged.
func TestCiteStepRefusesStableMap(t *testing.T) {
	stable, err := os.ReadFile(stableCEMFixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range []string{"", "1\tintent.md\t1:1\tspecification\n"} {
		root := t.TempDir()
		mapPath := filepath.Join(root, ".corvint", "change.cem.json")
		if err := os.MkdirAll(filepath.Dir(mapPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(mapPath, stable, 0o600); err != nil {
			t.Fatal(err)
		}
		source := filepath.Join(root, "plan.tsv")
		if err := os.WriteFile(source, []byte(plan), 0o600); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		c := &change{flow: flow{ctx: context.Background(), root: root, stderr: &stderr}, evidence: root, cemPrepared: true, intentSnapshot: []byte("intent.md\n")}
		c.options.Citations = source
		c.citeStep()
		if len(c.rows) != 1 || c.rows[0].name != "cem-cite" || c.rows[0].status != "NOT_PRODUCED" || c.rows[0].reason != "cem-map-stable-unsupported" {
			t.Fatalf("plan %q: steps=%+v", plan, c.rows)
		}
		for _, absent := range []string{root + citationBinding, root + "/cem-cite.jsonl"} {
			if _, statErr := os.Lstat(absent); !os.IsNotExist(statErr) {
				t.Fatalf("plan %q: refusal wrote %s (%v)", plan, absent, statErr)
			}
		}
		if got, err := os.ReadFile(mapPath); err != nil || !bytes.Equal(got, stable) {
			t.Fatalf("plan %q: the stable map changed (%v)", plan, err)
		}
		if hint := c.fixHint(c.rows[0]); !strings.Contains(hint, "cem/1.0") {
			t.Fatalf("fix=%q", hint)
		}
		if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".corvint/\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		row := c.rows[0]
		if err := observations.Append(root, observations.Event{Kind: "dogfood-step", Step: row.name, Status: row.status, Reason: row.reason}); err != nil {
			t.Fatalf("the refusal is lost at the observation boundary: %v", err)
		}
	}
}
