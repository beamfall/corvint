package snapshot

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func readRequestID(r *wire.Reader) string {
	s := r.String()
	if r.Err() != nil {
		return s
	}
	if _, err := mutation.ParseRequestID(r.Where(), s); err != nil {
		r.Fail(wire.CodeOf(err), "%v", err)
	}
	return s
}

// AttemptQueue validates an attempt identity and returns its queue identity.
func AttemptQueue(id string) (wire.QueueID, error) {
	parts := strings.Split(id, ":")
	if len(parts) != 4 || parts[0] != "attempt" || len(parts[3]) != 32 || len(id) > wire.MaxIdentifierBytes {
		return wire.QueueID{}, wire.Errorf(wire.CodeMalformed, id, "invalid attempt identity")
	}
	if _, err := wire.ParseDigest(id, parts[3]+parts[3]); err != nil {
		return wire.QueueID{}, err
	}
	return wire.ParseQueueID(id, "queue:"+parts[1]+":"+parts[2])
}

// PostBound admits only the closed archive-namespace receipt destinations.
// It returns the existing per-file bound, never a caller-chosen policy.
func PostBound(p string) (int, error) {
	if _, err := wire.ParseIdentifier(p, p); err != nil {
		return 0, err
	}
	if strings.Contains(p, "\\") {
		return 0, wire.Errorf(wire.CodeMalformed, p, "backslash in receipt path")
	}
	switch p {
	case "VERSION":
		return len(VersionBytes), nil
	case "barrier.json":
		return wire.MaxBarrierBytes, nil
	case "reservations.json":
		return wire.MaxReservationSetBytes, nil
	}
	if strings.HasPrefix(p, "intent/") {
		if n, ok := intent.BoundFor(strings.TrimPrefix(p, "intent/")); ok {
			return n, nil
		}
	}
	parts := strings.Split(p, "/")
	if len(parts) == 2 {
		name := strings.TrimSuffix(parts[1], ".json")
		switch parts[0] {
		case "attempts":
			if strings.HasSuffix(parts[1], ".json") {
				if _, err := AttemptQueue(name); err == nil {
					return wire.MaxAttemptRecordBytes, nil
				}
			}
		case "effects", "pinned":
			if strings.HasSuffix(parts[1], ".json") {
				if _, err := wire.ParseDigest(p, name); err == nil {
					if parts[0] == "pinned" {
						return wire.MaxPinnedBytes, nil
					}
					return wire.MaxAttemptRecordBytes, nil
				}
			}
		case "evidence":
			if _, err := wire.ParseDigest(p, parts[1]); err == nil {
				return wire.MaxEvidenceBlobBytes, nil
			}
		}
	}
	if len(parts) == 3 && parts[0] == "requests" && strings.HasSuffix(parts[2], ".json") {
		name := strings.TrimSuffix(parts[2], ".json")
		if _, err := wire.ParseDigest(p, name); err == nil && parts[1] == name[:2] {
			return wire.MaxAttemptRecordBytes, nil
		}
	}
	return 0, wire.Errorf(wire.CodeMalformed, p, "forbidden receipt destination")
}

// ContentPath checks identities against consumed bytes, not a manifest claim.
func ContentPath(p string, raw []byte) error {
	var name string
	switch {
	case strings.HasPrefix(p, "pinned/"):
		name = strings.TrimSuffix(strings.TrimPrefix(p, "pinned/"), ".json")
	case strings.HasPrefix(p, "evidence/"):
		name = strings.TrimPrefix(p, "evidence/")
	default:
		return nil
	}
	if name != string(wire.Sum(raw)) {
		return wire.Errorf(wire.CodeJournalForked, p, "content-addressed filename differs from bytes")
	}
	return nil
}

