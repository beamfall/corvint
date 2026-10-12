package shell

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func TestOwnsOnlyShellSuffixes_NSAV0006(t *testing.T) {
	language := New()
	for _, relative := range []string{"a.sh", "lib/a.bash", "z/a.zsh", "k/a.ksh", "test/a.bats"} {
		if !language.Owns(relative) {
			t.Fatalf("%s should be owned", relative)
		}
	}
	for _, relative := range []string{"script/release", "a.shx", "a.py", "a.fish", "Makefile", "a.sh.txt"} {
		if language.Owns(relative) {
			t.Fatalf("%s must not be owned", relative)
		}
	}
}

func TestClassifiesTestsAndHelpers_NSAV0007(t *testing.T) {
	root := t.TempDir()
	write(t, root, "suite.bats", "@test \"x\" { true; }\n")
	write(t, root, "script/a_test.sh", "true\n")
	write(t, root, "script/a-test.bash", "true\n")
	write(t, root, "script/test_a.sh", "true\n")
	write(t, root, "script/test-a.zsh", "true\n")
	write(t, root, "script/contest.sh", "true\n")
	write(t, root, "script/testing.sh", "true\n")
	write(t, root, "tests/run.sh", "#!/bin/sh\nset -e\n[ \"$(echo x)\" = x ]\n")
	write(t, root, "tests/helpers.sh", "#!/bin/bash\n# helpers\nset -euo pipefail\nROOT=$(cd \"$(dirname \"$0\")/..\" && pwd)\nsource ./common.sh\nassert_eq() {\n  [ \"$1\" = \"$2\" ] || exit 1\n}\nfunction fail {\n  echo \"$*\" >&2\n  exit 1\n}\n")
	write(t, root, "tests/common.sh", "export X=1\nreadonly Y=2\n")
	write(t, root, "test/guarded.sh", "if [ -z \"$X\" ]; then\n  X=1\nfi\n")
	result := units(t, root)
	for _, test := range []string{"suite.bats", "script/a_test.sh", "script/a-test.bash", "script/test_a.sh", "script/test-a.zsh", "tests/run.sh", "test/guarded.sh"} {
		if unit := unitOf(t, result, test); !slices.Equal(unit.Tests, []string{test}) || len(unit.Sources) != 0 {
			t.Fatalf("%s should be a test: %+v", test, unit)
		}
	}
	for _, source := range []string{"script/contest.sh", "script/testing.sh", "tests/helpers.sh", "tests/common.sh"} {
		if unit := unitOf(t, result, source); !slices.Equal(unit.Sources, []string{source}) || len(unit.Tests) != 0 {
			t.Fatalf("%s should be a source: %+v", source, unit)
		}
	}
}

