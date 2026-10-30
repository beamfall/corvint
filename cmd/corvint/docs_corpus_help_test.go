package main

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

// corpusHelpRun runs one help invocation and requires the GPK-V0-059 help shape: exit 0, empty
// stderr, no stdin read and no repository inspection.
func corpusHelpRun(t *testing.T, args ...string) string {
	t.Helper()
	reader := &forbiddenHelpReader{}
	var stdout, stderr bytes.Buffer
	exit := run(append([]string{"--root", "/definitely/not/a/repository"}, args...), reader, &stdout, &stderr)
	if exit != 0 || stderr.Len() != 0 || reader.reads != 0 {
		t.Fatalf("%q: exit=%d stderr=%q reads=%d", args, exit, &stderr, reader.reads)
	}
	return stdout.String()
}

// issue805CorpusOperations is every operation parseCorpusInvocation admits.
var issue805CorpusOperations = []string{
	"manifest", "build", "render", "maintain", "cem", "behavior-adapter", "behavior-provider", "compact-provider", "estate",
	"discover-playwright", "witness-playwright", "discover-legacy", "import-cases", "search-cases", "eval", "audit-sample", "audit-report", "delta", "carry-forward", "discover-tests", "extract", "patterns", "info", "validate", "inventory", "search", "get",
	"trace", "related", "journey", "stability", "locate", "coverage", "gaps", "concept", "claims",
	"flow", "dependencies", "recommend-tests", "navigation", "vocabulary", "intent",
}

// DCP-V1-056: `help docs corpus` and `docs corpus --help` print one corpus topic that lists every
// corpus operation with a one-line purpose, not the generic docs orientation.
func TestDocsCorpusHelpListsOperations(t *testing.T) {
	t.Parallel()
	overview := corpusHelpRun(t, "help", "docs", "corpus")
	for _, args := range [][]string{{"docs", "corpus", "--help"}, {"docs", "corpus", "-h"}} {
		if got := corpusHelpRun(t, args...); got != overview {
			t.Fatalf("%q differs from help docs corpus:\n%s", args, got)
		}
	}
	if overview == docsHelp || strings.Contains(overview, "docs consume") {
		t.Fatalf("corpus help is the generic docs orientation:\n%s", overview)
	}
	for _, op := range issue805CorpusOperations {
		found := false
		for _, line := range strings.Split(overview, "\n") {
			fields := strings.Fields(line)
			if len(fields) > 1 && fields[0] == op {
				found = true
			}
		}
		if !found {
			t.Errorf("corpus help has no purpose line for %q", op)
		}
	}
	if !strings.Contains(overview, "corvint docs corpus OPERATION --help") {
		t.Errorf("corpus help does not point at per-operation help")
	}
}

