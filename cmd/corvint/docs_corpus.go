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
	root, op, manifest, artifact, revision, scope, timestamp, query, id, path, page, cem string
	apply                                                                                bool
	limit                                                                                int
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
	flags := map[string]*string{"--manifest": &o.manifest, "--artifact": &o.artifact, "--revision": &o.revision, "--scope": &o.scope, "--timestamp": &o.timestamp, "--query": &o.query, "--id": &o.id, "--path": &o.path, "--page": &o.page, "--cem": &o.cem}
	seen := map[string]bool{}
	for i := pos + 3; i < len(args); i++ {
		flag, value, inline := strings.Cut(args[i], "=")
		if seen[flag] {
			return o, true, argumentError("duplicate corpus option")
		}
		seen[flag] = true
		if flag == "--apply" {
			if inline {
				return o, true, argumentError("--apply takes no value")
			}
			o.apply = true
			continue
		}
		if flag == "--limit" {
			if !inline {
				if i+1 >= len(args) {
					return o, true, argumentError("missing corpus limit")
				}
				i++
				value = args[i]
			}
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > doccorpus.MaxResults {
				return o, true, argumentError("invalid corpus limit")
			}
			o.limit = n
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
		"maintain": "--artifact --page --apply", "cem": "--artifact --cem --id",
		"info": "--artifact --limit", "validate": "--artifact --limit", "search": "--artifact --query --limit",
		"get": "--artifact --id --limit", "trace": "--artifact --id --limit", "related": "--artifact --id --limit",
		"journey": "--artifact --id --limit", "locate": "--artifact --path --limit", "coverage": "--artifact --limit", "gaps": "--artifact --id --limit",
	}
	valid, ok := allowed[o.op]
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
		"search": {o.artifact, o.query}, "locate": {o.artifact, o.path}, "get": {o.artifact, o.id}, "trace": {o.artifact, o.id}, "related": {o.artifact, o.id}, "journey": {o.artifact, o.id},
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
func runCorpus(ctx context.Context, o corpusOptions, stdout, stderr io.Writer) int {
	data, err := compileCorpus(ctx, o)
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
		raw, err := doccorpus.ReadFile(o.root, o.manifest)
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
	raw, err := doccorpus.ReadFile(o.root, o.artifact)
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
	receipt, err := doccorpus.Query(a, doccorpus.Request{Operation: o.op, Query: o.query, ID: o.id, Path: o.path, Limit: o.limit}, fresh, limits)
	if err != nil {
		return nil, err
	}
	return doccorpus.Encode(receipt)
}
