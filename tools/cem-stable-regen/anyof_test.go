package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const bindingPointer = "/$defs/criterionBinding"

// committedAnyOfRecord is the record of the V1-2615 run (CEM-V1-881).
var committedAnyOfRecord = filepath.Join("testdata", "v1-2615-any-of-record.json")

func anyOfRun(t *testing.T, root string, rec any, extra ...string) (int, string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "record.json")
	writeJSON(t, file, rec)
	return runTool(t, append([]string{"--root", root, "--any-of", "--record", file, "--date", "2026-10-31"}, extra...)...)
}

// schemaNode returns the node a pointer names in a schema copy.
func schemaNode(t *testing.T, src []byte, pointer string) *jnode {
	t.Helper()
	root, err := parseJSON(src)
	if err != nil {
		t.Fatal(err)
	}
	parent, i, err := resolveMember(root, src, pointer)
	if err != nil {
		t.Fatal(err)
	}
	return parent.elems[i]
}

// TestAnyOfCommittedCriterionBinding pins the committed result of the V1-2615
// run (CEM-V1-881): both schema copies hold criterionBinding as anyOf the
// unchanged sealed Tasks form and the recorded plain form, and CHANGES.md
// records the run from the sealed Tasks-only node.
func TestAnyOfCommittedCriterionBinding(t *testing.T) {
	var copies [][]byte
	for _, dir := range []string{stableDir, packetDir} {
		copies = append(copies, mustRead(t, filepath.Join(repoRoot, dir, schemaName)))
	}
	if !bytes.Equal(copies[0], copies[1]) {
		t.Fatal("the schema copies differ")
	}
	var rec anyOfRecord
	if err := json.Unmarshal(mustRead(t, committedAnyOfRecord), &rec); err != nil {
		t.Fatal(err)
	}
	node := schemaNode(t, copies[0], bindingPointer)
	alts, wrap, err := alternatives(node)
	if err != nil || wrap || len(alts) != 2 {
		t.Fatalf("criterionBinding alternatives %d (wrap %v, %v), want anyOf of two", len(alts), wrap, err)
	}
	if got := valueDigest(alts[0].raw(copies[0])); got != rec.PredecessorSHA256 {
		t.Fatalf("the first alternative %s is not the sealed Tasks form %s", got, rec.PredecessorSHA256)
	}
	if got, want := valueDigest(alts[1].raw(copies[0])), valueDigest(rec.Branch); got != want {
		t.Fatalf("the second alternative %s is not the recorded branch %s", got, want)
	}
	var plain struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(alts[1].raw(copies[0]), &plain); err != nil {
		t.Fatal(err)
	}
	if want := []string{"id", "criterionIndex", "criterionSha256", "evidenceId"}; !reflect.DeepEqual(plain.Required, want) {
		t.Fatalf("plain form requires %v, want %v", plain.Required, want)
	}
	changes := string(mustRead(t, filepath.Join(repoRoot, packetDir, changesName)))
	for _, want := range []string{
		"# Recorded schema anyOf branch at `" + bindingPointer + "`",
		"Record file sha256: " + digest(mustRead(t, committedAnyOfRecord)) + ".",
		"- Node sha256 (compact JSON): " + rec.PredecessorSHA256 + " -> " + valueDigest(node.raw(copies[0])) + "; branch sha256 " + valueDigest(rec.Branch) + ".",
	} {
		if !strings.Contains(changes, want) {
			t.Fatalf("CHANGES.md lacks %q", want)
		}
	}
}

