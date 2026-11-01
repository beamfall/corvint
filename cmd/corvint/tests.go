package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gateagreement"
	"github.com/Beamfall/corvint/internal/playwrightruns"
	"github.com/Beamfall/corvint/internal/testsource"
)

const testsHelp = `Read committed JS/TS and RSpec test sources (experimental, TSA-V0).

Usage:
  corvint [--root PATH] tests setup-select --specs PATH[,PATH...] [--specs ...]
    [--rev REV] [--format json|lines]
  corvint [--root PATH] tests lint (--base REV [--all-lines] | --all-lines)
    [--rev REV] [--runs REPORT.json ...] [--json] [--advisory] [--compare FILE] [--log FILE]
    [--label NAME]
  corvint [--root PATH] tests duplicates [--ref REV] [--base REV --gate TICKET]
    [--threshold F] [--max-line-tests N] [--decisions FILE ...]
    [--advisory] [--compare FILE] [--log FILE] [--label NAME]
  corvint tests agreement --log FILE [--log FILE ...] [--command NAME]
  corvint [--root PATH] tests validity --files PATH[,PATH...] [--files ...]
    [--legacy NEW=LEGACY ...] [--rev REV] [--profile NAME]
    [--advisory] [--compare FILE] [--log FILE] [--label NAME]
  corvint [--root PATH] tests teardown --files PATH[,PATH...] --declaration PATH
    [--receipt FILE ...] [--rev REV]
  corvint [--root PATH] tests unselected [--rev REV]
  corvint [--root PATH] tests role-locators [--rev REV]
  corvint [--root PATH] tests env --profile NAME [--rev REV] [--base REV]
  corvint [--root PATH] tests citations --docs PATH[,PATH...] [--docs ...]
    [--manifest PATH ...] [--glob PATTERN ...] [--syntax inline-code|html-sub]
    [--pattern RE [--marker RE]] [--repository NAME] [--rev REV]
  corvint [--root PATH] tests example-join --receipt FILE [--rev REV]
    [--select SELECTOR=REQ[,REQ...] ...] [--requirements FILE]

setup-select lists the setup projects (*.setup.* and *.seed.* files) the
named spec files reach at the revision: by a static import, export-from,
literal import() or require(), or by a literal key of a declared string-key
registry whose data context a setup file imports. A file importing every
setup file (a hub) and the registry itself are not followed; comments are
ignored. A computed lookup, unknown key, missing path, unresolved import or
empty spec list returns ALL setup files with typed reasons; a revision with
no setup file at all returns mode none with reason no-setups.
Inclusions give,
for every file the closure proved, the edge chain from a named spec to it.
The output names the revision and a sha256 digest of every input read;
--format lines prints only the setup paths, one per line, for a test
provider's setup list.

lint applies a closed rule set (no-assertion, weak-assertion-only,
conditional-assertion, swallowed-wait-error, fixed-wait,
tautological-assertion, brittle-selector, skip-only-without-reason) to test
and page-object files, reporting only lines changed since --base unless
--all-lines, which lints every line at the revision and needs no --base.
Two advisory rules never change the exit status:
one-shot-ui-state-assertion flags expect(await X) where X is a one-shot
UI-state read (isVisible, count, textContent, ...) directly or through a
helper in the file or a relatively imported module, followed at most 3 hops;
a helper it cannot follow is reported UNKNOWN. exact-text-cross-variant names
an exact-text locator in a test whose Playwright outcome (--runs reports
declaring corvintRunIdentity.testRevision) passes in one project and fails in
another at the linted revision; without such evidence it is NOT_PRODUCED.
A comment "corvint-lint-allow RULE: REASON" on the line, or
alone on the line above, suppresses exactly RULE there; every suppression is
echoed, and one without a reason or with an unknown rule suppresses nothing.
Exit 0 clean or advisory only, 1 findings, 2 error.

duplicates pairs tests in different files that share at least two
normalised expect() lines, scored by Jaccard (default --threshold 0.5); a line
held by more than --max-line-tests tests (default 5) is excluded. Pairs are
candidates, not verdicts. --gate with --base keeps only pairs with a test
added or changed since --base and exits 1 while any remains unresolved. The
committed .corvint/test-duplicates.json (schema
corvint-test-duplicate-decisions/0) resolves a pair with distinct-from and a
reason, or covered-by and an obligation. --decisions FILE (repeatable, at most
16, each at most 1 MiB, same schema) adds caller-supplied decisions read after
the committed file, so a committed decision wins; a caller decision also needs
a reviewer. A decision may carry reviewer, at (an RFC 3339 time or date),
testBlob and otherBlob; a blob (at least 7 hex digits) that is not the test
file's blob at the revision ignores the decision as blob-changed, reopening
the pair. Each resolved pair names its decisionSource (repository:PATH or
caller:FILE); each ignored decision its source and problem. A valid caller
decision missing either blob is listed under warnings as unpinned: it does
not expire when either test is edited. Validity and exit status are unchanged.
Example: corvint tests duplicates --base origin/main --gate V1-1 --decisions review.json

lint, duplicates, validity and migration-ratchet share an advisory/compare mode.
--advisory exits 0 where the command would exit 1 (an error still exits 2).
--log FILE (outside the analysed repository) appends one
corvint-gate-agreement/0 JSONL record per run: the command, its arguments,
the binary's build identity, the revision, both exit codes and the
comparison. --compare VERDICTS.json (needs --log; schema
corvint-gate-verdicts/0, verdicts of {rule, path, line, test, other}) sorts
each Corvint finding into agreements or corvintOnly and each caller verdict
not found into callerOnly, or abstentions where Corvint did not decide (an
uncertain file, an UNKNOWN or suppressed lint finding, every unmatched
verdict in changed-lines scope, a truncated pair list). A line of 0 matches
any line of the path; a duplicate pair is rule duplicate-pair with test and
other; a validity finding is its rule at the path and line of the test it
names, and an abstained finding or cross-file reading abstains there.
Without --compare the record's comparison is NOT_PRODUCED. --label NAME
(once) is copied to the record's label, so logs of several callers can be
split.
agreement reads such logs and prints one corvint-gate-agreement-summary/0
report: runs, compared runs and per rule its agreements, corvintOnly,
callerOnly, abstentions and agreementRate (agreements over the first three,
null when none); a line that is not a record is listed, never counted.
Example: corvint tests lint --base main --all-lines --advisory --compare v.json --log /tmp/a.jsonl
Example: corvint tests agreement --log /tmp/a.jsonl --command "tests lint"

validity applies the static test-validity rules to at most 256 named
committed files (each at most 1 MiB). To a JS/TS test file (a .spec. or .test.
name, or any JS/TS file under a Jest __tests__ directory): ISOLATED-IN-SERIAL,
JS-SKIPPED, JS-TODO, JS-FOCUSED and the advisory JS-EACH-ROW-BINDING,
JS-NO-ASSERTION, JS-CONSTANT-EXPECTATION, JS-SUBJECT-MOCKED,
JS-BOUNDARY-MOCKED, JS-ASSERTION-NOT-AWAITED and WRITE-RESPONSE-UNASSERTED (WRITE-RESPONSE-UNRESOLVED
when the waited method is unknown); with a committed .corvint/seed-rules.json
(corvint-seed-rules/0) also the advisory SEED-VALIDATION-REJECTED,
SEED-SIBLING-COLLISION, SEED-HOOK-BLAST-RADIUS and SEED-VALIDATION-UNRESOLVED.
To a _spec.rb file: MEMOIZED-BLOCK-EXPECTATION,
BLOCK-NOT-CALLED, RSPEC-SKIPPED, RSPEC-FOCUSED, SHARED-EXAMPLE-ONLY,
RB-BLOCK-NEVER-YIELDED, RB-ASSIGNMENT-IN-PREDICATE and the advisory
DESCRIBED-CLASS-STUBBED, RB-CHANGE-ON-IDENTITY, RB-RAISE-ERROR-ONLY and
RB-DENIAL-WITHOUT-NO-EFFECT (RB-DENIAL-VERB-UNRESOLVED or RB-DENIAL-MATCHER-CUSTOM
when the refused request or its matcher is unknown); an
example that skips conditionally or that a committed filter_run_excluding
removes abstains with RSPEC-CONDITIONAL-SKIP or RSPEC-CONFIG-EXCLUDED; a helper the reader cannot
resolve through spec/spec_helper.rb, spec/rails_helper.rb and spec/support
abstains with RB-HELPER-UNRESOLVED or RB-HELPER-AMBIGUOUS. A qualified
reference SPEC:INCLUDE_LINE>SHARED:EXAMPLE_LINE (SHARED may be "same") reads
a shared example through that one inclusion, or abstains with a named
rspec-inclusion-* or rspec-shared-group-* reason. To both, the advisory
intra-example STUB-ON-PATH, STUB-NEVER-INVOKED (static form) and
ASSERTED-VALUE-NOT-FROM-SUBJECT. A _spec.rb example also abstains with
RB-STUB-DECIDES-ASSERTION or RB-MESSAGE-CHAIN-PREFIX-ONLY, and a stub of a
flag lookup (the built-in list or a committed .corvint/flag-lookups.json,
corvint-flag-lookups/0) gives the advisory RB-FLAG-CONDITIONAL and
RB-FLAG-FORCED-NON-DEFAULT and a "conditional on flag KEY=VALUE" condition on
the test. A _spec.rb example also abstains with RB-ASSERT-TARGET-NEVER-WRITTEN
when it asserts a plain attr_accessor of an object it builds that nothing it
exercises writes, read from the committed app/, lib/ and db/schema.rb. It
prints one
corvint-test-source-validity/0 document: each file's findings (uncertain with
a reason where the reader abstained) and the tests. A JS/TS file lists every
example in line order with its hygiene and a NOT_MEASURED strength; other
files list each test a finding names with the hygiene those findings give it.
A JS/TS test file also gets EITHER-OR-OUTCOME and the advisory
CLEANUP-HANDLE-OUTLIVED, UNSTUBBED-OUTBOUND-INPUT and VACUOUS-EMPTY-EQUALITY;
another JS/TS file (a fixture file) CLEANUP-HANDLE-OUTLIVED; and a Markdown,
YAML or JSON behaviour contract EITHER-OR-OUTCOME for "X or Y" expected values.
Exit 1 when an invalid finding
makes a test INELIGIBLE (refused as proof), 0 otherwise. A named JS/TS test
file that unselected reports also gives each of its tests an invalid
TEST-FILE-UNSELECTED finding.

unselected lists every committed test-looking JS/TS file (a test name or
test directory, and a test or describe call) and whether a configured Jest,
Vitest, Playwright or Mocha runner selects it under its literal configuration
or defaults. Each file no runner selects is a TEST-FILE-UNSELECTED finding
with reason no-runner-for-framework, outside-include, excluded (naming the
pattern) or suffix-mismatch. A configuration it cannot read literally, an
unmodelled runner or framework, or no runner evidence at all makes the file
uncertain, never reported. It prints one corvint-test-file-selection/0
document. Exit 1 when any file is reported, 0 otherwise.

--legacy NEW=LEGACY (repeatable, at most 256; --files may then be omitted)
links a new test to the legacy test it claims to preserve, each PATH:LINE of
the example's declaring line. The report's seedParity lists, per link, each
side's setup as entity kinds, counts and explicitly set attributes (RSpec
FactoryBot create/create_list/build/build_list and YAML fixtures; JS/TS
create/build/createList/buildList factory calls) and the advisory
SEED-PARITY-DIVERGES, SEED-PARITY-UNCERTAIN, SEED-AGGREGATE-SINGLE-MEMBER and
SEED-PARITY-NOT-ANALYSED findings. Parity is never reported when a side was
not analysed, a call may not run, or an unknown value meets a different one.
Seed parity never changes the exit code. Next to it, assertionParity lists per
link each side's expectation tuples (subject, matcher, class exact|contains|
presence|other, polarity, expected value) read from RSpec and Capybara,
Jest/Jasmine, Mocha/Chai, Cypress (.cy. files too) and Playwright examples,
pairs legacy with new tuples by expected value and polarity, and gives the
advisory ASSERTION-PARITY-UNMATCHED-LEGACY, ASSERTION-PARITY-NEW-WITHOUT-ORIGIN,
ASSERTION-PARITY-WEAKENED and ASSERTION-PARITY-NOT-ANALYSED findings. A side
with an assertion helper, a conditional or loop, a compound or unknown
expectation form, or no expectation abstains with a reason and no parity is
reported. Assertion parity never changes the exit code.
Example: corvint tests validity --legacy e2e/a.spec.ts:3=spec/a_spec.rb:2

--profile NAME adds an env section: profile NAME's ENV-HARD-CODED-* warnings
on the named files, each with the path:line of the test containing it; it is
advisory and changes no hygiene or exit status.

teardown lists, for each resource an example creates and deletes in teardown
(afterEach and finally in JS/TS; after and ensure in RSpec), the actions on it
with lines. Which calls create, delete and produce dependents comes only from
the committed corvint-side-effect-declaration/0 file named by --declaration.
TEARDOWN-DEPENDENT-LEFTOVER names an action whose dependent is still present
at the parent delete; it is advisory, with reasons, when the graph is
incomplete. A --receipt JS provider receipt bound to the committed test blob
attaches recorded 409/422 delete outcomes. A verdict is never "safe". Exit 1
on a declared finding, 0 otherwise.

role-locators reports ROLE-LOCATOR-UNRENDERED where a component test locates
alert, alertdialog, status or log output by role and no mounted component's
markup renders that role; an unresolvable component or markup that could
render a role the reader cannot see abstains. Exit 1 on a finding.

env reads the committed shell bootstrap scripts of profile NAME (stem NAME or
ending -NAME, _NAME, .NAME): each variable's derivation, value (UNKNOWN with a
reason when unresolvable) and whether it is passed to tests; the stand-in
services launched, their URL and control routes; and advisory
ENV-HARD-CODED-OFFSET / ENV-HARD-CODED-ADDRESS warnings. --base lists changed
variables and the tests whose use stays valid or not. Dotenv files a script
sources, the environment and env_file entries of compose files a script
launches, and the env blocks of CI workflow steps that run a script are read
with their source cited; the profile's own compose.NAME / .env.NAME files are
named, not read. Stand-in routes are read from JS/TS, Go, Flask, Sinatra/Rails
and WireMock mappings; other stand-in languages abstain by name. An assignment
made in only some branches is UNKNOWN (branch-dependent) with its alternatives.

citations checks the revision-pinned source citations in the committed
documents read at --rev: named by --docs, listed one path per line in a
committed --manifest, or matched by a --glob (* ? [..] in a segment, ** for
any segments); at most 65536 documents and 262144 citations per run, read in
bounded chunks. The default inline-code syntax reads spans of the form
path[:START[-END]][#Symbol][@REV] whose path holds a "/" and that carry a
line, a symbol or a revision, or any such path when the document declares
<!-- corvint-cite-revision: REV -->. --syntax html-sub reads <sub> markers
carrying a path and "line N" or "lines N-M"; --pattern reads a Go regular
expression with named groups path (required), repo, start, end, symbol and
rev, optionally inside the candidates a --marker finds. A citation naming
another repo than --repository abstains. Each citation is read from the
object store at its own revision (default: the document's, else --rev) and
reported as missing-at-revision (with the nearest revision where the path
exists when known), range-out-of-bounds, symbol-not-in-range or not-a-file;
an unresolvable revision or a file over 1 MiB abstains; a candidate the
syntax cannot read is unparseable. It prints one corvint-source-citations/1
document with per-document totals and exits 1 while a finding or an
unparseable citation remains.

example-join prints one corvint-example-join/0 record per example of one
Ruby or JS provider receipt: a line-free identity (spec path plus RSpec scoped
id, or file > project > full title), the run outcome, the static hygiene and
strength verdict at the revision the receipt ran on, and the committed files
and blob IDs both used. A receipt without a retained revision, run across two
revisions, at another --rev, or bound to a file whose committed content
differs is refused. --select and --requirements (one SELECTOR=REQ[,REQ...]
pair per line) echo requirement IDs on the examples a PATH, PATH:LINE or
PATH[ID] selector names, marked caller-asserted; they never change a verdict.

Reads committed Git content and the named receipts only; writes nothing and
uses no network.
Refusals: tests-invalid-arguments, tests-invalid-revision,
tests-invalid-option, tests-source-unavailable, tests-bound-exceeded,
tests-decisions-invalid, tests-declaration-invalid, tests-output-unavailable,
tests-join-revision-unbound, tests-join-revision-mismatch,
tests-join-selector-malformed, tests-join-selector-unknown,
agreement-verdicts-invalid, agreement-log-unavailable,
agreement-log-inside-repository.
Contract: docs/specs/test-source-analysis-v0.md (experimental; V1-1587, V1-1589,
V1-1592, V1-1798, V1-1858, V1-1866, V1-1867, V1-1883, V1-1884,
V1-1887, V1-1888, V1-1889, V1-1933, V1-1934,
V1-1935, V1-1972, V1-2039, V1-2040, V1-1892).
`

