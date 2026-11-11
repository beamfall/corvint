// SPDX-License-Identifier: Apache-2.0
package shell

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// holdGraph writes each body as NAME_test.sh beside the shared files and
// builds the graph.
func holdGraph(t *testing.T, tests map[string]string, files map[string]string) *affected.Graph {
	t.Helper()
	root := t.TempDir()
	write(t, root, "script/golden.txt", "golden\n")
	write(t, root, "docs/x.md", "x\n")
	for relative, body := range files {
		write(t, root, relative, body)
	}
	for name, body := range tests {
		write(t, root, name+"_test.sh", body)
	}
	return build(t, root)
}

// wantSelected checks which named tests a dirty path selects.
func wantSelected(t *testing.T, graph *affected.Graph, dirty string, selected, unselected []string) {
	t.Helper()
	got := affected.Select(graph, []string{dirty}).SelectedTests()
	for _, name := range selected {
		if !slices.Contains(got, name+"_test.sh") {
			t.Errorf("%s: %s not selected: %v", dirty, name, got)
		}
	}
	for _, name := range unselected {
		if slices.Contains(got, name+"_test.sh") {
			t.Errorf("%s: %s selected: %v", dirty, name, got)
		}
	}
}

// wantUnbounded checks each named test's UnboundedReads reason.
func wantUnbounded(t *testing.T, graph *affected.Graph, reason string, names ...string) {
	t.Helper()
	for _, name := range names {
		unit, ok := graph.Unit("shell:" + name + "_test.sh")
		if !ok || unit.UnboundedReads != reason {
			t.Errorf("%s: unbounded %q; want %q", name, unit.UnboundedReads, reason)
		}
	}
}

// carriageReturnFixtures and emptyDelimiterFixtures read script/golden.txt
// under bash and dash, the heredoc ones through "$tmp" taken from the
// environment (AFP-V0-9970, AFP-V0-9971).
var carriageReturnFixtures = map[string]string{
	"cr_comment": "echo a\r#;cat script/golden.txt\n",
	"cr_heredoc": "cat <<EOF\nEOF\r\ntmp=$(mktemp -d)\nEOF\ncat \"$tmp/script/golden.txt\"\n",
}

var emptyDelimiterFixtures = map[string]string{
	"empty_single": "cat <<''\ntmp=$(mktemp -d)\n\ncat \"$tmp/script/golden.txt\"\n",
	"empty_double": "cat <<\"\"\ntmp=$(mktemp -d)\n\ncat \"$tmp/script/golden.txt\"\n",
	"empty_dash":   "cat <<-''\n\ttmp=$(mktemp -d)\n\t\ncat \"$tmp/script/golden.txt\"\n",
}

// AFP-V0-9970: a carriage return is word text, so it neither starts a
// comment nor ends a here-document delimiter, and the read after it keeps
// its token.
func TestCarriageReturnIsWordText_AFPV09970(t *testing.T) {
	tests := map[string]string{
		// A real comment and a real temp root keep their relation.
		"plain_comment": "echo a #;cat script/golden.txt\n",
		"temp_root":     "tmp=$(mktemp -d)\ncat \"$tmp/script/golden.txt\"\n",
	}
	for name, body := range carriageReturnFixtures {
		tests[name] = body
	}
	graph := holdGraph(t, tests, nil)
	wantSelected(t, graph, "script/golden.txt", []string{"cr_comment", "cr_heredoc"}, []string{"plain_comment", "temp_root"})
	wantSelected(t, graph, "docs/x.md", nil, []string{"cr_comment", "cr_heredoc"})

	// A trap handler `exit\r` runs no exit: it is not an INT exit handler.
	if trapAllowed([]string{"'exit\r'", "INT"}) {
		t.Error("trap 'exit\\r' INT admitted as an exit handler")
	}
	if !trapAllowed([]string{"'exit'", "INT"}) {
		t.Error("trap 'exit' INT refused")
	}
	// A discarded redirection is stripped around blanks only.
	if got := redirection.ReplaceAllString("cd \"$d\"\r>/dev/null && pwd", ""); !strings.Contains(got, "\r") {
		t.Errorf("redirection strip took the carriage return: %q", got)
	}
	if got := redirection.ReplaceAllString("cd \"$d\" >/dev/null && pwd", ""); got != "cd \"$d\" && pwd" {
		t.Errorf("redirection strip: %q", got)
	}
}