func TestResolvesEveryLiteralForm_NSAV0008(t *testing.T) {
	root := t.TempDir()
	for _, lib := range []string{"lib/root.sh", "script/near.sh", "script/dir0.sh", "script/strip0.sh", "script/bashsource.sh", "script/stripsource.sh", "script/assigned.sh", "lib/climbed.sh", "lib/toplevel.sh", "script/run-bash.sh", "script/run-sh.sh", "script/run-dot.sh", "script/run-var.sh", "script/quiet.sh", "lib/chained.sh", "lib/assigned-file.sh"} {
		write(t, root, lib, "true\n")
	}
	write(t, root, "script/user.sh", strings.Join([]string{
		"#!/usr/bin/env bash",
		"source lib/root.sh",
		". near.sh",
		"source \"$(dirname \"$0\")/dir0.sh\"",
		". \"${0%/*}/strip0.sh\"",
		"source \"$(dirname \"${BASH_SOURCE[0]}\")/bashsource.sh\"",
		"source \"${BASH_SOURCE%/*}/stripsource.sh\"",
		"HERE=\"$(dirname \"$0\")\"",
		"source \"$HERE/assigned.sh\"",
		"ROOT=$(cd \"$(dirname \"$0\")/..\" && pwd)",
		"source \"${ROOT}/lib/climbed.sh\"",
		"TOP=$(git rev-parse " + gitTopLevel + ")",
		". \"$TOP/lib/toplevel.sh\"",
		"LIB=\"$ROOT/lib\"",
		"source \"$LIB/chained.sh\"",
		"bash script/run-bash.sh --flag",
		"out=$(sh -e \"$HERE/run-sh.sh\")",
		"./run-dot.sh arg",
		"\"$HERE/run-var.sh\" >/dev/null 2>&1",
		"source_root=$(CDPATH='' cd -- \"$(dirname \"$0\")/..\" && pwd -P)",
		"tool=\"$source_root/lib/assigned-file.sh\"",
		"\"$tool\" --list",
		"SCRIPT_DIR=$(cd -P -- \"$(dirname -- \"${BASH_SOURCE[0]}\")\" >/dev/null 2>&1 && pwd -P)",
		"if ! \"$SCRIPT_DIR\"/quiet.sh; then exit 1; fi",
		"bash \"$ROOT/missing.sh\"",
		"source /etc/profile",
		"",
	}, "\n"))
	unit := unitOf(t, units(t, root), "script/user.sh")
	want := []string{
		"shell:lib/assigned-file.sh", "shell:lib/chained.sh", "shell:lib/climbed.sh", "shell:lib/root.sh", "shell:lib/toplevel.sh",
		"shell:script/assigned.sh", "shell:script/bashsource.sh", "shell:script/dir0.sh", "shell:script/near.sh",
		"shell:script/quiet.sh", "shell:script/run-bash.sh", "shell:script/run-dot.sh", "shell:script/run-sh.sh",
		"shell:script/run-var.sh", "shell:script/strip0.sh", "shell:script/stripsource.sh",
	}
	if !slices.Equal(unit.Imports, want) {
		t.Fatalf("imports=%v\nwant   %v", unit.Imports, want)
	}
	if len(unit.Frontier) != 0 {
		t.Fatalf("literal targets raised a frontier: %v", unit.Frontier)
	}
}

func TestReassignedVariableIsNotADirectoryForm_NSAV0008(t *testing.T) {
	root := t.TempDir()
	write(t, root, "lib/a.sh", "true\n")
	write(t, root, "use.sh", "DIR=$(dirname \"$0\")\nDIR=/tmp\nsource \"$DIR/lib/a.sh\"\n")
	unit := unitOf(t, units(t, root), "use.sh")
	if len(unit.Imports) != 0 || !slices.Equal(unit.Frontier, []string{FrontierDynamicSource}) {
		t.Fatalf("reassigned directory variable resolved: %+v", unit)
	}
}

func TestBatsLoadTriesTheBashSuffix_NSAV0008(t *testing.T) {
	root := t.TempDir()
	write(t, root, "test/test_helper.bash", "setup_common() { :; }\n")
	write(t, root, "test/helpers/extra.bash", "load ../test_helper\n")
	write(t, root, "test/plain", "")
	write(t, root, "test/suite.bats", "load test_helper\nload 'helpers/extra'\nload \"$BATS_TEST_DIRNAME/test_helper.bash\"\nsetup() {\n  DIR=\"$( cd \"$( dirname \"$BATS_TEST_FILENAME\" )\" >/dev/null 2>&1 && pwd )\"\n}\n@test \"x\" {\n  run bash \"$DIR/../script/tool.sh\"\n}\n")
	write(t, root, "script/tool.sh", "true\n")
	result := units(t, root)
	suite := unitOf(t, result, "test/suite.bats")
	want := []string{"shell:script/tool.sh", "shell:test/helpers/extra.bash", "shell:test/test_helper.bash"}
	if !slices.Equal(suite.Imports, want) || len(suite.Frontier) != 0 {
		t.Fatalf("suite=%+v want imports %v", suite, want)
	}
	if extra := unitOf(t, result, "test/helpers/extra.bash"); !slices.Equal(extra.Imports, []string{"shell:test/test_helper.bash"}) {
		t.Fatalf("helper load=%v", extra.Imports)
	}
}

