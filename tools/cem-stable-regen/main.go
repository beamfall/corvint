// Command cem-stable-regen adds one additive member to the sealed cem/1.0 stable
// result, re-emits recorded values of existing members, widens one recorded
// string const or enum of its schema, or adds one recorded anyOf alternative to
// one of its object-schema nodes, and re-binds every
// digest that seals it (docs/specs/cem-stable-v1.md, CEM-V1-091). It edits only
// by splicing new bytes into the existing text, so every untouched byte of every
// file stays identical, and it refuses verdict drift, a missing value, an
// unrecorded value change or a packet that is not already self-consistent. The predecessor files stay in Git history; each regenerated
// file records the digest of the bytes it replaces.
//
// Usage:
//
//	cem-stable-regen --member NAME --schema FRAGMENT.json --capture CAPTURE.jsonl
//	                 [--values OVERRIDES.json] [--cell-values CELLS.json] [--after MEMBER]
//	                 --note TEXT [--root DIR]
//	cem-stable-regen --replace --member NAME[,NAME...] --record RECORD.json
//	                 --capture CAPTURE.jsonl [--values OVERRIDES.json]
//	                 [--cell-values CELLS.json] [--root DIR]
//	cem-stable-regen --widen --record RECORD.json [--root DIR]
//	cem-stable-regen --any-of --record RECORD.json [--root DIR]
//	cem-stable-regen --check [--root DIR]
//
// --replace re-emits the values of existing members in place (CEM-V1-091 as
// amended by decision 0530). Overrides map an id to an object of member values;
// a member an override omits keeps its value. RECORD.json is
// {"reason": TEXT, "cases": {ID: {MEMBER: SHA256}}, "cells": {...}} and must
// name exactly the values that change, each with the sha256 of the compact JSON
// of the value it replaces; any other change is refused.
//
// --widen adds values to one string const or enum of both RESULT.schema.json
// copies (CEM-V1-620). RECORD.json is {"reason": TEXT, "pointer": POINTER,
// "predecessorSha256": SHA256, "add": [VALUE...]}: the JSON pointer names an
// object member whose value is exactly {"const": STRING} or {"enum": [STRING...]},
// the digest is the sha256 of that value's compact JSON, and every added value
// must be new. No case, cell or other schema byte changes.
//
// --any-of adds one alternative to one object-schema node of both
// RESULT.schema.json copies (CEM-V1-880). RECORD.json is {"reason": TEXT,
// "pointer": POINTER, "predecessorSha256": SHA256, "branch": SCHEMA}: a node
// that is not yet exactly {"anyOf": [...]} becomes {"anyOf": [OLD, SCHEMA]},
// and an existing anyOf gains SCHEMA as its last alternative. No case, cell or
// other schema byte changes.
//
// CAPTURE.jsonl is written by the native S0E harness with
// CEM_STABLE_REGEN_CAPTURE set (internal/cem/verify requireS0EResult).
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	stableDir   = "protocol/cem-1.0/stable"
	packetDir   = stableDir + "/repository-envelope-packet"
	tupleRoot   = "protocol/cem-1.0/structural-tuples"
	schemaName  = "RESULT.schema.json"
	casesName   = "FULL-RESULT-CASES.json"
	changesName = "CHANGES.md"
	readmeName  = "README.md"
	cellsName   = "REPOSITORY-ENVELOPE.cells.json"
	envelopeMD  = "REPOSITORY-ENVELOPE.md"
	regenStatus = "ADDITIVE_MEMBER_REGENERATED"
	// readmePointer marks the one README paragraph the first regeneration adds.
	readmePointer = "Additive member revisions:"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type options struct {
	root, member, schema, after, capture, values, cellValues, note, date, record string
	check, replace, widen, anyOf                                                 bool
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("cem-stable-regen", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var o options
	flags.StringVar(&o.root, "root", ".", "repository root")
	flags.StringVar(&o.member, "member", "", "additive result member name")
	flags.StringVar(&o.schema, "schema", "", "JSON Schema fragment for the member's value")
	flags.StringVar(&o.after, "after", "artifactChecks", "existing member the new member follows")
	flags.StringVar(&o.capture, "capture", "", "JSON Lines capture from the native S0E harness")
	flags.StringVar(&o.values, "values", "", "optional JSON object of case id to value for cases the capture cannot run")
	flags.StringVar(&o.cellValues, "cell-values", "", "optional JSON object of cell id to value for envelope cells that match no case's frozen verdict")
	flags.StringVar(&o.note, "note", "", "one-line driver note for CHANGES.md")
	flags.StringVar(&o.date, "date", time.Now().UTC().Format("2006-01-02"), "date recorded in CHANGES.md")
	flags.BoolVar(&o.check, "check", false, "verify the sealed chain and write nothing")
	flags.BoolVar(&o.replace, "replace", false, "re-emit the values of existing members (comma-separated --member) under --record")
	flags.BoolVar(&o.widen, "widen", false, "add recorded values to one string const or enum of RESULT.schema.json under --record")
	flags.BoolVar(&o.anyOf, "any-of", false, "add one recorded anyOf alternative to one object-schema node of RESULT.schema.json under --record")
	flags.StringVar(&o.record, "record", "", "with --replace: JSON record of the reason and each replaced value's predecessor digest; with --widen: the widening record; with --any-of: the branch record")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	t := &tree{root: o.root, next: map[string][]byte{}}
	if o.check {
		problems := check(t)
		for _, p := range problems {
			fmt.Fprintln(stderr, "cem-stable-regen: "+p)
		}
		if len(problems) > 0 {
			fmt.Fprintf(stdout, "cem-stable-regen check: FAIL (%d problems)\n", len(problems))
			return 1
		}
		fmt.Fprintln(stdout, "cem-stable-regen check: PASS "+t.summary)
		return 0
	}
	var report string
	var err error
	switch {
	case o.anyOf:
		if o.widen || o.replace || o.record == "" || o.member != "" || o.schema != "" || o.capture != "" || o.values != "" || o.cellValues != "" || o.note != "" {
			fmt.Fprintln(stderr, "cem-stable-regen: --any-of needs --record and takes no --widen, --replace, --member, --schema, --capture, --values, --cell-values or --note")
			return 2
		}
		report, err = anyOf(t, o)
	case o.widen:
		if o.replace || o.record == "" || o.member != "" || o.schema != "" || o.capture != "" || o.values != "" || o.cellValues != "" || o.note != "" {
			fmt.Fprintln(stderr, "cem-stable-regen: --widen needs --record and takes no --replace, --member, --schema, --capture, --values, --cell-values or --note")
			return 2
		}
		report, err = widen(t, o)
	case o.replace:
		if o.member == "" || o.record == "" || o.schema != "" || (o.capture == "" && o.values == "") {
			fmt.Fprintln(stderr, "cem-stable-regen: --replace needs --member, --record and --capture or --values, and takes no --schema")
			return 2
		}
		report, err = reemit(t, o)
	case o.member == "" || o.schema == "" || o.note == "" || (o.capture == "" && o.values == ""):
		fmt.Fprintln(stderr, "cem-stable-regen: --member, --schema, --note and --capture or --values are required (or --check)")
		return 2
	default:
		report, err = regenerate(t, o)
	}
	if err != nil {
		fmt.Fprintln(stderr, "cem-stable-regen: refused: "+err.Error())
		return 1
	}
	if err := t.write(); err != nil {
		fmt.Fprintln(stderr, "cem-stable-regen: "+err.Error())
		return 1
	}
	fmt.Fprint(stdout, report)
	return 0
}

// tree reads repository files and holds pending replacements, so a
// regeneration is computed and checked completely before anything is written.
type tree struct {
	root    string
	next    map[string][]byte
	summary string
}

func (t *tree) read(rel string) ([]byte, error) {
	if b, ok := t.next[rel]; ok {
		return b, nil
	}
	return os.ReadFile(filepath.Join(t.root, filepath.FromSlash(rel)))
}

func (t *tree) set(rel string, b []byte) { t.next[rel] = b }

func (t *tree) tuples() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(t.root, filepath.FromSlash(tupleRoot)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, path.Join(tupleRoot, e.Name()))
		}
	}
	return dirs, err
}