// AFP-V0-9971: a quoted empty delimiter opens a here-document ended by the
// first empty line; its body is data, never commands.
func TestEmptyHereDocumentDelimiter_AFPV09971(t *testing.T) {
	graph := holdGraph(t, emptyDelimiterFixtures, nil)
	names := []string{"empty_single", "empty_double", "empty_dash"}
	wantSelected(t, graph, "script/golden.txt", names, nil)

	parsed := parse("cat <<''\nx 'y\n\ncat z\n")
	if len(parsed.data) != 1 || parsed.data[0] != "x 'y" {
		t.Errorf("data %q; want the body before the empty line", parsed.data)
	}
	var commands []string
	for _, cmd := range parsed.commands {
		commands = append(commands, strings.Join(cmd.words, " "))
	}
	if !slices.Contains(commands, "cat z") || slices.ContainsFunc(commands, func(c string) bool { return strings.HasPrefix(c, "x") }) {
		t.Errorf("commands %q; want `cat z` and no body line", commands)
	}
	// An unquoted missing delimiter word still opens none.
	if parsed := parse("cat <<\ncat z\n"); len(parsed.data) != 0 {
		t.Errorf("missing delimiter opened a here-document: %q", parsed.data)
	}
}

// AFP-V0-9970, AFP-V0-9971: bash and dash read script/golden.txt in every
// fixture the plugin now selects for it.
func TestCarriageReturnAndEmptyDelimiterAgreeWithShells_AFPV09970(t *testing.T) {
	fixtures := map[string]string{}
	for name, body := range carriageReturnFixtures {
		fixtures[name] = body
	}
	for name, body := range emptyDelimiterFixtures {
		fixtures[name] = body
	}
	ran := false
	for _, shell := range []string{"bash", "dash"} {
		binary, err := exec.LookPath(shell)
		if err != nil {
			continue
		}
		ran = true
		for name, body := range fixtures {
			root := t.TempDir()
			write(t, root, "script/golden.txt", "GOLDEN-MARKER\n")
			write(t, root, "fixture.sh", body)
			command := exec.Command(binary, "fixture.sh")
			command.Dir = root
			command.Env = append(os.Environ(), "tmp=.")
			output, _ := command.CombinedOutput()
			if !strings.Contains(string(output), "GOLDEN-MARKER") {
				t.Errorf("%s %s: did not read script/golden.txt: %q", shell, name, output)
			}
		}
	}
	if !ran {
		t.Skip("neither bash nor dash is installed")
	}
}

// AFP-V0-9972: an option value is never read as the walker's path, so a
// working-directory walk with an option value stays a tree walk.
func TestWalkerOptionValues_AFPV09972(t *testing.T) {
	walkers := map[string]string{
		"grep_group_value": "! grep -rm 1 FIXME\n",
		"grep_value":       "! grep -r -m 1 FIXME\n",
		"grep_long_value":  "! grep -r --include '*.go' FIXME\n",
		"rg_glob":          "! rg -g '*.go' FIXME\n",
		"ag_value":         "! ag -G go FIXME\n",
		"du_depth":         "du -d 1\n",
		"tree_level":       "tree -L 2\n",
		"ls_ignore":        "ls -R -I x | wc -l\n",
	}
	bounded := map[string]string{
		"grep_flags_dir": "! grep -rn FIXME script\n",
		"grep_value_dir": "! grep -r -m 1 FIXME script\n",
		"rg_glob_dir":    "! rg -g '*.go' FIXME script\n",
		"du_depth_dir":   "du -d 1 script\n",
		"grep_attached":  "! grep -rm1 FIXME script\n",
	}
	tests := map[string]string{}
	for name, body := range walkers {
		tests[name] = body
	}
	for name, body := range bounded {
		tests[name] = body
	}
	graph := holdGraph(t, tests, nil)
	for name := range walkers {
		wantUnbounded(t, graph, UnboundedTreeWalk, name)
	}
	for name := range bounded {
		wantUnbounded(t, graph, "", name)
	}
}

