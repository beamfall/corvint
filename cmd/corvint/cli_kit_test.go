// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/clitree"
)

// cliKitRow is one corvint command in the CLI help and refusal conformance kit (CAP-V0-055 to
// CAP-V0-058). path is the command's words ("" is the root); refusal is the code an unknown
// option is refused with, or cliKitStdoutVerdict for the one command whose frozen contract prints
// its refusal verdict on stdout. refuse lists further argument vectors after the path that the
// command refuses with the same code; helpAnywhere lists vectors after the path that still print
// the path's own help (GPK-V0-062, GPK-V0-064); notHelp lists vectors where --help is a value or
// follows "--" and so is not a help request. ownsRoot marks a verb that takes its own --root
// after the verb (AHI-373), so a global --root before it is refused, even with --help.
// helpContains lists text the path's help must carry beyond the generic checks (a flag or usage
// form a reader relies on); processRefuse lists vectors after the path that a separate corvint
// process (the second entry point, candidateCommand) must refuse with the row's code.
type cliKitRow struct {
	path          string
	refusal       string
	ownsRoot      bool
	refuse        [][]string
	helpAnywhere  [][]string
	notHelp       [][]string
	helpContains  []string
	processRefuse [][]string
}

// cliKitStdoutVerdict marks `cem verify-stable`, whose stable contract (CEM-V1-017) prints a
// verification verdict on stdout and exits 2 rather than writing a refusal envelope on stderr.
const cliKitStdoutVerdict = "stdout-verdict"

// cliKitUnknownOption is an option no corvint command declares.
const cliKitUnknownOption = "--definitely-unknown-option"

// cliKitInvalidRoot is a --root value that is not a repository; help never opens it.
const cliKitInvalidRoot = "/definitely/not/a/repository"

