package localcompletion

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// LCP-V0-019: reuse immutable facts without changing policy results.
func TestEvaluateAgainstMatchesFullEvaluation(t *testing.T) {
	ctx := context.Background()
	root, key, plan := fixture(t, []string{"true"})
	beginFixture(t, root, key, plan)
	for _, phase := range []string{"unverified", "verified", "unrelated-history", "cancelled"} {
		t.Run(phase, func(t *testing.T) {
			switch phase {
			case "verified":
				if _, err := Verify(ctx, root, key, "test"); err != nil {
					t.Fatal(err)
				}
			case "unrelated-history":
				tree := localGit(t, root, "rev-parse", "HEAD^{tree}")
				target := localGit(t, root, "commit-tree", tree, "-m", "unrelated")
				localGit(t, root, "update-ref", "HEAD", target)
			case "cancelled":
				if _, err := Cancel(ctx, root, key, "test cancel"); err != nil {
					t.Fatal(err)
				}
			}
			want, observed, err := EvaluateObserved(ctx, root, key)
			if err != nil {
				t.Fatal(err)
			}
			if phase == "unrelated-history" && !hasUnmet(want, "base-not-ancestor-of-target") {
				t.Fatal("fixture did not reach the non-ancestor path")
			}
			got, err := EvaluateAgainst(ctx, root, key, observed)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("pinned = %#v, %v; want %#v", got, err, want)
			}
			if (Observed{}).Matches(want.Target, observed.tree) || observed.Matches("wrong", observed.tree) || observed.Matches(observed.target, "wrong") {
				t.Fatal("invalid identity matched")
			}
		})
	}
}

