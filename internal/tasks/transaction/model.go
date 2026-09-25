// Package transaction computes hypothetical fixture transactions and capacity.
// It performs no I/O. Supplied inventories and actors are claims, never physical
// observations, authentication, durability, admission or execution permission.
package transaction

import (
	"bytes"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/release"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const (
	Init        = snapshot.StageInit
	Pause       = snapshot.StagePause
	Unpause     = snapshot.StageUnpause
	KeepJournal = snapshot.StageKeepJournal
	AdoptFile   = snapshot.StageAdoptFile
	// Mutate carries one §3.3 ticket mutation envelope. One operation serves
	// every mutation verb because mutation.Apply already validates and
	// computes the post record for all of them; the staged artifact shape is
	// the same for each, so a per-verb operation would multiply the closed
	// tables without adding a check.
	Mutate      = snapshot.StageMutate
	Release     = snapshot.StageRelease
	NotObserved = "NOT_OBSERVED"
	// FixtureNoRuntime is an explicit hypothetical premise, not an observation.
	FixtureNoRuntime = "HYPOTHETICAL_FIXTURE_NO_RUNTIME"
	// LocalOperator is the observed premise of a real local store driven by the
	// operator who started the process (decision 0003). It states what the
	// caller observed about the runtime; it grants nothing and never makes a
	// Coverage axis observed.
	LocalOperator = "OBSERVED_LOCAL_OPERATOR"
)

// PolicyUpdate replaces intent/policy.json with the next policyVersion
// (ATM-V0-027, TM-V0-030, decision 0010). Request.Policy carries the new
// canonical bytes; it posts nothing else.
const PolicyUpdate = snapshot.StagePolicyUpdate

type Coverage struct {
	ActorAuthentication, AdministrativeAuthorization       string
	InventoryObservation, Durability, RuntimeQualification string
}

func coverage() Coverage {
	return Coverage{NotObserved, NotObserved, NotObserved, NotObserved, NotObserved}
}

// Request has operation-specific closed fields. Inapplicable fields must be empty.
// File is the original offered/discarded bytes, including a present empty file.
// CanonicalSha256 binds KEEP's original choice, not a refreshed projection.
type Request struct {
	Operation, QueueID, RequestID, TargetID string
	Actor                                   mutation.Binding
	PrimaryWorktree                         string
	Queue, Policy, File                     []byte
	CanonicalSha256                         wire.Digest
	// Envelope is the canonical taskman-mutation/0 bytes, set only for Mutate.
	// The request digest of a mutation is the digest of these bytes exactly
	// (TM-V0-006), so an identical retry replays and any other byte sequence
	// under the same requestId is REQUEST_ID_CONFLICT.
	Envelope []byte
	// ExpectedPolicyVersion is the policyVersion the caller read, set only for
	// PolicyUpdate.
	ExpectedPolicyVersion wire.Size
}

// ReplayObservation is mandatory; zero/unknown/error never means absence.
// FOUND carries the original canonical request-index bytes. The caller must
// eventually obtain a qualified fresh J1 observation; this model cannot do so.
type ReplayObservation struct {
	State  string
	Record []byte
}

// Input supplies complete metadata and canonical tickets independently of the
// possibly divergent physical projection. No callbacks or runtime facts enter.
type Input struct {
	Inventory                                  *Inventory
	Head, Queue, Policy, Barrier, Reservations []byte
	CanonicalTickets                           [][]byte
	CanonicalReleases                          [][]byte
	ReleaseCandidate                           *release.Candidate
	ReleaseGates                               []release.Gate
	ReleaseObservation                         release.Observation
	Premise, Branch                            string
	Replay                                     ReplayObservation
	RecordedAt                                 wire.Timestamp
}

type Result struct {
	Kind     string // Transaction, Replay, NoChange, Refused
	Outcome  mutation.Outcome
	Coverage Coverage
	Detail   string
	Plan     *Plan
}

// Plan is immutable; accessors return copies. Its bytes remain hypothetical.
type Plan struct {
	operation     string
	request       Request
	descriptor    []byte
	artifacts     []Artifact
	receipt, head []byte
	posts         map[string][]byte // nil means deletion; empty non-nil means present
	base          *Inventory
	baseHead      *snapshot.Head
}

func (p *Plan) Descriptor() []byte    { return bytes.Clone(p.descriptor) }
func (p *Plan) Receipt() []byte       { return bytes.Clone(p.receipt) }
func (p *Plan) Head() []byte          { return bytes.Clone(p.head) }
func (p *Plan) Artifacts() []Artifact { return cloneArtifacts(p.artifacts) }

