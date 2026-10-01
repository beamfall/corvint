package doccorpus

import (
	"github.com/Beamfall/corvint/internal/cem/wire"
	"time"
)

// RetirementPolicy is an operator declaration, never an authenticated review.
// Distance names both revisions; absent measurements remain unknown.
type RetirementPolicy struct {
	EvaluatedAt      string `json:"evaluated_at"`
	MaxAgeSeconds    *int64 `json:"max_age_seconds,omitempty"`
	MaxMergeDistance *int   `json:"max_merge_distance,omitempty"`
	SourceRevision   string `json:"source_revision,omitempty"`
	TargetRevision   string `json:"target_revision,omitempty"`
	MergeDistance    *int   `json:"merge_distance,omitempty"`
}
type RetirementStatus struct {
	State         string            `json:"state"`
	AgeSeconds    *int64            `json:"age_seconds,omitempty"`
	MergeDistance *int              `json:"merge_distance,omitempty"`
	Policy        *RetirementPolicy `json:"policy,omitempty"`
	Authority     string            `json:"authority"`
	Reasons       []string          `json:"reasons"`
}
type TrustEnvelope struct {
	ContentStatus    string           `json:"content_status"`
	SourceRevision   string           `json:"source_revision"`
	CorpusRevision   string           `json:"corpus_revision"`
	Builder          Builder          `json:"builder"`
	Freshness        string           `json:"freshness"`
	SourceValidation string           `json:"source_validation"`
	Retirement       RetirementStatus `json:"retirement"`
	Citations        []Anchor         `json:"citations"`
	Limitations      []string         `json:"limitations"`
}

func trustEnvelope(a *Artifact, freshness string, policy *RetirementPolicy) (*TrustEnvelope, error) {
	e := &TrustEnvelope{ContentStatus: "generated", SourceRevision: a.Manifest.Repository.Revision, CorpusRevision: a.SHA256, Builder: a.Builder, Freshness: freshness, SourceValidation: "caller-provided-validated-artifact", Retirement: RetirementStatus{State: "unknown", Authority: "NOT_OBSERVED", Reasons: []string{"retirement policy not supplied"}}, Citations: []Anchor{}, Limitations: []string{"imported review labels are attributed declarations; semantic truth and review authentication NOT_OBSERVED"}}
	if policy == nil {
		return e, nil
	}
	evaluated, err := time.Parse(time.RFC3339, policy.EvaluatedAt)
	if err != nil || policy.MaxAgeSeconds != nil && (*policy.MaxAgeSeconds < 0 || *policy.MaxAgeSeconds > 3155760000) || policy.MaxMergeDistance != nil && (*policy.MaxMergeDistance < 0 || *policy.MaxMergeDistance > 1000000) {
		return nil, fail("invalid retirement policy")
	}
	e.Retirement = RetirementStatus{State: "unknown", Policy: policy, Authority: "caller-declared; authentication NOT_OBSERVED", Reasons: []string{}}
	measured := false
	retired := false
	if policy.MaxAgeSeconds != nil {
		built, err := time.Parse(time.RFC3339, a.Manifest.BuiltAt)
		if err != nil || evaluated.Before(built) {
			return nil, fail("retirement evaluation precedes or cannot bind build time")
		}
		age := evaluated.Unix() - built.Unix()
		e.Retirement.AgeSeconds = &age
		measured = true
		if age > *policy.MaxAgeSeconds {
			retired = true
			e.Retirement.Reasons = append(e.Retirement.Reasons, "maximum declared age exceeded")
		}
	}
	if policy.MergeDistance != nil {
		if policy.MaxMergeDistance == nil || *policy.MergeDistance < 0 || *policy.MergeDistance > 1000000 || policy.SourceRevision != a.Manifest.Repository.Revision || !wire.IsGitOid(policy.TargetRevision) {
			return nil, fail("unbound retirement merge distance")
		}
		if policy.SourceRevision == policy.TargetRevision && *policy.MergeDistance != 0 {
			return nil, fail("same revision has nonzero merge distance")
		}
		e.Retirement.MergeDistance = policy.MergeDistance
		measured = true
		if *policy.MergeDistance > *policy.MaxMergeDistance {
			retired = true
			e.Retirement.Reasons = append(e.Retirement.Reasons, "maximum declared merge distance exceeded")
		}
	} else if policy.MaxMergeDistance != nil {
		e.Retirement.Reasons = append(e.Retirement.Reasons, "merge distance NOT_OBSERVED")
	}
	if retired {
		e.Retirement.State = "retired-by-policy"
	} else if measured && (policy.MaxMergeDistance == nil || policy.MergeDistance != nil) {
		e.Retirement.State = "within-declared-policy"
	}
	return e, nil
}
func finalizeEnvelope(r *Receipt) {
	if r.Envelope == nil {
		return
	}
	r.Envelope.Citations = append([]Anchor{}, r.Citations...)
	r.Envelope.Limitations = append(r.Envelope.Limitations, r.Limitations...)
	if r.State == "empty" && r.Miss != "page-exhausted" {
		r.Meaning = "not documented"
	}
	if r.State == "unavailable" {
		r.Meaning = "documentation capability unavailable"
	}
}

func RetirementInputSchema() map[string]any {
	return map[string]any{"type": "object", "required": []string{"evaluated_at"}, "additionalProperties": false, "properties": map[string]any{
		"evaluated_at":       map[string]any{"type": "string", "maxLength": 64},
		"max_age_seconds":    map[string]any{"type": "integer", "minimum": 0, "maximum": 3155760000},
		"max_merge_distance": map[string]any{"type": "integer", "minimum": 0, "maximum": 1000000},
		"merge_distance":     map[string]any{"type": "integer", "minimum": 0, "maximum": 1000000},
		"source_revision":    map[string]any{"type": "string", "pattern": "^[0-9a-f]{40,64}$"},
		"target_revision":    map[string]any{"type": "string", "pattern": "^[0-9a-f]{40,64}$"},
	}}
}
func ParseRetirement(raw []byte) (*RetirementPolicy, error) {
	var p RetirementPolicy
	if err := decodeBounded(raw, &p, 16<<10); err != nil {
		return nil, err
	}
	return &p, nil
}
