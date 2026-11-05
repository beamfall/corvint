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
// CLI defaulted to now (CAL-V0-250, V1-1384). A retry that differs from the
// recorded request only in that defaulted issuedAt never reaches it: the
// writer reuses the recorded issuedAt and replays (TCL-V0-800, V1-3958). So
// the request differs in more than issuedAt, and this names the digest inputs
// and never attributes the conflict to issuedAt alone.
func defaultedIssuedAtConflict(issued wire.Timestamp) string {
	return "the request ID is recorded with different request bytes; the digest covers actor, role, queue, target, expected revision, operation, canonical payload and issuedAt, " +
		"and issuedAt defaulted to " + string(issued) + " because --issued-at was not given: a retry replays only with the original --issued-at"
}

// recordedIssuedAt recovers the issuedAt of a recorded request whose retry
// omitted --issued-at (TCL-V0-800, TCL-V0-735, V1-3958): it rebuilds this invocation's
// envelope with the recordedAt of the receipt the request-index entry names
// (ATR-V0-275), then the recorded ticket's createdAt and updatedAt, each
// also up to two seconds earlier since the writer clock can tick past the CLI
// second, and returns the one whose
// digest equals the recorded mutation digest, with the recorded ticket. A
// different payload, actor, target or revision never matches, so it never
// turns a real conflict into a replay. found is false when nothing matched.
func recordedIssuedAt(repo *intent.Repository, st *intent.Store, operation string, actor mutation.Binding, flags mutateFlags, payload wire.Value, issued wire.Timestamp) (at wire.Timestamp, ticketID string, found bool) {
	if flags.requestID == "" {
		return "", "", false
	}
	idx := store.RequestIndexOf(repo, journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: st.Queue.QueueID, PrimaryWorktree: repo.PrimaryWorktree})
	entry, ok, err := idx.Lookup(flags.requestID)
	if err != nil || !ok || idx.TicketID == "" {
		return "", "", false
	}
	var bases []wire.Timestamp
	if entry.Outcome.ReceiptSeq != nil {
		if at, err := store.ReceiptRecordedAt(repo, entry.Outcome.ReceiptSeq.Uint64()); err == nil {
			bases = append(bases, at)
		}
	}
	if rec, ok := st.Inventory.Get(ticketID); ok {
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
				return "", "", false
			}
			d, err := transaction.Digest(transaction.Request{Operation: transaction.Mutate, QueueID: st.Queue.QueueID.Raw, RequestID: flags.requestID, Actor: actor, Envelope: envelope})
			if err == nil && d == entry.MutationSha256 {
				return at, idx.TicketID, true
			}
		}
	}
	return "", "", false
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
func recordedCurrentRevision(env Env, operation string, actor mutation.Binding, flags mutateFlags, payload wire.Value) (string, error) {
	live := flags.expected
	n, err := strconv.ParseUint(live, 10, 64)
	if flags.issuedAt == "" || err != nil {
		return live, nil
	}
	issued, err := wire.ParseTimestamp("issuedAt", flags.issuedAt)
	if err != nil {
		return live, nil
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return "", err
	}
	st, err := intent.Load(repo.IntentRoot())
	if err != nil {
		return "", err
	}
	idx := store.RequestIndexOf(repo, journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: st.Queue.QueueID, PrimaryWorktree: repo.PrimaryWorktree})
	r, ok, err := recordedRevision(idx, operation, st.Queue.QueueID.Raw, actor, flags, payload, issued, n)
	if wire.CodeOf(err) == wire.CodeRedoPending {
		if _, e := store.Settle(env.writerContext(), repo); e != nil {
			return "", e
		}
		r, ok, err = recordedRevision(idx, operation, st.Queue.QueueID.Raw, actor, flags, payload, issued, n)
	}
	if err != nil {
		return "", wire.Annotatef(err, "request index "+flags.requestID, "%v", err)
	}
	if ok {
		return r, nil
	}
	return live, nil
}

// recordedRevision is the revision at or below n whose envelope digest
// equals the one the request index records for flags.requestID, whatever
// that request's outcome, so a retry submits exactly the recorded request.
// ok is false when the index holds no such request or no revision matches;
// err is a request index that could not be read (TCL-V0-860, TLF-V0-1102).
func recordedRevision(idx journal.RequestIndex, operation, queueID string, actor mutation.Binding, flags mutateFlags, payload wire.Value, issued wire.Timestamp, n uint64) (string, bool, error) {
	entry, found, err := idx.Lookup(flags.requestID)
	if err != nil || !found {
		return "", false, err
	}
	for r := n; r >= 1; r-- {
		f := flags
		f.expected = strconv.FormatUint(r, 10)
		envelope, err := buildEnvelope(operation, queueID, actor, f, payload, issued)
		if err != nil {
			return "", false, nil
		}
		d, err := transaction.Digest(transaction.Request{Operation: transaction.Mutate, QueueID: queueID, RequestID: flags.requestID, Actor: actor, Envelope: envelope})
		if err == nil && d == entry.MutationSha256 {
			return f.expected, true, nil
		}
	}
	return "", false, nil
}
