// Package corpusrepublish is an experimental trusted-local companion. Host
// policy pins authorize local admission; matching bytes do not authenticate a forge.
package corpusrepublish

import (
	"bytes"
	json "encoding/json/v2"
	"fmt"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/postmergeconnector"
)

const Profile = "corvint-corpus-republish/0"
const ResultProfile = "corvint-corpus-republish-result/0"
const MaxBytes = 4 << 20

type Binding struct {
	Repository            string                    `json:"repository"`
	Target                string                    `json:"target"`
	DocumentationRevision string                    `json:"documentation_revision"`
	SourceRevision        string                    `json:"source_revision"`
	ManifestSHA256        string                    `json:"manifest_sha256"`
	BuiltAt               string                    `json:"built_at"`
	ReuseEligibility      []doccorpus.ShardIdentity `json:"reuse_eligibility"`
}
type Request struct {
	Profile     string             `json:"profile"`
	Binding     Binding            `json:"binding"`
	Manifest    doccorpus.Manifest `json:"manifest"`
	EvidenceURL string             `json:"evidence_url"`
}
type Policy struct {
	Profile                   string                    `json:"profile"`
	Binding                   Binding                   `json:"binding"`
	Decision                  string                    `json:"decision"`
	DecisionRecord            string                    `json:"decision_record"`
	PreviousResultSHA256      string                    `json:"previous_result_sha256"`
	PreviousArtifactSHA256    string                    `json:"previous_artifact_sha256"`
	PreviousSidecarProfile    string                    `json:"previous_sidecar_profile"`
	PreviousSidecarSHA256     string                    `json:"previous_sidecar_sha256"`
	ExpectedPendingGeneration uint64                    `json:"expected_pending_generation"`
	ExpectedPendingSHA256     string                    `json:"expected_pending_sha256"`
	AllowedShardRetirements   []doccorpus.ShardIdentity `json:"allowed_shard_retirements"`
	URLOrigins                []string                  `json:"url_origins"`
}
type SidecarPin struct {
	Profile string `json:"profile"`
	SHA256  string `json:"sha256"`
}
type Provenance struct {
	Binding           Binding              `json:"binding"`
	PolicySHA256      string               `json:"policy_sha256"`
	DecisionRecord    string               `json:"decision_record"`
	Builder           doccorpus.Builder    `json:"builder"`
	CompanionRevision string               `json:"companion_revision"`
	GoVersion         string               `json:"go_version"`
	Schemas           []string             `json:"schemas"`
	Providers         []doccorpus.Provider `json:"providers"`
	Inputs            []doccorpus.Input    `json:"inputs"`
	Limits            []string             `json:"limits"`
}
type Delta struct {
	Added   []string `json:"added"`
	Retired []string `json:"retired"`
	Changed []string `json:"changed"`
}
type Parity struct {
	Subjects      Delta                     `json:"subjects"`
	Claims        Delta                     `json:"claims"`
	Relations     Delta                     `json:"relations"`
	Journeys      Delta                     `json:"journeys"`
	Observations  Delta                     `json:"observations"`
	Details       Delta                     `json:"details"`
	Semantics     Delta                     `json:"semantics"`
	Shards        Delta                     `json:"shards"`
	RetiredShards []doccorpus.ShardIdentity `json:"retired_shards"`
}

// ConsumerWitness retains the actual indexed query digest and typed trust envelope.
// Generic receipt results stay in their separately retained original bytes.
type ConsumerWitness struct {
	Operation     string                   `json:"operation"`
	ReceiptSHA256 string                   `json:"receipt_sha256"`
	Envelope      *doccorpus.TrustEnvelope `json:"trust_envelope"`
}
type Result struct {
	Profile                string          `json:"profile"`
	Provenance             Provenance      `json:"provenance"`
	CorpusSHA256           string          `json:"corpus_sha256"`
	IndexSHA256            string          `json:"index_sha256"`
	Parity                 Parity          `json:"parity"`
	ReuseEligibilitySHA256 string          `json:"reuse_eligibility_sha256"`
	IncrementalSidecar     SidecarPin      `json:"incremental_sidecar"`
	Consumer               ConsumerWitness `json:"consumer"`
}
type Output struct {
	Result        Result
	ConsumerBytes []byte
	ResultBytes   []byte
	Corpus        []byte
	Index         []byte
	Sidecar       []byte
	Plan          postmergeconnector.RepublishPlan
	Stats         doccorpus.IncrementalStats
}

func Decode(raw []byte, target any) error {
	if len(raw) > MaxBytes {
		return fmt.Errorf("republish input byte bound exceeded")
	}
	if err := json.Unmarshal(raw, target, json.RejectUnknownMembers(true)); err != nil {
		return fmt.Errorf("republish invalid closed JSON")
	}
	return nil
}
func Encode(v any) ([]byte, error) {
	b, e := json.Marshal(v, json.Deterministic(true))
	if e != nil {
		return nil, e
	}
	if len(b)+1 > MaxBytes {
		return nil, fmt.Errorf("republish output byte bound exceeded")
	}
	return append(b, '\n'), nil
}
func same(a, b any) bool {
	aa, e := Encode(a)
	bb, f := Encode(b)
	return e == nil && f == nil && bytes.Equal(aa, bb)
}