// cliKitTable is the kit's one table: every node of the corvint command tree, plus the root.
// TestCLIKit fails when a tree node has no row or a row names no node, so a new command ships
// with its row.
var cliKitTable = []cliKitRow{
	{path: "", refusal: "invalid-arguments",
		refuse:       [][]string{{"help", "not-a-command"}, {"version"}},
		helpAnywhere: [][]string{{"--help", "context"}, {"-h", "context"}},
		helpContains: []string{"\n    cem  ", "\n    test-plan  ", "\n    adopt  ", "\n    init  alias of adopt\n"}},
	{path: "adopt", refusal: "invalid-arguments", refuse: [][]string{{"--revision"}},
		helpContains: []string{"--full-receipt"}, processRefuse: [][]string{{"--unknown"}}},
	{path: "affected", refusal: "invalid-arguments"},
	{path: "affected audit", refusal: "invalid-arguments"},
	{path: "affected cache", refusal: "invalid-arguments"},
	{path: "affected discovery", refusal: "invalid-arguments"},
	{path: "affected jobs", refusal: "invalid-arguments"},
	{path: "cem", refusal: "invalid-arguments", refuse: [][]string{{}}},
	{path: "cem begin", refusal: "invalid-arguments"},
	{path: "cem cite", refusal: "invalid-arguments"},
	{path: "cem cover", refusal: "invalid-arguments"},
	{path: "cem mark", refusal: "invalid-arguments"},
	{path: "cem prepare", refusal: "invalid-arguments"},
	{path: "cem report", refusal: "invalid-arguments"},
	{path: "cem status", refusal: "invalid-arguments", refuse: [][]string{{"--bogus", "x"}}},
	{path: "cem verify", refusal: "invalid-arguments"},
	{path: "cem verify-stable", refusal: cliKitStdoutVerdict},
	{path: "context", refusal: "invalid-arguments",
		helpAnywhere: [][]string{{"--task", "x", "--help"}, {"--help", "--task", "x"}, {"--limit", "-1", "--help"},
			{"--task", "x", "--subject", "a.go", "--help"}, {"--task", "x", "-h"}},
		notHelp: [][]string{{"--task", "--help"}, {"--task=--help"}, {"--task", "x", "--", "--help"}}},
	{path: "docs", refusal: "not-installed", helpAnywhere: [][]string{{"maintain", "--page", "p", "--help"}}},
	{path: "feature", refusal: "invalid-arguments"},
	{path: "features", refusal: "invalid-arguments"},
	{path: "flows", refusal: "not-installed"},
	{path: "flows appmap", refusal: "not-installed"},
	{path: "flows appmap build", refusal: "not-installed"},
	{path: "flows test-evidence", refusal: "invalid-arguments"},
	{path: "how", refusal: "not-installed"},
	{path: "impact", refusal: "invalid-arguments",
		helpAnywhere: [][]string{{"path/a.go", "--help"}}, notHelp: [][]string{{"--", "--help"}}},
	{path: "index", refusal: "invalid-arguments", helpAnywhere: [][]string{{"--if-stale", "--help"}}},
	{path: "init", refusal: "invalid-arguments", refuse: [][]string{{"--revision"}},
		helpContains: []string{"--full-receipt"}, processRefuse: [][]string{{"--revision"}}},
	{path: "mcp", refusal: "invalid-arguments", ownsRoot: true},
	{path: "overview", refusal: "invalid-arguments"},
	{path: "review", refusal: "invalid-arguments"},
	{path: "runs", refusal: "runs-invalid-arguments"},
	{path: "runs baseline", refusal: "runs-invalid-arguments"},
	{path: "runs baseline diff", refusal: "runs-invalid-arguments"},
	{path: "runs baseline record", refusal: "runs-invalid-arguments"},
	{path: "runs compare", refusal: "runs-invalid-arguments"},
	{path: "runs redundancy", refusal: "runs-invalid-arguments"},
	{path: "runs response-wait", refusal: "runs-invalid-arguments"},
	{path: "runs reusable", refusal: "runs-invalid-arguments"},
	{path: "runs server-errors", refusal: "runs-invalid-arguments"},
	{path: "runs stability", refusal: "runs-invalid-arguments"},
	{path: "test", refusal: "invalid-arguments"},
	{path: "test reuse", refusal: "invalid-arguments"},
	{path: "test reuse decide", refusal: "invalid-arguments"},
	{path: "test reuse record", refusal: "invalid-arguments"},
	{path: "test shards", refusal: "invalid-arguments"},
	{path: "test verify-shards", refusal: "invalid-arguments"},
	{path: "test-evidence", refusal: "invalid-arguments",
		refuse:       [][]string{{}, {"--explain", "x"}, {"x", "--lines", "0:0"}},
		helpContains: []string{"--explain"}},
	{path: "test-plan", refusal: "test-plan-invalid-arguments", refuse: [][]string{{}},
		helpContains: []string{"test-plan consolidate --input FILE"}},
	{path: "test-plan check", refusal: "test-plan-invalid-arguments"},
	{path: "test-plan consolidate", refusal: "test-plan-invalid-arguments"},
	{path: "test-plan page-state", refusal: "test-plan-invalid-arguments"},
	{path: "test-plan prefixes", refusal: "test-plan-invalid-arguments"},
	{path: "test-validity", refusal: "invalid-arguments", notHelp: [][]string{{"--receipt", "x", "--", "--help"}}},
	{path: "test-validity negate", refusal: "invalid-arguments"},
	{path: "tests", refusal: "tests-invalid-arguments"},
	{path: "tests citations", refusal: "tests-invalid-arguments"},
	{path: "tests duplicates", refusal: "tests-invalid-arguments"},
	{path: "tests env", refusal: "tests-invalid-arguments"},
	{path: "tests example-join", refusal: "tests-invalid-arguments"},
	{path: "tests lint", refusal: "tests-invalid-arguments"},
	{path: "tests role-locators", refusal: "tests-invalid-arguments"},
	{path: "tests setup-select", refusal: "tests-invalid-arguments"},
	{path: "tests teardown", refusal: "tests-invalid-arguments"},
	{path: "tests unselected", refusal: "tests-invalid-arguments"},
	{path: "tests validity", refusal: "tests-invalid-arguments"},
	{path: "watch", refusal: "watch-invalid-arguments"},
}

