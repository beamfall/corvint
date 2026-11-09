package main

import (
	"context"
	"errors"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
)

// errCorpusEvalRegressed marks an eval report whose --baseline ratchet found a regression or
// refused the comparison; the report is still the command output and the exit code is 1
// (DCP-V1-254, DCP-V1-255).
var errCorpusEvalRegressed = errors.New("corpus eval ratchet failed")

// compileCorpusEval scores the case file through the corpus MCP tool registry, so the CLI and
// the served answers share one code path (DCP-V1-251).
func compileCorpusEval(ctx context.Context, o corpusOptions) ([]byte, error) {
	format := o.legacyCases.format
	if format != "" && format != "json" && format != "table" {
		return nil, argumentError("eval --format must be json or table")
	}
	raw, err := doccorpus.ReadFile(o.root, o.cases)
	if err != nil {
		return nil, err
	}
	cases, err := corpusbridge.ParseEvalCases(raw)
	if err != nil {
		return nil, err
	}
	registry, bridgeErr := corpusbridge.New(o.root, o.artifact)
	if bridgeErr != nil {
		return nil, &doccorpus.Error{Code: "corpus-refused", Message: "eval: corpus unavailable (" + bridgeErr.Code + ")"}
	}
	report, err := registry.Evaluate(ctx, cases, o.cases, raw)
	if err != nil {
		return nil, err
	}
	if o.baseline != "" {
		baseline, err := doccorpus.ReadFile(o.root, o.baseline)
		if err != nil {
			return nil, err
		}
		if err := corpusbridge.CompareEval(report, o.baseline, baseline); err != nil {
			return nil, err
		}
	}
	var data []byte
	if format == "table" {
		data = corpusbridge.RenderEvalTable(report)
	} else if data, err = doccorpus.Encode(report); err != nil {
		return nil, err
	}
	if report.Regressed() {
		return data, errCorpusEvalRegressed
	}
	return data, nil
}