// AFP-V0-9973: git path output over a repository not proven private walks
// the tree; a private temporary repository does not.
func TestGitPathOutputWalks_AFPV09973(t *testing.T) {
	walkers := map[string]string{
		"ls_files":       "git ls-files | wc -l\n",
		"status":         "test -z \"$(git status --porcelain)\"\n",
		"log_names":      "git log --name-only -1\n",
		"reflog":         "git reflog --name-only -1\n",
		"show":           "git show HEAD:README.md\n",
		"computed_dir":   "git -C \"$root\" diff --name-only\n",
		"climbed_temp":   "tmp=$(mktemp -d)\ngit -C \"$tmp/..\" status\n",
		"wrapped":        "sudo -u u git grep FIXME\n",
		"cd_elsewhere":   "tmp=$(mktemp -d)\ncd \"$tmp\"\ncd /\ngit status\n",
		"cd_in_function": "tmp=$(mktemp -d)\ncd \"$tmp\"\nf() { git status; }\nf\n",
		"cd_after":       "tmp=$(mktemp -d)\ngit status\ncd \"$tmp\"\n",
		"cd_guarded":     "tmp=$(mktemp -d)\ntrue && cd \"$tmp\"\ngit status\n",
		"git_dir_repo":   "GIT_DIR=.git git log\n",
	}
	bounded := map[string]string{
		"temp_c":        "tmp=$(mktemp -d)\ngit -C \"$tmp\" status\n",
		"guarded_temp":  "tmp=$(mktemp -d) || exit 1\ngit -C \"$tmp/repo\" log --name-only\n",
		"cd_temp":       "tmp=$(mktemp -d) || exit 1\ncd \"$tmp\" || exit 1\ngit status\n",
		"cd_temp_and":   "tmp=$(mktemp -d)\ncd \"$tmp\" && git status\n",
		"temp_git_dir":  "tmp=$(mktemp -d)\nGIT_DIR=\"$tmp/x.git\" git log\n",
		"temp_long_dir": "tmp=$(mktemp -d)\ngit --git-dir=\"$tmp/x.git\" log\n",
		"rev_parse":     "git rev-parse --show-toplevel\n",
		"init":          "git init -q x\n",
	}
	tests := map[string]string{}
	for name, body := range walkers {
		tests[name] = body
	}
	for name, body := range bounded {
		tests[name] = body
	}
	graph := holdGraph(t, tests, nil)
	for name := range walkers {
		wantUnbounded(t, graph, UnboundedTreeWalk, name)
	}
	for name := range bounded {
		wantUnbounded(t, graph, "", name)
	}
	var names []string
	for name := range walkers {
		names = append(names, name)
	}
	wantSelected(t, graph, "docs/x.md", names, []string{"temp_c", "cd_temp", "rev_parse"})
}

// AFP-V0-9974: an awk or sed program read from a file makes the test an
// unbounded reader; an inline program keeps its tokens.
func TestAwkProgramFiles_AFPV09974(t *testing.T) {
	unbounded := map[string]string{
		"awk_f":         "awk -f tools/x.awk script/golden.txt\n",
		"awk_after_val": "awk -F , -f tools/x.awk script/golden.txt\n",
		"gawk_long":     "gawk --file=tools/x.awk script/golden.txt\n",
		"gawk_include":  "gawk -i lib '{print}' script/golden.txt\n",
		"sed_group":     "sed -nf tools/x.sed script/golden.txt\n",
		"sed_long":      "sed --file tools/x.sed script/golden.txt\n",
	}
	bounded := map[string]string{
		"awk_inline": "awk '{print $1}' script/golden.txt\n",
		"awk_vars":   "awk -F , -v x=1 '{print}' script/golden.txt\n",
		"sed_inline": "sed -n 1p script/golden.txt\n",
		"sed_e":      "sed -E -e s/a/b/ script/golden.txt\n",
	}
	tests := map[string]string{}
	for name, body := range unbounded {
		tests[name] = body
	}
	for name, body := range bounded {
		tests[name] = body
	}
	graph := holdGraph(t, tests, map[string]string{"tools/x.awk": "{print}\n", "tools/x.sed": "p\n"})
	for name := range unbounded {
		wantUnbounded(t, graph, UnboundedRunsToolchain, name)
	}
	for name := range bounded {
		wantUnbounded(t, graph, "", name)
	}
	wantSelected(t, graph, "docs/x.md", nil, []string{"awk_inline", "sed_inline"})
}

// AFP-V0-9975: a wrapper's option values or an unlisted wrapper no longer
// hide the shell, toolchain, walker or script it runs.
func TestWrappedCommands_AFPV09975(t *testing.T) {
	tests := map[string]string{
		"sudo_user":      "sudo -u u bash script/helper.sh\n",
		"nice_script":    "nice script/helper.sh\n",
		"nice_go":        "nice -n 5 go test ./...\n",
		"timeout_signal": "timeout -s KILL 5 python3 x.py\n",
		"env_unset_find": "env -u X find . -name '*.md'\n",
		"stdbuf_rg":      "stdbuf -oL rg FIXME\n",
		// Controls: a lookup runs nothing; a wrapped reader keeps its token.
		"command_v": "command -v go >/dev/null\n",
		"nice_cat":  "nice cat script/a.txt\n",
	}
	graph := holdGraph(t, tests, map[string]string{"script/helper.sh": "cat script/golden.txt\n", "script/a.txt": "a\n"})
	wantSelected(t, graph, "script/golden.txt", []string{"sudo_user", "nice_script"}, []string{"command_v", "nice_cat"})
	wantUnbounded(t, graph, UnboundedRunsToolchain, "nice_go", "timeout_signal")
	wantUnbounded(t, graph, UnboundedTreeWalk, "env_unset_find", "stdbuf_rg")
	wantUnbounded(t, graph, "", "command_v", "nice_cat", "sudo_user", "nice_script")
	wantSelected(t, graph, "script/a.txt", []string{"nice_cat"}, nil)
}