func (t *tree) write() error {
	names := make([]string, 0, len(t.next))
	for name := range t.next {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		target := filepath.Join(t.root, filepath.FromSlash(name))
		temp := target + ".regen-tmp"
		if err := os.WriteFile(temp, t.next[name], 0o644); err != nil {
			return err
		}
		if err := os.Rename(temp, target); err != nil {
			return err
		}
	}
	return nil
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// document is one parsed file.
type document struct {
	name string
	src  []byte
	root *jnode
}

func (t *tree) parse(rel string) (*document, error) {
	src, err := t.read(rel)
	if err != nil {
		return nil, err
	}
	root, err := parseJSON(src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	return &document{name: rel, src: src, root: root}, nil
}

// manifestProblems checks every {path, sha256, bytes} entry of
// dir/manifest.json. Each required name must be listed.
func manifestProblems(t *tree, dir string, required ...string) []string {
	m, err := t.parse(dir + "/manifest.json")
	if err != nil {
		return []string{err.Error()}
	}
	files := m.root.get("files")
	if files == nil || files.kind != '[' || len(files.elems) == 0 {
		return []string{dir + "/manifest.json lists no files"}
	}
	var problems []string
	listed := map[string]bool{}
	for _, entry := range files.elems {
		name := entry.get("path").text(m.src)
		listed[name] = true
		if name == "" || path.IsAbs(name) || strings.Contains(name, "..") {
			problems = append(problems, fmt.Sprintf("%s/manifest.json: unsafe path %q", dir, name))
			continue
		}
		b, err := t.read(dir + "/" + name)
		size, _ := strconv.Atoi(string(entry.get("bytes").raw(m.src)))
		if err != nil || digest(b) != entry.get("sha256").text(m.src) || len(b) != size {
			problems = append(problems, fmt.Sprintf("%s/%s differs from its manifest entry", dir, name))
		}
	}
	for _, name := range required {
		if !listed[name] {
			problems = append(problems, fmt.Sprintf("%s/manifest.json does not list %s", dir, name))
		}
	}
	return problems
}

// rebindManifest replaces sha256 and bytes of the listed entries.
func rebindManifest(m *document, changed map[string][]byte) ([]splice, error) {
	var edits []splice
	seen := map[string]bool{}
	for _, entry := range m.root.get("files").elems {
		name := entry.get("path").text(m.src)
		b, ok := changed[name]
		if !ok {
			continue
		}
		seen[name] = true
		edits = append(edits, replaceValue(entry.get("sha256"), quote(digest(b))), replaceValue(entry.get("bytes"), strconv.Itoa(len(b))))
	}
	for name := range changed {
		if !seen[name] {
			return nil, fmt.Errorf("%s does not list %s", m.name, name)
		}
	}
	return edits, nil
}

func stringList(d *document, n *jnode) []string {
	var out []string
	if n == nil {
		return nil
	}
	for _, e := range n.elems {
		out = append(out, e.text(d.src))
	}
	return out
}

// check verifies the whole sealed chain and returns every problem found.
func check(t *tree) []string {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	problems = append(problems, manifestProblems(t, packetDir, schemaName, casesName, inheritedCells, readmeName, changesName)...)
	// Every stable manifest entry is checked, not only the schema: the two
	// envelope entries stale since the packet seal were re-bound by the first
	// regeneration (af8f466dd), so the restriction is gone (V1-1287).
	problems = append(problems, manifestProblems(t, stableDir, schemaName)...)
	problems = append(problems, pointerProblems(t.root, recordedDanglingPointers)...)
	packetManifest, err := t.read(packetDir + "/manifest.json")
	if err != nil {
		return append(problems, err.Error())
	}
	packetSum := digest(packetManifest)

	schemaA, errA := t.read(stableDir + "/" + schemaName)
	schemaB, errB := t.read(packetDir + "/" + schemaName)
	if errA != nil || errB != nil || !bytes.Equal(schemaA, schemaB) {
		add("the stable and packet %s copies are not byte-identical", schemaName)
	}
	schema, err := t.parse(stableDir + "/" + schemaName)
	if err != nil {
		return append(problems, err.Error())
	}
	required := stringList(schema, schema.root.get("required"))
	properties := schema.root.get("properties")
	if properties == nil || len(required) == 0 {
		return append(problems, schemaName+" has no properties or required list")
	}
	if a, b := append([]string(nil), required...), append([]string(nil), properties.keys...); !sameSet(a, b) {
		add("%s required %v differs from its properties %v", schemaName, required, properties.keys)
	}

	cases, err := t.parse(packetDir + "/" + casesName)
	if err != nil {
		return append(problems, err.Error())
	}
	list := cases.root.get("cases")
	if list == nil || list.kind != '[' {
		return append(problems, casesName+" has no cases")
	}
	for _, c := range list.elems {
		if keys := c.get("expectedResult"); keys == nil || !reflect.DeepEqual(keys.keys, required) {
			add("case %s expectedResult keys differ from %s required", c.get("id").text(cases.src), schemaName)
		}
	}

	stable, err := t.parse(stableDir + "/manifest.json")
	if err != nil {
		return append(problems, err.Error())
	}
	if stable.root.get("publicAmendmentManifestSHA256").text(stable.src) != packetSum {
		add("stable manifest publicAmendmentManifestSHA256 does not bind the packet manifest")
	}
	cells, err := t.parse(stableDir + "/" + cellsName)
	if err != nil {
		return append(problems, err.Error())
	}
	amendment := cells.root.get("amendment")
	index := amendment.get("fullResultCases")
	count, _ := strconv.Atoi(string(index.get("count").raw(cells.src)))
	size, _ := strconv.Atoi(string(index.get("bytes").raw(cells.src)))
	if amendment.get("publicManifestSHA256").text(cells.src) != packetSum || index.get("sha256").text(cells.src) != digest(cases.src) || size != len(cases.src) {
		add("%s amendment index does not bind the packet manifest and %s", cellsName, casesName)
	}
	if count == 0 || count != len(list.elems) {
		add("%s has %d cases, the amendment index seals %d", casesName, len(list.elems), count)
	}
	problems = append(problems, cellProblems(t, required)...)
	if prose, err := t.read(stableDir + "/" + envelopeMD); err != nil || !bytes.Contains(prose, []byte(packetSum)) || !bytes.Contains(prose, []byte(digest(cases.src))) {
		add("%s does not name the current packet manifest and %s digests", envelopeMD, casesName)
	}

	tuples, err := t.tuples()
	if err != nil {
		add("%s: %v", tupleRoot, err)
	}
	for _, dir := range tuples {
		problems = append(problems, manifestProblems(t, dir, "TUPLE.json")...)
		tuple, err := t.parse(dir + "/TUPLE.json")
		if err != nil {
			add("%v", err)
			continue
		}
		inherited := tuple.root.get("inherited")
		if inherited.get("manifestSha256").text(tuple.src) != packetSum {
			add("%s/TUPLE.json does not bind the packet manifest", dir)
		}
		files := inherited.get("files")
		if files == nil || len(files.keys) == 0 {
			add("%s/TUPLE.json binds no packet file", dir)
			continue
		}
		for i, name := range files.keys {
			b, err := t.read(packetDir + "/" + name)
			if err != nil || digest(b) != files.elems[i].text(tuple.src) {
				add("%s/TUPLE.json does not bind packet file %s", dir, name)
			}
		}
	}
	t.summary = fmt.Sprintf("(%d cases x %d keys; packet manifest %s; %d structural tuple(s))", len(list.elems), len(required), packetSum, len(tuples))
	return problems
}

func sameSet(a, b []string) bool {
	sort.Strings(a)
	sort.Strings(b)
	for i := 1; i < len(a); i++ {
		if a[i] == a[i-1] {
			return false
		}
	}
	return reflect.DeepEqual(a, b)
}

// captured is one native harness record.
type captured struct {
	tuple  string
	exit   int
	src    []byte
	result *jnode
}

func loadCapture(file string) (map[string]*captured, error) {
	out := map[string]*captured{}
	if file == "" {
		return out, nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	for i, line := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		root, err := parseJSON(line)
		if err != nil {
			return nil, fmt.Errorf("capture line %d: %w", i+1, err)
		}
		id := root.get("id").text(line)
		exit, err := strconv.Atoi(string(root.get("exit").raw(line)))
		result := root.get("result")
		if id == "" || err != nil || result == nil || result.kind != '{' {
			return nil, fmt.Errorf("capture line %d is not an {id, tuple, exit, result} record", i+1)
		}
		c := &captured{tuple: root.get("tuple").text(line), exit: exit, src: line, result: result}
		if prior, ok := out[id]; ok {
			a, errA := prior.result.decode(prior.src)
			b, errB := result.decode(line)
			if errA != nil || errB != nil || prior.exit != exit || !reflect.DeepEqual(a, b) {
				return nil, fmt.Errorf("capture records case %s twice with different results", id)
			}
			continue
		}
		out[id] = c
	}
	return out, nil
}

var closedCount = regexp.MustCompile(`closed(\d+)-key|(\d+)-field`)

// bumpCount rewrites "closed<old>-key" and "<old>-field" in a string node.
func bumpCount(d *document, n *jnode, old int) []splice {
	if n == nil || n.kind != '"' {
		return nil
	}
	raw := string(n.raw(d.src))
	next := closedCount.ReplaceAllStringFunc(raw, func(m string) string {
		digits := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(m, "closed"), "-key"), "-field")
		if digits != strconv.Itoa(old) {
			return m
		}
		return strings.Replace(m, digits, strconv.Itoa(old+1), 1)
	})
	if next == raw {
		return nil
	}
	return []splice{replaceValue(n, next)}
}

