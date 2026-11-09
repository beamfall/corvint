package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

type forbiddenHelpReader struct{ reads int }

func TestLanePlanRetired(t *testing.T) {
	t.Run("WQO-V0-045 retired command is not a WQO alias", func(t *testing.T) {
		if _, known := publicHelpTopic("lane-plan"); known || strings.Contains(rootHelp, "lane-plan") {
			t.Fatal("retired lane-plan is still advertised")
		}
		for _, args := range [][]string{
			{"lane-plan", "--repo", "project", "--lane", "A=a/a.go"},
			{"lane-plan", "--help"},
			{"help", "lane-plan"},
			{"--root", "/definitely/not/a/repository", "lane-plan", "--repo", "project"},
			{"--root=/definitely/not/a/repository", "lane-plan", "--help"},
		} {
			reader := &forbiddenHelpReader{}
			var stdout, stderr bytes.Buffer
			code := run(args, reader, &stdout, &stderr)
			if code != 2 || stdout.Len() != 0 || reader.reads != 0 || !strings.Contains(stderr.String(), `"code": "invalid-arguments"`) {
				t.Fatalf("args=%q exit=%d stdout=%q stderr=%q reads=%d", args, code, &stdout, &stderr, reader.reads)
			}
		}
	})
}

func (reader *forbiddenHelpReader) Read(_ []byte) (int, error) {
	reader.reads++
	return 0, nil
}

func TestHelpInvocationsAreDeterministicAndDoNotInspectRootOrStdin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"flag", []string{"--help"}, rootHelp},
		{"command", []string{"help"}, rootHelp},
		{"query flag", []string{"query", "--help"}, helpText("query")},
		{"query command", []string{"help", "query"}, helpText("query")},
		{"impact flag", []string{"impact", "--help"}, helpText("impact")},
		{"impact command", []string{"help", "impact"}, helpText("impact")},
		{name: "ASD-V0-009 feature flag without repository access", args: []string{"--root", "/definitely/not/a/repository", "feature", "--help"}, want: helpText("feature")},
		{name: "ASD-V0-009 feature command without repository access", args: []string{"--root", "/definitely/not/a/repository", "help", "feature"}, want: helpText("feature")},
		{name: "ASD-V0-009 eval flag without repository access", args: []string{"--root", "/definitely/not/a/repository", "eval", "--help"}, want: helpText("eval")},
		{name: "ASD-V0-009 eval command without repository access", args: []string{"--root", "/definitely/not/a/repository", "help", "eval"}, want: helpText("eval")},
		{"event flag", []string{"harness", "event", "--help"}, helpText("harness-event")},
		{"event command", []string{"help", "harness", "event"}, helpText("harness-event")},
		{"harness flag", []string{"harness", "--help"}, helpText("harness-event")},
		{"harness command", []string{"help", "harness"}, helpText("harness-event")},
		{"adapter flag", []string{"adapter", "--help"}, helpText("adapter")},
		{"adapter command", []string{"help", "adapter"}, helpText("adapter")},
		{"LPCV-V0-051 test-validity flag", []string{"test-validity", "--help"}, helpText("test-validity")},
		{"test-validity command", []string{"help", "test-validity"}, helpText("test-validity")},
		{"invalid root is not opened", []string{"--root", "/definitely/not/a/repository", "query", "--help"}, helpText("query")},
		{"inline root is not opened", []string{"--root=/definitely/not/a/repository", "impact", "--help"}, helpText("impact")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := &forbiddenHelpReader{}
			var stdout, stderr bytes.Buffer
			if exit := run(test.args, reader, &stdout, &stderr); exit != 0 {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, &stdout, &stderr)
			}
			if stdout.String() != test.want || stderr.Len() != 0 || reader.reads != 0 {
				t.Fatalf("stdout=%q\nwant=%q\nstderr=%q stdin reads=%d", &stdout, test.want, &stderr, reader.reads)
			}
			if !strings.HasSuffix(stdout.String(), "\n") {
				t.Fatalf("help lacks terminal LF: %q", &stdout)
			}
		})
	}
}