func TestNonLiteralSourceRaisesDynamicSource_NSAV0009(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.sh", "for f in lib/*.sh; do source \"$f\"; done\n")
	write(t, root, "b.sh", ". \"$HOME/.profile\"\n")
	write(t, root, "c.bats", "load \"$HELPER\"\n@test x { true; }\n")
	write(t, root, "d.sh", "source\n")
	write(t, root, "e.sh", "# source \"$f\"\necho 'source $x'\ncat <<EOF\nsource $y\nEOF\n")
	result := units(t, root)
	for _, dynamicFile := range []string{"a.sh", "b.sh", "c.bats", "d.sh"} {
		if unit := unitOf(t, result, dynamicFile); !slices.Equal(unit.Frontier, []string{FrontierDynamicSource}) {
			t.Fatalf("%s frontier=%v", dynamicFile, unit.Frontier)
		}
	}
	if unit := unitOf(t, result, "e.sh"); len(unit.Frontier) != 0 {
		t.Fatalf("comment, string or heredoc raised a frontier: %v", unit.Frontier)
	}
	if len(result.Frontier) != 0 {
		t.Fatalf("unit frontier leaked into the global frontier: %v", result.Frontier)
	}
	graph := build(t, root)
	plan := affected.Select(graph, []string{"a.sh"})
	if !hasUnknown(plan, affected.UnknownLanguageFrontier, FrontierDynamicSource) {
		t.Fatalf("reached dynamic source is not named: %+v", plan.Unknown)
	}
}

func TestUndecodableTextRaisesUnreadableSource_NSAV0009(t *testing.T) {
	root := t.TempDir()
	write(t, root, "lib/ok.sh", "true\n")
	write(t, root, "tests/bad.sh", "source ../lib/ok.sh\n\xff\xfe\n")
	result := units(t, root)
	if !slices.Equal(result.Frontier, []string{FrontierUnreadableSource}) {
		t.Fatalf("frontier=%v", result.Frontier)
	}
	if bad := unitOf(t, result, "tests/bad.sh"); !slices.Equal(bad.Tests, []string{"tests/bad.sh"}) || len(bad.Imports) != 0 {
		t.Fatalf("unreadable unit=%+v", bad)
	}
	plan := affected.Select(build(t, root), []string{"lib/ok.sh"})
	if plan.Scope != affected.ScopeUnknown || !hasUnknown(plan, affected.UnknownLanguageFrontier, FrontierUnreadableSource) {
		t.Fatalf("plan scope=%s unknown=%+v", plan.Scope, plan.Unknown)
	}
}

func TestForeignSourceRaisesUnitFrontierOnSourcesOnly_NSAV0012(t *testing.T) {
	root := t.TempDir()
	write(t, root, "script/tool.sh", "true\n")
	write(t, root, "script/tool_test.sh", "bash script/tool.sh\n")
	write(t, root, "script/build_test.sh", "go test ./cmd\n")
	without := units(t, root)
	if unit := unitOf(t, without, "script/tool.sh"); len(unit.Frontier) != 0 || len(without.Frontier) != 0 {
		t.Fatalf("frontier without foreign source: unit=%v global=%v", unit.Frontier, without.Frontier)
	}
	write(t, root, "cmd/main.go", "package main\n")
	with := units(t, root)
	if len(with.Frontier) != 0 {
		t.Fatalf("foreign invocation must not be global: %v", with.Frontier)
	}
	if unit := unitOf(t, with, "script/tool.sh"); !slices.Equal(unit.Frontier, []string{FrontierForeignInvocation}) {
		t.Fatalf("source unit frontier=%v", unit.Frontier)
	}
	for _, test := range []string{"script/tool_test.sh", "script/build_test.sh"} {
		if unit := unitOf(t, with, test); len(unit.Frontier) != 0 {
			t.Fatalf("%s carries %v", test, unit.Frontier)
		}
	}
	graph := build(t, root)
	goOnly := affected.Select(graph, []string{"cmd/main.go"})
	if !slices.Contains(goOnly.SelectedTests(), "script/build_test.sh") {
		t.Fatalf("toolchain test not selected: %v", goOnly.SelectedTests())
	}
	if hasUnknown(goOnly, affected.UnknownLanguageFrontier, FrontierForeignInvocation) {
		t.Fatalf("a .go-only plan carries the foreign frontier: %+v", goOnly.Unknown)
	}
	shellPlan := affected.Select(graph, []string{"script/tool.sh"})
	if shellPlan.Scope != affected.ScopeUnknown || !hasUnknown(shellPlan, affected.UnknownLanguageFrontier, FrontierForeignInvocation) {
		t.Fatalf("dirty shell source plan scope=%s unknown=%+v", shellPlan.Scope, shellPlan.Unknown)
	}
}

func TestForeignSuffixesIncludeEveryPlannedLanguage_NSAV0012(t *testing.T) {
	for _, suffix := range []string{".go", ".py", ".rb", ".rs", ".swift", ".kt", ".cs", ".ts", ".mjs", ".c", ".h", ".mm"} {
		if !slices.Contains(ForeignSourceSuffixes, suffix) {
			t.Fatalf("missing %s", suffix)
		}
	}
	for _, suffix := range Suffixes {
		if slices.Contains(ForeignSourceSuffixes, suffix) {
			t.Fatalf("own suffix %s listed as foreign", suffix)
		}
	}
}

func TestPathTokensSelectANamedDataFileReader_NSAV0010(t *testing.T) {
	root := t.TempDir()
	write(t, root, "testdata/golden.json", "{}\n")
	write(t, root, "tools/gen.py", "print(1)\n")
	write(t, root, "script/golden_test.sh", "# names notes/ignored.txt only in a comment\ndiff -u \"$(dirname \"$0\")/../testdata/golden.json\" out.json\ngrep -q x tools/gen.py\n")
	write(t, root, "script/other_test.sh", "true\n")
	unit := unitOf(t, units(t, root), "script/golden_test.sh")
	for _, token := range []string{"/../testdata/golden.json", "out.json", "tools/gen.py"} {
		if !slices.Contains(unit.PathTokens, token) {
			t.Fatalf("token %s missing from %v", token, unit.PathTokens)
		}
	}
	if slices.Contains(unit.PathTokens, "notes/ignored.txt") {
		t.Fatalf("comment token kept: %v", unit.PathTokens)
	}
	graph := build(t, root)
	for _, dirty := range []string{"testdata/golden.json", "tools/gen.py"} {
		plan := affected.Select(graph, []string{dirty})
		if !slices.Equal(plan.SelectedTests(), []string{"script/golden_test.sh"}) {
			t.Fatalf("%s selected %v", dirty, plan.SelectedTests())
		}
		if plan.Selected[0].Witness.Kind != affected.WitnessPathLiteralReader {
			t.Fatalf("%s witness=%+v", dirty, plan.Selected[0].Witness)
		}
	}
}

func TestPathTokensAreBounded_NSAV0010(t *testing.T) {
	previous := maxPathTokens
	maxPathTokens = 2
	t.Cleanup(func() { maxPathTokens = previous })
	root := t.TempDir()
	write(t, root, "a_test.sh", "cat a/1.txt b/2.txt c/3.txt\n")
	write(t, root, "b_test.sh", "cat a/1.txt\n")
	result := units(t, root)
	if unit := unitOf(t, result, "a_test.sh"); !unit.PathTokensBounded || len(unit.PathTokens) != 0 {
		t.Fatalf("bounded unit=%+v", unit)
	}
	if unit := unitOf(t, result, "b_test.sh"); unit.PathTokensBounded || !slices.Equal(unit.PathTokens, []string{"a/1.txt"}) {
		t.Fatalf("unbounded unit=%+v", unit)
	}
}

func TestToolchainTestsReadUnboundedly_NSAV0011(t *testing.T) {
	root := t.TempDir()
	cases := map[string]bool{
		"t/go_test.sh":        true,
		"t/make_test.sh":      true,
		"t/gradlew_test.sh":   true,
		"t/env_test.sh":       true,
		"t/sub_test.sh":       true,
		"t/func_test.sh":      true,
		"t/bashc_test.sh":     true,
		"t/suite.bats":        true,
		"t/echo_test.sh":      false,
		"t/comment_test.sh":   false,
		"t/lookup_test.sh":    false,
		"t/heredoc_test.sh":   false,
		"script/builder.sh":   false,
		"t/plain_test.sh":     false,
		"t/pathpy_test.sh":    true,
		"t/condition_test.sh": true,
		"t/shadow_test.sh":    false,
		"t/kwshadow_test.sh":  false,
	}
	write(t, root, "t/go_test.sh", "go test ./cmd\n")
	write(t, root, "t/make_test.sh", "set -e; make -C x check\n")
	write(t, root, "t/gradlew_test.sh", "./gradlew test\n")
	write(t, root, "t/env_test.sh", "env -i FOO=1 cargo test\n")
	write(t, root, "t/sub_test.sh", "out=$(npm run x)\n")
	write(t, root, "t/func_test.sh", "build() {\n  xcodebuild test\n}\nbuild\n")
	write(t, root, "t/bashc_test.sh", "bash -c 'dotnet test'\n")
	write(t, root, "t/suite.bats", "@test \"x\" {\n  run python3 -c 'print(1)'\n}\n")
	write(t, root, "t/echo_test.sh", "echo \"run go test ./cmd\"\nprintf '%s\\n' make\n")
	write(t, root, "t/comment_test.sh", "# go test ./cmd\ntrue # make all\n")
	write(t, root, "t/lookup_test.sh", "command -v go >/dev/null || exit 0\n")
	write(t, root, "t/heredoc_test.sh", "cat <<'EOF'\ngo test ./cmd\nEOF\n")
	write(t, root, "script/builder.sh", "go build ./cmd\n")
	write(t, root, "t/plain_test.sh", "bash script/builder.sh\n")
	write(t, root, "t/pathpy_test.sh", "/usr/bin/python3 x.py\n")
	write(t, root, "t/condition_test.sh", "if ! bazel test //cmd:all; then exit 1; fi\n")
	write(t, root, "t/shadow_test.sh", "bundle() {\n  mktemp -d\n}\ndir=$(bundle match)\n")
	write(t, root, "t/kwshadow_test.sh", "function make {\n  printf x\n}\nmake all\n")
	result := units(t, root)
	for file, want := range cases {
		unit := unitOf(t, result, file)
		if got := unit.UnboundedReads == UnboundedRunsToolchain; got != want {
			t.Fatalf("%s unbounded=%v want %v", file, got, want)
		}
	}
	plan := affected.Select(build(t, root), []string{"README.md"})
	for _, selection := range plan.Selected {
		if !cases[strings.TrimPrefix(selection.UnitID, "shell:")] || selection.Witness.Kind != affected.WitnessUnboundedReader {
			t.Fatalf("unexpected selection %+v", selection)
		}
	}
	if !slices.Contains(plan.SelectedTests(), "t/go_test.sh") {
		t.Fatalf("toolchain test not selected on an unrelated path: %v", plan.SelectedTests())
	}
}

func TestSeamShapeSelectsTheClosure_NSAV0015(t *testing.T) {
	root := t.TempDir()
	writeSeam(t, root)
	graph := build(t, root)
	plan := affected.Select(graph, []string{"shell/lib/core.sh"})
	want := []string{"shell/tests/core_test.sh", "shell/tests/leaf_test.sh", "shell/tests/mid_test.sh"}
	if !slices.Equal(plan.SelectedTests(), want) || plan.Scope != affected.ScopeBounded {
		t.Fatalf("selected=%v scope=%s unknown=%+v", plan.SelectedTests(), plan.Scope, plan.Unknown)
	}
	excluded := make([]string, 0, len(plan.Excluded))
	for _, exclusion := range plan.Excluded {
		unit, _ := graph.Unit(exclusion.UnitID)
		excluded = append(excluded, unit.Tests...)
	}
	if !slices.Equal(excluded, []string{"shell/tests/solo_test.sh"}) {
		t.Fatalf("excluded=%v", excluded)
	}
}

func TestBuildsAreDeterministicAndSourceOnly_NSAV0013_NSAV0014(t *testing.T) {
	root := t.TempDir()
	writeSeam(t, root)
	write(t, root, "lib.go", "package lib\n")
	first, second := build(t, root), build(t, root)
	if first.Digest() != second.Digest() {
		t.Fatalf("digest drift %s vs %s", first.Digest(), second.Digest())
	}
	fromFS, err := affected.BuildFS(os.DirFS(root), New())
	if err != nil {
		t.Fatal(err)
	}
	if fromFS.Digest() != first.Digest() {
		t.Fatalf("BuildFS digest %s vs %s", fromFS.Digest(), first.Digest())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := affected.Build(ctx, root, New()); err == nil {
		t.Fatal("cancelled build succeeded")
	}
}

func TestLexerKeepsCommentsQuotesAndHeredocsApart(t *testing.T) {
	parsed := parse("echo '#not' \"a # b\" x#y # gone\ncat <<-EOF > out\n\tsource hidden.sh\n\tEOF\nsource real.sh\nf() { inner; }\n( sub )\n")
	var words []string
	for _, cmd := range parsed.commands {
		words = append(words, strings.Join(plainWords(cmd.words), " "))
	}
	want := []string{"echo '#not' \"a # b\" x#y", "cat", "source real.sh", "inner", "sub"}
	if !slices.Equal(words, want) {
		t.Fatalf("commands=%q want %q", words, want)
	}
	if !slices.Equal(parsed.data, []string{"\tsource hidden.sh"}) {
		t.Fatalf("data=%q", parsed.data)
	}
	if !parsed.commands[3].inFunction || parsed.commands[2].inFunction {
		t.Fatalf("function body flags wrong: %+v", parsed.commands)
	}
}

// writeSeam writes the shared seam shape: core <- mid <- leaf, plus solo.
func writeSeam(t *testing.T, root string) {
	t.Helper()
	write(t, root, "shell/lib/core.sh", "core() { echo core; }\n")
	write(t, root, "shell/lib/mid.sh", "source \"$(dirname \"${BASH_SOURCE[0]}\")/core.sh\"\nmid() { core; }\n")
	write(t, root, "shell/lib/leaf.sh", "source \"$(dirname \"${BASH_SOURCE[0]}\")/mid.sh\"\nleaf() { mid; }\n")
	write(t, root, "shell/lib/solo.sh", "solo() { echo solo; }\n")
	for _, name := range []string{"core", "mid", "leaf", "solo"} {
		write(t, root, "shell/tests/"+name+"_test.sh", "#!/usr/bin/env bash\nset -euo pipefail\nsource \"$(dirname \"$0\")/../lib/"+name+".sh\"\n[ \"$("+name+")\" != \"\" ]\n")
	}
}

func units(t *testing.T, root string) affected.Result {
	t.Helper()
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func build(t *testing.T, root string) *affected.Graph {
	t.Helper()
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func unitOf(t *testing.T, result affected.Result, relative string) affected.Unit {
	t.Helper()
	for _, unit := range result.Units {
		if unit.ID == "shell:"+relative {
			return unit
		}
	}
	t.Fatalf("no unit for %s in %+v", relative, result.Units)
	return affected.Unit{}
}

func hasUnknown(plan affected.Plan, reason, detail string) bool {
	for _, unknown := range plan.Unknown {
		if unknown.Reason == reason && unknown.Detail == detail {
			return true
		}
	}
	return false
}

func write(t *testing.T, root, relative, content string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
