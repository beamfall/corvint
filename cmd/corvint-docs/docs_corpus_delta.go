package main

import (
	"context"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// compileCorpusDelta runs `docs corpus delta` and `docs corpus carry-forward` (DCX-V1-360,
// DCX-V1-366): --previous is a committed corpus artifact read canonically, and the current side
// is exactly one of --artifact or --manifest (built in memory, never written). A delta admits
// --artifact through the verified read and rederives it only with --verify-rederive; carry-forward
// rederives it (DCR-V1-460). Both read Git objects only.
func compileCorpusDelta(ctx context.Context, o corpusOptions) ([]byte, error) {
	// Presence, not value, decides, as for build (DCX-V1-563): an empty --manifest= beside
	// --artifact is still both, and an empty value alone is none.
	artifact := o.given["--artifact"]
	if artifact == o.given["--manifest"] || artifact && o.artifact == "" || !artifact && o.manifest == "" {
		return nil, argumentError("docs corpus " + o.op + " requires exactly one of --artifact or --manifest")
	}
	if err := corpusDeltaViewError(o); err != nil {
		return nil, err
	}
	t := o.readTimings
	if t != nil {
		t.Delta = true
	}
	step := time.Now()
	raw, err := doccorpus.ReadCorpusFile(o.root, o.previous)
	if t != nil {
		t.Parse += time.Since(step)
	}
	if err != nil {
		return nil, err
	}
	// The previous side decodes while the current side opens; a previous refusal still wins
	// (DCR-V1-461). Its own timings are added after the join.
	var previous *doccorpus.Artifact
	var previousTimings doccorpus.ReadTimings
	var wg sync.WaitGroup
	wg.Go(func() { previous, err = doccorpus.OpenDeltaPrevious(raw, &previousTimings) })
	current, source, currentErr := openCorpusDeltaCurrent(ctx, o)
	wg.Wait()
	if t != nil {
		t.Parse += previousTimings.Parse
	}
	if err != nil {
		return nil, err
	}
	if currentErr != nil {
		return nil, currentErr
	}
	if o.op == "carry-forward" {
		record, err := doccorpus.CarryForward(ctx, o.root, previous, current, o.provider)
		if err != nil {
			return nil, err
		}
		return doccorpus.Encode(record)
	}
	options := doccorpus.DeltaOptions{Offset: o.offset, Limit: o.limit, Uncovered: o.uncovered, Anchorless: o.anchorless, Timings: t}
	if o.legacyCases.format == "jsonl" {
		return doccorpus.DeltaStream(ctx, o.root, previous, current, source, options)
	}
	receipt, err := doccorpus.DeltaWith(ctx, o.root, previous, current, source, options)
	if err != nil {
		return nil, err
	}
	step = time.Now()
	data, err := doccorpus.Encode(receipt)
	if t != nil {
		t.Encode += time.Since(step)
	}
	return data, err
}

// corpusDeltaViewError refuses an invalid delta view before any read (DCX-V1-421..423): the
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

// openCorpusDeltaCurrent opens the current side. A delta over a retained artifact takes the
// verified read, or the rederivation with --verify-rederive (DCR-V1-460), and still reaches
// classification when a declared additional checkout is unavailable (DCX-V1-425); carry-forward
// emits records for the next build, so it keeps requiring a rederived artifact.
func openCorpusDeltaCurrent(ctx context.Context, o corpusOptions) (*doccorpus.Artifact, string, error) {
	if o.artifact != "" {
		step := time.Now()
		raw, err := doccorpus.ReadCorpusFile(o.root, o.artifact)
		if o.readTimings != nil {
			o.readTimings.Parse += time.Since(step)
		}
		if err != nil {
			return nil, "", err
		}
		if o.op == "delta" {
			a, err := doccorpus.OpenDeltaRead(ctx, o.root, raw, doccorpus.ReadOptions{Rederive: o.verifyRederive, Timings: o.readTimings})
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
	step := time.Now()
	a, err := doccorpus.BuildAdmitted(ctx, o.root, m)
	if o.readTimings != nil {
		o.readTimings.Verify += time.Since(step)
		o.readTimings.Validation = doccorpus.ValidationManifestBuilt
	}
	return a, "manifest", err
}
