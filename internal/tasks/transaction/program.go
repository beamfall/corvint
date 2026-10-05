package transaction

import (
	"bytes"
	"encoding/json"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"sort"
	"time"
)

type ProgramChange struct {
	PreviousWorkerClean bool             `json:"previousWorkerClean"`
	PreviousOwnerGone   bool             `json:"previousOwnerGone"`
	Expected            wire.Digest      `json:"expected"`
	Next                snapshot.Program `json:"next"`
	Output              []byte           `json:"output"`
}

func planProgram(c leaseContext) leaseOutcome {
	raw := c.in.LeaseFacts.Program
	if len(raw) > 65536 || string(wire.Sum(raw)) != c.l.Evidence {
		return c.fail(malformed("program change binding"))
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var change ProgramChange
	if e := d.Decode(&change); e != nil {
		return c.fail(e)
	}
	state := snapshot.Programs{Profile: "taskman-programs/0", QueueID: c.r.QueueID, Entries: []snapshot.Program{}}
	if len(c.in.Programs) > 0 {
		if !c.in.Inventory.matches("programs.json", c.in.Programs) {
			return c.fail(malformed("program inventory binding"))
		}
		old, e := snapshot.DecodePrograms(c.in.Programs)
		if e != nil {
			return c.fail(e)
		}
		state = *old
	}
	if change.Expected != wire.Sum(c.in.Programs) {
		return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "program inventory moved")
	}
	next := change.Next
	at := -1
	for i, p := range state.Entries {
		if p.ID == next.ID {
			at = i
		}
	}
	enabled := false
	for _, r := range c.st.policy.Runtimes {
		if r.RuntimeID == snapshot.SupervisedProfile && r.Enabled {
			enabled = true
		}
	}
	if !enabled {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeUnsupported, "supervised profile is not enabled in policy")
	}
	if at < 0 {
		if next.Phase != "ADMITTED" || next.Epoch != 1 || next.LeaderPID != 0 || !freshRepositories(next.Repositories) {
			return c.fail(malformed("new program admission"))
		}
		state.Entries = append(state.Entries, next)
	} else {
		old := state.Entries[at]
		reassign := old.Phase == "FINISHED" && next.Phase == "ADMITTED" && next.Assignment == old.Assignment+1 && next.CurrentAttempt == "" && next.CurrentGeneration == ""
		if reassign {
			a := c.st.attempts[old.CurrentAttempt]
			if a == nil || a.Live() || a.Quiescence != "PROVED" {
				return c.recordProgramRefusal("prior assignment not terminal and proved")
			}
		}
		if !reassign && next.Assignment != old.Assignment {
			return c.recordProgramRefusal("assignment moved")
		}
		binding := old.Phase == "ADMITTED" && next.Phase == "ADMITTED" && old.CurrentAttempt == "" && next.CurrentAttempt != ""
		if binding {
			a := c.st.attempts[next.CurrentAttempt]
			if a == nil || string(a.Generation) != next.CurrentGeneration || a.Lease == nil || a.Lease.Holder != next.ID+"-implement" || a.BaseCommit != next.Base {
				return c.recordProgramRefusal("assignment claim binding")
			}
		}
		if !binding && !reassign && (next.CurrentAttempt != old.CurrentAttempt || next.CurrentGeneration != old.CurrentGeneration) {
			return c.recordProgramRefusal("assignment tuple changed")
		}
		if next.OwnerReleased {
			a := c.st.attempts[next.CurrentAttempt]
			if a != nil && (a.Quiescence != "PROVED" || a.Supervision != nil && a.Supervision.Worker) {
				return c.recordProgramRefusal("owner release before quiescence")
			}
		}
		recovering := old.OwnerPID != next.OwnerPID || old.OwnerStarted != next.OwnerStarted
		if old.ConfigSHA256 != next.ConfigSHA256 || (!reassign && old.Base != next.Base) || old.Group != next.Group || old.StartedAt != next.StartedAt {
			return c.recordProgramRefusal("immutable program binding differs")
		}
		if !sameRepositories(old.Repositories, next.Repositories, reassign) {
			return c.recordProgramRefusal("immutable repository binding differs")
		}
		if recovering {
			safePhase := (old.Phase == "FINISHED" || old.Phase == "ADMITTED" || old.Phase == "WORKTREE_ADD" || old.Phase == "READY") && next.Phase == old.Phase
			recovered := change.PreviousWorkerClean && next.Phase == "FINISHED" && next.Quiescence == "PROVED"
			if !change.PreviousOwnerGone || next.Epoch != old.Epoch+1 || (!safePhase && !recovered) || old.Worktree != next.Worktree {
				return c.recordProgramRefusal("prior program owner not proved stopped")
			}
		} else if old.Epoch != next.Epoch {
			return c.recordProgramRefusal("program epoch differs")
		}
		if old.Worktree != next.Worktree && !reassign && !((old.Phase == "FINISHED" || old.Phase == "ADMITTED") && (next.Phase == "WORKTREE_ADD" || next.Phase == "WORKTREE_REMOVE")) {
			return c.recordProgramRefusal("worktree changed outside add effect")
		}
		transitions := map[string][]string{"ADMITTED": {"WORKTREE_ADD"}, "WORKTREE_ADD": {"READY", "BLOCKED_RECOVERY"}, "READY": {"SPAWNING"}, "SPAWNING": {"RUNNING", "STOPPING", "BLOCKED_RECOVERY"}, "RUNNING": {"STOPPING"}, "STOPPING": {"FINISHED", "BLOCKED_RECOVERY"}, "FINISHED": {"WORKTREE_ADD", "WORKTREE_REMOVE", "FINISHED", "READY"}, "WORKTREE_REMOVE": {"FINISHED", "BLOCKED_RECOVERY"}}
		controlOnly := old.Phase == next.Phase && old.Control != next.Control
		if controlOnly {
			copy := next
			copy.Control = old.Control
			a, _ := snapshot.EncodeProgramJSON(old)
			b, _ := snapshot.EncodeProgramJSON(copy)
			if !bytes.Equal(a, b) {
				return c.fail(malformed("control changes other program fields"))
			}
			if next.Control != "DRAIN" && next.Control != "CANCEL" && next.Control != "" {
				return c.fail(malformed("program control"))
			}
		}
		ok := recovering || controlOnly || binding || reassign
		for _, phase := range transitions[old.Phase] {
			ok = ok || phase == next.Phase
		}
		if !ok {
			return c.fail(malformed("program transition " + old.Phase + " -> " + next.Phase))
		}
		if next.Phase == "RUNNING" && (next.LeaderPID <= 0 || next.LeaderStarted == "" || next.Effect != old.Effect) {
			return c.fail(malformed("running lane lacks bound identity"))
		}
		if next.Phase == "FINISHED" && next.Quiescence != "PROVED" {
			return c.fail(malformed("unproved lane cleanup"))
		}
		if old.Phase == "STOPPING" && (next.Phase == "FINISHED" || next.Phase == "BLOCKED_RECOVERY") && next.ResultClass != "NO_EXEC" {
			i, o, known := policyHostUsage(c.st.policy, change.Output)
			if next.InputTokens != old.InputTokens+i || next.OutputTokens != old.OutputTokens+o || next.UsageKnown != (old.UsageKnown && known) {
				return c.fail(malformed("usage must derive from retained host output"))
			}
		} else if next.InputTokens != old.InputTokens || next.OutputTokens != old.OutputTokens || next.UsageKnown != old.UsageKnown {
			return c.fail(malformed("usage changed outside retained outcome"))
		}
		if next.Phase == "SPAWNING" && old.Phase != "SPAWNING" {
			if next.Turns != old.Turns+1 {
				return c.fail(malformed("turn reservation"))
			}
			cap := c.st.policy.Supervision
			if cap == nil {
				return c.refuse(mutation.OutcomeBlocked, wire.CodeUnsupported, "supervision policy absent")
			}
			turns, input, output := next.Turns, next.InputTokens, next.OutputTokens
			earliest, e := time.Parse(time.RFC3339, next.StartedAt)
			if e != nil {
				return c.fail(e)
			}
			known := next.UsageKnown
			for _, p := range state.Entries {
				if p.ID != next.ID && p.Group == next.Group {
					if p.ConfigSHA256 != next.ConfigSHA256 {
						return c.fail(malformed("group config differs"))
					}
					turns += p.Turns
					input += p.InputTokens
					output += p.OutputTokens
					known = known && p.UsageKnown
					if t, e := time.Parse(time.RFC3339, p.StartedAt); e == nil && t.Before(earliest) {
						earliest = t
					}
				}
			}
			if turns > uint64(cap.Turns.Int()) || func() time.Duration {
				now, _ := time.Parse(time.RFC3339, string(c.in.RecordedAt))
				return now.Sub(earliest)
			}() >= time.Duration(cap.WallClockMinutes.Int())*time.Minute {
				return c.refuse(mutation.OutcomeBlocked, wire.CodeLimitExceeded, "shared program turn/wall cap")
			}
			if (cap.InputTokens.Uint64() > 0 || cap.OutputTokens.Uint64() > 0) && !known {
				return c.refuse(mutation.OutcomeBlocked, wire.CodeMissingEvidence, "program token usage not observed")
			}
			if cap.InputTokens.Uint64() > 0 && input >= cap.InputTokens.Uint64() || cap.OutputTokens.Uint64() > 0 && output >= cap.OutputTokens.Uint64() {
				return c.refuse(mutation.OutcomeBlocked, wire.CodeLimitExceeded, "observed program token cutoff")
			}
		} else if next.Turns != old.Turns {
			return c.fail(malformed("turn count outside spawn reservation"))
		}
		state.Entries[at] = next
	}
	sort.Slice(state.Entries, func(i, j int) bool { return state.Entries[i].ID < state.Entries[j].ID })
	out, e := state.Encode()
	if e != nil {
		return c.fail(e)
	}
	posts := map[string][]byte{"programs.json": out, "evidence/" + string(wire.Sum(raw)): raw}
	if len(change.Output) > 0 {
		if next.ResultSHA256 != string(wire.Sum(change.Output)) {
			return c.fail(malformed("program result binding"))
		}
		posts["evidence/"+next.ResultSHA256] = change.Output
	}
	return leaseOutcome{posts: posts, effect: &leaseEffect{kind: "TRANSITION", outcome: mutation.OutcomeCompleted, codes: []string{}}, detail: next.Phase}
}

func (c leaseContext) recordProgramRefusal(reason string) leaseOutcome {
	return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, reason)
}

// freshRepositories reports whether a newly admitted program's extra
// repositories carry no candidate yet (CAL-V0-071).
func freshRepositories(repos []snapshot.RepositoryRecord) bool {
	for _, r := range repos {
		if r.Candidate != "" {
			return false
		}
	}
	return true
}

// sameRepositories keeps a program's extra repository set, checkouts and Git
// identities immutable; a base moves only on reassignment, which also clears
// the candidate (CAL-V0-071, CAL-V0-072).
func sameRepositories(old, next []snapshot.RepositoryRecord, reassign bool) bool {
	if len(old) != len(next) {
		return false
	}
	for i := range old {
		o, n := old[i], next[i]
		if o.Name != n.Name || o.Checkout != n.Checkout || o.CommonIdentity != n.CommonIdentity {
			return false
		}
		if reassign && n.Candidate != "" || !reassign && o.Base != n.Base {
			return false
		}
	}
	return true
}
