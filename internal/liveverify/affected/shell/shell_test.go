package shell

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
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

// A line continuation inside a word joins it, as the shell does.
func TestContinuedInvocationKeepsItsEdge_NSAV0008(t *testing.T) {
	root := t.TempDir()
	write(t, root, "core.sh", "true\n")
	write(t, root, "continued_test.sh", "bash co\\\nre.sh\nsh \\\n  core.sh\n")
	unit := unitOf(t, units(t, root), "continued_test.sh")
	if !slices.Equal(unit.Imports, []string{"shell:core.sh"}) {
		t.Fatalf("imports=%v", unit.Imports)
	}
}

func TestCasePatternsAndConditionalsAreNotCommands_NSAV0008_NSAV0009(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.sh", "true\n")
	write(t, root, "b.sh", "true\n")
	write(t, root, "use.sh", "case \"$1\" in\n  ''|/*|.|-) source a.sh ;;\n  (x) . b.sh ;&\n  *) exit 1\nesac\n"+
		"[[ $1 =~ ^v\\.([0-9]+)|(x)$ ]] && true\n")
	unit := unitOf(t, units(t, root), "use.sh")
	if !slices.Equal(unit.Imports, []string{"shell:a.sh", "shell:b.sh"}) || unit.UnboundedReads != "" {
		t.Fatalf("case pattern or [[ ]] regex parsed as a command: %+v", unit)
	}
}

