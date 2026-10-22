package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
)

const (
	replaceStatus = "MEMBER_VALUE_REEMITTED"
	// replacePointer marks the one README paragraph the first recorded
	// re-emission adds.
	replacePointer = "Recorded value re-emissions:"
)

// record is the --record file: the reason for a re-emission and, for every
// value that changes, the sha256 of the compact JSON of the value it replaces.
type record struct {
	Reason string                       `json:"reason"`
	Cases  map[string]map[string]string `json:"cases"`
	Cells  map[string]map[string]string `json:"cells"`
}

func loadRecord(file string) (*record, []byte, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, err
	}
	if _, err := parseJSON(src); err != nil {
		return nil, nil, fmt.Errorf("--record: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(src))
	dec.DisallowUnknownFields()
	var r record
	if err := dec.Decode(&r); err != nil {
		return nil, nil, fmt.Errorf("--record: %w", err)
	}
	if strings.TrimSpace(r.Reason) == "" {
		return nil, nil, errors.New("--record has no reason")
	}
	return &r, src, nil
}

// valueDigest is the sha256 of a value's compact JSON, independent of its
// indentation in the file that carries it.
func valueDigest(raw []byte) string {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return digest(raw)
	}
	return digest(b.Bytes())
}

// change is one re-emitted value.
type change struct {
	kind, id, member, old, new string
}

// reemitter applies new member values and refuses any value change that the
// record does not name with the digest of the value it replaces.
type reemitter struct {
	rec     *record
	used    map[string]bool
	changes []change
}

func (r *reemitter) value(kind, id, member string, oldSrc []byte, oldNode *jnode, newSrc []byte, newNode *jnode, indent string) (*splice, error) {
	a, errA := oldNode.decode(oldSrc)
	b, errB := newNode.decode(newSrc)
	if errA != nil || errB != nil {
		return nil, fmt.Errorf("%s %s member %q is not valid JSON", kind, id, member)
	}
	recorded := r.rec.Cases
	if kind == "cell" {
		recorded = r.rec.Cells
	}
	want, named := recorded[id][member]
	if reflect.DeepEqual(a, b) {
		if named {
			return nil, fmt.Errorf("%s %s member %q is recorded but does not change", kind, id, member)
		}
		return nil, nil
	}
	if !named {
		return nil, fmt.Errorf("%s %s member %q changes but the record does not name it", kind, id, member)
	}
	old := valueDigest(oldNode.raw(oldSrc))
	if want != old {
		return nil, fmt.Errorf("%s %s member %q: the record names predecessor %s, the packet holds %s", kind, id, member, want, old)
	}
	r.used[kind+"\x00"+id+"\x00"+member] = true
	text := render(newSrc, newNode, indent)
	r.changes = append(r.changes, change{kind, id, member, old, valueDigest([]byte(text))})
	s := replaceValue(oldNode, text)
	return &s, nil
}

