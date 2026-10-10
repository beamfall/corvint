package store_test

import (
	"bytes"
	"context"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCALV0062_OpenWorkflowRefusesBeforeMutation proves a disallowed effort or
// wall time is refused before any program record, worktree or host launch,
// without changing the writer checkpoint (including when none is present),
// and that a policy allowance lets the same config reach the runtime pin.
func TestCALV0062_OpenWorkflowRefusesBeforeMutation(t *testing.T) {
	t.Parallel()
	for _, present := range []bool{false, true} {
		name := "checkpoint-absent"
		if present {
			name = "checkpoint-present"
		}
		t.Run(name, func(t *testing.T) {
			s := newLeaseStore(t)
			checkpoint := journal.CheckpointPath(s.repo.StateDir)
			type checkpointState struct {
				info os.FileInfo
				raw  []byte
			}
			readCheckpoint := func() checkpointState {
				t.Helper()
				info, err := os.Stat(checkpoint)
				if os.IsNotExist(err) {
					return checkpointState{}
				}
				if err != nil {
					t.Fatal(err)
				}
				raw, err := os.ReadFile(checkpoint)
				if err != nil {
					t.Fatal(err)
				}
				return checkpointState{info, raw}
			}
			prepareCheckpoint := func(ticketName string) {
				t.Helper()
				// A legitimate ticket writer retains the derived checkpoint.
				// Refresh after the policy update rather than comparing across
				// allowed writes; only the absent fixture removes this file.
				s.ticket(t, ticketName)
				state := readCheckpoint()
				if state.info == nil {
					t.Fatal("fixture writer did not retain a checkpoint")
				}
				if _, err := journal.DecodeCheckpoint(state.raw); err != nil {
					t.Fatalf("fixture writer checkpoint: %v", err)
				}
				if !present {
					if err := os.Remove(checkpoint); err != nil {
						t.Fatal(err)
					}
				}
			}
			prepareCheckpoint("checkpoint-before-policy")
			work := t.TempDir()
			c := store.ProgramConfig{Profile: snapshot.SupervisedProfile, Executable: filepath.Join(work, "absent-codex"), Model: "pinned-model", Effort: "low", StageEfforts: map[string]string{"implement": "high"}, WorkRoot: work, WallSeconds: 2 * 3600}
			programs := filepath.Join(s.repo.StateDir, "programs.json")
			before, _ := os.ReadFile(programs)
			open := func() error {
				t.Helper()
				checkpointBefore := readCheckpoint()
				_, e := store.OpenWorkflow(context.Background(), s.repo, operator(), "program", "/unused-self", c, "")
				checkpointAfter := readCheckpoint()
				if (checkpointBefore.info == nil) != (checkpointAfter.info == nil) {
					t.Fatal("refused config changed checkpoint presence")
				}
				if checkpointBefore.info != nil {
					if !os.SameFile(checkpointBefore.info, checkpointAfter.info) {
						t.Fatal("refused config replaced the checkpoint")
					}
					if !checkpointBefore.info.ModTime().Equal(checkpointAfter.info.ModTime()) {
						t.Fatal("refused config changed checkpoint mtime")
					}
					if !bytes.Equal(checkpointBefore.raw, checkpointAfter.raw) {
						t.Fatal("refused config changed checkpoint bytes")
					}
				}
				after, _ := os.ReadFile(programs)
				if string(after) != string(before) {
					t.Fatal("refused config changed the program inventory")
				}
				if entries, _ := os.ReadDir(work); len(entries) != 0 {
					t.Fatalf("refused config created work under %s", work)
				}
				return e
			}
			if e := open(); e == nil || !strings.Contains(e.Error(), "wallSeconds outside 1..3600") {
				t.Fatalf("default policy accepted a two-hour stage: %v", e)
			}
			c.WallSeconds = 3600
			if e := open(); e == nil || !strings.Contains(e.Error(), `effort "high" for stage implement`) {
				t.Fatalf("default policy accepted high effort: %v", e)
			}

			v := fixture.PolicyValue()
			v.Obj.Set("policyVersion", str("3"))
			v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
			budgets, _ := v.Obj.Get("budgets")
			budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
			v.Obj.Set("supervision", obj("profile", str(snapshot.SupervisedProfile), "contextRequired", wire.Bool(true), "maxRepairCycles", str("1"),
				"program", obj("turns", str("8"), "wallClockMinutes", str("600"), "inputTokens", str("0"), "outputTokens", str("0")),
				"efforts", obj("implement", wire.Strings([]string{"high", "low"})), "stageWallMinutes", str("180")))
			report, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("policy-effort", "2", wire.EncodeFile(v)), now(t))
			if e != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("policy %+v %v", report, e)
			}
			prepareCheckpoint("checkpoint-after-policy")
			c.WallSeconds = 2 * 3600
			if e := open(); e == nil || strings.Contains(e.Error(), "policy") || strings.Contains(e.Error(), "wallSeconds") {
				t.Fatalf("allowed config must pass the policy check and stop at the runtime pin: %v", e)
			}
			c.StageEfforts["review"] = "medium"
			if e := open(); e == nil || !strings.Contains(e.Error(), `effort "medium" for stage review`) {
				t.Fatalf("review outside its allowlist: %v", e)
			}
		})
	}
}
