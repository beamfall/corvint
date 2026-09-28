package main

import (
	"context"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/flowdocs"
	"github.com/Beamfall/corvint/internal/gitstatus"
)

type docsFlowsOptions struct{ root, mode, revision, scope, output, corpus, previous, providerRevision, providerPath string }

func parseDocsFlowsInvocation(arguments []string) (docsFlowsOptions, bool, error) {
	pos := commandPositionAfterRoots(arguments)
	o := docsFlowsOptions{root: "."}
	if pos < 0 || arguments[pos] != "docs" || len(arguments) <= pos+1 || arguments[pos+1] != "flows" {
		return o, false, nil
	}
	for i := 0; i < pos; i++ {
		if arguments[i] == "--root" {
			o.root = arguments[i+1]
			i++
		} else {
			o.root = strings.TrimPrefix(arguments[i], "--root=")
		}
	}
	if len(arguments) <= pos+2 {
		return o, true, argumentError("docs flows requires generate, check or finalize")
	}
	o.mode = arguments[pos+2]
	allowed := map[string]map[string]bool{
		"generate": {"--revision": true, "--scope": true, "--output-dir": true, "--corpus": true, "--previous": true},
		"check":    {"--revision": true, "--previous": true, "--output-dir": true},
		"finalize": {"--previous": true, "--provider-revision": true, "--provider-path": true},
	}
	if allowed[o.mode] == nil {
		return o, true, argumentError("unknown docs flows mode")
	}
	seen := map[string]bool{}
	for i := pos + 3; i < len(arguments); i++ {
		flag, value, inline := strings.Cut(arguments[i], "=")
		if !allowed[o.mode][flag] || seen[flag] {
			return o, true, argumentError("unknown or duplicate docs flows flag")
		}
		seen[flag] = true
		if !inline {
			if i+1 >= len(arguments) || argparseOptionLike(arguments[i+1]) {
				return o, true, argumentError("missing docs flows flag value")
			}
			i++
			value = arguments[i]
		}
		if value == "" {
			return o, true, argumentError("empty docs flows flag value")
		}
		switch flag {
		case "--revision":
			o.revision = value
		case "--scope":
			o.scope = value
		case "--output-dir":
			o.output = value
		case "--corpus":
			o.corpus = value
		case "--previous":
			o.previous = value
		case "--provider-revision":
			o.providerRevision = value
		case "--provider-path":
			o.providerPath = value
		}
	}
	if o.mode == "generate" && (o.revision == "" || o.scope == "" || o.output == "") || o.mode == "check" && (o.revision == "" || o.previous == "") || o.mode == "finalize" && (o.previous == "" || o.providerRevision == "" || o.providerPath == "") {
		return o, true, argumentError("missing required docs flows flags")
	}
	root, err := resolveExplicitRoot(o.root)
	o.root = root
	return o, true, err
}
func runDocsFlows(ctx context.Context, o docsFlowsOptions, stdout, stderr io.Writer) int {
	ctx = gitstatus.WithIsolation(ctx)
	var value any
	var err error
	var previous []byte
	exit := 0
	if o.previous != "" {
		previous, err = flowdocs.ReadFile(o.previous)
	}
	if err == nil {
		switch o.mode {
		case "generate":
			var corpus []byte
			if o.corpus != "" {
				corpus, err = flowdocs.ReadCorpusFile(o.corpus)
			}
			if err == nil && len(previous) > 0 {
				var old *flowdocs.Result
				old, err = flowdocs.Open(ctx, o.root, previous)
				if err == nil && old.Manifest.Scope != o.scope {
					err = argumentError("previous generation scope differs")
				}
			}
			if err == nil {
				var r *flowdocs.Result
				r, err = flowdocs.Generate(ctx, o.root, flowdocs.Options{Revision: o.revision, Scope: o.scope, Corpus: corpus})
				if err == nil {
					err = flowdocs.Materialize(o.output, r)
					value = r.Manifest
				}
			}
		case "check":
			var check flowdocs.Check
			check, err = flowdocs.CheckRevision(ctx, o.root, previous, o.revision, o.output)
			value = check
			if !check.Clean {
				exit = 1
			}
		case "finalize":
			var r *flowdocs.Result
			r, err = flowdocs.Open(ctx, o.root, previous)
			if err == nil {
				value, err = flowdocs.Finalize(ctx, o.root, r, o.providerRevision, o.providerPath)
			}
		}
	}
	if err == nil {
		var raw []byte
		raw, err = flowdocs.Encode(value)
		if err == nil {
			_, err = stdout.Write(append(raw, '\n'))
		}
	}
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	return exit
}

const docsFlowsHelp = `Generate bounded source-derived flow documentation (generated trust).

Usage:
  corvint [--root PATH] docs flows generate --revision FULL_SHA --scope PATH --output-dir FRESH_DIR [--corpus FILE] [--previous FILE]
  corvint [--root PATH] docs flows check --revision FULL_SHA --previous FILE [--output-dir DIR]
  corvint [--root PATH] docs flows finalize --previous FILE --provider-revision FULL_SHA --provider-path PATH

Generate exclusively writes a new output directory: eight Markdown pages per flow,
provider.json and generation.json. Ruby/Rails, JS/TS, React JSX and literal Angular
bindings are bounded lexical observations, never accepted intent or runtime proof.
Check and finalize are read-only. Check reports each stable claim/paragraph's
fresh/stale/unresolved source binding and optional byte drift (exit 1).
Finalize admits separately committed provider bytes without changing source anchors.
See docs/FLOW-DOCUMENTATION.md for limits and original corpus/runtime joins.
`
