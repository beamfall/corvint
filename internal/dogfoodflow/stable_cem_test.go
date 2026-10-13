package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/observations"
)

// stableCEMFixture is the public sealed positive cem/1.0 map; its hunks carry
// supported and mechanical dispositions a loose 0.x decode would count.
const stableCEMFixture = "../../protocol/cem-1.0/stable/repository-envelope-packet/inherited/fixtures/maps/sha1-positive-sealed.json"

// CEM-V1-147 (amends CEM-V1-032): the bind loop reads the hunks of a map that
// declares cem/1.0 like a 0.x map, so it counts the cited ones and binds a
// citation plan to them.
func TestMapHunksReadsStableMap(t *testing.T) {
	stable, err := os.ReadFile(stableCEMFixture)
	if err != nil {
		t.Fatal(err)
	}
	document, err := wire.ParseStable(stable)
	if err != nil {
		t.Fatal(err)
	}
	got := mapHunks(stable)
	if len(got) != len(document.Change.Hunks) || len(got) == 0 {
		t.Fatalf("mapHunks decoded %d hunks from a cem/1.0 map; want %d", len(got), len(document.Change.Hunks))
	}
	cited := 0
	for i, hunk := range document.Change.Hunks {
		if got[i]["id"] != hunk.ID || got[i]["path"] != hunk.Path || got[i]["disposition"] != hunk.Disposition {
			t.Fatalf("hunk %d = %v; want %+v", i, got[i], hunk)
		}
		if hunk.Disposition != "unknown" {
			cited++
		}
	}
	path := filepath.Join(t.TempDir(), "change.cem.json")
	if err := os.WriteFile(path, stable, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := citedHunks(path); got != cited {
		t.Fatalf("citedHunks = %d for a cem/1.0 map; want %d", got, cited)
	}
	// Refusal control: a map that does not decode still has no hunks.
	for _, data := range []string{`{"spec": "cem/1.0", "hunks": {}}`, `not json`} {
		if got := mapHunks([]byte(data)); len(got) != 0 {
			t.Errorf("%s: mapHunks = %v; want none", data, got)
		}
	}
}

// stableStatusLine is the stable `cem status` line of an accepted map.
func stableStatusLine(base, bind string) string {
	return `{"counts":{"mechanical":0,"supported":1,"total":1,"unknown":0},"ok":true,"state":"accepted","tool":"cem-status",` +
		`"stableVerification":{"accept":true,"expectedBase":"` + base + `","outcome":"ACCEPT","targetRevision":"` + bind + `"}}` + "\n"
}

// CEM-V1-148 (amends CEM-V1-032): batch-check admits a sealed cem/1.0
// constituent when the verifier, run with the repository root as artifact
// root, reports a stable ACCEPT of exactly its base and bind; a 0.x status
// line never proves a stable map, and a 0.x seal is retired, never verified
// (CEM-V1-203).
func TestBatchCheckAdmitsStableConstituent(t *testing.T) {
	repo := newBatchRepo(t)
	bind, seal := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", `{"spec": "cem/1.0", "baseRevision": "`+repo.base+`", "hunks": []}`+"\n")
	repo.batch(seal)
	repo.output = func(base, bind string) string { return stableStatusLine(base, bind) }
	code, stdout, stderr := repo.check()
	root, _ := filepath.Abs(repo.root)
	want := []string{"cem", "status", "--map", ".corvint/changes/" + bind + ".cem.json", "--expected-base", repo.base, "--target", bind,
		"--max-unknown", "0", "--max-mechanical", "0", "--artifacts", root}
	if code != 0 || stderr != "" || len(repo.args) != 1 || strings.Join(repo.args[0], " ") != strings.Join(want, " ") ||
		!strings.Contains(stdout, "CONSTITUENT bind="+bind+" base="+repo.base+" hunks=1 supported=1 unknown=0 bootstrapUnknown=0 reasonedUnknown=0 state=accepted\n") {
		t.Fatalf("cem/1.0 exit=%d stdout=%s stderr=%s argv=%q", code, stdout, stderr, repo.args)
	}
	// Refusal controls: the 0.x line does not prove the stable map, a stable
	// REJECT or a different bind does not either.
	for name, output := range map[string]func(base, bind string) string{
		"legacy-line": func(base, bind string) string {
			return `{"counts":{"supported":1,"total":1,"unknown":0},"ok":true,"state":"ready-for-ci","tool":"cem-status",` +
				`"verification":{"assurance":"canonical","baseRevision":"` + base + `","targetRevision":"` + bind + `","valid":true}}` + "\n"
		},
		"reject": func(base, bind string) string {
			return strings.Replace(stableStatusLine(base, bind), `"outcome":"ACCEPT"`, `"outcome":"REJECT"`, 1)
		},
		"other-bind": func(base, _ string) string { return stableStatusLine(base, repo.base) },
	} {
		repo.output = output
		if code, _, stderr := repo.check(); code != 1 || !strings.Contains(stderr, "FAIL constituent-unverified count=1\n  bind="+bind) {
			t.Fatalf("%s: exit=%d stderr=%s", name, code, stderr)
		}
	}
}

// CEM-V1-147 (amends CEM-V1-032): the cite step treats a cem/1.0 map like a
// 0.x map: with no plan it reports the missing plan, never a stable refusal,
// and an unreadable plan path is refused before the map is touched.
func TestCiteStepAdmitsStableMap(t *testing.T) {
	stable, err := os.ReadFile(stableCEMFixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ plan, reason string }{
		{"", "citation-plan-not-provided"},
		{"missing", "citation-plan-unavailable"},
	} {
		root := t.TempDir()
		mapPath := filepath.Join(root, ".corvint", "change.cem.json")
		if err := os.MkdirAll(filepath.Dir(mapPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(mapPath, stable, 0o600); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		ch := &change{flow: flow{ctx: context.Background(), root: root, stderr: &stderr}, evidence: root, cemPrepared: true, intentSnapshot: []byte("intent.md\n")}
		if c.plan != "" {
			ch.options.Citations = filepath.Join(root, c.plan)
		}
		ch.citeStep()
		if len(ch.rows) != 1 || ch.rows[0].name != "cem-cite" || ch.rows[0].status != "NOT_PRODUCED" || ch.rows[0].reason != c.reason {
			t.Fatalf("plan %q: steps=%+v", c.plan, ch.rows)
		}
		if got, err := os.ReadFile(mapPath); err != nil || !bytes.Equal(got, stable) {
			t.Fatalf("plan %q: the stable map changed (%v)", c.plan, err)
		}
		if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".corvint/\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		row := ch.rows[0]
		if err := observations.Append(root, observations.Event{Kind: "dogfood-step", Step: row.name, Status: row.status, Reason: row.reason}); err != nil {
			t.Fatalf("the NOT_PRODUCED reason is lost at the observation boundary: %v", err)
		}
	}
}

// CEM-V1-146, CEM-V1-147, CEM-V1-224: the bind loop's and the check's `cem
// status` name the absolute repository root as --artifacts for every map, with
// no retired cem/0.x pass-through, and a stable prepare's different-spec
// refusal is an outdated map the loop regenerates with --replace.
func TestStableStatusArgumentsAddArtifactRoot(t *testing.T) {
	root := t.TempDir()
	args := []string{"cem", "status", "--map", "change.cem.json"}
	absolute, _ := filepath.Abs(root)
	if got := stableStatusArguments(root, args...); strings.Join(got, " ") != strings.Join(append(args, "--artifacts", absolute), " ") {
		t.Fatalf("stable arguments = %q", got)
	}
	if !outdatedCEMMaps[`{"code": "map-unavailable", "error": "cannot read CEM map: the existing map records a different spec; pass --replace to regenerate", "ok": false}`] {
		t.Fatal("the different-spec refusal is not an outdated map")
	}
}
