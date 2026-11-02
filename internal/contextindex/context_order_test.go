package contextindex

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestContextOrderDefaultBytes: with every CRK ordering flag unset, off or
// any value other than its exact opt-in, the packet is byte-identical to the
// recipe golden (CRK-V0-007).
func TestContextOrderDefaultBytes(t *testing.T) {
	t.Run("CRK-V0-007", func(t *testing.T) {
		index := recipeFixtureIndex(t)
		golden, err := os.ReadFile("testdata/context-recipe-default-golden.json")
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range []string{"", "off", "ON", "yes"} {
			t.Setenv("CORVINT_CONTEXT_RRF", value)
			t.Setenv("CORVINT_CONTEXT_ADDITIVE", value)
			t.Setenv("CORVINT_CONTEXT_ACRONYMS", value)
			encoded, err := json.MarshalIndent(recipePacket(t, index, "", recipeFixtureTask), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(append(encoded, '\n'), golden) {
				t.Fatalf("flag value %q changed the default packet bytes", value)
			}
		}
	})
}

// TestContextOrderFuseRanks: RRF orders rows by the summed reciprocal ranks of
// the admitting and corroborating relations, stably, and names every term in
// the reason (CRK-V0-004).
func TestContextOrderFuseRanks(t *testing.T) {
	t.Run("CRK-V0-004", func(t *testing.T) {
		compiler := &taskContextCompiler{
			rrfOrder: true,
			support:  map[string][]string{"b.go": {"test"}},
			relationRank: map[string]map[string]int{
				"a.go": {"lexical": 1},
				"b.go": {"definition": 1, "test": 3},
				"c.go": {"test": 1},
			},
		}
		rows := compiler.orderRows([]contextRow{
			{kind: "lexical", path: "a.go", reason: "a"},
			{kind: "definition", path: "b.go", reason: "b", score: 800},
			{kind: "test", path: "c.go", reason: "c"},
		})
		order := []string{rows[0].path, rows[1].path, rows[2].path}
		if strings.Join(order, " ") != "b.go a.go c.go" {
			t.Fatalf("order = %v", order)
		}
		if rows[0].reason != "b; rrf 0.0323 (definition@1, test@3)" || rows[0].score != 850 {
			t.Fatalf("fused row = %#v", rows[0])
		}
		if rows[1].reason != "a; rrf 0.0164 (lexical@1)" {
			t.Fatalf("single-relation reason = %q", rows[1].reason)
		}
	})
}

// TestContextOrderRecordRelationRanks: a relation's rank counts its candidates
// across slot calls with the subject removed, and the first rank a path takes
// in a relation is kept (CRK-V0-004).
func TestContextOrderRecordRelationRanks(t *testing.T) {
	t.Run("CRK-V0-004", func(t *testing.T) {
		compiler := &taskContextCompiler{subject: "s.go"}
		compiler.rrfOrder = true
		compiler.relationRank, compiler.relationSeen = map[string]map[string]int{}, map[string]int{}
		compiler.recordRelationRanks([]contextRow{{kind: "test", path: "s.go"}, {kind: "test", path: "x.go"}})
		compiler.recordRelationRanks([]contextRow{{kind: "test", path: "y.go"}, {kind: "test", path: "x.go"}})
		if compiler.relationRank["x.go"]["test"] != 1 || compiler.relationRank["y.go"]["test"] != 2 {
			t.Fatalf("ranks = %#v", compiler.relationRank)
		}
		if _, ok := compiler.relationRank["s.go"]; ok {
			t.Fatal("the subject took a rank")
		}
		off := &taskContextCompiler{}
		off.recordRelationRanks([]contextRow{{kind: "test", path: "x.go"}})
		if off.relationRank != nil {
			t.Fatal("ranks recorded with RRF off")
		}
	})
}

// TestContextOrderAdditive: the additive key is the lexical band plus the
// slot bonus plus 50 per corroboration, reported as the score below the
// governing band; with both orderings on RRF governs (CRK-V0-005, -007).
func TestContextOrderAdditive(t *testing.T) {
	_, index := spanFixture(t)
	task := "Why does `RunEngine` return the wrong value?"
	t.Run("CRK-V0-005", func(t *testing.T) {
		t.Setenv("CORVINT_CONTEXT_ADDITIVE", "on")
		compiler := newTaskContextCompiler(index, task, "")
		rows := compiler.compile(20)
		band := compiler.lexicalBand()
		previous := 1 << 30
		for _, row := range rows {
			if instructionReservation(row.kind) {
				continue
			}
			key := band[row.path] + contextSlotBonus[row.kind] + 50*len(compiler.support[row.path])
			if !strings.Contains(row.reason, "; additive ") || row.score != min(contextLexicalBase+key, contextAdditiveMaxScore) {
				t.Fatalf("row %s: score %d key %d reason %q", row.path, row.score, key, row.reason)
			}
			if row.score > previous {
				t.Fatalf("additive rows out of order at %s", row.path)
			}
			previous = row.score
		}
	})
	t.Run("CRK-V0-007", func(t *testing.T) {
		t.Setenv("CORVINT_CONTEXT_ADDITIVE", "on")
		t.Setenv("CORVINT_CONTEXT_RRF", "on")
		compiler := newTaskContextCompiler(index, task, "")
		fused := false
		for _, row := range compiler.compile(20) {
			if strings.Contains(row.reason, "; additive ") {
				t.Fatalf("additive ran under RRF: %q", row.reason)
			}
			fused = fused || strings.Contains(row.reason, "; rrf ")
		}
		if !fused {
			t.Fatal("no row carries an RRF reason")
		}
	})
}

// TestContextOrderAcronymProse: under CORVINT_CONTEXT_ACRONYMS=prose an
// unquoted all-capital word is not a definition-slot name, a quoted one still
// is, and the rule is off by default (CRK-V0-006).
func TestContextOrderAcronymProse(t *testing.T) {
	t.Run("CRK-V0-006", func(t *testing.T) {
		// An empty index: definitionSlotEligible (TCP-V0-961) reads its
		// vocabulary, where no word is common.
		on := &taskContextCompiler{index: &Index{}, acronymProse: true}
		off := &taskContextCompiler{index: &Index{}}
		for _, test := range []struct {
			identifier taskIdentifier
			on, off    bool
		}{
			{taskIdentifier{"CEM", 1}, false, true},
			{taskIdentifier{"CEM", 3}, true, true},
			{taskIdentifier{"CEMBinding", 1}, true, true},
			{taskIdentifier{"A", 1}, false, false},
		} {
			if got := on.slotDefinitionEligible(test.identifier); got != test.on {
				t.Fatalf("prose %v = %v", test.identifier, got)
			}
			if got := off.slotDefinitionEligible(test.identifier); got != test.off {
				t.Fatalf("default %v = %v", test.identifier, got)
			}
		}
		for word, want := range map[string]bool{"CEM": true, "OK": true, "A": false, "CEMs": false, "C3": false, "": false} {
			if allCapitalWord(word) != want {
				t.Fatalf("allCapitalWord(%q) != %v", word, want)
			}
		}
	})
}
