package contextindex

import (
	"slices"
	"testing"
)

func TestEvalPruneAuxiliarySymbolsPrefersBehaviorUnlessValueIsNamed(t *testing.T) {
	candidate := func(name, kind string, score int) evalSymbolCandidate {
		id := "active_help.go:" + name
		return evalSymbolCandidate{
			score: score,
			id:    id,
			symbol: evalPreparedSymbol{
				symbol:      Symbol{Path: "active_help.go", Name: name, Kind: kind},
				name:        terms(name),
				nameParts:   evalOrderedTerms(name),
				compactName: compactText(name),
			},
		}
	}
	candidates := []evalSymbolCandidate{
		candidate("AppendActiveHelp", "func", 235),
		candidate("GetActiveHelpConfig", "func", 232),
		candidate("activeHelpEnvVar", "func", 232),
		candidate("activeHelpMarker", "var", 231),
		candidate("activeHelpGlobalDisable", "var", 228),
		candidate("activeHelpEnvVarSuffix", "var", 226),
	}
	got := evalPruneAuxiliarySymbols(candidates, "read active help configuration from the environment")
	ids := make([]string, len(got))
	for index, item := range got {
		ids[index] = item.id
	}
	want := []string{
		"active_help.go:AppendActiveHelp",
		"active_help.go:GetActiveHelpConfig",
		"active_help.go:activeHelpEnvVar",
	}
	if !slices.Equal(ids, want) {
		t.Fatalf("pruned ids = %v, want %v", ids, want)
	}

	named := evalPruneAuxiliarySymbols(candidates, "change activeHelpMarker")
	if len(named) != 4 || named[3].id != "active_help.go:activeHelpMarker" {
		t.Fatalf("explicitly named value was removed: %v", named)
	}
	separated := append(candidates, candidate("GATE_ROWS", "const", 200))
	if got := evalPruneAuxiliarySymbols(separated, "render gate panel rows"); got[len(got)-1].id != "active_help.go:GATE_ROWS" {
		t.Fatalf("value named by separated task words was removed: %v", got)
	}
}
