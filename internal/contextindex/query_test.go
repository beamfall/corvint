package contextindex

import (
	"strings"
	"testing"
)

const queryTaskFixture = "Identify the active work queue, required workflow gates, and minimum context needed to safely take the next roadmap ticket"

func TestQueryIntentUsesEvalQueryNormalization(t *testing.T) {
	for _, test := range []struct {
		text, want string
	}{
		{strings.ToUpper(queryTaskFixture), "project-operations"},
		{"  ROADMAP\u2003WORK QUEUE  ", "project-operations"},
		{"CORVINT\u2003AGENT\u2003CONTEXT\u2003ROADMAP", "agent-tooling"},
		{"fix token parser", "repository"},
	} {
		if got := QueryIntent(test.text); got != test.want {
			t.Fatalf("QueryIntent(%q)=%q want=%q", test.text, got, test.want)
		}
		if got := evalInferIntent(test.text).id; got != test.want {
			t.Fatalf("evalInferIntent(%q)=%q want=%q", test.text, got, test.want)
		}
	}
}

func TestTCPV0008NestedInstructionEligibilityIsShared(t *testing.T) {
	for _, candidate := range []string{
		".github/workflows/AGENTS.md",
		"docs/agent-workflows/CLAUDE.md",
		"docs/plans/AGENTS.md",
		"script/GEMINI.md",
	} {
		if documentKind(candidate) != "instructions" {
			t.Fatalf("%s is no longer classified as instructions", candidate)
		}
		if projectOperationPath(candidate) {
			t.Errorf("project operations admitted nested instruction path %s", candidate)
		}
		if _, eligible := instructionRank(candidate); eligible {
			t.Errorf("governing rank admitted nested instruction path %s", candidate)
		}
	}
	for _, candidate := range []string{"AGENTS.md", "CLAUDE.md", "GEMINI.md"} {
		if !projectOperationPath(candidate) {
			t.Errorf("project operations rejected existing root instruction path %s", candidate)
		}
		if _, eligible := instructionRank(candidate); !eligible {
			t.Errorf("governing rank rejected existing root instruction path %s", candidate)
		}
	}
	for _, candidate := range []string{
		"copilot-instructions.md",
		".github/copilot-instructions.md",
		".github/instructions/review.instructions.md",
	} {
		if projectOperationPath(candidate) {
			t.Errorf("project operations newly admitted instruction path %s", candidate)
		}
		if _, eligible := instructionRank(candidate); !eligible {
			t.Errorf("governing rank rejected its existing instruction path %s", candidate)
		}
	}
	for _, candidate := range []string{"Makefile", ".github/workflows/test.yml", "docs/plans/release.md", "script/check.sh"} {
		if !projectOperationPath(candidate) {
			t.Errorf("project operations rejected non-instruction path %s", candidate)
		}
	}
}

func authorityRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	files := map[string]string{
		".gitignore": ".context-corvint/\n",
		"AGENTS.md": "# Project instructions\n\nThe roadmap is the only active work queue.\n" +
			"Run `make orient`, then `script/context-packet.sh --ticket ID`.\n" +
			"Use `script/roadmap.sh` and run the required workflow gates.\n",
		"script/context-packet.sh": "#!/bin/sh\nexit 0\n",
		"script/roadmap.sh":        "#!/bin/sh\nexit 0\n",
	}
	for path, content := range files {
		writeTestFile(t, root, path, content)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "initial authority fixture")
	return root
}

func TestQueryIntentMatchesProjectOperationsBoundary(t *testing.T) {
	for _, test := range []struct {
		text, want string
	}{
		{queryTaskFixture, "project-operations"},
		{"roadmap only", "repository"},
		{"corvint agent context roadmap", "agent-tooling"},
		{"work queue ticket", "project-operations"},
	} {
		intent, _ := queryIntent(strings.ToLower(test.text), terms(test.text))
		if intent != test.want {
			t.Fatalf("intent(%q)=%q want=%q", test.text, intent, test.want)
		}
	}
}

// TestPythonLowerSplitsTheDottedCapitalIAsTheOracleDoes: Python's str.lower
// maps U+0130 to "i" plus U+0307, so the oracle tokenizes "İstanbul" as
// "stanbul"; Go's simple case mapping would keep "istanbul".
func TestPythonLowerSplitsTheDottedCapitalIAsTheOracleDoes(t *testing.T) {
	values := terms(pythonLower("İstanbul Parser fix for the naïve tokenizer AİB"))
	for _, want := range []string{"stanbul", "parser", "fix", "na", "ve", "tokenizer", "ai"} {
		if _, ok := values[want]; !ok {
			t.Errorf("missing %q in %v", want, values)
		}
	}
	for _, unwanted := range []string{"istanbul", "aib", "naïve"} {
		if _, ok := values[unwanted]; ok {
			t.Errorf("unexpected %q in %v", unwanted, values)
		}
	}
}
