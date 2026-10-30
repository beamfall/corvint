// SPDX-License-Identifier: Apache-2.0
package localcompletion

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/dogfoodflow"
)

type sealedCancellationProbe struct {
	context.Context
	observed bool
}

func (p *sealedCancellationProbe) Err() error { p.observed = true; return p.Context.Err() }

// LCP-V0-040: report validation must carry the caller's cancellation into
// the seal resolver, rather than launching it with an unbounded background context.
func TestLocalCompletionSealedReportCancellation(t *testing.T) {
	root, key, plan := fixture(t, []string{"true"})
	repo := beginFixture(t, root, key, plan)
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("package fixture\nconst Answer = 2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	localGit(t, root, "add", "source.go")
	localGit(t, root, "commit", "-qm", "change")
	var out, errout strings.Builder
	if code := cli.Run(context.Background(), root, []string{"prepare", "--base", plan.Base, "--target", "HEAD"}, &out, &errout); code != 0 {
		t.Fatalf("canonical fixture initialization: %d %s %s", code, out.String(), errout.String())
	}
	localGit(t, root, "add", "-f", ".corvint/change.cem.json")
	localGit(t, root, "commit", "-qm", "bind")
	bind := localGit(t, root, "rev-parse", "HEAD")
	archive := ".corvint/changes/" + bind + ".cem.json"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, archive)), 0755); err != nil {
		t.Fatal(err)
	}
	localGit(t, root, "mv", ".corvint/change.cem.json", archive)
	localGit(t, root, "commit", "-qm", "seal")
	snap, err := repo.snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	binding, err := dogfoodflow.ResolveSealBinding(context.Background(), root, snap.target)
	if err != nil || binding.EvidenceTarget != bind {
		t.Fatalf("positive seal resolver: %#v %v", binding, err)
	}
	saved, err := repo.load()
	if err != nil {
		t.Fatal(err)
	}
	saved.Sealed = &sealedContinuation{Binding: binding}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	probe := &sealedCancellationProbe{Context: cancelled}
	if repo.reportCurrent(probe, saved, snap) {
		t.Fatal("cancelled report became current")
	}
	if !probe.observed {
		t.Fatal("caller cancellation never reached sealed report validation")
	}
}