func object(kv ...any) wire.Value {
	o := wire.NewObject()
	for i := 0; i < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1].(wire.Value))
	}
	return wire.ObjectValue(o)
}
func s(v string) wire.Value { return wire.String(v) }
func digestValue(v *wire.Digest) wire.Value {
	if v == nil {
		return wire.Null()
	}
	return s(string(*v))
}
func countValue(v *wire.Count) wire.Value {
	if v == nil {
		return wire.Null()
	}
	return s(string(*v))
}
func malformed(detail string) error {
	return wire.Errorf(wire.CodeMalformed, "transaction", "%s", detail)
}
func limit(detail string) error {
	return wire.Errorf(wire.CodeLimitExceeded, "transaction", "%s", detail)
}
func canonical(raw []byte) error {
	v, e := wire.Parse(raw)
	if e != nil {
		return e
	}
	if !bytes.Equal(raw, wire.EncodeFile(v)) {
		return malformed("noncanonical bytes")
	}
	return nil
}

// Digest computes the closed administrative preimage. ADOPT delegates its exact
// digest to the existing reducer; timestamps and head state never enter it.
func Digest(r Request) (wire.Digest, error) {
	if _, e := wire.ParseLabel("actor", r.Actor.ID); e != nil {
		return "", e
	}
	if r.Actor.Role != "OWNER" && r.Actor.Role != "OPERATOR" {
		return "", wire.Errorf(wire.CodeUnsupported, "actor", "outside hypothetical administrative role subset")
	}
	q, e := wire.ParseQueueID("queueId", r.QueueID)
	if e != nil {
		return "", e
	}
	if _, e = mutation.ParseRequestID("requestId", r.RequestID); e != nil {
		return "", e
	}
	fileLimit := wire.MaxTicketFileBytes
	if r.Operation == Release {
		fileLimit = wire.MaxReleaseFileBytes
	}
	if len(r.File) > fileLimit {
		return "", limit("offered file")
	}
	if r.Operation != Init && (len(r.Queue) != 0 || r.PrimaryWorktree != "") {
		return "", malformed("inapplicable INIT inputs")
	}
	if r.Operation != Init && r.Operation != PolicyUpdate && len(r.Policy) != 0 {
		return "", malformed("inapplicable policy input")
	}
	if r.Operation != PolicyUpdate && r.ExpectedPolicyVersion != "" {
		return "", malformed("inapplicable expected policy version")
	}
	if r.Operation != KeepJournal && r.Operation != Release && r.CanonicalSha256 != "" {
		return "", malformed("inapplicable canonical choice")
	}
	if r.Operation != KeepJournal && r.Operation != AdoptFile && r.Operation != Release && (r.TargetID != "" || r.File != nil) {
		return "", malformed("inapplicable ticket inputs")
	}
	if r.Operation != Mutate && r.Operation != Release && r.Envelope != nil {
		return "", malformed("inapplicable mutation envelope")
	}
	o := object("actor", object("id", s(r.Actor.ID), "role", s(r.Actor.Role)), "operation", s(r.Operation), "queueId", s(r.QueueID), "requestId", s(r.RequestID))
	switch r.Operation {
	case Init:
		if _, e = wire.ParsePathText("primary", r.PrimaryWorktree); e != nil {
			return "", e
		}
		queue, e := intent.DecodeQueue(r.Queue)
		if e != nil {
			return "", e
		}
		if queue.QueueID != q || !queue.Fixture {
			return "", malformed("INIT fixture identity")
		}
		if _, e = intent.DecodePolicy(r.Policy); e != nil {
			return "", e
		}
		if e = canonical(r.Queue); e != nil {
			return "", e
		}
		if e = canonical(r.Policy); e != nil {
			return "", e
		}
		o.Obj.Set("primaryWorktree", s(r.PrimaryWorktree))
		o.Obj.Set("queueSha256", s(string(wire.Sum(r.Queue))))
		o.Obj.Set("policySha256", s(string(wire.Sum(r.Policy))))
		o.Obj.Set("versionSha256", s(string(wire.Sum([]byte(snapshot.VersionBytes)))))
	case PolicyUpdate:
		if _, e = wire.ParseSize("expectedPolicyVersion", string(r.ExpectedPolicyVersion)); e != nil {
			return "", e
		}
		if _, e = intent.DecodePolicy(r.Policy); e != nil {
			return "", e
		}
		if e = canonical(r.Policy); e != nil {
			return "", e
		}
		o.Obj.Set("expectedPolicyVersion", s(string(r.ExpectedPolicyVersion)))
		o.Obj.Set("policySha256", s(string(wire.Sum(r.Policy))))
	case Pause:
		o.Obj.Set("reason", s("OPERATOR"))
		o.Obj.Set("scope", s("ADMISSION"))
	case Unpause:
	case Mutate:
		env, e := mutation.Decode(r.Envelope)
		if e != nil {
			return "", e
		}
		if e = canonical(r.Envelope); e != nil {
			return "", e
		}
		if env.QueueID != q {
			return "", malformed("envelope queue")
		}
		if env.RequestID != r.RequestID {
			return "", malformed("envelope requestId")
		}
		// The mutation's own canonical bytes are the preimage (TM-V0-006);
		// no timestamp and no current revision enters it.
		return env.Sha256(), nil
	case Release:
		if r.File != nil {
			if _, e = wire.ParseLabel("targetId", r.TargetID); e != nil {
				return "", e
			}
			if _, e = wire.ParseDigest("canonicalSha256", string(r.CanonicalSha256)); e != nil {
				return "", e
			}
			o.Obj.Set("targetId", s(r.TargetID))
			o.Obj.Set("fileSha256", s(string(wire.Sum(r.File))))
			o.Obj.Set("canonicalSha256", s(string(r.CanonicalSha256)))
			return wire.Sum(wire.EncodeFile(o)), nil
		}
		env, e := release.DecodeEnvelope(r.Envelope)
		if e != nil {
			return "", e
		}
		if env.QueueID != q || env.RequestID != r.RequestID || env.ReleaseID != r.TargetID {
			return "", malformed("release envelope identity")
		}
		if env.Actor.ID != r.Actor.ID || env.Actor.Role != r.Actor.Role {
			return "", malformed("release envelope actor")
		}
		return env.Sha256(), nil
	case KeepJournal, AdoptFile:
		id, e := wire.ParseTicketID("targetId", r.TargetID)
		if e != nil {
			return "", e
		}
		if id.QueueID() != q.Raw {
			return "", malformed("target queue")
		}
		if r.File == nil {
			return "", malformed("original file absent")
		}
		if r.Operation == AdoptFile {
			return mutation.AdoptDigest(r.Actor, q, id, r.RequestID, r.File), nil
		}
		if _, e = wire.ParseDigest("canonicalSha256", string(r.CanonicalSha256)); e != nil {
			return "", e
		}
		o.Obj.Set("canonicalSha256", s(string(r.CanonicalSha256)))
		o.Obj.Set("fileSha256", s(string(wire.Sum(r.File))))
		o.Obj.Set("targetId", s(r.TargetID))
	default:
		return "", wire.Errorf(wire.CodeUnsupported, "operation", "outside model subset")
	}
	return wire.Sum(wire.EncodeFile(o)), nil
}

