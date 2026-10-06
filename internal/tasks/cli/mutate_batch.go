package cli

import (
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Batch refine input bounds (CAL-V0-106).
const (
	maxBatchEntries = 1000
	maxBatchBytes   = 8 * wire.MiB
)

// batchEntryOutcomes are the per-entry outcomes a batch reports besides the
// taskman-outcome/0 outcome of an entry that ran.
const (
	batchNotAttempted = "NOT_ATTEMPTED"
	batchError        = "ERROR"
)

// hasBatchFlag reports whether args select the batch form of a verb.
func hasBatchFlag(args []string) bool {
	return slices.Contains(args, "--batch")
}

// batchEntryRequestID derives entry i's request ID from the batch request ID
// (CAL-V0-106), so a retry of the same batch replays each entry under the
// single-mutation request-ID contract.
func batchEntryRequestID(batch string, i int) string {
	return batch + "/" + strconv.Itoa(i)
}

// refineBatchCommand runs `ticket refine --batch`: a JSON array of
// {target, expectedRevision, payload} entries, each the single-ticket refine
// of that target. Every entry is validated before anything is locked or
// written; the store then applies the entries in bounded chunks that release
// the writer lock between chunks (CAL-V0-106).
func refineBatchCommand(env Env, args []string) *wire.Result {
	cmd := []string{"ticket", "refine"}
	rest := make([]string, 0, len(args))
	for _, a := range args {
		if a != "--batch" {
			rest = append(rest, a)
		}
	}
	flags, res := parseMutateFlags(cmd, rest)
	if res != nil {
		return res
	}
	if flags.help {
		return mutationHelp(cmd, mutation.OpRefine)
	}
	if flags.template || flags.target != "" || flags.expected != "" {
		return usage(cmd, "--batch takes each target and expected revision from its entries; --target, --expected-revision and --template are refused")
	}
	actor, err := initActor(flags.role)
	if err != nil {
		return errorResult(cmd, err)
	}
	if flags.requestID == "" {
		return usage(cmd, "--request-id is required: entry i replays under request ID <request-id>/<i>")
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	store0, err := intent.Load(repo.IntentRoot())
	if err != nil {
		return errorResult(cmd, err)
	}
	now, err := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return errorResult(cmd, err)
	}
	issued := now
	if flags.issuedAt != "" {
		if issued, err = wire.ParseTimestamp("issuedAt", flags.issuedAt); err != nil {
			return errorResult(cmd, err)
		}
	}
	envelopes, targets, err := batchEnvelopes(env, flags, store0, actor, issued)
	if err != nil {
		return errorResult(cmd, err)
	}
	report, err := store.MutateBatch(writerContext(), repo, actor, envelopes, now)
	if err != nil {
		return errorResult(cmd, err)
	}
	return batchResult(cmd, flags.requestID, issued, targets, report)
}

// batchEnvelopes reads and validates the whole batch and returns one
// canonical REFINE envelope per entry, in input order, with the qualified
// target of each. Nothing is written; any refusal names its entry.
func batchEnvelopes(env Env, flags mutateFlags, st *intent.Store, actor mutation.Binding, issued wire.Timestamp) ([][]byte, []string, error) {
	raw := flags.payload
	if flags.payloadFromStdin {
		data, err := io.ReadAll(io.LimitReader(env.Stdin, int64(maxBatchBytes)+1))
		if err != nil {
			return nil, nil, err
		}
		if len(data) > maxBatchBytes {
			return nil, nil, wire.Errorf(wire.CodeLimitExceeded, "payload", "batch larger than %d bytes", maxBatchBytes)
		}
		raw = string(data)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, nil, wire.Errorf(wire.CodeMalformed, "payload", "no batch: pass a JSON array of {target, expectedRevision, payload} with --payload-stdin or --payload")
	}
	v, err := wire.ParseInput([]byte(raw))
	if err != nil {
		return nil, nil, err
	}
	if v.Kind != wire.KindArray || len(v.Arr) == 0 {
		return nil, nil, wire.Errorf(wire.CodeMalformed, "/", "a batch is a non-empty JSON array of {target, expectedRevision, payload}")
	}
	if len(v.Arr) > maxBatchEntries {
		return nil, nil, wire.Errorf(wire.CodeLimitExceeded, "/", "batch longer than %d entries (%d)", maxBatchEntries, len(v.Arr))
	}
	if len(batchEntryRequestID(flags.requestID, len(v.Arr)-1)) > wire.MaxRequestIDBytes {
		return nil, nil, wire.Errorf(wire.CodeLimitExceeded, "requestId", "entry request IDs <request-id>/<index> would exceed %d bytes", wire.MaxRequestIDBytes)
	}
	queueID := st.Queue.QueueID.Raw
	known := make(map[string]bool, len(st.Tickets))
	for _, rec := range st.Tickets {
		known[rec.TicketID.Raw] = true
	}
	seen := map[string]int{}
	envelopes := make([][]byte, len(v.Arr))
	targets := make([]string, len(v.Arr))
	for i, entry := range v.Arr {
		where := "/" + strconv.Itoa(i)
		r := wire.NewReader(entry, where).Closed("target", "expectedRevision", "payload")
		target := r.Field("target").Identifier()
		expected := r.Field("expectedRevision").Count()
		if err := r.Err(); err != nil {
			return nil, nil, err
		}
		id, err := wire.ParseTicketID(where+"/target", qualifyTicket(queueID, target))
		if err != nil {
			return nil, nil, err
		}
		if !known[id.Raw] {
			return nil, nil, wire.Errorf(wire.CodeOutOfScope, where+"/target", "no ticket %s in this queue", id.Raw)
		}
		if first, dup := seen[id.Raw]; dup {
			return nil, nil, wire.Errorf(wire.CodeDuplicateID, where+"/target", "ticket %s is already entry %d; a batch names each target once", id.Raw, first)
		}
		seen[id.Raw] = i
		payload, err := mutation.CanonicalPayload(mutation.OpRefine, entry.Obj.Vals["payload"])
		if err != nil {
			return nil, nil, wire.Errorf(wire.CodeOf(err), where+"/payload", "%v", err)
		}
		f := flags
		f.requestID, f.target, f.expected = batchEntryRequestID(flags.requestID, i), id.Raw, string(expected)
		envelope, err := buildEnvelope(mutation.OpRefine, queueID, actor, f, payload, issued)
		if err == nil {
			_, err = mutation.Decode(envelope)
		}
		if err != nil {
			return nil, nil, wire.Errorf(wire.CodeOf(err), where, "%v", err)
		}
		envelopes[i], targets[i] = envelope, id.Raw
	}
	return envelopes, targets, nil
}

// batchResult renders one item: the batch facts and one entry per input
// entry, in input order. The result is OK only when every entry completed
// (fresh or replayed); otherwise it carries the union of the entries' codes.
func batchResult(cmd []string, requestID string, issued wire.Timestamp, targets []string, report *store.BatchReport) *wire.Result {
	res := &wire.Result{Command: cmd, Outcome: wire.OutcomeOK}
	codes := map[string]bool{}
	counts := map[string]int{}
	entries := make([]wire.Value, len(targets))
	for i, target := range targets {
		e := report.Entries[i]
		o := wire.NewObject()
		o.Set("index", wire.String(strconv.Itoa(i)))
		o.Set("requestId", wire.String(batchEntryRequestID(requestID, i)))
		o.Set("ticketId", wire.String(target))
		o.Set("chunk", wire.String(strconv.Itoa(e.Chunk)))
		outcome, entryCodes, detail := batchNotAttempted, []string{}, ""
		receipt, replayed := "", false
		revision := wire.Null()
		switch {
		case e.Err != nil:
			outcome, entryCodes, detail = batchError, []string{wire.CodeOf(e.Err)}, e.Err.Error()
			res.Outcome = wire.OutcomeError
			res.NotRetryable = res.NotRetryable || wire.RetryForbidden(e.Err)
		case e.Report != nil:
			outcome, detail, receipt = e.Report.Outcome.Outcome, e.Report.Detail, e.Report.Receipt
			replayed = e.Report.Outcome.Replayed
			revision = nullableCount(e.Report.Outcome.ResultingRevision)
			if outcome != mutation.OutcomeCompleted {
				entryCodes = append(entryCodes, e.Report.Outcome.Codes...)
				if res.Outcome == wire.OutcomeOK {
					res.Outcome = wire.OutcomeRefused
				}
			}
			if e.Report.Redone {
				res.Warnings = append(res.Warnings, "a receipt left pending by an interrupted run was completed before this batch (§5.2 redo)")
			}
		default:
			if res.Outcome == wire.OutcomeOK {
				res.Outcome = wire.OutcomeRefused
			}
		}
		switch {
		case outcome == mutation.OutcomeCompleted && replayed:
			counts["replayed"]++
		case outcome == mutation.OutcomeCompleted:
			counts["completed"]++
		case outcome == batchNotAttempted:
			counts["notAttempted"]++
		default:
			counts["failed"]++
		}
		for _, c := range entryCodes {
			codes[c] = true
		}
		o.Set("outcome", wire.String(outcome))
		o.Set("codes", wire.Strings(entryCodes))
		o.Set("replayed", wire.Bool(replayed))
		o.Set("receipt", wire.String(receipt))
		o.Set("resultingRevision", revision)
		o.Set("detail", wire.String(prose0(detail)))
		entries[i] = wire.ObjectValue(o)
	}
	res.Codes = slices.Sorted(func(yield func(string) bool) {
		for c := range codes {
			if !yield(c) {
				return
			}
		}
	})
	o := wire.NewObject()
	o.Set("batchRequestId", wire.String(requestID))
	o.Set("issuedAt", wire.String(string(issued)))
	o.Set("chunkEntries", wire.String(strconv.Itoa(store.BatchChunkEntries)))
	o.Set("chunks", wire.String(strconv.Itoa(report.Chunks)))
	for _, k := range []string{"completed", "replayed", "failed", "notAttempted"} {
		o.Set(k, wire.String(strconv.Itoa(counts[k])))
	}
	o.Set("entries", wire.Array(entries...))
	res.Items = []wire.Value{wire.ObjectValue(o)}
	if res.Outcome != wire.OutcomeOK {
		res.Warnings = append(res.Warnings, "retry the same batch with the same --request-id and --issued-at "+string(issued)+": completed entries replay and are not applied again")
	}
	res.Warnings = append(res.Warnings,
		"the actor binding is a recorded local-operator claim, not an authentication (decision 0003); a real queue still needs the §7.4 cutover record")
	return res
}

// prose0 is prose for a member that may be empty.
func prose0(s string) string {
	if s == "" {
		return ""
	}
	return prose(s)
}

// batchHelp documents `ticket refine --batch` in `ticket refine --help`.
var batchHelp = "ticket refine --batch reads a JSON array of 1.." + strconv.Itoa(maxBatchEntries) +
	" entries {\"target\":TICKET|LOCAL,\"expectedRevision\":\"N\",\"payload\":{REFINE payload}} (at most " +
	strconv.Itoa(maxBatchBytes/wire.MiB) + " MiB, each target once). Every entry is validated (closed shape, " +
	"canonical REFINE payload, existing target, envelope) before anything is locked or written; one bad entry refuses the whole batch. " +
	"Entries then apply in input order in chunks of at most " + strconv.Itoa(store.BatchChunkEntries) +
	" entries, each chunk under one writer lock, released between chunks so claims and heartbeats can commit. " +
	"Each entry is an ordinary REFINE with its own expectedRevision CAS and receipt, under request ID <request-id>/<index>: " +
	"a stale entry is refused alone and the rest still apply. The result lists every entry's outcome and the batch issuedAt; " +
	"retry with the same --request-id and --issued-at to replay completed entries without applying them again (CAL-V0-106)."