// cliKitInvocation is one in-process run of the corvint CLI with a stdin that records reads.
type cliKitInvocation struct {
	exit           int
	stdout, stderr string
	reads          int
}

func cliKitRun(args []string) cliKitInvocation {
	reader := &forbiddenHelpReader{}
	var stdout, stderr bytes.Buffer
	exit := run(args, reader, &stdout, &stderr)
	return cliKitInvocation{exit: exit, stdout: stdout.String(), stderr: stderr.String(), reads: reader.reads}
}

// TestCLIKit is the CLI help and refusal conformance kit (CAP-V0-055 to CAP-V0-058). From the one
// table above it proves, for the root and every command-tree node: every help form prints that
// node's own help and nothing else (CAP-V0-056), every argument refusal exits 2 with one closed,
// argument-class diagnostic and no output (CAP-V0-057), and the table covers exactly the tree
// (CAP-V0-055). Run one command with -run 'TestCLIKit/^cem_begin$'.
func TestCLIKit(t *testing.T) {
	// PATH is an empty directory, so the docs, flows and how shims find no corvint-docs on any
	// host and their refusal is the same everywhere. No help or refusal path the kit drives starts
	// Git (the kit passes with no Git on PATH), so nothing is linked or copied in and no platform
	// skips it. The repository fixture is made before PATH narrows.
	root := cliRepository(t)
	t.Setenv("PATH", t.TempDir())

	tree := commandTree()
	t.Run("CAP-V0-055 the table covers exactly the command tree", func(t *testing.T) {
		for _, problem := range clitree.Problems(tree) {
			t.Errorf("command tree problem: %s", problem)
		}
		seen := map[string]bool{}
		for _, node := range tree.Nodes {
			if path := strings.Join(node.Path, " "); seen[path] {
				t.Errorf("command tree lists node %q twice", path)
			} else {
				seen[path] = true
			}
		}
		rows := map[string]bool{}
		for _, row := range cliKitTable {
			if rows[row.path] {
				t.Errorf("row %q is listed twice", row.path)
			}
			rows[row.path] = true
		}
		nodes := map[string]bool{"": true}
		for _, node := range tree.Nodes {
			path := strings.Join(node.Path, " ")
			nodes[path] = true
			if !rows[path] {
				t.Errorf("command %q has no cliKitTable row; add one with its refusal code", path)
			}
		}
		for path := range rows {
			if !nodes[path] {
				t.Errorf("cliKitTable row %q names no command-tree node", path)
			}
		}
		for _, command := range topLevelCommands {
			if !rows[command] {
				t.Errorf("dispatched verb %q has no cliKitTable row", command)
			}
		}
	})

	for _, row := range cliKitTable {
		path := strings.Fields(row.path)
		name := row.path
		if name == "" {
			name = "root"
		}
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			cliKitHelp(t, tree.Nodes, row, path)
			cliKitRefusals(t, root, row, path)
		})
	}
}