func refused(id, outcome, code, detail string) Result {
	codes := []string{}
	if code != "" {
		codes = append(codes, code)
	}
	return Result{Kind: "Refused", Outcome: mutation.Outcome{RequestID: id, Outcome: outcome, Codes: codes}, Coverage: coverage(), Detail: detail}
}
func failed(id string, e error) Result {
	out := mutation.OutcomeValidationFailed
	if wire.CodeOf(e) == wire.CodeUnsupported || wire.CodeOf(e) == wire.CodeUnsupportedVersion {
		out = mutation.OutcomeUnsupported
	}
	return refused(id, out, wire.CodeOf(e), e.Error())
}
func noChange(id string) Result {
	return Result{Kind: "NoChange", Outcome: mutation.Outcome{RequestID: id, Outcome: mutation.OutcomeCompleted, Codes: []string{}}, Coverage: coverage()}
}

// Model produces one closed template and proves its modeled capacity closure.
// Replay is consulted before fresh-transition preconditions. No output grants I/O.
func Model(r Request, in Input) Result {
	if _, e := wire.ParseLabel("actor", r.Actor.ID); e != nil {
		return refused(r.RequestID, mutation.OutcomeUnauthorized, "", "hypothetical actor malformed")
	}
	if r.Actor.Role != "OWNER" && r.Actor.Role != "OPERATOR" {
		return refused(r.RequestID, mutation.OutcomeUnauthorized, "", "outside hypothetical role subset")
	}
	if r.Operation == Mutate {
		env, err := mutation.Decode(r.Envelope)
		if err != nil {
			return failed(r.RequestID, err)
		}
		if env.Actor.ID != r.Actor.ID || env.Actor.Role != r.Actor.Role {
			return refused(r.RequestID, mutation.OutcomeUnauthorized, "", "envelope actor differs from invoking binding")
		}
	}
	d, e := Digest(r)
	if e != nil {
		return failed(r.RequestID, e)
	}
	switch in.Replay.State {
	case "ABSENT":
		if len(in.Replay.Record) != 0 {
			return failed(r.RequestID, malformed("absence with record"))
		}
	case "FOUND":
		req, e := snapshot.DecodeRequest(in.Replay.Record)
		if e != nil {
			return failed(r.RequestID, e)
		}
		if e = canonical(in.Replay.Record); e != nil {
			return failed(r.RequestID, e)
		}
		if req.Entry.RequestID != r.RequestID || req.Seq.Uint64() > MaxReceipts {
			return failed(r.RequestID, malformed("replay request identity/sequence"))
		}
		if req.Entry.MutationSha256 != d {
			return refused(r.RequestID, mutation.OutcomeRequestIDConflict, wire.CodeRequestIDConflict, "different original request")
		}
		if req.Entry.Outcome.Outcome == mutation.OutcomeCompleted {
			ticketOperation := r.Operation == KeepJournal || r.Operation == AdoptFile || r.Operation == Mutate
			releaseOperation := r.Operation == Release
			if ticketOperation != (req.Entry.Outcome.ResultingRevision != nil) || releaseOperation != (req.Entry.Outcome.ReleaseID != nil) || len(req.Entry.Outcome.Codes) != 0 {
				return failed(r.RequestID, malformed("replay outcome shape"))
			}
		}
		out := req.Entry.Outcome
		out.Replayed = true
		return Result{Kind: "Replay", Outcome: out, Coverage: coverage()}
	default:
		return refused(r.RequestID, mutation.OutcomeStorageFailed, wire.CodeMalformed, "replay observation missing, unknown or failed")
	}
	if in.Premise != FixtureNoRuntime && in.Premise != LocalOperator {
		return refused(r.RequestID, mutation.OutcomeUnsupported, wire.CodeUnsupported, "explicit no-runtime fixture or local-operator premise required")
	}
	if _, e = wire.ParseTimestamp("recordedAt", string(in.RecordedAt)); e != nil {
		return failed(r.RequestID, e)
	}
	state, e := validateInput(r, in)
	if e != nil {
		return failed(r.RequestID, e)
	}
	if r.Operation == Init && state.head != nil {
		return refused(r.RequestID, mutation.OutcomeBlocked, "", "already initialized")
	}
	if r.Operation != Init && state.head == nil {
		return refused(r.RequestID, mutation.OutcomeBlocked, wire.CodeUninitialized, "no initialized base")
	}
	if r.Operation == Pause && state.barrier != nil {
		if state.barrier.Scope == "ADMISSION" && state.barrier.Reason == "OPERATOR" {
			return noChange(r.RequestID)
		}
		return refused(r.RequestID, mutation.OutcomeBlocked, wire.CodePaused, "barrier replacement forbidden")
	}
	if state.barrier != nil && state.barrier.Scope == "ALL" && r.Operation != KeepJournal && r.Operation != AdoptFile && r.Operation != Unpause {
		return refused(r.RequestID, mutation.OutcomeBlocked, wire.CodePaused, "ALL barrier forbids mutation")
	}
	if r.Operation == Unpause && state.barrier == nil {
		return noChange(r.RequestID)
	}
	if (r.Operation == Init || r.Operation == KeepJournal || r.Operation == AdoptFile || r.Operation == Mutate || r.Operation == Release || r.Operation == PolicyUpdate) && in.Branch != state.queue.IntentBranch {
		return refused(r.RequestID, mutation.OutcomeBlocked, wire.CodeIntentBranchMismatch, "primary intent branch differs")
	}
	posts := map[string][]byte{}
	var effect *ticketEffect
	var relEffect *releaseEffect
	switch r.Operation {
	case Init:
		for path := range in.Inventory.files {
			if !strings.HasPrefix(path, "evidence/") {
				return failed(r.RequestID, malformed("genesis targets not empty"))
			}
		}
		posts["VERSION"] = []byte(snapshot.VersionBytes)
		posts["intent/queue.json"] = bytes.Clone(r.Queue)
		posts["intent/policy.json"] = bytes.Clone(r.Policy)
		posts["reservations.json"] = emptyReservations(state.queue.QueueID.Raw)
		init := snapshot.Init{QueueID: state.queue.QueueID, PrimaryWorktree: r.PrimaryWorktree, VersionSha256: wire.Sum(posts["VERSION"])}
		raw := wire.EncodeFile(init.Value())
		posts["pinned/"+string(wire.Sum(raw))+".json"] = raw
	case Pause:
		posts["barrier.json"] = wire.EncodeFile(object("profile", s(snapshot.ProfileBarrier), "queueId", s(r.QueueID), "scope", s("ADMISSION"), "reason", s("OPERATOR"), "actor", s(r.Actor.ID), "sinceSeq", s(string(wire.SizeOf(state.head.LastSeq.Uint64()+1))), "since", s(string(in.RecordedAt))))
	case Unpause:
		posts["barrier.json"] = nil
	case PolicyUpdate:
		// TM-V0-030: the caller's expected version must be the current one, and
		// the new file must be exactly the next version.
		current := state.policy.PolicyVersion
		if r.ExpectedPolicyVersion.Uint64() != current.Uint64() {
			return refused(r.RequestID, mutation.OutcomeRevisionConflict, "", "expectedPolicyVersion "+string(r.ExpectedPolicyVersion)+" but the current policy is at version "+string(current))
		}
		next, e := intent.DecodePolicy(r.Policy)
		if e != nil {
			return failed(r.RequestID, e)
		}
		if next.PolicyVersion.Uint64() != current.Uint64()+1 {
			return failed(r.RequestID, malformed("policyVersion must be the current version plus one"))
		}
		if len(next.Runtimes) != 0 {
			return failed(r.RequestID, malformed("runtime inventory outside subset"))
		}
		posts["intent/policy.json"] = bytes.Clone(r.Policy)
	case Mutate:
		env, e := mutation.Decode(r.Envelope)
		if e != nil {
			return failed(r.RequestID, e)
		}
		// The replay decision was already made above against this request's
		// digest, so the pure library is handed an absent index: consulting a
		// second index here could only disagree with that decision.
		ctx := mutation.Context{Binding: r.Actor, Queue: state.queue, Policy: state.policy, Inventory: state.tickets, Attempts: zeroAttempts{}, Requests: absentIndex{}, Now: in.RecordedAt}
		applied := mutation.Apply(ctx, env)
		if !applied.Planned() {
			return Result{Kind: "Refused", Outcome: applied.Outcome, Coverage: coverage(), Detail: applied.Detail}
		}
		pre, _ := state.tickets.Get(applied.Post.TicketID.Raw)
		path := "intent/tickets/" + applied.Post.TicketID.Local + ".json"
		// Every ticket this operation did not name must already agree with its
		// canonical record (validateInput), and the named one must agree with
		// the record the mutation read: otherwise the post would be computed
		// from a record the projection does not hold.
		if pre != nil && !in.Inventory.matches(path, wire.EncodeFile(pre.Value())) {
			return failed(r.RequestID, malformed("physical projection differs from canonical record"))
		}
		if pre == nil {
			if _, exists := in.Inventory.files[path]; exists {
				return failed(r.RequestID, malformed("created ticket already has a projection"))
			}
		}
		posts[path] = bytes.Clone(wire.EncodeFile(applied.Post.Value()))
		if applied.QueuePost != nil {
			// CREATE advanced nextSerial; the manifest is republished with it.
			posts["intent/queue.json"] = bytes.Clone(wire.EncodeFile(applied.QueuePost.Value()))
		}
		effect = &ticketEffect{pre: pre, post: applied.Post, kind: receiptKind(env.Operation)}
	case Release:
		if r.File != nil {
			current := state.releases[r.TargetID]
			if current == nil || wire.Sum(release.Encode(current)) != r.CanonicalSha256 {
				return failed(r.RequestID, malformed("canonical release choice changed"))
			}
			path := "intent/releases/" + current.ReleaseID + ".json"
			if !in.Inventory.matches(path, r.File) {
				return failed(r.RequestID, malformed("physical release projection differs from original choice"))
			}
			posts[path] = release.Encode(current)
			posts["evidence/"+string(wire.Sum(r.File))] = bytes.Clone(r.File)
			relEffect = &releaseEffect{pre: current, post: current, kind: "RECONCILE"}
			break
		}
		env, e := release.DecodeEnvelope(r.Envelope)
		if e != nil {
			return failed(r.RequestID, e)
		}
		current := state.releases[env.ReleaseID]
		configured := state.policy.Roles
		post, e := release.Apply(env, release.Actor{ID: r.Actor.ID, Role: r.Actor.Role}, current, state.queue.QueueID, configured, state.ticketIDs(), in.RecordedAt, in.ReleaseCandidate, in.ReleaseGates, in.ReleaseObservation)
		if e != nil {
			return failed(r.RequestID, e)
		}
		prospective := make([]*release.Record, 0, len(state.releases)+1)
		for id, existing := range state.releases {
			if id != post.ReleaseID {
				prospective = append(prospective, existing)
			}
		}
		prospective = append(prospective, post)
		if e = release.ValidateGraph(prospective); e != nil {
			return failed(r.RequestID, e)
		}
		path := "intent/releases/" + post.ReleaseID + ".json"
		if current == nil {
			if _, exists := in.Inventory.files[path]; exists {
				return failed(r.RequestID, malformed("created release already has a projection"))
			}
		} else if !in.Inventory.matches(path, release.Encode(current)) {
			return failed(r.RequestID, malformed("physical release projection differs from canonical record"))
		}
		posts[path] = release.Encode(post)
		relEffect = &releaseEffect{pre: current, post: post}
	case KeepJournal, AdoptFile:
		target, _ := state.tickets.Get(r.TargetID)
		if target == nil {
			return failed(r.RequestID, malformed("missing canonical target"))
		}
		path := "intent/tickets/" + target.TicketID.Local + ".json"
		if !in.Inventory.matches(path, r.File) {
			return failed(r.RequestID, malformed("physical projection differs from original choice"))
		}
		if r.Operation == KeepJournal {
			if target.FileDigest() != r.CanonicalSha256 {
				return failed(r.RequestID, malformed("canonical choice changed"))
			}
			if bytes.Equal(wire.EncodeFile(target.Value()), r.File) {
				return noChange(r.RequestID)
			}
			posts[path] = bytes.Clone(wire.EncodeFile(target.Value()))
			posts["evidence/"+string(wire.Sum(r.File))] = bytes.Clone(r.File)
			effect = &ticketEffect{pre: target, post: target, kind: "RECONCILE"}
		} else {
			ctx := mutation.Context{Binding: r.Actor, Queue: state.queue, Policy: state.policy, Inventory: state.tickets, Attempts: zeroAttempts{}, Requests: absentIndex{}, Now: in.RecordedAt}
			adopted := mutation.Adopt(ctx, r.RequestID, target, r.File)
			if !adopted.Planned() {
				return Result{Kind: "Refused", Outcome: adopted.Outcome, Coverage: coverage(), Detail: adopted.Detail}
			}
			posts[path] = bytes.Clone(wire.EncodeFile(adopted.Post.Value()))
			effect = &ticketEffect{pre: target, post: adopted.Post, kind: "RECONCILE"}
		}
	}
	p, out, e := freeze(r, d, in.RecordedAt, in.Inventory, state.head, posts, effect, relEffect)
	if e != nil {
		return failed(r.RequestID, e)
	}
	if _, e = CheckCapacity(p); e != nil {
		return refused(r.RequestID, mutation.OutcomeCapacityExhausted, wire.CodeOf(e), e.Error())
	}
	return Result{Kind: "Transaction", Outcome: out, Coverage: coverage(), Plan: p}
}