// regenerate computes every replacement file in t and returns a report.
func regenerate(t *tree, o options) (string, error) {
	if problems := check(t); len(problems) > 0 {
		return "", fmt.Errorf("the packet is not self-consistent before regeneration: %s", strings.Join(problems, "; "))
	}
	packetManifestOld, _ := t.read(packetDir + "/manifest.json")
	fragmentSrc, err := os.ReadFile(o.schema)
	if err != nil {
		return "", err
	}
	fragment, err := parseJSON(fragmentSrc)
	if err != nil {
		return "", fmt.Errorf("schema fragment: %w", err)
	}
	if fragment.kind != '{' {
		return "", errors.New("schema fragment must be a JSON Schema object")
	}

	// 1. RESULT.schema.json, both byte-identical copies.
	schema, _ := t.parse(stableDir + "/" + schemaName)
	schemaOld := schema.src
	property, err := insertMember(schema.src, schema.root.get("properties"), o.after, o.member, func(indent string) string { return render(fragmentSrc, fragment, indent) })
	if err != nil {
		return "", fmt.Errorf("%s properties: %w", schemaName, err)
	}
	listed, err := insertString(schema.src, schema.root.get("required"), o.after, o.member)
	if err != nil {
		return "", fmt.Errorf("%s required: %w", schemaName, err)
	}
	schemaNew, err := applySplices(schema.src, []splice{property, listed})
	if err != nil {
		return "", err
	}
	t.set(stableDir+"/"+schemaName, schemaNew)
	t.set(packetDir+"/"+schemaName, schemaNew)

	// 2. FULL-RESULT-CASES.json.
	capture, err := loadCapture(o.capture)
	if err != nil {
		return "", err
	}
	var values *document
	if o.values != "" {
		if values, err = t.parseFile(o.values); err != nil {
			return "", err
		}
		if values.root.kind != '{' {
			return "", errors.New("--values must be a JSON object of case id to value")
		}
	}
	substitutions, err := tupleSubstitutions(t)
	if err != nil {
		return "", err
	}
	cases, _ := t.parse(packetDir + "/" + casesName)
	list := cases.root.get("cases")
	oldKeys := len(list.elems[0].get("expectedResult").keys)
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
		var value string
		switch {
		case hasCapture && override != nil:
			return "", fmt.Errorf("case %s has both a captured value and a --values override", id)
		case hasCapture:
			if err := compareCapture(cases, c, capturedCase, map[string]bool{o.member: true}, substitutions); err != nil {
				return "", fmt.Errorf("case %s: %w", id, err)
			}
			node := capturedCase.result.get(o.member)
			value = render(capturedCase.src, node, lineIndent(cases.src, expected.kspan[0][0]))
			tuplesSeen[capturedCase.tuple]++
		case override != nil:
			value = render(values.src, override, lineIndent(cases.src, expected.kspan[0][0]))
			overridden = append(overridden, id)
		default:
			return "", fmt.Errorf("case %s has no captured value and no --values override for %q", id, o.member)
		}
		edit, err := insertMember(cases.src, expected, o.after, o.member, func(string) string { return value })
		if err != nil {
			return "", fmt.Errorf("case %s: %w", id, err)
		}
		edits = append(edits, edit)
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
	casesOld := cases.src
	predecessor := cases.root.get("predecessorSHA256")
	if predecessor == nil {
		return "", fmt.Errorf("%s has no predecessorSHA256", casesName)
	}
	edits = append(edits, replaceValue(predecessor, quote(digest(casesOld))))
	edits = append(edits, bumpCount(cases, cases.root.get("comparison"), oldKeys)...)
	casesNew, err := applySplices(cases.src, edits)
	if err != nil {
		return "", err
	}
	t.set(packetDir+"/"+casesName, casesNew)

	// 3. The packet's inherited envelope cells and the stable copy.
	cellEdits, stableCellEdits, cellsOutcome, err := cellSplices(t, o, casesNew)
	if err != nil {
		return "", err
	}
	inheritedOld, _ := t.read(packetDir + "/" + inheritedCells)
	inheritedNew, err := applySplices(inheritedOld, cellEdits)
	if err != nil {
		return "", err
	}
	t.set(packetDir+"/"+inheritedCells, inheritedNew)

	// 4. README pointer, added once.
	readmeOld, err := t.read(packetDir + "/" + readmeName)
	if err != nil {
		return "", err
	}
	packetChanged := map[string][]byte{schemaName: schemaNew, casesName: casesNew, inheritedCells: inheritedNew}
	if !bytes.Contains(readmeOld, []byte(readmePointer)) {
		readmeNew := append(append([]byte(nil), readmeOld...), []byte("\n\n"+readmePointer+" each later revision adds one stable result\nmember with `make cem-stable-regen` (CEM-V1-091), which keeps every frozen\nkey, exit and case, re-binds the digests and appends a section to CHANGES.md.\n")...)
		t.set(packetDir+"/"+readmeName, readmeNew)
		packetChanged[readmeName] = readmeNew
	}
	cells, _ := t.parse(stableDir + "/" + cellsName)
	stableCellEdits = append(stableCellEdits, bumpCount(cells, cells.root.get("amendment").get("note"), oldKeys)...)

	// 5-8. CHANGES.md, the packet manifest, the stable bindings and every tuple.
	oldSum, newSum, err := sealChain(t, packetManifestOld, packetChanged, casesOld, casesNew, stableCellEdits, regenStatus, func(changesOld []byte, outside map[string][]byte) []byte {
		return changesSection(o, t, packetManifestOld, fragmentSrc, values, tuplesSeen, overridden, cellsOutcome, len(list.elems), oldKeys, outside, map[string][2][]byte{
			schemaName:     {schemaOld, schemaNew},
			casesName:      {casesOld, casesNew},
			inheritedCells: {inheritedOld, inheritedNew},
			readmeName:     {readmeOld, packetChanged[readmeName]},
			changesName:    {changesOld, nil},
		})
	})
	if err != nil {
		return "", err
	}
	// The regenerated chain must pass the same check, with one more key per case.
	if problems := check(t); len(problems) > 0 {
		return "", fmt.Errorf("regenerated packet fails its own check: %s", strings.Join(problems, "; "))
	}
	after, _ := t.parse(packetDir + "/" + casesName)
	if n := len(after.root.get("cases").elems); n != len(list.elems) {
		return "", fmt.Errorf("case count changed from %d to %d", len(list.elems), n)
	}
	var report strings.Builder
	fmt.Fprintf(&report, "cem-stable-regen: added %q after %q to %d cases (%d -> %d keys)\n", o.member, o.after, len(list.elems), oldKeys, oldKeys+1)
	fmt.Fprintf(&report, "packet manifest %s -> %s\n", oldSum, newSum)
	names := make([]string, 0, len(t.next))
	for name := range t.next {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintln(&report, "wrote "+name)
	}
	return report.String(), nil
}

