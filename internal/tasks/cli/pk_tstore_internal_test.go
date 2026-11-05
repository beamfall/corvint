// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"context"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TCD-V0-7212: phases remain distinct when the request ID needs a digest suffix.
func TestTCDV07212_LongTargetRequestIncludesLocal(t *testing.T) {
	base := strings.Repeat("x", 64)
	a, b := targetRequestID(base, "ticket:a:q:t1", "create"), targetRequestID(base, "ticket:a:q:t1", "archive")
	if a == b || len(a) > wire.MaxRequestIDBytes || len(b) > wire.MaxRequestIDBytes {
		t.Fatalf("phase collision: %s %s", a, b)
	}
}

// TRM-V0-7208: release update and membership inherit the shared submission context.
func TestTRMV07208_ReleaseWritesUseSubmissionContext(t *testing.T) {
	for _, verb := range []string{"update", "membership"} {
		t.Run(verb, func(t *testing.T) {
			r := fixture.TempRepo(t)
			fixture.WriteWritableState(t, r)
			a := fixture.Ticket("A")
			b := fixture.Ticket("B")
			fixture.WriteIntent(t, r, a, b)
			fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/tickets/A.json": a.Encode(), "intent/tickets/B.json": b.Encode()})
			env := Env{Cwd: r.Root, Actor: "tester"}
			payload := `{"acceptanceCriteria":["release criterion"],"predecessorReleaseIds":[],"requiredGates":[],"ticketIds":["ticket:demo:main:A"],"title":"Release","version":"v1"}`
			// Use the canonical queue identity rather than a fixture alias.
			payload = strings.ReplaceAll(payload, "ticket:demo:main:A", a.TicketID.Raw)
			create := releaseCommand(env, "create", []string{"--request-id", "release-create", "--target", "v1", "--payload", payload})
			if create.Outcome != wire.OutcomeOK {
				t.Fatalf("create: %s", wire.EncodeFile(create.Value()))
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			env.batchWriteContext = ctx
			args := []string{"--request-id", "release-" + verb, "--target", "v1", "--expected-revision", "1"}
			if verb == "update" {
				args = append(args, "--payload", payload)
			} else {
				args = append(args, "--add", "B")
			}
			got := releaseCommand(env, verb, args)
			if got.Outcome == wire.OutcomeOK {
				t.Fatalf("%s ignored submission cancellation", verb)
			}
		})
	}
}

// TCD-V0-7214: a failed ancestry query never proves NOT_ANCESTOR.
func TestTCDV07214_CheckpointAncestryAbstainsOnGitError(t *testing.T) {
	r := fixture.TempRepo(t)
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=t", "-c", "user.email=t@example.test", "commit", "--allow-empty", "-qm", "base"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = r.Root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %s %v", out, err)
		}
	}
	p := reviewPacket{Commit: strings.Repeat("f", 40)}
	p.bindCheckpoint(r.Root, "HEAD")
	if p.CheckpointCommit != "" || p.CheckpointRelation == CheckpointNotAncestor {
		t.Fatalf("failed query became fact: %+v", p)
	}
}

// TCD-V0-7214: successful revision resolution followed by a failed Git ancestry
// query must abstain in the gated seal, delivery screen and debt folding.
func TestTCDV07214_AncestryCallersRefuseObservationFailure(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteWritableState(t, r)
	a := fixture.Ticket("A")
	fixture.WriteIntent(t, r, a)
	fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/tickets/A.json": a.Encode()})
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(real, append([]string{"-c", "user.name=t", "-c", "user.email=t@example.test"}, args...)...)
		cmd.Dir = r.Root
		raw, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s %v", args, raw, err)
		}
		return strings.TrimSpace(string(raw))
	}
	runGit("init", "-q")
	runGit("add", ".taskman")
	runGit("commit", "-qm", "base")
	base := runGit("rev-parse", "HEAD")
	tree := runGit("rev-parse", "HEAD^{tree}")
	side := runGit("commit-tree", tree, "-p", base, "-m", "side")
	inner := runGit("commit-tree", tree, "-p", base, "-p", side, "-m", "Refs A")
	outer := runGit("commit-tree", tree, "-p", base, "-p", inner, "-m", "Refs A")
	runGit("update-ref", "refs/heads/main", outer)
	detached := filepath.Join(t.TempDir(), "detached")
	runGit("worktree", "add", "--detach", detached, side)
	wrapper := t.TempDir()
	fixture.Write(t, filepath.Join(wrapper, "git"), []byte("#!/bin/sh\ncase \" $* \" in *\" merge-base --is-ancestor \"*) exit 128;; esac\nexec "+strconv.Quote(real)+" \"$@\"\n"))
	if err := os.Chmod(filepath.Join(wrapper, "git"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", wrapper+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Run("seal", func(t *testing.T) {
		b := gatedBatch{root: r.Root, merge: outer, parent: base, seals: map[string]string{"A": inner}}
		row := closeRow{}
		b.plan(&row, a, nil, transaction.PlanInput{}, 1)
		if row.code != wire.CodeUnsupported || !strings.Contains(row.detail, "NOT_OBSERVED") {
			t.Fatalf("seal query: %+v", row)
		}
	})
	t.Run("delivery", func(t *testing.T) {
		if got := laneAncestry(r.Root, outer, &ghPR{HeadRefOid: inner}); got != laneHeadNotObserved {
			t.Fatalf("delivery query became %q", got)
		}
	})
	t.Run("detached", func(t *testing.T) {
		if _, err := store.LaneHeads(r.Root, outer); err == nil {
			t.Fatal("detached lane ancestry error was discarded")
		}
	})
	t.Run("debt", func(t *testing.T) {
		got, _ := closeoutDebtCommand(Env{Cwd: r.Root, Actor: "tester"}, []string{"--ref", "HEAD", "--since", base})
		if got.Outcome == wire.OutcomeOK || !slices.Contains(got.Codes, wire.CodeUnsupported) || !strings.Contains(strings.Join(got.Warnings, " "), "merge-base") {
			t.Fatalf("debt ancestry error became delivery facts: %s", wire.EncodeFile(got.Value()))
		}
	})
}
