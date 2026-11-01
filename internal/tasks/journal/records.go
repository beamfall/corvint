package journal

import (
	"maps"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/release"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type latest struct {
	seq                wire.Size
	digest, pendingPre *wire.Digest
	pending            bool
	// bytes is the afterimage size (0 for a deletion), or -1 where the
	// walk resumed from a read checkpoint or a retired prefix, which carry
	// no sizes. Only a writer checkpoint derivation consumes it.
	bytes int64
}

// chain is the state one receipt hands to the next. A complete audit builds
// it from receipt 1; a checkpoint read resumes it at the checkpoint sequence.
type chain struct {
	canonical  map[string]latest
	prev       *wire.Digest
	generation uint64
	init       *snapshot.Init
	// primary is the primary worktree the chain has recorded so far: the
	// INIT descriptor's, then each RELOCATE descriptor's destination
	// (TRL-V0-002). A resumed walk seeds it from its checkpoint.
	primary       string
	genesisQueue  []byte
	selectedBytes int
	notes         *noteState
	// elided is the writer checkpoint a ModeWriter walk resumed from: its
	// request paths are retained history the tail must not repeat.
	elided *WriterCheckpoint
	// resumed is the checkpoint receipt a resumed walk starts after (a read
	// or writer checkpoint); zero for a walk from receipt 1. resumedSha is
	// that checkpoint's receipt digest (ON-V0-015).
	resumed    uint64
	resumedSha wire.Digest
	// requests is the read checkpoint's filter of request paths posted at or
	// before resumed (CAL-V0-321); nil on every other walk.
	requests *RequestFilter
	// restore rebuilds elided liveness receipts (taskman-receipt/1,
	// CAL-V0-301) on a complete walk from receipt 1. A walk resumed from a
	// checkpoint has none: it refuses a tombstone as an unusable checkpoint so
	// the caller falls back to the complete walk.
	restore *snapshot.ReceiptRestorer
	// seal is the retired prefix a sealed complete walk started from
	// (JRT-V0-004); checkpoint routes decline a chain that carries one.
	seal *RetiredPrefix
	// prefetch holds receipts a complete walk read and decoded ahead of
	// step (V1-1701); nil on every other route.
	prefetch *receiptPrefetch
	// atSealed is the state a complete walk of a segmented store held as it
	// passed the sealed sequence, from which its writer checkpoint keeps a
	// base there (JSG-V0-780); nil on every other walk.
	atSealed *sealedCapture
	// posted names every path a ModeWriter walk's tail posted; the writer
	// checkpoint and the liveness intent summary adjust only these
	// (JSG-V0-001). Nil on every other walk.
	posted map[string]bool
	// view marks a ModeWriter walk whose canonical holds only what its tail
	// posted: every other latest entry is looked up in elided where it is
	// used, never copied (JSG-V0-004). added counts the paths the tail
	// posted that elided does not hold.
	view  bool
	added int
}

// get returns the latest afterimage of p: the walk's own, else, on a view
// walk, the writer checkpoint's.
func (st *chain) get(p string) (latest, bool, error) {
	if l, ok := st.canonical[p]; ok {
		return l, true, nil
	}
	if st.view {
		return st.elided.lookup(p)
	}
	return latest{}, false, nil
}

// has reports whether p has a latest entry, without decoding one.
func (st *chain) has(p string) bool {
	if _, ok := st.canonical[p]; ok {
		return true
	}
	return st.view && st.elided.holds(p)
}

// count is the number of latest entries, request paths the tail posted
// included.
func (st *chain) count() int {
	if st.view {
		return st.added + st.elided.count()
	}
	return len(st.canonical)
}

// each calls fn for every latest entry in path order, skipping every path
// under a prefix in skip (each ends in '/'). On a view walk the tail's
// entry replaces the checkpoint's.
func (st *chain) each(skip []string, fn func(string, latest) error) error {
	keys := make([]string, 0, len(st.canonical))
	for p := range st.canonical {
		if skipped(p, skip) == "" {
			keys = append(keys, p)
		}
	}
	slices.Sort(keys)
	i := 0
	if st.view {
		err := st.elided.each(skip, func(p string, l latest) error {
			for ; i < len(keys) && keys[i] < p; i++ {
				if err := fn(keys[i], st.canonical[keys[i]]); err != nil {
					return err
				}
			}
			if i < len(keys) && keys[i] == p {
				l = st.canonical[p]
				i++
			}
			return fn(p, l)
		})
		if err != nil {
			return err
		}
	}
	for ; i < len(keys); i++ {
		if err := fn(keys[i], st.canonical[keys[i]]); err != nil {
			return err
		}
	}
	return nil
}

// inMap is has for a plain latest map.
func inMap(m map[string]latest) func(string) bool {
	return func(p string) bool {
		_, ok := m[p]
		return ok
	}
}

// decode decodes one receipt of the walk and returns it with its original
// bytes, the bytes the chain, the head and every fold bind.
func (st *chain) decode(name string, raw []byte) (*snapshot.Receipt, []byte, error) {
	if st.restore != nil {
		return st.restore.Next(raw)
	}
	rc, err := snapshot.DecodeReceipt(raw)
	if err != nil {
		if _, _, elided, herr := snapshot.DecodeRetainedReceipt(raw); herr == nil && elided {
			return nil, nil, errCheckpoint(name, "an elided liveness receipt needs the complete walk from receipt 1")
		}
		return nil, nil, err
	}
	return rc, raw, nil
}

// statePath names the private mutable state SelectState retains.
func statePath(p string) bool {
	return p == "reservations.json" || p == "pools.json" || strings.HasPrefix(p, "attempts/")
}

func (r Reader) selects(p string, selected map[string]bool) bool {
	return selected[p] || (r.writerCache && writerRecord(p)) || (r.SelectState && statePath(p))
}