// cliKitHelp checks CAP-V0-056 for one row.
func cliKitHelp(t *testing.T, nodes []clitree.Node, row cliKitRow, path []string) {
	t.Helper()
	want := rootHelp
	if len(path) > 0 {
		topic, ok, err := parseHelpInvocation(append(slices.Clone(path), "--help"))
		if err != nil || !ok {
			t.Fatalf("%q --help: no help topic (%v)", path, err)
		}
		own := topic == nodeHelpPrefix+row.path || slices.Equal(helpTopicPath(topic), path) ||
			onlyChildTopic(commandTree(), path, helpTopicPath(topic))
		if !own {
			t.Fatalf("%q --help resolves to topic %q, not its own command", path, topic)
		}
		want = helpText(topic)
		if !strings.Contains(want, " "+row.path) || !strings.Contains(want, "Example") || !strings.Contains(want, "Required:") {
			t.Errorf("%q help lacks its usage, its required inputs or an example:\n%s", path, want)
		}
		for _, node := range nodes {
			if len(node.Path) == len(path)+1 && slices.Equal(node.Path[:len(path)], path) &&
				!strings.Contains(want, node.Path[len(path)]) {
				t.Errorf("%q help does not name its subcommand %q", path, node.Path[len(path)])
			}
		}
		if len(path) == 1 {
			cliKitHouseFormat(t, row.path, want)
		}
	}
	if !strings.HasSuffix(want, "\n") {
		t.Errorf("%q help lacks a terminal LF", path)
	}
	for _, text := range row.helpContains {
		if !strings.Contains(want, text) {
			t.Errorf("%q help does not contain %q", path, text)
		}
	}
	var forms [][]string
	for _, prefix := range [][]string{nil, {"--root", cliKitInvalidRoot}, {"--root=" + cliKitInvalidRoot}} {
		for _, flag := range []string{"--help", "-h"} {
			args := slices.Concat(prefix, path, []string{flag})
			if row.ownsRoot && prefix != nil {
				cliKitEnvelope(t, args, cliKitRun(args), "invalid-arguments")
				continue
			}
			forms = append(forms, args)
		}
		forms = append(forms, slices.Concat(prefix, []string{"help"}, path))
		for _, tail := range row.helpAnywhere {
			forms = append(forms, slices.Concat(prefix, path, tail))
		}
	}
	for _, args := range forms {
		got := cliKitRun(args)
		if got.exit != 0 || got.stdout != want || got.stderr != "" || got.reads != 0 {
			t.Errorf("%q: exit %d stderr %q stdin reads %d, help matches %v", args, got.exit, got.stderr, got.reads, got.stdout == want)
		}
	}
	for _, tail := range row.notHelp {
		args := slices.Concat(path, tail)
		if topic, requested, err := parseHelpInvocation(args); requested || err != nil {
			t.Errorf("%q is read as a help request for %q (%v)", args, topic, err)
		}
	}
}

// cliKitHouseFormat checks CCF-V1-650: a verb's help opens with a one-line summary sentence, a
// blank line, a Usage: heading and an indented corvint invocation.
func cliKitHouseFormat(t *testing.T, verb, text string) {
	t.Helper()
	lines := strings.SplitN(text, "\n", 5)
	if len(lines) < 5 {
		t.Errorf("%s help has %d lines, want a summary, a blank line, Usage: and an invocation", verb, len(lines))
		return
	}
	if summary := lines[0]; summary == "" || strings.HasPrefix(summary, " ") || !strings.HasSuffix(summary, ".") {
		t.Errorf("%s help summary %q is not one unindented sentence ending in a period", verb, summary)
	}
	if lines[1] != "" || lines[2] != "Usage:" || !strings.HasPrefix(lines[3], "  corvint ") {
		t.Errorf("%s help does not continue with a blank line, Usage: and a corvint invocation: %q", verb, lines[1:4])
	}
}

