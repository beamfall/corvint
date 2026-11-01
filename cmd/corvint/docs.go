package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccompiler"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
)

type docsOptions struct {
	root, mode, source, directory, task, request, inspectPython string
	trusted, dryRun, ownerStdin                                 bool
	page                                                        int
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
	if position+1 >= len(arguments) || !slices.Contains(docsSubcommands, arguments[position+1]) {
		return options, true, argumentError("docs requires one of " + strings.Join(docsSubcommands, ", "))
	}
	options.mode = arguments[position+1]
	if options.mode == "nav" {
		var err error
		options, err = parseTrustedNavArgs(options, arguments[position+2:])
		return options, true, err
	}
	if options.mode != "draft" && options.mode != "consume" {
		// A listed subcommand whose own parser did not claim it is a dispatch defect.
		return options, true, errors.New("docs " + options.mode + " is listed but has no parser")
	}
	seen := map[string]bool{}
	for index := position + 2; index < len(arguments); index++ {
		flag, value, inline := strings.Cut(arguments[index], "=")
		if flag == "--dry-run" || flag == "--owner-stdin" {
			if inline || seen[flag] {
				return options, true, argumentError("docs " + flag + " takes no value and may appear once")
			}
			seen[flag] = true
			options.dryRun = options.dryRun || flag == "--dry-run"
			options.ownerStdin = options.ownerStdin || flag == "--owner-stdin"
			continue
		}
		if flag != "--source" && flag != "--package" && flag != "--task" && flag != "--page" {
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
		case "--page":
			page, err := strconv.Atoi(value)
			if err != nil || page < 1 || page > 9999 || strconv.Itoa(page) != value {
				return options, true, argumentError("docs --page requires a decimal page number from 1 to 9999")
			}
			options.page = page
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
	if options.mode == "consume" && options.dryRun {
		return options, true, argumentError("docs consume does not accept --dry-run: a dry-run draft is not admissible")
	}
	if options.ownerStdin && !options.dryRun {
		return options, true, argumentError("docs draft --owner-stdin requires --dry-run")
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
	var provided, owner []byte
	if options.mode == "draft" && options.dryRun {
		var err error
		if options.ownerStdin {
			owner, err = readInputBounded(ctx, stdin, doccompiler.DraftMaxBytes)
			if err == nil && len(owner) > doccompiler.DraftMaxBytes {
				err = &doccompiler.Error{Code: "documentation-limit-exceeded", Message: "dry-run owner source on stdin exceeds 64 KiB"}
			}
		} else {
			owner, err = doccompiler.ReadDryRunOwner(options.root, options.source)
		}
		if err != nil {
			return nil, err
		}
	}
	if options.mode == "consume" {
		input, err := readInputBounded(ctx, stdin, doccompiler.DraftMaxBytes)
		if err != nil {
			return nil, err
		}
		if len(input) > doccompiler.DraftMaxBytes {
			return nil, &doccompiler.Error{Code: "documentation-limit-exceeded", Message: fmt.Sprintf("consume input exceeds the DraftMaxBytes limit of %d bytes (64 KiB): observed at least %d bytes on stdin", doccompiler.DraftMaxBytes, len(input))}
		}
		if doccompiler.IsDryRunDraft(input) {
			return nil, &doccompiler.Error{Code: "unsupported-documentation-source", Message: "dry-run draft is not admissible: commit the owner source, draft again and consume that draft"}
		}
		provided = input
	}
	index, err := contextindex.BuildContext(ctx, options.root, "")
	if err != nil {
		return nil, err
	}
	if options.dryRun {
		draft, err := doccompiler.DraftDryRunWith(index, options.source, owner, options.directory, doccompiler.DraftOptions{Page: options.page})
		if err != nil {
			return nil, err
		}
		return draft.Markdown, nil
	}
	draft, err := doccompiler.DraftSourcesWith(index, options.source, options.directory, doccompiler.DraftOptions{Page: options.page})
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
		return nil, &doccompiler.Error{Code: "documentation-limit-exceeded", Message: fmt.Sprintf("encoded consumption result exceeds the DraftMaxBytes limit of %d bytes (64 KiB): observed %d bytes including its trailing newline", doccompiler.DraftMaxBytes, len(encoded)+1)}
	}
	return append(encoded, '\n'), nil
}

// docsSubcommands is the `docs` dispatch table: nav, draft and consume dispatch through
// parseDocsInvocation, corpus through parseCorpusInvocation, maintain through
// docs_maintain.go and flows through docs_flows.go. parseDocsInvocation, the catch-all that
// runs after those parsers, refuses any name outside this table with a refusal built from it,
// so a name dispatches only if listed. docsHelp lists every entry with its maturity, and
// TestDocsHelpListsEveryDispatchedSubcommand drives each entry into its own parser (V1-2134).
var docsSubcommands = []string{"nav", "draft", "consume", "corpus", "maintain", "flows"}

const docsHelp = `Experimental source documentation orientation

Usage:
  corvint [--root PATH] docs nav --trusted-project --request FILE
  corvint [--root PATH] docs nav --trusted-project --inspect-python ABSOLUTE_PYTHON
  corvint [--root PATH] docs draft --source PATH --package DIRECTORY [--page N]
  corvint [--root PATH] docs draft --dry-run [--owner-stdin] --source PATH --package DIRECTORY [--page N]
  corvint [--root PATH] docs consume --source PATH --package DIRECTORY [--page N] --task TEXT < draft.md
  corvint [--root PATH] docs corpus OPERATION ...      (corvint help docs corpus)
  corvint [--root PATH] docs maintain --page PATH ...  (corvint help docs maintain)
  corvint [--root PATH] docs flows generate|check|finalize ...  (corvint help docs flows)
Maturity: every docs subcommand (nav, draft, consume, corpus, maintain, flows) is experimental.

Navigation is the separate explicitly trusted pinned MkDocs profile. It stages committed
source, emits an exact proposal patch, and validates navigation by a full candidate reload.
It never applies to source; build NOT_RUN, offline NOT_OBSERVED. See
docs/specs/trusted-project-navigation-v0.md for pins and bounded configuration.

Draft ordinary Markdown from immutable owner-source excerpts, tracked non-test exported-name
declarations, and Go import edges. Consume the actual draft after fresh original-source
rederivation. GENERATED orientation; build applicability and behavioral validation UNKNOWN.
Read-only except the bounded self-observation ledger on unsupported-* failures (SOL-V0-007).
No HDC/MkDocs qualification, authority promotion, or persistent generated index.

Prerequisites:
  Use committed sources in a Git repository with HEAD. The owner must be admitted,
  tracked .md text (UTF-8, LF, not generated), with exactly one literal ## Agent digest
  heading outside fenced code. Its preamble and digest together must fit 64 lines
  and 4 KiB; unclosed backtick/tilde fences are refused.
  Package is one repository-relative module directory, not ., in a language the
  native index extracts declarations from (Go, Python, TypeScript/JavaScript, Rust,
  C#, Swift, Kotlin, Ruby), with tracked non-test exported-name declarations;
  other languages are refused by name. Dirty worktree edits are not source evidence.

Paging (SDD-V0-632): declarations and importers each show at most 64 entries per page.
  A larger package prints "Page: N of P" and a TRUNCATED limitation naming the shown
  range; entries on other pages are UNKNOWN there. --page N (default 1) selects a page;
  an out-of-range page is refused. Consume a page with the same --page.

Dry run (SDD-V0-019): --dry-run previews a first draft before the owner is committed.
  It reads the owner from the working-tree file at --source, or from stdin with
  --owner-stdin (--source then only names it), up to 64 KiB. The package must still be
  tracked. Output is marked NOT_ADMISSIBLE with Authority NONE, cites no Git blob for the
  owner, cannot be cited as authority, and docs consume refuses it. Example:
    corvint docs draft --dry-run --source docs/new-owner.md --package internal/docmaintain

Example from a Corvint checkout with committed sources:
  corvint docs draft --source docs/specs/source-documentation-draft-v0.md --package internal/docmaintain > /tmp/corvint-source-draft.md
  corvint docs consume --source docs/specs/source-documentation-draft-v0.md --package internal/docmaintain --task Preview < /tmp/corvint-source-draft.md

Contract and remaining bounds: docs/specs/source-documentation-draft-v0.md

Separate experimental corpus profile:
  corvint [--root PATH] docs corpus manifest --revision FULL_COMMIT --scope PATH --timestamp RFC3339
  corvint [--root PATH] docs corpus build --manifest INPUT.json
  corvint [--root PATH] docs corpus behavior-adapter --input REQUEST.json [--previous RESULT.json] [--check]
  corvint [--root PATH] docs corpus behavior-provider --input REQUEST-V2.json
  corvint [--root PATH] docs corpus compact-provider --input PROVIDER.json
  corvint [--root PATH] docs corpus estate --input ESTATE.json
  corvint [--root PATH] docs corpus discover-playwright --migration MIGRATION.json --config PATH --playwright-list LIST.json [--receipt RECEIPT.json]
  corvint [--root PATH] docs corpus witness-playwright --input REQUEST.json --receipt-input ID --report REPORT.json
  corvint [--root PATH] docs corpus discover-legacy --runner jasmine|mocha --revision FULL_COMMIT --path PATH [--input REGISTRY.json]
  corvint [--root PATH] docs corpus discover-tests --runner playwright|rspec --revision FULL_COMMIT --path PATH
  corvint [--root PATH] docs corpus extract --profile PATH --revision FULL_COMMIT [--profile-sha256 HEX]
  corvint [--root PATH] docs corpus import-cases --format testrail-json|csv --input EXPORT [--content ids-paths-titles|full]
  corvint [--root PATH] docs corpus search-cases --format testrail-json|csv --input EXPORT (--query TEXT [--match all|any] | --id C<digits>) [--limit N]
  corvint [--root PATH] docs corpus search --artifact CORPUS.json --query TEXT
  corvint [--root PATH] docs corpus render --artifact CORPUS.json
  corvint [--root PATH] docs corpus maintain --artifact CORPUS.json --page PAGE.md [--apply]
Read operations: info, validate, get/trace/related/journey/stability --id ID, locate --path PATH,
coverage, gaps [--id ID]. Optional --limit 1..256. All require --artifact.
Local input files (--manifest, --artifact, --input, --previous, --cem, --retirement, --migration,
--playwright-list, --receipt, --report) are canonical paths relative to --root and must sit under
it: no leading /, no .., no ./ or //. Absolute paths are refused (corpus-refused, naming the flag).
Native query/context/impact/affected/test-validity/work observe/propose-wave and CEM
status/verify/report accept --corpus=CORPUS.json. Generated evidence grants no task
or test-selection authority. Read commands print bounded JSON and never collect tests.
Every operation: corvint help docs corpus; its options: corvint docs corpus OPERATION --help.
Guide: docs/DOCUMENTATION-CORPUS.md; contract: docs/specs/documentation-corpus-v1.md

` + playwrightReceiptHelp + "\n" + behaviorAnnotationHelp + "\n" + legacyCasesHelp

// behaviorAnnotationHelp is the DCP-V1-080 annotation contract that docs help and the
// witness-playwright help topic share.
const behaviorAnnotationHelp = `Behavior annotations (DCP-V1-080): witness-playwright reads the annotations of the single
first attempt (retry 0, passed) of each registered test in REPORT.json, in published order:
  corvint-behavior-event    one closed JSON event per annotation, no other keys: browser_context,
                            page, frame, navigation, parent_page, behavior, criterion, matcher,
                            locator, value, passed, sequence, kind, id. sequence is 1, 2, 3, ...;
                            passed is true; browser_context, page, frame and id are non-empty;
                            kind:id is unique; at most 4096 events.
                            kind: page | assertion | negative-control.
                            navigation (required for page): main-frame | frame | redirect | popup |
                            setup; popup also names parent_page.
  corvint-behavior-cleanup  exactly one, after the cleanup ran: passed. None is cleanup-unobserved;
                            failed or more than one is cleanup-failed.
  corvint-behavior-fixture  one per registered fixture, equal to the test's fixtures in order.
  corvint-behavior-role     one per registered role, equal to the test's roles in order.
To count as runtime-witnessed, the events must equal the ordered_events of each flow the test
registers, and the assertion events must equal the test's registered assertion tuples (id,
behavior, criterion, matcher, locator, value) one for one. An unwitnessed test names one reason; for an annotation-stage reason (events-unobserved, event-malformed,
cleanup-unobserved, cleanup-failed, fixture-role-mismatch) missing_annotations lists the needed
types never published, in the order events, cleanup, fixture, role (DCP-V1-082).
Producer (DCP-V1-081): internal/doccorpus/behavior-events.cjs extends a Playwright test with a
behavior fixture that publishes these annotations at teardown. It records an assertion only
after Playwright's own matcher passed and a page event only for a main-frame http(s) commit:
    const test = require('CORVINT/internal/doccorpus/behavior-events.cjs').extend(base);
    await behavior.expect({id, behavior, criterion})(page.locator('#count')).toHaveText('2');
Walkthrough: docs/DOCUMENTATION-CORPUS.md, "Playwright behavior witness walkthrough".
`

// playwrightReceiptHelp is the DCP-V1-053 receipt guidance that docs help and the
// discover-playwright and witness-playwright help topics share.
const playwrightReceiptHelp = `Playwright qualified receipt (DCP-V1-053): the producer is corvint-js-test-provider e2e
--external-server. Its stdout is the canonical qualified receipt, and --retain writes the same
bytes under .corvint/test-evidence/. Managed-server e2e output is not a qualified receipt.
discover-playwright --receipt reads the file unchanged. witness-playwright --receipt-input names a
REQUEST.json input, not a file. The one conversion step: commit the receipt bytes unchanged at a
tracked path and add them as an observed external-provider request input with a full-file anchor.
REPORT.json is a caller-run Playwright JSON report of the same config file and runner version.
End-to-end example, run at the repository root:
    corvint-js-test-provider e2e --external-server --server-ready-url http://127.0.0.1:3000 --app-identity web --config playwright.config.ts --runner-version 1.64.0 --test-file e2e/items.spec.ts --retain > evidence/receipt.json
    corvint docs corpus discover-playwright --migration evidence/migration.json --config playwright.config.ts --playwright-list evidence/list.json --receipt evidence/receipt.json > evidence/discovery.json
    git add evidence/receipt.json && git commit -m "retain playwright receipt"
    corvint docs corpus witness-playwright --input evidence/request.json --receipt-input receipt --report evidence/report.json
REQUEST.json names the committed receipt as input "receipt", beside the migration and discovery.

Test discovery (DCX-V1-370..376): discover-tests reads Playwright or RSpec test files from Git
at --revision (one file, or the test files under a directory; --path . for all) and prints a
generated corvint-test-discovery/1 record without running a test. Each example carries its
example-join identity, title path, tags, annotations, expectations (matcher, negation, receiver),
navigations and requests, the imports, fixtures and shared groups it reaches, and the hooks that
apply with whether each asserts. Every fact is anchored to its committed lines; whatever the
reader cannot classify is listed under unknown with a reason.

Legacy inventory (DCP-V1-060..069): discover-legacy reads Jasmine or Mocha/Cypress test files
from Git at --revision (one file, or the test-named files under a directory; --path . for all)
and prints a generated corvint-legacy-inventory/1 record without running a test. Cases use the
legacy case shape: disabled cases are marked, assertions become matcher/locator/value tuples,
precondition fixtures carry the digest of their call text, and every unparsed assertion is listed
under unknown. --input REGISTRY.json adds a generated crosswalk of candidate flows and tests and
re-checks the registry's reviewed legacy_criteria; a refused same/stronger mapping exits 1 with
the report printed. Nothing it emits is a reviewed legacy_criteria relation.

Extractor profiles (DCX-V1-390..399): extract reads a committed corvint-extractor-profile/1 file
and the files its line-oriented patterns name from Git at --revision and prints a generated
corvint-extractor-inventory/1 record. Its provider member is a corvint-corpus-provider/1 record
of endpoint, ui_surface, data_entity, job, event, symbol and interaction subjects with exact line
anchors and implements, calls, depends_on and related_to relations; unreadable files, unsupported
shapes and unresolved references are listed under gaps. Tool output is read only as a committed
file whose SHA-256 the profile pins; no tool runs. With --profile-sha256 HEX (DCX-V1-478), --profile
is the absolute path of a caller-owned profile outside the repository, admitted only when its
bytes hash to HEX; the inventory records origin caller, the digest and the canonical profile, and
sources are still read only from Git at --revision.
`

// legacyCasesHelp is the DCP-V1-070..079 test-management export guidance that docs help and the
// import-cases and search-cases help topics share.
const legacyCasesHelp = `Test-management export (DCP-V1-070..079): import-cases reads a caller-exported TestRail JSON
({suites, sections, cases}) or CSV file, joins each case with its suite and section path and emits
corvint-legacy-cases/1: state manual legacy cases keyed by case ID. Manual cases are never
executable and never count as runtime parity. --content defaults to ids-paths-titles, which omits
preconditions, steps and expected results (the case digest still covers them); full keeps the
HTML-stripped text and marks the record local_only, so keep it out of commits. Oversized fields are
bounded and listed under bounded, never silently cut. search-cases matches every query word as a
prefix in title, suite/section path, preconditions and steps, returns case ID, path and title only,
and names its query, fields and searched export, so a none-found result can be audited.
--match any (DCP-V1-166) instead ranks cases matching any non-stop-word query word by weighted
score (title > path > preconditions/steps), then export order, with a per-hit score. --id C12345 or
--id 12345 (DCP-V1-165) returns exactly that case. A query that is exactly one C<digits> word naming
a case in the export is that ID lookup; otherwise it searches as before (DCP-V1-167).
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