func equalDigest(a, b *wire.Digest) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (r Reader) walk(o *observation, selected map[string]bool, request string, lim limits, checkIntent bool) (*Result, error) {
	result := &Result{Identity: o.identity, Head: o.head, Mode: ModeFull, StagingPresent: o.staging, Records: map[string]Record{}, StructuralConsistency: "NOT_OBSERVED", ProjectionAgreement: "NOT_OBSERVED", SemanticCoverage: "NOT_OBSERVED", HistoricalAcceptance: "NOT_OBSERVED", ActorAuthentication: "NOT_OBSERVED", Liveness: "NOT_OBSERVED", RuntimeQualification: "NOT_OBSERVED"}
	if r.writerCache {
		result.RequestDigests = map[string]wire.Digest{}
	}
	if r.ReceiptFold != nil {
		result.fold = r.ReceiptFold()
	}
	if r.handoffPolicy != nil {
		result.HandoffPolicy = &HandoffPolicyHistory{Selector: *r.handoffPolicy, Compatible: true}
	}
	if o.segErr != nil {
		return result, o.segErr
	}
	if o.stageErr != nil {
		return result, o.stageErr
	}
	if len(o.receipts) == 0 && o.head == nil {
		// Preserve legacy uninitialized-remnant reporting. The newly observed
		// root identities and an empty persistent staging directory add no remnant.
		for p := range o.files {
			if p != "." && p != "intent" && p != "staging" {
				result.StagingPresent = true
				break
			}
		}
		return result, wire.Errorf(wire.CodeUninitialized, "/", "no head or linked genesis; no initialization or cleanup performed")
	}
	if o.head != nil {
		if err := r.validateStage(o, nil); err != nil {
			return result, err
		}
	}
	result.StagingPresent = o.staging || len(o.stageDigests) > 0
	result.OrphanStageSlots = o.orphanSlots
	headSeq := uint64(0)
	if o.head != nil {
		headSeq = o.head.LastSeq.Uint64()
		if o.head.QueueID != r.QueueID || o.head.PrimaryWorktree != r.PrimaryWorktree {
			return result, wire.Errorf(wire.CodeJournalForked, "head.json", "selected queue/primary differs from head; relocation needs an explicit RELOCATE receipt")
		}
	}
	if o.seal != nil {
		st, err := r.walkSealed(o, result, selected, request, lim, headSeq)
		if err != nil {
			return result, err
		}
		return r.settle(o, st, result, checkIntent)
	}
	count := uint64(len(o.receipts))
	if count < headSeq || count > headSeq+1 || count == 0 {
		return result, wire.Errorf(wire.CodeJournalForked, "receipts", "receipt count must equal head or head+1")
	}
	for i, name := range o.receipts {
		seq, err := receiptSeq(name)
		if err != nil {
			return result, err
		}
		if seq != uint64(i+1) {
			return result, wire.Errorf(wire.CodeJournalForked, name, "receipt gap or late extra")
		}
	}
	result.Pending = count == headSeq+1
	if result.Pending && (r.divergentIntent != "" || (r.unpauseTickets && !r.pendingUnpause)) {
		return result, wire.Errorf(wire.CodeRedoPending, "receipts", "this observation requires a settled journal")
	}
	st := &chain{canonical: map[string]latest{}, restore: &snapshot.ReceiptRestorer{}, prefetch: newReceiptPrefetch()}
	sealedAt := uint64(0)
	if o.head != nil && o.head.Segments != nil && o.head.Segments.Through.Uint64() < headSeq {
		sealedAt = o.head.Segments.Through.Uint64()
	}
	for i, name := range o.receipts {
		if i%abortEvery == abortEvery-1 && r.headMoved(o) {
			return result, moved("head.json")
		}
		r.prefetchFrom(st.prefetch, o.receipts, i)
		if err := r.step(o, st, result, name, uint64(i+1), headSeq, selected, request, lim); err != nil {
			return result, err
		}
		if uint64(i+1) == sealedAt {
			st.atSealed = st.captureSealed(result)
		}
	}
	if err := st.restore.Superseded(); err != nil {
		return result, err
	}
	return r.settle(o, st, result, checkIntent)
}

// settle completes a complete walk once every receipt is folded: projection
// agreement, the intent observations and the pending-redo refusal.
func (r Reader) settle(o *observation, st *chain, result *Result, checkIntent bool) (*Result, error) {
	result.chain = st
	canonical, genesisQueue := st.canonical, st.genesisQueue
	result.StructuralConsistency = "CONSISTENT"
	if result.SemanticCoverage != "UNKNOWN" {
		result.SemanticCoverage = "KNOWN_CODECS"
	}
	if err := r.projections(o, canonical, checkIntent && !r.writerCache); err != nil {
		return result, err
	}
	if r.writerCache {
		onlyIntent := r
		onlyIntent.intentOnly = true
		result.IntentError = onlyIntent.projections(o, canonical, true)
	}
	if r.observedIntent {
		onlyIntent := r
		onlyIntent.intentOnly = true
		result.intentErr = onlyIntent.projections(o, canonical, true)
	}
	if o.head == nil {
		if err := r.validateStage(o, genesisQueue); err != nil {
			return result, err
		}
	}
	result.ProjectionAgreement = "AGREES"
	if r.writerCache && result.IntentError != nil {
		result.ProjectionAgreement = "PRIVATE_AGREES_INTENT_NOT_OBSERVED"
	}
	if r.divergentIntent != "" {
		result.ProjectionAgreement = "ALL_EXCEPT_TARGET_AGREE"
	}
	if r.unpauseTickets {
		result.ProjectionAgreement = "TICKETS_NOT_COMPARED"
	}
	if !checkIntent {
		result.ProjectionAgreement = "PRIVATE_AGREES_INTENT_NOT_OBSERVED"
	}
	if result.Pending {
		if result.IntentError != nil {
			return result, result.IntentError
		}
		if !r.unpauseTickets {
			result.ProjectionAgreement = "PRE_OR_POST"
		}
		return result, wire.Errorf(wire.CodeRedoPending, "receipts", "one fully validated linked receipt awaits head projection; no redo performed")
	}
	return result, nil
}