func TestReassignedVariableIsNotADirectoryForm_NSAV0008(t *testing.T) {
	root := t.TempDir()
	write(t, root, "lib/a.sh", "true\n")
	write(t, root, "use.sh", "DIR=$(dirname \"$0\")\nDIR=/tmp\nsource \"$DIR/lib/a.sh\"\n")
	unit := unitOf(t, units(t, root), "use.sh")
	if len(unit.Imports) != 0 || unit.UnboundedReads != UnboundedDynamicSource {
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

func TestNonLiteralSourceReadsUnboundedly_NSAV0009(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.sh", "for f in lib/*.sh; do source \"$f\"; done\n")
	write(t, root, "b.sh", ". \"$HOME/.profile\"\n")
	write(t, root, "c.bats", "load \"$HELPER\"\n@test x { true; }\n")
	write(t, root, "d.sh", "source\n")
	write(t, root, "e.sh", "# source \"$f\"\necho 'source $x'\ncat <<EOF\nsource $y\nEOF\n")
	result := units(t, root)
	for _, dynamicFile := range []string{"a.sh", "b.sh", "c.bats", "d.sh"} {
		unit := unitOf(t, result, dynamicFile)
		if unit.UnboundedReads != UnboundedDynamicSource || unit.LocatesRoot != (dynamicFile != "c.bats") || len(unit.Frontier) != 0 {
			t.Fatalf("%s=%+v", dynamicFile, unit)
		}
	}
	if unit := unitOf(t, result, "e.sh"); unit.UnboundedReads != "" {
		t.Fatalf("comment, string or heredoc read unboundedly: %+v", unit)
	}
	if len(result.Frontier) != 0 {
		t.Fatalf("dynamic source raised a global frontier: %v", result.Frontier)
	}
}

// A computed source target is not an edge, so an unreached frontier would let
// a plan stay bounded without the test that sources the dirty file.
func TestComputedSourceIsSelectedOnAnyDirtyPath_NSAV0009(t *testing.T) {
	root := t.TempDir()
	write(t, root, "lib/core.sh", "core() { :; }\n")
	write(t, root, "lib/other.sh", "other() { :; }\n")
	write(t, root, "direct_test.sh", ". lib/core.sh\ncore\n")
	write(t, root, "computed_test.sh", "name=core\n. \"lib/${name}.sh\"\ncore\n")
	write(t, root, "lib/loader.sh", "name=other\n. \"lib/${name}.sh\"\n")
	write(t, root, "loader_test.sh", ". lib/loader.sh\nother\n")
	graph := build(t, root)
	plan := affected.Select(graph, []string{"lib/core.sh"})
	want := []string{"computed_test.sh", "direct_test.sh", "loader_test.sh"}
	if got := plan.SelectedTests(); !slices.Equal(got, want) {
		t.Fatalf("selected %v, want %v", got, want)
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

// NSA-V0-016: a word a command only creates or truncates is no path token: a
// mkdir operand, the destination of cp or mv, and the target of a `>`
// redirection cannot carry a dirty path's bytes into the test, so a change to
// a file only such words name does not select it. The cp source, a read
// redirection, an appending redirection, a `-t` destination and a word also
// read elsewhere keep their tokens (fail closed).
func TestWriteOnlyWordsAreNoPathTokens_NSAV0016(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.sh", "b.sh", "c.sh", "d.sh", "e.sh", "f.sh", "g.sh"} {
		write(t, root, "script/"+name, "true\n")
	}
	write(t, root, "lib/narrow_test.sh", strings.Join([]string{
		"#!/usr/bin/env bash",
		"h=$(mktemp -d)",
		"mkdir -p \"$h/script\" \"$h/out\"",
		"cp \"$(git rev-parse --show-toplevel)/script/a.sh\" \"$h/script/\"",
		"mv \"$h/script/a.sh\" \"$h/renamed.sh\"",
		"echo ok > \"$h/script/out.txt\"",
		"printf x >| \"$h/truncated.log\"",
		"bash \"$h/renamed.sh\" 2>&1",
		"",
	}, "\n"))
	unit := unitOf(t, units(t, root), "lib/narrow_test.sh")
	for _, token := range []string{"/script", "/script/", "/out", "/script/out.txt", "/truncated.log"} {
		if slices.Contains(unit.PathTokens, token) {
			t.Fatalf("write-only token %s kept in %v", token, unit.PathTokens)
		}
	}
	for _, token := range []string{"/script/a.sh", "/renamed.sh"} {
		if !slices.Contains(unit.PathTokens, token) {
			t.Fatalf("read token %s missing from %v", token, unit.PathTokens)
		}
	}
	write(t, root, "lib/wide_test.sh", strings.Join([]string{
		"#!/usr/bin/env bash",
		"cp -t \"$h/out/\" script/b.sh",          // -t names the destination first: the last operand is read
		"cat < script/c.sh",                      // a read redirection
		"echo more >> script/d.sh",               // an appending redirection keeps prior bytes
		"mkdir -p script/e.sh && ls script/e.sh", // also read
		"cp script/f.sh",                         // one operand: no destination to drop
		"mv -T script/g.sh \"$h/g.sh\"",          // -T makes the last operand the destination; the source stays read
		"",
	}, "\n"))
	graph := build(t, root)
	for dirty, want := range map[string][]string{
		"script/a.sh": {"lib/narrow_test.sh"},
		"script/b.sh": {"lib/wide_test.sh"},
		"script/c.sh": {"lib/wide_test.sh"},
		"script/d.sh": {"lib/wide_test.sh"},
		"script/e.sh": {"lib/wide_test.sh"},
		"script/f.sh": {"lib/wide_test.sh"},
		"script/g.sh": {"lib/wide_test.sh"},
	} {
		if got := affected.Select(graph, []string{dirty}).SelectedTests(); !slices.Equal(got, want) {
			t.Fatalf("%s selected %v, want %v", dirty, got, want)
		}
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
		"t/late_test.sh":      true,
		"t/child_test.sh":     true,
		"t/command_test.sh":   true,
		"t/expanding_test.sh": true,
		"t/quoted_test.sh":    false,
		"t/joined_test.sh":    true,
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
	write(t, root, "t/late_test.sh", "make all\nmake() {\n  printf x\n}\n")
	write(t, root, "t/child_test.sh", "go() {\n  printf x\n}\nbash -c 'go build ./cmd'\n")
	write(t, root, "t/command_test.sh", "make() { :; }\ncommand make test\n")
	write(t, root, "t/expanding_test.sh", "cat <<EOF\n$(make test)\nEOF\n")
	write(t, root, "t/quoted_test.sh", "cat <<'EOF'\n$(make test)\nEOF\n")
	write(t, root, "t/joined_test.sh", "ma\\\nke test\n")
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
	fromFS, err := affected.BuildFS(boundedDirFS{os.DirFS(root)}, New())
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

// AFP-V0-400: a shell reader whose only naming token is a lone file name
// (README.md) is selected only where it can place the path: at the repository
// root, in its own subtree, or below directories its tokens name. A script
// that changes to any directory not proven to be the root or its own, reads
// recursively or runs git keeps the component-run relation; one that changes
// only to the root or its own directory through a form of the file itself is
// placed.
func TestLoneFileNameShellReaderMustPlaceThePath_AFPV0400(t *testing.T) {
	root := t.TempDir()
	write(t, root, "script/readme_test.sh", "#!/bin/sh\ngrep -q corvint README.md\n")
	write(t, root, "script/specs_test.sh", "#!/bin/sh\ncd \"$(dirname \"$0\")/../docs/specs\" && grep -q corvint README.md\n")
	write(t, root, "script/multi_test.sh", "#!/bin/sh\ngrep -q corvint specs/README.md\n")
	write(t, root, "script/computed_test.sh", "#!/bin/sh\ncd \"$FIXTURE\" && git -C \"$FIXTURE\" add -A && grep -q corvint README.md\n")
	write(t, root, "script/cd_test.sh", "#!/bin/sh\ncd docs; cd specs; grep -q corvint README.md\n")
	write(t, root, "script/find_test.sh", "#!/bin/sh\nfind docs/ -name README.md | xargs grep -q corvint\n")
	write(t, root, "script/recursive_test.sh", "#!/bin/sh\ngrep -rq corvint --include README.md docs\n")
	write(t, root, "script/git_test.sh", "#!/bin/sh\ngit grep -q corvint -- README.md\n")
	write(t, root, "script/diff_test.sh", "#!/bin/sh\ndiff -r docs \"$TMP\" || grep -q corvint README.md\n")
	write(t, root, "script/lsfiles_test.sh", "#!/bin/sh\ngit -C \"$ROOT\" ls-files -- '*README.md' | while read -r f; do grep -q corvint \"$ROOT/$f\"; done\n")
	write(t, root, "script/string_test.sh", "#!/bin/sh\nsh -c 'cd docs && grep -q corvint README.md'\n")
	// A cd whose target is not proven to be the root or the file's own
	// directory keeps the wider relation: a variable, a computed or climbing
	// directory, `cd -`, a bare cd, a `$0` form where `$0` is not the file,
	// and env's directory and command-string options.
	write(t, root, "script/variable_test.sh", "#!/bin/sh\nd=docs; cd \"$d\"; grep -q needle README.md\n")
	write(t, root, "script/back_test.sh", "#!/bin/sh\ncd -; grep -q needle README.md\n")
	write(t, root, "script/home_test.sh", "#!/bin/sh\ncd; grep -q needle README.md\n")
	write(t, root, "script/zero.bats", "@test x {\n  cd \"$(dirname \"$0\")\" && grep -q needle README.md\n}\n")
	write(t, root, "script/child_test.sh", "#!/bin/sh\nsh -c 'cd \"$(dirname \"$0\")\" && grep -q needle README.md' docs/x\n")
	write(t, root, "script/env_test.sh", "#!/bin/sh\nenv -C docs grep -q needle README.md\n")
	write(t, root, "script/split_test.sh", "#!/bin/sh\nenv --split-string='grep -rq needle --include README.md docs'\n")
	// grep reads recursively through any spelling of -r, -R or -d recurse.
	var kept []string
	for index, options := range []string{"-d recurse", "-drecurse", "-d rec", "--directories recurse", "--directories=recurse", "--recursive", "--recur", "-R", "--dereference-recursive", "-qr", "-rq"} {
		write(t, root, fmt.Sprintf("script/grep%d_test.sh", index), "#!/bin/sh\ngrep "+options+" needle --include README.md docs\n")
		kept = append(kept, fmt.Sprintf("script/grep%d_test.sh", index))
	}
	// A cd to the repository root or into the file's own directory, through a
	// form of the file itself, still places.
	write(t, root, "script/up_test.sh", "#!/bin/sh\ncd -P -- \"$(dirname \"$0\")/..\" && grep -q needle README.md\n")
	write(t, root, "script/source_test.sh", "#!/bin/bash\ncd \"${BASH_SOURCE[0]%/*}\" && grep -q needle README.md\n")
	graph := build(t, root)
	kept = append(kept, "script/back_test.sh", "script/cd_test.sh", "script/child_test.sh", "script/computed_test.sh", "script/diff_test.sh", "script/env_test.sh", "script/find_test.sh",
		"script/git_test.sh", "script/home_test.sh", "script/lsfiles_test.sh", "script/recursive_test.sh", "script/split_test.sh", "script/string_test.sh", "script/variable_test.sh", "script/zero.bats")
	placed := []string{"script/readme_test.sh", "script/source_test.sh", "script/up_test.sh"}
	cases := []struct {
		dirty string
		want  []string
	}{
		// The V1-1817 shape: a spec index edit no longer selects the lone readers.
		{"docs/specs/README.md", append([]string{"script/multi_test.sh", "script/specs_test.sh"}, kept...)},
		{"README.md", append(append([]string{"script/specs_test.sh"}, placed...), kept...)},
		{"script/README.md", append(append([]string{"script/specs_test.sh"}, placed...), kept...)},
		{"docs/README.md", append([]string{"script/specs_test.sh"}, kept...)},
	}
	for _, c := range cases {
		slices.Sort(c.want)
		plan := affected.Select(graph, []string{c.dirty})
		if got := plan.SelectedTests(); !slices.Equal(got, c.want) {
			t.Errorf("%s selected %v, want %v", c.dirty, got, c.want)
		}
	}
}

// NSA-V0-012, AFP-V0-581: the foreign-invocation frontier stands for unseen
// invokers, so it bears on a plan that changes the script or a script it
// sources, that follows a test-less reader of an unowned path, or that selects
// the script as a reader of a path another plugin owns (what the script does
// changes with the path, and so may what its invokers do); it does not bear
// on a change that reaches no script.
func TestForeignFrontierBearsOnlyWhenDependentsAreFollowed_NSAV0012(t *testing.T) {
	root := t.TempDir()
	write(t, root, "script/lib.sh", "true\n")
	write(t, root, "script/tool.sh", ". script/lib.sh\n")
	write(t, root, "script/tool_test.sh", "bash script/tool.sh\n")
	write(t, root, "check.sh", "grep -q main cmd/main.go\n")
	write(t, root, "check_test.sh", "bash check.sh\n")
	write(t, root, "script/notes.sh", "cat docs/notes.txt\n")
	write(t, root, "cmd/main.go", "package main\n")
	write(t, root, "cmd/other.go", "package main\n")
	write(t, root, "go.mod", "module example.test/m\n")
	graph, err := affected.Build(context.Background(), root, New(), golang.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		dirty string
		want  bool
	}{
		{"script/tool.sh", true},
		{"script/lib.sh", true},
		{"docs/notes.txt", true},
		{"cmd/main.go", true},
		{"cmd/other.go", false},
	} {
		plan := affected.Select(graph, []string{tc.dirty})
		if got := hasUnknown(plan, affected.UnknownLanguageFrontier, FrontierForeignInvocation); got != tc.want {
			t.Errorf("%s: foreign frontier %v, want %v (selected %v)", tc.dirty, got, tc.want, plan.SelectedTests())
		}
	}
	if reached := graph.ReachedUnitIDs([]string{"cmd/main.go"}); !slices.Contains(reached, "shell:check.sh") {
		t.Errorf("cmd/main.go: the reader is not reached: %v", reached)
	}
}

// TSP-V0-004: a source of a file inside a directory `mktemp -d` just created
// reads a runtime file outside the repository; every near miss stays a
// dynamic, root-locating source.
func TestFreshTemporaryDirectorySourceIsBounded_TSP_V0_004(t *testing.T) {
	const serve = "serve() {\n  local a b\n  infra() { exit 97; }\n  w=$(mktemp -d \"${TMPDIR:-/tmp}/x.XXXXXX\") || infra \"mktemp failed\"\n  trap '/bin/rm -rf \"$w\"' EXIT\n  base64 -d | tar -x -C \"$w\" || infra decode\n  source \"${w}/sub/meta.env\" || infra meta\n}\n"
	root := t.TempDir()
	write(t, root, "fresh.sh", serve)
	negatives := map[string]string{
		"other-value.sh":    "w=$(pwd)\n. \"$w/meta.env\"\n",
		"twice.sh":          "w=$(mktemp -d)\nw=$(mktemp -d)\n. \"$w/meta.env\"\n",
		"for-rebind.sh":     "w=$(mktemp -d)\nfor w in lib; do :; done\n. \"$w/meta.env\"\n",
		"read-rebind.sh":    "w=$(mktemp -d)\nread -r w\n. \"$w/meta.env\"\n",
		"local-rebind.sh":   "f() {\n  w=$(mktemp -d)\n  local w\n  . \"$w/meta.env\"\n}\n",
		"unset.sh":          "w=$(mktemp -d)\nunset w\n. \"$w/meta.env\"\n",
		"default-bind.sh":   "w=$(mktemp -d)\n: \"${w:=lib}\"\n. \"$w/meta.env\"\n",
		"local-decl.sh":     "f() {\n  local w=$(mktemp -d)\n  . \"$w/meta.env\"\n}\n",
		"other-func.sh":     "f() { w=$(mktemp -d); }\ng() { . \"$w/meta.env\"; }\n",
		"subshell.sh":       "( w=$(mktemp -d) )\n. \"$w/meta.env\"\n",
		"branch.sh":         "if true; then\n  w=$(mktemp -d)\nfi\n. \"$w/meta.env\"\n",
		"conditional.sh":    "true && w=$(mktemp -d)\n. \"$w/meta.env\"\n",
		"pipeline.sh":       "w=$(mktemp -d) | cat\n. \"$w/meta.env\"\n",
		"inner-pipe.sh":     "w=$(mktemp -d | head -1)\n. \"$w/meta.env\"\n",
		"dynamic-rest.sh":   "w=$(mktemp -d)\n. \"$w/$name.env\"\n",
		"dotdot-rest.sh":    "w=$(mktemp -d)\n. \"$w/../lib.sh\"\n",
		"bare-dir.sh":       "w=$(mktemp -d)\n. \"$w\"\n",
		"file-temp.sh":      "w=$(mktemp)\n. \"$w/meta.env\"\n",
		"dry-run.sh":        "w=$(mktemp -du)\n. \"$w/meta.env\"\n",
		"prefix.sh":         "w=$(mktemp -d) true\n. \"$w/meta.env\"\n",
		"envfile.sh":        "w=$(mktemp -d)\n. \"$w/meta.env\"\n. \"${ENVFILE/#\\~/$HOME}\"\n",
		"source-between.sh": "w=$(mktemp -d)\n. ./lib.sh\n. \"$w/meta.env\"\n",
		"after-source.sh":   "w=$(mktemp -d)\n. \"$w/meta.env\"\n. \"$w/more.env\"\n",
		"later-source.sh":   ". \"$w/meta.env\"\nw=$(mktemp -d)\n",
		"backticks.sh":      "w=`mktemp -d`\n. \"$w/meta.env\"\n",
	}
	for name, text := range negatives {
		write(t, root, name, text)
	}
	write(t, root, "lib.sh", "true\n")
	result := units(t, root)
	if unit := unitOf(t, result, "fresh.sh"); unit.UnboundedReads != "" || unit.LocatesRoot {
		t.Fatalf("fresh temporary directory source stayed dynamic: %+v", unit)
	}
	for name := range negatives {
		if unit := unitOf(t, result, name); unit.UnboundedReads != UnboundedDynamicSource || !unit.LocatesRoot {
			t.Errorf("%s is not a dynamic source: %+v", name, unit)
		}
	}
}
