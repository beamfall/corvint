package testdiscovery

import (
	"slices"
)

// RSpecItem is one child of an example group in RSpec's definition order: a nested group, an
// example, or an inclusion.
type RSpecItem struct {
	Group, Example, Include int
	line                    int
	Loop                    bool
}

// Children returns group g's children in line order. A shared group definition is not a child; an
// it_behaves_like group is its inclusion.
func (r *RSpecReader) Children(g int) []RSpecItem {
	nested := map[int]int{}
	for k, inc := range r.Includes {
		if inc.Nested >= 0 {
			nested[inc.Nested] = k
		}
	}
	out := []RSpecItem{}
	for k, group := range r.Groups {
		if k == 0 || group.Parent != g || SharedDefinitionWords[Declarator(group.Head)] {
			continue
		}
		item := RSpecItem{Group: k, Example: -1, Include: -1, line: group.Line, Loop: group.Loop}
		if inc, ok := nested[k]; ok {
			item.Group, item.Include = -1, inc
		}
		out = append(out, item)
	}
	for k, e := range r.Examples {
		if e.Group == g {
			out = append(out, RSpecItem{Group: -1, Example: k, Include: -1, line: e.Line, Loop: e.Loop})
		}
	}
	for k, inc := range r.Includes {
		if inc.Group == g && inc.Nested < 0 {
			// it_behaves_like without a block still makes a nested group; the others expand inline.
			out = append(out, RSpecItem{Group: -1, Example: -1, Include: k, line: inc.Line, Loop: inc.Loop})
		}
	}
	slices.SortFunc(out, func(a, b RSpecItem) int { return a.line - b.line })
	return out
}

// Nests reports whether the inclusion makes a nested example group (it_behaves_like and
// it_should_behave_like) rather than expanding inline (include_examples and include_context).
func (inc RSpecInclude) Nests() bool {
	return inc.Kind == "it_behaves_like" || inc.Kind == "it_should_behave_like"
}
