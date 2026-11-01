package corpusbinding

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// lineageRegister is decision 0588's register of the committed maps whose
// base no retained ref reaches (V1-3758). V1-3852 reuses it as the
// historical-only disposition: a fresh clone cannot replay these maps.
var lineageRegister = filepath.FromSlash("../../../.corvint/cem-lineage.tsv")

// historicalOnly reads the register as map target -> label.
func historicalOnly() (map[string]string, error) {
	data, err := os.ReadFile(lineageRegister)
	if err != nil {
		return nil, err
	}
	listed := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			return nil, fmt.Errorf("%s: row %q has %d fields, want 4", lineageRegister, line, len(fields))
		}
		switch fields[2] {
		case "archive-lineage", "rebased-lane-base", "base-unavailable":
			listed[fields[0]] = fields[2]
		default:
			return nil, fmt.Errorf("%s: %s has unknown label %q", lineageRegister, fields[0], fields[2])
		}
	}
	return listed, nil
}

// CEM-V1-1020/1021 (V1-3852): every committed map's cited objects (the bind
// commit its file name names, baseRevision, each evidence blob and a migrated
// map's source blob) are reachable from HEAD, so a fresh clone replays it,
// unless the lineage register labels the map. Missing history is
// NOT_RUN outside CI, as for the binding test.
func TestCEMV11020CommittedMapObjectsReplayable(t *testing.T) {
	if out, err := fixtureGit(committedChanges, nil, 64, "rev-parse", "--is-shallow-repository"); err != nil || strings.TrimSpace(string(out)) != "false" {
		notRun(t, "full history unavailable (shallow=%q err=%v)", strings.TrimSpace(string(out)), err)
	}
	reachable, err := reachableObjects(committedChanges)
	if err != nil {
		notRun(t, "%v", err)
	}
	listed, err := historicalOnly()
	if err != nil {
		t.Fatal(err)
	}
	problems, replayable, err := checkReplayable(committedChanges, reachable, listed)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
	t.Logf("replayable=%d historical-only=%d objects-reachable=%d", replayable, len(listed), len(reachable))
}

// CEM-V1-1021: the check names an unlisted map with an unreachable object and
// a stale historical-only entry.
func TestCEMV11021ReplayCheckControls(t *testing.T) {
	dir := t.TempDir()
	data, doc := stableFixture(t)
	bind := "1111111111111111111111111111111111111111"
	if err := os.WriteFile(filepath.Join(dir, bind+".cem.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	base := doc.Change.BaseRevision
	reachable := map[string]bool{bind: true, base: true}
	for _, e := range doc.Change.Evidence {
		reachable[e.BlobOid] = true
	}
	problems, replayable, err := checkReplayable(dir, reachable, nil)
	if err != nil || len(problems) != 0 || replayable != 1 {
		t.Fatalf("complete map: problems=%v replayable=%d err=%v", problems, replayable, err)
	}
	delete(reachable, base)
	problems, _, _ = checkReplayable(dir, reachable, nil)
	if len(problems) != 1 || !strings.Contains(problems[0], bind+".cem.json baseRevision "+base) {
		t.Fatalf("missing base not named: %v", problems)
	}
	problems, _, _ = checkReplayable(dir, reachable, map[string]string{bind: "reason"})
	if len(problems) != 0 {
		t.Fatalf("listed map reported: %v", problems)
	}
	reachable[base] = true
	stale := "2222222222222222222222222222222222222222"
	problems, _, _ = checkReplayable(dir, reachable, map[string]string{bind: "reason", stale: ""})
	if len(problems) != 3 || !strings.Contains(strings.Join(problems, "\n"), bind+" is replayable") ||
		!strings.Contains(strings.Join(problems, "\n"), stale+" names no committed map") ||
		!strings.Contains(strings.Join(problems, "\n"), stale+" has no reason") {
		t.Fatalf("stale entries not named: %v", problems)
	}
}

// stableFixture is a committed native map with evidence, to build controls from.
func stableFixture(t *testing.T) ([]byte, *wire.StableMap) {
	t.Helper()
	entries, err := os.ReadDir(committedChanges)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(committedChanges, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if doc, err := wire.ParseStable(data); err == nil && doc.Provenance == nil && len(doc.Change.Evidence) > 0 {
			return data, doc
		}
	}
	t.Fatal("no native committed map with evidence")
	return nil, nil
}

func reachableObjects(repo string) (map[string]bool, error) {
	out, err := fixtureGit(repo, nil, 256<<20, "rev-list", "--objects", "HEAD")
	if err != nil {
		return nil, &proofUnavailable{err}
	}
	set := make(map[string]bool)
	for _, line := range bytes.Split(out, []byte("\n")) {
		if oid, _, _ := bytes.Cut(line, []byte(" ")); len(oid) > 0 {
			set[string(oid)] = true
		}
	}
	return set, nil
}

// checkReplayable returns one problem line per unreachable cited object of an
// unlisted map and per stale register row, sorted.
func checkReplayable(changes string, reachable map[string]bool, listed map[string]string) ([]string, int, error) {
	entries, err := os.ReadDir(changes)
	if err != nil {
		return nil, 0, err
	}
	var problems []string
	seen := make(map[string]bool)
	replayable := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".cem.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(changes, name))
		if err != nil {
			return nil, 0, err
		}
		doc, err := wire.ParseStable(data)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", name, err)
		}
		bind := strings.TrimSuffix(name, ".cem.json")
		seen[bind] = true
		cited := [][2]string{{"bind commit", bind}, {"baseRevision", doc.Change.BaseRevision}}
		for _, e := range doc.Change.Evidence {
			cited = append(cited, [2]string{"evidence " + e.ID + " blob", e.BlobOid})
		}
		if p := doc.Provenance; p != nil {
			cited = append(cited, [2]string{"provenance source blob", p.SourceBlobOid})
		}
		var missing []string
		for _, c := range cited {
			if !reachable[c[1]] {
				missing = append(missing, fmt.Sprintf("%s %s %s unreachable from HEAD", name, c[0], c[1]))
			}
		}
		_, isListed := listed[bind]
		switch {
		case len(missing) == 0 && isListed:
			problems = append(problems, fmt.Sprintf("cem-lineage.tsv %s is replayable; remove the entry", bind))
		case len(missing) == 0:
			replayable++
		case !isListed:
			problems = append(problems, missing...)
		}
	}
	for bind, reason := range listed {
		if !seen[bind] {
			problems = append(problems, fmt.Sprintf("cem-lineage.tsv %s names no committed map", bind))
		}
		if reason == "" {
			problems = append(problems, fmt.Sprintf("cem-lineage.tsv %s has no reason", bind))
		}
	}
	sort.Strings(problems)
	return problems, replayable, nil
}
