package main

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/docmaintain"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// docsMaintainOptions parses `corvint docs maintain`, an explicitly enabled,
// bounded local session (docs/specs/source-documentation-draft-v0.md
// SDD-V0-007+) that refreshes one page's generated blocks over the same
// native doccompiler.DraftSources call `docs draft` uses. It is a separate,
// experimental verb: it never runs unless both --enable and --preview/--apply
// are given, and --preview (never --apply) is the default a caller should
// reach for first.
type docsMaintainOptions struct {
	root, page, source, pkg string
	enable, apply, watch    bool
	check                   bool
	maxWrites               int
	maxWallClock            time.Duration
}

func parseDocsMaintainInvocation(arguments []string) (docsMaintainOptions, bool, error) {
	position := commandPositionAfterRoots(arguments)
	if position < 0 || arguments[position] != "docs" || position+1 >= len(arguments) || arguments[position+1] != "maintain" {
		return docsMaintainOptions{}, false, nil
	}
	options := docsMaintainOptions{root: ".", maxWrites: 8, maxWallClock: time.Minute}
	for index := 0; index < position; index++ {
		if arguments[index] == "--root" {
			options.root, index = arguments[index+1], index+1
			continue
		}
		options.root = strings.TrimPrefix(arguments[index], "--root=")
	}
	seen := map[string]bool{}
	previewOrApplySeen := false
	for index := position + 2; index < len(arguments); index++ {
		flag, value, inline := strings.Cut(arguments[index], "=")
		if flag == "--enable" || flag == "--preview" || flag == "--apply" || flag == "--watch" || flag == "--check" {
			if inline || seen[flag] {
				return options, true, argumentError("duplicate or valued docs maintain boolean: " + flag)
			}
			seen[flag] = true
		}
		switch flag {
		case "--check":
			options.check = true
			continue
		case "--watch":
			options.watch = true
			continue
		case "--enable":
			options.enable = true
			continue
		case "--preview":
			options.apply, previewOrApplySeen = false, true
			continue
		case "--apply":
			options.apply, previewOrApplySeen = true, true
			continue
		}
		if flag != "--page" && flag != "--source" && flag != "--package" && flag != "--max-writes" && flag != "--max-wall-clock" {
			return options, true, argumentError("unrecognized docs maintain argument: " + flag)
		}
		if seen[flag] {
			return options, true, argumentError("duplicate docs maintain argument: " + flag)
		}
		seen[flag] = true
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return options, true, argumentError("missing docs maintain argument value: " + flag)
			}
			value, index = arguments[index+1], index+1
		}
		switch flag {
		case "--page":
			options.page = value
		case "--source":
			options.source = value
		case "--package":
			options.pkg = value
		case "--max-writes":
			count, convErr := strconv.Atoi(value)
			if convErr != nil || count <= 0 {
				return options, true, argumentError("--max-writes must be a positive integer")
			}
			options.maxWrites = count
		case "--max-wall-clock":
			duration, convErr := time.ParseDuration(value)
			if convErr != nil || duration <= 0 {
				return options, true, argumentError("--max-wall-clock must be a positive duration")
			}
			options.maxWallClock = duration
		}
	}
	if options.check {
		// DMF-V1-003: a read-only check needs no --enable and takes no session flags.
		if previewOrApplySeen || options.watch || options.enable || seen["--max-writes"] || seen["--max-wall-clock"] {
			return options, true, argumentError("docs maintain --check takes only --page and an optional --source with --package")
		}
		if options.page == "" || (options.source == "") != (options.pkg == "") {
			return options, true, argumentError("docs maintain --check requires --page, and --source with --package to check one block")
		}
		root, err := resolveExplicitRoot(options.root)
		options.root = root
		return options, true, err
	}
	if options.page == "" || options.source == "" || options.pkg == "" {
		return options, true, argumentError("docs maintain requires --page, --source and --package")
	}
	if !options.enable {
		return options, true, argumentError("docs maintain requires --enable")
	}
	if !previewOrApplySeen {
		return options, true, argumentError("docs maintain requires --preview or --apply")
	}
	if seen["--preview"] && seen["--apply"] {
		return options, true, argumentError("docs maintain requires exactly one of --preview or --apply")
	}
	if options.watch && !options.apply {
		return options, true, argumentError("--watch requires --apply")
	}
	if options.watch && (options.maxWrites > 1024 || options.maxWallClock > 24*time.Hour) {
		return options, true, argumentError("--watch requires --max-writes <= 1024 and --max-wall-clock <= 24h")
	}
	root, err := resolveExplicitRoot(options.root)
	options.root = root
	return options, true, err
}