// DCP-V1-056: `docs corpus OPERATION --help` prints that operation's usage and flags, names the
// flags whose files are relative to --root, and exits 0; every flag it names is admitted.
func TestDocsCorpusOperationHelpPrintsFlags(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"discover-playwright": {"--migration", "--config", "--playwright-list", "--receipt"},
		"witness-playwright":  {"--input", "--receipt-input", "--report"},
		"behavior-adapter":    {"--input", "--previous", "--check"},
		"search":              {"--artifact", "--query", "--limit", "--offset", "--retirement"},
		"maintain":            {"--artifact", "--page", "--apply"},
		"import-cases":        {"--format", "--input", "--content"},
		"search-cases":        {"--format", "--input", "--query", "--id", "--match", "--limit"},
		"eval":                {"--artifact", "--cases", "--baseline", "--format"},
		"audit-sample":        {"--artifact", "--seed", "--size", "--stratify", "--scope"},
		"audit-report":        {"--artifact", "--sample", "--format"},
		"delta":               {"--previous", "--artifact", "--manifest", "--limit", "--offset"},
		"carry-forward":       {"--previous", "--provider", "--artifact", "--manifest"},
		"patterns":            {"--artifact", "--format"},
		"build":               {"--manifest", "--artifact", "--max-unresolved-mention-rate", "--max-pattern-rate"},
	}
	rootRelative := map[string][]string{
		"discover-playwright": {"--migration", "--playwright-list", "--receipt"},
		"witness-playwright":  {"--input", "--report"},
		"behavior-adapter":    {"--input", "--previous"},
		"search":              {"--artifact", "--retirement"},
		"maintain":            {"--artifact"},
		"import-cases":        {"--input"},
		"search-cases":        {"--input"},
		"eval":                {"--artifact", "--cases", "--baseline"},
		"audit-sample":        {"--artifact"},
		"audit-report":        {"--artifact", "--sample"},
		"build":               {"--manifest", "--artifact"},
		"delta":               {"--manifest", "--artifact", "--previous"},
		"carry-forward":       {"--manifest", "--artifact", "--previous"},
		"patterns":            {"--artifact"},
	}
	for _, op := range issue805CorpusOperations {
		help := corpusHelpRun(t, "docs", "corpus", op, "--help")
		if help == docsHelp || !strings.Contains(help, "corvint [--root PATH] docs corpus "+op+" ") {
			t.Fatalf("%s --help is not its own usage:\n%s", op, help)
		}
		if got := corpusHelpRun(t, "docs", "corpus", op, "-h"); got != help {
			t.Fatalf("%s -h differs from --help", op)
		}
		// The usage line names exactly the literal admitted options, required ones unbracketed.
		usage, _, _ := strings.Cut(strings.SplitN(help, "corvint [--root PATH] docs corpus "+op+" ", 2)[1], "\n")
		var required, optional []string
		for _, token := range strings.Fields(usage) {
			if flag, bracketed := strings.CutPrefix(token, "["); bracketed && strings.HasPrefix(flag, "--") {
				optional = append(optional, strings.TrimSuffix(flag, "]"))
			} else if strings.HasPrefix(token, "--") {
				required = append(required, token)
			}
		}
		want := corpusFlagExpectations[op]
		want.optional = append(slices.Clone(want.optional), corpusWorkspaceOptional...) // DCP-V1-604
		if !slices.Equal(required, want.required) || !slices.Equal(optional, want.optional) {
			t.Errorf("%s usage options = %q %q, want %q %q", op, required, optional, want.required, want.optional)
		}
		for _, flag := range cases[op] {
			if !strings.Contains(help, flag+" ") {
				t.Errorf("%s help omits %s", op, flag)
			}
		}
		marker := "Relative to --root: "
		_, after, ok := strings.Cut(help, marker)
		if want := rootRelative[op]; want != nil {
			line, _, _ := strings.Cut(after, ".")
			if !ok || strings.Join(strings.Fields(strings.ReplaceAll(line, ",", " ")), " ") != strings.Join(want, " ") {
				t.Errorf("%s help root-relative line = %q, want %q", op, line, want)
			}
		}
	}
	// An unknown operation is not a help topic; the parser refuses its --help first as an option.
	reader := &forbiddenHelpReader{}
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"docs", "corpus", "nope", "--help"}, reader, &stdout, &stderr); exit != 2 || stdout.Len() != 0 ||
		stderr.String() != "{\"code\": \"invalid-arguments\", \"error\": \"unsupported corpus option\", \"ok\": false}\n" {
		t.Errorf("unknown operation --help: exit=%d stdout=%q stderr=%q", exit, &stdout, &stderr)
	}
	witness := corpusHelpRun(t, "docs", "corpus", "witness-playwright", "--help")
	if !strings.Contains(witness, "names a\nREQUEST.json input, not a file") || !strings.Contains(witness, "corvint-js-test-provider e2e") {
		t.Errorf("witness-playwright help drops the DCP-V1-053 receipt producer:\n%s", witness)
	}
	// A --help after a value-taking option's value, or after a no-value option, is still help.
	if got := corpusHelpRun(t, "docs", "corpus", "behavior-adapter", "--input", "x.json", "--check", "--help"); got != corpusHelpRun(t, "docs", "corpus", "behavior-adapter", "--help") {
		t.Errorf("behavior-adapter --help after options differs")
	}
}

// DCP-V1-057: an unknown help topic stays the conventional invalid-arguments refusal and lists
// every valid topic, each of which resolves.
func TestUnknownHelpTopicListsValidTopics(t *testing.T) {
	t.Parallel()
	reader := &forbiddenHelpReader{}
	var stdout, stderr bytes.Buffer
	exit := run([]string{"help", "docs", "nope"}, reader, &stdout, &stderr)
	prefix := "{\"code\": \"invalid-arguments\", \"error\": \"unknown help topic: 'docs nope' (choose from "
	if exit != 2 || stdout.Len() != 0 || reader.reads != 0 || !strings.HasPrefix(stderr.String(), prefix) || !strings.HasSuffix(stderr.String(), ")\", \"ok\": false}\n") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, &stdout, &stderr)
	}
	list := strings.TrimSuffix(strings.TrimPrefix(stderr.String(), prefix), ")\", \"ok\": false}\n")
	topics := map[string]bool{}
	for _, quoted := range strings.Split(list, ", ") {
		topic := strings.Trim(quoted, "'")
		topics[topic] = true
		corpusHelpRun(t, append([]string{"help"}, strings.Fields(topic)...)...)
	}
	for _, want := range []string{"query", "docs", "docs corpus", "harness event", "test-plan"} {
		if !topics[want] {
			t.Errorf("valid topics omit %q: %s", want, list)
		}
	}
}

// DCP-V1-057 (V1-2308, GitHub #1476): every topic the unknown-topic refusal lists resolves when
// passed exactly as listed, as one argument, to the same help as its separate-word form.
func TestListedHelpTopicResolvesAsOneArgument(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	run([]string{"help", "not-a-topic"}, &forbiddenHelpReader{}, &stdout, &stderr)
	prefix := "{\"code\": \"invalid-arguments\", \"error\": \"unknown help topic: 'not-a-topic' (choose from "
	list := strings.TrimSuffix(strings.TrimPrefix(stderr.String(), prefix), ")\", \"ok\": false}\n")
	if list == stderr.String() {
		t.Fatalf("unexpected refusal: %q", stderr.String())
	}
	nested := 0
	for _, quoted := range strings.Split(list, ", ") {
		topic := strings.Trim(quoted, "'")
		if strings.Contains(topic, " ") {
			nested++
		}
		if got, want := corpusHelpRun(t, "help", topic), corpusHelpRun(t, append([]string{"help"}, strings.Fields(topic)...)...); got != want {
			t.Errorf("help %q differs from its separate-word form", topic)
		}
	}
	if nested != len(nestedHelpTopics) {
		t.Fatalf("listed %d nested topics, want %d: %s", nested, len(nestedHelpTopics), list)
	}
}