// testsInvocation recognises `[--root PATH] tests ...`.
func testsInvocation(args []string) (string, []string, bool) {
	root := ""
	for len(args) > 0 {
		if args[0] == "--root" {
			if !rootPreambleValue(args, 1) {
				return "", nil, false
			}
			root = args[1]
			args = args[2:]
			continue
		}
		if strings.HasPrefix(args[0], "--root=") {
			root = strings.TrimPrefix(args[0], "--root=")
			args = args[1:]
			continue
		}
		break
	}
	if len(args) == 0 || args[0] != "tests" {
		return "", nil, false
	}
	return root, args[1:], true
}

func testsRefuse(diagnostic io.Writer, code, message string) int {
	_ = json.NewEncoder(diagnostic).Encode(map[string]any{"code": code, "error": message, "ok": false})
	return 2
}

// testsFailure reports a typed refusal from the analysis packages.
func testsFailure(diagnostic io.Writer, err error) int {
	var sourceError *testsource.Error
	if errors.As(err, &sourceError) {
		return testsRefuse(diagnostic, sourceError.Code, sourceError.Message)
	}
	var indexError *contextindex.Error
	if errors.As(err, &indexError) {
		return testsRefuse(diagnostic, "tests-source-unavailable", indexError.Error())
	}
	return testsRefuse(diagnostic, "tests-source-unavailable", err.Error())
}

