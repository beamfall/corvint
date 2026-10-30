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
	// raw is at most MaxBytes, so a match is within Encode's bound. The comparisons stream and the
	// re-encodes are pre-sized from raw, which keeps allocation independent of json/v2 encoder pool
	// state (V1-1380).
	if !doccorpus.EncodedEqual(a, raw) {
		return nil, fmt.Errorf("noncanonical indexed artifact")
	}
	corpus, e := doccorpus.EncodeSized(a.Corpus, len(raw))
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
	// The supplied index is a member of raw beside the corpus, so it fits in what raw has left.
	wanted, e := encodeSized(derived, len(raw)-len(corpus)+1)
	if e != nil {
		return nil, e
	}
	if !doccorpus.EncodedEqual(a.Index, wanted) {
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

// ReadFile is ReadFileContext without cancellation.
func ReadFile(path string, limit int64) ([]byte, error) {
	return ReadFileContext(context.Background(), path, limit)
}

// readChunk is how much ReadFileContext reads between cancellation checks.
const readChunk = 1 << 20

// ReadFileContext reads a regular file of at most limit bytes. A path that is
// not a regular file is refused from its metadata before it is opened, and
// the open never blocks (O_NONBLOCK where the platform has it), so a FIFO
// with no writer is refused instead of hanging the caller (V1-3006). The
// opened descriptor must still be that regular file, and ctx is checked
// between bounded chunks.
func ReadFileContext(ctx context.Context, path string, limit int64) ([]byte, error) {
	before, e := os.Stat(path)
	if e != nil {
		return nil, e
	}
	if !before.Mode().IsRegular() || before.Size() > limit {
		return nil, fmt.Errorf("unsafe or oversized input")
	}
	f, e := openInput(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	s, e := f.Stat()
	if e != nil || !s.Mode().IsRegular() || !os.SameFile(before, s) || s.Size() > limit {
		return nil, fmt.Errorf("unsafe or oversized input")
	}
	var b bytes.Buffer
	b.Grow(int(s.Size()))
	bounded := io.LimitReader(f, limit+1)
	for {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		n, e := io.CopyN(&b, bounded, readChunk)
		if int64(b.Len()) > limit {
			return nil, fmt.Errorf("input bound exceeded")
		}
		if e == io.EOF || (e == nil && n < readChunk) {
			return b.Bytes(), nil
		}
		if e != nil {
			return nil, e
		}
	}
}
func ParseRequest(raw []byte) (doccorpus.Request, error) {
	var q doccorpus.Request
	if len(raw) > 16<<10 || json.Unmarshal(raw, &q, json.RejectUnknownMembers(true)) != nil {
		return q, fmt.Errorf("invalid bounded query")
	}
	// The struct loses whether a member was present: an explicitly empty
	// status member is refused as the CLI and MCP refuse it (DCP-V1-318).
	// Query, id and path are always encoded, so an empty one means absent.
	var members map[string]any
	if json.Unmarshal(raw, &members) != nil {
		return q, fmt.Errorf("invalid bounded query")
	}
	for _, name := range []string{"state", "qualifier", "run"} {
		if v, ok := members[name]; ok && v == "" {
			return q, fmt.Errorf("query arguments do not apply to operation")
		}
	}
	kind := doccorpus.OperationInput(q.Operation)
	if q.Operation == "claims" && q.Query != "" {
		// DCP-V1-313: a claim text query replaces the path.
		if q.Path != "" || q.ID != "" || q.State != "" || q.Qualifier != "" || q.Run != "" {
			return q, fmt.Errorf("query arguments do not apply to operation")
		}
		return q, nil
	}
	if q.Operation == "claims" && (q.State != "" || q.Qualifier != "" || q.Run != "") {
		// DCP-V1-318: a status filter replaces the path.
		if q.Path != "" || q.ID != "" || !doccorpus.ValidStatusFilter(q.State, q.Qualifier, q.Run) {
			return q, fmt.Errorf("query arguments do not apply to operation")
		}
		return q, nil
	}
	if q.State != "" || q.Qualifier != "" || q.Run != "" {
		return q, fmt.Errorf("query arguments do not apply to operation")
	}
	if kind == "unsupported" || kind == "query" && q.Query == "" || kind == "id" && q.ID == "" && q.Operation != "intent" || kind == "path" && q.Path == "" || kind != "query" && q.Query != "" || kind != "id" && q.Operation != "gaps" && q.ID != "" || kind != "path" && q.Path != "" {
		return q, fmt.Errorf("query arguments do not apply to operation")
	}
	return q, nil
}

// encodeSized is Encode with its output buffer pre-sized to sizeHint bytes (V1-1380).
func encodeSized(v any, sizeHint int) ([]byte, error) {
	var out bytes.Buffer
	out.Grow(max(sizeHint, 0))
	n, e := doccorpus.EncodeTo(&out, v)
	if e != nil {
		return nil, e
	}
	if n+1 > MaxBytes {
		return nil, fmt.Errorf("indexed artifact output bound exceeded")
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}
