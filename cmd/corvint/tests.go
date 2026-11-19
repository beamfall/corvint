package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/testsource"
)

const testsHelp = `Read committed JS/TS and RSpec test sources (experimental, TSA-V0).

Usage:
  corvint [--root PATH] tests setup-select --specs PATH[,PATH...] [--specs ...]
    [--rev REV] [--format json|lines]
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

unselected lists every committed test-looking JS/TS file (a test name or
test directory, and a test or describe call) and whether a configured Jest,
Vitest, Playwright or Mocha runner selects it under its literal configuration
or defaults. Each file no runner selects is a TEST-FILE-UNSELECTED finding
with reason no-runner-for-framework, outside-include, excluded (naming the
pattern) or suffix-mismatch. A configuration it cannot read literally, an
unmodelled runner or framework, or no runner evidence at all makes the file
uncertain, never reported. It prints one corvint-test-file-selection/0
document. Exit 1 when any file is reported, 0 otherwise.

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

Shared source-reader finding vocabulary (retained library rules):
ASSERTED-VALUE-NOT-FROM-SUBJECT, ASSERTION-PARITY-NEW-WITHOUT-ORIGIN,
ASSERTION-PARITY-NOT-ANALYSED, ASSERTION-PARITY-UNMATCHED-LEGACY, ASSERTION-PARITY-WEAKENED,
BLOCK-NOT-CALLED, CLEANUP-HANDLE-OUTLIVED, DESCRIBED-CLASS-STUBBED, EITHER-OR-OUTCOME,
ENV-HARD-CODED-ADDRESS, ENV-HARD-CODED-OFFSET, ISOLATED-IN-SERIAL, JS-ASSERTION-NOT-AWAITED,
JS-BOUNDARY-MOCKED, JS-CONSTANT-EXPECTATION, JS-EACH-ROW-BINDING, JS-FOCUSED, JS-NO-ASSERTION,
JS-SKIPPED, JS-SUBJECT-MOCKED, JS-TODO, MEMOIZED-BLOCK-EXPECTATION,
RB-ASSERT-TARGET-NEVER-WRITTEN, RB-ASSIGNMENT-IN-PREDICATE, RB-BLOCK-NEVER-YIELDED,
RB-CHANGE-ON-IDENTITY, RB-DENIAL-MATCHER-CUSTOM, RB-DENIAL-VERB-UNRESOLVED,
RB-DENIAL-WITHOUT-NO-EFFECT, RB-FLAG-CONDITIONAL, RB-FLAG-FORCED-NON-DEFAULT,
RB-HELPER-AMBIGUOUS, RB-HELPER-UNRESOLVED, RB-MESSAGE-CHAIN-PREFIX-ONLY, RB-RAISE-ERROR-ONLY,
RB-STUB-DECIDES-ASSERTION, ROLE-LOCATOR-UNRENDERED, RSPEC-CONDITIONAL-SKIP,
RSPEC-CONFIG-EXCLUDED, RSPEC-FOCUSED, RSPEC-SKIPPED, SEED-AGGREGATE-SINGLE-MEMBER,
SEED-HOOK-BLAST-RADIUS, SEED-PARITY-DIVERGES, SEED-PARITY-NOT-ANALYSED, SEED-PARITY-UNCERTAIN,
SEED-SIBLING-COLLISION, SEED-VALIDATION-REJECTED, SEED-VALIDATION-UNRESOLVED,
SHARED-EXAMPLE-ONLY, STUB-NEVER-INVOKED, STUB-ON-PATH, TEARDOWN-DEPENDENT-LEFTOVER,
TEST-FILE-UNSELECTED, UNSTUBBED-OUTBOUND-INPUT, VACUOUS-EMPTY-EQUALITY,
WRITE-RESPONSE-UNASSERTED, WRITE-RESPONSE-UNRESOLVED.

Reads committed Git content and the named receipts only; writes nothing and
uses no network.
Refusals: tests-invalid-arguments, tests-invalid-revision,
tests-invalid-option, tests-source-unavailable, tests-bound-exceeded,
tests-decisions-invalid, tests-declaration-invalid, tests-output-unavailable,
tests-join-revision-unbound, tests-join-revision-mismatch,
tests-join-selector-malformed, tests-join-selector-unknown.
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
		return testsRefuse(diagnostic, "tests-invalid-arguments", "usage: corvint tests setup-select|lint|duplicates|validity|teardown|unselected|role-locators|env|citations|example-join")
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
	return testsRefuse(diagnostic, "tests-invalid-arguments", "usage: corvint tests setup-select|lint|duplicates|validity|teardown|unselected|role-locators|env|citations|example-join")
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

// runTestsValidity prints the static test-validity findings of the named committed files
// (TSA-V0-071) and the advisory seed parity of each --legacy link (TSA-V0-108). It exits 1 when a
// finding refuses a test as proof; --advisory applies as on lint (GVA-V0-094).

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

// gateAdvisory holds the --advisory option of a gate-shaped tests command: it exits 0 where the
// command would exit 1, and an error still exits 2 (GVA-V0-001). The agreement log and verdict
// comparison (--compare, --log, --label, tests agreement) were retired by decision 0594.
type gateAdvisory struct{ advisory bool }

const gateAgreementUsage = "[--advisory]"

// splitGateAgreement removes --advisory from args and returns the rest in order; it may be given
// once and takes no value.
func splitGateAgreement(args []string) (gateAdvisory, []string, error) {
	g, rest := gateAdvisory{}, []string{}
	for _, arg := range args {
		name, _, inline := strings.Cut(arg, "=")
		if name != "--advisory" {
			rest = append(rest, arg)
			continue
		}
		if inline || g.advisory {
			return g, nil, errors.New("--advisory takes no value and is given once")
		}
		g.advisory = true
	}
	return g, rest, nil
}

// finish returns the status to exit with: 0 for an advisory run's 1.
func (g gateAdvisory) finish(code int) int {
	if g.advisory && code == 1 {
		return 0
	}
	return code
}
