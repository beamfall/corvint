package main

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/gokernel"
)

type corpusOptions struct {
	root, op, manifest, artifact, input, previous, revision, scope, timestamp, query, id, path, page, cem, retirement string
	apply, check                                                                                                      bool
	limit, offset                                                                                                     int
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
	flags := map[string]*string{"--retirement": &o.retirement, "--manifest": &o.manifest, "--artifact": &o.artifact, "--input": &o.input, "--previous": &o.previous, "--revision": &o.revision, "--scope": &o.scope, "--timestamp": &o.timestamp, "--query": &o.query, "--id": &o.id, "--path": &o.path, "--page": &o.page, "--cem": &o.cem}
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
	allowed := map[string]string{
		"manifest": "--revision --scope --timestamp", "build": "--manifest", "render": "--artifact",
		"behavior-adapter":  "--input --previous --check",
		"behavior-provider": "--input",
		"maintain":          "--artifact --page --apply", "cem": "--artifact --cem --id",
		"info": "--artifact --limit", "validate": "--artifact --limit", "search": "--artifact --query --limit",
		"get": "--artifact --id --limit", "trace": "--artifact --id --limit", "related": "--artifact --id --limit",
		"journey": "--artifact --id --limit", "stability": "--artifact --id --limit", "locate": "--artifact --path --limit", "coverage": "--artifact --limit", "gaps": "--artifact --id --limit",
	}
	for _, op := range []string{"concept", "claims", "flow", "dependencies", "recommend-tests", "navigation", "vocabulary", "intent"} {
		allowed[op] = "--artifact --limit --" + doccorpus.OperationInput(op)
	}
	allowed["inventory"] = "--artifact --limit --offset"
	valid, ok := allowed[o.op]
	if strings.Contains(valid, "--limit") {
		valid += " --offset --retirement"
	}
	if !ok {
		return o, true, argumentError("unsupported corpus operation")
	}
	for flag := range seen {
		if !strings.Contains(" "+valid+" ", " "+flag+" ") {
			return o, true, argumentError("option does not apply to corpus operation")
		}
	}
	required := map[string][]string{
		"manifest": {o.revision, o.scope, o.timestamp}, "build": {o.manifest}, "maintain": {o.artifact, o.page}, "cem": {o.artifact, o.cem},
		"behavior-adapter":  {o.input},
		"behavior-provider": {o.input},
		"search":            {o.artifact, o.query}, "locate": {o.artifact, o.path}, "get": {o.artifact, o.id}, "trace": {o.artifact, o.id}, "related": {o.artifact, o.id}, "journey": {o.artifact, o.id}, "stability": {o.artifact, o.id},
	}
	if kind := doccorpus.OperationInput(o.op); kind == "query" {
		required[o.op] = []string{o.artifact, o.query}
	} else if kind == "id" {
		required[o.op] = []string{o.artifact, o.id}
	} else if kind == "path" {
		required[o.op] = []string{o.artifact, o.path}
	}
	values, ok := required[o.op]
	if !ok {
		values = []string{o.artifact}
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return o, true, argumentError("missing required corpus option")
		}
	}
	var err error
	o.root, err = resolveExplicitRoot(o.root)
	return o, true, err
}

// errCorpusCheckRefused marks a behavior-adapter check report that lists
// refusals; the report is still the command output.
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
func compileCorpus(ctx context.Context, o corpusOptions) ([]byte, error) {
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
	if o.op == "cem" {
		data, err := doccorpus.ReadFile(o.root, o.cem)
		if err != nil {
			return nil, err
		}
		receipt, err := doccorpus.CEMProjection(ctx, o.root, a, data, o.id)
		if err != nil {
			return nil, err
		}
		return doccorpus.Encode(receipt)
	}
	fresh, limits, err := doccorpus.Freshness(ctx, o.root, a)
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
	receipt, err := doccorpus.QueryContext(ctx, a, doccorpus.Request{Operation: o.op, Retirement: retirement, Query: o.query, ID: o.id, Path: o.path, Limit: o.limit, Offset: o.offset}, fresh, limits)
	if err != nil {
		return nil, err
	}
	return doccorpus.Encode(receipt)
}