func runDocsMaintain(ctx context.Context, options docsMaintainOptions, stdout, stderr io.Writer) int {
	policy := docmaintain.Policy{
		Enabled: options.enable, Apply: options.apply,
		MaxWrites: options.maxWrites, MaxWallClock: options.maxWallClock,
	}
	selectors := []docmaintain.Selector{{Source: options.source, Package: options.pkg}}
	if options.check {
		return runDocsMaintainCheck(ctx, options, stdout, stderr)
	}
	if options.watch {
		return runDocsWatch(ctx, options, policy, stdout, stderr)
	}
	result, err := docmaintain.Run(ctx, options.root, options.page, selectors, policy)
	if err != nil {
		// A draft limit refusal's message names the bound, its value and the observed size (V1-2416).
		code, message := "internal-error", err.Error()
		if refusal, ok := err.(*docmaintain.Refusal); ok {
			code, message = refusal.Code, cmp.Or(refusal.Message, message)
		}
		emitError(stderr, &gokernel.Error{Code: code, Message: message})
		if result == nil {
			return 2
		}
	}
	encoded, encodeErr := gokernel.CanonicalJSON(maintainReceiptObject(result))
	if encodeErr != nil {
		emitError(stderr, &gokernel.Error{Code: "internal-error", Message: "could not encode maintenance receipt"})
		return 1
	}
	if _, writeErr := fmt.Fprintf(stdout, "%s\n", encoded); writeErr != nil {
		return 1
	}
	if err != nil {
		return 2
	}
	return 0
}

// docsMaintainStale is the error code of a `docs maintain --check` that found a stale block.
const docsMaintainStale = "docs-maintain-stale"

// runDocsMaintainCheck prints the corvint-docmaintain-check/0 report and exits 1 when any selected
// block is stale, 0 when none is, and 2 when the check cannot run (DMF-V1-003). It writes nothing.
func runDocsMaintainCheck(ctx context.Context, options docsMaintainOptions, stdout, stderr io.Writer) int {
	page := docmaintain.CheckPage{Page: options.page}
	if options.source != "" {
		page.Selectors = []docmaintain.Selector{{Source: options.source, Package: options.pkg}}
	}
	report, err := docmaintain.Check(ctx, options.root, []docmaintain.CheckPage{page})
	if err != nil {
		code, message := "internal-error", err.Error()
		if refusal, ok := err.(*docmaintain.Refusal); ok {
			code, message = refusal.Code, cmp.Or(refusal.Message, message)
		}
		emitError(stderr, &gokernel.Error{Code: code, Message: message})
		return 2
	}
	encoded, err := gokernel.CanonicalJSON(map[string]any{"profile": "corvint-docmaintain-check/0", "commit": report.Commit, "blocks": report.Blocks, "stale": report.Stale, "worktree": report.Worktree})
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "internal-error", Message: "could not encode the maintenance check"})
		return 2
	}
	if _, err = fmt.Fprintf(stdout, "%s\n", encoded); err != nil {
		return 2
	}
	if report.Stale == 0 {
		return 0
	}
	names := []string{}
	for _, b := range report.Blocks {
		if b.State != docmaintain.BlockFresh {
			names = append(names, fmt.Sprintf("%s:%d source=%s package=%s %s (%s)", b.Page, b.Line, b.Source, b.Package, b.State, b.Reason))
		}
	}
	emitError(stderr, &gokernel.Error{Code: docsMaintainStale, Message: fmt.Sprintf("%d stale generated block(s): %s; refresh with docs maintain --enable --apply", report.Stale, strings.Join(names, "; "))})
	return 1
}

