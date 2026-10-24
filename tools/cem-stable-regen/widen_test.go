package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const profilePointer = "/$defs/runnerReceipt/properties/profile"

// legacyProfileNode is the sealed runner receipt profile node before V1-2107.
const legacyProfileNode = `{"const":"corvint-test-runner-receipt/0"}`

func widenRun(t *testing.T, root string, rec any, extra ...string) (int, string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "record.json")
	writeJSON(t, file, rec)
	return runTool(t, append([]string{"--root", root, "--widen", "--record", file, "--date", "2026-10-24"}, extra...)...)
}

// profileNode returns the runner receipt profile node of a schema copy.
func profileNode(t *testing.T, src []byte) *jnode {
	t.Helper()
	root, err := parseJSON(src)
	if err != nil {
		t.Fatal(err)
	}
	parent, i, err := resolveMember(root, src, profilePointer)
	if err != nil {
		t.Fatal(err)
	}
	return parent.elems[i]
}

// TestWidenCommittedProfile pins the committed result of the V1-2107 run
// (CEM-V1-621): both schema copies admit exactly the two runner receipt
// labels, and CHANGES.md records the widening from the sealed legacy const.
func TestWidenCommittedProfile(t *testing.T) {
	var copies [][]byte
	for _, dir := range []string{stableDir, packetDir} {
		copies = append(copies, mustRead(t, filepath.Join(repoRoot, dir, schemaName)))
	}
	if !bytes.Equal(copies[0], copies[1]) {
		t.Fatal("the schema copies differ")
	}
	node := profileNode(t, copies[0])
	got, err := enumValues(copies[0], node)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"corvint-test-runner-receipt/0", "opaque/0"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("runner receipt profiles %v, want exactly %v", got, want)
	}
	changes := string(mustRead(t, filepath.Join(repoRoot, packetDir, changesName)))
	for _, want := range []string{
		"# Recorded schema enum widening of `" + profilePointer + "`",
		"- Widened node sha256 (compact JSON): " + digest([]byte(legacyProfileNode)) + " -> " + valueDigest(node.raw(copies[0])) + ".",
	} {
		if !strings.Contains(changes, want) {
			t.Fatalf("CHANGES.md lacks %q", want)
		}
	}
}

// TestWidenRealPacketCopy widens the runner receipt profile node of a copy of
// the real packet (CEM-V1-620): both schema copies change only at that node,
// every case and cell stays byte-identical, the chain checks, and a repeated
// value is refused.
func TestWidenRealPacketCopy(t *testing.T) {
	root := copyProtocol(t)
	before := snapshot(t, root)
	stable, packet := stableDir+"/"+schemaName, packetDir+"/"+schemaName
	oldNode := profileNode(t, before[stable])
	values, err := enumValues(before[stable], oldNode)
	if err != nil {
		t.Fatal(err)
	}
	rec := widenRecord{Reason: "test widening", Pointer: profilePointer, PredecessorSHA256: valueDigest(oldNode.raw(before[stable])), Add: []string{"synthetic/1", "synthetic/2"}}
	if code, out := widenRun(t, root, rec); code != 0 {
		t.Fatalf("widen: exit %d\n%s", code, out)
	}
	after := snapshot(t, root)
	if code, out := runTool(t, "--root", root, "--check"); code != 0 {
		t.Fatalf("check after widen: %s", out)
	}
	if !bytes.Equal(after[stable], after[packet]) {
		t.Fatal("the schema copies differ after widening")
	}
	node := profileNode(t, after[stable])
	var got map[string][]string
	if err := json.Unmarshal(node.raw(after[stable]), &got); err != nil {
		t.Fatal(err)
	}
	want := append(append([]string(nil), values...), "synthetic/1", "synthetic/2")
	if !reflect.DeepEqual(got, map[string][]string{"enum": want}) {
		t.Fatalf("widened node %v, want enum %v", got, want)
	}
	restored := append(append(append([]byte(nil), after[stable][:node.start]...), oldNode.raw(before[stable])...), after[stable][node.end:]...)
	if !bytes.Equal(restored, before[stable]) {
		t.Fatal("the schema changed beyond the widened node")
	}
	for _, rel := range []string{packetDir + "/" + casesName, packetDir + "/" + inheritedCells} {
		if !bytes.Equal(before[rel], after[rel]) {
			t.Fatalf("%s changed", rel)
		}
	}
	manifest := after[packetDir+"/manifest.json"]
	m, _ := parseJSON(manifest)
	if s := m.get("status").text(manifest); s != widenStatus {
		t.Fatalf("status %q, want %q", s, widenStatus)
	}
	if p := m.get("predecessorPublicManifestSHA256").text(manifest); p != digest(before[packetDir+"/manifest.json"]) {
		t.Fatalf("predecessorPublicManifestSHA256 %s does not bind the base manifest", p)
	}
	changes := string(after[packetDir+"/"+changesName])
	for _, want := range []string{
		"# Recorded schema enum widening of `" + profilePointer + "` (2026-10-24)",
		"Reason: test widening",
		"it now admits `" + strings.Join(want, "`, `") + "`",
		"| " + schemaName + " | " + digest(before[stable]) + " | " + digest(after[stable]) + " |",
	} {
		if !strings.Contains(changes, want) {
			t.Fatalf("CHANGES.md lacks %q", want)
		}
	}
	if !bytes.Contains(after[packetDir+"/"+readmeName], []byte(widenPointer)) {
		t.Fatal("README lacks the widening pointer")
	}
	rec = widenRecord{Reason: "repeat", Pointer: profilePointer, PredecessorSHA256: valueDigest(node.raw(after[stable])), Add: []string{"synthetic/1"}}
	if code, out := widenRun(t, root, rec); code != 1 || !strings.Contains(out, "already admits") {
		t.Fatalf("repeat widening: exit %d\n%s", code, out)
	}
}