// reemit replaces the values of existing result members in place (CEM-V1-091
// as amended by decision 0530): every other key, every exit and the schema stay
// identical, and each changed value is recorded with its predecessor digest.
func reemit(t *tree, o options) (string, error) {
	if problems := check(t); len(problems) > 0 {
		return "", fmt.Errorf("the packet is not self-consistent before re-emission: %s", strings.Join(problems, "; "))
	}
	rec, recSrc, err := loadRecord(o.record)
	if err != nil {
		return "", err
	}
	schema, _ := t.parse(stableDir + "/" + schemaName)
	required := map[string]bool{}
	for _, name := range stringList(schema, schema.root.get("required")) {
		required[name] = true
	}
	members := map[string]bool{}
	for _, m := range strings.Split(o.member, ",") {
		if !required[m] {
			return "", fmt.Errorf("--member %q is not an existing result member", m)
		}
		members[m] = true
	}
	memberList := sortedKeys(members)
	r := &reemitter{rec: rec, used: map[string]bool{}}

	// 1. FULL-RESULT-CASES.json.
	capture, err := loadCapture(o.capture)
	if err != nil {
		return "", err
	}
	values, err := memberOverrides(t, o.values, "--values", members)
	if err != nil {
		return "", err
	}
	substitutions, err := tupleSubstitutions(t)
	if err != nil {
		return "", err
	}
	cases, _ := t.parse(packetDir + "/" + casesName)
	list := cases.root.get("cases")
	var edits []splice
	known := map[string]bool{}
	tuplesSeen := map[string]int{}
	var overridden []string
	for _, c := range list.elems {
		id := c.get("id").text(cases.src)
		known[id] = true
		expected := c.get("expectedResult")
		capturedCase, hasCapture := capture[id]
		override := values.lookup(id)
		var src []byte
		var source *jnode
		switch {
		case hasCapture && override != nil:
			return "", fmt.Errorf("case %s has both a captured value and a --values override", id)
		case hasCapture:
			if err := compareCapture(cases, c, capturedCase, members, substitutions); err != nil {
				return "", fmt.Errorf("case %s: %w", id, err)
			}
			src, source = capturedCase.src, capturedCase.result
			if members["runtime"] {
				if src, source, err = frozenGoVersion(cases, expected, capturedCase, substitutions); err != nil {
					return "", fmt.Errorf("case %s: %w", id, err)
				}
			}
			tuplesSeen[capturedCase.tuple]++
		case override != nil:
			src, source = values.src, override
			overridden = append(overridden, id)
		default:
			return "", fmt.Errorf("case %s has no captured value and no --values override", id)
		}
		for _, m := range memberList {
			node := source.get(m)
			if node == nil {
				continue
			}
			i := expected.index(m)
			edit, err := r.value("case", id, m, cases.src, expected.elems[i], src, node, lineIndent(cases.src, expected.kspan[i][0]))
			if err != nil {
				return "", err
			}
			if edit != nil {
				edits = append(edits, *edit)
			}
		}
	}
	for id := range capture {
		if !known[id] {
			return "", fmt.Errorf("capture names unknown case %s", id)
		}
	}
	if values != nil {
		for _, id := range values.root.keys {
			if !known[id] {
				return "", fmt.Errorf("--values names unknown case %s", id)
			}
		}
	}
	// A re-emission that changes no case value (only cells no case matches)
	// leaves the case file and its predecessorSHA256 byte-identical (decision
	// 0535).
	casesOld, casesNew, casesChanged := cases.src, cases.src, len(edits) > 0
	if casesChanged {
		predecessor := cases.root.get("predecessorSHA256")
		if predecessor == nil {
			return "", fmt.Errorf("%s has no predecessorSHA256", casesName)
		}
		edits = append(edits, replaceValue(predecessor, quote(digest(casesOld))))
		if casesNew, err = applySplices(cases.src, edits); err != nil {
			return "", err
		}
		t.set(packetDir+"/"+casesName, casesNew)
	}

	// 2. Both envelope cell copies.
	cellEdits, stableCellEdits, cellsOutcome, err := reemitCells(t, o, r, members, casesNew)
	if err != nil {
		return "", err
	}
	if len(r.changes) == 0 {
		return "", errors.New("no value changes; nothing to re-emit")
	}
	for kind, entries := range map[string]map[string]map[string]string{"case": rec.Cases, "cell": rec.Cells} {
		for id, byMember := range entries {
			for m := range byMember {
				if !r.used[kind+"\x00"+id+"\x00"+m] {
					return "", fmt.Errorf("the record names %s %s member %q, which is not re-emitted", kind, id, m)
				}
			}
		}
	}
	inheritedOld, _ := t.read(packetDir + "/" + inheritedCells)
	inheritedNew, err := applySplices(inheritedOld, cellEdits)
	if err != nil {
		return "", err
	}
	t.set(packetDir+"/"+inheritedCells, inheritedNew)

	// 3. README pointer, added once.
	readmeOld, err := t.read(packetDir + "/" + readmeName)
	if err != nil {
		return "", err
	}
	packetChanged := map[string][]byte{inheritedCells: inheritedNew}
	if casesChanged {
		packetChanged[casesName] = casesNew
	}
	if !bytes.Contains(readmeOld, []byte(replacePointer)) {
		readmeNew := append(append([]byte(nil), readmeOld...), []byte("\n"+replacePointer+" a revision may also re-emit the value of an existing\nmember with `make cem-stable-regen-replace` (CEM-V1-091, decision 0530), which\nkeeps every other key, exit and case, records the predecessor digest of each\nreplaced value and the reason, and appends a section to CHANGES.md.\n")...)
		t.set(packetDir+"/"+readmeName, readmeNew)
		packetChanged[readmeName] = readmeNew
	}

	// 4. CHANGES.md, the packet manifest, the stable bindings and every tuple.
	packetManifestOld, _ := t.read(packetDir + "/manifest.json")
	oldSum, newSum, err := sealChain(t, packetManifestOld, packetChanged, casesOld, casesNew, stableCellEdits, replaceStatus, func(changesOld []byte, outside map[string][]byte) []byte {
		var b strings.Builder
		fmt.Fprintf(&b, "\n# Recorded value re-emission of `%s` (%s)\n\n", strings.Join(memberList, "`, `"), o.date)
		fmt.Fprintf(&b, "Base: the public packet with manifest sha256\n%s.\n", digest(packetManifestOld))
		fmt.Fprintf(&b, "Reason: %s\n", rec.Reason)
		fmt.Fprintf(&b, "Record file sha256: %s.\n", digest(recSrc))
		b.WriteString("Produced by `make cem-stable-regen-replace` (tools/cem-stable-regen, CEM-V1-091\nas amended by decision 0530).\n\n")
		fmt.Fprintf(&b, "- Members re-emitted in place: `%s`. No key is added or removed and\n  RESULT.schema.json is unchanged.\n", strings.Join(memberList, "`, `"))
		for _, name := range sortedKeys(tuplesSeen) {
			fmt.Fprintf(&b, "- Values captured from the native verifier on %s for %d cases.\n", name, tuplesSeen[name])
		}
		if len(overridden) > 0 {
			fmt.Fprintf(&b, "- Values from the override file (sha256 %s) for: %s.\n", digest(values.src), strings.Join(overridden, ", "))
		}
		fmt.Fprintf(&b, "- Envelope cells (%s and its ../%s copy):\n  %d cells take the values of the cases with the same exit and other keys.\n", inheritedCells, cellsName, cellsOutcome.derived)
		if len(cellsOutcome.overridden) > 0 {
			fmt.Fprintf(&b, "- Cell values from the cell override file (sha256 %s) for: %s.\n", digest(cellsOutcome.overrides.src), strings.Join(cellsOutcome.overridden, ", "))
		}
		fmt.Fprintf(&b, "- Every other key and expectedExit is unchanged; every captured result\n  matched its frozen expectation on them (runtime.goVersion only through a\n  sealed structural-tuple substitution). The case count stays %d.\n\n", len(list.elems))
		b.WriteString("## Re-emitted values\n\nEach digest is the sha256 of the value's compact JSON.\n\n| Kind | Id | Member | Predecessor sha256 | New sha256 |\n| --- | --- | --- | --- | --- |\n")
		for _, c := range r.changes {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", c.kind, c.id, c.member, c.old, c.new)
		}
		b.WriteString("\n")
		writeFileTables(&b, packetManifestOld, outside, map[string][2][]byte{
			casesName:      {casesOld, packetChanged[casesName]},
			inheritedCells: {inheritedOld, inheritedNew},
			readmeName:     {readmeOld, packetChanged[readmeName]},
			changesName:    {changesOld, nil},
		})
		return []byte(b.String())
	})
	if err != nil {
		return "", err
	}
	if problems := check(t); len(problems) > 0 {
		return "", fmt.Errorf("re-emitted packet fails its own check: %s", strings.Join(problems, "; "))
	}
	after, _ := t.parse(packetDir + "/" + casesName)
	if n := len(after.root.get("cases").elems); n != len(list.elems) {
		return "", fmt.Errorf("case count changed from %d to %d", len(list.elems), n)
	}
	var report strings.Builder
	fmt.Fprintf(&report, "cem-stable-regen: re-emitted %d recorded values of %s\n", len(r.changes), strings.Join(memberList, ", "))
	fmt.Fprintf(&report, "packet manifest %s -> %s\n", oldSum, newSum)
	for _, name := range sortedKeys(t.next) {
		fmt.Fprintln(&report, "wrote "+name)
	}
	return report.String(), nil
}