func testsWrite(out, diagnostic io.Writer, value any) int {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err == nil {
		_, err = out.Write(append(encoded, '\n'))
	}
	if err != nil {
		return testsRefuse(diagnostic, "tests-output-unavailable", "the output could not be written")
	}
	return 0
}

func runTests(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	if len(args) == 0 {
		return testsRefuse(diagnostic, "tests-invalid-arguments", "usage: corvint tests setup-select|lint|duplicates|agreement|validity|teardown|unselected|role-locators|env|citations|example-join")
	}
	if args[0] == "agreement" {
		// The summary reads only the named logs, so it needs no repository (GVA-V0-007).
		return runTestsAgreement(args[1:], out, diagnostic)
	}
	if root == "" {
		root = "."
	}
	resolved, err := resolveExplicitRoot(root)
	if err != nil {
		emitError(diagnostic, err)
		return 2
	}
	repo := testsource.OpenRepo(ctx, resolved)
	switch args[0] {
	case "setup-select":
		return runTestsSetupSelect(ctx, repo, resolved, args[1:], out, diagnostic)
	case "lint":
		return runTestsLint(repo, resolved, args[1:], out, diagnostic)
	case "duplicates":
		return runTestsDuplicates(repo, resolved, args[1:], out, diagnostic)
	case "validity":
		return runTestsValidity(repo, resolved, args[1:], out, diagnostic)
	case "teardown":
		return runTestsTeardown(repo, args[1:], out, diagnostic)
	case "unselected":
		return runTestsUnselected(repo, args[1:], out, diagnostic)
	case "role-locators":
		return runTestsRoleLocators(repo, args[1:], out, diagnostic)
	case "env":
		return runTestsEnv(repo, args[1:], out, diagnostic)
	case "citations":
		return runTestsCitations(repo, args[1:], out, diagnostic)
	case "example-join":
		return runTestsExampleJoin(repo, args[1:], out, diagnostic)
	}
	return testsRefuse(diagnostic, "tests-invalid-arguments", "usage: corvint tests setup-select|lint|duplicates|agreement|validity|teardown|unselected|role-locators|env|citations|example-join")
}

