package release

import (
	"bytes"
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const MutationProfile = "taskman-release-mutation/0"

const (
	OpCreate         = "RELEASE_CREATE"
	OpUpdate         = "RELEASE_UPDATE"
	OpCandidate      = "RELEASE_CANDIDATE"
	OpExternalAttest = "RELEASE_EXTERNAL_ATTEST"
	OpManualAttest   = "RELEASE_MANUAL_ATTEST"
	OpPromote        = "RELEASE_PROMOTE"
)

var Operations = []string{OpCandidate, OpCreate, OpExternalAttest, OpManualAttest, OpPromote, OpUpdate}

var DefaultRoleMatrix = map[string][]string{
	"OWNER":    Operations,
	"OPERATOR": {OpCandidate, OpCreate, OpExternalAttest, OpUpdate},
}

type Actor struct{ ID, Role string }

type Envelope struct {
	RequestID        string
	Actor            Actor
	QueueID          wire.QueueID
	ReleaseID        string
	ExpectedRevision *wire.Count
	Operation        string
	Payload          wire.Value
	IssuedAt         wire.Timestamp
	Raw              []byte
}

func (e *Envelope) Sha256() wire.Digest { return wire.Sum(e.Raw) }

func DecodeEnvelope(data []byte) (*Envelope, error) {
	if len(data) > wire.MaxMutationEnvelopeBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "release mutation envelope larger than %d bytes", wire.MaxMutationEnvelopeBytes)
	}
	v, err := wire.Parse(data)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, wire.EncodeFile(v)) {
		return nil, wire.Errorf(wire.CodeMalformed, "/", "release mutation envelope is not canonical")
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "requestId", "actor", "queueId", "releaseId", "expectedRevision", "operation", "payload", "issuedAt")
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), MutationProfile); err != nil {
		return nil, err
	}
	env := &Envelope{Raw: append([]byte(nil), data...)}
	env.RequestID = requestID(r.Field("requestId"))
	a := r.Field("actor")
	a.Closed("id", "role")
	env.Actor.ID = a.Field("id").Label()
	env.Actor.Role = a.Field("role").Enum("OWNER", "OPERATOR", "WORKER", "REVIEWER", "IMPORTER", "SYSTEM")
	env.QueueID = r.Field("queueId").QueueID()
	env.ReleaseID = r.Field("releaseId").Label()
	env.ExpectedRevision = r.Field("expectedRevision").CountOrNull()
	env.Operation = r.Field("operation").Enum(Operations...)
	env.Payload = r.Field("payload").Value()
	env.IssuedAt = r.Field("issuedAt").Timestamp()
	if err := r.Err(); err != nil {
		return nil, err
	}
	if env.Operation == OpCreate {
		if env.ExpectedRevision != nil {
			return nil, wire.Errorf(wire.CodeMalformed, "/expectedRevision", "RELEASE_CREATE carries null expectedRevision")
		}
	} else if env.ExpectedRevision == nil {
		return nil, wire.Errorf(wire.CodeMalformed, "/expectedRevision", "%s requires expectedRevision", env.Operation)
	}
	if err := validatePayload(env.Operation, env.Payload); err != nil {
		return nil, err
	}
	return env, nil
}

func requestID(r *wire.Reader) string {
	id := r.Identifier()
	if r.Err() == nil && len(id) > wire.MaxRequestIDBytes {
		r.Fail(wire.CodeLimitExceeded, "requestId longer than %d bytes", wire.MaxRequestIDBytes)
	}
	return id
}

func validatePayload(operation string, v wire.Value) error {
	r := wire.NewReader(v, "/payload")
	switch operation {
	case OpCreate, OpUpdate:
		r.Closed("version", "title", "predecessorReleaseIds", "ticketIds", "acceptanceCriteria", "requiredGates")
		r.Field("version").Label()
		r.Field("title").Prose(1, wire.MaxTitleBytes)
		r.Field("predecessorReleaseIds").Strings(wire.MaxReleasesPerQueue, false, (*wire.Reader).Label)
		for _, x := range r.Field("ticketIds").Array(wire.MaxTicketsPerQueue, false) {
			x.TicketID()
		}
		r.Field("acceptanceCriteria").Strings(wire.MaxAcceptanceCriteria, true, func(x *wire.Reader) string { return x.Prose(0, wire.MaxCriterionBytes) })
		r.Field("requiredGates").Strings(wire.MaxRequiredGates, false, (*wire.Reader).Label)
	case OpCandidate:
		r.Closed("headCommit")
		if !r.Field("headCommit").IsNull() {
			oid := r.Field("headCommit").String()
			if _, err := wire.ParseOID("/payload/headCommit", oid); err != nil {
				return err
			}
		}
	case OpExternalAttest, OpManualAttest:
		r.Closed("attestation")
		decodeAttestation(r.Field("attestation"))
	case OpPromote:
		r.Closed()
	default:
		return wire.Errorf(wire.CodeUnsupported, "/operation", "unknown release operation")
	}
	return r.Err()
}

