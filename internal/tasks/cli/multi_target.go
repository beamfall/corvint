package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Multi-target writes (TCD-V0-172..173, V1-2398): `ticket note set` and
// `ticket hold` apply one text, or one holdId and reason, to several tickets.
// Each target is its own mutation through the ordinary writer, so one
// refusal never blocks the others; the single-target forms are unchanged.
const (
	profileMultiTarget = "taskman-multi-target/0"
	maxMultiTargets    = 64
	actorClaimWarning  = "the actor binding is a recorded local-operator claim"
)

const multiTargetHoldHelp = "--target may repeat (TCD-V0-173): each target gets its own HOLD with the same payload, request id <ID>-<local> (a digest suffix when over 64 bytes) and one shared issuedAt, and the taskman-multi-target/0 item reports each target's outcome; one refusal never blocks the others. Without --expected-revision hold reads each target's current revision itself, lock-free, just before its write (a single target too); with several targets --expected-revision is refused. Rerun with the same --request-id and the reported issuedAt to replay."

// multiTarget is one resolved target of a multi-target write.
type multiTarget struct {
	arg, id, local, revision string
	err                      error
}

// resolveTargets resolves each argument lock-free and reads its current
// revision; an unresolvable target keeps its error and stops nothing.
func resolveTargets(env Env, args []string) ([]multiTarget, error) {
	var out []multiTarget
	_, err := withInventoryStore(env, func(rc *readCtx) error {
		out = nil
		seen := map[string]bool{}
		for _, arg := range args {
			t := multiTarget{arg: arg}
			id, e := resolveTicketArg(rc, arg)
			if e == nil {
				rec, ok := rc.store.Inventory.Get(id)
				if !ok {
					e = wire.Errorf(wire.CodeMalformed, "argv", "ticket %s is not in the queue", prose(arg))
				} else {
					t.id, t.local, t.revision = id, rec.TicketID.Local, string(rec.Revision)
				}
			}
			t.err = e
			if t.id != "" && seen[t.id] {
				continue
			}
			seen[t.id] = t.id != ""
			out = append(out, t)
		}
		return nil
	})
	return out, err
}

// readTargetRevision is the current revision of one hold target, read
// lock-free when --expected-revision is absent (TCD-V0-173).
func readTargetRevision(env Env, arg string) (string, error) {
	ts, err := resolveTargets(env, []string{arg})
	if err != nil {
		return "", err
	}
	return ts[0].revision, ts[0].err
}

// landedRevision keeps a retried hold on the revision its original envelope
// named (TCD-V0-967): when requestID already completed, every ticket
// mutation sets its resulting revision to the expected one plus one, so the
// original expected revision is that minus one. A request not yet landed, or
// one whose lookup fails, keeps current; the writer's mutation digest still
// refuses a retry whose other inputs changed (REQUEST_ID_CONFLICT).
func landedRevision(env Env, requestID, current string) string {
	expected := current
	_, _ = withInventoryStore(env, func(rc *readCtx) error {
		idx := journal.RequestIndex{Reader: journal.Reader{Source: journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.PrimaryWorktree}, QueueID: rc.snap.Head.QueueID, PrimaryWorktree: rc.repo.PrimaryWorktree}}
		entry, found, err := idx.Lookup(requestID)
		if err != nil || !found || entry.Outcome.Outcome != mutation.OutcomeCompleted || entry.Outcome.ResultingRevision == nil {
			return nil
		}
		if rev := entry.Outcome.ResultingRevision.Int(); rev > 1 {
			expected = string(wire.CountOf(rev - 1))
		}
		return nil
	})
	return expected
}

// targetRequestID derives one target's request id from the caller's.
func targetRequestID(base, ticketID, local string) string {
	id := base + "-" + local
	if len(id) <= wire.MaxRequestIDBytes {
		return id
	}
	sum := sha256.Sum256([]byte(base + "\x00" + ticketID))
	return truncateUTF8(base, wire.MaxRequestIDBytes-25) + "-" + hex.EncodeToString(sum[:12])
}