// testsFlags builds a quiet flag set whose string options may each be given once.
func testsFlags(name string) (*flag.FlagSet, func(string, *string)) {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	seen := map[string]bool{}
	return f, func(option string, dst *string) {
		f.Func(option, option, func(value string) error {
			if seen[option] {
				return errors.New("repeated")
			}
			seen[option], *dst = true, value
			return nil
		})
	}
}

func runTestsSetupSelect(ctx context.Context, repo testsource.Repo, root string, args []string, out, diagnostic io.Writer) int {
	f, once := testsFlags("tests setup-select")
	specs := []string{}
	revision, format := "HEAD", "json"
	f.Func("specs", "spec paths", func(value string) error {
		for _, name := range strings.Split(value, ",") {
			if len(specs) >= testsource.MaxFiles {
				return errors.New("too many specs")
			}
			if strings.TrimSpace(name) != "" {
				specs = append(specs, name)
			}
		}
		return nil
	})
	once("rev", &revision)
	once("format", &format)
	if err := f.Parse(args); err != nil || f.NArg() != 0 || (format != "json" && format != "lines") {
		return testsRefuse(diagnostic, "tests-invalid-arguments",
			"usage: corvint tests setup-select --specs PATH[,PATH...] [--rev REV] [--format json|lines]")
	}
	commit, err := repo.Resolve(revision)
	if err != nil {
		return testsFailure(diagnostic, err)
	}
	index, err := contextindex.BuildRevisionContext(ctx, root, commit)
	if err != nil {
		return testsFailure(diagnostic, err)
	}
	selection := contextindex.SelectSetup(index, specs)
	if format == "json" {
		return testsWrite(out, diagnostic, selection)
	}
	var b strings.Builder
	for _, name := range selection.SetupFiles {
		b.WriteString(name + "\n")
	}
	if _, err := io.WriteString(out, b.String()); err != nil {
		return testsRefuse(diagnostic, "tests-output-unavailable", "the output could not be written")
	}
	return 0
}