func TestHelpFlagAnywhereFollowsArgparse(t *testing.T) {
	t.Parallel()
	invalidRoot := []string{"--root", "/definitely/not/a/repository"}
	t.Run("GPK-V0-062 help anywhere before -- prints the COMMAND --help bytes", func(t *testing.T) {
		for _, test := range []struct {
			args []string
			want string
		}{
			{[]string{"query", "--task", "x", "--help"}, helpText("query")},
			{[]string{"query", "--help", "--task", "x"}, helpText("query")},
			{[]string{"query", "--limit", "-1", "--help"}, helpText("query")},
			{[]string{"context", "--task", "x", "--subject", "a.go", "--help"}, helpText("context")},
			{[]string{"impact", "path/a.go", "--help"}, helpText("impact")},
			{[]string{"index", "--if-stale", "--help"}, helpText("index")},
			{[]string{"harness", "event", "--host", "codex", "--help"}, helpText("harness-event")},
			{[]string{"--help", "query"}, rootHelp},
		} {
			args := append(append([]string{}, invalidRoot...), test.args...)
			reader := &forbiddenHelpReader{}
			var stdout, stderr bytes.Buffer
			if exit := run(args, reader, &stdout, &stderr); exit != 0 || stdout.String() != test.want || stderr.Len() != 0 || reader.reads != 0 {
				t.Fatalf("args=%q exit=%d stderr=%q reads=%d stdout matches=%v", args, exit, &stderr, reader.reads, stdout.String() == test.want)
			}
		}
	})
	t.Run("GPK-V0-062 --help as an option value, after --, or after an invalid choice is not help", func(t *testing.T) {
		for _, args := range [][]string{
			{"query", "--task", "--help"},
			{"query", "--task=--help"},
			{"query", "--task", "x", "--", "--help"},
			{"impact", "--", "--help"},
			{"cem", "bogus", "--help"},
		} {
			if topic, requested, err := parseHelpInvocation(args); requested || err != nil {
				t.Fatalf("args=%q topic=%q requested=%v err=%v", args, topic, requested, err)
			}
		}
	})
}

// TestArgparseOptionLikeClassification pins the shared GPK-V0-064 helper: a
// lone dash, a negative number, and a token containing a space are values, not
// options, matching argparse's _parse_optional.
func TestArgparseOptionLikeClassification(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		token string
		want  bool
	}{
		{"-x", true},
		{"--help", true},
		{"-h", true},
		{"-", false},
		{"-5", false},
		{"-5.5", false},
		{"-.5", false},
		{"-5x", true},
		{"-not a number", false},
		{"value", false},
		{"", false},
	} {
		if got := argparseOptionLike(test.token); got != test.want {
			t.Errorf("argparseOptionLike(%q) = %v, want %v", test.token, got, test.want)
		}
	}
}

// TestRootPreambleFollowsArgparseOptionLikeClassification pins GPK-V0-064 as
// amended by decision 0181: before every top-level command, a bare `--root`
// followed by an option-like token is refused as a missing value, while the
// inline form, a lone dash, a negative number, and `--` stay root values.
func TestRootPreambleFollowsArgparseOptionLikeClassification(t *testing.T) {
	t.Parallel()
	for _, command := range topLevelCommands {
		for _, args := range [][]string{{"--root", "--task", command}, {"--root", "-h", command}} {
			var stdout, stderr bytes.Buffer
			if exit := run(args, strings.NewReader(""), &stdout, &stderr); exit != 2 || !strings.Contains(stderr.String(), "missing value for --root") || stdout.Len() != 0 {
				t.Errorf("args=%q exit=%d stdout=%q stderr=%q", args, exit, &stdout, &stderr)
			}
		}
	}
}

// TestNativeVerbOptionValuesFollowArgparseOptionLikeClassification pins
// GPK-V0-064 as amended by decision 0196 for the parsers decision 0173 did not
// list: an option-like next token is refused through that parser's own
// missing-value path, while the inline `=` form and a negative number are
// values.
func TestNativeVerbOptionValuesFollowArgparseOptionLikeClassification(t *testing.T) {
	t.Parallel()
	errorOf := func(_ any, err error) error { return err }
	for _, test := range []struct {
		name, missing string
		parse         func(value string, inline bool) error
	}{
		{"test-validity", "missing value for --receipt", func(v string, inline bool) error {
			if inline {
				return errorOf(parseTestValidityOptions([]string{"--receipt=" + v}))
			}
			return errorOf(parseTestValidityOptions([]string{"--receipt", v}))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, value := range []string{"-x", "--help", "-h"} {
				if err := test.parse(value, false); err == nil || !strings.Contains(err.Error(), test.missing) {
					t.Errorf("bare value %q: err=%v, want %q", value, err, test.missing)
				}
				if err := test.parse(value, true); err != nil && strings.Contains(err.Error(), test.missing) {
					t.Errorf("inline value %q refused as missing: %v", value, err)
				}
			}
			if err := test.parse("-5", false); err != nil && strings.Contains(err.Error(), test.missing) {
				t.Errorf("negative number refused as missing: %v", err)
			}
		})
	}
}