// cliKitRefusals checks CAP-V0-057 for one row.
func cliKitRefusals(t *testing.T, root string, row cliKitRow, path []string) {
	t.Helper()
	tails := append([][]string{{cliKitUnknownOption}}, row.refuse...)
	for _, tail := range tails {
		args := slices.Concat([]string{"--root", root}, path, tail)
		got := cliKitRun(args)
		if row.refusal == cliKitStdoutVerdict {
			cliKitStdoutRefusal(t, args, got)
			continue
		}
		cliKitEnvelope(t, args, got, row.refusal)
	}
	for _, tail := range row.processRefuse {
		args := slices.Concat([]string{"--root", root}, path, tail)
		result := execute(t, candidateCommand(args...))
		cliKitEnvelope(t, args, cliKitInvocation{exit: result.exit, stdout: string(result.stdout), stderr: string(result.stderr)}, row.refusal)
	}
	// Under a root that is not a repository the command still refuses with an envelope, before or
	// after it would open the root, and never reads stdin.
	args := slices.Concat([]string{"--root", cliKitInvalidRoot}, path, []string{cliKitUnknownOption})
	got := cliKitRun(args)
	if row.refusal == cliKitStdoutVerdict {
		cliKitStdoutRefusal(t, args, got)
	} else {
		cliKitEnvelope(t, args, got, "")
	}
	if len(path) == 1 {
		// GPK-V0-064 as amended by decision 0181: a bare --root followed by an option-like token
		// is refused as a missing value before every verb.
		for _, args := range [][]string{{"--root", "--task", path[0]}, {"--root", "-h", path[0]}} {
			got := cliKitRun(args)
			cliKitEnvelope(t, args, got, "invalid-arguments")
			if !strings.Contains(got.stderr, "missing value for --root") {
				t.Errorf("%q: stderr %q does not name the missing --root value", args, got.stderr)
			}
		}
	}
	if len(path) == 0 {
		got := cliKitRun([]string{"help", "not-a-command"})
		if !strings.Contains(got.stderr, "unknown help topic: 'not-a-command' (choose from ") {
			t.Errorf("unknown help topic: stderr %q", got.stderr)
		}
		for _, command := range topLevelCommands {
			if !strings.Contains(got.stderr, "'"+command+"'") {
				t.Errorf("unknown help topic refusal does not list %q: %q", command, got.stderr)
			}
		}
	}
}

// cliKitEnvelope checks one refusal: exit 2, nothing on stdout, no stdin read, and exactly one
// stderr line holding a JSON object with ok false and an argument-class code. want, when set, is
// the exact code.
func cliKitEnvelope(t *testing.T, args []string, got cliKitInvocation, want string) {
	t.Helper()
	if got.exit != 2 || got.stdout != "" || got.reads != 0 {
		t.Errorf("%q: exit %d stdout %q stdin reads %d, want a refusal", args, got.exit, got.stdout, got.reads)
		return
	}
	var envelope struct {
		OK   *bool  `json:"ok"`
		Code string `json:"code"`
	}
	line, rest, _ := strings.Cut(got.stderr, "\n")
	if rest != "" || json.Unmarshal([]byte(line), &envelope) != nil || envelope.OK == nil || *envelope.OK {
		t.Errorf("%q: stderr %q is not one refusal envelope line", args, got.stderr)
		return
	}
	if want != "" && envelope.Code != want {
		t.Errorf("%q: code %q, want %q (stderr %q)", args, envelope.Code, want, got.stderr)
	}
	if want != "" && !cliKitArgumentCode(envelope.Code) {
		t.Errorf("%q: code %q is not an argument refusal code", args, envelope.Code)
	}
	if want == "" && envelope.Code == "internal-error" {
		t.Errorf("%q: an argument refusal is reported as internal-error: %q", args, got.stderr)
	}
}

// cliKitArgumentCode is the closed set of codes an argument refusal may carry: the shared
// invalid-arguments code, a command's own COMMAND-invalid-arguments code, or not-installed for a
// shim whose bundled app is absent.
func cliKitArgumentCode(code string) bool {
	return code == "invalid-arguments" || code == "not-installed" ||
		strings.HasSuffix(code, "-invalid-arguments") && len(code) > len("-invalid-arguments")
}

// cliKitStdoutRefusal checks the stable verifier's refusal: exit 2, no stderr, no stdin read and
// one JSON object on stdout.
func cliKitStdoutRefusal(t *testing.T, args []string, got cliKitInvocation) {
	t.Helper()
	var verdict map[string]any
	if got.exit != 2 || got.stderr != "" || got.reads != 0 || json.Unmarshal([]byte(got.stdout), &verdict) != nil {
		t.Errorf("%q: exit %d stderr %q stdin reads %d stdout %q, want a stable verdict", args, got.exit, got.stderr, got.reads, got.stdout)
	}
}
