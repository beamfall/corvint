package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/frontier"
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
		{"impact flag", []string{"impact", "--help"}, impactHelp},
		{"impact command", []string{"help", "impact"}, impactHelp},
		{name: "ASD-V0-009 feature flag without repository access", args: []string{"--root", "/definitely/not/a/repository", "feature", "--help"}, want: featureHelp},
		{name: "ASD-V0-009 feature command without repository access", args: []string{"--root", "/definitely/not/a/repository", "help", "feature"}, want: featureHelp},
		{name: "ASD-V0-009 eval flag without repository access", args: []string{"--root", "/definitely/not/a/repository", "eval", "--help"}, want: evalHelp},
		{name: "ASD-V0-009 eval command without repository access", args: []string{"--root", "/definitely/not/a/repository", "help", "eval"}, want: evalHelp},
		{"lrf flag", []string{"lrf", "--help"}, lrfHelp},
		{"lrf command", []string{"help", "lrf"}, lrfHelp},
		{name: "GPK-V0-002 ocm help parity", args: []string{"ocm", "--help"}, want: ocmHelp},
		{"ocm command", []string{"help", "ocm"}, ocmHelp},
		{"frontier flag", []string{"frontier", "--help"}, frontierHelp},
		{"frontier command", []string{"help", "frontier"}, frontierHelp},
		{"event flag", []string{"harness", "event", "--help"}, harnessEventHelp},
		{"event command", []string{"help", "harness", "event"}, harnessEventHelp},
		{"cem prepare flag", []string{"cem", "prepare", "--help"}, cemHelp},
		{"harness flag", []string{"harness", "--help"}, harnessEventHelp},
		{"harness command", []string{"help", "harness"}, harnessEventHelp},
		{"WQO-V0-032 work flag is help, not command input", []string{"work", "--help"}, workHelp},
		{"work command", []string{"help", "work"}, workHelp},
		{"prove-observe flag", []string{"prove-observe", "--help"}, proveObserveHelp},
		{"prove-observe command", []string{"help", "prove-observe"}, proveObserveHelp},
		{"adapter flag", []string{"adapter", "--help"}, adapterHelp},
		{"adapter command", []string{"help", "adapter"}, adapterHelp},
		{"dogfood OCM flag", []string{"dogfood-ocm", "--help"}, dogfoodOCMHelp},
		{"dogfood OCM command", []string{"help", "dogfood-ocm"}, dogfoodOCMHelp},
		{"witness flag", []string{"witness", "--help"}, witnessHelp},
		{"witness command", []string{"help", "witness"}, witnessHelp},
		{"LPCV-V0-051 test-validity flag", []string{"test-validity", "--help"}, testValidityHelp},
		{"test-validity command", []string{"help", "test-validity"}, testValidityHelp},
		{"invalid root is not opened", []string{"--root", "/definitely/not/a/repository", "query", "--help"}, helpText("query")},
		{"inline root is not opened", []string{"--root=/definitely/not/a/repository", "impact", "--help"}, impactHelp},
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
			{[]string{"impact", "path/a.go", "--help"}, impactHelp},
			{[]string{"prove", "--task", "x", "--help"}, proveHelp},
			{[]string{"index", "--if-stale", "--help"}, indexHelp},
			{[]string{"cem", "begin", "--patch", "p", "--help"}, cemHelp},
			{[]string{"harness", "event", "--host", "codex", "--help"}, harnessEventHelp},
			{[]string{"docs", "maintain", "--page", "p", "--help"}, docsMaintainHelp},
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
	for _, test := range []struct {
		args      []string
		wantRoot  string
		wantIndex int
	}{
		{[]string{"--root=-x", "calibrate"}, "-x", 1},
		{[]string{"--root", "-", "calibrate"}, "-", 2},
		{[]string{"--root", "-5", "calibrate"}, "-5", 2},
		{[]string{"--root", "--", "calibrate"}, "--", 2},
		{[]string{"--root", "a", "--root", "--task"}, "a", 2},
		{[]string{"--root"}, "", 0},
	} {
		if root, index := calibrateRoot(test.args); root != test.wantRoot || index != test.wantIndex {
			t.Errorf("calibrateRoot(%q) = %q, %d; want %q, %d", test.args, root, index, test.wantRoot, test.wantIndex)
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
	docs := func(args ...string) error {
		_, _, err := parseDocsInvocation(append([]string{"docs"}, args...))
		return err
	}
	maintain := func(args ...string) error {
		_, _, err := parseDocsMaintainInvocation(append([]string{"docs", "maintain"}, args...))
		return err
	}
	for _, test := range []struct {
		name, missing string
		parse         func(value string, inline bool) error
	}{
		{"docs", "missing docs argument value: --source", func(v string, inline bool) error {
			if inline {
				return docs("draft", "--source="+v)
			}
			return docs("draft", "--source", v)
		}},
		{"docs-maintain", "missing docs maintain argument value: --page", func(v string, inline bool) error {
			if inline {
				return maintain("--page=" + v)
			}
			return maintain("--page", v)
		}},
		{"witness", "missing value for --base", func(v string, inline bool) error {
			if inline {
				return errorOf(parseWitnessOptions([]string{"--base=" + v}))
			}
			return errorOf(parseWitnessOptions([]string{"--base", v}))
		}},
		{"test-validity", "missing value for --receipt", func(v string, inline bool) error {
			if inline {
				return errorOf(parseTestValidityOptions([]string{"--receipt=" + v}))
			}
			return errorOf(parseTestValidityOptions([]string{"--receipt", v}))
		}},
		{"frontier", frontier.CodeInvalidInput, func(v string, inline bool) error {
			if inline {
				return errorOf(parseFrontierOptions([]string{"--target=" + v}))
			}
			return errorOf(parseFrontierOptions([]string{"--target", v}))
		}},
		{"work", "missing work argument value", func(v string, inline bool) error {
			if inline {
				return errorOf(parseWorkOptions([]string{"propose-wave", "--limit", "1", "--envelope=" + v}))
			}
			return errorOf(parseWorkOptions([]string{"propose-wave", "--limit", "1", "--envelope", v}))
		}},
		{"local-completion", "local-completion-option-value-required", func(v string, inline bool) error {
			if inline {
				return errorOf(localCompletionFlags([]string{"begin", "--plan=" + v}))
			}
			return errorOf(localCompletionFlags([]string{"begin", "--plan", v}))
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
		{"nested cem action", []string{"cem", "begin", "--patch", "p", "-h"}, cemHelp},
		{"harness event", []string{"harness", "event", "--host", "codex", "-h"}, harnessEventHelp},
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
		if !strings.HasPrefix(rootHelp, "Corvint extraction alpha\n") {
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
	for _, command := range []string{"adapter", "dogfood-ocm", "witness"} {
		if !strings.Contains("\n"+commands, "\n  "+command+" ") {
			t.Errorf("root help command list omits %q", command)
		}
	}
}

func TestRootHelpListsContextAndWorkCommands(t *testing.T) {
	t.Parallel()
	for _, required := range []string{
		"context --task TEXT [--subject PATH] [--limit N]",
		"context        Compile the task-context packet",
		"work observe",
		"work propose-wave --envelope PATH --limit N",
		"work           Validate a repository queue observation or compile a non-operative shadow wave.",
	} {
		if !strings.Contains(rootHelp, required) {
			t.Fatalf("root help does not contain %q", required)
		}
	}
}

func TestOCMHelpExposesNativeWorkflowAndMutationBoundary(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		required []string
	}{
		{name: "GPK-V0-001 complete OCM CLI surface", required: []string{
			"ocm prepare", "ocm link", "ocm mark", "ocm status", "ocm verify", "ocm report",
		}},
		{name: "GPK-V0-008 OCM mutation boundary", required: []string{
			"source-body-free 0600 local map atomically",
			"verifies the OCM and bound CEM before mutation",
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, required := range test.required {
				if !strings.Contains(ocmHelp, required) {
					t.Fatalf("OCM help does not contain %q", required)
				}
			}
		})
	}
	if strings.Contains(ocmHelp, "Python implementation") {
		t.Fatal("OCM help still claims native mutators are unavailable")
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
	for _, required := range []string{
		"repository or trace\n  state",
		"self-observation ledger (.corvint/self-observations.jsonl) on an unsupported-* failure",
		"a ledger storage failure never alters the response",
	} {
		if !strings.Contains(rootHelp, required) {
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
		stderr.String() != "{\"code\": \"invalid-arguments\", \"error\": \"unknown help topic\", \"ok\": false}\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q reads=%d", exit, &stdout, &stderr, reader.reads)
	}
}
