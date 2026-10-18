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
	"github.com/Beamfall/corvint/internal/playwrightruns"
	"github.com/Beamfall/corvint/internal/testsource"
)

const testsHelp = `Read committed JS/TS and RSpec test sources (experimental, TSA-V0).

Usage:
  corvint [--root PATH] tests setup-select --specs PATH[,PATH...] [--specs ...]
    [--rev REV] [--format json|lines]
  corvint [--root PATH] tests lint --base REV [--rev REV] [--all-lines]
    [--runs REPORT.json ...] [--json]
  corvint [--root PATH] tests duplicates [--ref REV] [--base REV --gate TICKET]
    [--threshold F] [--max-line-tests N]
  corvint [--root PATH] tests validity --files PATH[,PATH...] [--files ...]
    [--rev REV]

setup-select lists the setup projects (*.setup.* and *.seed.* files) the
named spec files reach at the revision: by a static import, export-from,
literal import() or require(), or by a literal key of a declared string-key
registry whose data context a setup file imports. A file importing every
setup file (a hub) and the registry itself are not followed; comments are
ignored. A computed lookup, unknown key, missing path, unresolved import or
empty spec list returns ALL setup files with typed reasons. The output names
the revision and a sha256 digest of every input read; --format lines prints
only the setup paths, one per line, for a test provider's setup list.

lint applies a closed rule set (no-assertion, weak-assertion-only,
conditional-assertion, swallowed-wait-error, fixed-wait,
tautological-assertion, brittle-selector, skip-only-without-reason) to test
and page-object files, reporting only lines changed since --base unless
--all-lines. Two advisory rules never change the exit status:
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
reason, or covered-by and an obligation.

validity applies the static test-validity rules to at most 256 named
committed files (each at most 1 MiB): ISOLATED-IN-SERIAL to a JS/TS test file,
and to a _spec.rb file MEMOIZED-BLOCK-EXPECTATION, RSPEC-SKIPPED,
RSPEC-FOCUSED, SHARED-EXAMPLE-ONLY and the advisory DESCRIBED-CLASS-STUBBED.
It prints one corvint-test-source-validity/0 document: each file's findings
(uncertain with a reason where the reader abstained) and, for each test a
finding names, the hygiene axis those findings give it. Exit 1 when an
invalid finding makes a test INELIGIBLE (refused as proof), 0 otherwise.

Reads committed Git content only; writes nothing and uses no network.
Refusals: tests-invalid-arguments, tests-invalid-revision,
tests-invalid-option, tests-source-unavailable, tests-bound-exceeded,
tests-decisions-invalid, tests-output-unavailable.
Contract: docs/specs/test-source-analysis-v0.md (experimental; V1-1587, V1-1589,
V1-1592, V1-1798).
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
		return testsRefuse(diagnostic, "tests-invalid-arguments", "usage: corvint tests setup-select|lint|duplicates|validity")
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
		return runTestsLint(repo, args[1:], out, diagnostic)
	case "duplicates":
		return runTestsDuplicates(repo, args[1:], out, diagnostic)
	case "validity":
		return runTestsValidity(repo, args[1:], out, diagnostic)
	}
	return testsRefuse(diagnostic, "tests-invalid-arguments", "usage: corvint tests setup-select|lint|duplicates|validity")
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

func runTestsLint(repo testsource.Repo, args []string, out, diagnostic io.Writer) int {
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
	if err := f.Parse(args); err != nil || f.NArg() != 0 || options.Base == "" {
		return testsRefuse(diagnostic, "tests-invalid-arguments", "usage: corvint tests lint --base REV [--rev REV] [--all-lines] [--runs REPORT.json ...] [--json]")
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
	fmt.Fprintf(&b, "%d finding(s), %d suppressed, %d file(s), base %s, revision %s, %s\n",
		len(report.Findings), len(report.Suppressed), len(report.Files), report.Base, report.Revision, report.Scope)
	if _, err := io.WriteString(out, b.String()); err != nil {
		return testsRefuse(diagnostic, "tests-output-unavailable", "the output could not be written")
	}
	return status
}

func runTestsDuplicates(repo testsource.Repo, args []string, out, diagnostic io.Writer) int {
	f, once := testsFlags("tests duplicates")
	options := testsource.DuplicatesOptions{Revision: "HEAD"}
	threshold, cutoff := "0.5", "5"
	once("ref", &options.Revision)
	once("base", &options.Base)
	once("gate", &options.Gate)
	once("threshold", &threshold)
	once("max-line-tests", &cutoff)
	usage := "usage: corvint tests duplicates [--ref REV] [--base REV --gate TICKET] [--threshold F] [--max-line-tests N]"
	if err := f.Parse(args); err != nil || f.NArg() != 0 || strings.ContainsAny(options.Gate, "\x00\n\r") {
		return testsRefuse(diagnostic, "tests-invalid-arguments", usage)
	}
	var err error
	if options.Threshold, err = strconv.ParseFloat(threshold, 64); err != nil {
		return testsRefuse(diagnostic, "tests-invalid-option", "--threshold must be a number in (0,1]")
	}
	if options.MaxLineTests, err = strconv.Atoi(cutoff); err != nil {
		return testsRefuse(diagnostic, "tests-invalid-option", "--max-line-tests must be an integer in [2,100]")
	}
	report, err := testsource.Duplicates(repo, options)
	if err != nil {
		return testsFailure(diagnostic, err)
	}
	if code := testsWrite(out, diagnostic, report); code != 0 {
		return code
	}
	if options.Gate != "" && report.UnresolvedCount > 0 {
		return 1
	}
	return 0
}

// runTestsValidity prints the static test-validity findings of the named committed files
// (TSA-V0-071). It exits 1 when a finding refuses a test as proof.
func runTestsValidity(repo testsource.Repo, args []string, out, diagnostic io.Writer) int {
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
	once("rev", &options.Revision)
	if err := f.Parse(args); err != nil || f.NArg() != 0 || len(options.Files) == 0 {
		return testsRefuse(diagnostic, "tests-invalid-arguments", "usage: corvint tests validity --files PATH[,PATH...] [--rev REV]")
	}
	report, err := testsource.SourceValidity(repo, options)
	if err != nil {
		return testsFailure(diagnostic, err)
	}
	if code := testsWrite(out, diagnostic, report); code != 0 {
		return code
	}
	if report.Refused() {
		return 1
	}
	return 0
}
