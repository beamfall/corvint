package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// replaceFixture builds the inputs for re-emitting verdictSha256 of one real
// case: a capture equal to every frozen expectation except that case's new
// value, empty cell entries for every cell no case matches, and the record of
// the case and of each cell that derives from it.
type replaceFixture struct {
	root, capture, cells, target, oldValue, newValue string
	derived                                          []string
	record                                           record
}

func newReplaceFixture(t *testing.T) *replaceFixture {
	t.Helper()
	f := &replaceFixture{root: copyProtocol(t), target: "normal-exit-lingering-descendant-contained", newValue: strings.Repeat("ab", 32)}
	dir := t.TempDir()
	f.capture = filepath.Join(dir, "capture.jsonl")
	cases := frozenCases(t, f.root)
	var targetKey string
	for _, c := range cases {
		if c.ID == f.target {
			f.oldValue, _ = c.ExpectedResult["verdictSha256"].(string)
			targetKey = verdictKey(t, c, "verdictSha256")
		}
	}
	if f.oldValue == "" {
		t.Fatalf("case %s not found", f.target)
	}
	captureFor(t, f.capture, cases, func(int) any { return nil }, func(i int, r map[string]any) {
		delete(r, "probe")
		if cases[i].ID == f.target {
			r["verdictSha256"] = f.newValue
		}
	})
	f.cells, _ = cellOverrides(t, f.root, filepath.Join(dir, "cells.json"), func(string) any { return map[string]any{} }, "verdictSha256")
	old, _ := json.Marshal(f.oldValue)
	f.record = record{Reason: "test re-emission", Cases: map[string]map[string]string{f.target: {"verdictSha256": digest(old)}}, Cells: map[string]map[string]string{}}
	for _, c := range frozenCells(t, f.root, "verdictSha256") {
		if c.key == targetKey {
			f.derived = append(f.derived, c.ID)
			f.record.Cells[c.ID] = map[string]string{"verdictSha256": digest(old)}
		}
	}
	if len(f.derived) == 0 {
		t.Fatalf("no cell derives from case %s", f.target)
	}
	return f
}

func (f *replaceFixture) run(t *testing.T, rec any, extra ...string) (int, string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "record.json")
	writeJSON(t, file, rec)
	args := append([]string{"--root", f.root, "--replace", "--member", "verdictSha256", "--record", file, "--capture", f.capture, "--cell-values", f.cells, "--date", "2026-10-15"}, extra...)
	return runTool(t, args...)
}

func TestReplaceRealPacketCopy(t *testing.T) {
	f := newReplaceFixture(t)
	before := snapshot(t, f.root)
	code, out := f.run(t, f.record)
	if code != 0 {
		t.Fatalf("replace: exit %d\n%s", code, out)
	}
	after := snapshot(t, f.root)
	if code, out := runTool(t, "--root", f.root, "--check"); code != 0 {
		t.Fatalf("check after replace: %s", out)
	}
	// FULL-RESULT-CASES.json differs only by the value and its predecessor pin.
	cases := packetDir + "/" + casesName
	restored := bytes.ReplaceAll(after[cases], []byte(f.newValue), []byte(f.oldValue))
	prior, _ := parseJSON(before[cases])
	now, _ := parseJSON(restored)
	restored = append(append(append([]byte(nil), restored[:now.get("predecessorSHA256").start]...), prior.get("predecessorSHA256").raw(before[cases])...), restored[now.get("predecessorSHA256").end:]...)
	if !bytes.Equal(restored, before[cases]) {
		t.Fatal("FULL-RESULT-CASES.json changed beyond the recorded value and predecessorSHA256")
	}
	if n := bytes.Count(after[cases], []byte(f.newValue)); n != 1 {
		t.Fatalf("new value appears %d times in %s, want 1", n, casesName)
	}
	for _, rel := range []string{packetDir + "/" + inheritedCells, stableDir + "/" + cellsName} {
		if n := bytes.Count(after[rel], []byte(f.newValue)); n != len(f.derived) {
			t.Fatalf("%s carries the new value %d times, want %d derived cells", rel, n, len(f.derived))
		}
	}
	for _, rel := range []string{stableDir + "/" + schemaName, packetDir + "/" + schemaName} {
		if !bytes.Equal(before[rel], after[rel]) {
			t.Fatalf("%s changed", rel)
		}
	}
	manifest, _ := parseJSON(after[packetDir+"/manifest.json"])
	if got := manifest.get("status").text(after[packetDir+"/manifest.json"]); got != replaceStatus {
		t.Fatalf("status %q, want %q", got, replaceStatus)
	}
	changes := string(after[packetDir+"/"+changesName])
	old, _ := json.Marshal(f.oldValue)
	for _, want := range []string{"# Recorded value re-emission of `verdictSha256` (2026-10-15)", "Reason: test re-emission", "| case | " + f.target + " | verdictSha256 | " + digest(old) + " |", "| cell | " + f.derived[0] + " | verdictSha256 |"} {
		if !strings.Contains(changes, want) {
			t.Fatalf("CHANGES.md lacks %q", want)
		}
	}
	if !bytes.Contains(after[packetDir+"/"+readmeName], []byte(replacePointer)) {
		t.Fatal("README lacks the re-emission pointer")
	}
}

