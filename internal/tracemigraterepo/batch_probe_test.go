package tracemigraterepo

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// TestBatchedProbeMatchesOneShotProbes pins V1-3321: the batched probe (one
// cat-file --batch session and one rev-list) plans exactly what the one-shot
// probes plan, beyond the replay window (reachable, stranded) and inside it
// (canonical, legacy tree name, stranded), for the dry run and the apply.
func TestBatchedProbeMatchesOneShotProbes(t *testing.T) {
	for _, window := range []int{2, maximumAncestry} {
		results := map[bool][2]any{}
		for _, batched := range []bool{true, false} {
			savedWindow, savedBatched := maximumAncestry, batchedProbes
			maximumAncestry, batchedProbes = window, batched
			root := fastImportRepository(t, 8, 3)
			stranded := strings.Fields(git(t, root, "rev-list", "side", "^HEAD"))
			git(t, root, "update-ref", "-d", "refs/heads/side")
			names := append(strings.Fields(git(t, root, "rev-list", "--skip=1", "HEAD")), stranded...)
			if window != 2 {
				names = append(names[1:], git(t, root, "rev-parse", "HEAD~6^{tree}"))
			}
			directory := filepath.Join(root, ".context-corvint", "traces")
			if err := os.MkdirAll(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			for _, name := range names {
				if err := os.WriteFile(filepath.Join(directory, name+".jsonl"), traceRow(name), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			dry, err := Evaluate(context.Background(), root, Options{})
			if err != nil {
				t.Fatalf("window %d batched %v: dry run: %v", window, batched, err)
			}
			digest := dry.PlanDigest
			applied, err := Evaluate(context.Background(), root, Options{Apply: true, PlanDigest: &digest})
			if err != nil {
				t.Fatalf("window %d batched %v: apply: %v", window, batched, err)
			}
			maximumAncestry, batchedProbes = savedWindow, savedBatched
			if len(dry.StrandedRevisions) != len(stranded) {
				t.Fatalf("window %d batched %v: %d stranded, want %d", window, batched, len(dry.StrandedRevisions), len(stranded))
			}
			results[batched] = [2]any{dry, applied}
		}
		if !reflect.DeepEqual(results[true], results[false]) {
			t.Fatalf("window %d: batched probe changed the result:\nbatched  %+v\none-shot %+v", window, results[true], results[false])
		}
	}
}

// TestBatchedProbeChargesTheOperationBudget pins V1-3321's bound: every answer
// the session gives is reserved against the migration's Git operation budget,
// as the one-shot command it replaces was, so an exhausted budget refuses.
func TestBatchedProbeChargesTheOperationBudget(t *testing.T) {
	root := deepRepository(t, 1)
	head := git(t, root, "rev-parse", "HEAD")
	tree := git(t, root, "rev-parse", "HEAD^{tree}")
	probe := &objectProbe{ctx: context.Background(), budget: gitrun.NewBudget(2, time.Minute), root: root, length: 40,
		session: &gitrun.Session{}, objects: map[string]objectInfo{}, reachable: map[string]bool{}, trees: map[string][]string{}}
	defer probe.session.Close()
	if commit, err := probe.commitObject(head); err != nil || !commit {
		t.Fatalf("commit object = %v, %v", commit, err)
	}
	paths, err := probe.trackedPaths(tree)
	if err != nil || !reflect.DeepEqual(paths, []string{".gitignore", "go.mod", "internal/example/value.go"}) {
		t.Fatalf("tracked paths = %q, %v", paths, err)
	}
	if _, err := probe.trackedPaths(git(t, root, "rev-parse", "HEAD:internal")); err == nil || !strings.Contains(err.Error(), "budget") {
		t.Fatalf("third operation past a budget of two: %v", err)
	}
}
