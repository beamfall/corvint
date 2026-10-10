package main

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccompiler"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
)

type docsOptions struct {
	root, mode, source, directory, task, request, inspectPython string
	trusted                                                     bool
}

func parseDocsInvocation(arguments []string) (docsOptions, bool, error) {
	position := commandPositionAfterRoots(arguments)
	if position < 0 || arguments[position] != "docs" {
		return docsOptions{}, false, nil
	}
	options := docsOptions{root: "."}
	for index := 0; index < position; index++ {
		if arguments[index] == "--root" {
			options.root, index = arguments[index+1], index+1
			continue
		}
		options.root = strings.TrimPrefix(arguments[index], "--root=")
	}
	if position+1 >= len(arguments) {
		return options, true, argumentError("docs requires draft or consume")
	}
	options.mode = arguments[position+1]
	if options.mode == "nav" {
		var err error
		options, err = parseTrustedNavArgs(options, arguments[position+2:])
		return options, true, err
	}
	if options.mode != "draft" && options.mode != "consume" {
		return options, true, argumentError("docs requires draft or consume")
	}
	seen := map[string]bool{}
	for index := position + 2; index < len(arguments); index++ {
		flag, value, inline := strings.Cut(arguments[index], "=")
		if flag != "--source" && flag != "--package" && flag != "--task" {
			return options, true, argumentError("unrecognized docs argument: " + flag)
		}
		if seen[flag] {
			return options, true, argumentError("duplicate docs argument: " + flag)
		}
		seen[flag] = true
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return options, true, argumentError("missing docs argument value: " + flag)
			}
			value, index = arguments[index+1], index+1
		}
		switch flag {
		case "--source":
			options.source = value
		case "--package":
			options.directory = value
		case "--task":
			options.task = value
		}
	}
	if options.source == "" || options.directory == "" {
		return options, true, argumentError("docs requires --source and --package")
	}
	if options.mode == "consume" && options.task == "" {
		return options, true, argumentError("docs consume requires --task")
	}
	if options.mode == "draft" && seen["--task"] {
		return options, true, argumentError("docs draft does not accept --task")
	}
	root, err := resolveExplicitRoot(options.root)
	options.root = root
	return options, true, err
}

func runDocs(ctx context.Context, options docsOptions, stdin io.Reader, stdout, stderr io.Writer) int {
	output, err := compileDocs(ctx, options, stdin)
	if err != nil {
		var problem *doccompiler.Error
		if errors.As(err, &problem) {
			err = &gokernel.Error{Code: problem.Code, Message: problem.Message}
		}
		observeUnsupported(options.root, err, options.task)
		emitError(stderr, err)
		return 2
	}
	if _, err := stdout.Write(output); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write docs output"})
		return 2
	}
	return 0
}

func compileDocs(ctx context.Context, options docsOptions, stdin io.Reader) ([]byte, error) {
	ctx = gitstatus.WithIsolation(ctx)
	if options.mode == "nav" {
		if options.inspectPython != "" {
			return doccompiler.InspectTrustedNavigation(ctx, options.inspectPython, options.trusted)
		}
		raw, err := doccompiler.ReadTrustedNavFile(options.request, doccompiler.TrustedNavMaxInput)
		if err != nil {
			return nil, err
		}
		return doccompiler.TrustedNavigation(ctx, options.root, raw, options.trusted)
	}
	var provided []byte
	if options.mode == "consume" {
		input, err := readInputBounded(ctx, stdin, doccompiler.DraftMaxBytes)
		if err != nil {
			return nil, err
		}
		if len(input) > doccompiler.DraftMaxBytes {
			return nil, &doccompiler.Error{Code: "documentation-limit-exceeded", Message: "draft exceeds 64 KiB"}
		}
		provided = input
	}
	index, err := contextindex.BuildContext(ctx, options.root, "")
	if err != nil {
		return nil, err
	}
	draft, err := doccompiler.DraftSources(index, options.source, options.directory)
	if err != nil {
		return nil, err
	}
	if options.mode == "draft" {
		return draft.Markdown, nil
	}
	result, err := doccompiler.ConsumeDraft(draft, provided, options.task)
	if err != nil {
		return nil, err
	}
	// This command freshly built the immutable source index and reconstructed
	// the exact provided draft above; byte equality alone cannot grant this label.
	result["validation"] = "SOURCE_REDERIVED"
	encoded, err := gokernel.CanonicalJSON(result)
	if err != nil {
		return nil, err
	}
	if len(encoded)+1 > doccompiler.DraftMaxBytes {
		return nil, &doccompiler.Error{Code: "documentation-limit-exceeded", Message: "encoded consumption result exceeds 64 KiB"}
	}
	return append(encoded, '\n'), nil
}