func TestReplaceRefusals(t *testing.T) {
	f := newReplaceFixture(t)
	clone := func(edit func(r *record)) record {
		r := record{Reason: f.record.Reason, Cases: map[string]map[string]string{}, Cells: map[string]map[string]string{}}
		for id, m := range f.record.Cases {
			r.Cases[id] = map[string]string{"verdictSha256": m["verdictSha256"]}
		}
		for id, m := range f.record.Cells {
			r.Cells[id] = map[string]string{"verdictSha256": m["verdictSha256"]}
		}
		edit(&r)
		return r
	}
	unknownMember := filepath.Join(t.TempDir(), "values.json")
	writeJSON(t, unknownMember, map[string]any{f.target: map[string]any{"citation": nil}})
	cases := []struct {
		name  string
		rec   any
		extra []string
		want  string
	}{
		{"unrecorded case change", clone(func(r *record) { delete(r.Cases, f.target) }), nil, "case " + f.target + " member \"verdictSha256\" changes but the record does not name it"},
		{"unrecorded cell change", clone(func(r *record) { delete(r.Cells, f.derived[0]) }), nil, "cell " + f.derived[0] + " member \"verdictSha256\" changes but the record does not name it"},
		{"wrong predecessor", clone(func(r *record) { r.Cases[f.target]["verdictSha256"] = strings.Repeat("0", 64) }), nil, "the record names predecessor"},
		{"recorded but unchanged", clone(func(r *record) {
			r.Cases["ledger-336-evidence"] = map[string]string{"verdictSha256": strings.Repeat("0", 64)}
		}), nil, "is recorded but does not change"},
		{"recorded but not re-emitted", clone(func(r *record) { r.Cases["no-such-case"] = map[string]string{"verdictSha256": "x"} }), nil, "which is not re-emitted"},
		{"empty reason", clone(func(r *record) { r.Reason = " " }), nil, "--record has no reason"},
		{"unknown record field", map[string]any{"reason": "x", "extra": 1}, nil, "unknown field"},
		{"override names a non-member", f.record, []string{"--values", unknownMember}, "which is not a --member"},
	}
	before := snapshot(t, f.root)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, out := f.run(t, c.rec, c.extra...)
			if code != 1 || !strings.Contains(out, c.want) {
				t.Fatalf("exit %d, want 1 with %q:\n%s", code, c.want, out)
			}
		})
	}
	if code, out := runTool(t, "--root", f.root, "--replace", "--member", "verdictSha256", "--record", "r.json", "--capture", f.capture, "--schema", "s.json"); code != 2 || !strings.Contains(out, "takes no --schema") {
		t.Fatalf("--replace with --schema: exit %d\n%s", code, out)
	}
	file := filepath.Join(t.TempDir(), "record.json")
	writeJSON(t, file, f.record)
	if code, out := runTool(t, "--root", f.root, "--replace", "--member", "verdictSha256,noSuchMember", "--record", file, "--capture", f.capture); code != 1 || !strings.Contains(out, "is not an existing result member") {
		t.Fatalf("non-existing member: exit %d\n%s", code, out)
	}
	after := snapshot(t, f.root)
	for name, b := range before {
		if !bytes.Equal(b, after[name]) {
			t.Fatalf("a refused re-emission wrote %s", name)
		}
	}
}