func maintainReceiptObject(result *docmaintain.Result) map[string]any {
	if result == nil {
		return map[string]any{"profile": "corvint-docmaintain-receipt/0"}
	}
	receipt := result.Receipt
	blocks := make([]map[string]any, 0, len(receipt.Blocks))
	for _, block := range receipt.Blocks {
		blocks = append(blocks, map[string]any{
			"source": block.Selector.Source, "package": block.Selector.Package,
			"existed": block.Existed, "eligible": block.Eligible, "applied": block.Applied,
			"skipped": block.Skipped, "old_source_digest": block.OldSHA256, "new_source_digest": block.NewSHA256,
			"commit": block.Commit, "tree": block.Tree,
		})
	}
	return map[string]any{
		"profile": "corvint-docmaintain-receipt/0",
		"page":    receipt.Page, "applied": receipt.Applied, "preview_only": receipt.PreviewOnly,
		"conflict": receipt.Conflict, "conflict_detail": receipt.ConflictDetail,
		"page_existed_at_start": receipt.PageExistedAtStart,
		"page_digest_before":    receipt.PageDigestBefore, "page_digest_after": receipt.PageDigestAfter,
		"stopped_reason": receipt.StoppedReason, "blocks": blocks, "diff": result.Diff,
	}
}

const docsMaintainHelp = `Experimental automatic maintenance of one human documentation page

Usage:
  corvint-docs [--root PATH] docs maintain --page PATH --source PATH --package DIRECTORY --enable (--preview|--apply)
    [--max-writes N] [--max-wall-clock DURATION] [--watch]
  corvint-docs [--root PATH] docs maintain --check --page PATH [--source PATH --package DIRECTORY]

Example:
  corvint-docs docs maintain --check --page docs/guide.md

An explicitly enabled, bounded local session that refreshes exactly one
generated block of PATH, delimited by "<!-- corvint:docmaintain begin ... -->"
/ "<!-- corvint:docmaintain end -->" markers, using the same native compiler
"docs draft" uses (internal/doccompiler.DraftSources over
internal/contextindex.BuildContext). It detects eligibility by comparing the
draft's cited source digests to the digest recorded in the page's existing
marker, never by hashing the rendered page. Both --enable and one of
--preview/--apply are required; --preview computes and prints the proposed
change without writing; --apply additionally re-checks that the page on disk
still equals the bytes read at session start (refusing with no write on any
difference) and then replaces the page atomically (temp file + rename).
Content outside generated markers, including a missing block for another
selector, is never touched. This is a maintenance session, not a renderer:
docs/specs/human-documentation-compiler-v0.md remains partial (HDCV0-023..029,
HDCV0-041), and no generated block is promoted to accepted intent (AGENTS.md
invariant 8).

--check is read-only and needs no --enable: it compares each generated block of
the page as committed at HEAD (or only the --source/--package block) with a fresh
draft at HEAD, prints corvint-docmaintain-check/0 and exits 1 when any block is
stale or missing (each named with its page line), 0 when none is, and 2 when it
cannot run. --preview keeps exiting 0 whatever it finds.

--watch is Unix-only, requires --apply and stays in the foreground, polling committed HEAD
once per second. It stops on any human page edit, source drift, signal, or bound.
Watch limits: 1024 writes, 24h, 86400 cycles, 1 MiB page, 64 KiB receipt.
Unrelated commits and dirty source bytes do not trigger generated-block writes.

Contract: docs/specs/source-documentation-draft-v0.md (SDD-V0-007+)
`

func runDocsWatch(ctx context.Context, options docsMaintainOptions, policy docmaintain.Policy, stdout, stderr io.Writer) int {
	result, err := docmaintain.Watch(ctx, options.root, options.page, docmaintain.Selector{Source: options.source, Package: options.pkg}, policy)
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: err.Error(), Message: err.Error()})
	}
	encoded, encodeErr := gokernel.CanonicalJSON(result)
	if encodeErr != nil || len(encoded)+1 > 64*1024 {
		return 1
	}
	if _, writeErr := fmt.Fprintf(stdout, "%s\n", encoded); writeErr != nil {
		return 1
	}
	if err != nil {
		return 2
	}
	return 0
}