func (t *tree) parseFile(file string) (*document, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	root, err := parseJSON(src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return &document{name: file, src: src, root: root}, nil
}

func (d *document) lookup(id string) *jnode {
	if d == nil {
		return nil
	}
	return d.root.get(id)
}

// tupleSubstitutions maps each admitted runtime.goVersion substitution
// "from -> to" of a sealed structural-tuple packet.
func tupleSubstitutions(t *tree) (map[[2]string]bool, error) {
	out := map[[2]string]bool{}
	tuples, err := t.tuples()
	for _, dir := range tuples {
		tuple, err := t.parse(dir + "/TUPLE.json")
		if err != nil {
			return nil, err
		}
		s := tuple.root.get("substitution")
		if s.get("field").text(tuple.src) == "expectedResult.runtime.goVersion" {
			out[[2]string{s.get("from").text(tuple.src), s.get("to").text(tuple.src)}] = true
		}
	}
	return out, err
}

// compareCapture refuses any drift between a captured native result and the
// frozen expectation: every frozen key but the regenerated members, the exit,
// and no extra key but a member. runtime.goVersion may differ only by an
// admitted tuple substitution.
func compareCapture(cases *document, c *jnode, got *captured, members map[string]bool, substitutions map[[2]string]bool) error {
	expectedExit, _ := strconv.Atoi(string(c.get("expectedExit").raw(cases.src)))
	if got.exit != expectedExit {
		return fmt.Errorf("verdict drift: exit %d, frozen %d", got.exit, expectedExit)
	}
	expected := c.get("expectedResult")
	for _, key := range got.result.keys {
		if !members[key] && expected.index(key) < 0 {
			return fmt.Errorf("capture carries key %q that is neither frozen nor --member", key)
		}
	}
	for _, member := range sortedKeys(members) {
		if got.result.get(member) == nil {
			return fmt.Errorf("capture has no %q member", member)
		}
	}
	for i, key := range expected.keys {
		if members[key] {
			continue
		}
		node := got.result.get(key)
		if node == nil {
			return fmt.Errorf("verdict drift: capture lacks frozen key %q", key)
		}
		want, err := expected.elems[i].decode(cases.src)
		if err != nil {
			return fmt.Errorf("frozen key %q: %w", key, err)
		}
		have, err := node.decode(got.src)
		if err != nil {
			return fmt.Errorf("captured key %q: %w", key, err)
		}
		if key == "runtime" {
			wantRuntime, _ := want.(map[string]any)
			haveRuntime, _ := have.(map[string]any)
			from, _ := wantRuntime["goVersion"].(string)
			to, _ := haveRuntime["goVersion"].(string)
			if haveRuntime != nil && from != to && substitutions[[2]string{from, to}] {
				haveRuntime["goVersion"] = from
			}
		}
		if !reflect.DeepEqual(have, want) {
			return fmt.Errorf("verdict drift in frozen key %q", key)
		}
	}
	return nil
}

func changesSection(o options, t *tree, manifestOld, fragment []byte, values *document, tuples map[string]int, overridden []string, cells cellOutcome, count, oldKeys int, outside map[string][]byte, files map[string][2][]byte) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "\n# Additive member `%s` (%s)\n\n", o.member, o.date)
	fmt.Fprintf(&b, "Base: the public packet with manifest sha256\n%s.\n", digest(manifestOld))
	fmt.Fprintf(&b, "Driver: %s\n", o.note)
	fmt.Fprintf(&b, "Produced by `make cem-stable-regen` (tools/cem-stable-regen, CEM-V1-091).\n\n")
	fmt.Fprintf(&b, "- Member: `%s`, after `%s` in RESULT.schema.json `properties` and\n  `required` and in every expectedResult.\n", o.member, o.after)
	fmt.Fprintf(&b, "- Schema fragment sha256: %s.\n", digest(fragment))
	names := make([]string, 0, len(tuples))
	for name := range tuples {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&b, "- Values captured from the native verifier on %s for %d cases.\n", name, tuples[name])
	}
	switch {
	case len(overridden) > 8:
		fmt.Fprintf(&b, "- Values from the override file (sha256 %s) for %d cases.\n", digest(values.src), len(overridden))
	case len(overridden) > 0:
		fmt.Fprintf(&b, "- Values from the override file (sha256 %s) for: %s.\n", digest(values.src), strings.Join(overridden, ", "))
	}
	fmt.Fprintf(&b, "- Envelope cells (%s and its ../%s copy):\n  %d cells take the value of the cases with the same exit and frozen keys.\n", inheritedCells, cellsName, cells.derived)
	if len(cells.overridden) > 0 {
		fmt.Fprintf(&b, "- Cell values from the cell override file (sha256 %s) for: %s.\n", digest(cells.overrides.src), strings.Join(cells.overridden, ", "))
	}
	fmt.Fprintf(&b, "- No frozen key, expectedExit, recipe or status changed; every captured\n  result matched its frozen expectation on all %d frozen keys and its exit\n  (runtime.goVersion only through a sealed structural-tuple substitution).\n  The case count stays %d.\n\n", oldKeys, count)
	writeFileTables(&b, manifestOld, outside, files)
	fmt.Fprintf(&b, "| %s/%s | identical to %s/%s |\n", stableDir, schemaName, packetDir, schemaName)
	return []byte(b.String())
}