func validateEntries(rc *Receipt) error {
	if rc.Seq == "1" && (rc.Kind != "INIT" || rc.HeadGeneration != "0") {
		return wire.Errorf(wire.CodeMalformed, "/kind", "receipt 1 must be INIT at generation 0")
	}
	if rc.Seq != "1" && rc.Kind == "INIT" {
		return wire.Errorf(wire.CodeMalformed, "/kind", "later INIT forbidden")
	}
	if rc.Kind == "PRUNE" {
		return wire.Errorf(wire.CodeUnsupported, "/kind", "J1 requires retained contiguous history; PRUNE unsupported")
	}
	if rc.AttemptID != nil {
		if _, err := AttemptQueue(*rc.AttemptID); err != nil {
			return err
		}
	}
	if len(rc.Pre) != len(rc.Post) {
		return wire.Errorf(wire.CodeMalformed, "/pre", "pre/post sets must be paired")
	}
	for i, p := range rc.Post {
		bound, err := PostBound(p.Path)
		if err != nil {
			return err
		}
		if rc.Pre[i].Path != p.Path {
			return wire.Errorf(wire.CodeMalformed, p.Path, "unpaired pre/post path")
		}
		if i > 0 && rc.Post[i-1].Path >= p.Path {
			return wire.Errorf(wire.CodeMalformed, p.Path, "paths must be sorted and duplicate-free")
		}
		if p.Sha256 == nil {
			if p.Record != nil || p.BlobSha256 != nil || rc.Kind != "UNPAUSE" || p.Path != "barrier.json" || rc.Pre[i].Sha256 == nil {
				return wire.Errorf(wire.CodeMalformed, p.Path, "only UNPAUSE barrier deletion with a non-null pre is permitted")
			}
			continue
		}
		if p.Record != nil {
			raw := wire.EncodeFile(*p.Record)
			if len(raw) > bound {
				return wire.Errorf(wire.CodeLimitExceeded, p.Path, "post exceeds destination bound")
			}
			if wire.Sum(raw) != *p.Sha256 {
				return wire.Errorf(wire.CodeJournalForked, p.Path, "inline digest mismatch")
			}
			if err := ContentPath(p.Path, raw); err != nil {
				return err
			}
		}
		if p.BlobSha256 != nil && *p.BlobSha256 != *p.Sha256 {
			return wire.Errorf(wire.CodeJournalForked, p.Path, "blob and post digest differ")
		}
		if p.Path == "VERSION" && p.BlobSha256 == nil {
			return wire.Errorf(wire.CodeMalformed, p.Path, "VERSION requires raw blob bytes")
		}
	}
	return nil
}

const ProfileInit = "taskman-init/0"

// Init is the noncircular, pinned genesis identity; head.initSha256 hashes
// receipt 1 instead of this descriptor.
type Init struct {
	QueueID         wire.QueueID
	PrimaryWorktree string
	VersionSha256   wire.Digest
}

func DecodeInit(raw []byte) (*Init, error) {
	// Descriptor PathText can be 4096 bytes, so use the existing pinned bound.
	if len(raw) > wire.MaxPinnedBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "INIT descriptor too large")
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "queueId", "primaryWorktree", "versionSha256")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ProfileInit); err != nil {
		return nil, err
	}
	d := &Init{QueueID: r.Field("queueId").QueueID(), PrimaryWorktree: r.Field("primaryWorktree").PathText(), VersionSha256: r.Field("versionSha256").Digest()}
	return d, r.Err()
}

func (d Init) Value() wire.Value {
	o := wire.NewObject()
	o.Set("profile", wire.String(ProfileInit))
	o.Set("queueId", wire.String(d.QueueID.Raw))
	o.Set("primaryWorktree", wire.String(d.PrimaryWorktree))
	o.Set("versionSha256", wire.String(string(d.VersionSha256)))
	return wire.ObjectValue(o)
}

// Request is receipt-bound by its ordinary post digest; it contains no
// receipt hash and grants no actor authority.
type Request struct {
	Seq   wire.Size
	Entry mutation.IndexEntry
}

func RequestPath(id string) (string, error) {
	if _, err := mutation.ParseRequestID("/requestId", id); err != nil {
		return "", err
	}
	d := string(wire.Sum([]byte(id)))
	return "requests/" + d[:2] + "/" + d + ".json", nil
}

func DecodeRequest(raw []byte) (*Request, error) {
	if len(raw) > wire.MaxAttemptRecordBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "request entry too large")
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed("requestId", "seq", "mutationSha256", "outcome")
	req := &Request{}
	req.Entry.RequestID = readRequestID(r.Field("requestId"))
	req.Seq = r.Field("seq").Size()
	req.Entry.MutationSha256 = r.Field("mutationSha256").Digest()
	if err := r.Err(); err != nil {
		return nil, err
	}
	out, err := mutation.DecodeOutcome(wire.EncodeFile(r.Field("outcome").Value()))
	if err != nil {
		return nil, err
	}
	if req.Seq == "0" || out.ReceiptSeq == nil || *out.ReceiptSeq != req.Seq || out.RequestID != req.Entry.RequestID || out.Replayed {
		return nil, wire.Errorf(wire.CodeMalformed, "/outcome", "request entry must carry its original receipt-bound outcome")
	}
	req.Entry.Outcome = *out
	return req, nil
}