func runTestsLint(repo testsource.Repo, root string, args []string, out, diagnostic io.Writer) int {
	gate, args, gateErr := splitGateAgreement(args)
	commandArgs := args
	f, once := testsFlags("tests lint")
	options := testsource.LintOptions{Revision: "HEAD"}
	once("base", &options.Base)
	once("rev", &options.Revision)
	f.BoolVar(&options.AllLines, "all-lines", false, "")
	runs := []string{}
	f.Func("runs", "Playwright JSON reports", func(value string) error {
		if len(runs) >= 64 {
			return errors.New("too many reports")
		}
		runs = append(runs, value)
		return nil
	})
	asJSON := f.Bool("json", false, "")
	if err := f.Parse(args); gateErr != nil || err != nil || f.NArg() != 0 || options.Base == "" && !options.AllLines {
		return testsRefuse(diagnostic, "tests-invalid-arguments", "usage: corvint tests lint (--base REV [--all-lines] | --all-lines) [--rev REV] [--runs REPORT.json ...] [--json] "+gateAgreementUsage)
	}
	if err := gate.prepare(root); err != nil {
		return testsAgreementFailure(diagnostic, err)
	}
	for _, name := range runs {
		run, err := playwrightruns.ReadFile(name)
		if err != nil {
			return testsRefuse(diagnostic, "tests-source-unavailable", "the Playwright report "+strconv.Quote(name)+" is unreadable: "+err.Error())
		}
		evidence := testsource.RunEvidence{Revision: run.TestRevision, Root: run.TestRootPath, Tests: make([]testsource.RunTest, 0, len(run.Tests))}
		for _, t := range run.Tests {
			evidence.Tests = append(evidence.Tests, testsource.RunTest{Project: t.Project, File: t.File, TitlePath: t.TitlePath, Status: t.Status, Executed: t.Executed})
		}
		options.Runs = append(options.Runs, evidence)
	}
	report, err := testsource.Lint(repo, options)
	if err != nil {
		return testsFailure(diagnostic, err)
	}
	status := 0
	if report.Blocking() > 0 {
		status = 1
	}
	status, err = gate.finish("tests lint", commandArgs, report.Revision, lintRules(report), lintObserved(report), status)
	if err != nil {
		return testsAgreementFailure(diagnostic, err)
	}
	if *asJSON {
		if code := testsWrite(out, diagnostic, report); code != 0 {
			return code
		}
		return status
	}
	var b strings.Builder
	for _, finding := range report.Findings {
		label := finding.Rule
		switch {
		case finding.Resolution != "":
			label += " (advisory, " + finding.Resolution + ")"
		case finding.Advisory:
			label += " (advisory)"
		}
		fmt.Fprintf(&b, "%s:%d: %s: %s\n", finding.Path, finding.Line, label, finding.Message)
	}
	for _, s := range report.Suppressions {
		state := "unused"
		switch {
		case !s.Valid:
			state = "ignored (" + s.Problem + ")"
		case s.Used:
			state = "applied"
		}
		fmt.Fprintf(&b, "%s:%d: suppression %s %s: %s\n", s.Path, s.Line, s.Rule, state, s.Reason)
	}
	for _, name := range report.Uncertain {
		fmt.Fprintf(&b, "%s: uncertain: the file did not lex or balance; findings may be incomplete\n", name)
	}
	cross := report.CrossVariant
	if cross.Status == "PRODUCED" {
		fmt.Fprintf(&b, "cross-variant: PRODUCED from %d run(s), projects %s, %d split test(s)\n", cross.Runs, strings.Join(cross.Projects, ","), len(cross.Splits))
	} else {
		fmt.Fprintf(&b, "cross-variant: NOT_PRODUCED (%s)\n", cross.Reason)
	}
	base := report.Base
	if base == "" {
		base = "none"
	}
	fmt.Fprintf(&b, "%d finding(s), %d suppressed, %d file(s), base %s, revision %s, %s\n",
		len(report.Findings), len(report.Suppressed), len(report.Files), base, report.Revision, report.Scope)
	if _, err := io.WriteString(out, b.String()); err != nil {
		return testsRefuse(diagnostic, "tests-output-unavailable", "the output could not be written")
	}
	return status
}