// TestReplaceCellOnly re-emits a value of the first envelope cell no case
// matches while every case value stays: FULL-RESULT-CASES.json, its
// predecessorSHA256 and the amendment index stay byte-identical (decision 0535).
func TestReplaceCellOnly(t *testing.T) {
	f := newReplaceFixture(t)
	captureFor(t, f.capture, frozenCases(t, f.root), func(int) any { return nil }, func(_ int, r map[string]any) { delete(r, "probe") })
	var cell, oldValue string
	f.cells, _ = cellOverrides(t, f.root, filepath.Join(t.TempDir(), "cells.json"), func(id string) any {
		if cell == "" {
			cell = id
			return map[string]any{"verdictSha256": f.newValue}
		}
		return map[string]any{}
	}, "verdictSha256")
	for _, c := range frozenCells(t, f.root, "verdictSha256") {
		if c.ID == cell {
			oldValue, _ = c.ExpectedResult["verdictSha256"].(string)
		}
	}
	if oldValue == "" || !bytes.Contains(mustRead(t, f.cells), []byte(f.newValue)) {
		t.Fatalf("cell %s is missing or matches a case", cell)
	}
	old, _ := json.Marshal(oldValue)
	rec := record{Reason: "cell-only re-emission", Cases: map[string]map[string]string{}, Cells: map[string]map[string]string{cell: {"verdictSha256": digest(old)}}}
	before := snapshot(t, f.root)
	if code, out := f.run(t, record{Reason: "nothing", Cases: map[string]map[string]string{}, Cells: map[string]map[string]string{}}, "--cell-values", emptyCells(t, f.root)); code != 1 || !strings.Contains(out, "no value changes; nothing to re-emit") {
		t.Fatalf("no change at all: exit %d\n%s", code, out)
	}
	code, out := f.run(t, rec)
	if code != 0 {
		t.Fatalf("cell-only replace: exit %d\n%s", code, out)
	}
	after := snapshot(t, f.root)
	if code, out := runTool(t, "--root", f.root, "--check"); code != 0 {
		t.Fatalf("check after cell-only replace: %s", out)
	}
	if !bytes.Equal(before[packetDir+"/"+casesName], after[packetDir+"/"+casesName]) {
		t.Fatal("FULL-RESULT-CASES.json changed although no case value changed")
	}
	for _, rel := range []string{packetDir + "/" + inheritedCells, stableDir + "/" + cellsName} {
		if n := bytes.Count(after[rel], []byte(f.newValue)); n != 1 {
			t.Fatalf("%s carries the new value %d times, want 1", rel, n)
		}
	}
	changes := string(after[packetDir+"/"+changesName])
	section := changes[len(before[packetDir+"/"+changesName]):]
	if !strings.Contains(section, "| cell | "+cell+" | verdictSha256 | "+digest(old)+" |") || strings.Contains(section, "| case | ") || strings.Contains(section, "| "+casesName+" | ") {
		t.Fatalf("CHANGES.md section does not record exactly the cell change:\n%s", section)
	}
}

