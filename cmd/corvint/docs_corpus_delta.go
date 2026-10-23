package main

import (
	"context"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// compileCorpusDelta runs `docs corpus delta` and `docs corpus carry-forward` (DCP-V1-360,
// DCP-V1-366): --previous is a committed corpus artifact read canonically, and the current side
// is exactly one of --artifact (reopened and rederived) or --manifest (built in memory, never
// written). Both read Git objects only.
func compileCorpusDelta(ctx context.Context, o corpusOptions) ([]byte, error) {
	if (o.artifact == "") == (o.manifest == "") {
		return nil, argumentError("docs corpus " + o.op + " requires exactly one of --artifact or --manifest")
	}
	raw, err := doccorpus.ReadCorpusFile(o.root, o.previous)
	if err != nil {
		return nil, err
	}
	previous, err := doccorpus.ParseArtifact(raw)
	if err != nil {
		return nil, err
	}
	current, source, err := openCorpusDeltaCurrent(ctx, o)
	if err != nil {
		return nil, err
	}
	if o.op == "carry-forward" {
		record, err := doccorpus.CarryForward(ctx, o.root, previous, current, o.provider)
		if err != nil {
			return nil, err
		}
		return doccorpus.Encode(record)
	}
	receipt, err := doccorpus.Delta(ctx, o.root, previous, current, source, o.offset, o.limit)
	if err != nil {
		return nil, err
	}
	return doccorpus.Encode(receipt)
}

func openCorpusDeltaCurrent(ctx context.Context, o corpusOptions) (*doccorpus.Artifact, string, error) {
	if o.artifact != "" {
		raw, err := doccorpus.ReadCorpusFile(o.root, o.artifact)
		if err != nil {
			return nil, "", err
		}
		a, err := doccorpus.Open(ctx, o.root, raw)
		return a, "artifact", err
	}
	raw, err := doccorpus.ReadCorpusFile(o.root, o.manifest)
	if err != nil {
		return nil, "", err
	}
	m, err := doccorpus.ParseManifest(raw)
	if err != nil {
		return nil, "", err
	}
	a, err := doccorpus.BuildAdmitted(ctx, o.root, m)
	return a, "manifest", err
}