func runTestsDuplicates(repo testsource.Repo, root string, args []string, out, diagnostic io.Writer) int {
	gate, args, gateErr := splitGateAgreement(args)
	commandArgs := args
	f, once := testsFlags("tests duplicates")
	options := testsource.DuplicatesOptions{Revision: "HEAD"}
	threshold, cutoff := "0.5", "5"
	once("ref", &options.Revision)
	once("base", &options.Base)
	once("gate", &options.Gate)
	once("threshold", &threshold)
	once("max-line-tests", &cutoff)
	decisionFiles := []string{}
	f.Func("decisions", "caller decision file", func(value string) error {
		if len(decisionFiles) >= testsource.MaxDecisionFiles || strings.TrimSpace(value) == "" {
			return errors.New("too many or empty")
		}
		decisionFiles = append(decisionFiles, value)
		return nil
	})
	usage := "usage: corvint tests duplicates [--ref REV] [--base REV --gate TICKET] [--threshold F] [--max-line-tests N] [--decisions FILE ...] " + gateAgreementUsage
	if err := f.Parse(args); gateErr != nil || err != nil || f.NArg() != 0 || strings.ContainsAny(options.Gate, "\x00\n\r") {
		return testsRefuse(diagnostic, "tests-invalid-arguments", usage)
	}
	for _, name := range decisionFiles {
		data, err := readBoundedFile(name, testsource.MaxDecisionFileBytes)
		if err != nil {
			return testsRefuse(diagnostic, "tests-decisions-invalid", "the decision file "+strconv.Quote(name)+" is unreadable or larger than 1 MiB")
		}
		options.CallerDecisions = append(options.CallerDecisions, testsource.DecisionFile{Name: name, Data: data})
	}
	var err error
	if options.Threshold, err = strconv.ParseFloat(threshold, 64); err != nil {
		return testsRefuse(diagnostic, "tests-invalid-option", "--threshold must be a number in (0,1]")
	}
	if options.MaxLineTests, err = strconv.Atoi(cutoff); err != nil {
		return testsRefuse(diagnostic, "tests-invalid-option", "--max-line-tests must be an integer in [2,100]")
	}
	if err := gate.prepare(root); err != nil {
		return testsAgreementFailure(diagnostic, err)
	}
	report, err := testsource.Duplicates(repo, options)
	if err != nil {
		return testsFailure(diagnostic, err)
	}
	status := 0
	if options.Gate != "" && report.UnresolvedCount > 0 {
		status = 1
	}
	status, err = gate.finish("tests duplicates", commandArgs, report.Revision, []string{duplicatesAgreementRule}, duplicatesObserved(report), status)
	if err != nil {
		return testsAgreementFailure(diagnostic, err)
	}
	if code := testsWrite(out, diagnostic, report); code != 0 {
		return code
	}
	return status
}