// AFP-V0-9976: assigning or exporting an interpreter startup name makes the
// file a dynamic source, also for a test that sources it.
func TestInterpreterStartupNames_AFPV09976(t *testing.T) {
	dynamic := map[string]string{
		"prefix":       "BASH_ENV=script/hook.sh bash script/run.sh\n",
		"env_wrapper":  "env BASH_ENV=\"$hook\" bash script/run.sh\n",
		"export_perl":  "export PERL5LIB=lib\nperl x.pl\n",
		"export_bare":  "export NODE_OPTIONS\nperl x.pl\n",
		"sudo_preload": "sudo LD_PRELOAD=./x.so true\n",
		"default":      ": \"${BASH_ENV:=script/hook.sh}\"\nbash script/run.sh\n",
		"declare":      "declare -x RUBYOPT=-rx\n",
		"standalone":   "ENV=script/hook.sh\nsh -i script/run.sh\n",
		"mixed_list":   "PERL5LIB=/opt/lib:lib perl x.pl\n",
		"climbed_abs":  "BASH_ENV=\"$HOME/hook.sh\" bash script/run.sh\n",
		"sources":      ". script/env.sh\n",
	}
	bounded := map[string]string{
		"unset":   "env -u BASH_ENV bash script/run.sh\n",
		"expand":  "echo \"$PERL5LIB\"\n",
		"other":   "FOO=1 bash script/run.sh\n",
		"perl_rd": "perl -ne 'print $ENV{HOME}' script/golden.txt\n",
		"cleared": "BASH_ENV= ENV='' bash script/run.sh\n",
		"outside": "export PERL5LIB=/usr/share/perl5:/opt/lib\nperl x.pl\n",
	}
	tests := map[string]string{}
	for name, body := range dynamic {
		tests[name] = body
	}
	for name, body := range bounded {
		tests[name] = body
	}
	graph := holdGraph(t, tests, map[string]string{
		"script/run.sh":  "true\n",
		"script/hook.sh": "true\n",
		"script/env.sh":  "export BASH_ENV=script/hook.sh\n",
	})
	var names []string
	for name := range dynamic {
		names = append(names, name)
		if name == "sources" {
			continue
		}
		wantUnbounded(t, graph, UnboundedDynamicSource, name)
	}
	for name := range bounded {
		wantUnbounded(t, graph, "", name)
	}
	if unit, ok := graph.Unit("shell:script/env.sh"); !ok || unit.UnboundedReads != UnboundedDynamicSource {
		t.Errorf("the sourced env helper: unbounded %q; want %q", unit.UnboundedReads, UnboundedDynamicSource)
	}
	var controls []string
	for name := range bounded {
		controls = append(controls, name)
	}
	wantSelected(t, graph, "docs/x.md", names, controls)
}

// AFP-V0-9977: where cp, mv or mkdir may not be the standard utility, none
// of its operands is dropped as written.
func TestRedefinedUtilitiesKeepOperands_AFPV09977(t *testing.T) {
	held := map[string]string{
		"function_mv": "mv() { cat \"$2\"; }\nmv x script/golden.txt\n",
		"alias_cp":    "alias cp=cat\ncp x script/golden.txt\n",
		"path_mkdir":  "PATH=\"$PWD/bin:$PATH\"\nmkdir script/golden.txt\n",
		"hash_cp":     "hash -p ./bin/reader cp\ncp x script/golden.txt\n",
		"command_fn":  "command() { cat \"$3\"; }\ncommand mv x script/golden.txt\n",
	}
	dropped := map[string]string{
		"plain_mv":       "mv x script/golden.txt\n",
		"plain_mkdir":    "mkdir script/golden.txt\n",
		"function_write": "mv() { :; }\necho hi > script/golden.txt\n",
	}
	tests := map[string]string{}
	for name, body := range held {
		tests[name] = body
	}
	for name, body := range dropped {
		tests[name] = body
	}
	graph := holdGraph(t, tests, nil)
	var heldNames, droppedNames []string
	for name := range held {
		heldNames = append(heldNames, name)
	}
	for name := range dropped {
		droppedNames = append(droppedNames, name)
	}
	wantSelected(t, graph, "script/golden.txt", heldNames, droppedNames)
}
