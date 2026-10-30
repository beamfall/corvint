package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

// inheritedCells is the packet's own copy of the repository-envelope cells,
// whose full expectedResult objects the portable cell test compares exactly.
// The stable copy beside the packet carries the same cells plus the amendment
// index.
const inheritedCells = "inherited/s0e/" + cellsName

// cellProblems checks that both cell copies list the same cells and that every
// cell expectedResult carries exactly the required keys in order.
func cellProblems(t *tree, required []string) []string {
	var problems []string
	var lists [2]any
	for i, rel := range []string{packetDir + "/" + inheritedCells, stableDir + "/" + cellsName} {
		d, err := t.parse(rel)
		if err != nil {
			return append(problems, err.Error())
		}
		cells := d.root.get("cases")
		if cells == nil || cells.kind != '[' || len(cells.elems) == 0 {
			problems = append(problems, rel+" has no cells")
			continue
		}
		for _, c := range cells.elems {
			if keys := c.get("expectedResult"); keys == nil || !reflect.DeepEqual(keys.keys, required) {
				problems = append(problems, fmt.Sprintf("%s cell %s expectedResult keys differ from %s required", rel, c.get("id").text(d.src), schemaName))
			}
		}
		lists[i], _ = cells.decode(d.src)
	}
	if lists[0] != nil && lists[1] != nil && !reflect.DeepEqual(lists[0], lists[1]) {
		problems = append(problems, fmt.Sprintf("%s/%s and %s/%s list different cells", packetDir, inheritedCells, stableDir, cellsName))
	}
	return problems
}

// cellOutcome records where each cell's member value came from.
type cellOutcome struct {
	derived    int
	overridden []string
	overrides  *document
}

// frozenKey identifies a verdict by its exit and every frozen key, without
// the regenerated members.
func frozenKey(src []byte, c *jnode, members ...string) (string, error) {
	result, err := c.get("expectedResult").decode(src)
	object, ok := result.(map[string]any)
	if err != nil || !ok {
		return "", errors.New("expectedResult is not an object")
	}
	for _, member := range members {
		delete(object, member)
	}
	exit, err := strconv.Atoi(string(c.get("expectedExit").raw(src)))
	if err != nil {
		return "", errors.New("expectedExit is not an integer")
	}
	key, err := json.Marshal([]any{exit, object})
	return string(key), err
}

// cellSplices inserts member into every cell expectedResult of the packet's
// inherited cells and returns the splices for each copy. A cell takes the value
// of the regenerated cases whose exit and frozen keys equal its own, since those
// keys bind the map, base, target, patch, drift and hunks that the value is
// computed from; all such cases must agree. A cell no case matches needs an
// explicit --cell-values override, and a matched cell may not have one.
func cellSplices(t *tree, o options, casesNew []byte) (packet, stable []splice, outcome cellOutcome, err error) {
	cases, err := parseJSON(casesNew)
	if err != nil {
		return nil, nil, outcome, err
	}
	type source struct {
		src  []byte
		node *jnode
	}
	byVerdict := map[string][]source{}
	for _, c := range cases.get("cases").elems {
		key, err := frozenKey(casesNew, c, o.member)
		if err != nil {
			return nil, nil, outcome, fmt.Errorf("%s case %s: %w", casesName, c.get("id").text(casesNew), err)
		}
		byVerdict[key] = append(byVerdict[key], source{casesNew, c.get("expectedResult").get(o.member)})
	}
	if o.cellValues != "" {
		if outcome.overrides, err = t.parseFile(o.cellValues); err != nil {
			return nil, nil, outcome, err
		}
		if outcome.overrides.root.kind != '{' {
			return nil, nil, outcome, errors.New("--cell-values must be a JSON object of cell id to value")
		}
	}
	values := map[string]source{}
	inherited, err := t.parse(packetDir + "/" + inheritedCells)
	if err != nil {
		return nil, nil, outcome, err
	}
	for _, c := range inherited.root.get("cases").elems {
		id := c.get("id").text(inherited.src)
		key, err := frozenKey(inherited.src, c, o.member)
		if err != nil {
			return nil, nil, outcome, fmt.Errorf("cell %s: %w", id, err)
		}
		matches := byVerdict[key]
		override := outcome.overrides.lookup(id)
		switch {
		case len(matches) > 0 && override != nil:
			return nil, nil, outcome, fmt.Errorf("cell %s matches a case's frozen verdict and also has a --cell-values override", id)
		case len(matches) > 0:
			first, _ := matches[0].node.decode(matches[0].src)
			for _, m := range matches[1:] {
				if other, _ := m.node.decode(m.src); !reflect.DeepEqual(first, other) {
					return nil, nil, outcome, fmt.Errorf("cell %s: cases with its frozen verdict disagree on %q", id, o.member)
				}
			}
			values[id] = matches[0]
			outcome.derived++
		case override != nil:
			values[id] = source{outcome.overrides.src, override}
			outcome.overridden = append(outcome.overridden, id)
		default:
			return nil, nil, outcome, fmt.Errorf("cell %s matches no case's frozen verdict and has no --cell-values override for %q", id, o.member)
		}
	}
	if outcome.overrides != nil {
		for _, id := range outcome.overrides.root.keys {
			if _, ok := values[id]; !ok {
				return nil, nil, outcome, fmt.Errorf("--cell-values names unknown cell %s", id)
			}
		}
	}
	insert := func(d *document) ([]splice, error) {
		var edits []splice
		for _, c := range d.root.get("cases").elems {
			id := c.get("id").text(d.src)
			v, ok := values[id]
			if !ok {
				return nil, fmt.Errorf("%s cell %s is not in %s", d.name, id, inheritedCells)
			}
			expected := c.get("expectedResult")
			text := render(v.src, v.node, lineIndent(d.src, expected.kspan[0][0]))
			edit, err := insertMember(d.src, expected, o.after, o.member, func(string) string { return text })
			if err != nil {
				return nil, fmt.Errorf("%s cell %s: %w", d.name, id, err)
			}
			edits = append(edits, edit)
		}
		return edits, nil
	}
	if packet, err = insert(inherited); err != nil {
		return nil, nil, outcome, err
	}
	copyDoc, err := t.parse(stableDir + "/" + cellsName)
	if err != nil {
		return nil, nil, outcome, err
	}
	if stable, err = insert(copyDoc); err != nil {
		return nil, nil, outcome, err
	}
	return packet, stable, outcome, nil
}
