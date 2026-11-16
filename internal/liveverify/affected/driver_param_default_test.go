package affected_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestDriverRecipeParameterDefaultIsRead_V1_5628: the word of a
// value-supplying parameter expansion in a make recipe (`$${F:-data/x.txt}`,
// make's spelling of the shell's `${F:-data/x.txt}`) is a path the recipe
// reads when the parameter is unset, so the target's driver is a reader of
// it. Before V1-5628 the driver tokenizer skipped the whole braced expansion
// and the driver was not selected for an edit of the default: an
// under-selection.
func TestDriverRecipeParameterDefaultIsRead_V1_5628(t *testing.T) {
	const target = "driver:make:Makefile#check"
	for _, c := range []struct {
		name, recipe, changed string
	}{
		{"colon-dash", "cat $${F:-data/x.txt}", "data/x.txt"},
		{"dash", "cat $${F-data/x.txt}", "data/x.txt"},
		{"colon-equals", "cat $${F:=data/x.txt}", "data/x.txt"},
		{"equals", "cat $${F=data/x.txt}", "data/x.txt"},
		{"colon-plus", "cat $${F:+data/x.txt}", "data/x.txt"},
		{"plus", "cat $${F+data/x.txt}", "data/x.txt"},
		{"quoted", "cat \"$${F:-data/x.txt}\"", "data/x.txt"},
		{"nested", "cat $${F:-$${G:-data/y.txt}}", "data/y.txt"},
		{"nested-outer-suffix", "cat $${F:-$${ROOT:-.}/data/y.txt}", "data/y.txt"},
		{"nested-sibling", "cat $${F:-$${G:-data/y.txt}} $${H:-data/x.txt}", "data/x.txt"},
		{"positional", "sh -c 'cat $${1:-data/x.txt}' sh", "data/x.txt"},
		{"unclosed", "cat $${F:-data/x.txt", "data/x.txt"},
	} {
		t.Run(c.name, func(t *testing.T) {
			graph := driverGraph(t, map[string]string{
				"Makefile":   "check:\n\t" + c.recipe + "\n\nother:\n\techo other\n",
				"data/x.txt": "x\n",
				"data/y.txt": "y\n",
			}, nil)
			plan := affected.Select(graph, []string{c.changed})
			selected := driverSelections(plan)
			if _, hit := selected[target]; hit {
				return
			}
			t.Errorf("recipe %q: %s does not select %s; scope %s, selected %v, unknown %v",
				c.recipe, c.changed, target, plan.Scope, slices.Sorted(maps.Keys(selected)), plan.Unknown)
		})
	}
}
