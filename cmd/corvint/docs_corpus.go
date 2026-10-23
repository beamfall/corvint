package main

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/gokernel"
)

type corpusOptions struct {
	root, op, manifest, artifact, input, previous, revision, scope, timestamp, query, id, path, page, cem, retirement, runner, schema string
	// cases and baseline are the eval case file and ratchet receipt (DCP-V1-250, DCP-V1-254).
	cases, baseline string
	// repository is the locate and recommend-tests qualifier: a declared repository's root-commit
	// identity (DCP-V1-310, DCP-V1-312).
	repository string
	// state, qualifier and run select claims' supplied status claims (DCP-V1-318).
	state, qualifier, run string
	// scopes holds every repeated manifest --scope in argument order (DCP-V1-161).
	scopes []string
	// section and include are the flow filters as comma-separated lists (DCP-V1-170, DCP-V1-171).
	section, include string
	// stable is the cem-stable verifier binding; its artifact root is not a --root-relative input.
	stable        doccorpus.StableCEMInputs
	playwright    playwrightCorpusOptions
	legacyCases   legacyCasesCorpusOptions
	audit         corpusAuditOptions
	apply, check  bool
	limit, offset int
}

// stringFlags maps each value-taking corpus option to its field.
func (o *corpusOptions) stringFlags() map[string]*string {
	flags := map[string]*string{"--retirement": &o.retirement, "--manifest": &o.manifest, "--artifact": &o.artifact, "--input": &o.input, "--previous": &o.previous, "--revision": &o.revision, "--scope": &o.scope, "--timestamp": &o.timestamp, "--query": &o.query, "--id": &o.id, "--path": &o.path, "--page": &o.page, "--cem": &o.cem, "--runner": &o.runner, "--schema": &o.schema, "--section": &o.section, "--include": &o.include, "--cases": &o.cases, "--baseline": &o.baseline, "--repository": &o.repository, "--state": &o.state, "--qualifier": &o.qualifier, "--run": &o.run,
		"--expected-base": &o.stable.ExpectedBase, "--target": &o.stable.Target, "--stable-artifact-root": &o.stable.ArtifactRoot}
	o.playwright.addFlags(flags)
	o.legacyCases.addFlags(flags)
	o.audit.addFlags(flags)
	return flags
}

func parseCorpusInvocation(args []string) (corpusOptions, bool, error) {
	o := corpusOptions{root: ".", limit: 20}
	pos := commandPositionAfterRoots(args)
	if pos < 0 || pos+1 >= len(args) || args[pos] != "docs" || args[pos+1] != "corpus" {
		return o, false, nil
	}
	for i := 0; i < pos; i++ {
		if args[i] == "--root" {
			o.root = args[i+1]
			i++
		} else {
			o.root = strings.TrimPrefix(args[i], "--root=")
		}
	}
	if pos+2 >= len(args) {
		return o, true, argumentError("docs corpus requires an operation")
	}
	o.op = args[pos+2]
	if op, ok := corpusAuditOperation(args[pos+2:]); ok {
		o.op = op
		pos++
	}
	flags := o.stringFlags()
	seen := map[string]bool{}
	for i := pos + 3; i < len(args); i++ {
		flag, value, inline := strings.Cut(args[i], "=")
		if seen[flag] && flag != "--scope" {
			return o, true, argumentError("duplicate corpus option")
		}
		seen[flag] = true
		if flag == "--apply" || flag == "--check" {
			if inline {
				return o, true, argumentError(flag + " takes no value")
			}
			o.apply = o.apply || flag == "--apply"
			o.check = o.check || flag == "--check"
			continue
		}
		if flag == "--limit" || flag == "--offset" {
			if !inline {
				if i+1 >= len(args) {
					return o, true, argumentError("missing corpus limit")
				}
				i++
				value = args[i]
			}
			n, err := strconv.Atoi(value)
			if err != nil || flag == "--limit" && (n < 1 || n > doccorpus.MaxResults) || flag == "--offset" && (n < 0 || n > doccorpus.MaxCorpusRecords*4) {
				return o, true, argumentError("invalid corpus limit")
			}
			if flag == "--limit" {
				o.limit = n
			} else {
				o.offset = n
			}
			continue
		}
		target, ok := flags[flag]
		if !ok {
			return o, true, argumentError("unsupported corpus option")
		}
		if !inline {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return o, true, argumentError("missing corpus option value")
			}
			i++
			value = args[i]
		}
		*target = value
		if flag == "--scope" {
			o.scopes = append(o.scopes, value)
		}
	}
	operation, ok := lookupCorpusOperation(o.op)
	if !ok {
		return o, true, argumentError("unsupported corpus operation")
	}
	for flag := range seen {
		if !operation.admits(flag) {
			return o, true, argumentError("option does not apply to corpus operation")
		}
	}
	for _, flag := range operation.required {
		if strings.TrimSpace(*flags[flag]) == "" {
			return o, true, argumentError("missing required corpus option")
		}
	}
	if seen["--repository"] && o.repository == "" {
		return o, true, argumentError("--repository must name a repository")
	}
	// DCP-V1-318, DCP-V1-319: a present status filter or intent --id is non-empty.
	for _, flag := range []string{"--state", "--qualifier", "--run"} {
		if seen[flag] && strings.TrimSpace(*flags[flag]) == "" {
			return o, true, argumentError(flag + " must not be empty")
		}
	}
	if o.op == "intent" && seen["--id"] && strings.TrimSpace(o.id) == "" {
		return o, true, argumentError("--id must not be empty")
	}
	if seen["--schema"] && o.schema != "1" && o.schema != "2" {
		return o, true, argumentError("--schema must be 1 or 2")
	}
	if !doccorpus.ValidFlowFilter(corpusList(o.section), corpusList(o.include)) {
		return o, true, argumentError("invalid flow section or include filter")
	}
	var err error
	o.root, err = resolveExplicitRoot(o.root)
	return o, true, err
}