// Authorized applies the policy-removable maximum role matrix. A configured
// role row may remove operations but can never add beyond the maximum.
func Authorized(role, operation, provenance string, configured map[string][]string) bool {
	maximum := contains(DefaultRoleMatrix[role], operation)
	if !maximum {
		return false
	}
	if row, configuredRole := configured[role]; configuredRole && !contains(row, operation) {
		return false
	}
	if operation == OpManualAttest && role != "OWNER" {
		return false
	}
	if operation == OpPromote && role != "OWNER" {
		return false
	}
	return true
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func Apply(env *Envelope, binding Actor, current *Record, queue wire.QueueID, configured map[string][]string, tickets map[string]bool, now wire.Timestamp, observed *Candidate, gates []Gate, observation Observation) (*Record, error) {
	if env.Actor.ID != binding.ID || env.Actor.Role != binding.Role || env.QueueID != queue || !Authorized(binding.Role, env.Operation, "", configured) {
		return nil, wire.Errorf(wire.CodeMalformed, "actor", "release operation unauthorized")
	}
	if env.Operation == OpCreate {
		if current != nil {
			return nil, wire.Errorf(wire.CodeDuplicateID, "releaseId", "release exists")
		}
	} else {
		if current == nil {
			return nil, wire.Errorf(wire.CodeMalformed, "releaseId", "release absent")
		}
		if current.Revision != *env.ExpectedRevision {
			return nil, wire.Errorf(wire.CodeStaleTicket, "expectedRevision", "release revision changed")
		}
	}
	if current != nil && current.Promotion != nil {
		return nil, wire.Errorf(wire.CodeMalformed, "releaseId", "promoted release is immutable")
	}
	var out *Record
	switch env.Operation {
	case OpCreate, OpUpdate:
		r := wire.NewReader(env.Payload, "payload")
		out = &Record{QueueID: queue, ReleaseID: env.ReleaseID, Version: r.Field("version").Label(), Title: r.Field("title").Prose(1, wire.MaxTitleBytes), PredecessorIDs: r.Field("predecessorReleaseIds").Strings(wire.MaxReleasesPerQueue, false, (*wire.Reader).Label), AcceptanceCriteria: r.Field("acceptanceCriteria").Strings(wire.MaxAcceptanceCriteria, true, func(x *wire.Reader) string { return x.Prose(0, wire.MaxCriterionBytes) }), RequiredGates: r.Field("requiredGates").Strings(wire.MaxRequiredGates, false, (*wire.Reader).Label), Attestations: []Attestation{}}
		for _, v := range r.Field("ticketIds").Array(wire.MaxTicketsPerQueue, false) {
			out.TicketIDs = append(out.TicketIDs, v.TicketID())
		}
		if err := r.Err(); err != nil {
			return nil, err
		}
		for _, id := range out.TicketIDs {
			if !tickets[id.Raw] {
				return nil, wire.Errorf(wire.CodeMalformed, "ticketIds", "unknown scoped ticket")
			}
		}
		for _, id := range out.RequiredGates {
			found := false
			for _, gate := range gates {
				if gate.GateID == id {
					found = true
				}
			}
			if !found {
				return nil, wire.Errorf(wire.CodeMalformed, "requiredGates", "unknown required gate")
			}
		}
	case OpCandidate:
		if observed == nil {
			return nil, wire.Errorf(wire.CodeMalformed, "candidate", "observation absent")
		}
		claimed := wire.NewReader(env.Payload, "payload").Field("headCommit")
		if !claimed.IsNull() && claimed.String() != observed.HeadCommit {
			return nil, wire.Errorf(wire.CodeSnapshotMoved, "candidate", "supplied candidate differs from observation")
		}
		out = cloneRecord(current)
		out.Candidate = observed
		out.Attestations = []Attestation{}
	case OpExternalAttest, OpManualAttest:
		if current.Candidate == nil {
			return nil, wire.Errorf(wire.CodeMalformed, "candidate", "capture required")
		}
		out = cloneRecord(current)
		a := decodeAttestation(wire.NewReader(env.Payload, "payload").Field("attestation"))
		if a.CandidateSha256 != CandidateDigest(current.Candidate) {
			return nil, wire.Errorf(wire.CodeSnapshotMoved, "candidateSha256", "attestation candidate differs from current candidate")
		}
		a.Actor = binding.ID
		a.RecordedAt = now
		if env.Operation == OpManualAttest {
			a.Provenance = Manual
		} else {
			a.Provenance = External
		}
		out.Attestations = append(out.Attestations, a)
		sort.Slice(out.Attestations, func(i, j int) bool {
			return bytes.Compare(wire.EncodeFile(attestationValue(out.Attestations[i])), wire.EncodeFile(attestationValue(out.Attestations[j]))) < 0
		})
	case OpPromote:
		if ready := Assess(current, gates, observation); ready.State != ReadyAttested {
			return nil, wire.Errorf(wire.CodeMalformed, "readiness", "release not ready")
		}
		out = cloneRecord(current)
		ds := []wire.Digest{}
		for _, a := range out.Attestations {
			if passingAttestation(a, CandidateDigest(out.Candidate), gates) {
				ds = append(ds, AttestationDigest(a))
			}
		}
		sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
		out.Promotion = &Promotion{CandidateSha256: CandidateDigest(out.Candidate), AttestationSha256s: ds, Predecessors: append([]PredecessorBinding(nil), out.Candidate.Predecessors...), Actor: binding.ID, RecordedAt: now}
	}
	if current == nil {
		out.Revision = "1"
	} else {
		d := wire.Sum(Encode(current))
		out.PreviousSha256 = &d
		out.Revision = wire.CountOf(current.Revision.Int() + 1)
	}
	if _, err := Decode(Encode(out)); err != nil {
		return nil, err
	}
	return out, nil
}

func cloneRecord(r *Record) *Record { x, _ := Decode(Encode(r)); return x }