// writeFileTables appends the changed-file table, with the old and new digest
// of every replaced packet file, and the predecessor digest of each re-bound
// file outside the packet.
func writeFileTables(b *strings.Builder, manifestOld []byte, outside map[string][]byte, files map[string][2][]byte) {
	b.WriteString("## Changed files\n\n| File | Old sha256 | New sha256 |\n| --- | --- | --- |\n")
	for _, name := range []string{schemaName, casesName, inheritedCells, readmeName, changesName} {
		pair := files[name]
		switch {
		case name == changesName:
			fmt.Fprintf(b, "| %s | %s | See manifest.json; this file cannot contain its own final digest. |\n", name, digest(pair[0]))
		case pair[1] != nil:
			fmt.Fprintf(b, "| %s | %s | %s |\n", name, digest(pair[0]), digest(pair[1]))
		}
	}
	fmt.Fprintf(b, "| manifest.json | %s | See ../manifest.json publicAmendmentManifestSHA256; manifest.json is not listed inside its own file list. |\n\n", digest(manifestOld))
	b.WriteString("Files not listed above are byte-identical to the base packet. INPUTS.json\nkeeps its repair-era pins as history.\n\n## Re-bound files outside the packet\n\nThese files are re-bound to the digests above; their new digests depend on this\nfile and are recorded in Git, in ../manifest.json and in each tuple manifest.\n\n| File | Predecessor sha256 |\n| --- | --- |\n")
	for _, name := range sortedKeys(outside) {
		fmt.Fprintf(b, "| %s | %s |\n", name, digest(outside[name]))
	}
}

func sortedKeys[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
