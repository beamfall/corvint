package intent

import (
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ProfileQueue is the queue manifest profile (§3.1).
const ProfileQueue = "taskman-queue/0"

// ExecutionCutover is the owner's cutover record.
type ExecutionCutover struct {
	EnabledBy    string
	DecisionRef  string
	GateEvidence []wire.Digest
}

// WriteBarrier is the queue's write barrier on the old source.
type WriteBarrier struct {
	Reason string
	Since  *wire.Timestamp
}

// Queue is a validated taskman-queue/0.
type Queue struct {
	QueueID               wire.QueueID
	RepositoryAuthorityID string
	Prefix                string
	NextSerial            wire.Count
	SchemaVersion         string
	CanonicalWriter       string
	ForeignAdapterID      *string
	IntentBranch          string
	Fixture               bool
	ExecutionCutover      *ExecutionCutover
	ImportMapSha256       *wire.Digest
	WriteBarrier          WriteBarrier
}

// DecodeQueue parses and validates queue.json (≤1 MiB).
func DecodeQueue(data []byte) (*Queue, error) {
	if len(data) > wire.MaxQueueFileBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "queue.json larger than %d bytes", wire.MaxQueueFileBytes)
	}
	v, err := wire.Parse(data)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "queueId", "repositoryAuthorityId", "prefix", "nextSerial", "schemaVersion",
		"canonicalWriter", "foreignAdapterId", "intentBranch", "fixture", "executionCutover",
		"importMapSha256", "writeBarrier")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ProfileQueue); err != nil {
		return nil, err
	}
	q := &Queue{}
	q.QueueID = r.Field("queueId").QueueID()
	repoTok := ""
	{
		f := r.Field("repositoryAuthorityId")
		s := f.String()
		if f.Err() == nil {
			tok, err := wire.ParseRepoID(f.Where(), s)
			if err != nil {
				f.Fail(wire.CodeOf(err), "%v", err)
			}
			repoTok = tok
		}
		q.RepositoryAuthorityID = s
	}
	q.Prefix = r.Field("prefix").Label()
	q.NextSerial = r.Field("nextSerial").Count()
	q.SchemaVersion = r.Field("schemaVersion").Exact("0")
	q.CanonicalWriter = r.Field("canonicalWriter").Enum("NATIVE", "ROADMAP", "FOREIGN")
	q.ForeignAdapterID = r.Field("foreignAdapterId").LabelOrNull()
	q.IntentBranch = r.Field("intentBranch").Label()
	q.Fixture = r.Field("fixture").Bool()
	ec := r.Field("executionCutover")
	if !ec.IsNull() {
		ec.Closed("enabledBy", "decisionRef", "gateEvidence")
		c := &ExecutionCutover{}
		c.EnabledBy = ec.Field("enabledBy").Label()
		c.DecisionRef = ec.Field("decisionRef").Label()
		for _, e := range ec.Field("gateEvidence").Array(-1, false) {
			c.GateEvidence = append(c.GateEvidence, e.Digest())
		}
		q.ExecutionCutover = c
	}
	q.ImportMapSha256 = r.Field("importMapSha256").DigestOrNull()
	wb := r.Field("writeBarrier")
	wb.Closed("reason", "since")
	q.WriteBarrier.Reason = wb.Field("reason").Enum("CUTOVER", "EMERGENCY", "NONE")
	q.WriteBarrier.Since = wb.Field("since").TimestampOrNull()
	if err := r.Err(); err != nil {
		return nil, err
	}
	// Field relationships (§2 ID grammar, §3.1).
	if q.QueueID.Authority != repoTok {
		return nil, wire.Errorf(wire.CodeMalformed, "/queueId", "queue authority %q differs from repositoryAuthorityId %q", q.QueueID.Authority, repoTok)
	}
	if (q.CanonicalWriter == "FOREIGN") != (q.ForeignAdapterID != nil) {
		return nil, wire.Errorf(wire.CodeMalformed, "/foreignAdapterId", "foreignAdapterId is non-null iff canonicalWriter is FOREIGN")
	}
	if (q.WriteBarrier.Reason == "NONE") != (q.WriteBarrier.Since == nil) {
		return nil, wire.Errorf(wire.CodeMalformed, "/writeBarrier/since", "since is null iff reason is NONE")
	}
	if _, err := wire.ParseToken("/prefix", q.Prefix, wire.MaxLabelBytes); err != nil {
		return nil, err
	}
	return q, nil
}

// Value renders the queue manifest.
func (q *Queue) Value() wire.Value {
	o := wire.NewObject()
	o.Set("profile", wire.String(ProfileQueue))
	o.Set("queueId", wire.String(q.QueueID.Raw))
	o.Set("repositoryAuthorityId", wire.String(q.RepositoryAuthorityID))
	o.Set("prefix", wire.String(q.Prefix))
	o.Set("nextSerial", wire.String(string(q.NextSerial)))
	o.Set("schemaVersion", wire.String(q.SchemaVersion))
	o.Set("canonicalWriter", wire.String(q.CanonicalWriter))
	o.Set("foreignAdapterId", wire.StringOrNull(q.ForeignAdapterID))
	o.Set("intentBranch", wire.String(q.IntentBranch))
	o.Set("fixture", wire.Bool(q.Fixture))
	if q.ExecutionCutover == nil {
		o.Set("executionCutover", wire.Null())
	} else {
		c := wire.NewObject()
		c.Set("enabledBy", wire.String(q.ExecutionCutover.EnabledBy))
		c.Set("decisionRef", wire.String(q.ExecutionCutover.DecisionRef))
		ev := make([]string, len(q.ExecutionCutover.GateEvidence))
		for i, d := range q.ExecutionCutover.GateEvidence {
			ev[i] = string(d)
		}
		c.Set("gateEvidence", wire.Strings(ev))
		o.Set("executionCutover", wire.ObjectValue(c))
	}
	if q.ImportMapSha256 == nil {
		o.Set("importMapSha256", wire.Null())
	} else {
		o.Set("importMapSha256", wire.String(string(*q.ImportMapSha256)))
	}
	wb := wire.NewObject()
	wb.Set("reason", wire.String(q.WriteBarrier.Reason))
	if q.WriteBarrier.Since == nil {
		wb.Set("since", wire.Null())
	} else {
		wb.Set("since", wire.String(string(*q.WriteBarrier.Since)))
	}
	o.Set("writeBarrier", wire.ObjectValue(wb))
	return wire.ObjectValue(o)
}
