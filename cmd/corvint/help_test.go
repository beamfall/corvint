package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/frontier"
	"github.com/Beamfall/corvint/internal/runtimeenv"
)

type forbiddenHelpReader struct{ reads int }

// The supported basis relation table in docs/CHANGE-EVIDENCE-MAP.md governs these values.
func TestCEMCiteHelpListsRelations(t *testing.T) {
	reader := &forbiddenHelpReader{}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--root", "/definitely/not/a/repository", "cem", "cite", "--help"}, reader, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 || reader.reads != 0 {
		t.Fatalf("exit=%d stderr=%q reads=%d", code, &stderr, reader.reads)
	}
	for _, text := range []string{
		"--relation accepts: call-site, decision, dependency, implementation,\n           incident, specification, test-claim.",
		"docs/CHANGE-EVIDENCE-MAP.md#supported",
	} {
		if !strings.Contains(stdout.String(), text) {
			t.Fatalf("cem cite help missing %q", text)
		}
	}
}

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
		{"lrf flag", []string{"lrf", "--help"}, helpText("lrf")},
		{"lrf command", []string{"help", "lrf"}, helpText("lrf")},
		{name: "GPK-V0-002 ocm help parity", args: []string{"ocm", "--help"}, want: helpText("ocm")},
		{"ocm command", []string{"help", "ocm"}, helpText("ocm")},
		{"frontier flag", []string{"frontier", "--help"}, helpText("frontier")},
		{"frontier command", []string{"help", "frontier"}, helpText("frontier")},
		{"event flag", []string{"harness", "event", "--help"}, helpText("harness-event")},
		{"event command", []string{"help", "harness", "event"}, helpText("harness-event")},
		{"CEM-V1-340 cem prepare flag", []string{"cem", "prepare", "--help"}, helpText(cemActionHelpPrefix + "prepare")},
		{"cem flag", []string{"cem", "--help"}, helpText("cem")},
		{"harness flag", []string{"harness", "--help"}, helpText("harness-event")},
		{"harness command", []string{"help", "harness"}, helpText("harness-event")},
		{"WQO-V0-032 work flag is help, not command input", []string{"work", "--help"}, helpText("work")},
		{"work command", []string{"help", "work"}, helpText("work")},
		{"prove-observe flag", []string{"prove-observe", "--help"}, helpText("prove-observe")},
		{"prove-observe command", []string{"help", "prove-observe"}, helpText("prove-observe")},
		{"adapter flag", []string{"adapter", "--help"}, helpText("adapter")},
		{"adapter command", []string{"help", "adapter"}, helpText("adapter")},
		{"dogfood OCM flag", []string{"dogfood-ocm", "--help"}, helpText("dogfood-ocm")},
		{"dogfood OCM command", []string{"help", "dogfood-ocm"}, helpText("dogfood-ocm")},
		{"witness flag", []string{"witness", "--help"}, helpText("witness")},
		{"witness command", []string{"help", "witness"}, helpText("witness")},
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
			{[]string{"prove", "--task", "x", "--help"}, helpText("prove")},
			{[]string{"index", "--if-stale", "--help"}, helpText("index")},
			{[]string{"cem", "begin", "--patch", "p", "--help"}, helpText(cemActionHelpPrefix + "begin")},
			{[]string{"harness", "event", "--host", "codex", "--help"}, helpText("harness-event")},
			{[]string{"docs", "maintain", "--page", "p", "--help"}, helpText("docs-maintain")},
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
		{"nested cem action", []string{"cem", "begin", "--patch", "p", "-h"}, helpText(cemActionHelpPrefix + "begin")},
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
	for _, command := range []string{"adapter", "dogfood-ocm", "witness"} {
		if !strings.Contains("\n"+commands, "\n    "+command+" ") {
			t.Errorf("root help command list omits %q", command)
		}
	}
}