// TestHFlagAliasesHelpFlag pins GPK-V0-064: `-h` prints the same output as
// `--help` at root, top-level, and nested-choice scope (argparse's add_help).
func TestHFlagAliasesHelpFlag(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"root", []string{"-h"}, rootHelp},
		{"command", []string{"query", "-h"}, helpText("query")},
		{"anywhere before --", []string{"query", "--task", "x", "-h"}, helpText("query")},
		{"harness event", []string{"harness", "event", "--host", "codex", "-h"}, helpText("harness-event")},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := &forbiddenHelpReader{}
			var stdout, stderr bytes.Buffer
			if exit := run(test.args, reader, &stdout, &stderr); exit != 0 || stdout.String() != test.want || stderr.Len() != 0 || reader.reads != 0 {
				t.Fatalf("args=%q exit=%d stderr=%q reads=%d stdout matches=%v", test.args, exit, &stderr, reader.reads, stdout.String() == test.want)
			}
		})
	}
}

func TestRootHelpListsReleaseAuditCommands(t *testing.T) {
	t.Parallel()
	t.Run("CRB-V0-001 current public help uses the Corvint brand", func(t *testing.T) {
		if !strings.HasPrefix(rootHelp, "Corvint: proof-carrying context for coding agents\n") {
			t.Fatalf("root help lacks the Corvint product name: %q", rootHelp)
		}
		for _, command := range topLevelCommands {
			topic, known := publicHelpTopic(command)
			if !known {
				t.Fatalf("public command %q has no help topic", command)
			}
			text := helpText(topic)
			if !strings.Contains(text, "corvint") {
				t.Fatalf("%s help lacks a Corvint executable example: %q", command, text)
			}
		}
	})
	commands := strings.SplitN(rootHelp, "\nCommands:\n", 2)[1]
	commands, _, _ = strings.Cut(commands, "\nGlobal options:")
	for _, command := range []string{"adapter"} {
		if !strings.Contains("\n"+commands, "\n    "+command+" ") {
			t.Errorf("root help command list omits %q", command)
		}
	}
}

func TestRootHelpListsContextAndWorkCommands(t *testing.T) {
	t.Parallel()
	for _, required := range []string{
		"\n    context  task-context packet",
		"corvint COMMAND --help | corvint help COMMAND",
	} {
		if !strings.Contains(rootHelp, required) {
			t.Fatalf("root help does not contain %q", required)
		}
	}
	for _, required := range []string{"context --task TEXT [--subject PATH] [--limit N]"} {
		if !strings.Contains(helpText(strings.Fields(required)[0]), required) {
			t.Fatalf("%s help does not contain %q", strings.Fields(required)[0], required)
		}
	}
}

// TestRootHelpIsACompactVerbIndex pins CCF-V1-009: root help is a verb index with one line per
// verb under its Core or Experimental group header (CCF-V1-008), names each verb once, carries no
// per-verb usage line, and stays within its byte bound; the detail is per verb.
func TestRootHelpIsACompactVerbIndex(t *testing.T) {
	t.Parallel()
	if len(rootHelp) > rootHelpMaxBytes {
		t.Fatalf("root help is %d bytes, want at most %d", len(rootHelp), rootHelpMaxBytes)
	}
	usage, _, _ := strings.Cut(strings.SplitN(rootHelp, "\nUsage:\n", 2)[1], "\n\n")
	if lines := strings.Count(usage, "\n") + 1; lines != 3 {
		t.Fatalf("root usage has %d lines, want 3:\n%s", lines, usage)
	}
	headers, lines := rootHelpGroups(t)
	listed := map[string]int{}
	for _, header := range headers {
		for _, line := range lines[header] {
			names, _, _ := strings.Cut(strings.TrimPrefix(line, "    "), "  ")
			for _, family := range strings.Split(names, " | ") {
				listed[strings.Fields(family)[0]]++
			}
		}
	}
	if strings.Contains(rootHelp, "\nCommand maturity:\n") {
		t.Error("root help repeats every verb in a separate Command maturity section")
	}
	for _, command := range topLevelCommands {
		_, hidden := hiddenCommands[command]
		if want := map[bool]int{true: 0, false: 1}[hidden]; listed[command] != want {
			t.Errorf("root help lists verb %q %d times, want %d", command, listed[command], want)
		}
	}
	if len(listed)+len(hiddenCommands) != len(topLevelCommands) {
		t.Errorf("root help lists %d verbs and hides %d, want the %d dispatched verbs", len(listed), len(hiddenCommands), len(topLevelCommands))
	}
	for _, flag := range []string{"--task", "--base", "--cem", "--limit"} {
		if strings.Contains(rootHelp, flag) {
			t.Errorf("root help carries per-verb option %q", flag)
		}
	}
}

