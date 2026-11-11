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

// TestNativeVerbOptionValuesFollowArgparseOptionLikeClassification pins
// GPK-V0-064 as amended by decision 0196 for the parsers decision 0173 did not
// list: an option-like next token is refused through that parser's own
// missing-value path, while the inline `=` form and a negative number are
// values.
func TestNativeVerbOptionValuesFollowArgparseOptionLikeClassification(t *testing.T) {
	t.Parallel()
	errorOf := func(_ any, err error) error { return err }
	// The docs parsers moved to cmd/corvint-docs with their own copy of this pin.
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
	for _, command := range []string{} {
		if !strings.Contains("\n"+commands, "\n    "+command+" ") {
			t.Errorf("root help command list omits %q", command)
		}
	}
}

func TestRootHelpListsContextAndWorkCommands(t *testing.T) {
	t.Parallel()
	for _, required := range []string{
		"\n    context  path brief",
		"corvint COMMAND --help | corvint help COMMAND",
	} {
		if !strings.Contains(rootHelp, required) {
			t.Fatalf("root help does not contain %q", required)
		}
	}
	for _, required := range []string{"context --path PATH [--path PATH ...]"} {
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

func TestImpactHelpExposesActualLimitsAndUnsupportedProfiles(t *testing.T) {
	t.Parallel()
	for _, required := range []string{
		"default: 10; range: 1-50",
		"the root package included",
		"at most 100 paths by default (256 only for the\n  explicit expanded range profile) and 1024 characters per path.",
		"1,000,000 bytes per source and 128 MiB in total",
		"Mixed-worktree freshness is disclosed",
		"Every profile uses only revision-tracked evidence",
		"admitted but lacking a reverse-import rule",
		"unsupported-impact-path-suffix",
	} {
		if !strings.Contains(impactHelp, required) {
			t.Fatalf("impact help does not contain %q", required)
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