func mustRead(t *testing.T, file string) []byte {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// emptyCells writes a cell override file that keeps every unmatched cell.
func emptyCells(t *testing.T, root string) string {
	t.Helper()
	file, _ := cellOverrides(t, root, filepath.Join(t.TempDir(), "empty-cells.json"), func(string) any { return map[string]any{} }, "verdictSha256")
	return file
}

// TestReplaceCellDisagreeingCases re-emits code and verdictSha256 of a cell
// whose other keys equal those of cases that disagree on code: the value
// cannot be derived, so the cell needs an explicit --cell-values entry
// (decision 0535). Without one the re-emission is refused.
func TestReplaceCellDisagreeingCases(t *testing.T) {
	f := newReplaceFixture(t)
	var cell string
	members := []string{"code", "verdictSha256"}
	cases := frozenCases(t, f.root)
	captureFor(t, f.capture, cases, func(int) any { return nil }, func(_ int, r map[string]any) { delete(r, "probe") })
	values := map[string]map[string]bool{}
	for _, c := range cases {
		key := verdictKey(t, c, members...)
		if values[key] == nil {
			values[key] = map[string]bool{}
		}
		v, _ := json.Marshal([]any{c.ExpectedResult["code"], c.ExpectedResult["verdictSha256"]})
		values[key][string(v)] = true
	}
	partial, full := map[string]any{}, map[string]any{}
	var oldCode, oldVerdict string
	for _, c := range frozenCells(t, f.root, members...) {
		switch n := len(values[c.key]); {
		case n == 0:
			partial[c.ID], full[c.ID] = map[string]any{}, map[string]any{}
		case n > 1:
			full[c.ID] = map[string]any{}
		}
		if cell == "" && len(values[c.key]) > 1 {
			cell = c.ID
			oldCode, _ = c.ExpectedResult["code"].(string)
			oldVerdict, _ = c.ExpectedResult["verdictSha256"].(string)
			newCode := "repository-object-unavailable"
			if oldCode == newCode {
				newCode = "git-read-failed"
			}
			full[c.ID] = map[string]any{"code": newCode, "verdictSha256": f.newValue}
		}
	}
	if cell == "" {
		t.Fatal("no cell matches cases that disagree on code")
	}
	dir := t.TempDir()
	partialFile, fullFile := filepath.Join(dir, "partial.json"), filepath.Join(dir, "full.json")
	writeJSON(t, partialFile, partial)
	writeJSON(t, fullFile, full)
	codeOld, _ := json.Marshal(oldCode)
	verdictOld, _ := json.Marshal(oldVerdict)
	rec := record{Reason: "missing object", Cases: map[string]map[string]string{}, Cells: map[string]map[string]string{cell: {"code": digest(codeOld), "verdictSha256": digest(verdictOld)}}}
	before := snapshot(t, f.root)
	if code, out := f.run(t, rec, "--member", "code,verdictSha256", "--cell-values", partialFile); code != 1 || !strings.Contains(out, "cases with its verdict disagree on \"code\"; it needs a --cell-values entry") {
		t.Fatalf("disagreeing cases without an entry: exit %d\n%s", code, out)
	}
	if after := snapshot(t, f.root); !bytes.Equal(before[packetDir+"/manifest.json"], after[packetDir+"/manifest.json"]) {
		t.Fatal("a refused re-emission wrote the packet")
	}
	code, out := f.run(t, rec, "--member", "code,verdictSha256", "--cell-values", fullFile)
	if code != 0 {
		t.Fatalf("replace: exit %d\n%s", code, out)
	}
	after := snapshot(t, f.root)
	if code, out := runTool(t, "--root", f.root, "--check"); code != 0 {
		t.Fatalf("check after replace: %s", out)
	}
	if !bytes.Equal(before[packetDir+"/"+casesName], after[packetDir+"/"+casesName]) {
		t.Fatal("FULL-RESULT-CASES.json changed although no case value changed")
	}
	section := string(after[packetDir+"/"+changesName][len(before[packetDir+"/"+changesName]):])
	for _, want := range []string{"| cell | " + cell + " | code | " + digest(codeOld) + " |", "| cell | " + cell + " | verdictSha256 | " + digest(verdictOld) + " |"} {
		if !strings.Contains(section, want) {
			t.Fatalf("CHANGES.md section lacks %q:\n%s", want, section)
		}
	}
}

// TestReplaceRuntimeKeepsFrozenTuple re-emits runtime from a capture on a later
// admitted tuple, here in the direction a rollback of decision 0551 would take:
// the structural qualification changes and goVersion stays the frozen tuple
// every substitution starts from. A captured goVersion that no sealed
// substitution admits is refused.
func TestReplaceRuntimeKeepsFrozenTuple(t *testing.T) {
	f := newReplaceFixture(t)
	const qualified, pending = "QUALIFIED_EXACT_TUPLE", "EXPERIMENTAL_TUPLE_PENDING_FULL_CORPUS"
	requalify := func(rt any, goVersion string) (any, bool) {
		m, ok := rt.(map[string]any)
		if !ok || m["structuralQualification"] != qualified {
			return rt, false
		}
		c := map[string]any{"structuralQualification": pending}
		for k, v := range m {
			if k != "structuralQualification" {
				c[k] = v
			}
		}
		if goVersion != "" {
			c["goVersion"] = goVersion
		}
		return c, true
	}
	cases := frozenCases(t, f.root)
	rec := record{Reason: "structural qualification rolled back", Cases: map[string]map[string]string{}, Cells: map[string]map[string]string{}}
	changed := map[string]bool{}
	for _, c := range cases {
		if _, ok := requalify(c.ExpectedResult["runtime"], ""); ok {
			old, _ := json.Marshal(c.ExpectedResult["runtime"])
			rec.Cases[c.ID] = map[string]string{"runtime": digest(old)}
			changed[verdictKey(t, c, "runtime")] = true
		}
	}
	// A cell no case matches takes its requalified runtime from --cell-values.
	verdicts := map[string]bool{}
	for _, c := range cases {
		verdicts[verdictKey(t, c, "runtime")] = true
	}
	overrides, derived := map[string]any{}, 0
	for _, c := range frozenCells(t, f.root, "runtime") {
		next, ok := requalify(c.ExpectedResult["runtime"], "")
		if ok {
			old, _ := json.Marshal(c.ExpectedResult["runtime"])
			rec.Cells[c.ID] = map[string]string{"runtime": digest(old)}
		}
		switch {
		case verdicts[c.key] && changed[c.key]:
			derived++
		case !verdicts[c.key] && ok:
			overrides[c.ID] = map[string]any{"runtime": next}
		case !verdicts[c.key]:
			overrides[c.ID] = map[string]any{}
		}
	}
	if len(rec.Cases) == 0 || derived == 0 {
		t.Fatalf("%d cases and %d derived cells carry %s", len(rec.Cases), derived, qualified)
	}
	cells := filepath.Join(t.TempDir(), "cells.json")
	writeJSON(t, cells, overrides)
	casesFile := packetDir + "/" + casesName
	before := snapshot(t, f.root)

	captureFor(t, f.capture, cases, func(int) any { return nil }, func(_ int, r map[string]any) {
		delete(r, "probe")
		r["runtime"], _ = requalify(r["runtime"], "go9.9.9")
	})
	if code, out := f.run(t, rec, "--member", "runtime", "--cell-values", cells); code != 1 || !strings.Contains(out, `captured runtime.goVersion "go9.9.9" is not an admitted substitution of the frozen "go1.27.1"`) {
		t.Fatalf("unadmitted goVersion: exit %d\n%s", code, out)
	}
	if after := snapshot(t, f.root); !bytes.Equal(before[casesFile], after[casesFile]) {
		t.Fatal("a refused re-emission wrote the packet")
	}

	captureFor(t, f.capture, cases, func(int) any { return nil }, func(_ int, r map[string]any) {
		delete(r, "probe")
		r["runtime"], _ = requalify(r["runtime"], "")
	})
	if code, out := f.run(t, rec, "--member", "runtime", "--cell-values", cells); code != 0 {
		t.Fatalf("replace: exit %d\n%s", code, out)
	}
	if code, out := runTool(t, "--root", f.root, "--check"); code != 0 {
		t.Fatalf("check after replace: %s", out)
	}
	after := snapshot(t, f.root)
	// cancelledContextExpectedResult, the original packet's 21-key snapshot in
	// each cell file, is never re-emitted and keeps the pending label.
	for _, rel := range []string{casesFile, packetDir + "/" + inheritedCells, stableDir + "/" + cellsName} {
		if n := bytes.Count(after[rel], []byte(qualified)); n != 0 {
			t.Errorf("%s still carries %s %d times", rel, qualified, n)
		}
		if a, b := bytes.Count(before[rel], []byte(`"go1.27.1"`)), bytes.Count(after[rel], []byte(`"go1.27.1"`)); a != b {
			t.Errorf("%s: frozen go1.27.1 count %d -> %d", rel, a, b)
		}
	}
	if n := bytes.Count(after[casesFile], []byte(pending)); n != len(rec.Cases) {
		t.Fatalf("%s carries %s %d times, want %d", casesName, pending, n, len(rec.Cases))
	}
}
