package main

import (
	"strconv"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// corpusAuditOptions are the claim-audit sample and report options (DCP-V1-265..268).
type corpusAuditOptions struct {
	seed, size, stratify, sample string
}

func (a *corpusAuditOptions) addFlags(flags map[string]*string) {
	flags["--seed"] = &a.seed
	flags["--size"] = &a.size
	flags["--stratify"] = &a.stratify
	flags["--sample"] = &a.sample
}

// corpusAuditOperation maps the spelled `docs corpus audit sample|report` form to its
// operation name, so help, the parser and the flag table share one entry.
func corpusAuditOperation(args []string) (string, bool) {
	if len(args) > 1 && args[0] == "audit" && (args[1] == "sample" || args[1] == "report") {
		return "audit-" + args[1], true
	}
	return "", false
}

// compileCorpusAudit draws a sample from, or reports verdicts against, a rederived corpus.
func compileCorpusAudit(a *doccorpus.Artifact, o corpusOptions) ([]byte, error) {
	if o.op == "audit-sample" {
		seed, err := strconv.ParseUint(o.audit.seed, 10, 64)
		if err != nil || seed > doccorpus.MaxAuditSeed {
			return nil, argumentError("audit sample --seed must be 0.." + strconv.FormatUint(doccorpus.MaxAuditSeed, 10))
		}
		size, err := strconv.Atoi(o.audit.size)
		if err != nil || size < 1 || size > doccorpus.MaxAuditSample {
			return nil, argumentError("audit sample --size must be 1.." + strconv.Itoa(doccorpus.MaxAuditSample))
		}
		if !doccorpus.ValidAuditStratify(o.audit.stratify) {
			return nil, argumentError("audit sample --stratify must be provider, kind, flow, trust or patterns")
		}
		if len(o.scopes) > 1 {
			return nil, argumentError("audit sample takes one --scope")
		}
		sample, err := doccorpus.SampleClaims(a, doccorpus.AuditSampleOptions{Seed: seed, Size: size, Stratify: o.audit.stratify, Scope: o.scope})
		if err != nil {
			return nil, err
		}
		return doccorpus.Encode(sample)
	}
	format := o.legacyCases.format
	if format != "" && format != "json" && format != "table" {
		return nil, argumentError("audit report --format must be json or table")
	}
	raw, err := doccorpus.ReadFile(o.root, o.audit.sample)
	if err != nil {
		return nil, err
	}
	sample, err := doccorpus.ParseAuditSample(raw)
	if err != nil {
		return nil, err
	}
	report, err := doccorpus.ReportAudit(a, sample, o.audit.sample, raw)
	if err != nil {
		return nil, err
	}
	if format == "table" {
		return doccorpus.RenderAuditTable(report), nil
	}
	return doccorpus.Encode(report)
}

// corpusBuildThresholds parses the opt-in build refusals (DCX-V1-346, DCX-V1-352).
func corpusBuildThresholds(o corpusOptions) (doccorpus.BuildThresholds, error) {
	var t doccorpus.BuildThresholds
	// A given flag is parsed even when empty, so `--flag=` is refused instead of disabling it.
	if o.given["--max-unresolved-mention-rate"] {
		r, err := doccorpus.ParseRate(o.maxMentionRate)
		if err != nil {
			return t, argumentError("--max-unresolved-mention-rate must be a decimal in [0, 1]")
		}
		t.MaxUnresolvedMentionRate = &r
	}
	if o.given["--max-pattern-rate"] {
		rates, err := doccorpus.ParsePatternRates(o.maxPatternRate)
		if err != nil {
			return t, argumentError(err.(*doccorpus.Error).Message)
		}
		t.MaxPatternRates = rates
	}
	return t, nil
}

// compileCorpusPatterns reports the corpus patterns of a rederived corpus (DCX-V1-349).
func compileCorpusPatterns(a *doccorpus.Artifact, o corpusOptions) ([]byte, error) {
	format := o.legacyCases.format
	if format != "" && format != "json" && format != "table" {
		return nil, argumentError("patterns --format must be json or table")
	}
	report := doccorpus.PatternReport(a)
	if format == "table" {
		return doccorpus.RenderPatternTable(report), nil
	}
	return doccorpus.Encode(report)
}
