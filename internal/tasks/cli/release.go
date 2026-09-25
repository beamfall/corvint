package cli

import (
	"context"
	"sort"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/release"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

var releaseVerbs = map[string]string{"create": release.OpCreate, "update": release.OpUpdate, "candidate": release.OpCandidate, "promote": release.OpPromote}

func releaseCommand(env Env, verb string, args []string) *wire.Result {
	cmd := []string{"release", verb}
	if verb == "list" || verb == "show" || verb == "readiness" {
		return releaseRead(env, verb, args)
	}
	op, ok := releaseVerbs[verb]
	if !ok && verb != "record-gate" {
		return usage(cmd, "unknown release verb")
	}
	f, res := parseMutateFlags(cmd, args)
	if res != nil {
		return res
	}
	if f.requestID == "" || f.target == "" {
		return usage(cmd, "--request-id and --target are required")
	}
	actor, err := initActor(f.role)
	if err != nil {
		return errorResult(cmd, err)
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	st, err := intent.Load(repo.PrimaryWorktree)
	if err != nil {
		return errorResult(cmd, err)
	}
	var payload wire.Value
	if verb == "candidate" && f.payload == "" && !f.payloadFromStdin {
		payload = wire.ObjectValue(wire.NewObject().Set("headCommit", wire.Null()))
	} else if verb == "promote" && f.payload == "" && !f.payloadFromStdin {
		payload = wire.ObjectValue(wire.NewObject())
	} else {
		payload, err = readPayloadWithWhitespace(env, f, true)
		if err != nil {
			return errorResult(cmd, err)
		}
	}
	if verb == "record-gate" {
		r := wire.NewReader(payload, "payload")
		provenance := r.Field("attestation").Field("provenance").Enum(release.Manual, release.External)
		if err := r.Err(); err != nil {
			return errorResult(cmd, err)
		}
		op = release.OpExternalAttest
		if provenance == release.Manual {
			op = release.OpManualAttest
		}
	}
	expected := wire.Null()
	if op != release.OpCreate {
		if f.expected == "" {
			return usage(cmd, "--expected-revision is required")
		}
		expected = wire.String(f.expected)
	}
	now, _ := wire.ParseTimestamp("now", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	issued := now
	if f.issuedAt != "" {
		issued, err = wire.ParseTimestamp("issuedAt", f.issuedAt)
		if err != nil {
			return errorResult(cmd, err)
		}
	}
	o := wire.NewObject().Set("profile", wire.String(release.MutationProfile)).Set("requestId", wire.String(f.requestID)).Set("actor", wire.ObjectValue(wire.NewObject().Set("id", wire.String(actor.ID)).Set("role", wire.String(actor.Role)))).Set("queueId", wire.String(st.Queue.QueueID.Raw)).Set("releaseId", wire.String(f.target)).Set("expectedRevision", expected).Set("operation", wire.String(op)).Set("payload", payload).Set("issuedAt", wire.String(string(issued)))
	report, err := store.Release(context.Background(), repo, actor, wire.EncodeFile(wire.ObjectValue(o)), now)
	if err != nil {
		return errorResult(cmd, err)
	}
	item := wire.NewObject().Set("releaseId", wire.String(report.Release)).Set("receipt", wire.String(report.Receipt)).Set("replayed", wire.Bool(report.Outcome.Replayed)).Set("nativeGateExecution", wire.String("NOT_RUN"))
	item.Set("resultingRevision", nullableCount(report.Outcome.ResultingReleaseRevision))
	result := &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(item)}}
	if report.Outcome.Outcome != "COMPLETED" {
		result.Outcome = wire.OutcomeRefused
		result.Codes = report.Outcome.Codes
		if report.Detail != "" {
			result.Warnings = append(result.Warnings, prose(report.Detail))
		}
	}
	return result
}

func releaseRead(env Env, verb string, args []string) *wire.Result {
	cmd := []string{"release", verb}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	st, err := intent.Load(repo.PrimaryWorktree)
	if err != nil {
		return errorResult(cmd, err)
	}
	sort.Slice(st.Releases, func(i, j int) bool { return st.Releases[i].ReleaseID < st.Releases[j].ReleaseID })
	if verb == "list" {
		vs := make([]wire.Value, len(st.Releases))
		for i, r := range st.Releases {
			vs[i] = releaseSummary(r)
		}
		return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: vs}
	}
	if len(args) != 1 {
		return usage(cmd, "release id required")
	}
	var target *release.Record
	for _, r := range st.Releases {
		if r.ReleaseID == args[0] {
			target = r
		}
	}
	if target == nil {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "releaseId", "release absent"))
	}
	if verb == "show" {
		return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{releaseDetail(target)}}
	}
	head, _, source, err := store.ObserveSource(repo.PrimaryWorktree, false)
	if err != nil {
		return errorResult(cmd, err)
	}
	tickets := map[string]*ticket.Record{}
	td := map[string]wire.Digest{}
	for _, t := range st.Tickets {
		tickets[t.TicketID.Raw] = t
		td[t.TicketID.Raw] = st.Digests[intent.TicketsDir+"/"+t.TicketID.Local+".json"]
	}
	pred := map[string]wire.Digest{}
	for _, r := range st.Releases {
		if r.Promotion != nil {
			pred[r.ReleaseID] = release.PromotionDigest(r.Promotion)
		}
	}
	gates := make([]release.Gate, len(st.Policy.Gates))
	for i, g := range st.Policy.Gates {
		gates[i] = release.Gate{GateID: g.GateID, Kind: g.Kind, Required: g.Required}
	}
	ready := release.Assess(target, gates, release.Observation{HeadCommit: head, SourceSha256: source, PolicySha256: wire.Sum(st.Policy.Raw), Tickets: tickets, TicketDigests: td, PredecessorPromotions: pred})
	o := releaseDetail(target).Obj
	o.Set("readiness", wire.String(ready.State)).Set("missing", wire.Strings(ready.Missing)).Set("nativeGateExecution", wire.String("NOT_RUN"))
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
}