// frozenGoVersion maps a captured runtime.goVersion back through its admitted
// tuple substitution, so a re-emitted runtime member keeps the frozen tuple
// that every substitution starts from (decision 0551). Any other goVersion
// change is refused.
func frozenGoVersion(cases *document, expected *jnode, got *captured, substitutions map[[2]string]bool) ([]byte, *jnode, error) {
	from := expected.get("runtime").get("goVersion").text(cases.src)
	version := got.result.get("runtime").get("goVersion")
	to := version.text(got.src)
	if from == to {
		return got.src, got.result, nil
	}
	if version == nil || !substitutions[[2]string{from, to}] {
		return nil, nil, fmt.Errorf("captured runtime.goVersion %q is not an admitted substitution of the frozen %q", to, from)
	}
	src, err := applySplices(got.src, []splice{replaceValue(version, quote(from))})
	if err != nil {
		return nil, nil, err
	}
	root, err := parseJSON(src)
	if err != nil {
		return nil, nil, err
	}
	return src, root.get("result"), nil
}

// reemitCells re-emits members in both envelope cell copies. A cell takes the
// values of the re-emitted cases whose exit and other keys equal its own when
// all such cases agree. A cell no case matches, or whose matching cases
// disagree, needs a --cell-values entry (an empty object keeps every value);
// a cell its agreeing cases determine may not have one.
func reemitCells(t *tree, o options, r *reemitter, members map[string]bool, casesNew []byte) (packet, stable []splice, outcome cellOutcome, err error) {
	memberList := sortedKeys(members)
	cases, err := parseJSON(casesNew)
	if err != nil {
		return nil, nil, outcome, err
	}
	byVerdict := map[string][]*jnode{}
	for _, c := range cases.get("cases").elems {
		key, err := frozenKey(casesNew, c, memberList...)
		if err != nil {
			return nil, nil, outcome, fmt.Errorf("%s case %s: %w", casesName, c.get("id").text(casesNew), err)
		}
		byVerdict[key] = append(byVerdict[key], c.get("expectedResult"))
	}
	if outcome.overrides, err = memberOverrides(t, o.cellValues, "--cell-values", members); err != nil {
		return nil, nil, outcome, err
	}
	type source struct {
		src  []byte
		node *jnode
	}
	values := map[string]source{}
	inherited, err := t.parse(packetDir + "/" + inheritedCells)
	if err != nil {
		return nil, nil, outcome, err
	}
	for _, c := range inherited.root.get("cases").elems {
		id := c.get("id").text(inherited.src)
		key, err := frozenKey(inherited.src, c, memberList...)
		if err != nil {
			return nil, nil, outcome, fmt.Errorf("cell %s: %w", id, err)
		}
		matches := byVerdict[key]
		override := outcome.overrides.lookup(id)
		// Cases that disagree on a re-emitted member do not determine the
		// cell's value, so it takes an explicit entry (decision 0535).
		disagree := ""
		for _, m := range memberList {
			if disagree != "" || len(matches) == 0 {
				break
			}
			first, _ := matches[0].get(m).decode(casesNew)
			for _, other := range matches[1:] {
				if v, _ := other.get(m).decode(casesNew); !reflect.DeepEqual(first, v) {
					disagree = m
					break
				}
			}
		}
		switch {
		case len(matches) > 0 && disagree == "" && override != nil:
			return nil, nil, outcome, fmt.Errorf("cell %s matches a case's verdict and also has a --cell-values override", id)
		case len(matches) > 0 && disagree == "":
			values[id] = source{casesNew, matches[0]}
			outcome.derived++
		case disagree != "" && override == nil:
			return nil, nil, outcome, fmt.Errorf("cell %s: cases with its verdict disagree on %q; it needs a --cell-values entry", id, disagree)
		case override != nil:
			values[id] = source{outcome.overrides.src, override}
			outcome.overridden = append(outcome.overridden, id)
		default:
			return nil, nil, outcome, fmt.Errorf("cell %s matches no case's verdict and has no --cell-values entry", id)
		}
	}
	if outcome.overrides != nil {
		for _, id := range outcome.overrides.root.keys {
			if _, ok := values[id]; !ok {
				return nil, nil, outcome, fmt.Errorf("--cell-values names unknown cell %s", id)
			}
		}
	}
	// The packet copy is checked against the record; the stable copy lists the
	// same cells (check), so it takes the same splices without re-recording.
	replace := func(d *document, rr *reemitter) ([]splice, error) {
		var edits []splice
		for _, c := range d.root.get("cases").elems {
			id := c.get("id").text(d.src)
			v, ok := values[id]
			if !ok {
				return nil, fmt.Errorf("%s cell %s is not in %s", d.name, id, inheritedCells)
			}
			expected := c.get("expectedResult")
			for _, m := range memberList {
				node := v.node.get(m)
				if node == nil {
					continue
				}
				i := expected.index(m)
				edit, err := rr.value("cell", id, m, d.src, expected.elems[i], v.src, node, lineIndent(d.src, expected.kspan[i][0]))
				if err != nil {
					return nil, fmt.Errorf("%s: %w", d.name, err)
				}
				if edit != nil {
					edits = append(edits, *edit)
				}
			}
		}
		return edits, nil
	}
	if packet, err = replace(inherited, r); err != nil {
		return nil, nil, outcome, err
	}
	copyDoc, err := t.parse(stableDir + "/" + cellsName)
	if err != nil {
		return nil, nil, outcome, err
	}
	if stable, err = replace(copyDoc, &reemitter{rec: r.rec, used: map[string]bool{}}); err != nil {
		return nil, nil, outcome, err
	}
	sort.SliceStable(r.changes, func(i, j int) bool { return r.changes[i].kind < r.changes[j].kind })
	return packet, stable, outcome, nil
}

// memberOverrides loads an optional JSON object of id to an object of member
// values; a member it omits keeps its value, and any other key is refused.
func memberOverrides(t *tree, file, flagName string, members map[string]bool) (*document, error) {
	if file == "" {
		return nil, nil
	}
	d, err := t.parseFile(file)
	if err != nil {
		return nil, err
	}
	if d.root.kind != '{' {
		return nil, fmt.Errorf("%s must be a JSON object of id to an object of member values", flagName)
	}
	for i, id := range d.root.keys {
		v := d.root.elems[i]
		if v.kind != '{' {
			return nil, fmt.Errorf("%s entry %s is not an object of member values", flagName, id)
		}
		for _, k := range v.keys {
			if !members[k] {
				return nil, fmt.Errorf("%s entry %s names %q, which is not a --member", flagName, id, k)
			}
		}
	}
	return d, nil
}
