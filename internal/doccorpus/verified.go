package doccorpus

import (
	"bytes"
	"context"
)

// VerifiedCorpus carries an immutable result of the complete source-pinned
// compiler boundary. It is local process evidence, never a wire certificate or
// producer authentication. Only this package can mint it; the zero value refuses.
type VerifiedCorpus struct {
	canonical []byte
}

func verifiedCorpus(a *Artifact) (VerifiedCorpus, error) {
	raw, err := Encode(a)
	if err != nil {
		return VerifiedCorpus{}, err
	}
	return VerifiedCorpus{raw}, nil
}

// Bytes returns a private copy; caller writes cannot change the verified result.
func (v VerifiedCorpus) Bytes() ([]byte, error) {
	if len(v.canonical) == 0 {
		return nil, fail("compiler-verified corpus required")
	}
	return bytes.Clone(v.canonical), nil
}

// Snapshot decodes a fresh copy for a consumer without exposing token storage.
// Parsing is safe here only because this token was minted at the compiler boundary.
func (v VerifiedCorpus) Snapshot() (*Artifact, error) {
	if len(v.canonical) == 0 {
		return nil, fail("compiler-verified corpus required")
	}
	return ParseArtifact(v.canonical)
}

func BuildVerified(ctx context.Context, root string, m Manifest) (VerifiedCorpus, error) {
	a, err := Build(ctx, root, m)
	if err != nil {
		return VerifiedCorpus{}, err
	}
	return verifiedCorpus(a)
}

func BuildIncrementalVerified(ctx context.Context, root string, m Manifest, ids []ShardIdentity, priorRaw []byte, expected string) (VerifiedCorpus, *IncrementalCache, IncrementalStats, error) {
	a, cache, stats, err := BuildIncremental(ctx, root, m, ids, priorRaw, expected)
	if err != nil {
		return VerifiedCorpus{}, nil, stats, err
	}
	v, err := verifiedCorpus(a)
	return v, cache, stats, err
}

func OpenVerified(ctx context.Context, root string, raw []byte) (VerifiedCorpus, error) {
	a, err := Open(ctx, root, raw)
	if err != nil {
		return VerifiedCorpus{}, err
	}
	return verifiedCorpus(a)
}

// OpenIncrementalVerified rederives the complete prior corpus from pinned
// sources under the same provider/global predicates. Cache admission compares
// every retained record with its normalized original. Original canonical bytes
// must still match; ParseArtifact or a supplied cache digest alone never qualifies.
func OpenIncrementalVerified(ctx context.Context, root string, raw []byte, ids []ShardIdentity, priorRaw []byte, expected string) (VerifiedCorpus, IncrementalStats, error) {
	stats := IncrementalStats{}
	a, err := ParseArtifact(raw)
	if err != nil {
		return VerifiedCorpus{}, stats, err
	}
	v, _, stats, err := BuildIncrementalVerified(ctx, root, a.Manifest, ids, priorRaw, expected)
	if err != nil {
		return VerifiedCorpus{}, stats, err
	}
	if !bytes.Equal(raw, v.canonical) {
		return VerifiedCorpus{}, stats, fail("artifact cannot be rederived from current builder and pinned inputs")
	}
	return v, stats, nil
}