// TestAnyOfRealPacketCopy adds branches to a copy of the real packet
// (CEM-V1-880): wrapping a plain object node keeps its bytes, only
// re-indented, appending to an existing anyOf keeps every alternative byte,
// both copies change only at the node, every case and cell stays
// byte-identical, the chain checks, and a repeated branch is refused.
func TestAnyOfRealPacketCopy(t *testing.T) {
	root := copyProtocol(t)
	stable, packet := stableDir+"/"+schemaName, packetDir+"/"+schemaName
	branch := json.RawMessage(`{"type":"object","required":["synthetic"],"properties":{"synthetic":{"const":"x"}}}`)
	for _, c := range []struct {
		pointer string
		wrap    bool
	}{
		{"/$defs/criterionLink", true},
		{bindingPointer, false},
	} {
		before := snapshot(t, root)
		old := schemaNode(t, before[stable], c.pointer)
		oldAlts, wrap, err := alternatives(old)
		if err != nil || wrap != c.wrap {
			t.Fatalf("%s: wrap %v (%v), want %v", c.pointer, wrap, err, c.wrap)
		}
		rec := anyOfRecord{Reason: "test branch", Pointer: c.pointer, PredecessorSHA256: valueDigest(old.raw(before[stable])), Branch: branch}
		if code, out := anyOfRun(t, root, rec); code != 0 {
			t.Fatalf("%s: exit %d\n%s", c.pointer, code, out)
		}
		after := snapshot(t, root)
		if code, out := runTool(t, "--root", root, "--check"); code != 0 {
			t.Fatalf("check after %s: %s", c.pointer, out)
		}
		if !bytes.Equal(after[stable], after[packet]) {
			t.Fatal("the schema copies differ")
		}
		node := schemaNode(t, after[stable], c.pointer)
		alts, wrapAfter, err := alternatives(node)
		if err != nil || wrapAfter || len(alts) != len(oldAlts)+1 {
			t.Fatalf("%s: %d alternatives (%v), want %d", c.pointer, len(alts), err, len(oldAlts)+1)
		}
		for k, a := range oldAlts {
			want := string(a.raw(before[stable]))
			if c.wrap {
				want = strings.ReplaceAll(want, "\n", "\n    ")
			}
			if got := string(alts[k].raw(after[stable])); got != want {
				t.Fatalf("%s: alternative %d changed:\n%s\nwant\n%s", c.pointer, k, got, want)
			}
		}
		if got := valueDigest(alts[len(alts)-1].raw(after[stable])); got != valueDigest(branch) {
			t.Fatalf("%s: last alternative %s is not the branch", c.pointer, got)
		}
		if !bytes.Equal(after[stable][:node.start], before[stable][:old.start]) || !bytes.Equal(after[stable][node.end:], before[stable][old.end:]) {
			t.Fatalf("%s: the schema changed beyond the node", c.pointer)
		}
		for _, rel := range []string{packetDir + "/" + casesName, packetDir + "/" + inheritedCells} {
			if !bytes.Equal(before[rel], after[rel]) {
				t.Fatalf("%s changed", rel)
			}
		}
		manifest := after[packetDir+"/manifest.json"]
		m, _ := parseJSON(manifest)
		if s := m.get("status").text(manifest); s != anyOfStatus {
			t.Fatalf("status %q, want %q", s, anyOfStatus)
		}
		changes := string(after[packetDir+"/"+changesName])
		for _, want := range []string{
			"# Recorded schema anyOf branch at `" + c.pointer + "` (2026-10-31)",
			"Reason: test branch",
			"| " + schemaName + " | " + digest(before[stable]) + " | " + digest(after[stable]) + " |",
		} {
			if !strings.Contains(changes, want) {
				t.Fatalf("CHANGES.md lacks %q", want)
			}
		}
		if bytes.Count(after[packetDir+"/"+readmeName], []byte(anyOfPointer)) != 1 {
			t.Fatal("README does not carry the anyOf pointer exactly once")
		}
		rec = anyOfRecord{Reason: "repeat", Pointer: c.pointer, PredecessorSHA256: valueDigest(node.raw(after[stable])), Branch: branch}
		if code, out := anyOfRun(t, root, rec); code != 1 || !strings.Contains(out, "already admits the recorded branch") {
			t.Fatalf("%s repeat: exit %d\n%s", c.pointer, code, out)
		}
	}
}

