package main

import (
	"bytes"
	"encoding/json"
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
