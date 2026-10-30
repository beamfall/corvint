package mutate

import (
	"bytes"
	"go/parser"
	"go/token"
	"testing"
)

const planFixture = `package sample

import "fmt"

type Config struct{ Name string }

func Classify(n int) string {
	if n > 0 && n < 10 {
		return "small"
	}
	total := n + 1
	fmt.Println(total)
	for total < 100 {
		total = total - 1
	}
	return ""
}
`

// TestPlanMutationsListsOperatorsInSourceOrder pins the closed operator set and
// the order mutants are generated in, which is the falsifier's determinism.
func TestPlanMutationsListsOperatorsInSourceOrder(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "sample.go", planFixture, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	type pair struct {
		operator string
		line     int
	}
	expected := []pair{
		{operatorNegateCondition, 8},
		{operatorSwapBinary, 8},
		{operatorSwapBinary, 8},
		{operatorSwapBinary, 8},
		{operatorReplaceLiteral, 9},
		{operatorSwapBinary, 11},
		{operatorDeleteStatement, 12},
		{operatorNegateCondition, 13},
		{operatorSwapBinary, 13},
		{operatorDeleteStatement, 14},
		{operatorSwapBinary, 14},
		{operatorReplaceLiteral, 16},
	}
	plan := planMutations(fset, file)
	if len(plan) != len(expected) {
		for index, candidate := range plan {
			t.Logf("site %d: %s line %d", index, candidate.Operator, candidate.Line)
		}
		t.Fatalf("planned %d sites, want %d", len(plan), len(expected))
	}
	for index, want := range expected {
		got := pair{plan[index].Operator, plan[index].Line}
		if got != want {
			t.Errorf("site %d = %+v, want %+v", index, got, want)
		}
	}
}

// TestGenerateMutantsIsDeterministic proves two runs over one source produce
// byte-identical mutants in the same order.
func TestGenerateMutantsIsDeterministic(t *testing.T) {
	first, err := generateMutants([]byte(planFixture), 32, nil)
	if err != nil {
		t.Fatalf("first generation: %v", err)
	}
	second, err := generateMutants([]byte(planFixture), 32, nil)
	if err != nil {
		t.Fatalf("second generation: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("no mutants generated for the fixture")
	}
	if len(first) != len(second) {
		t.Fatalf("generated %d then %d mutants", len(first), len(second))
	}
	for index := range first {
		if first[index].Operator != second[index].Operator || first[index].Line != second[index].Line {
			t.Fatalf("mutant %d differs: %+v vs %+v", index, first[index], second[index])
		}
		if !bytes.Equal(first[index].Source, second[index].Source) {
			t.Fatalf("mutant %d source differs", index)
		}
	}
}

// TestGenerateMutantsHonoursTheCap keeps the run inside the caller's budget.
func TestGenerateMutantsHonoursTheCap(t *testing.T) {
	mutants, err := generateMutants([]byte(planFixture), 3, nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(mutants) != 3 {
		t.Fatalf("generated %d mutants, want 3", len(mutants))
	}
}

// TestGenerateMutantsSkipsFilesWithoutFunctionBodies is the NO_MUTANTS input.
func TestGenerateMutantsSkipsFilesWithoutFunctionBodies(t *testing.T) {
	mutants, err := generateMutants([]byte("package sample\n\ntype Config struct{ Name string }\n"), 8, nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(mutants) != 0 {
		t.Fatalf("generated %d mutants, want 0", len(mutants))
	}
}

// TestGenerateMutantsConfinesToTheGivenLines: a claim about a change is judged
// on the change, so sites outside the spans are never rendered.
func TestGenerateMutantsConfinesToTheGivenLines(t *testing.T) {
	inside, err := generateMutants([]byte(planFixture), 32, []LineSpan{{Start: 11, End: 12}})
	if err != nil {
		t.Fatal(err)
	}
	if len(inside) != 2 {
		t.Fatalf("generated %d mutants inside lines 11-12, want 2", len(inside))
	}
	for _, candidate := range inside {
		if candidate.Line < 11 || candidate.Line > 12 {
			t.Fatalf("mutant on line %d is outside the span", candidate.Line)
		}
	}
	if none, _ := generateMutants([]byte(planFixture), 32, []LineSpan{{Start: 1, End: 5}}); len(none) != 0 {
		t.Fatalf("generated %d mutants in a span without sites", len(none))
	}
}