func TestAnyOfRefusals(t *testing.T) {
	root := copyProtocol(t)
	src := snapshot(t, root)[stableDir+"/"+schemaName]
	n := schemaNode(t, src, bindingPointer)
	alts, _, err := alternatives(n)
	if err != nil {
		t.Fatal(err)
	}
	ok := anyOfRecord{Reason: "test", Pointer: bindingPointer, PredecessorSHA256: valueDigest(n.raw(src)), Branch: json.RawMessage(`{"type":"null"}`)}
	with := func(edit func(r *anyOfRecord)) anyOfRecord {
		r := ok
		edit(&r)
		return r
	}
	cases := []struct {
		name string
		rec  any
		want string
	}{
		{"wrong predecessor", with(func(r *anyOfRecord) { r.PredecessorSHA256 = strings.Repeat("0", 64) }), "the record names predecessor"},
		{"existing alternative", with(func(r *anyOfRecord) { r.Branch = append(json.RawMessage(nil), alts[0].raw(src)...) }), "already admits the recorded branch"},
		{"not an object node", with(func(r *anyOfRecord) { r.Pointer = "/$defs/criterionId/pattern" }), "is not an object schema"},
		{"missing member", with(func(r *anyOfRecord) { r.Pointer = "/$defs/noSuchDef" }), "names no member"},
		{"through an array", with(func(r *anyOfRecord) { r.Pointer = "/required/0" }), "does not name an object member"},
		{"root pointer", with(func(r *anyOfRecord) { r.Pointer = "" }), "is not a non-root JSON pointer"},
		{"empty reason", with(func(r *anyOfRecord) { r.Reason = " " }), "--record has no reason"},
		{"no branch", map[string]any{"reason": "x", "pointer": bindingPointer, "predecessorSha256": ok.PredecessorSHA256}, "--record branch is not a non-empty JSON object schema"},
		{"empty branch", with(func(r *anyOfRecord) { r.Branch = json.RawMessage(`{}`) }), "--record branch is not a non-empty JSON object schema"},
		{"array branch", with(func(r *anyOfRecord) { r.Branch = json.RawMessage(`[{"type":"null"}]`) }), "--record branch is not a non-empty JSON object schema"},
		{"unknown record field", map[string]any{"reason": "x", "extra": 1}, "unknown field"},
	}
	before := snapshot(t, root)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if code, out := anyOfRun(t, root, c.rec); code != 1 || !strings.Contains(out, c.want) {
				t.Fatalf("exit %d, want 1 with %q:\n%s", code, c.want, out)
			}
		})
	}
	// An anyOf beside another keyword is refused at the node-shape check.
	if _, _, err := alternatives(mustNode(t, `{"anyOf":[{"type":"null"}],"type":"object"}`)); err == nil || !strings.Contains(err.Error(), "anyOf beside another keyword") {
		t.Fatalf("anyOf beside a keyword: %v", err)
	}
	if _, _, err := alternatives(mustNode(t, `{"anyOf":[]}`)); err == nil || !strings.Contains(err.Error(), "not a non-empty array") {
		t.Fatalf("empty anyOf: %v", err)
	}
	for _, extra := range [][]string{{"--member", "x"}, {"--replace"}, {"--widen"}, {"--schema", "s.json"}, {"--capture", "c.jsonl"}, {"--note", "n"}} {
		if code, out := anyOfRun(t, root, ok, extra...); code != 2 || !strings.Contains(out, "--any-of needs --record") {
			t.Fatalf("--any-of with %v: exit %d\n%s", extra, code, out)
		}
	}
	if code, out := runTool(t, "--root", root, "--any-of"); code != 2 {
		t.Fatalf("--any-of without --record: exit %d\n%s", code, out)
	}
	after := snapshot(t, root)
	for name, b := range before {
		if !bytes.Equal(b, after[name]) {
			t.Fatalf("a refused branch wrote %s", name)
		}
	}
}

func mustNode(t *testing.T, src string) *jnode {
	t.Helper()
	n, err := parseJSON([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return n
}
