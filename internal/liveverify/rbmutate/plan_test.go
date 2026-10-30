package rbmutate

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
)

func describeSites(sites []site) []string {
	described := make([]string, 0, len(sites))
	for _, candidate := range sites {
		described = append(described, strconv.Itoa(candidate.Line)+" "+candidate.Operator+" "+candidate.Original+" -> "+candidate.Replacement)
	}
	return described
}

// TestFPKV0068PlannerAppliesTheBoundedOperatorSet pins FPK-V0-068: the
// planner mutates relational boundaries, equality, arithmetic, integer
// constants by one, and conditions, and nothing else.
func TestFPKV0068PlannerAppliesTheBoundedOperatorSet(t *testing.T) {
	source := "def band(count, total)\n" +
		"  if count >= 8\n" +
		"    total / count + 1\n" +
		"  elsif count == 0 then 0\n" +
		"  end\n" +
		"end\n"
	sites, ok := planMutations([]byte(source), nil)
	if !ok {
		t.Fatal("fixture does not lex")
	}
	want := []string{
		"2 negate-condition count >= 8 -> !(count >= 8)",
		"2 relational-boundary >= -> >",
		"2 constant-decrement 8 -> 7",
		"2 constant-increment 8 -> 9",
		"3 arithmetic-swap / -> *",
		"3 arithmetic-swap + -> -",
		"3 constant-decrement 1 -> 0",
		"3 constant-increment 1 -> 2",
		"4 negate-condition count == 0 -> !(count == 0)",
		"4 relational-negation == -> !=",
		"4 constant-increment 0 -> 1",
		"4 constant-increment 0 -> 1",
	}
	if got := describeSites(sites); !reflect.DeepEqual(got, want) {
		t.Fatalf("sites:\n got %q\nwant %q", got, want)
	}
	for _, candidate := range sites {
		if rendered := string(render([]byte(source), candidate)); rendered == source {
			t.Fatalf("%v renders the original", candidate)
		}
	}
}

// TestFPKV0068PlannerLeavesNonCodeAndAmbiguousSitesAlone pins FPK-V0-068's
// exclusions: strings, comments, regexes, heredocs, symbols, floats and
// radix literals, superclass `<`, spaceship and power operators, unary signs
// and splats, method-call `if`, hash keys, and conditions that continue on
// the next line.
func TestFPKV0068PlannerLeavesNonCodeAndAmbiguousSitesAlone(t *testing.T) {
	source := "class Widget < Base\n" +
		"  LABEL = \"a < b + 1\" # 2 > 1\n" +
		"  PATTERN = /x+ < 3/\n" +
		"  RATE = 1.5\n" +
		"  MASK = 0x1F\n" +
		"  TEXT = <<~DOC\n" +
		"    9 - 9 == 9\n" +
		"  DOC\n" +
		"  def call(*args, **opts)\n" +
		"    puts -value\n" +
		"    compare a <=> b, a ** b, :sym\n" +
		"    run(if: ready?)\n" +
		"    obj.if\n" +
		"    if ready? &&\n" +
		"      go\n" +
		"    end\n" +
		"  end\n" +
		"end\n"
	sites, ok := planMutations([]byte(source), nil)
	if !ok {
		t.Fatal("fixture does not lex")
	}
	if len(sites) != 0 {
		t.Fatalf("unexpected sites: %q", describeSites(sites))
	}
}

// TestFPKV0068PlannerConfinesSitesToChangedLines pins FPK-V0-068's
// confinement: only lines inside a span are mutated, and a source that does
// not lex has no plan.
func TestFPKV0068PlannerConfinesSitesToChangedLines(t *testing.T) {
	source := "a = 1\nb = 2\nc = 3\n"
	sites, ok := planMutations([]byte(source), []mutate.LineSpan{{Start: 2, End: 2}})
	if !ok {
		t.Fatal("fixture does not lex")
	}
	want := []string{"2 constant-decrement 2 -> 1", "2 constant-increment 2 -> 3"}
	if got := describeSites(sites); !reflect.DeepEqual(got, want) {
		t.Fatalf("sites: got %q, want %q", got, want)
	}
	if _, ok := planMutations([]byte("x = \"unterminated\n"), nil); ok {
		t.Fatal("an unterminated literal lexed")
	}
}

// TestFPKV0068PlannerKeepsLiteralsCommentsAndExponentsWhole pins three
// FPK-V0-068 boundaries: a `#` inside a string is not a comment, a condition
// that a bare `end` closes on its own line is never negated, and a float's
// exponent is part of the float.
func TestFPKV0068PlannerKeepsLiteralsCommentsAndExponentsWhole(t *testing.T) {
	source := "if value == \"a # b\" then 1 end\n" +
		"if label == \"a\" # note\n" +
		"end\n" +
		"def f; true if ready? end\n" +
		"x = 1e-3\n" +
		"y = 2.5E+4\n" +
		"z = 0x1e - 3\n"
	sites, ok := planMutations([]byte(source), nil)
	if !ok {
		t.Fatal("fixture does not lex")
	}
	want := []string{
		"1 negate-condition value == \"a # b\" -> !(value == \"a # b\")",
		"1 relational-negation == -> !=",
		"1 constant-decrement 1 -> 0",
		"1 constant-increment 1 -> 2",
		"2 negate-condition label == \"a\" -> !(label == \"a\")",
		"2 relational-negation == -> !=",
		"7 arithmetic-swap - -> +",
		"7 constant-decrement 3 -> 2",
		"7 constant-increment 3 -> 4",
	}
	if got := describeSites(sites); !reflect.DeepEqual(got, want) {
		t.Fatalf("sites:\n got %q\nwant %q", got, want)
	}
}