// testsAgreementFailure reports a typed agreement refusal (GVA-V0-001).
func testsAgreementFailure(diagnostic io.Writer, err error) int {
	var agreementError *gateagreement.Error
	if errors.As(err, &agreementError) {
		return testsRefuse(diagnostic, agreementError.Code, agreementError.Message)
	}
	return testsRefuse(diagnostic, "agreement-log-unavailable", err.Error())
}

// runTestsAgreement prints the per-rule agreement over agreement logs (GVA-V0-007).
func runTestsAgreement(args []string, out, diagnostic io.Writer) int {
	f, once := testsFlags("tests agreement")
	logs := []string{}
	command := ""
	f.Func("log", "agreement log", func(value string) error {
		if len(logs) >= gateagreement.MaxLogs || strings.TrimSpace(value) == "" {
			return errors.New("too many or empty")
		}
		logs = append(logs, value)
		return nil
	})
	once("command", &command)
	if err := f.Parse(args); err != nil || f.NArg() != 0 || len(logs) == 0 {
		return testsRefuse(diagnostic, "tests-invalid-arguments", "usage: corvint tests agreement --log FILE.jsonl [--log ...] [--command NAME]")
	}
	summary, err := gateagreement.Summarise(logs, command, openAgreementLog)
	if err != nil {
		return testsAgreementFailure(diagnostic, err)
	}
	return testsWrite(out, diagnostic, summary)
}