// corpusList splits a comma-separated option value; an absent option is nil.
func corpusList(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}

// errCorpusCheckRefused marks a behavior-adapter check report, or a legacy inventory whose
// crosswalk refused a reviewed mapping, that lists refusals; the report is still the command output.
var errCorpusCheckRefused = errors.New("behavior adapter check refused")

func runCorpus(ctx context.Context, o corpusOptions, stdout, stderr io.Writer) int {
	data, err := compileCorpus(ctx, o)
	if errors.Is(err, errCorpusCheckRefused) || errors.Is(err, errCorpusEvalRegressed) {
		// DCP-V1-044: a refused check still prints its full report; so does a failed eval
		// ratchet (DCP-V1-254).
		if _, err := stdout.Write(data); err != nil {
			emitCorpusError(stderr, err)
			return 2
		}
		return 1
	}
	if err != nil {
		emitCorpusError(stderr, err)
		return 2
	}
	if _, err := stdout.Write(data); err != nil {
		emitCorpusError(stderr, err)
		return 2
	}
	return 0
}
func emitCorpusError(w io.Writer, err error) {
	err = doccorpus.Typed(err) // a Git capacity failure is typed, never internal-error (DCP-V1-158)
	var e *doccorpus.Error
	if errors.As(err, &e) {
		emitError(w, &gokernel.Error{Code: e.Code, Message: e.Message})
		return
	}
	emitError(w, err)
}