// LCP-V0-019: artifacts and private state are fresh inputs even though Git's
// immutable content is reused. Prove qualification before invalidating its log.
func TestEvaluateAgainstReReadsPrivateInputs(t *testing.T) {
	ctx := context.Background()
	root, key, plan := fixture(t, []string{"true"})
	repo := beginFixture(t, root, key, plan)
	if _, err := Verify(ctx, root, key, "test"); err != nil {
		t.Fatal(err)
	}
	before, observed, err := EvaluateObserved(ctx, root, key)
	if err != nil || len(before.Checks) != 1 || !before.Checks[0].Qualified {
		t.Fatalf("fixture not qualified: %#v, %v", before, err)
	}
	if err := os.WriteFile(before.Checks[0].Stdout, []byte("changed log"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := EvaluateAgainst(ctx, root, key, observed)
	want, fullErr := Evaluate(ctx, root, key)
	if err != nil || fullErr != nil || !reflect.DeepEqual(got, want) || got.Checks[0].Qualified {
		t.Fatalf("artifact reread: %#v, %v; full %#v, %v", got, err, want, fullErr)
	}
	if err := os.WriteFile(filepath.Join(repo.directory, "owner"), []byte(HashSession("other")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = EvaluateAgainst(ctx, root, key, observed)
	want, fullErr = Evaluate(ctx, root, key)
	if err != nil || fullErr != nil || !reflect.DeepEqual(got, want) || !hasUnmet(got, "worktree-owner-mismatch") {
		t.Fatalf("owner reread: %#v, %v; full %#v, %v", got, err, want, fullErr)
	}
}

// LCP-V0-019: status detects both dirt introduced between evaluations and dirt
// introduced during the pinned evaluation. Compare the full path's refusal.
func TestEvaluateAgainstDirtyAndDrift(t *testing.T) {
	ctx := context.Background()
	root, key, plan := fixture(t, []string{"true"})
	beginFixture(t, root, key, plan)
	_, observed, err := EvaluateObserved(ctx, root, key)
	if err != nil {
		t.Fatal(err)
	}
	dirty := filepath.Join(root, "untracked")
	if err := os.WriteFile(dirty, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := EvaluateAgainst(ctx, root, key, observed)
	want, fullErr := Evaluate(ctx, root, key)
	if err != nil || fullErr != nil || !reflect.DeepEqual(got, want) || !hasUnmet(got, "uncommitted-work") {
		t.Fatalf("dirty reread: %#v, %v; full %#v, %v", got, err, want, fullErr)
	}
	if err := os.Remove(dirty); err != nil {
		t.Fatal(err)
	}
	clean, err := EvaluateAgainst(ctx, root, key, observed)
	if err != nil || hasUnmet(clean, "uncommitted-work") {
		t.Fatalf("injection fixture not clean: %#v, %v", clean, err)
	}
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "status-seen")
	// Mutate before the second status call, after the first status proved clean.
	shim := "#!/bin/sh\ncase \" $* \" in\n*' status --porcelain=v1 '*)\nif [ -f '" + marker + "' ]; then printf drift > '" + dirty + "'; else printf seen > '" + marker + "'; fi;;\nesac\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(shim), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, pinned := range []bool{true, false} {
		if pinned {
			_, err = EvaluateAgainst(ctx, root, key, observed)
		} else {
			_, err = Evaluate(ctx, root, key)
		}
		if err == nil || err.Error() != "repository-snapshot-drift" {
			t.Fatalf("pinned=%v: drift error = %v", pinned, err)
		}
		if raw, readErr := os.ReadFile(dirty); readErr != nil || string(raw) != "drift" {
			t.Fatalf("injected status path not reached: %q, %v", raw, readErr)
		}
		for _, name := range []string{marker, dirty} {
			if err := os.Remove(name); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// LCP-V0-019: ancestry is re-proved even when the base is unchanged.
func TestEvaluateAgainstChangedBase(t *testing.T) {
	ctx := context.Background()
	root, key, plan := fixture(t, []string{"true"})
	localGit(t, root, "commit", "--allow-empty", "-qm", "target")
	beginFixture(t, root, key, plan)
	_, observed, err := EvaluateObserved(ctx, root, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Cancel(ctx, root, key, "test cancel"); err != nil {
		t.Fatal(err)
	}
	plan.Base = localGit(t, root, "rev-parse", "HEAD")
	beginFixture(t, root, key, plan)
	want, err := Evaluate(ctx, root, key)
	if err != nil {
		t.Fatal(err)
	}
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	shim := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + log + "'\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(shim), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	got, err := EvaluateAgainst(ctx, root, key, observed)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("changed base: %#v, %v; want %#v", got, err, want)
	}
	raw, err := os.ReadFile(log)
	if err != nil || strings.Count(string(raw), "\n") != 4 || !strings.Contains(string(raw), "merge-base --is-ancestor "+plan.Base+" "+observed.target) {
		t.Fatalf("missing fresh ancestry proof: %s, %v", raw, err)
	}
}

// LCP-V0-019: a clean HEAD move to a same-tree commit during the pinned read is drift.
func TestEvaluateAgainstDetectsSameTreeHeadMove(t *testing.T) {
	ctx := context.Background()
	root, key, plan := fixture(t, []string{"true"})
	beginFixture(t, root, key, plan)
	_, observed, err := EvaluateObserved(ctx, root, key)
	if err != nil {
		t.Fatal(err)
	}
	tree := localGit(t, root, "rev-parse", "HEAD^{tree}")
	other := localGit(t, root, "commit-tree", tree, "-m", "same tree")
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "status-seen")
	shim := "#!/bin/sh\ncase \" $* \" in\n*' status --porcelain=v1 '*)\nif [ -f '" + marker + "' ]; then '" + real + "' -C '" + root + "' update-ref HEAD " + other + "; else printf seen > '" + marker + "'; fi;;\nesac\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(shim), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, err = EvaluateAgainst(ctx, root, key, observed); err == nil || err.Error() != "repository-snapshot-drift" {
		t.Fatalf("same-tree HEAD move: %v", err)
	}
}
