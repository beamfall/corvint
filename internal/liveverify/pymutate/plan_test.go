package pymutate

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
)

const planSource = `import os

LIMIT = 10


def total(items, scale=-1):
    count = 0
    for item in items:
        count += item * scale
    if count > LIMIT and count != 0:
        return "big"
    items.clear()
    self.cache[0] = count
    print(*items)
    return 0


def sign(n):
    while n <= 0:
        n = n + 1
    return -n
`

func operatorsAndLines(sites []site) string {
	parts := make([]string, 0, len(sites))
	for _, candidate := range sites {
		parts = append(parts, candidate.Operator+"@"+itoa(candidate.Line))
	}
	return strings.Join(parts, " ")
}

func itoa(value int) string { return fmt.Sprintf("%02d", value) }

// TestPlanMutationsListsOperatorsInSourceOrder pins the operator table: a
// unary minus, a star argument, a bare-name binding, a definition, and a
// return are never touched; everything else is, once, in source order.
func TestPlanMutationsListsOperatorsInSourceOrder(t *testing.T) {
	sites, err := planMutations([]byte(planSource))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"delete-statement@09", // count += item * scale (augmented: the name stays bound)
		"swap-binary@09",      // item * scale
		"negate-condition@10", // if count > LIMIT and count != 0
		"swap-binary@10",      // >
		"swap-binary@10",      // and
		"swap-binary@10",      // !=
		"replace-literal@11",  // "big"
		"delete-statement@12", // items.clear()
		"delete-statement@13", // self.cache[0] = count
		"delete-statement@14", // print(*items)
		"replace-literal@15",  // return 0
		"negate-condition@19", // while n <= 0
		"swap-binary@19",      // <=
		"swap-binary@20",      // n + 1
	}, " ")
	if got := operatorsAndLines(sites); got != want {
		t.Fatalf("plan:\n got %s\nwant %s", got, want)
	}
}

func TestRenderedMutantsAreSingleEdits(t *testing.T) {
	mutants, err := generateMutants([]byte(planSource), nil)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{
		"negate-condition@10": "    if not (count > LIMIT and count != 0):\n",
		"swap-binary@20":      "        n = n - 1\n",
		"replace-literal@11":  "        return \"\"\n",
		"replace-literal@15":  "    return 1\n",
	}
	for _, candidate := range mutants {
		want, checked := expected[candidate.Operator+"@"+itoa(candidate.Line)]
		if !checked {
			continue
		}
		if !bytes.Contains(candidate.Source, []byte(want)) {
			t.Errorf("%s@%d: mutant lacks %q", candidate.Operator, candidate.Line, want)
		}
		if bytes.Count(candidate.Source, []byte("\n")) != bytes.Count([]byte(planSource), []byte("\n"))+lineDelta(candidate.Operator) {
			t.Errorf("%s@%d: line count changed unexpectedly", candidate.Operator, candidate.Line)
		}
	}
	deleted := 0
	for _, candidate := range mutants {
		if candidate.Operator == operatorDeleteStatement && !bytes.Contains(candidate.Source, []byte("items.clear()")) {
			deleted++
		}
	}
	if deleted != 1 {
		t.Errorf("items.clear() deleted by %d mutants, want 1", deleted)
	}
}

func lineDelta(operator string) int {
	if operator == operatorDeleteStatement {
		return -1
	}
	return 0
}

// TestPlanNeverDeletesADefinition: a bare-name binding, a def, a return, an
// import, a block opener, and a statement sharing its line are never deleted.
func TestPlanNeverDeletesADefinition(t *testing.T) {
	source := "import os\nx = 1\ndef f():\n    y = 2; z = 3\n    a, b = 1, 2\n    w: int = 4\n    return y\nclass C:\n    pass\n"
	sites, err := planMutations([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range sites {
		if candidate.Operator == operatorDeleteStatement {
			t.Errorf("deletion planned at line %d", candidate.Line)
		}
	}
}

func TestGenerateMutantsConfinesToTheGivenLines(t *testing.T) {
	all, err := generateMutants([]byte(planSource), nil)
	if err != nil {
		t.Fatal(err)
	}
	confined, err := generateMutants([]byte(planSource), []mutate.LineSpan{{Start: 19, End: 20}})
	if err != nil {
		t.Fatal(err)
	}
	if len(confined) != 3 || len(all) <= len(confined) {
		t.Fatalf("confined %d of %d mutants", len(confined), len(all))
	}
	for _, candidate := range confined {
		if candidate.Line < 19 || candidate.Line > 20 {
			t.Errorf("mutant at line %d outside the span", candidate.Line)
		}
	}
	if none, _ := generateMutants([]byte(planSource), []mutate.LineSpan{{Start: 1, End: 3}}); len(none) != 0 {
		t.Errorf("lines 1-3 yielded %d mutants", len(none))
	}
	if _, err := generateMutants([]byte("x = 'unterminated\n"), nil); err == nil {
		t.Error("an untokenizable file must be an error")
	}
}