const docsHelp = `Experimental source documentation orientation

Usage:
  corvint [--root PATH] docs nav --trusted-project --request FILE
  corvint [--root PATH] docs nav --trusted-project --inspect-python ABSOLUTE_PYTHON
  corvint [--root PATH] docs draft --source PATH --package DIRECTORY
  corvint [--root PATH] docs consume --source PATH --package DIRECTORY --task TEXT < draft.md

Navigation is the separate explicitly trusted pinned MkDocs profile. It stages committed
source, emits an exact proposal patch, and validates navigation by a full candidate reload.
It never applies to source; build NOT_RUN, offline NOT_OBSERVED. See
docs/specs/trusted-project-navigation-v0.md for pins and bounded configuration.

Draft ordinary Markdown from immutable owner-source excerpts and tracked non-test Go
exported-name declarations/import edges. Consume the actual draft after fresh original-source
rederivation. GENERATED orientation; build applicability and behavioral validation UNKNOWN.
Read-only except the bounded self-observation ledger on unsupported-* failures (SOL-V0-007).
No HDC/MkDocs qualification, authority promotion, or persistent generated index.

Prerequisites:
  Use committed sources in a Git repository with HEAD. The owner must be admitted,
  tracked .md text (UTF-8, LF, not generated), with exactly one literal ## Agent digest
  heading outside fenced code. Its preamble and digest together must fit 64 lines
  and 4 KiB; unclosed backtick/tilde fences are refused.
  Package is one repository-relative Go directory, not ., with supported tracked
  non-test exported-name declarations. Dirty worktree edits are not source evidence.

Example from a Corvint checkout with committed sources:
  corvint docs draft --source docs/specs/source-documentation-draft-v0.md --package internal/docmaintain > /tmp/corvint-source-draft.md
  corvint docs consume --source docs/specs/source-documentation-draft-v0.md --package internal/docmaintain --task Preview < /tmp/corvint-source-draft.md

Contract and remaining bounds: docs/specs/source-documentation-draft-v0.md

Separate experimental corpus profile:
  corvint [--root PATH] docs corpus manifest --revision FULL_COMMIT --scope PATH --timestamp RFC3339
  corvint [--root PATH] docs corpus build --manifest INPUT.json
  corvint [--root PATH] docs corpus behavior-adapter --input REQUEST.json [--previous RESULT.json] [--check]
  corvint [--root PATH] docs corpus behavior-provider --input REQUEST-V2.json
  corvint [--root PATH] docs corpus search --artifact CORPUS.json --query TEXT
  corvint [--root PATH] docs corpus render --artifact CORPUS.json
  corvint [--root PATH] docs corpus maintain --artifact CORPUS.json --page PAGE.md [--apply]
Read operations: info, validate, get/trace/related/journey/stability --id ID, locate --path PATH,
coverage, gaps [--id ID]. Optional --limit 1..256. All require --artifact.
Native query/context/impact/affected/test-validity/work observe/propose-wave and CEM
status/verify/report accept --corpus=CORPUS.json. Generated evidence grants no task
or test-selection authority. Read commands print bounded JSON and never collect tests.
Guide: docs/DOCUMENTATION-CORPUS.md; contract: docs/specs/documentation-corpus-v1.md
`

func parseTrustedNavArgs(options docsOptions, args []string) (docsOptions, error) {
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		flag, value, inline := strings.Cut(args[i], "=")
		if seen[flag] {
			return options, argumentError("duplicate navigation argument")
		}
		seen[flag] = true
		switch flag {
		case "--trusted-project":
			if inline {
				return options, argumentError("--trusted-project takes no value")
			}
			options.trusted = true
		case "--request", "--inspect-python":
			if !inline {
				if i+1 >= len(args) || argparseOptionLike(args[i+1]) {
					return options, argumentError("missing navigation request")
				}
				i++
				value = args[i]
			}
			if flag == "--request" {
				options.request = value
			} else {
				options.inspectPython = value
			}
		default:
			return options, argumentError("unrecognized navigation argument")
		}
	}
	if !options.trusted || (options.request == "") == (options.inspectPython == "") {
		return options, argumentError("docs nav requires --trusted-project and exactly one of --request or --inspect-python")
	}
	root, err := resolveExplicitRoot(options.root)
	options.root = root
	return options, err
}