func TestWidenRefusals(t *testing.T) {
	root := copyProtocol(t)
	src := snapshot(t, root)[stableDir+"/"+schemaName]
	n := profileNode(t, src)
	current, err := enumValues(src, n)
	if err != nil {
		t.Fatal(err)
	}
	ok := widenRecord{Reason: "test", Pointer: profilePointer, PredecessorSHA256: valueDigest(n.raw(src)), Add: []string{"new/0"}}
	with := func(edit func(r *widenRecord)) widenRecord {
		r := ok
		r.Add = append([]string(nil), ok.Add...)
		edit(&r)
		return r
	}
	cases := []struct {
		name string
		rec  any
		want string
	}{
		{"wrong predecessor", with(func(r *widenRecord) { r.PredecessorSHA256 = strings.Repeat("0", 64) }), "the record names predecessor"},
		{"already admitted", with(func(r *widenRecord) { r.Add = []string{current[0]} }), "already admits"},
		{"not a const or enum", with(func(r *widenRecord) { r.Pointer = "/$defs/runnerReceipt/properties/sha256" }), "is not exactly a string const or enum"},
		{"missing member", with(func(r *widenRecord) { r.Pointer = "/$defs/noSuchDef" }), "names no member"},
		{"through an array", with(func(r *widenRecord) { r.Pointer = "/required/0" }), "does not name an object member"},
		{"root pointer", with(func(r *widenRecord) { r.Pointer = "" }), "is not a non-root JSON pointer"},
		{"empty reason", with(func(r *widenRecord) { r.Reason = " " }), "--record has no reason"},
		{"no value", with(func(r *widenRecord) { r.Add = nil }), "--record adds no value"},
		{"repeated value", with(func(r *widenRecord) { r.Add = []string{"new/0", "new/0"} }), "is empty or repeated"},
		{"empty value", with(func(r *widenRecord) { r.Add = []string{""} }), "is empty or repeated"},
		{"unknown record field", map[string]any{"reason": "x", "extra": 1}, "unknown field"},
	}
	before := snapshot(t, root)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if code, out := widenRun(t, root, c.rec); code != 1 || !strings.Contains(out, c.want) {
				t.Fatalf("exit %d, want 1 with %q:\n%s", code, c.want, out)
			}
		})
	}
	for _, extra := range [][]string{{"--member", "x"}, {"--replace"}, {"--schema", "s.json"}, {"--capture", "c.jsonl"}, {"--note", "n"}} {
		if code, out := widenRun(t, root, ok, extra...); code != 2 || !strings.Contains(out, "--widen needs --record") {
			t.Fatalf("--widen with %v: exit %d\n%s", extra, code, out)
		}
	}
	if code, out := runTool(t, "--root", root, "--widen"); code != 2 {
		t.Fatalf("--widen without --record: exit %d\n%s", code, out)
	}
	after := snapshot(t, root)
	for name, b := range before {
		if !bytes.Equal(b, after[name]) {
			t.Fatalf("a refused widening wrote %s", name)
		}
	}
}
