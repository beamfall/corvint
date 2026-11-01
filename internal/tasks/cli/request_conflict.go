package cli

import (
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// defaultedIssuedAtConflict explains a REQUEST_ID_CONFLICT whose issuedAt the
// CLI defaulted to now (CAL-V0-250, V1-1384). The request digest covers
// issuedAt, so a byte-identical retry in a later second conflicts. The
// journal keeps only the original digest, so this rebuilds the envelope with
// the recordedAt of the receipt the request-index entry names, which equals
// a defaulted issuedAt and survives later updates (ATR-V0-275, V1-1438),
// then the recorded ticket's createdAt and updatedAt (and up to two seconds
// before each, since the writer clock can tick past the CLI second) and names issuedAt as the
// difference only when one rebuilt digest equals the recorded one. It never
// replays or writes: the retry with --issued-at does that. Otherwise it
// names the digest inputs, so the refusal never guesses.
func defaultedIssuedAtConflict(repo *intent.Repository, st *intent.Store, operation string, actor mutation.Binding, flags mutateFlags, payload wire.Value, issued wire.Timestamp) string {
	generic := "the request ID is recorded with different request bytes; the digest covers actor, role, queue, target, expected revision, operation, canonical payload and issuedAt, " +
		"and issuedAt defaulted to " + string(issued) + " because --issued-at was not given: a retry replays only with the original --issued-at"
	idx := journal.RequestIndex{Reader: journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: st.Queue.QueueID, PrimaryWorktree: repo.PrimaryWorktree}}
	entry, found, err := idx.Lookup(flags.requestID)
	if err != nil || !found || idx.TicketID == "" {
		return generic
	}
	var bases []wire.Timestamp
	if entry.Outcome.ReceiptSeq != nil {
		if at, err := store.ReceiptRecordedAt(repo, entry.Outcome.ReceiptSeq.Uint64()); err == nil {
			bases = append(bases, at)
		}
	}
	if rec, ok := st.Inventory.Get(idx.TicketID); ok {
		bases = append(bases, rec.CreatedAt, rec.UpdatedAt)
	}
	seen := map[wire.Timestamp]bool{issued: true}
	for _, base := range bases {
		t, err := time.Parse(time.RFC3339Nano, string(base))
		if err != nil {
			continue
		}
		for lead := 0; lead <= 2; lead++ {
			at := wire.Timestamp(t.Add(-time.Duration(lead) * time.Second).Format("2006-01-02T15:04:05Z"))
			if seen[at] {
				continue
			}
			seen[at] = true
			envelope, err := buildEnvelope(operation, st.Queue.QueueID.Raw, actor, flags, payload, at)
			if err != nil {
				return generic
			}
			d, err := transaction.Digest(transaction.Request{Operation: transaction.Mutate, QueueID: st.Queue.QueueID.Raw, RequestID: flags.requestID, Actor: actor, Envelope: envelope})
			if err == nil && d == entry.MutationSha256 {
				return "request " + flags.requestID + " is recorded for ticket " + idx.TicketID + "; this retry differs from the recorded request only in issuedAt: the original was " +
					string(at) + " and this one defaulted to " + string(issued) + " because --issued-at was not given; retry with --issued-at " + string(at) + " to replay"
			}
		}
	}
	return generic
}

// recordedCurrentRevision is the revision `--expected-revision current`
// submits (TCL-V0-860). The live revision is flags.expected; after a
// recorded write it has advanced past the one the original request was
// digested with, so a retry with the same --request-id and --issued-at
// would conflict. When the request index records this request ID, the
// envelope is rebuilt at each earlier revision, and the one whose digest
// equals the recorded digest is submitted, so the store replays the
// recorded result. Without --issued-at, a recorded entry or a matching
// digest, the live revision stands and the store judges it as before.
func recordedCurrentRevision(env Env, operation string, actor mutation.Binding, flags mutateFlags, payload wire.Value) string {
	live := flags.expected
	n, err := strconv.ParseUint(live, 10, 64)
	if flags.issuedAt == "" || err != nil {
		return live
	}
	issued, err := wire.ParseTimestamp("issuedAt", flags.issuedAt)
	if err != nil {
		return live
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return live
	}
	st, err := intent.Load(repo.IntentRoot())
	if err != nil {
		return live
	}
	idx := journal.RequestIndex{Reader: journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: st.Queue.QueueID, PrimaryWorktree: repo.PrimaryWorktree}}
	entry, found, err := idx.Lookup(flags.requestID)
	if err != nil || !found {
		return live
	}
	for r := n; r >= 1; r-- {
		f := flags
		f.expected = strconv.FormatUint(r, 10)
		envelope, err := buildEnvelope(operation, st.Queue.QueueID.Raw, actor, f, payload, issued)
		if err != nil {
			return live
		}
		d, err := transaction.Digest(transaction.Request{Operation: transaction.Mutate, QueueID: st.Queue.QueueID.Raw, RequestID: flags.requestID, Actor: actor, Envelope: envelope})
		if err == nil && d == entry.MutationSha256 {
			return f.expected
		}
	}
	return live
}