// runTestsValidity prints the static test-validity findings of the named committed files
// (TSA-V0-071) and the advisory seed parity of each --legacy link (TSA-V0-108). It exits 1 when a
// finding refuses a test as proof; --advisory, --compare and --log apply as on lint (GVA-V0-094).
func runTestsValidity(repo testsource.Repo, root string, args []string, out, diagnostic io.Writer) int {
	gate, args, gateErr := splitGateAgreement(args)
	commandArgs := args
	f, once := testsFlags("tests validity")
	options := testsource.SourceValidityOptions{Revision: "HEAD"}
	f.Func("files", "test file paths", func(value string) error {
		for _, name := range strings.Split(value, ",") {
			if strings.TrimSpace(name) != "" {
				options.Files = append(options.Files, name)
			}
		}
		return nil
	})
	f.Func("legacy", "NEW=LEGACY test link, each PATH:LINE", func(value string) error {
		current, legacy, ok := strings.Cut(value, "=")
		if !ok || current == "" || legacy == "" {
			return fmt.Errorf("want NEW=LEGACY")
		}
		options.Links = append(options.Links, testsource.SeedLink{New: current, Legacy: legacy})
		return nil
	})
	once("rev", &options.Revision)
	once("profile", &options.Profile)
	if err := f.Parse(args); gateErr != nil || err != nil || f.NArg() != 0 || len(options.Files) == 0 && len(options.Links) == 0 {
		return testsRefuse(diagnostic, "tests-invalid-arguments", "usage: corvint tests validity --files PATH[,PATH...] [--legacy NEW=LEGACY ...] [--rev REV] [--profile NAME] "+gateAgreementUsage)
	}
	if err := gate.prepare(root); err != nil {
		return testsAgreementFailure(diagnostic, err)
	}
	report, err := testsource.SelectedSourceValidity(repo, options)
	if err != nil {
		return testsFailure(diagnostic, err)
	}
	status := 0
	if report.Refused() {
		status = 1
	}
	status, err = gate.finish("tests validity", commandArgs, report.Revision, nil, validityObserved(report), status)
	if err != nil {
		return testsAgreementFailure(diagnostic, err)
	}
	if code := testsWrite(out, diagnostic, report); code != 0 {
		return code
	}
	return status
}

// runTestsCitations prints the revision-pinned citation findings of the selected committed
// documents (TSA-V0-255, TSA-V0-386). It exits 1 when a citation is not supported by the object
// store or cannot be read.
func runTestsCitations(repo testsource.Repo, args []string, out, diagnostic io.Writer) int {
	f, once := testsFlags("tests citations")
	options := testsource.CitationOptions{Revision: "HEAD"}
	f.Func("docs", "document paths", func(value string) error {
		for _, name := range strings.Split(value, ",") {
			if strings.TrimSpace(name) != "" {
				options.Docs = append(options.Docs, name)
			}
		}
		return nil
	})
	f.Func("manifest", "committed manifest", func(value string) error {
		options.Manifests = append(options.Manifests, value)
		return nil
	})
	f.Func("glob", "committed path glob", func(value string) error {
		options.Globs = append(options.Globs, value)
		return nil
	})
	once("rev", &options.Revision)
	once("syntax", &options.Syntax)
	once("pattern", &options.Pattern)
	once("marker", &options.Marker)
	once("repository", &options.Repository)
	if err := f.Parse(args); err != nil || f.NArg() != 0 || len(options.Docs)+len(options.Manifests)+len(options.Globs) == 0 {
		return testsRefuse(diagnostic, "tests-invalid-arguments", "usage: corvint tests citations --docs PATH[,PATH...] [--manifest PATH] [--glob PATTERN] [--syntax NAME | --pattern RE [--marker RE]] [--repository NAME] [--rev REV]")
	}
	report, err := testsource.Citations(repo, options)
	if err != nil {
		return testsFailure(diagnostic, err)
	}
	if code := testsWrite(out, diagnostic, report); code != 0 {
		return code
	}
	if len(report.Findings) > 0 || len(report.Unparseable) > 0 {
		return 1
	}
	return 0
}
