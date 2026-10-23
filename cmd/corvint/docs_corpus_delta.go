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
	if err := corpusDeltaViewError(o); err != nil {
		return nil, err
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
	options := doccorpus.DeltaOptions{Offset: o.offset, Limit: o.limit, Uncovered: o.uncovered, Anchorless: o.anchorless}
	if o.legacyCases.format == "jsonl" {
		return doccorpus.DeltaStream(ctx, o.root, previous, current, source, options)
	}
	receipt, err := doccorpus.DeltaWith(ctx, o.root, previous, current, source, options)
	if err != nil {
		return nil, err
	}
	return doccorpus.Encode(receipt)
}

// corpusDeltaViewError refuses an invalid delta view before any read (DCP-V1-421..423): the
// uncovered and anchorless modes, the output format, and paging options with the jsonl stream.
func corpusDeltaViewError(o corpusOptions) error {
	switch {
	case o.uncovered != "" && o.uncovered != doccorpus.DeltaUncoveredAll && o.uncovered != doccorpus.DeltaUncoveredChanged:
		return argumentError("delta --uncovered must be changed or all")
	case o.anchorless != "" && o.anchorless != doccorpus.DeltaAnchorlessSummary && o.anchorless != doccorpus.DeltaAnchorlessList:
		return argumentError("delta --anchorless must be summary or list")
	case o.legacyCases.format != "" && o.legacyCases.format != "json" && o.legacyCases.format != "jsonl":
		return argumentError("delta --format must be json or jsonl")
	case o.legacyCases.format == "jsonl" && o.paged:
		return argumentError("delta --format jsonl streams the whole worklist and admits no --limit or --offset")
	}
	return nil
}

// openCorpusDeltaCurrent opens the current side. A delta over a retained artifact still reaches
// classification when a declared additional checkout is unavailable (DCP-V1-425); carry-forward
// emits records for the next build, so it keeps requiring a rederived artifact.
func openCorpusDeltaCurrent(ctx context.Context, o corpusOptions) (*doccorpus.Artifact, string, error) {
	if o.artifact != "" {
		raw, err := doccorpus.ReadCorpusFile(o.root, o.artifact)
		if err != nil {
			return nil, "", err
		}
		if o.op == "delta" {
			a, err := doccorpus.OpenDeltaCurrent(ctx, o.root, raw)
			return a, "artifact", err
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