// corpusInputPathError names the first local-input flag whose path the bounded reader would refuse
// and the rule it breaks (DCP-V1-054). The code stays corpus-refused; what the reader admits is unchanged.
func corpusInputPathError(o corpusOptions) error {
	flags := o.stringFlags()
	for _, flag := range corpusLocalInputFlags {
		value := *flags[flag]
		if value == "" {
			continue
		}
		if rule := doccorpus.LocalInputPathRule(value); rule != "" {
			return &doccorpus.Error{Code: "corpus-refused", Message: flag + ": local input paths must be canonical and relative to --root (" + rule + ")"}
		}
	}
	// --page names the page maintain rewrites, so its refusal names the flag and the broken rule
	// too, before any read; the code is unchanged (DCP-V1-241).
	if o.op == "maintain" && o.page != "" {
		if rule := doccorpus.CorpusPageRule(o.page); rule != "" {
			return &doccorpus.Error{Code: "corpus-refused", Message: "--page: maintenance requires a Markdown page relative to --root outside Git metadata (" + rule + ")"}
		}
	}
	return nil
}
func compileCorpus(ctx context.Context, o corpusOptions) ([]byte, error) {
	if err := corpusInputPathError(o); err != nil {
		return nil, err
	}
	if o.op == "manifest" {
		schema := doccorpus.ManifestSchema
		if o.schema == "2" {
			schema = doccorpus.ManifestSchemaV2
		}
		m, err := doccorpus.InventoryScopes(ctx, o.root, o.revision, o.scopes, schema, o.timestamp)
		if err != nil {
			return nil, err
		}
		return doccorpus.Encode(m)
	}
	if o.op == "build" {
		raw, err := doccorpus.ReadCorpusFile(o.root, o.manifest)
		if err != nil {
			return nil, err
		}
		m, err := doccorpus.ParseManifest(raw)
		if err != nil {
			return nil, err
		}
		a, err := doccorpus.BuildAdmitted(ctx, o.root, m)
		if err != nil {
			return nil, err
		}
		return doccorpus.Encode(a)
	}
	if o.op == "discover-playwright" || o.op == "witness-playwright" {
		return compilePlaywrightCorpus(ctx, o)
	}
	if o.op == "discover-legacy" {
		return compileLegacyCorpus(ctx, o)
	}
	if o.op == "discover-tests" {
		return compileTestDiscovery(ctx, o)
	}
	if o.op == "import-cases" || o.op == "search-cases" {
		return compileLegacyCasesCorpus(o)
	}
	if o.op == "behavior-provider" {
		raw, err := doccorpus.ReadFile(o.root, o.input)
		if err != nil {
			return nil, err
		}
		provider, err := doccorpus.BuildBehaviorProviderV2(raw)
		if err != nil {
			return nil, err
		}
		return doccorpus.Encode(provider)
	}
	if o.op == "behavior-adapter" {
		request, err := doccorpus.ReadFile(o.root, o.input)
		if err != nil {
			return nil, err
		}
		var previous []byte
		if o.previous != "" {
			previous, err = doccorpus.ReadFile(o.root, o.previous)
			if err != nil {
				return nil, err
			}
		}
		if o.check {
			report := doccorpus.CheckBehaviorAdapter(request, previous)
			data, err := doccorpus.Encode(report)
			if err == nil && !report.Accepted {
				err = errCorpusCheckRefused
			}
			return data, err
		}
		result, err := doccorpus.BuildBehaviorAdapter(request, previous)
		if err != nil {
			return nil, err
		}
		return doccorpus.Encode(result)
	}
	if o.op == "eval" {
		return compileCorpusEval(ctx, o)
	}
	raw, err := doccorpus.ReadCorpusFile(o.root, o.artifact)
	if err != nil {
		return nil, err
	}
	a, err := doccorpus.Open(ctx, o.root, raw)
	if err != nil {
		return nil, err
	}
	if o.op == "audit-sample" || o.op == "audit-report" {
		return compileCorpusAudit(a, o)
	}
	if o.op == "render" {
		output, err := doccorpus.Render(a)
		if err != nil {
			return nil, err
		}
		return doccorpus.Encode(output)
	}
	if o.op == "maintain" {
		state, _, err := doccorpus.Freshness(ctx, o.root, a)
		if err != nil {
			return nil, err
		}
		if state != "fresh" {
			return nil, argumentError("maintenance requires fresh corpus evidence")
		}
		output, err := doccorpus.Maintain(o.root, o.page, a, o.apply)
		if err != nil {
			return nil, err
		}
		return doccorpus.Encode(output)
	}
	if o.apply {
		return nil, argumentError("--apply requires corpus maintain")
	}
	if o.op == "cem-stable" {
		return compileStableCorpusCEM(ctx, o, a)
	}
	if o.op == "cem" {
		data, err := doccorpus.ReadFile(o.root, o.cem)
		if err != nil {
			return nil, err
		}
		return nil, doccorpus.RefuseCEMProjection(data)
	}
	// DCP-V1-313, DCP-V1-318: corpus claims takes exactly one of --path, --query or --state;
	// --qualifier and --run refine --state.
	if o.op == "claims" {
		forms := 0
		for _, v := range []string{o.path, o.query, o.state} {
			if strings.TrimSpace(v) != "" {
				forms++
			}
		}
		if forms != 1 {
			return nil, argumentError("corpus claims takes exactly one of --path, --query or --state")
		}
		if o.state == "" && (o.qualifier != "" || o.run != "") || o.state != "" && !doccorpus.ValidStatusFilter(o.state, o.qualifier, o.run) {
			return nil, argumentError("invalid corpus claims status filter")
		}
	}
	overlay, err := doccorpus.LiveFreshness(ctx, o.root, a)
	if err != nil {
		return nil, err
	}
	var retirement *doccorpus.RetirementPolicy
	if o.retirement != "" {
		raw, e := doccorpus.ReadFile(o.root, o.retirement)
		if e != nil {
			return nil, e
		}
		retirement, e = doccorpus.ParseRetirement(raw)
		if e != nil {
			return nil, e
		}
	}
	receipt, err := doccorpus.QueryLive(ctx, a, doccorpus.Request{Operation: o.op, Retirement: retirement, Query: o.query, ID: o.id, Path: o.path, Limit: o.limit, Offset: o.offset, Sections: corpusList(o.section), Include: corpusList(o.include), Repository: o.repository, State: o.state, Qualifier: o.qualifier, Run: o.run}, overlay)
	if err != nil {
		return nil, err
	}
	return doccorpus.Encode(receipt)
}

// compileStableCorpusCEM is the explicit cem/1.0 documentation projection
// (CEM-V1-162, CEM-V1-163). The repository is --root; the map is admitted only
// on a stable verifier ACCEPT, and a refusal exits 2 with no stdout, so the
// 0.x `cem` operation is never relabelled stable.
func compileStableCorpusCEM(ctx context.Context, o corpusOptions, a *doccorpus.Artifact) ([]byte, error) {
	data, err := doccorpus.ReadFile(o.root, o.cem)
	if err != nil {
		return nil, err
	}
	inputs := o.stable
	inputs.Repository = o.root
	if inputs.ArtifactRoot, err = filepath.Abs(inputs.ArtifactRoot); err != nil {
		return nil, argumentError("cannot resolve --stable-artifact-root")
	}
	receipt, err := doccorpus.StableCEMProjection(ctx, a, data, o.id, inputs)
	if err != nil {
		return nil, err
	}
	return doccorpus.Encode(receipt)
}
