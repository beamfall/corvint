package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const repoRoot = "../.."

func runTool(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out, errs bytes.Buffer
	code := run(args, &out, &errs)
	return code, out.String() + errs.String()
}

// copyProtocol copies the real protocol/cem-1.0 tree into a temporary root.
func copyProtocol(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join(repoRoot, "protocol", "cem-1.0")
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(repoRoot, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(root, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func snapshot(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		files[filepath.ToSlash(rel)], err = os.ReadFile(p)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func writeJSON(t *testing.T, file string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

type frozenCase struct {
	ID             string         `json:"id"`
	ExpectedExit   int            `json:"expectedExit"`
	ExpectedResult map[string]any `json:"expectedResult"`
}

func frozenCases(t *testing.T, root string) []frozenCase {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(packetDir), casesName))
	if err != nil {
		t.Fatal(err)
	}
	var f struct{ Cases []frozenCase }
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f.Cases
}

// captureFor writes a native-harness capture whose frozen keys equal the
// expectation (runtime.goVersion through the go1.27.2 substitution) and whose
// member value is supplied by value; mutate may introduce drift.
func captureFor(t *testing.T, file string, cases []frozenCase, value func(i int) any, mutate func(i int, r map[string]any)) {
	t.Helper()
	var b bytes.Buffer
	for i, c := range cases {
		r := map[string]any{}
		for k, v := range c.ExpectedResult {
			r[k] = v
		}
		if rt, ok := r["runtime"].(map[string]any); ok && rt["goVersion"] == "go1.27.1" {
			copied := map[string]any{}
			for k, v := range rt {
				copied[k] = v
			}
			copied["goVersion"] = "go1.27.2"
			r["runtime"] = copied
		}
		r["probe"] = value(i)
		if mutate != nil {
			mutate(i, r)
		}
		line, _ := json.Marshal(map[string]any{"id": c.ID, "tuple": "go1.27.2", "exit": c.ExpectedExit, "result": r})
		b.Write(append(line, '\n'))
	}
	if err := os.WriteFile(file, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

type frozenCell struct {
	frozenCase
	key string
}

// verdictKey is the cell-to-case matching key: the exit and every frozen key.
func verdictKey(t *testing.T, c frozenCase, members ...string) string {
	t.Helper()
	result := map[string]any{}
	for k, v := range c.ExpectedResult {
		result[k] = v
	}
	for _, m := range members {
		delete(result, m)
	}
	b, err := json.Marshal([]any{c.ExpectedExit, result})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func frozenCells(t *testing.T, root string, members ...string) []frozenCell {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(packetDir), filepath.FromSlash(inheritedCells)))
	if err != nil {
		t.Fatal(err)
	}
	var f struct{ Cases []frozenCase }
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	out := make([]frozenCell, len(f.Cases))
	for i, c := range f.Cases {
		out[i] = frozenCell{c, verdictKey(t, c, members...)}
	}
	return out
}

// cellOverrides writes value(id) for every cell whose verdict matches no case
// and returns the file and the number of matched cells.
func cellOverrides(t *testing.T, root, file string, value func(id string) any, members ...string) (string, int) {
	t.Helper()
	verdicts := map[string]bool{}
	for _, c := range frozenCases(t, root) {
		verdicts[verdictKey(t, c, members...)] = true
	}
	overrides, matched := map[string]any{}, 0
	for _, c := range frozenCells(t, root, members...) {
		if verdicts[c.key] {
			matched++
			continue
		}
		overrides[c.ID] = value(c.ID)
	}
	writeJSON(t, file, overrides)
	return file, matched
}

func probeValue(i int) any {
	if i%2 == 0 {
		return nil
	}
	return map[string]any{"z": i, "a": []any{"x", true}}
}

// strip removes every inserted member from obj-shaped nodes and the inserted
// required entry, which must reproduce the original bytes.
func stripMember(t *testing.T, src []byte, member string, objects []*jnode, arrays []*jnode) []byte {
	t.Helper()
	var edits []splice
	for _, obj := range objects {
		i := obj.index(member)
		if i < 1 {
			t.Fatalf("member %q missing or first", member)
		}
		edits = append(edits, splice{at: obj.elems[i-1].end, end: obj.elems[i].end})
	}
	for _, arr := range arrays {
		for i, e := range arr.elems {
			if e.text(src) == member {
				edits = append(edits, splice{at: arr.elems[i-1].end, end: e.end})
			}
		}
	}
	out, err := applySplices(src, edits)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCheckPassesOnRepositoryPacket(t *testing.T) {
	if code, out := runTool(t, "--check", "--root", repoRoot); code != 0 {
		t.Fatalf("check on the sealed packet failed:\n%s", out)
	}
}

// TestCheckCoversEveryStableManifestEntry drifts stable files that no other
// check reads: --check must name each of them, not only RESULT.schema.json
// (V1-1287).
func TestCheckCoversEveryStableManifestEntry(t *testing.T) {
	for _, name := range []string{"PACKING.md", "tamper-matrix/README.md"} {
		root := copyProtocol(t)
		file := filepath.Join(root, filepath.FromSlash(stableDir+"/"+name))
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		want := stableDir + "/" + name + " differs from its manifest entry"
		if code, out := runTool(t, "--check", "--root", root); code != 1 || !strings.Contains(out, want) {
			t.Fatalf("check on drifted %s: code %d, want %q\n%s", name, code, want, out)
		}
	}
}

func TestRegenerateRealPacketCopy(t *testing.T) {
	root := copyProtocol(t)
	before := snapshot(t, root)
	cases := frozenCases(t, root)
	if len(cases) != 56 {
		t.Fatalf("frozen cases = %d, want 56", len(cases))
	}
	// The sealed packet grows by one key per landed member, so the expected
	// counts follow the frozen shape rather than a fixed number.
	frozen := len(cases[0].ExpectedResult)
	scratch := t.TempDir()
	fragment := filepath.Join(scratch, "fragment.json")
	writeJSON(t, fragment, map[string]any{"type": []any{"object", "null"}})
	capture := filepath.Join(scratch, "capture.jsonl")
	// Cases that share a verdict share a value, as the native verifier's would.
	first := map[string]int{}
	for i, c := range cases {
		if _, ok := first[verdictKey(t, c)]; !ok {
			first[verdictKey(t, c)] = i
		}
	}
	caseProbe := func(i int) any { return probeValue(first[verdictKey(t, cases[i])]) }
	captureFor(t, capture, cases[:50], caseProbe, nil)
	values := filepath.Join(scratch, "values.json")
	overrides := map[string]any{}
	for i, c := range cases[50:] {
		overrides[c.ID] = caseProbe(50 + i)
	}
	writeJSON(t, values, overrides)
	cellValues, matched := cellOverrides(t, root, filepath.Join(scratch, "cells.json"), func(id string) any { return "cell " + id })
	if matched == 0 || matched == 29 {
		t.Fatalf("matched cells = %d, want some but not all of 29", matched)
	}

	code, out := runTool(t, "--root", root, "--member", "probe", "--schema", fragment, "--capture", capture, "--values", values, "--cell-values", cellValues, "--note", "test probe", "--date", "2026-01-02")
	if code != 0 {
		t.Fatalf("regeneration failed:\n%s", out)
	}
	if code, out := runTool(t, "--check", "--root", root); code != 0 || !strings.Contains(out, fmt.Sprintf("56 cases x %d keys", frozen+1)) {
		t.Fatalf("check after regeneration: %d\n%s", code, out)
	}
	after := snapshot(t, root)
	sum := func(files map[string][]byte, name string) string { return digest(files[name]) }
	packetManifest := packetDir + "/manifest.json"
	oldManifest, newManifest := sum(before, packetManifest), sum(after, packetManifest)

	// Every file outside the chain is byte-identical.
	changed := map[string]bool{
		stableDir + "/" + schemaName: true, packetDir + "/" + schemaName: true, packetDir + "/" + casesName: true,
		packetDir + "/" + changesName: true, packetManifest: true,
		stableDir + "/manifest.json": true, stableDir + "/" + cellsName: true, stableDir + "/" + envelopeMD: true,
		packetDir + "/" + inheritedCells:   true,
		tupleRoot + "/go1.27.2/TUPLE.json": true, tupleRoot + "/go1.27.2/manifest.json": true,
		tupleRoot + "/go1.27.3/TUPLE.json": true, tupleRoot + "/go1.27.3/manifest.json": true,
	}
	// The README gains its pointer paragraph only on the packet's first member.
	if !bytes.Contains(before[packetDir+"/"+readmeName], []byte(readmePointer)) {
		changed[packetDir+"/"+readmeName] = true
	}
	for name, b := range before {
		if !changed[name] && !bytes.Equal(b, after[name]) {
			t.Errorf("%s changed outside the regeneration chain", name)
		}
		if changed[name] && bytes.Equal(b, after[name]) {
			t.Errorf("%s was not regenerated", name)
		}
	}

	// Every case carries one more key with the member after artifactChecks and its value.
	casesDoc, err := parseJSON(after[packetDir+"/"+casesName])
	if err != nil {
		t.Fatal(err)
	}
	src := after[packetDir+"/"+casesName]
	var objects []*jnode
	for i, c := range casesDoc.get("cases").elems {
		expected := c.get("expectedResult")
		objects = append(objects, expected)
		if len(expected.keys) != frozen+1 || expected.keys[expected.index("artifactChecks")+1] != "probe" {
			t.Fatalf("case %d keys %v", i, expected.keys)
		}
		got, _ := expected.get("probe").decode(src)
		want, _ := json.Marshal(caseProbe(i))
		have, _ := json.Marshal(got)
		if !bytes.Equal(have, want) {
			t.Errorf("case %d probe = %s, want %s", i, have, want)
		}
	}
	// Every cell carries the value of its matching case, or its override.
	byVerdict := map[string]any{}
	for i, c := range cases {
		byVerdict[verdictKey(t, c)] = caseProbe(i)
	}
	for _, c := range frozenCells(t, root, "probe") {
		want, ok := byVerdict[c.key]
		if !ok {
			want = "cell " + c.ID
		}
		have, _ := json.Marshal(c.ExpectedResult["probe"])
		wantJSON, _ := json.Marshal(want)
		if _, present := c.ExpectedResult["probe"]; !present || !bytes.Equal(have, wantJSON) {
			t.Errorf("cell %s probe = %s, want %s", c.ID, have, wantJSON)
		}
	}
	if got := casesDoc.get("predecessorSHA256").text(src); got != sum(before, packetDir+"/"+casesName) {
		t.Errorf("predecessorSHA256 = %s", got)
	}

	// Removing the member restores the original bytes.
	stripped := stripMember(t, src, "probe", objects, nil)
	originalCases := before[packetDir+"/"+casesName]
	oldDoc, _ := parseJSON(originalCases)
	stripped = bytes.Replace(stripped, []byte(sum(before, packetDir+"/"+casesName)), []byte(oldDoc.get("predecessorSHA256").text(originalCases)), 1)
	stripped = bytes.Replace(stripped, []byte(fmt.Sprintf("closed%d-key", frozen+1)), []byte(fmt.Sprintf("closed%d-key", frozen)), 1)
	if !bytes.Equal(stripped, originalCases) {
		t.Errorf("%s without the member differs from the original", casesName)
	}
	schemaSrc := after[stableDir+"/"+schemaName]
	schemaDoc, _ := parseJSON(schemaSrc)
	if !bytes.Equal(stripMember(t, schemaSrc, "probe", []*jnode{schemaDoc.get("properties")}, []*jnode{schemaDoc.get("required")}), before[stableDir+"/"+schemaName]) {
		t.Errorf("%s without the member differs from the original", schemaName)
	}
	if !bytes.Equal(schemaSrc, after[packetDir+"/"+schemaName]) {
		t.Errorf("schema copies differ")
	}

	// Predecessor digests are recorded and every binding names the new manifest.
	manifestDoc, _ := parseJSON(after[packetManifest])
	if got := manifestDoc.get("predecessorPublicManifestSHA256").text(after[packetManifest]); got != oldManifest {
		t.Errorf("predecessorPublicManifestSHA256 = %s, want %s", got, oldManifest)
	}
	if got := manifestDoc.get("status").text(after[packetManifest]); got != regenStatus {
		t.Errorf("status = %s", got)
	}
	tupleSrc := after[tupleRoot+"/go1.27.2/TUPLE.json"]
	tupleDoc, _ := parseJSON(tupleSrc)
	inherited := tupleDoc.get("inherited")
	if inherited.get("manifestSha256").text(tupleSrc) != newManifest || inherited.get("predecessorManifestSha256").text(tupleSrc) != oldManifest {
		t.Errorf("TUPLE.json inherited binding not re-bound: %s", inherited.raw(tupleSrc))
	}
	changes := string(after[packetDir+"/"+changesName])
	for _, want := range []string{"# Additive member `probe` (2026-01-02)", oldManifest, "test probe", "on go1.27.2 for 50 cases", "for: " + cases[50].ID, "The case count stays 56.", fmt.Sprintf("%d cells take the value", matched), "Cell values from the cell override file"} {
		if !strings.Contains(changes, want) {
			t.Errorf("CHANGES.md lacks %q", want)
		}
	}
	if !bytes.HasPrefix(after[packetDir+"/"+changesName], before[packetDir+"/"+changesName]) {
		t.Errorf("CHANGES.md history was not preserved")
	}

	// A second member stacks on the first and keeps the README pointer single.
	cellValues2, _ := cellOverrides(t, root, filepath.Join(scratch, "cells2.json"), func(string) any { return nil }, "probe2")
	if code, out := runTool(t, "--root", root, "--member", "probe2", "--after", "probe", "--schema", fragment, "--values", writeAll(t, scratch, cases), "--cell-values", cellValues2, "--note", "second", "--date", "2026-01-03"); code != 0 {
		t.Fatalf("second regeneration failed:\n%s", out)
	}
	final := snapshot(t, root)
	if n := bytes.Count(final[packetDir+"/"+readmeName], []byte(readmePointer)); n != 1 {
		t.Errorf("README pointer count = %d", n)
	}
	finalTuple, _ := parseJSON(final[tupleRoot+"/go1.27.2/TUPLE.json"])
	if got := finalTuple.get("inherited").get("predecessorManifestSha256").text(final[tupleRoot+"/go1.27.2/TUPLE.json"]); got != newManifest {
		t.Errorf("second predecessorManifestSha256 = %s, want %s", got, newManifest)
	}
	if code, out := runTool(t, "--check", "--root", root); code != 0 || !strings.Contains(out, fmt.Sprintf("x %d keys", frozen+2)) {
		t.Fatalf("check after second regeneration: %d\n%s", code, out)
	}
}

func writeAll(t *testing.T, dir string, cases []frozenCase) string {
	t.Helper()
	all := map[string]any{}
	for _, c := range cases {
		all[c.ID] = nil
	}
	file := filepath.Join(dir, "all.json")
	writeJSON(t, file, all)
	return file
}

func TestRegenerateRefusals(t *testing.T) {
	root := copyProtocol(t)
	cases := frozenCases(t, root)
	scratch := t.TempDir()
	fragment := filepath.Join(scratch, "fragment.json")
	writeJSON(t, fragment, map[string]any{"type": "null"})
	before := snapshot(t, root)
	capture := func(name string, mutate func(i int, r map[string]any)) string {
		file := filepath.Join(scratch, name)
		captureFor(t, file, cases, func(int) any { return nil }, mutate)
		return file
	}
	missing := map[string]any{}
	for _, c := range cases[1:] {
		missing[c.ID] = nil
	}
	missingFile := filepath.Join(scratch, "missing.json")
	writeJSON(t, missingFile, missing)
	exitDrift := capture("exit.jsonl", nil)
	lines, _ := os.ReadFile(exitDrift)
	lines = bytes.Replace(lines, []byte(fmt.Sprintf(`{"exit":%d,`, cases[0].ExpectedExit)), []byte(`{"exit":99,`), 1)
	if err := os.WriteFile(exitDrift, lines, 0o644); err != nil {
		t.Fatal(err)
	}
	both := filepath.Join(scratch, "both.json")
	writeJSON(t, both, map[string]any{cases[3].ID: nil})
	cellValues, _ := cellOverrides(t, root, filepath.Join(scratch, "cells.json"), func(string) any { return nil })
	var cellBoth map[string]any
	b, _ := os.ReadFile(cellValues)
	if err := json.Unmarshal(b, &cellBoth); err != nil {
		t.Fatal(err)
	}
	cellBoth["ordinary-sha1"] = nil
	cellBothFile := filepath.Join(scratch, "cell-both.json")
	writeJSON(t, cellBothFile, cellBoth)
	delete(cellBoth, "ordinary-sha1")
	cellBoth["no-such-cell"] = nil
	alternating := filepath.Join(scratch, "alternating.jsonl")
	captureFor(t, alternating, cases, probeValue, nil)
	cellUnknownFile := filepath.Join(scratch, "cell-unknown.json")
	writeJSON(t, cellUnknownFile, cellBoth)
	duplicate := capture("duplicate.jsonl", nil)
	lines, _ = os.ReadFile(duplicate)
	if err := os.WriteFile(duplicate, bytes.Replace(lines, []byte(`"result":{`), []byte(`"result":{"outcome":"ACCEPT",`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	overflow := capture("overflow.jsonl", nil)
	lines, _ = os.ReadFile(overflow)
	lines = bytes.Replace(lines, []byte(`"accept":true`), []byte(`"accept":1e999`), 1)
	lines = bytes.Replace(lines, []byte(`"accept":false`), []byte(`"accept":1e999`), 1)
	if err := os.WriteFile(overflow, lines, 0o644); err != nil {
		t.Fatal(err)
	}
	numericFragment := filepath.Join(scratch, "seven.json")
	writeJSON(t, numericFragment, 7)

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"drift", []string{"--member", "probe", "--capture", capture("drift.jsonl", func(i int, r map[string]any) {
			if i == 5 {
				r["outcome"] = "ACCEPT"
			}
		})}, `verdict drift in frozen key "outcome"`},
		{"exit drift", []string{"--member", "probe", "--capture", exitDrift}, "verdict drift: exit 99"},
		{"capture and override", []string{"--member", "probe", "--capture", capture("both.jsonl", nil), "--values", both}, "both a captured value and a --values override"},
		{"extra key", []string{"--member", "probe", "--capture", capture("extra.jsonl", func(i int, r map[string]any) { r["other"] = 1 })}, `key "other" that is neither frozen nor --member`},
		{"missing value", []string{"--member", "probe", "--values", missingFile}, "has no captured value and no --values override"},
		{"existing member", []string{"--member", "runtime", "--values", writeAll(t, scratch, cases)}, `member "runtime" already exists`},
		{"unknown after", []string{"--member", "probe", "--after", "nope", "--values", writeAll(t, scratch, cases)}, `no member "nope"`},
		{"cases disagree for a cell", []string{"--member", "probe", "--capture", alternating, "--cell-values", cellValues}, `cases with its frozen verdict disagree on "probe"`},
		{"cell without value", []string{"--member", "probe", "--values", writeAll(t, scratch, cases)}, "matches no case's frozen verdict and has no --cell-values override"},
		{"cell match and override", []string{"--member", "probe", "--values", writeAll(t, scratch, cases), "--cell-values", cellBothFile}, "cell ordinary-sha1 matches a case's frozen verdict and also has a --cell-values override"},
		{"unknown cell", []string{"--member", "probe", "--values", writeAll(t, scratch, cases), "--cell-values", cellUnknownFile}, "--cell-values names unknown cell no-such-cell"},
		{"duplicate capture key", []string{"--member", "probe", "--capture", duplicate}, `duplicate key "outcome"`},
		{"captured number out of range", []string{"--member", "probe", "--capture", overflow}, `captured key "accept"`},
		{"numeric schema fragment", []string{"--member", "probe", "--schema", numericFragment, "--values", writeAll(t, scratch, cases)}, "schema fragment must be a JSON Schema object"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"--root", root, "--schema", fragment, "--note", "refusal"}, tc.args...)
			code, out := runTool(t, args...)
			if code != 1 || !strings.Contains(out, tc.want) {
				t.Fatalf("code %d, output:\n%s\nwant %q", code, out, tc.want)
			}
		})
	}
	for name, b := range snapshot(t, root) {
		if !bytes.Equal(b, before[name]) {
			t.Errorf("a refused regeneration wrote %s", name)
		}
	}

	// A packet that no longer matches its seal is refused before any edit.
	full := filepath.Join(root, filepath.FromSlash(packetDir), casesName)
	b = before[packetDir+"/"+casesName]
	if err := os.WriteFile(full, bytes.Replace(b, []byte(`"UNSUPPORTED"`), []byte(`"ACCEPTED"`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	stableCells := filepath.Join(root, filepath.FromSlash(stableDir), cellsName)
	if err := os.WriteFile(stableCells, bytes.Replace(before[stableDir+"/"+cellsName], []byte(`"UNSUPPORTED"`), []byte(`"ACCEPTED"`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := runTool(t, "--root", root, "--member", "probe", "--schema", fragment, "--note", "x", "--values", writeAll(t, scratch, cases)); code != 1 || !strings.Contains(out, "not self-consistent") {
		t.Fatalf("tampered packet: code %d\n%s", code, out)
	}
	if code, out := runTool(t, "--check", "--root", root); code != 1 || !strings.Contains(out, "differs from its manifest entry") || !strings.Contains(out, "list different cells") {
		t.Fatalf("check on tampered packet: code %d\n%s", code, out)
	}
	// A manifest that no longer lists a mandatory file leaves it unsealed.
	stableManifest := filepath.Join(root, filepath.FromSlash(stableDir), "manifest.json")
	var whole map[string]any
	b, _ = os.ReadFile(stableManifest)
	if err := json.Unmarshal(b, &whole); err != nil {
		t.Fatal(err)
	}
	var kept []any
	for _, entry := range whole["files"].([]any) {
		if entry.(map[string]any)["path"] != schemaName {
			kept = append(kept, entry)
		}
	}
	whole["files"] = kept
	writeJSON(t, stableManifest, whole)
	if code, out := runTool(t, "--check", "--root", root); code != 1 || !strings.Contains(out, "manifest.json does not list "+schemaName) {
		t.Fatalf("check on unlisted schema: code %d\n%s", code, out)
	}
}

// TestRegenerateSyntheticPacket builds a minimal two-case chain from scratch,
// so the tool's contract does not depend on the shape of the sealed packet.
func TestRegenerateSyntheticPacket(t *testing.T) {
	root := t.TempDir()
	stable := filepath.Join(root, filepath.FromSlash(stableDir))
	packet := filepath.Join(root, filepath.FromSlash(packetDir))
	tuple := filepath.Join(root, filepath.FromSlash(tupleRoot), "goX")
	for _, dir := range []string{filepath.Join(packet, "inherited", "s0e"), tuple} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(file, text string) []byte {
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return []byte(text)
	}
	entry := func(name string, b []byte) string {
		return fmt.Sprintf(`{"path": %q, "sha256": %q, "bytes": %d}`, name, digest(b), len(b))
	}
	schema := "{\n  \"required\": [\n    \"accept\",\n    \"runtime\"\n  ],\n  \"properties\": {\n    \"accept\": {\n      \"type\": \"boolean\"\n    },\n    \"runtime\": {\n      \"type\": \"object\"\n    }\n  }\n}\n"
	s1 := write(filepath.Join(stable, schemaName), schema)
	write(filepath.Join(packet, schemaName), schema)
	cases := write(filepath.Join(packet, casesName), "{\n  \"cases\": [\n    {\n      \"id\": \"a\",\n      \"expectedExit\": 0,\n      \"expectedResult\": {\n        \"accept\": true,\n        \"runtime\": {\n          \"goVersion\": \"go1\"\n        }\n      }\n    },\n    {\n      \"id\": \"b\",\n      \"expectedExit\": 2,\n      \"expectedResult\": {\n        \"accept\": false,\n        \"runtime\": {}\n      }\n    }\n  ],\n  \"comparison\": \"closed2-key\",\n  \"predecessorSHA256\": \"none\"\n}\n")
	readme := write(filepath.Join(packet, readmeName), "# Packet\n")
	changes := write(filepath.Join(packet, changesName), "# Changes\n")
	cellList := func(c1, c2 string) string {
		return "  \"cases\": [\n    {\n      \"id\": \"c1\",\n      \"expectedExit\": 0,\n      \"expectedResult\": {\n        \"accept\": true,\n" + c1 + "        \"runtime\": {\n          \"goVersion\": \"go1\"\n        }\n      }\n    },\n    {\n      \"id\": \"c2\",\n      \"expectedExit\": 2,\n      \"expectedResult\": {\n        \"accept\": false,\n" + c2 + "        \"runtime\": {\n          \"x\": 1\n        }\n      }\n    }\n  ]"
	}
	inheritedCellsFile := filepath.Join(packet, filepath.FromSlash(inheritedCells))
	packetCells := write(inheritedCellsFile, "{\n"+cellList("", "")+"\n}\n")
	manifest := write(filepath.Join(packet, "manifest.json"), "{\n  \"status\": \"SEALED\",\n  \"predecessorPublicManifestSHA256\": \"none\",\n  \"files\": ["+strings.Join([]string{entry(schemaName, s1), entry(casesName, cases), entry(inheritedCells, packetCells), entry(readmeName, readme), entry(changesName, changes)}, ", ")+"]\n}\n")
	cells := write(filepath.Join(stable, cellsName), fmt.Sprintf("{\n%s,\n  \"amendment\": {\n    \"publicManifestSHA256\": %q,\n    \"fullResultCases\": {\n      \"sha256\": %q,\n      \"bytes\": %d,\n      \"count\": 2\n    },\n    \"note\": \"The complete 2-field expectation.\"\n  }\n}\n", cellList("", ""), digest(manifest), digest(cases), len(cases)))
	prose := write(filepath.Join(stable, envelopeMD), "manifest "+digest(manifest)+" cases "+digest(cases)+"\n")
	write(filepath.Join(stable, "manifest.json"), fmt.Sprintf("{\n  \"publicAmendmentManifestSHA256\": %q,\n  \"files\": [%s, %s, %s]\n}\n", digest(manifest), entry(schemaName, s1), entry(cellsName, cells), entry(envelopeMD, prose)))
	tupleJSON := write(filepath.Join(tuple, "TUPLE.json"), fmt.Sprintf("{\n  \"inherited\": {\n    \"manifestSha256\": %q,\n    \"files\": {\n      %q: %q\n    }\n  },\n  \"substitution\": {\n    \"field\": \"expectedResult.runtime.goVersion\",\n    \"from\": \"go1\",\n    \"to\": \"goX\"\n  }\n}\n", digest(manifest), casesName, digest(cases)))
	write(filepath.Join(tuple, "manifest.json"), "{\n  \"files\": ["+entry("TUPLE.json", tupleJSON)+"]\n}\n")
	if code, out := runTool(t, "--check", "--root", root); code != 0 || !strings.Contains(out, "2 cases x 2 keys") {
		t.Fatalf("synthetic check: %d\n%s", code, out)
	}

	scratch := t.TempDir()
	fragment := filepath.Join(scratch, "fragment.json")
	write(fragment, `{"enum": [1, null]}`)
	capture := filepath.Join(scratch, "capture.jsonl")
	write(capture, `{"id":"a","tuple":"goX","exit":0,"result":{"accept":true,"note":1,"runtime":{"goVersion":"goX"}}}`+"\n"+`{"id":"a","tuple":"goX","exit":0,"result":{"accept":true,"note":1,"runtime":{"goVersion":"goX"}}}`+"\n")
	values := filepath.Join(scratch, "values.json")
	write(values, `{"b": null}`)
	cellValues := filepath.Join(scratch, "cells.json")
	write(cellValues, `{"c2": null}`)
	if code, out := runTool(t, "--root", root, "--member", "note", "--after", "accept", "--schema", fragment, "--capture", capture, "--values", values, "--cell-values", cellValues, "--note", "synthetic"); code != 0 {
		t.Fatalf("synthetic regeneration: %d\n%s", code, out)
	}
	gotPacketCells, _ := os.ReadFile(inheritedCellsFile)
	if want := "{\n" + cellList("        \"note\": 1,\n", "        \"note\": null,\n") + "\n}\n"; string(gotPacketCells) != want {
		t.Errorf("synthetic inherited cells:\n%s\nwant:\n%s", gotPacketCells, want)
	}
	got, _ := os.ReadFile(filepath.Join(packet, casesName))
	wantCases := "{\n  \"cases\": [\n    {\n      \"id\": \"a\",\n      \"expectedExit\": 0,\n      \"expectedResult\": {\n        \"accept\": true,\n        \"note\": 1,\n        \"runtime\": {\n          \"goVersion\": \"go1\"\n        }\n      }\n    },\n    {\n      \"id\": \"b\",\n      \"expectedExit\": 2,\n      \"expectedResult\": {\n        \"accept\": false,\n        \"note\": null,\n        \"runtime\": {}\n      }\n    }\n  ],\n  \"comparison\": \"closed3-key\",\n  \"predecessorSHA256\": \"" + digest(cases) + "\"\n}\n"
	if string(got) != wantCases {
		t.Errorf("synthetic cases:\n%s\nwant:\n%s", got, wantCases)
	}
	gotSchema, _ := os.ReadFile(filepath.Join(stable, schemaName))
	wantSchema := "{\n  \"required\": [\n    \"accept\",\n    \"note\",\n    \"runtime\"\n  ],\n  \"properties\": {\n    \"accept\": {\n      \"type\": \"boolean\"\n    },\n    \"note\": {\n      \"enum\": [\n        1,\n        null\n      ]\n    },\n    \"runtime\": {\n      \"type\": \"object\"\n    }\n  }\n}\n"
	if string(gotSchema) != wantSchema {
		t.Errorf("synthetic schema:\n%s\nwant:\n%s", gotSchema, wantSchema)
	}
	if code, out := runTool(t, "--check", "--root", root); code != 0 || !strings.Contains(out, "2 cases x 3 keys") {
		t.Fatalf("synthetic check after regeneration: %d\n%s", code, out)
	}
	gotCells, _ := os.ReadFile(filepath.Join(stable, cellsName))
	if !bytes.Contains(gotCells, []byte("complete 3-field expectation")) {
		t.Errorf("cells note not bumped:\n%s", gotCells)
	}
}