type zeroAttempts struct{}

func (zeroAttempts) LiveAttempt(string) ticket.Observation { return ticket.Unsatisfied }

type absentIndex struct{}

func (absentIndex) Lookup(string) (mutation.IndexEntry, bool, error) {
	return mutation.IndexEntry{}, false, nil
}
func emptyReservations(q string) []byte {
	return wire.EncodeFile(object("profile", s("taskman-reservation-set/0"), "queueId", s(q), "entries", wire.Array()))
}

type inputState struct {
	head     *snapshot.Head
	barrier  *snapshot.Barrier
	queue    *intent.Queue
	policy   *intent.Policy
	tickets  *ticket.Inventory
	releases map[string]*release.Record
}

func (s inputState) ticketIDs() map[string]bool {
	out := map[string]bool{}
	for _, id := range s.tickets.IDs() {
		out[id] = true
	}
	return out
}

func validateInput(r Request, in Input) (inputState, error) {
	var st inputState
	if in.Inventory == nil {
		return st, malformed("complete inventory required")
	}
	if len(in.CanonicalTickets) > wire.MaxTicketsPerQueue {
		return st, limit("ticket inventory")
	}
	qraw, praw := in.Queue, in.Policy
	if r.Operation == Init && len(in.Head) == 0 {
		qraw, praw = r.Queue, r.Policy
	}
	q, e := intent.DecodeQueue(qraw)
	if e != nil {
		return st, e
	}
	st.queue = q
	if q.QueueID.Raw != r.QueueID || !q.Fixture || q.ImportMapSha256 != nil || q.ExecutionCutover != nil {
		return st, malformed("unsupported queue identity/state")
	}
	p, e := intent.DecodePolicy(praw)
	if e != nil {
		return st, e
	}
	st.policy = p
	if len(p.Runtimes) != 0 {
		return st, malformed("runtime inventory outside subset")
	}
	if e = canonical(qraw); e != nil {
		return st, e
	}
	if e = canonical(praw); e != nil {
		return st, e
	}
	if len(in.Head) > 0 {
		st.head, e = snapshot.DecodeHead(in.Head)
		if e != nil {
			return st, e
		}
		if e = canonical(in.Head); e != nil {
			return st, e
		}
		if st.head.QueueID.Raw != r.QueueID || !in.Inventory.matches("head.json", in.Head) {
			return st, malformed("head inventory binding")
		}
		if !in.Inventory.matches("intent/queue.json", qraw) || !in.Inventory.matches("intent/policy.json", praw) {
			return st, malformed("queue/policy inventory binding")
		}
		if !bytes.Equal(in.Reservations, emptyReservations(r.QueueID)) || !in.Inventory.matches("reservations.json", in.Reservations) {
			return st, malformed("empty reservations required")
		}
		if e = in.Inventory.chain(st.head); e != nil {
			return st, e
		}
	}
	if in.Barrier != nil {
		st.barrier, e = snapshot.DecodeBarrier(in.Barrier)
		if e != nil {
			return st, e
		}
		if e = canonical(in.Barrier); e != nil {
			return st, e
		}
		if st.head == nil || st.barrier.QueueID.Raw != r.QueueID || st.barrier.SinceSeq.Uint64() < 1 || st.barrier.SinceSeq.Uint64() > st.head.LastSeq.Uint64() || !in.Inventory.matches("barrier.json", in.Barrier) {
			return st, malformed("barrier binding")
		}
	} else if _, ok := in.Inventory.files["barrier.json"]; ok {
		return st, malformed("barrier observation missing")
	}
	records := make([]*ticket.Record, 0, len(in.CanonicalTickets))
	var total uint64
	for _, raw := range in.CanonicalTickets {
		if uint64(len(raw)) > wire.MaxIntentTreeBytes-total {
			return st, limit("canonical inventory bytes")
		}
		total += uint64(len(raw))
		rec, e := ticket.Decode(raw)
		if e != nil {
			return st, e
		}
		if e = canonical(raw); e != nil {
			return st, e
		}
		path := "intent/tickets/" + rec.TicketID.Local + ".json"
		f, ok := in.Inventory.files[path]
		if !ok {
			return st, malformed("canonical ticket has no physical projection")
		}
		if r.Operation != Unpause && rec.TicketID.Raw != r.TargetID && (f.Sha256 != rec.FileDigest() || f.Bytes.Uint64() != uint64(len(raw))) {
			return st, malformed("unselected divergent ticket")
		}
		records = append(records, rec)
	}
	st.tickets, e = ticket.NewInventory(q.QueueID, records)
	if e != nil {
		return st, e
	}
	count := 0
	for path := range in.Inventory.files {
		if strings.HasPrefix(path, "intent/tickets/") {
			count++
		}
	}
	if count != len(records) {
		return st, malformed("incomplete canonical ticket inventory")
	}
	st.releases = map[string]*release.Record{}
	for _, raw := range in.CanonicalReleases {
		rec, decodeErr := release.Decode(raw)
		if decodeErr != nil {
			return st, decodeErr
		}
		if rec.QueueID != q.QueueID || st.releases[rec.ReleaseID] != nil {
			return st, malformed("invalid canonical release inventory")
		}
		path := "intent/releases/" + rec.ReleaseID + ".json"
		physical, ok := in.Inventory.files[path]
		if !ok {
			return st, malformed("canonical release has no physical projection")
		}
		if r.Operation != Release || rec.ReleaseID != r.TargetID {
			if physical.Sha256 != wire.Sum(raw) || physical.Bytes.Uint64() != uint64(len(raw)) {
				return st, malformed("unselected divergent release")
			}
		}
		st.releases[rec.ReleaseID] = rec
	}
	releaseCount := 0
	for path := range in.Inventory.files {
		if strings.HasPrefix(path, "intent/releases/") {
			releaseCount++
		}
	}
	if releaseCount != len(st.releases) {
		return st, malformed("incomplete canonical release inventory")
	}
	all := make([]*release.Record, 0, len(st.releases))
	for _, rec := range st.releases {
		all = append(all, rec)
	}
	if e = release.ValidateGraph(all); e != nil {
		return st, e
	}
	return st, nil
}