// multiTargetSubmit runs operation once per target with the same payload.
// readRevision sets each envelope's expectedRevision to the revision read
// for that target; otherwise base.expected (empty for an unconditional note).
func multiTargetSubmit(env Env, cmd []string, operation string, actor mutation.Binding, base mutateFlags, args []string, payload wire.Value, readRevision bool) *wire.Result {
	if len(args) > maxMultiTargets {
		return usage(cmd, "at most 64 targets per call")
	}
	issued := base.issuedAt
	if issued == "" {
		issued = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	}
	if _, err := wire.ParseTimestamp("issuedAt", issued); err != nil {
		return errorResult(cmd, err)
	}
	targets, err := resolveTargets(env, args)
	if err != nil {
		return errorResult(cmd, err)
	}
	var rows []wire.Value
	var codes, human []string
	ok := 0
	for _, t := range targets {
		o := wire.NewObject()
		o.Set("target", wire.String(t.arg))
		o.Set("ticketId", orNull(t.id))
		outcome, replayed := wire.OutcomeRefused, false
		var tcodes, warnings []string
		receipt, resulting, requestID, expected := "", "", "", ""
		if t.err != nil {
			tcodes, warnings = []string{wire.CodeOf(t.err)}, []string{prose(t.err.Error())}
		} else {
			f := base
			f.target, f.issuedAt = t.id, issued
			f.requestID = targetRequestID(base.requestID, t.id, t.local)
			if readRevision {
				f.expected = landedRevision(env, f.requestID, t.revision)
			}
			requestID, expected = f.requestID, f.expected
			res := submitMutation(env, cmd, operation, actor, f, payload)
			outcome, tcodes = res.Outcome, res.Codes
			for _, w := range res.Warnings {
				if !strings.HasPrefix(w, actorClaimWarning) {
					warnings = append(warnings, w)
				}
			}
			if len(res.Items) == 1 && res.Items[0].Obj != nil {
				item := res.Items[0]
				if v, has := item.Obj.Get("replayed"); has {
					replayed = v.Bool
				}
				if v, has := item.Obj.Get("receipt"); has {
					receipt = v.Str
				}
				if v, has := item.Obj.Get("resultingRevision"); has {
					resulting = v.Str
				}
			}
		}
		if outcome == wire.OutcomeOK {
			ok++
		}
		for _, c := range tcodes {
			if !slices.Contains(codes, c) {
				codes = append(codes, c)
			}
		}
		o.Set("requestId", orNull(requestID))
		o.Set("expectedRevision", orNull(expected))
		o.Set("outcome", wire.String(outcome))
		o.Set("codes", wire.Strings(nonNilStrings(tcodes)))
		o.Set("replayed", wire.Bool(replayed))
		o.Set("receipt", orNull(receipt))
		o.Set("resultingRevision", orNull(resulting))
		o.Set("warnings", wire.Strings(nonNilStrings(warnings)))
		rows = append(rows, wire.ObjectValue(o))
		line := t.arg + " " + outcome
		if len(tcodes) > 0 {
			line += " " + strings.Join(tcodes, ",")
		}
		human = append(human, line)
	}
	item := wire.NewObject()
	item.Set("profile", wire.String(profileMultiTarget))
	item.Set("operation", wire.String(operation))
	item.Set("issuedAt", wire.String(issued))
	item.Set("targets", wire.Array(rows...))
	item.Set("human", wire.Strings(human))
	slices.Sort(codes)
	res := &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(item)}}
	if ok != len(targets) {
		res.Outcome = wire.OutcomeRefused
		res.Codes = codes
		if len(res.Codes) == 0 {
			res.Codes = []string{wire.CodeMalformed}
		}
		res.Warnings = append(res.Warnings, "not every target applied; each target row names its outcome and codes; rerun with the same --request-id and --issued-at "+issued+" to replay the applied ones")
	}
	res.Warnings = append(res.Warnings,
		"the actor binding is a recorded local-operator claim, not an authentication (decision 0003); a real queue still needs the §7.4 cutover record")
	return res
}

func nonNilStrings(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}
