package clitree

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const sampleHelp = `Run the sample tool.

Usage:
  sample build --out FILE [--force] [--jobs N]
  sample (start | stop) --name NAME
  sample cache <list|clear> [--all]
  sample deploy --target ENV
    [--dry-run] [--retries N]
`

func sampleTree() Tree {
	builder := NewBuilder("sample")
	builder.AddText(sampleHelp, "")
	builder.Declare([]string{"build"}, "Build the thing")
	builder.Example([]string{"deploy"}, "sample deploy --target staging")
	return builder.Tree("sample help --json", "test", []string{"cmd/sample"}, []string{"--root"},
		[]Internal{{Word: "worker", Reason: "self re-entry"}, {Word: "--secret", Reason: "worker option"}})
}

func node(t *testing.T, tree Tree, path ...string) Node {
	t.Helper()
	found, ok := tree.Find(path)
	if !ok || !slices.Equal(found.Path, path) {
		t.Fatalf("no node %v", path)
	}
	return found
}

// TestCAPV0040TreeDerivesFromHelp checks that synopses become nodes at every depth, with
// alternatives expanded, continuation lines joined, flags, required inputs and examples derived.
func TestCAPV0040TreeDerivesFromHelp(t *testing.T) {
	tree := sampleTree()
	if tree.Schema != Schema || tree.Binary != "sample" {
		t.Fatalf("header %q %q", tree.Schema, tree.Binary)
	}
	var paths []string
	for _, n := range tree.Nodes {
		paths = append(paths, strings.Join(n.Path, " "))
	}
	for _, want := range []string{"build", "start", "stop", "cache", "cache list", "cache clear", "deploy"} {
		if !slices.Contains(paths, want) {
			t.Errorf("missing node %q in %v", want, paths)
		}
	}
	build := node(t, tree, "build")
	if build.Summary != "Build the thing" || !slices.Equal(build.Required, []string{"--out FILE"}) ||
		!slices.Contains(build.Flags, "--force") || !slices.Contains(build.Flags, "--jobs") || build.Example != "sample build --out FILE" {
		t.Errorf("build node %+v", build)
	}
	deploy := node(t, tree, "deploy")
	if !slices.Contains(deploy.Flags, "--dry-run") || deploy.Example != "sample deploy --target staging" {
		t.Errorf("deploy node %+v", deploy)
	}
	if problems := Problems(tree); len(problems) != 0 {
		t.Errorf("problems %v", problems)
	}
	broken := tree
	broken.Nodes = append(slices.Clone(tree.Nodes), Node{Path: []string{"ghost"}, Command: "sample ghost"})
	if got := Problems(broken); len(got) != 3 {
		t.Errorf("ghost node problems %v", got)
	}
	unexplained := tree
	unexplained.Internal = append(slices.Clone(tree.Internal), Internal{Word: "secret-verb"})
	if got := Problems(unexplained); len(got) != 1 || !strings.Contains(got[0], "secret-verb") {
		t.Errorf("internal entry without a reason: problems %v", got)
	}
}

// TestCAPV0042FindRenderAndLeadingWords checks the deepest-prefix lookup a --help request uses and
// that a node's rendered help carries usage, flags, required inputs and its example.
func TestCAPV0042FindRenderAndLeadingWords(t *testing.T) {
	tree := sampleTree()
	words := LeadingWords([]string{"--root", "/r", "cache", "clear", "--all", "--help"}, map[string]bool{"--root": true})
	if !slices.Equal(words, []string{"cache", "clear"}) {
		t.Fatalf("leading words %v", words)
	}
	found, ok := tree.Find(append(words, "extra"))
	if !ok || !slices.Equal(found.Path, []string{"cache", "clear"}) {
		t.Fatalf("find %v", found.Path)
	}
	text := Render(tree, found)
	for _, part := range []string{"Usage:", "sample cache clear [--all]", "Flags: --all", "Required: none", "Example:", "sample help --json"} {
		if !strings.Contains(text, part) {
			t.Errorf("render lacks %q:\n%s", part, text)
		}
	}
	if _, ok := tree.Find([]string{"unknown"}); ok {
		t.Error("found a node for an unknown word")
	}
}