// step validates one receipt against the chain and folds its posts in.
func (r Reader) step(o *observation, st *chain, result *Result, name string, seq, headSeq uint64, selected map[string]bool, request string, lim limits) error {
	canonical := st.canonical
	pre := st.prefetch.take(seq)
	rc, raw, err := pre.receipt(r, st, name)
	if err != nil {
		return err
	}
	if rc.Seq.Uint64() != seq || !equalDigest(rc.Prev, st.prev) || rc.HeadGeneration.Uint64() < st.generation {
		return wire.Errorf(wire.CodeJournalForked, name, "seq/prev/generation does not continue chain")
	}
	if rc.TicketID != nil && rc.TicketID.QueueID() != r.QueueID.Raw {
		return wire.Errorf(wire.CodeJournalForked, name, "receipt ticket scope differs")
	}
	if rc.AttemptID != nil {
		q, err := snapshot.AttemptQueue(*rc.AttemptID)
		if err != nil {
			return err
		}
		if q != r.QueueID {
			return wire.Errorf(wire.CodeJournalForked, name, "receipt attempt scope differs")
		}
	}
	digest := wire.Sum(raw)
	if seq == 1 && o.head != nil && o.head.InitSha256 != digest {
		return wire.Errorf(wire.CodeJournalForked, name, "head INIT digest differs from complete receipt 1")
	}
	if seq == headSeq && (*o.head.LastReceiptSha256 != digest || o.head.Generation != rc.HeadGeneration) {
		return wire.Errorf(wire.CodeJournalForked, name, "head digest/generation differs")
	}
	requiredGenesis := map[string]bool{}
	descriptorCount := 0
	requestCount := 0
	relocateCount := 0
	var relocate *snapshot.Relocate
	var boundRequest *snapshot.Request
	var target *ticket.Record
	notes := r.noteAudit(st, rc)
	for j, p := range rc.Post {
		prior, found, err := st.get(p.Path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(p.Path, "requests/") && (prior.seq != "" || (st.seal != nil && st.seal.Anchor.HasRequest(p.Path))) {
			return wire.Errorf(wire.CodeJournalForked, p.Path, "request ID occurs more than once in retained history")
		}
		if strings.HasPrefix(p.Path, "requests/") && st.elided != nil && st.elided.HasRequest(p.Path) {
			return errCheckpoint(p.Path, "request ID repeats one retained before the checkpoint")
		}
		if strings.HasPrefix(p.Path, "requests/") && st.resumed > 0 && prior.seq == "" && rc.Pre[j].Sha256 != nil {
			// A request is posted once, so a pre-state means the path already
			// held a request: one the read checkpoint does not retain. The
			// complete audit classifies it (PSR-V0-190, V1-0917).
			return errCheckpoint(p.Path, "request post names a pre-state the checkpoint does not retain")
		}
		if strings.HasPrefix(p.Path, "requests/") && st.requests != nil && prior.seq == "" && st.requests.mayHold(p.Path) {
			// A null pre-state proves nothing: the request may have been
			// posted before the checkpoint. The filter has no false
			// negatives, so a miss proves it was not; a hit is classified
			// by the complete audit (PSR-V0-203, V1-2352).
			return errCheckpoint(p.Path, "request post may repeat one posted before the checkpoint")
		}
		if !found && st.count() >= lim.scan+intent.MaxIntentRootEntries+wire.MaxTicketsPerQueue+wire.MaxReleasesPerQueue {
			return wire.Errorf(wire.CodeLimitExceeded, p.Path, "latest metadata exceeds store scan bound")
		}
		checked := pre.post(j)
		var post []byte
		if checked != nil {
			post = checked.raw
		} else if post, err = r.postBytes(p); err != nil {
			return err
		}
		notes.observe(j, p, prior, post)
		var postTicket *ticket.Record
		var postRequest *snapshot.Request
		if post != nil {
			if checked == nil {
				checked = &checkedPost{raw: post}
				if checked.coverage, checked.descriptor, checked.decoded, err = r.validateRecord(p.Path, post, rc); err != nil {
					return err
				}
			}
			coverage, descriptor, decoded := checked.coverage, checked.descriptor, checked.decoded
			postTicket, postRequest = decoded.ticket, decoded.request
			if decoded.relocate != nil {
				relocateCount++
				relocate = decoded.relocate
			}
			notes.decoded(j, postTicket)
			if !coverage {
				result.SemanticCoverage = "UNKNOWN"
			}
			if descriptor != nil {
				if seq != 1 {
					return wire.Errorf(wire.CodeJournalForked, p.Path, "INIT descriptor may only be posted in receipt 1")
				}
				descriptorCount++
				st.init = descriptor
			}
		}
		if o.head == nil && seq == 1 && p.Path == "intent/queue.json" {
			st.genesisQueue = post
		}
		if result.HandoffPolicy != nil {
			if err := observeHandoffPolicy(result.HandoffPolicy, st, rc, p, post, digest, lim); err != nil {
				return err
			}
		}
		if rc.TicketID != nil && p.Path == "intent/tickets/"+rc.TicketID.Local+".json" && post != nil {
			// validateRecord decoded this exact post (V1-1701).
			target = postTicket
		}
		if seq == 1 {
			if rc.Pre[j].Sha256 != nil || p.Sha256 == nil || strings.HasPrefix(p.Path, "attempts/") || strings.HasPrefix(p.Path, "effects/") {
				return wire.Errorf(wire.CodeJournalForked, p.Path, "genesis cannot update/delete existing state or post attempts/effects")
			}
			requiredGenesis[p.Path] = true
		}
		if strings.HasPrefix(p.Path, "requests/") {
			req := postRequest
			if req == nil {
				// A deleted request (nil post) is refused here as before.
				if req, err = snapshot.DecodeRequest(post); err != nil {
					return err
				}
			}
			expected, _ := snapshot.RequestPath(req.Entry.RequestID)
			if rc.RequestID == nil || req.Entry.RequestID != *rc.RequestID || req.Seq != rc.Seq || expected != p.Path || req.Entry.Outcome.Outcome != rc.Outcome || strings.Join(req.Entry.Outcome.Codes, "\x00") != strings.Join(rc.Codes, "\x00") {
				return wire.Errorf(wire.CodeJournalForked, p.Path, "request afterimage does not bind receipt outcome")
			}
			requestCount++
			boundRequest = req
			if r.writerCache {
				result.RequestDigests[p.Path] = wire.Sum(post)
			}
			if req.Entry.RequestID == request {
				result.request = req
				if rc.TicketID != nil {
					result.requestTicket = rc.TicketID.Raw
				}
			}
		}
		canonical[p.Path] = latest{seq: rc.Seq, digest: p.Sha256, pendingPre: rc.Pre[j].Sha256, pending: seq > headSeq, bytes: int64(len(post))}
		if st.view && !found {
			st.added++
		}
		if st.posted != nil {
			st.posted[p.Path] = true
		}
		if result.selectErr == nil && r.selects(p.Path, selected) {
			old := result.Records[p.Path]
			st.selectedBytes -= len(old.Raw)
			if len(post) > lim.selected-st.selectedBytes {
				err := wire.Errorf(wire.CodeLimitExceeded, p.Path, "selected canonical bytes exceed aggregate live intent-state budget; select fewer paths")
				if !r.observedIntent {
					return err
				}
				// A separate Audit would stop here, after the lookup succeeded.
				// Keep validating so every lookup refusal still comes first.
				result.selectErr = err
			} else {
				st.selectedBytes += len(post)
				result.Records[p.Path] = Record{Seq: rc.Seq, Sha256: p.Sha256, Raw: post}
			}
		}
	}
	if rc.RequestID != nil && requestCount != 1 {
		return wire.Errorf(wire.CodeJournalForked, name, "non-null requestId requires one request afterimage")
	}
	if err := bindRevisions(rc, boundRequest, target); err != nil {
		return err
	}
	if err := notes.bind(boundRequest, target); err != nil {
		return err
	}
	if seq == 1 {
		// A non-fixture queue's genesis may also post its import map (CTS-V0-014).
		maps := 0
		if requiredGenesis["intent/import-map.json"] {
			maps = 1
		}
		if len(rc.Post) != 5+requestCount+maps || descriptorCount != 1 || !requiredGenesis["VERSION"] || !requiredGenesis["intent/queue.json"] || !requiredGenesis["intent/policy.json"] || !requiredGenesis["reservations.json"] {
			return wire.Errorf(wire.CodeJournalForked, name, "genesis requires VERSION blob, queue, policy, empty reservations and unique INIT descriptor")
		}
		// A relocated store's head names a later primary worktree; the
		// chain carries it from the INIT descriptor through each RELOCATE
		// receipt and binds it to head at headSeq (TRL-V0-002).
		if st.init.VersionSha256 != wire.Sum([]byte(snapshot.VersionBytes)) || st.init.QueueID != r.QueueID || (o.head == nil && st.init.PrimaryWorktree != r.PrimaryWorktree) {
			return wire.Errorf(wire.CodeJournalForked, name, "genesis identity/version mismatch")
		}
		if o.head != nil && (o.head.VersionSha256 != st.init.VersionSha256 || o.head.QueueID != st.init.QueueID) {
			return wire.Errorf(wire.CodeJournalForked, "head.json", "head and INIT descriptor disagree")
		}
		st.primary = st.init.PrimaryWorktree
	}
	if relocateCount != 0 || rc.Kind == snapshot.StageRelocate {
		if rc.Kind != snapshot.StageRelocate || relocateCount != 1 || seq == 1 || rc.RequestID == nil || relocate.RequestID != *rc.RequestID || relocate.QueueID != r.QueueID || relocate.FromPrimaryWorktree != st.primary {
			return wire.Errorf(wire.CodeJournalForked, name, "a RELOCATE receipt posts exactly one descriptor continuing the recorded primary worktree")
		}
		st.primary = relocate.ToPrimaryWorktree
	}
	if seq == headSeq && st.primary != o.head.PrimaryWorktree {
		return wire.Errorf(wire.CodeJournalForked, "head.json", "head primary worktree differs from the receipt chain")
	}
	st.prev = &digest
	st.generation = rc.HeadGeneration.Uint64()
	result.LastSeq = rc.Seq
	result.LastReceiptSha256 = digest
	if result.fold != nil && result.foldErr == nil {
		if result.foldErr = result.fold(rc, digest); result.foldErr == nil {
			result.folded = seq
		}
	}
	return nil
}

// ReceiptFold reports the Reader.ReceiptFold outcome of a complete audit:
// the last sequence folded without error, contiguously from 1 (0 when no
// fold ran), and the fold's first error.
func (r *Result) ReceiptFold() (folded uint64, err error) {
	return r.folded, r.foldErr
}

func observeHandoffPolicy(h *HandoffPolicyHistory, st *chain, rc *snapshot.Receipt, p snapshot.PostEntry, raw []byte, receipt wire.Digest, lim limits) error {
	if p.Path == "intent/policy.json" {
		if p.Sha256 == nil {
			h.FinalPolicySha256 = ""
			h.Compatible = false
		} else {
			h.FinalPolicySha256 = *p.Sha256
			if h.OriginalPolicy.Raw == nil && *p.Sha256 == h.Selector.OriginalPolicySha256 {
				if len(raw) > lim.selected-st.selectedBytes {
					return wire.Errorf(wire.CodeLimitExceeded, p.Path, "original handoff policy exceeds aggregate selected-byte budget")
				}
				st.selectedBytes += len(raw)
				h.OriginalPolicy = Record{Seq: rc.Seq, Sha256: p.Sha256, Raw: append([]byte(nil), raw...)}
				h.OriginalReceiptSha256 = receipt
			} else if h.OriginalPolicy.Raw != nil {
				compatible, err := intent.HandoffPolicyCompatible(h.OriginalPolicy.Raw, raw, h.Selector.PoolID, h.Selector.MemberID)
				if err != nil {
					return err
				}
				h.Compatible = h.Compatible && compatible
			}
		}
	}
	if p.Path == "attempts/"+h.Selector.AttemptID+".json" && raw != nil && h.FirstAttemptSeq == "" {
		a, err := snapshot.DecodeAttempt(raw)
		if err != nil {
			return err
		}
		if a.Generation != h.Selector.Generation {
			return nil
		}
		h.FirstAttemptSeq, h.FirstAttemptSha256, h.FirstAttemptReceiptSha256 = rc.Seq, wire.Sum(raw), receipt
		h.FirstPolicySha256, h.FirstConfigSha256, h.FirstCapabilitySha256 = a.PolicySha256, a.ConfigSha256, a.CapabilityProfileSha256
		allocation := wire.Null()
		if a.PoolAllocation != nil {
			allocation = snapshot.PoolAllocationValue(a.PoolAllocation)
		}
		h.FirstAllocationSha256 = wire.Sum(wire.EncodeFile(allocation))
		if h.OriginalPolicy.Raw == nil || h.OriginalPolicy.Seq.Uint64() >= rc.Seq.Uint64() {
			h.Compatible = false
		}
	}
	return nil
}

func writerRecord(path string) bool {
	return path == "programs.json" || path == "pools.json" || path == "intent/queue.json" || path == "intent/policy.json" || strings.HasPrefix(path, "intent/tickets/") || strings.HasPrefix(path, "intent/releases/") || strings.HasPrefix(path, "attempts/")
}

func (r Reader) postBytes(p snapshot.PostEntry) ([]byte, error) {
	if p.Sha256 == nil {
		return nil, nil
	}
	bound, err := snapshot.PostBound(p.Path)
	if err != nil {
		return nil, err
	}
	var raw []byte
	if p.Record != nil {
		raw = wire.EncodeFile(*p.Record)
	} else {
		raw, err = requiredRead(r.Source, "evidence/"+string(*p.BlobSha256), bound)
		if err != nil {
			return nil, err
		}
	}
	if len(raw) > bound {
		return nil, wire.Errorf(wire.CodeLimitExceeded, p.Path, "post bytes exceed destination bound")
	}
	if wire.Sum(raw) != *p.Sha256 {
		return nil, wire.Errorf(wire.CodeJournalForked, p.Path, "consumed post/blob digest differs")
	}
	if err := snapshot.ContentPath(p.Path, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// decodedPost carries what validateRecord decoded from a ticket or request
// post, so step and the note binding reuse it instead of decoding the same
// bytes again (V1-1701).
type decodedPost struct {
	ticket   *ticket.Record
	request  *snapshot.Request
	relocate *snapshot.Relocate
}

// afterValidateRecord counts validateRecord calls in the V1-1701 parse-once
// witness; it is nil outside tests and may be called from prepare goroutines.
var afterValidateRecord func()

func (r Reader) validateRecord(p string, raw []byte, rc *snapshot.Receipt) (bool, *snapshot.Init, decodedPost, error) {
	if afterValidateRecord != nil {
		afterValidateRecord()
	}
	if p == "VERSION" {
		if string(raw) != snapshot.VersionBytes {
			return false, nil, decodedPost{}, wire.Errorf(wire.CodeUnsupportedVersion, p, "VERSION bytes differ")
		}
		if rc.Seq != "1" {
			return false, nil, decodedPost{}, wire.Errorf(wire.CodeUnsupported, p, "VERSION changes unsupported")
		}
		return true, nil, decodedPost{}, nil
	}
	if strings.HasPrefix(p, "evidence/") {
		return isOperatorNoteEvent(p, raw), nil, decodedPost{}, nil
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return false, nil, decodedPost{}, err
	}
	if v.Kind != wire.KindObject {
		return false, nil, decodedPost{}, wire.Errorf(wire.CodeMalformed, p, "post JSON must be an object")
	}
	if err := checkScope(v, r.QueueID); err != nil {
		return false, nil, decodedPost{}, err
	}
	switch {
	case p == "intent/queue.json":
		q, err := intent.DecodeQueue(raw)
		if err != nil {
			return false, nil, decodedPost{}, err
		}
		if q.QueueID != r.QueueID {
			return false, nil, decodedPost{}, wire.Errorf(wire.CodeJournalForked, p, "queue identity differs")
		}
		return true, nil, decodedPost{}, nil
	case p == "intent/policy.json":
		_, err := intent.DecodePolicy(raw)
		return true, nil, decodedPost{}, err
	case p == "intent/import-map.json":
		_, err := intent.DecodeImportMap(raw)
		return true, nil, decodedPost{}, err
	case strings.HasPrefix(p, "intent/tickets/"):
		// ticket.Decode's size bound, then its validation of the value
		// parsed above, so the walk parses each ticket post once (V1-1701).
		if len(raw) > wire.MaxTicketFileBytes {
			return false, nil, decodedPost{}, wire.Errorf(wire.CodeLimitExceeded, "/", "ticket record file larger than %d bytes", wire.MaxTicketFileBytes)
		}
		t, err := ticket.FromValue(v)
		if err != nil {
			return false, nil, decodedPost{}, err
		}
		if p != "intent/tickets/"+t.TicketID.Local+".json" {
			return false, nil, decodedPost{}, wire.Errorf(wire.CodeJournalForked, p, "ticket filename identity differs")
		}
		return true, nil, decodedPost{ticket: t}, nil
	case strings.HasPrefix(p, "intent/releases/"):
		x, err := release.Decode(raw)
		if err != nil {
			return false, nil, decodedPost{}, err
		}
		if p != "intent/releases/"+x.ReleaseID+".json" {
			return false, nil, decodedPost{}, wire.Errorf(wire.CodeJournalForked, p, "release filename identity differs")
		}
		return true, nil, decodedPost{}, nil
	case p == "barrier.json":
		b, err := snapshot.DecodeBarrier(raw)
		if err != nil {
			return false, nil, decodedPost{}, err
		}
		if b.SinceSeq != rc.Seq {
			return false, nil, decodedPost{}, wire.Errorf(wire.CodeJournalForked, p, "barrier sinceSeq differs")
		}
		if rc.Kind != "PAUSE" && rc.Kind != "DRAIN" && rc.Kind != "RESTORE" {
			return false, nil, decodedPost{}, wire.Errorf(wire.CodeMalformed, p, "barrier post requires barrier receipt kind")
		}
		return true, nil, decodedPost{}, nil
	case strings.HasPrefix(p, "requests/"):
		// snapshot.DecodeRequest over the value parsed above (V1-1701).
		if len(raw) > wire.MaxAttemptRecordBytes {
			return false, nil, decodedPost{}, wire.Errorf(wire.CodeLimitExceeded, "/", "request entry too large")
		}
		req, err := snapshot.RequestFromValue(v)
		if err != nil {
			return false, nil, decodedPost{}, err
		}
		return true, nil, decodedPost{request: req}, nil
	case p == "programs.json":
		_, err := snapshot.DecodePrograms(raw)
		return true, nil, decodedPost{}, err
	case p == "pools.json":
		_, err := snapshot.DecodePools(raw)
		if err != nil {
			return false, nil, decodedPost{}, err
		}
		return true, nil, decodedPost{}, nil
	case p == "reservations.json":
		rd := wire.NewReader(v, p)
		if err := wire.ProfileVersion(p, v, "taskman-reservation-set/0"); err != nil {
			return false, nil, decodedPost{}, err
		}
		rd.Closed("profile", "queueId", "entries")
		if err := rd.Err(); err != nil {
			return false, nil, decodedPost{}, err
		}
		if err := wire.CheckProfile(p, rd.Field("profile").String(), "taskman-reservation-set/0"); err != nil {
			return false, nil, decodedPost{}, err
		}
		rd.Field("queueId").QueueID()
		entries := rd.Field("entries").Array(wire.MaxActiveAttempts, true)
		if err := rd.Err(); err != nil {
			return false, nil, decodedPost{}, err
		}
		if rc.Seq == "1" && len(entries) != 0 {
			return false, nil, decodedPost{}, wire.Errorf(wire.CodeJournalForked, p, "genesis reservations must be empty")
		}
		for _, e := range entries {
			if err := checkScope(e.Value(), r.QueueID); err != nil {
				return false, nil, decodedPost{}, err
			}
		}
		return len(entries) == 0, nil, decodedPost{}, nil
	}
	profile, _ := v.Obj.Get("profile")
	if profile.Kind == wire.KindString && strings.HasPrefix(profile.Str, "taskman-init/") {
		if !strings.HasPrefix(p, "pinned/") {
			return false, nil, decodedPost{}, wire.Errorf(wire.CodeMalformed, p, "INIT descriptor requires pinned path")
		}
		d, err := snapshot.DecodeInit(raw)
		return true, d, decodedPost{}, err
	}
	if profile.Kind == wire.KindString && strings.HasPrefix(profile.Str, "taskman-relocate/") {
		if !strings.HasPrefix(p, "pinned/") {
			return false, nil, decodedPost{}, wire.Errorf(wire.CodeMalformed, p, "RELOCATE descriptor requires pinned path")
		}
		d, err := snapshot.DecodeRelocate(raw)
		if err != nil {
			return false, nil, decodedPost{}, err
		}
		return true, nil, decodedPost{relocate: d}, nil
	}
	// Later attempt/effect/pinned profiles lack full J1 semantic decoders.
	// Recognized scope and path identity fields still cannot conflict.
	if strings.HasPrefix(p, "attempts/") {
		q, err := snapshot.AttemptQueue(strings.TrimSuffix(strings.TrimPrefix(p, "attempts/"), ".json"))
		if err != nil {
			return false, nil, decodedPost{}, err
		}
		if q != r.QueueID {
			return false, nil, decodedPost{}, wire.Errorf(wire.CodeJournalForked, p, "attempt filename scope differs")
		}
		id, ok := v.Obj.Get("attemptId")
		if ok && (id.Kind != wire.KindString || p != "attempts/"+id.Str+".json") {
			return false, nil, decodedPost{}, wire.Errorf(wire.CodeJournalForked, p, "attempt path identity differs")
		}
	}
	if strings.HasPrefix(p, "effects/") {
		key, ok := v.Obj.Get("key")
		if ok && (key.Kind != wire.KindString || p != "effects/"+key.Str+".json") {
			return false, nil, decodedPost{}, wire.Errorf(wire.CodeJournalForked, p, "effect path identity differs")
		}
	}
	if profile.Kind != wire.KindString {
		return false, nil, decodedPost{}, wire.Errorf(wire.CodeMalformed, p, "later record requires an explicit profile")
	}
	if _, err := wire.ParseIdentifier(p, profile.Str); err != nil {
		return false, nil, decodedPost{}, err
	}
	return false, nil, decodedPost{}, nil
}

func checkScope(v wire.Value, q wire.QueueID) error {
	if v.Kind != wire.KindObject {
		return wire.Errorf(wire.CodeMalformed, "/", "record must be an object")
	}
	if id, ok := v.Obj.Get("queueId"); ok {
		if id.Kind != wire.KindString || id.Str != q.Raw {
			return wire.Errorf(wire.CodeJournalForked, "/queueId", "record queue differs")
		}
	}
	if id, ok := v.Obj.Get("ticketId"); ok && id.Kind != wire.KindNull {
		t, err := wire.ParseTicketID("/ticketId", id.Str)
		if err != nil {
			return err
		}
		if t.QueueID() != q.Raw {
			return wire.Errorf(wire.CodeJournalForked, "/ticketId", "record ticket differs from selected queue")
		}
	}
	if id, ok := v.Obj.Get("attemptId"); ok && id.Kind != wire.KindNull {
		a, err := snapshot.AttemptQueue(id.Str)
		if err != nil {
			return err
		}
		if a != q {
			return wire.Errorf(wire.CodeJournalForked, "/attemptId", "record attempt differs from selected queue")
		}
	}
	return nil
}

// sortedPaths returns the keys of m in ascending byte order. Every walk that
// can refuse uses it, so with several faults the refusal names the
// byte-smallest faulty path and its code on every run (CAL-V0-114).
func sortedPaths[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}

func (r Reader) projections(o *observation, canonical map[string]latest, checkIntent bool) error {
	for i, p := range sortedPaths(canonical) {
		if i%abortEvery == abortEvery-1 && r.headMoved(o) {
			return moved("head.json")
		}
		record := canonical[p]
		if r.intentOnly && !strings.HasPrefix(p, "intent/") {
			continue
		}
		if r.unpauseTickets && strings.HasPrefix(p, "intent/tickets/") {
			info, present := o.files[p]
			if !present || !info.Mode().IsRegular() {
				return wire.Errorf(wire.CodeIntentDiverged, p, "canonical ticket lacks a regular physical projection")
			}
			continue
		}
		if p == r.divergentIntent {
			continue
		}
		if !checkIntent && strings.HasPrefix(p, "intent/") {
			continue
		}
		digest, _, err := r.projected(o, p, false)
		if err != nil {
			return err
		}
		if equalDigest(digest, record.digest) {
			continue
		}
		if record.pending && equalDigest(digest, record.pendingPre) {
			continue
		}
		code := wire.CodeJournalForked
		if strings.HasPrefix(p, "intent/") {
			code = wire.CodeIntentDiverged
		}
		return wire.Errorf(code, p, "projection differs from latest canonical afterimage")
	}
	return r.strays(o, inMap(canonical), checkIntent)
}

// projected observes one physical projection. Intent bytes were already read
// and digested by the capture this audit is bound to, so they are not read a
// second time unless the caller needs the bytes themselves.
func (r Reader) projected(o *observation, p string, needRaw bool) (*wire.Digest, []byte, error) {
	if raw, ok := o.intentRaw[p]; ok {
		// A writer capture kept the bytes it digested (CAL-V0-189, proposed).
		d := o.intentDigests[p]
		return &d, raw, nil
	}
	if strings.HasPrefix(p, "intent/") && !needRaw {
		if d, ok := o.intentDigests[p]; ok {
			return &d, nil, nil
		}
		return nil, nil, nil
	}
	bound, err := snapshot.PostBound(p)
	if err != nil {
		return nil, nil, err
	}
	raw, present, err := optionalRead(r.Source, p, bound)
	if err != nil || !present {
		return nil, nil, err
	}
	if raw == nil {
		raw = []byte{}
	}
	d := wire.Sum(raw)
	return &d, raw, nil
}

// strays refuses every listed projection the journal never posted.
func (r Reader) strays(o *observation, has func(string) bool, checkIntent bool) error {
	for _, p := range sortedPaths(o.files) {
		info := o.files[p]
		if r.intentOnly && !strings.HasPrefix(p, "intent/") {
			continue
		}
		if !checkIntent && strings.HasPrefix(p, "intent/") {
			continue
		}
		if info.IsDir() || isTemp(p) || strings.HasPrefix(p, "staging/") || strings.HasPrefix(p, snapshot.SegmentsDir+"/") {
			continue
		}
		// A sealed request is served from its segment; a linked copy of it
		// is a second afterimage the journal never posted (JSG-V0-005).
		if v, ok := r.Source.(*segmentView); ok && v.sealed(p) {
			return wire.Errorf(wire.CodeJournalForked, p, "physical duplicate of a sealed request")
		}
		if has(p) {
			continue
		}
		// A request a retired receipt posted is the seal's, not a stray
		// (JRT-V0-013); walkSealed matched these names to the anchor.
		if o.seal != nil && strings.HasPrefix(p, "requests/") && o.seal.Anchor.HasRequest(p) {
			continue
		}
		if p == "head.json" || p == RetiredPrefixFile || strings.HasPrefix(p, "receipts/") || strings.HasPrefix(p, "evidence/") || strings.HasPrefix(p, "pinned/") || strings.HasSuffix(p, ".boot") || strings.HasSuffix(p, ".ack") {
			continue
		}
		code := wire.CodeJournalForked
		if strings.HasPrefix(p, "intent/") {
			code = wire.CodeIntentDiverged
		}
		return wire.Errorf(code, p, "projection has no retained journal afterimage")
	}
	return nil
}

// Revision fields are assertions until they equal the actual target afterimage.
// Physical redo preconditions are deliberately not canonical predecessors.
func bindRevisions(rc *snapshot.Receipt, req *snapshot.Request, target *ticket.Record) error {
	if req == nil || req.Entry.Outcome.Outcome != "COMPLETED" {
		return nil
	}
	out := req.Entry.Outcome
	if rc.TicketID == nil {
		if out.ResultingRevision != nil || out.ResultingAcceptanceRevision != nil {
			return wire.Errorf(wire.CodeJournalForked, "/outcome", "no-ticket success must not claim ticket revisions")
		}
		return nil
	}
	if target == nil || target.TicketID.Raw != rc.TicketID.Raw || out.ResultingRevision == nil || out.ResultingAcceptanceRevision == nil {
		return wire.Errorf(wire.CodeJournalForked, "/outcome", "ticket success requires target afterimage and both revisions")
	}
	if *out.ResultingRevision != target.Revision || *out.ResultingAcceptanceRevision != target.AcceptanceRevision {
		return wire.Errorf(wire.CodeJournalForked, "/outcome", "claimed revisions differ from target afterimage")
	}
	return nil
}

// errCheckpoint marks a checkpoint that cannot serve this observation. It is
// never returned to a caller: the complete audit runs instead.
func errCheckpoint(where, why string) error {
	return wire.Errorf(wire.CodeUnsupported, where, "checkpoint unusable: %s", why)
}

// inFlight marks a checkpoint refusal that a writer between its receipt
// link-in and its head rename causes: the head is moving, so the resumed read
// is retried within the CAL-V0-061 movement bound before the complete audit
// classifies what remains. Its code is the wrapped refusal's.
type inFlight struct{ error }

func (e inFlight) Unwrap() error { return e.error }

// walkTail resumes the chain at a checkpoint (CAL-V0-059..061). It rebinds
// the checkpoint to the retained receipt it names, validates every later
// receipt exactly as walk does, and compares every non-request latest
// afterimage with its projection. Receipts before the checkpoint, their
// request afterimages and evidence blobs are not re-read; Mode says so.
func (r Reader) walkTail(o *observation, cp *Checkpoint, selected map[string]bool, lim limits) (*Result, error) {
	result := &Result{Identity: o.identity, Head: o.head, Mode: ModeCheckpoint, Records: map[string]Record{}, StructuralConsistency: "NOT_OBSERVED", ProjectionAgreement: "NOT_OBSERVED", SemanticCoverage: "NOT_OBSERVED", HistoricalAcceptance: "NOT_OBSERVED", ActorAuthentication: "NOT_OBSERVED", Liveness: "NOT_OBSERVED", RuntimeQualification: "NOT_OBSERVED"}
	if o.seal != nil {
		return result, errCheckpoint(RetiredPrefixFile, "store carries a retired prefix")
	}
	if o.segErr != nil {
		return result, o.segErr
	}
	if o.head == nil || o.stageErr != nil || o.staging || len(o.stageDigests) > 0 {
		if o.head != nil {
			// A stage beside a head is a writer in flight (or one that
			// crashed there): retried like a moved head, then classified by
			// the complete audit (CAL-V0-061, V1-1060).
			return result, inFlight{errCheckpoint("/", "journal is not plainly settled")}
		}
		return result, errCheckpoint("/", "journal is not plainly settled")
	}
	head := o.head
	headSeq, from := head.LastSeq.Uint64(), cp.Seq.Uint64()
	if head.QueueID != r.QueueID || head.PrimaryWorktree != r.PrimaryWorktree || cp.QueueID != r.QueueID || cp.PrimaryWorktree != r.PrimaryWorktree || cp.InitSha256 != head.InitSha256 {
		return result, errCheckpoint("/queueId", "identity differs from head")
	}
	if head.VersionSha256 != wire.Sum([]byte(snapshot.VersionBytes)) {
		return result, errCheckpoint("head.json", "head version differs")
	}
	if from > headSeq || cp.Generation.Uint64() > head.Generation.Uint64() || len(cp.Entries) > lim.scan+intent.MaxIntentRootEntries+wire.MaxTicketsPerQueue+wire.MaxReleasesPerQueue {
		return result, errCheckpoint("/seq", "checkpoint is not a prefix of head")
	}
	name, err := snapshot.ReceiptName(from)
	if err != nil {
		return result, err
	}
	raw, err := requiredRead(r.Source, "receipts/"+name, wire.MaxReceiptFileBytes)
	if err != nil {
		return result, err
	}
	if wire.Sum(raw) != cp.ReceiptSha256 {
		return result, errCheckpoint(name, "retained receipt differs")
	}
	if rc, err := snapshot.DecodeReceipt(raw); err != nil || rc.Seq.Uint64() != from || rc.HeadGeneration != cp.Generation {
		return result, errCheckpoint(name, "retained receipt sequence/generation differs")
	}
	if from == headSeq && (*head.LastReceiptSha256 != cp.ReceiptSha256 || head.Generation != cp.Generation) {
		return result, errCheckpoint("head.json", "head digest/generation differs")
	}
	// A linked receipt beyond head is pending or forked; the complete audit
	// classifies it.
	next, err := snapshot.ReceiptName(headSeq + 1)
	if err != nil {
		return result, err
	}
	if _, present, err := optionalRead(r.Source, "receipts/"+next, wire.MaxReceiptFileBytes); err != nil || present {
		if err != nil {
			return result, err
		}
		return result, inFlight{errCheckpoint(next, "receipt beyond head")}
	}
	if cp.Requests == nil {
		// Without the filter a null request pre-state is unprovable; the
		// next writer retains a checkpoint that carries one (CAL-V0-321).
		return result, errCheckpoint("/requests", "checkpoint carries no request filter")
	}
	st := &chain{canonical: make(map[string]latest, len(cp.Entries)), prev: &cp.ReceiptSha256, generation: cp.Generation.Uint64(), resumed: from, resumedSha: cp.ReceiptSha256, primary: cp.PrimaryWorktree, requests: cp.Requests}
	for _, e := range cp.Entries {
		st.canonical[e.Path] = latest{seq: e.Seq, digest: e.Sha256, bytes: -1}
	}
	result.SemanticCoverage = cp.SemanticCoverage
	result.LastSeq, result.LastReceiptSha256 = cp.Seq, cp.ReceiptSha256
	for seq := from + 1; seq <= headSeq; seq++ {
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			return result, err
		}
		if err := r.step(o, st, result, name, seq, headSeq, selected, "", lim); err != nil {
			return result, err
		}
	}
	result.StructuralConsistency = ModeCheckpoint
	for _, p := range sortedPaths(st.canonical) {
		record := st.canonical[p]
		_, have := result.Records[p]
		need := !have && r.selects(p, selected)
		digest, raw, err := r.projected(o, p, need)
		if err != nil {
			return result, err
		}
		if !equalDigest(digest, record.digest) {
			code := wire.CodeJournalForked
			if strings.HasPrefix(p, "intent/") {
				code = wire.CodeIntentDiverged
			}
			return result, wire.Errorf(code, p, "projection differs from latest canonical afterimage")
		}
		if need {
			if len(raw) > lim.selected-st.selectedBytes {
				return result, wire.Errorf(wire.CodeLimitExceeded, p, "selected canonical bytes exceed aggregate live intent-state budget; select fewer paths")
			}
			st.selectedBytes += len(raw)
			result.Records[p] = Record{Seq: record.seq, Sha256: record.digest, Raw: raw}
		}
	}
	if err := r.strays(o, inMap(st.canonical), true); err != nil {
		return result, err
	}
	result.ProjectionAgreement = "AGREES"
	return result, nil
}
