package store

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/journal"
)

// TestJSGV0003_CompleteRouteVerifiesRetainedCheckpoint covers JSG-V0-003:
// the complete mutation route and the scheduled refresh compare the retained
// writer checkpoint with their complete audit. An honest checkpoint stays
// retained; one whose carried archive cost was forged under a valid trailer is
// invalidated, and only the complete audit's own checkpoint may replace it,
// so no writer resumes from the forged values.
func TestJSGV0003_CompleteRouteVerifiesRetainedCheckpoint(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"mutate", "refresh"} {
		for _, forged := range []bool{false, true} {
			name := fmt.Sprintf("%s forged=%v", route, forged)
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				repo := writerStore(t, 70)
				// At the base's own sequence the codec itself refuses a forged
				// aggregate, because the base carries its aggregates (JSG-V0-009);
				// one advanced writer-route write puts the delta past the base,
				// where only the complete audit can tell.
				writerPad(t, repo, journal.WriterAdvanceTail)
				if run := writerMutate(t, repo, "past-base", nil); !run.completed() || !run.fast() {
					t.Fatalf("%s: advance: %+v %v %v", name, run.rep, run.err, run.stages)
				}
				seed := readWriterCheckpoint(repo)
				if seed == nil || seed.Seq.Uint64() <= 70 {
					t.Fatalf("%s: no checkpoint past the base: %+v", name, seed)
				}
				honest, seq := seed.Cost, seed.Seq
				if forged {
					writerCheckpointRewrite(t, repo, func(wc *journal.WriterCheckpoint) { wc.Cost.PayloadBytes++ })
				}
				var stages []string
				switch route {
				case "mutate":
					// Above the floor the writer route declines and the
					// complete route runs.
					writerCheckpointFloors.Store(repo.StateDir, uint64(1)<<40)
					run := writerMutate(t, repo, "complete-route", nil)
					if !run.completed() || run.fast() {
						t.Fatalf("%s: %+v %v %v", name, run.rep, run.err, run.stages)
					}
					stages = run.stages
				case "refresh":
					ctx := context.WithValue(context.Background(), mutationStageKey{}, func(s string) { stages = append(stages, s) })
					refreshWriterCheckpoint(ctx, repo)
				}
				differs := false
				for _, s := range stages {
					differs = differs || strings.HasPrefix(s, "verify.differs")
				}
				kept := readWriterCheckpoint(repo)
				// A checkpoint at the seed's sequence must carry its values; a
				// later one is the complete audit's own.
				survives := kept != nil && kept.Seq == seq && kept.Cost != honest
				if forged && (!differs || survives) {
					t.Fatalf("%s: forged checkpoint kept (differs=%v, %+v): %v", name, differs, kept, stages)
				}
				if !forged && (differs || kept == nil || survives) {
					t.Fatalf("%s: honest checkpoint invalidated (differs=%v, %+v): %v", name, differs, kept, stages)
				}
				if route == "refresh" && kept == nil {
					t.Fatalf("%s: refresh retained no checkpoint of its own", name)
				}
			})
		}
	}
}
