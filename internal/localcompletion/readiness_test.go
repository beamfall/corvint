package localcompletion

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func assertNextAction(t *testing.T, result Evaluation, key, action string, extra ...string) {
	t.Helper()
	want := [][]string{append([]string{"corvint", "dogfood", action, "--session-key", key}, extra...)}
	if !reflect.DeepEqual(result.NextActions, want) {
		t.Fatalf("next actions: got %v, want %v", result.NextActions, want)
	}
}

func TestCompletionNextActions(t *testing.T) {
	t.Run("LCP-V0-003 check readiness and target drift", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"true"})
		beginFixture(t, root, key, plan)
		result, err := Evaluate(context.Background(), root, key)
		if err != nil {
			t.Fatal(err)
		}
		assertNextAction(t, result, key, "verify", "--check", "test")
		result, err = Verify(context.Background(), root, key, "test")
		if err != nil {
			t.Fatal(err)
		}
		assertNextAction(t, result, key, "finish")
		localGit(t, root, "commit", "--allow-empty", "-qm", "new target")
		result, err = Evaluate(context.Background(), root, key)
		if err != nil {
			t.Fatal(err)
		}
		assertNextAction(t, result, key, "verify", "--check", "test")
	})
	t.Run("LCP-V0-003 wrong owner retains original handle", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"true"})
		repo := beginFixture(t, root, key, plan)
		if err := os.WriteFile(filepath.Join(repo.directory, "owner"), []byte(HashSession("other")+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := Evaluate(context.Background(), root, key)
		if err != nil || !hasUnmet(result, "worktree-owner-mismatch") {
			t.Fatalf("owner: %#v %v", result, err)
		}
		assertNextAction(t, result, key, "status")
	})
	t.Run("LCP-V0-003 exhausted observations block only required verification", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"true"})
		repo := beginFixture(t, root, key, plan)
		if _, err := Verify(context.Background(), root, key, "test"); err != nil {
			t.Fatal(err)
		}
		saved, err := repo.load()
		if err != nil {
			t.Fatal(err)
		}
		for len(saved.Observations) < maxAttempts {
			observed := saved.Observations[0]
			for index, log := range []*artifact{&observed.Stdout, &observed.Stderr} {
				raw, err := os.ReadFile(log.Path)
				if err != nil {
					t.Fatal(err)
				}
				log.Path = repo.local(fmt.Sprintf("check-%03d-%d.log", len(saved.Observations)+1, index))
				if err = os.WriteFile(log.Path, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			saved.Observations = append(saved.Observations, observed)
		}
		if err = repo.save(saved); err != nil {
			t.Fatal(err)
		}
		result, err := Evaluate(context.Background(), root, key)
		if err != nil || hasUnmet(result, "verification-attempt-bound-exceeded") {
			t.Fatalf("qualified at bound: %#v %v", result, err)
		}
		assertNextAction(t, result, key, "finish")
		localGit(t, root, "commit", "--allow-empty", "-qm", "stale observations")
		result, err = Evaluate(context.Background(), root, key)
		if err != nil || !hasUnmet(result, "selected-check-unverified:test") || !hasUnmet(result, "verification-attempt-bound-exceeded") {
			t.Fatalf("exhausted: %#v %v", result, err)
		}
		if len(slices.DeleteFunc(slices.Clone(result.Unmet), func(s string) bool { return s != "verification-attempt-bound-exceeded" })) != 1 {
			t.Fatal("attempt bound must appear once")
		}
		assertNextAction(t, result, key, "status")
	})
}