func releaseSummary(r *release.Record) wire.Value {
	return wire.ObjectValue(wire.NewObject().Set("releaseId", wire.String(r.ReleaseID)).Set("revision", wire.String(string(r.Revision))).Set("version", wire.String(r.Version)).Set("title", wire.String(r.Title)).Set("candidate", wire.Bool(r.Candidate != nil)).Set("promoted", wire.Bool(r.Promotion != nil)))
}

func releaseDetail(r *release.Record) wire.Value {
	o := releaseSummary(r).Obj
	ticketIDs := make([]string, len(r.TicketIDs))
	for i, id := range r.TicketIDs {
		ticketIDs[i] = id.Raw
	}
	o.Set("predecessorReleaseIds", wire.Strings(r.PredecessorIDs)).Set("ticketIds", wire.Strings(ticketIDs)).Set("acceptanceCriteria", wire.Strings(r.AcceptanceCriteria)).Set("requiredGates", wire.Strings(r.RequiredGates))
	if r.Candidate == nil {
		o.Set("candidateSha256", wire.Null()).Set("candidateBinding", wire.Null())
	} else {
		o.Set("candidateSha256", wire.String(string(release.CandidateDigest(r.Candidate)))).Set("candidateBinding", candidateBindingValue(r.Candidate))
	}
	attestations := make([]wire.Value, len(r.Attestations))
	for i, a := range r.Attestations {
		criteria := make([]string, len(a.Criteria))
		for j, criterion := range a.Criteria {
			criteria[j] = string(criterion)
		}
		evidence := make([]string, len(a.Evidence))
		for j, digest := range a.Evidence {
			evidence[j] = string(digest)
		}
		attestations[i] = wire.ObjectValue(wire.NewObject().Set("profile", wire.String(release.AttestationProfile)).Set("attestationId", wire.String(a.AttestationID)).Set("attestationSha256", wire.String(string(release.AttestationDigest(a)))).Set("candidateSha256", wire.String(string(a.CandidateSha256))).Set("gateId", wire.String(a.GateID)).Set("provenance", wire.String(a.Provenance)).Set("actor", wire.String(a.Actor)).Set("recordedAt", wire.String(string(a.RecordedAt))).Set("result", wire.String(a.Result)).Set("criteria", wire.Strings(criteria)).Set("evidence", wire.Strings(evidence)).Set("sourceIdentity", wire.String(a.SourceIdentity)))
	}
	o.Set("attestations", wire.Array(attestations...))
	if r.Promotion == nil {
		o.Set("promotionSha256", wire.Null()).Set("promotion", wire.Null())
	} else {
		o.Set("promotionSha256", wire.String(string(release.PromotionDigest(r.Promotion)))).Set("promotion", promotionValue(r.Promotion))
	}
	return wire.ObjectValue(o)
}

func predecessorBindingValues(bindings []release.PredecessorBinding) wire.Value {
	values := make([]wire.Value, len(bindings))
	for i, binding := range bindings {
		values[i] = wire.ObjectValue(wire.NewObject().Set("releaseId", wire.String(binding.ReleaseID)).Set("promotionSha256", wire.String(string(binding.PromotionSha256))))
	}
	return wire.Array(values...)
}

func candidateBindingValue(candidate *release.Candidate) wire.Value {
	tickets := make([]wire.Value, len(candidate.Tickets))
	for i, binding := range candidate.Tickets {
		tickets[i] = wire.ObjectValue(wire.NewObject().Set("ticketId", wire.String(binding.TicketID.Raw)).Set("recordSha256", wire.String(string(binding.RecordSha256))).Set("acceptanceRevision", wire.String(string(binding.AcceptanceRevision))))
	}
	return wire.ObjectValue(wire.NewObject().Set("repositoryIdentity", wire.String(candidate.RepositoryIdentity)).Set("headCommit", wire.String(candidate.HeadCommit)).Set("headTree", wire.String(candidate.HeadTree)).Set("sourceSha256", wire.String(string(candidate.SourceSha256))).Set("definitionSha256", wire.String(string(candidate.DefinitionSha256))).Set("policySha256", wire.String(string(candidate.PolicySha256))).Set("tickets", wire.Array(tickets...)).Set("predecessors", predecessorBindingValues(candidate.Predecessors)))
}

func promotionValue(promotion *release.Promotion) wire.Value {
	digests := make([]string, len(promotion.AttestationSha256s))
	for i, digest := range promotion.AttestationSha256s {
		digests[i] = string(digest)
	}
	return wire.ObjectValue(wire.NewObject().Set("candidateSha256", wire.String(string(promotion.CandidateSha256))).Set("attestationSha256s", wire.Strings(digests)).Set("predecessors", predecessorBindingValues(promotion.Predecessors)).Set("actor", wire.String(promotion.Actor)).Set("recordedAt", wire.String(string(promotion.RecordedAt))))
}
