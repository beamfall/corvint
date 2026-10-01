package contextindex

import (
	"slices"
	"testing"
)

// evalSiblingCandidate builds one same-path symbol candidate whose full
// support is its name support plus the given window words.
func evalSiblingCandidate(path, name, kind string, score int, queryTerms map[string]struct{}, window ...string) evalSymbolCandidate {
	prepared := evalPreparedSymbol{
		symbol:      Symbol{Path: path, Name: name, Kind: kind},
		name:        terms(name),
		nameParts:   evalOrderedTerms(name),
		compactName: compactText(name),
	}
	candidate := evalSymbolCandidate{score: score, id: path + ":" + name, symbol: prepared}
	candidate.support = evalNameSupport(candidate, queryTerms)
	for _, word := range window {
		candidate.support[word] = struct{}{}
	}
	return candidate
}

func evalSiblingIDs(candidates []evalSymbolCandidate) []string {
	ids := make([]string, len(candidates))
	for index, candidate := range candidates {
		ids[index] = candidate.id
	}
	return ids
}

// The blind-v6 forbidden-hit class (decision 0307): a sibling admitted beside
// the target because the two share a context window. GPK-V0-040 (amended
// 2026-10-01): a same-path sibling stays only with evidence of its own.
func TestEvalExplainAwaySiblingsDropsNeighboursWithoutOwnEvidence(t *testing.T) {
	t.Run("GPK-V0-040 shared-window sibling is explained away", func(t *testing.T) {
		query := "read active help configuration from the environment"
		queryTerms := terms(query)
		candidates := []evalSymbolCandidate{
			evalSiblingCandidate("active_help.go", "AppendActiveHelp", "func", 235, queryTerms, "environment"),
			evalSiblingCandidate("active_help.go", "GetActiveHelpConfig", "func", 232, queryTerms, "environment", "read"),
		}
		got := evalSiblingIDs(evalExplainAwaySiblings(candidates, queryTerms, query))
		if want := []string{"active_help.go:GetActiveHelpConfig"}; !slices.Equal(got, want) {
			t.Fatalf("kept %v, want %v: `config` answers \"configuration\", so AppendActiveHelp's {active, help} is a strict subset", got, want)
		}
	})
	t.Run("GPK-V0-040 a sibling with a distinct window word stays", func(t *testing.T) {
		query := "read active help configuration from the environment"
		queryTerms := terms(query)
		candidates := []evalSymbolCandidate{
			evalSiblingCandidate("active_help.go", "GetActiveHelpConfig", "func", 232, queryTerms, "environment"),
			evalSiblingCandidate("active_help.go", "AppendActiveHelp", "func", 235, queryTerms, "read"),
		}
		if got := evalSiblingIDs(evalExplainAwaySiblings(candidates, queryTerms, query)); len(got) != 2 {
			t.Fatalf("kept %v, want both: \"read\" reaches only the sibling's window", got)
		}
	})
	t.Run("GPK-V0-040 identical support keeps both", func(t *testing.T) {
		query := "generate Bash shell completion output"
		queryTerms := terms(query)
		candidates := []evalSymbolCandidate{
			evalSiblingCandidate("bash_completions.go", "GenBashCompletion", "func", 300, queryTerms),
			evalSiblingCandidate("bash_completions.go", "GenBashCompletionFile", "func", 290, queryTerms),
		}
		if got := evalSiblingIDs(evalExplainAwaySiblings(candidates, queryTerms, query)); len(got) != 2 {
			t.Fatalf("kept %v, want both: nothing in the task tells them apart", got)
		}
	})
	t.Run("GPK-V0-040 a sibling the task names stays", func(t *testing.T) {
		query := "why does AppendActiveHelp read the active help configuration"
		queryTerms := terms(query)
		candidates := []evalSymbolCandidate{
			evalSiblingCandidate("active_help.go", "GetActiveHelpConfig", "func", 232, queryTerms, "read"),
			evalSiblingCandidate("active_help.go", "AppendActiveHelp", "func", 200, queryTerms),
		}
		if got := evalSiblingIDs(evalExplainAwaySiblings(candidates, queryTerms, query)); len(got) != 2 {
			t.Fatalf("kept %v, want both: the task writes the sibling's name", got)
		}
	})
	t.Run("GPK-V0-040 different paths are never compared", func(t *testing.T) {
		query := "read active help configuration from the environment"
		queryTerms := terms(query)
		candidates := []evalSymbolCandidate{
			evalSiblingCandidate("active_help.go", "GetActiveHelpConfig", "func", 232, queryTerms),
			evalSiblingCandidate("other.go", "ActiveHelp", "type", 100, queryTerms),
		}
		if got := evalSiblingIDs(evalExplainAwaySiblings(candidates, queryTerms, query)); len(got) != 2 {
			t.Fatalf("kept %v, want both", got)
		}
	})
}

func TestEvalNameSupportAnswersPrefixes(t *testing.T) {
	t.Run("GPK-V0-040 four-byte prefixes in either direction", func(t *testing.T) {
		queryTerms := terms("convert an async view to synchronous execution with the config")
		candidate := evalSiblingCandidate("app.py", "ensure_sync", "method", 1, queryTerms)
		support := evalNameSupport(candidate, queryTerms)
		if _, ok := support["synchronous"]; !ok {
			t.Fatalf("`sync` does not answer \"synchronous\": %v", support)
		}
		short := evalSiblingCandidate("app.py", "to_env", "func", 1, queryTerms)
		if support := evalNameSupport(short, terms("read the environment")); len(support) != 0 {
			t.Fatalf("a three-byte part answered a prefix: %v", support)
		}
	})
}