func TestQueryHelpExposesRepositoryAndFrozenAuthorityProfiles(t *testing.T) {
	t.Parallel()
	t.Run("EAF-V0-005", func(t *testing.T) {
		text := helpText("query")
		for _, required := range []string{
			authorityStartPrompt,
			"Required UTF-8 task",
			"up\n               to three advisory learned paths",
			"default: 10; range: 1-50",
			"BuildEval -> EvalQuery",
			"Repository and agent-tooling tasks",
			"Darwin or Linux",
			"root AGENTS.md",
			"--budget-bytes",
			"read-only and local-only",
			"Recovery for unsupported-query-trace-state",
			"Keep the trace store and original task unchanged",
			"context --task TASK --limit 5",
			"Never delete traces, dirty the worktree, or rewrite the task",
			// GPK-V0-960 (V1-3663): the packet revision is labelled a tree id.
			"revision and freshness.revision are the Git tree id",
			"not a commit id, so they never\n  equal learning.history_tip",
			"the two\n  differing is not staleness (GPK-V0-960)",
		} {
			if !strings.Contains(text, required) {
				t.Fatalf("query help does not contain %q:\n%s", required, text)
			}
		}
		if strings.Count(text, authorityStartPrompt) != 1 {
			t.Fatalf("frozen prompt count=%d", strings.Count(text, authorityStartPrompt))
		}
	})
}

func TestImpactAndHarnessHelpExposeActualLimitsAndUnsupportedProfiles(t *testing.T) {
	t.Parallel()
	for _, required := range []string{
		"default: 10; range: 1-50",
		"the root package included",
		"at most 100 paths by default (256 only for the\n  explicit expanded range profile) and 1024 characters per path.",
		"1,000,000 bytes per source and 128 MiB in total",
		"Mixed-worktree freshness is disclosed",
		"--working-tree-untracked",
		"default profile is unchanged",
		"stable twice-read bytes",
		"observed, not immutable Git",
		"64,000,000 bytes",
		"admitted but lacking a reverse-import rule",
		"unsupported-impact-path-suffix",
		"untracked-path impact is implemented for .go files only",
		"before repository eligibility",
	} {
		if !strings.Contains(impactHelp, required) {
			t.Fatalf("impact help does not contain %q", required)
		}
	}
	for _, required := range []string{
		"claude-code, codex, gemini-cli, or opencode",
		"file-change, post-tool, session-end, session-start,",
		"default: 8000; range: 4096-1000000 bytes",
		"Input is limited to 131072 bytes",
		"user-prompt, file-change, and compact\nsession-start additionally build a repository index",
		"user-prompt shares query's BuildEval -> EvalQuery repository path",
		"corvint-harness-event/0 FALLBACK",
	} {
		if !strings.Contains(harnessEventHelp, required) {
			t.Fatalf("harness help does not contain %q", required)
		}
	}
}

func TestSupportBoundaryDisclosesTheSelfObservationLedgerWrite(t *testing.T) {
	t.Parallel()
	if strings.Contains(rootHelp, "impact, harness, and lrf read without mutating.") {
		t.Fatal("root help still claims impact and harness never mutate anything, contradicting the self-observation ledger append")
	}
	flowed := strings.Join(strings.Fields(rootHelp), " ")
	for _, required := range []string{
		"read without mutating repository or trace state.",
		"self-observation ledger (.corvint/self-observations.jsonl) on an unsupported-* failure",
		"a ledger storage failure never alters the response",
	} {
		if !strings.Contains(flowed, required) {
			t.Fatalf("root help does not disclose the ledger write: missing %q", required)
		}
	}
}

func TestUnknownHelpTopicIsConventionalError(t *testing.T) {
	t.Parallel()
	reader := &forbiddenHelpReader{}
	var stdout, stderr bytes.Buffer
	exit := run([]string{"help", "not-a-command"}, reader, &stdout, &stderr)
	if exit != 2 || stdout.Len() != 0 || reader.reads != 0 ||
		!strings.HasPrefix(stderr.String(), "{\"code\": \"invalid-arguments\", \"error\": \"unknown help topic: 'not-a-command' (choose from 'init', ") ||
		!strings.HasSuffix(stderr.String(), "'docs maintain')\", \"ok\": false}\n") {
		t.Fatalf("exit=%d stdout=%q stderr=%q reads=%d", exit, &stdout, &stderr, reader.reads)
	}
}

// V1-1643: the CPU-profile help note names the variable runtimeenv resolves
// for the "CPUPROFILE" suffix that main.go and taskcontext.go read.
func TestCPUProfileHelpNamesResolvedVariable(t *testing.T) {
	t.Parallel()
	var asked []string
	runtimeenv.Resolve(func(name string) (string, bool) {
		asked = append(asked, name)
		return "", false
	}, "CPUPROFILE")
	if len(asked) != 1 {
		t.Fatalf("runtimeenv looked up %q", asked)
	}
	if !strings.Contains(cpuProfileHelpNote, "\n"+asked[0]+"=PATH") {
		t.Fatalf("cpuProfileHelpNote does not name %s: %q", asked[0], cpuProfileHelpNote)
	}
}