func TestRootHelpListsContextAndWorkCommands(t *testing.T) {
	t.Parallel()
	for _, required := range []string{
		"\n    context  task-context packet",
		"\n    work  queue observation",
		"corvint COMMAND --help | corvint help COMMAND",
	} {
		if !strings.Contains(rootHelp, required) {
			t.Fatalf("root help does not contain %q", required)
		}
	}
	for _, required := range []string{"context --task TEXT [--subject PATH] [--limit N]", "work propose-wave --envelope PATH --limit N"} {
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
	commands, _, _ := strings.Cut(strings.SplitN(rootHelp, "\nCommands:\n", 2)[1], "\nGlobal options:")
	if lines := strings.Count(commands, "\n"); lines != len(topLevelCommands)+2 {
		t.Fatalf("Commands has %d lines, want two group headers and one per verb (%d)", lines, len(topLevelCommands))
	}
	if strings.Contains(rootHelp, "\nCommand maturity:\n") {
		t.Error("root help repeats every verb in a separate Command maturity section")
	}
	for _, command := range topLevelCommands {
		if count := strings.Count(rootHelp, "\n    "+command+" "); count != 1 {
			t.Errorf("root help lists verb %q %d times, want once", command, count)
		}
	}
	for _, flag := range []string{"--task", "--base", "--cem", "--limit"} {
		if strings.Contains(rootHelp, flag) {
			t.Errorf("root help carries per-verb option %q", flag)
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
		// V1-0706: the implemented --replace recovery and its review loss.
		{name: "OCM prepare --replace recovery", required: []string{
			"--replace", "map-outdated", "links, marks and assessments are\n                      discarded",
		}},
		// V1-0558: the report output confinement and its worktree visibility.
		{name: "OCM report output confinement", required: []string{
			".git/corvint/ocm-review.md", "unsupported-ocm-output-path", "blocks clean-worktree admission",
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

// TestCEMV1340CEMActionHelpNamesOnlyThatAction: every parsed cem action has
// its own help, projected from cemHelp, that carries its usage and Actions
// paragraph and no other action's usage line (CEM-V1-340).
func TestCEMV1340CEMActionHelpNamesOnlyThatAction(t *testing.T) {
	t.Parallel()
	for action := range cemHelpActions {
		text := cemActionHelp(action)
		if text == "" {
			t.Fatalf("cem %s has no action help", action)
		}
		if !strings.Contains(text, "  corvint [--root PATH] cem "+action+" ") && !strings.Contains(text, "  corvint cem "+action+" ") {
			t.Errorf("cem %s help lacks its usage: %q", action, text)
		}
		if len(text) >= len(cemHelp) {
			t.Errorf("cem %s help is not narrower than cem --help", action)
		}
		for other := range cemHelpActions {
			if other != action && (strings.Contains(text, "  corvint [--root PATH] cem "+other+" ") || strings.Contains(text, "  corvint cem "+other+" ")) {
				t.Errorf("cem %s help carries the usage of cem %s", action, other)
			}
		}
		verifying := action == "status" || action == "verify" || action == "report"
		if strings.Contains(text, "\nVerification options:\n") != verifying {
			t.Errorf("cem %s help: Verification options present = %v, want %v", action, !verifying, verifying)
		}
	}
	if !strings.Contains(cemActionHelp("cite"), "--relation accepts: call-site") {
		t.Errorf("cem cite help lacks its relation list: %q", cemActionHelp("cite"))
	}
	if cemActionHelp("no-such-action") != "" {
		t.Error("an undocumented action has help")
	}
}

// TestCEMV1340HelpCEMActionAndOwnedArgvActions covers the per-action help
// routes V1-2109 adds: `corvint help cem ACTION` for every action, and
// `--help`/`-h` on verify-stable and gate-verdict, which own their argv and
// otherwise refuse it. Each prints exactly that action's help and exits 0.
func TestCEMV1340HelpCEMActionAndOwnedArgvActions(t *testing.T) {
	t.Parallel()
	check := func(args []string, action string) {
		t.Helper()
		reader := &forbiddenHelpReader{}
		var stdout, stderr bytes.Buffer
		code := run(args, reader, &stdout, &stderr)
		if code != 0 || stderr.Len() != 0 || reader.reads != 0 {
			t.Fatalf("%q: exit=%d stderr=%q reads=%d", args, code, &stderr, reader.reads)
		}
		if stdout.String() != helpText(cemActionHelpPrefix+action) {
			t.Fatalf("%q printed %q, want cem %s help", args, stdout.String(), action)
		}
	}
	for action := range cemHelpActions {
		check([]string{"help", "cem", action}, action)
		check([]string{"--root", "/definitely/not/a/repository", "help", "cem", action}, action)
	}
	for _, action := range []string{"verify-stable", "gate-verdict", "bundle"} {
		check([]string{"cem", action, "--help"}, action)
		check([]string{"--root", "/definitely/not/a/repository", "cem", action, "-h"}, action)
		check([]string{"cem", action, "--base", "x", "--help"}, action)
	}
	// A --help consumed as an option value is not a help request: the closed
	// parser still owns the argv.
	var stdout, stderr bytes.Buffer
	if code := run([]string{"cem", "verify-stable", "--map", "--help"}, &forbiddenHelpReader{}, &stdout, &stderr); code != 2 ||
		!strings.Contains(stdout.String(), `"code":"invalid-arguments"`) {
		t.Fatalf("verify-stable --map --help: exit=%d stdout=%q", code, &stdout)
	}
}

// TestTCBV0045GateVerdictHelpNamesBuildMismatchRecovery covers V1-2324:
// `corvint help cem gate-verdict` names the verifier-build-mismatch refusal
// and the consumer rebuild command TCB-V0-045 prescribes.
func TestTCBV0045GateVerdictHelpNamesBuildMismatchRecovery(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"help", "cem", "gate-verdict"}, &forbiddenHelpReader{}, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("help cem gate-verdict: exit=%d stderr=%q", code, &stderr)
	}
	for _, want := range []string{"verifier-disagreement", "verifier-build-mismatch", `(cd interop/cem01-go && GOTOOLCHAIN=local go build -o "$CONSUMER" .)`} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("gate-verdict help lacks %q:\n%s", want, &stdout)
		}
	}
}

// V1-1643: the CPU-profile help note names the variable runtimeenv resolves
// for the "CPUPROFILE" suffix that main.go and taskcontext.go read.
func TestCPUProfileHelpNamesResolvedVariable(t *testing.T) {
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

// TestGVAV0096HelpDocumentsCompareKeysAndLabel pins GVA-V0-096 (V1-2960, #1946): migration-ratchet
// help names every compare key form it emits and --label, and the tests validity, lint and
// duplicates usage lines list --label next to the other agreement options.
func TestGVAV0096HelpDocumentsCompareKeysAndLabel(t *testing.T) {
	t.Parallel()
	ratchet := strings.Join(strings.Fields(helpText("migration-ratchet")), " ")
	for _, required := range []string{
		"[--log FILE.jsonl] [--label NAME]",
		"a policy failure CODE:ID as rule CODE on test ID",
		"a removal as rule removal:KIND on the baseline test ID",
		"rule state-regression:KIND on the candidate test ID",
		"rule broken-reverse-link:KIND on the candidate (else baseline) test ID",
		"an excepted delta abstains",
		"--label NAME (once) is copied to each record's label",
	} {
		if !strings.Contains(ratchet, required) {
			t.Errorf("migration-ratchet help does not contain %q", required)
		}
	}
	tests := strings.Join(strings.Fields(testsHelp), " ")
	for _, required := range []string{
		"tests validity --files PATH[,PATH...] [--files ...] [--legacy NEW=LEGACY ...] [--rev REV] [--profile NAME] [--advisory] [--compare FILE] [--log FILE] [--label NAME]",
		"[--json] [--advisory] [--compare FILE] [--log FILE] [--label NAME]",
		"[--decisions FILE ...] [--advisory] [--compare FILE] [--log FILE] [--label NAME]",
		"--label NAME (once) is copied to the record's label",
	} {
		if !strings.Contains(tests, required) {
			t.Errorf("tests help does not contain %q", required)
		}
	}
}