// TestCAPV0043WordsAndFlagsVocabulary checks the vocabulary drift is judged against.
func TestCAPV0043WordsAndFlagsVocabulary(t *testing.T) {
	tree := sampleTree()
	words, flags := tree.Words(), tree.AllFlags()
	for _, word := range []string{"build", "cache", "clear", "worker"} {
		if !words[word] {
			t.Errorf("word %q missing", word)
		}
	}
	for _, flag := range []string{"--out", "--retries", "--root", "--secret", "--help", "-h"} {
		if !flags[flag] {
			t.Errorf("flag %q missing", flag)
		}
	}
	if words["deploys"] || flags["--nope"] {
		t.Error("vocabulary invents words")
	}
	first, err := Encode(tree)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := Encode(sampleTree())
	if string(first) != string(second) || !strings.HasSuffix(string(first), "}\n") {
		t.Error("encoding is not stable")
	}
}

// TestCAPV0041ScanFindsDispatch checks the AST scan finds switch, comparison, table and flag-set
// dispatch at fixed argument positions, through assignments and var declarations alike, and
// ignores test files and unrelated strings.
func TestCAPV0041ScanFindsDispatch(t *testing.T) {
	dir := t.TempDir()
	source := `package main

import "flag"

var verbs = map[string]func(){"table-verb": nil}

func dispatch(args []string) {
	switch args[0] {
	case "switch-verb", "--switch-flag":
	}
	if args[1] == "compared-verb" {
	}
	verbs[args[0]]()
	for _, arg := range args {
		if arg == "--loop-flag" {
		}
	}
	set := flag.NewFlagSet("x", flag.ContinueOnError)
	set.String("set-flag", "", "")
	var sub = args[0]
	switch sub {
	case "declared-verb":
	}
	var declared = flag.NewFlagSet("y", flag.ContinueOnError)
	declared.Bool("declared-flag", false, "")
	message := "not-a-verb"
	_ = message
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main_test.go"), []byte("package main\n\nfunc x(args []string) bool { return args[0] == \"test-verb\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scan, err := ScanDirs(dir)
	if err != nil {
		t.Fatal(err)
	}
	values := func(literals []Literal) []string {
		var out []string
		for _, literal := range literals {
			out = append(out, literal.Value)
		}
		return out
	}
	words, flags := values(scan.Words), values(scan.Flags)
	if !slices.Equal(words, []string{"compared-verb", "declared-verb", "switch-verb", "table-verb"}) {
		t.Errorf("words %v", words)
	}
	if !slices.Equal(flags, []string{"--declared-flag", "--loop-flag", "--set-flag", "--switch-flag"}) {
		t.Errorf("flags %v", flags)
	}
}

// TestCAPV0047ChildArgvIsNotDispatch checks that a slice a function builds or hands whole to a
// child process (exec.Command, exec.CommandContext, syscall.Exec, a Cmd's Args, through append),
// and an argv parameter every caller fills from a non-command-line value (wholeGoTest), are not
// scanned as this binary's arguments, while a parameter it only dispatches on, one that receives
// the command line or escapes into a handler table, a single element handed to a child, a
// closure's same-named parameter and its own command line re-executed (os.Args, flag.Args(), an
// append of them) still are (CAP-V0-200, V1-3153).
func TestCAPV0047ChildArgvIsNotDispatch(t *testing.T) {
	dir := t.TempDir()
	source := `package main

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"syscall"
)

func runGit(argv []string) {
	if argv[0] == "log" {
	}
	for _, arg := range argv {
		if arg == "--no-pager" {
		}
	}
	_ = exec.Command("git", argv...)
}

func runContext(ctx context.Context, args []string) {
	switch args[0] {
	case "context-child":
	}
	_ = exec.CommandContext(ctx, args[0], args[1:]...)
}

func runField(argv []string) {
	if argv[0] == "field-child" {
	}
	command := exec.Command("x")
	command.Args = argv
}

func runLiteral(argv []string) {
	if argv[0] == "literal-field-child" {
	}
	_ = exec.Cmd{Path: "x", Args: argv}
}

func runExec(argv []string) {
	if argv[1] == "--exec-child" {
	}
	_ = syscall.Exec("/bin/x", argv, nil)
}

func built() {
	args := []string{"status", "--short"}
	if args[0] == "built-child" {
	}
	var cmdArgs = []string{"x"}
	if cmdArgs[0] == "var-child" {
	}
}

func runAppended(dir string, argv []string) {
	if argv[0] == "appended-child" {
	}
	_ = exec.Command("git", append([]string{"-C", dir}, argv...)...)
}

func reexec() {
	args := os.Args[1:]
	if args[0] == "reexec-verb" {
	}
	_ = exec.Command(os.Args[0], args...)
	rest := flag.Args()
	if rest[0] == "flag-args-verb" {
	}
	_ = exec.Command("x", rest...)
}

