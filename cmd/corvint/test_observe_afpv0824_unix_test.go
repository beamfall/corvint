//go:build darwin || linux

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// AFP-V0-824: the plan compiled from the HEAD and dirty set `test` read for
// its source identity equals the plan that reads them itself, and an
// observation the worktree has since left is still refused by the closing
// drift recheck.
func TestAFPV0824SharedObservationPlansAsItsOwnRead(t *testing.T) {
	root, base := sliceFixture(t)
	useAffectedUserCacheDir(t, "", errors.New("no cache in this run"))
	if err := os.WriteFile(filepath.Join(root, "b", "b.go"), []byte("package b\n\nimport \"example.test/m/a\"\n\nfunc B() int { return a.A() + 2 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	own, _, err := compileAffectedGraph(ctx, affectedInvocation{Root: root, Base: base})
	if err != nil {
		t.Fatal(err)
	}
	_, observed, err := testObserveSource(ctx, gitExecutable, root)
	if err != nil {
		t.Fatal(err)
	}
	shared, _, err := compileAffectedGraph(ctx, affectedInvocation{Root: root, Base: base, observed: observed})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(own, shared) {
		t.Fatalf("the shared observation planned differently:\n%+v\n%+v", shared, own)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "late.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err = compileAffectedGraph(ctx, affectedInvocation{Root: root, Base: base, observed: observed})
	var refusal *gokernel.Error
	if !errors.As(err, &refusal) || refusal.Code != "unsupported-affected-drift" {
		t.Fatalf("a stale observation was not refused as drift: %v", err)
	}
}
