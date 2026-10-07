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
}

// chain is the state one receipt hands to the next. A complete audit builds
// it from receipt 1; a checkpoint read resumes it at the checkpoint sequence.
type chain struct {
	canonical     map[string]latest
	prev          *wire.Digest
	generation    uint64
	init          *snapshot.Init
	genesisQueue  []byte
	selectedBytes int
	notes         *noteState
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
	if r.handoffPolicy != nil {
		result.HandoffPolicy = &HandoffPolicyHistory{Selector: *r.handoffPolicy, Compatible: true}
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
	headSeq := uint64(0)
	if o.head != nil {
		headSeq = o.head.LastSeq.Uint64()
		if o.head.QueueID != r.QueueID || o.head.PrimaryWorktree != r.PrimaryWorktree {
			return result, wire.Errorf(wire.CodeJournalForked, "head.json", "selected queue/primary differs from head; relocation unsupported")
		}
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
	if result.Pending && (r.divergentIntent != "" || r.unpauseTickets) {
		return result, wire.Errorf(wire.CodeRedoPending, "receipts", "this observation requires a settled journal")
	}
	st := &chain{canonical: map[string]latest{}}
	for i, name := range o.receipts {
		if err := r.step(o, st, result, name, uint64(i+1), headSeq, selected, request, lim); err != nil {
			return result, err
		}
	}
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
		result.ProjectionAgreement = "PRE_OR_POST"
		return result, wire.Errorf(wire.CodeRedoPending, "receipts", "one fully validated linked receipt awaits head projection; no redo performed")
	}
	return result, nil
}

// step validates one receipt against the chain and folds its posts in.
func (r Reader) step(o *observation, st *chain, result *Result, name string, seq, headSeq uint64, selected map[string]bool, request string, lim limits) error {
	canonical := st.canonical
	raw, err := requiredRead(r.Source, "receipts/"+name, wire.MaxReceiptFileBytes)
	if err != nil {
		return err
	}
	rc, err := snapshot.DecodeReceipt(raw)
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
	var boundRequest *snapshot.Request
	var target *ticket.Record
	notes := r.noteAudit(st, rc)
	for j, p := range rc.Post {
		prior := canonical[p.Path]
		if strings.HasPrefix(p.Path, "requests/") && prior.seq != "" {
			return wire.Errorf(wire.CodeJournalForked, p.Path, "request ID occurs more than once in retained history")
		}
		if _, ok := canonical[p.Path]; !ok && len(canonical) >= lim.scan+intent.MaxIntentRootEntries+wire.MaxTicketsPerQueue+wire.MaxReleasesPerQueue {
			return wire.Errorf(wire.CodeLimitExceeded, p.Path, "latest metadata exceeds store scan bound")
		}
		post, err := r.postBytes(p)
		if err != nil {
			return err
		}
		notes.observe(j, p, prior, post)
		if post != nil {
			coverage, descriptor, err := r.validateRecord(p.Path, post, rc)
			if err != nil {
				return err
			}
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
			target, err = ticket.Decode(post)
			if err != nil {
				return err
			}
		}
		if seq == 1 {
			if rc.Pre[j].Sha256 != nil || p.Sha256 == nil || strings.HasPrefix(p.Path, "attempts/") || strings.HasPrefix(p.Path, "effects/") {
				return wire.Errorf(wire.CodeJournalForked, p.Path, "genesis cannot update/delete existing state or post attempts/effects")
			}
			requiredGenesis[p.Path] = true
		}
		if strings.HasPrefix(p.Path, "requests/") {
			req, err := snapshot.DecodeRequest(post)
			if err != nil {
				return err
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
		canonical[p.Path] = latest{seq: rc.Seq, digest: p.Sha256, pendingPre: rc.Pre[j].Sha256, pending: seq > headSeq}
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
		if len(rc.Post) != 5+requestCount || descriptorCount != 1 || !requiredGenesis["VERSION"] || !requiredGenesis["intent/queue.json"] || !requiredGenesis["intent/policy.json"] || !requiredGenesis["reservations.json"] {
			return wire.Errorf(wire.CodeJournalForked, name, "genesis requires VERSION blob, queue, policy, empty reservations and unique INIT descriptor")
		}
		if st.init.VersionSha256 != wire.Sum([]byte(snapshot.VersionBytes)) || st.init.QueueID != r.QueueID || st.init.PrimaryWorktree != r.PrimaryWorktree {
			return wire.Errorf(wire.CodeJournalForked, name, "genesis identity/version mismatch")
		}
		if o.head != nil && (o.head.VersionSha256 != st.init.VersionSha256 || o.head.PrimaryWorktree != st.init.PrimaryWorktree || o.head.QueueID != st.init.QueueID) {
			return wire.Errorf(wire.CodeJournalForked, "head.json", "head and INIT descriptor disagree")
		}
	}
	st.prev = &digest
	st.generation = rc.HeadGeneration.Uint64()
	result.LastSeq = rc.Seq
	result.LastReceiptSha256 = digest
	return nil
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

func (r Reader) validateRecord(p string, raw []byte, rc *snapshot.Receipt) (bool, *snapshot.Init, error) {
	if p == "VERSION" {
		if string(raw) != snapshot.VersionBytes {
			return false, nil, wire.Errorf(wire.CodeUnsupportedVersion, p, "VERSION bytes differ")
		}
		if rc.Seq != "1" {
			return false, nil, wire.Errorf(wire.CodeUnsupported, p, "VERSION changes unsupported")
		}
		return true, nil, nil
	}
	if strings.HasPrefix(p, "evidence/") {
		return isOperatorNoteEvent(p, raw), nil, nil
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return false, nil, err
	}
	if v.Kind != wire.KindObject {
		return false, nil, wire.Errorf(wire.CodeMalformed, p, "post JSON must be an object")
	}
	if err := checkScope(v, r.QueueID); err != nil {
		return false, nil, err
	}
	switch {
	case p == "intent/queue.json":
		q, err := intent.DecodeQueue(raw)
		if err != nil {
			return false, nil, err
		}
		if q.QueueID != r.QueueID {
			return false, nil, wire.Errorf(wire.CodeJournalForked, p, "queue identity differs")
		}
		return true, nil, nil
	case p == "intent/policy.json":
		_, err := intent.DecodePolicy(raw)
		return true, nil, err
	case p == "intent/import-map.json":
		_, err := intent.DecodeImportMap(raw)
		return true, nil, err
	case strings.HasPrefix(p, "intent/tickets/"):
		t, err := ticket.Decode(raw)
		if err != nil {
			return false, nil, err
		}
		if p != "intent/tickets/"+t.TicketID.Local+".json" {
			return false, nil, wire.Errorf(wire.CodeJournalForked, p, "ticket filename identity differs")
		}
		return true, nil, nil
	case strings.HasPrefix(p, "intent/releases/"):
		x, err := release.Decode(raw)
		if err != nil {
			return false, nil, err
		}
		if p != "intent/releases/"+x.ReleaseID+".json" {
			return false, nil, wire.Errorf(wire.CodeJournalForked, p, "release filename identity differs")
		}
		return true, nil, nil
	case p == "barrier.json":
		b, err := snapshot.DecodeBarrier(raw)
		if err != nil {
			return false, nil, err
		}
		if b.SinceSeq != rc.Seq {
			return false, nil, wire.Errorf(wire.CodeJournalForked, p, "barrier sinceSeq differs")
		}
		if rc.Kind != "PAUSE" && rc.Kind != "DRAIN" && rc.Kind != "RESTORE" {
			return false, nil, wire.Errorf(wire.CodeMalformed, p, "barrier post requires barrier receipt kind")
		}
		return true, nil, nil
	case strings.HasPrefix(p, "requests/"):
		_, err := snapshot.DecodeRequest(raw)
		return true, nil, err
	case p == "programs.json":
		_, err := snapshot.DecodePrograms(raw)
		return true, nil, err
	case p == "pools.json":
		_, err := snapshot.DecodePools(raw)
		if err != nil {
			return false, nil, err
		}
		return true, nil, nil
	case p == "reservations.json":
		rd := wire.NewReader(v, p)
		if err := wire.ProfileVersion(p, v, "taskman-reservation-set/0"); err != nil {
			return false, nil, err
		}
		rd.Closed("profile", "queueId", "entries")
		if err := rd.Err(); err != nil {
			return false, nil, err
		}
		if err := wire.CheckProfile(p, rd.Field("profile").String(), "taskman-reservation-set/0"); err != nil {
			return false, nil, err
		}
		rd.Field("queueId").QueueID()
		entries := rd.Field("entries").Array(wire.MaxActiveAttempts, true)
		if err := rd.Err(); err != nil {
			return false, nil, err
		}
		if rc.Seq == "1" && len(entries) != 0 {
			return false, nil, wire.Errorf(wire.CodeJournalForked, p, "genesis reservations must be empty")
		}
		for _, e := range entries {
			if err := checkScope(e.Value(), r.QueueID); err != nil {
				return false, nil, err
			}
		}
		return len(entries) == 0, nil, nil
	}
	profile, _ := v.Obj.Get("profile")
	if profile.Kind == wire.KindString && strings.HasPrefix(profile.Str, "taskman-init/") {
		if !strings.HasPrefix(p, "pinned/") {
			return false, nil, wire.Errorf(wire.CodeMalformed, p, "INIT descriptor requires pinned path")
		}
		d, err := snapshot.DecodeInit(raw)
		return true, d, err
	}
	// Later attempt/effect/pinned profiles lack full J1 semantic decoders.
	// Recognized scope and path identity fields still cannot conflict.
	if strings.HasPrefix(p, "attempts/") {
		q, err := snapshot.AttemptQueue(strings.TrimSuffix(strings.TrimPrefix(p, "attempts/"), ".json"))
		if err != nil {
			return false, nil, err
		}
		if q != r.QueueID {
			return false, nil, wire.Errorf(wire.CodeJournalForked, p, "attempt filename scope differs")
		}
		id, ok := v.Obj.Get("attemptId")
		if ok && (id.Kind != wire.KindString || p != "attempts/"+id.Str+".json") {
			return false, nil, wire.Errorf(wire.CodeJournalForked, p, "attempt path identity differs")
		}
	}
	if strings.HasPrefix(p, "effects/") {
		key, ok := v.Obj.Get("key")
		if ok && (key.Kind != wire.KindString || p != "effects/"+key.Str+".json") {
			return false, nil, wire.Errorf(wire.CodeJournalForked, p, "effect path identity differs")
		}
	}
	if profile.Kind != wire.KindString {
		return false, nil, wire.Errorf(wire.CodeMalformed, p, "later record requires an explicit profile")
	}
	if _, err := wire.ParseIdentifier(p, profile.Str); err != nil {
		return false, nil, err
	}
	return false, nil, nil
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
	for _, p := range sortedPaths(canonical) {
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
	return r.strays(o, canonical, checkIntent)
}

// projected observes one physical projection. Intent bytes were already read
// and digested by the capture this audit is bound to, so they are not read a
// second time unless the caller needs the bytes themselves.
func (r Reader) projected(o *observation, p string, needRaw bool) (*wire.Digest, []byte, error) {
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
func (r Reader) strays(o *observation, canonical map[string]latest, checkIntent bool) error {
	for _, p := range sortedPaths(o.files) {
		info := o.files[p]
		if r.intentOnly && !strings.HasPrefix(p, "intent/") {
			continue
		}
		if !checkIntent && strings.HasPrefix(p, "intent/") {
			continue
		}
		if info.IsDir() || isTemp(p) || strings.HasPrefix(p, "staging/") {
			continue
		}
		if _, ok := canonical[p]; ok {
			continue
		}
		if p == "head.json" || strings.HasPrefix(p, "receipts/") || strings.HasPrefix(p, "evidence/") || strings.HasPrefix(p, "pinned/") || strings.HasSuffix(p, ".boot") || strings.HasSuffix(p, ".ack") {
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

// walkTail resumes the chain at a checkpoint (CAL-V0-059..061). It rebinds
// the checkpoint to the retained receipt it names, validates every later
// receipt exactly as walk does, and compares every non-request latest
// afterimage with its projection. Receipts before the checkpoint, their
// request afterimages and evidence blobs are not re-read; Mode says so.
func (r Reader) walkTail(o *observation, cp *Checkpoint, selected map[string]bool, lim limits) (*Result, error) {
	result := &Result{Identity: o.identity, Head: o.head, Mode: ModeCheckpoint, Records: map[string]Record{}, StructuralConsistency: "NOT_OBSERVED", ProjectionAgreement: "NOT_OBSERVED", SemanticCoverage: "NOT_OBSERVED", HistoricalAcceptance: "NOT_OBSERVED", ActorAuthentication: "NOT_OBSERVED", Liveness: "NOT_OBSERVED", RuntimeQualification: "NOT_OBSERVED"}
	if o.head == nil || o.stageErr != nil || o.staging || len(o.stageDigests) > 0 {
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
		return result, errCheckpoint(next, "receipt beyond head")
	}
	st := &chain{canonical: make(map[string]latest, len(cp.Entries)), prev: &cp.ReceiptSha256, generation: cp.Generation.Uint64()}
	for _, e := range cp.Entries {
		st.canonical[e.Path] = latest{seq: e.Seq, digest: e.Sha256}
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
	if err := r.strays(o, st.canonical, true); err != nil {
		return result, err
	}
	result.ProjectionAgreement = "AGREES"
	return result, nil
}