func cloneRequest(r Request) Request {
	r.Queue = bytes.Clone(r.Queue)
	r.Policy = bytes.Clone(r.Policy)
	r.File = bytes.Clone(r.File)
	r.Envelope = bytes.Clone(r.Envelope)
	return r
}

// ticketEffect names the ticket a transaction changes. pre is the record the
// post chains from and is nil exactly for a CREATE, which has none; post is
// always the published record. kind is the §3.1 receipt kind. A non-ticket
// operation leaves the effect nil.
type ticketEffect struct {
	pre, post *ticket.Record
	kind      string
}

type releaseEffect struct {
	pre, post *release.Record
	kind      string
}

// receiptKind maps a §3.3 mutation operation to its §3.1 receipt kind. Only
// ARCHIVE and RESTORE have their own kind; every other mutation is MUTATION.
func receiptKind(op string) string {
	switch op {
	case mutation.OpArchive:
		return "ARCHIVE"
	case mutation.OpRestore:
		return "RESTORE"
	}
	return "MUTATION"
}

func freeze(r Request, d wire.Digest, now wire.Timestamp, inv *Inventory, base *snapshot.Head, posts map[string][]byte, eff *ticketEffect, rel *releaseEffect) (*Plan, mutation.Outcome, error) {
	seq := uint64(1)
	generation := wire.Size("0")
	var prev *wire.Digest
	if base != nil {
		if base.LastSeq.Uint64() >= MaxReceipts {
			return nil, mutation.Outcome{}, limit("no receipt slot")
		}
		seq = base.LastSeq.Uint64() + 1
		generation = base.Generation
		prev = base.LastReceiptSha256
	}
	out := mutation.Outcome{RequestID: r.RequestID, Outcome: mutation.OutcomeCompleted, Codes: []string{}}
	n := wire.SizeOf(seq)
	out.ReceiptSeq = &n
	var expected *wire.Count
	ticketVal := wire.Null()
	if eff != nil {
		out.ResultingRevision = &eff.post.Revision
		out.ResultingAcceptanceRevision = &eff.post.AcceptanceRevision
		ticketVal = s(eff.post.TicketID.Raw)
		// A CREATE has no prior revision to expect; every other ticket
		// operation chains from the record it read.
		if eff.pre != nil {
			expected = &eff.pre.Revision
		}
	}
	if rel != nil {
		id := rel.post.ReleaseID
		out.ReleaseID = &id
		out.ResultingReleaseRevision = &rel.post.Revision
		if rel.pre != nil {
			expected = &rel.pre.Revision
		}
	}
	req := wire.EncodeFile(object("requestId", s(r.RequestID), "seq", s(string(n)), "mutationSha256", s(string(d)), "outcome", out.Value()))
	if _, e := snapshot.DecodeRequest(req); e != nil {
		return nil, out, e
	}
	rp, _ := snapshot.RequestPath(r.RequestID)
	if _, ok := inv.files[rp]; ok {
		return nil, out, malformed("absent replay conflicts with request path")
	}
	posts[rp] = req
	pre, post := []wire.Value{}, []wire.Value{}
	arts := []Artifact{}
	blobs := map[string][]byte{}
	paths := make([]string, 0, len(posts))
	for path := range posts {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		raw := posts[path]
		var before *wire.Digest
		if old, ok := inv.files[path]; ok {
			b := old.Sha256
			before = &b
		}
		pre = append(pre, object("path", s(path), "sha256", digestValue(before)))
		if raw == nil {
			post = append(post, object("path", s(path), "sha256", wire.Null(), "record", wire.Null(), "blobSha256", wire.Null()))
			continue
		}
		bound, e := snapshot.PostBound(path)
		if e != nil {
			return nil, out, e
		}
		if len(raw) > bound {
			return nil, out, limit(path)
		}
		if e = snapshot.ContentPath(path, raw); e != nil {
			return nil, out, e
		}
		hash := wire.Sum(raw)
		rec := wire.Null()
		blob := wire.Null()
		if path != "VERSION" && !strings.HasPrefix(path, "evidence/") && len(raw) <= wire.MaxInlinePostEntryBytes {
			v, e := wire.Parse(raw)
			if e != nil {
				return nil, out, e
			}
			rec = v
		} else {
			blob = s(string(hash))
			blobs["evidence/"+string(hash)] = raw
		}
		post = append(post, object("path", s(path), "sha256", s(string(hash)), "record", rec, "blobSha256", blob))
		arts = append(arts, newArtifact("POST", path, raw))
	}
	kind := r.Operation
	if eff != nil {
		kind = eff.kind
	}
	if rel != nil && rel.kind != "" {
		kind = rel.kind
	}
	receiptValue := object("profile", s(snapshot.ProfileReceipt), "seq", s(string(n)), "prev", digestValue(prev), "kind", s(kind), "requestId", s(r.RequestID), "actor", object("id", s(r.Actor.ID), "role", s(r.Actor.Role)), "ticketId", ticketVal, "attemptId", wire.Null(), "generation", wire.Null(), "expectedRevision", countValue(expected), "headGeneration", s(string(generation)), "pre", wire.Array(pre...), "post", wire.Array(post...), "outcome", s(out.Outcome), "codes", wire.Array(), "recordedAt", s(string(now)))
	if rel != nil {
		receiptValue.Obj.Set("releaseId", s(rel.post.ReleaseID))
	}
	raw := wire.EncodeFile(receiptValue)
	if _, e := snapshot.DecodeReceipt(raw); e != nil {
		return nil, out, e
	}
	hash := wire.Sum(raw)
	h := snapshot.Head{QueueID: mustQueue(r.QueueID), LastSeq: n, LastReceiptSha256: &hash, Generation: generation, InitSha256: hash, PrimaryWorktree: r.PrimaryWorktree, VersionSha256: wire.Sum([]byte(snapshot.VersionBytes))}
	if base != nil {
		h.InitSha256 = base.InitSha256
		h.PrimaryWorktree = base.PrimaryWorktree
		h.VersionSha256 = base.VersionSha256
	}
	head := wire.EncodeFile(h.Value())
	if _, e := snapshot.DecodeHead(head); e != nil {
		return nil, out, e
	}
	name, _ := snapshot.ReceiptName(seq)
	arts = append(arts, newArtifact("RECEIPT", "receipts/"+name, raw), newArtifact("HEAD", "head.json", head))
	for path, b := range blobs {
		if old, ok := inv.files[path]; ok {
			if old.Sha256 != wire.Sum(b) || old.Bytes.Uint64() != uint64(len(b)) {
				return nil, out, malformed("existing blob conflicts")
			}
			continue
		}
		if _, ok := posts[path]; !ok {
			arts = append(arts, newArtifact("EVIDENCE", path, b))
		}
	}
	sort.Slice(arts, func(i, j int) bool {
		if arts[i].Role != arts[j].Role {
			return arts[i].Role < arts[j].Role
		}
		return arts[i].Target < arts[j].Target
	})
	desc := Descriptor{QueueID: r.QueueID, Operation: r.Operation, RequestID: r.RequestID, RequestSha256: d, RecordedAt: now}
	if base != nil {
		desc.Base = &Base{LastSeq: base.LastSeq, LastReceiptSha256: *base.LastReceiptSha256}
	}
	for i := range arts {
		arts[i].Slot = slotName(i)
		desc.Artifacts = append(desc.Artifacts, arts[i].Description)
	}
	encoded, e := desc.Encode()
	if e != nil {
		return nil, out, e
	}
	return &Plan{operation: r.Operation, request: cloneRequest(r), descriptor: encoded, artifacts: arts, receipt: raw, head: head, posts: posts, base: inv, baseHead: base}, out, nil
}
func mustQueue(q string) wire.QueueID { id, _ := wire.ParseQueueID("", q); return id }
