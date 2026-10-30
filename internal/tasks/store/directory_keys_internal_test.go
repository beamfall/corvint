package store

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0214_CommittedEffectsStayBoundToTransaction verifies both writer
// routes retain the committed effects rather than an earlier CLI inventory
// or a later ticket revision. Replays carry no effects for warning generation.
func TestCALV0214_CommittedEffectsStayBoundToTransaction(t *testing.T) {
	t.Parallel()
	for _, fast := range []bool{false, true} {
		name := "complete-audit"
		if fast {
			name = "writer-checkpoint"
		}
		t.Run(name, func(t *testing.T) {
			repo := historyStoreAt(t, filepath.Join(t.TempDir(), "repo"), 1, 4)
			if fast {
				lowerWriterCheckpoint(t, repo)
				seed := writerMutate(t, repo, "effects-seed", nil)
				if !seed.completed() || seed.fast() {
					t.Fatalf("checkpoint seed: %+v %v %v", seed.rep, seed.err, seed.stages)
				}
			}
			created := writerMutate(t, repo, "effects-create", nil)
			if !created.completed() || created.fast() != fast || created.rep.MutationEffects == nil {
				t.Fatalf("create: %+v %v %v", created.rep, created.err, created.stages)
			}
			stale, err := intent.Load(repo.IntentRoot())
			if err != nil {
				t.Fatal(err)
			}
			run := func(id, op, rev, payload string) writerRun {
				t.Helper()
				v, err := wire.Parse(historyCreate(id, "unused"))
				if err != nil {
					t.Fatal(err)
				}
				p, err := wire.ParseInput([]byte(payload))
				if err != nil {
					t.Fatal(err)
				}
				v.Obj.Set("operation", wire.String(op))
				v.Obj.Set("targetId", wire.String(created.rep.Ticket))
				v.Obj.Set("expectedRevision", wire.String(rev))
				v.Obj.Set("payload", p)
				r := stagedWrite(writerEnvelope(repo, wire.EncodeFile(v), WallClock()), nil)
				if !r.completed() {
					t.Fatalf("%s: %+v %v %v", op, r.rep, r.err, r.stages)
				}
				return r
			}
			run("effects-set", mutation.OpSetEffects, "1", `{"capabilities":[],"effects":{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["src"]},"executionClass":"AUTONOMOUS"}`)
			refined := run("effects-refine", mutation.OpRefine, "2", `{"title":"refined"}`)
			if refined.fast() != fast || refined.rep.MutationEffects == nil || !slices.Equal(refined.rep.MutationEffects.TouchPaths, []string{"src"}) {
				t.Fatalf("refine did not retain committed effects: %+v %v", refined.rep, refined.stages)
			}
			old, _ := stale.Inventory.Get(created.rep.Ticket)
			if slices.Equal(old.Effects.TouchPaths, refined.rep.MutationEffects.TouchPaths) {
				t.Fatal("fixture did not establish a stale pre-write inventory")
			}
			run("effects-later", mutation.OpSetEffects, "3", `{"capabilities":[],"effects":{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["src/"]},"executionClass":"AUTONOMOUS"}`)
			if !slices.Equal(refined.rep.MutationEffects.TouchPaths, []string{"src"}) {
				t.Fatal("later write changed the retained effects")
			}
			replay := run("effects-refine", mutation.OpRefine, "2", `{"title":"refined"}`)
			if replay.rep.Kind != "Replay" || replay.rep.MutationEffects != nil {
				t.Fatalf("replay retained effects: %+v", replay.rep)
			}
		})
	}
}
