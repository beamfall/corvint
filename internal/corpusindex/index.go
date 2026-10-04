// Package corpusindex is an experimental immutable companion artifact profile.
// Operator-pinned digests establish byte identity, never source authenticity.
package corpusindex

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"fmt"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"io"
	"os"
)

const Schema = "corvint-corpus-index/1"
const MaxBytes = 256 << 20

type Artifact struct {
	Schema             string                `json:"schema"`
	Corpus             *doccorpus.Artifact   `json:"corpus"`
	Index              *doccorpus.QueryIndex `json:"index"`
	ProducerValidation string                `json:"producer_validation"`
}
type Reader struct {
	artifact *Artifact
	digest   string
}

func Encode(v any) ([]byte, error) {
	b, e := json.Marshal(v, json.Deterministic(true))
	if e != nil {
		return nil, e
	}
	if len(b)+1 > MaxBytes {
		return nil, fmt.Errorf("indexed artifact output bound exceeded")
	}
	return append(b, '\n'), nil
}

// Build rederives the original immutable sources before producing a portable
// artifact. Consumers must pin the digest out-of-band from this producer.
func Build(ctx context.Context, root string, raw []byte) ([]byte, error) {
	a, e := doccorpus.Open(ctx, root, raw)
	if e != nil {
		return nil, e
	}
	return buildSourceValidated(ctx, a)
}

// BuildVerified consumes only an immutable result minted by the source-pinned
// corpus compiler. It cannot admit an arbitrary parsed or caller-mutated Artifact.
func BuildVerified(ctx context.Context, verified doccorpus.VerifiedCorpus) ([]byte, error) {
	a, err := verified.Snapshot()
	if err != nil {
		return nil, err
	}
	return buildSourceValidated(ctx, a)
}

func buildSourceValidated(ctx context.Context, a *doccorpus.Artifact) ([]byte, error) {
	if a.Schema != doccorpus.SchemaV2 {
		return nil, fmt.Errorf("indexed profile requires corpus /2")
	}
	x, e := doccorpus.BuildQueryIndex(ctx, a)
	if e != nil {
		return nil, e
	}
	return Encode(Artifact{Schema, a, x, "source-rederived-at-producer; producer authentication NOT_OBSERVED"})
}
func Open(ctx context.Context, raw []byte, expected string) (*Reader, error) {
	if len(raw) > MaxBytes || !wire.IsSha256(expected) || doccorpus.Digest(raw) != expected {
		return nil, fmt.Errorf("operator index digest missing or mismatched")
	}
	var a Artifact
	if e := json.Unmarshal(raw, &a, json.RejectUnknownMembers(true)); e != nil {
		return nil, fmt.Errorf("invalid closed indexed artifact")
	}
	if a.Schema != Schema || a.Corpus == nil || a.Corpus.Schema != doccorpus.SchemaV2 || a.Index == nil || a.ProducerValidation != "source-rederived-at-producer; producer authentication NOT_OBSERVED" {
		return nil, fmt.Errorf("unsupported indexed profile")
	}
	canonical, e := Encode(a)
	if e != nil || !bytes.Equal(raw, canonical) {
		return nil, fmt.Errorf("noncanonical indexed artifact")
	}
	corpus, e := doccorpus.Encode(a.Corpus)
	if e != nil {
		return nil, e
	}
	validated, e := doccorpus.ParseArtifact(corpus)
	if e != nil {
		return nil, e
	}
	derived, e := doccorpus.BuildQueryIndex(ctx, validated)
	if e != nil {
		return nil, e
	}
	wanted, e := Encode(derived)
	if e != nil {
		return nil, e
	}
	supplied, e := Encode(a.Index)
	if e != nil || !bytes.Equal(wanted, supplied) {
		return nil, fmt.Errorf("indexed offsets or postings differ from corpus")
	}
	validated.RuntimeIndex = derived
	a.Corpus = validated
	a.Index = derived
	return &Reader{&a, expected}, nil
}
func (r *Reader) Digest() string { return r.digest }
func (r *Reader) Query(ctx context.Context, q doccorpus.Request) (doccorpus.Receipt, error) {
	receipt, e := doccorpus.QueryContext(ctx, r.artifact.Corpus, q, "unknown", []string{"historical embedded artifact; no live source or Git access", "operator digest pins bytes; producer authentication and current source validation NOT_OBSERVED"})
	if e != nil {
		return receipt, e
	}
	receipt.Envelope.SourceValidation = "index-digest-validated; source-revalidation-unavailable"
	if q.Operation == "info" || q.Operation == "validate" {
		for _, v := range receipt.Results {
			if m, ok := v.(map[string]any); ok {
				if _, has := m["validation"]; has {
					m["validation"] = "index-digest-validated; source-revalidation-unavailable"
				}
			}
		}
	}
	return receipt, nil
}
func ReadFile(path string, limit int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	s, e := f.Stat()
	if e != nil || !s.Mode().IsRegular() || s.Size() > limit {
		return nil, fmt.Errorf("unsafe or oversized input")
	}
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("input bound exceeded")
	}
	return b, e
}
func ParseRequest(raw []byte) (doccorpus.Request, error) {
	var q doccorpus.Request
	if len(raw) > 16<<10 || json.Unmarshal(raw, &q, json.RejectUnknownMembers(true)) != nil {
		return q, fmt.Errorf("invalid bounded query")
	}
	kind := doccorpus.OperationInput(q.Operation)
	if kind == "unsupported" || kind == "query" && q.Query == "" || kind == "id" && q.ID == "" || kind == "path" && q.Path == "" || kind != "query" && q.Query != "" || kind != "id" && q.Operation != "gaps" && q.ID != "" || kind != "path" && q.Path != "" {
		return q, fmt.Errorf("query arguments do not apply to operation")
	}
	return q, nil
}