type invocation struct{ Argv []string }

// wholeGoTest is only ever handed a recorded child command, never this binary's arguments.
func wholeGoTest(argv []string) bool {
	if argv[0] != "go" || argv[1] != "test" {
		return false
	}
	for _, word := range argv[2:] {
		if word == "-run" {
			return false
		}
	}
	return true
}

func checkRecorded(recorded invocation) bool { return wholeGoTest(recorded.Argv) }

// forwarded receives this binary's own arguments from Main, so it is dispatch.
func forwarded(args []string) {
	if args[0] == "forwarded-verb" {
	}
}

func handler(args []string) {
	if args[0] == "handler-verb" {
	}
}

var handlers = map[string]func([]string){"h": handler}

func Main(args []string) {
	forwarded(args[1:])
}

// Run's parameter is not argument-named, but it carries the command line into routed.
func Run(input []string) { routed(input) }

func routed(args []string) {
	if args[0] == "input-verb" {
	}
}

func nested(args []string) {
	helper := func(args []string) { _ = exec.Command("git", args...) }
	_ = helper
	if args[0] == "nested-verb" {
	}
}

func element(args []string) {
	switch args[0] {
	case "elem-verb":
		_ = exec.Command("xdg-open", args[1])
	}
}

func prepended() {
	words := []string{}
	words = append(words, os.Args[1:]...)
	if words[0] == "appended-own" {
	}
	_ = exec.Command("x", words...)
}

func dispatch(argv []string) {
	if argv[0] == "real-verb" {
	}
	for _, arg := range argv {
		if arg == "--real-flag" {
		}
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	scan, err := ScanDirs(dir)
	if err != nil {
		t.Fatal(err)
	}
	var words, flags []string
	for _, literal := range scan.Words {
		words = append(words, literal.Value)
	}
	for _, literal := range scan.Flags {
		flags = append(flags, literal.Value)
	}
	if want := []string{"appended-own", "elem-verb", "flag-args-verb", "forwarded-verb", "handler-verb", "input-verb", "nested-verb", "real-verb", "reexec-verb"}; !slices.Equal(words, want) {
		t.Errorf("words %v, want %v", words, want)
	}
	if !slices.Equal(flags, []string{"--real-flag"}) {
		t.Errorf("flags %v, want only --real-flag", flags)
	}
}

// TestCAPV0931AnswerRoutesHelpOnly checks that Answer serves help --json, root help, help PATH and
// a node's --help or -h, refuses an unknown help path, and leaves every other invocation alone.
func TestCAPV0931AnswerRoutesHelpOnly(t *testing.T) {
	tree := sampleTree()
	const root = "sample root help\n"
	encoded, err := Encode(tree)
	if err != nil {
		t.Fatal(err)
	}
	build := Render(tree, node(t, tree, "build"))
	clear := Render(tree, node(t, tree, "cache", "clear"))
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"help", "--json"}, string(encoded)},
		{[]string{"help"}, root},
		{[]string{"--help"}, root},
		{[]string{"-h"}, root},
		{[]string{"help", "build"}, build},
		{[]string{"help", "cache", "clear", "--help"}, clear},
		{[]string{"build", "--help"}, build},
		{[]string{"build", "--out", "x", "-h"}, build},
		{[]string{"cache", "clear", "--all", "--help"}, clear},
	} {
		var out strings.Builder
		handled, err := Answer(tree, tc.args, root, &out)
		if !handled || err != nil || out.String() != tc.want {
			t.Fatalf("%v: handled=%t err=%v\n%s", tc.args, handled, err, out.String())
		}
	}
	for _, args := range [][]string{{"help", "nope"}, {"help", "build", "extra"}, {"nope", "--help"}} {
		var out strings.Builder
		handled, err := Answer(tree, args, root, &out)
		if !handled || err == nil || !strings.Contains(err.Error(), "run `sample help`") || out.Len() != 0 {
			t.Fatalf("%v: handled=%t err=%v out=%q", args, handled, err, out.String())
		}
	}
	for _, args := range [][]string{nil, {}, {"build", "--out", "x"}, {"build", "--", "--help"}, {"help-me"}, {"--bogus", "--help"}} {
		var out strings.Builder
		if handled, err := Answer(tree, args, root, &out); handled || err != nil || out.Len() != 0 {
			t.Fatalf("%v answered: handled=%t err=%v out=%q", args, handled, err, out.String())
		}
	}
}
