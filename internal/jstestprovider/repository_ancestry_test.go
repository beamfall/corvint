package jstestprovider

import (
	"context"
	"os/exec"
	"testing"

	"github.com/Beamfall/corvint/internal/gitancestry/ancestrytest"
)

// EEP-V0-771: the test repository identity ignores info/grafts, so a graft cannot rename its root
// commit; a shallow boundary is named as the root, a real commit of this history.
func TestObserveTestRepositoryIgnoresGrafts_EEPV0771(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	repository := initAttestationGitRepository(t, git, "basic")
	plain := ObserveTestRepositoryAtStart(context.Background(), repository)
	if plain == nil {
		t.Fatal("plain checkout has no identity")
	}
	gitTestOutput(t, git, repository, "-c", "user.name=F", "-c", "user.email=f@example.invalid", "commit", "-q", "--allow-empty", "-m", "child")
	head := gitTestOutput(t, git, repository, "rev-parse", "HEAD")
	undo := ancestrytest.Graft(t, repository, head)
	if grafted := ObserveTestRepositoryAtStart(context.Background(), repository); grafted == nil || grafted.RootCommit != plain.RootCommit {
		t.Errorf("grafted identity = %+v, want root %s", grafted, plain.RootCommit)
	}
	undo()
	ancestrytest.Shallow(t, repository, head)
	if shallow := ObserveTestRepositoryAtStart(context.Background(), repository); shallow == nil || shallow.RootCommit != head {
		t.Errorf("shallow identity = %+v, want boundary root %s", shallow, head)
	}
}
