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
	root, op, manifest, artifact, input, previous, revision, scope, timestamp, query, id, path, page, cem, retirement, runner string
	// stable is the cem-stable verifier binding; its artifact root is not a --root-relative input.
	stable        doccorpus.StableCEMInputs
	playwright    playwrightCorpusOptions
	legacyCases   legacyCasesCorpusOptions
	apply, check  bool
	limit, offset int
}

// stringFlags maps each value-taking corpus option to its field.
func (o *corpusOptions) stringFlags() map[string]*string {
	flags := map[string]*string{"--retirement": &o.retirement, "--manifest": &o.manifest, "--artifact": &o.artifact, "--input": &o.input, "--previous": &o.previous, "--revision": &o.revision, "--scope": &o.scope, "--timestamp": &o.timestamp, "--query": &o.query, "--id": &o.id, "--path": &o.path, "--page": &o.page, "--cem": &o.cem, "--runner": &o.runner,
		"--expected-base": &o.stable.ExpectedBase, "--target": &o.stable.Target, "--stable-artifact-root": &o.stable.ArtifactRoot}
	o.playwright.addFlags(flags)
	o.legacyCases.addFlags(flags)
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
	flags := o.stringFlags()
	seen := map[string]bool{}
	for i := pos + 3; i < len(args); i++ {
		flag, value, inline := strings.Cut(args[i], "=")
		if seen[flag] {
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
	var err error
	o.root, err = resolveExplicitRoot(o.root)
	return o, true, err
}

// errCorpusCheckRefused marks a behavior-adapter check report, or a legacy inventory whose
// crosswalk refused a reviewed mapping, that lists refusals; the report is still the command output.
var errCorpusCheckRefused = errors.New("behavior adapter check refused")

func runCorpus(ctx context.Context, o corpusOptions, stdout, stderr io.Writer) int {
	data, err := compileCorpus(ctx, o)
	if errors.Is(err, errCorpusCheckRefused) {
		// DCP-V1-044: a refused check still prints its full report.
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
	return nil
}
func compileCorpus(ctx context.Context, o corpusOptions) ([]byte, error) {
	if err := corpusInputPathError(o); err != nil {
		return nil, err
	}
	if o.op == "manifest" {
		m, err := doccorpus.Inventory(ctx, o.root, o.revision, o.scope, o.timestamp)
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
		a, err := doccorpus.Build(ctx, o.root, m)
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
	raw, err := doccorpus.ReadCorpusFile(o.root, o.artifact)
	if err != nil {
		return nil, err
	}
	a, err := doccorpus.Open(ctx, o.root, raw)
	if err != nil {
		return nil, err
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
	receipt, err := doccorpus.QueryLive(ctx, a, doccorpus.Request{Operation: o.op, Retirement: retirement, Query: o.query, ID: o.id, Path: o.path, Limit: o.limit, Offset: o.offset}, overlay)
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
