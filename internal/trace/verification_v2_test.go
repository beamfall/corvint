package trace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// LTPM-V0-014: independent spec-authored basis, hashed without the candidate.
func TestLTPMV0014TypedGolden(t *testing.T) {
	t.Run("LTPM-V0-014 independent typed canonical identity", func(t *testing.T) {
		argv := []string{"printf", "a b", "", `{"key":1}`, "|", "é"}
		record, err := NewRecord(Input{Producer: ProducerCLI, Revision: testRevision, Task: "literal arguments", Outcome: "passed", Verification: []string{" go test ./... ", "go test ./..."}, VerificationArgv: [][]string{argv, argv}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		record = asLegacy(t, record)
		const basis = `{"changed_paths":[],"opened_paths":[],"outcome":"passed","revision":"0123456789abcdef0123456789abcdef01234567","schema_version":2,"task":"literal arguments","verification":[{"argv":["printf","a b","","{\"key\":1}","|","\u00e9"],"kind":"argv"},{"command":"go test ./...","kind":"command"}]}`
		got, err := canonicalRecord(record, false)
		if err != nil || string(got) != basis || record.TraceID != "c33abd97f08c6892a4388b2e3d4e204e21319eae66d6dc1072cec57cc6c42eb7" {
			t.Fatalf("basis=%s id=%s err=%v", got, record.TraceID, err)
		}
		row, err := Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := DecodeStore(row, testRevision, nil)
		if err != nil || len(rows) != 1 || !reflect.DeepEqual(rows[0], record) {
			t.Fatalf("roundtrip: %+v %v", rows, err)
		}
		// Transport spelling does not change the decoded identity.
		alternate := bytes.ReplaceAll(row, []byte(`\u00e9`), []byte("é"))
		if _, err := DecodeTyped(alternate, testRevision); err != nil {
			t.Fatal(err)
		}
		argv[1] = "caller mutation"
		if record.TypedVerification[0].Argv[1] != "a b" {
			t.Fatal("retained caller slice")
		}

	})
}

func TestLTPMV0014TypedRefusals(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `[null]`, `["x",null]`, `[""]`, `["x",1]`, `["x"] {}`, `["x","\ud800"]`, `["x","\u0000"]`, `["x","\u0085"]`, `["x","` + strings.Repeat("a", 513) + `"]`} {
		if _, err := ParseVerificationArgv([]byte(raw)); VerificationFailureReason(err) != "unsupported-verify-argv" {
			t.Errorf("admitted %q: %v", raw, err)
		}
	}
	good := []string{"x", "", " a b ", ";", "$(touch absent)", "`literal`", "\\", `{"a":1}`, "😀"}
	if _, err := ParseVerificationArgv(mustJSON(t, good)); err != nil {
		t.Fatal(err)
	}
	tooMany := make([]string, 33)
	tooMany[0] = "x"
	if _, err := ParseVerificationArgv(mustJSON(t, tooMany)); err == nil {
		t.Fatal("33 arguments admitted")
	}
	oversized := []string{"x", strings.Repeat("😀", 512)} // ASCII-escaped canonical vector exceeds4096 bytes.
	if _, err := ParseVerificationArgv(mustJSON(t, oversized)); err == nil {
		t.Fatal("oversized canonical bytes admitted")
	}
	record, err := NewRecord(Input{Producer: ProducerCLI, Revision: testRevision, Task: "task", Outcome: "passed", VerificationArgv: [][]string{{"x"}, {"z"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	record = asLegacy(t, record)
	encoded, _ := Encode(record)
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	for _, verification := range []string{`[{"kind":"argv","argv":["x"],"extra":1}]`, `[{"kind":"argv","kind":"argv","argv":["x"]}]`, `[{"kind":"unknown","argv":["x"]}]`, `[{"kind":"command","command":null}]`, `[{"kind":"argv","argv":["z"]},{"kind":"argv","argv":["x"]}]`, `[{"kind":"argv","argv":["x"]},{"kind":"argv","argv":["x"]}]`} {
		object["verification"] = json.RawMessage(verification)
		bad, _ := json.Marshal(object)
		if _, err := DecodeTyped(bad, testRevision); err == nil {
			t.Errorf("admitted %s", verification)
		}
	}
	bad := bytes.Replace(encoded, []byte(`"schema_version":2`), []byte(`"schema_version":2,"schema_version":2`), 1)
	if _, err := DecodeStore(bad, testRevision, nil); err == nil {
		t.Fatal("duplicate top-level field admitted")
	}
	entries := make([][]string, 51)
	for i := range entries {
		entries[i] = []string{"x"}
	}
	if _, err := NewRecord(Input{Producer: ProducerCLI, Revision: testRevision, Task: "task", Outcome: "passed", VerificationArgv: entries}, nil); err == nil {
		t.Fatal("input entry bound bypassed")
	}
}

// LTA-V0-013: the current screen applies to decoded v2 values on write and read.
func TestLTAV0013TypedSecrets(t *testing.T) {
	t.Run("LTA-V0-013 decoded typed secret screening", func(t *testing.T) {
		for _, argv := range [][]string{{"x", "--token", "literal value"}, {"x", "--api-key=abc123"}, {"curl", "--user", "name:pass word"}, {"docker", "login", "-p", "hunter2"}, {"x", `{"token":"abcd1234"}`}, {"x", "ghp_" + strings.Repeat("a", 20)}, {"x", "https://name:password@host"}, {"x", "bearer", strings.Repeat("a", 20)}, {"x", "AKIA" + strings.Repeat("A", 16), strings.Repeat("b", 40)}} {
			if _, err := NewRecord(Input{Producer: ProducerCLI, Revision: testRevision, Task: "task", Outcome: "passed", VerificationArgv: [][]string{argv}}, nil); err == nil {
				t.Fatalf("writer admitted %+v", argv)
			}
			record := Record{SchemaVersion: 2, Revision: testRevision, Task: "task", Outcome: "passed", OpenedPaths: []string{}, ChangedPaths: []string{}, TypedVerification: []VerificationEntry{{Kind: "argv", Argv: argv}}}
			record.TraceID, _ = traceID(record)
			raw, _ := canonicalRecord(record, true)
			if _, err := DecodeTyped(raw, testRevision); err == nil {
				t.Fatalf("reader admitted %+v", argv)
			}
		}
		if _, err := ParseVerificationArgv([]byte(`["x","--\u0074oken","abc123"]`)); err == nil {
			t.Fatal("escaped credential flag admitted")
		}

	})
}

func TestLTPMV0014MixedMigrationAndNonexecution(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "never-created")
	v1, _ := NewRecord(Input{Producer: ProducerCLI, Revision: testRevision, Task: "old task", Outcome: "passed", Verification: []string{"go test ./..."}}, nil)
	v1 = asLegacy(t, v1)
	v3, err := NewRecord(Input{Producer: ProducerCLI, Revision: testRevision, Task: "new task", Outcome: "passed", VerificationArgv: [][]string{{"sh", "-c", "touch " + sentinel}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	v2 := asLegacy(t, v3)
	v3.Task = "newest task"
	v3.TraceID, _ = traceID(v3)
	a, _ := Encode(v1)
	b, _ := Encode(v2)
	c, _ := Encode(v3)
	mixed := append(append(append([]byte{}, a...), b...), c...)
	rows, err := DecodeStore(mixed, testRevision, nil)
	if err != nil || len(rows) != 3 || rows[2].ProducerName() != ProducerCLI || rows[1].ProducerName() != ProducerUnknown {
		t.Fatalf("mixed: %v", err)
	}
	migrated, err := TransformLegacy(mixed, testRevision, zeroRevision, nil)
	if err != nil {
		t.Fatal(err)
	}
	moved, err := DecodeStore(migrated, zeroRevision, nil)
	if err != nil || moved[1].SchemaVersion != 2 || !reflect.DeepEqual(moved[1].TypedVerification, v2.TypedVerification) || moved[2].SchemaVersion != 3 || moved[2].Producer != ProducerCLI {
		t.Fatalf("migration: %v", err)
	}
	if bytes.Equal(migrated, mixed) {
		t.Fatal("revision identity not rebound")
	}
	if err := validateUnreachableRecord(v2, testRevision); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("argv was executed")
	}
}

// asLegacy rewrites a new schema-3 record to the schema-1 or schema-2 shape it
// carries, without a producer, and reseals its identity. It reproduces rows
// written before schema 3, so their goldens and readers stay pinned.
func asLegacy(t *testing.T, record Record) Record {
	t.Helper()
	record.Producer = ""
	record.SchemaVersion = SchemaVersion
	if record.TypedVerification != nil {
		record.SchemaVersion = SchemaVersionV2
	}
	var err error
	if record.TraceID, err = traceID(record); err != nil {
		t.Fatal(err)
	}
	return record
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func FuzzTypedVerificationRoundTrip(f *testing.F) {
	for _, s := range []string{`["go","test","name with spaces"]`, `["x","",";","\\","😀"]`, `["x","--password","secret1"]`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		argv, err := ParseVerificationArgv([]byte(raw))
		if err != nil {
			return
		}
		record, err := NewRecord(Input{Producer: ProducerCLI, Revision: testRevision, Task: "fuzz task", Outcome: "passed", VerificationArgv: [][]string{argv}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeTyped(encoded, testRevision)
		if err != nil || !reflect.DeepEqual(decoded.TypedVerification, record.TypedVerification) {
			t.Fatalf("roundtrip: %v", err)
		}
	})
}

func TestTypedStoreAppendAndRetention(t *testing.T) {
	root := t.TempDir()
	old := strings.Repeat("b", 40)
	revisions := map[string]Revision{testRevision: {TreeRevision: strings.Repeat("c", 40)}, old: {TreeRevision: strings.Repeat("d", 40), AncestryDistance: 1}}
	var retained []byte
	for i := 0; i < MaxTraces; i++ {
		input := Input{Producer: ProducerCLI, Revision: old, Task: fmt.Sprintf("prior %d", i), Outcome: "passed"}
		if i%2 == 0 {
			input.VerificationArgv = [][]string{{"printf", "a b"}}
		}
		row := mustRecord(t, input, nil)
		raw, err := Encode(row)
		if err != nil {
			t.Fatal(err)
		}
		retained = append(retained, raw...)
	}
	writeTraceFixture(t, root, old, retained)
	store := newTestStore(t, root, revisions, func() error { return nil })
	next := mustRecord(t, Input{Producer: ProducerCLI, Revision: testRevision, Task: "next", Outcome: "passed", VerificationArgv: [][]string{{"printf", "new value"}}}, nil)
	if written, err := store.Append(next); err != nil || !written {
		t.Fatalf("append: %v %v", written, err)
	}
	rows, state, err := store.Read()
	if err != nil || state != StateReady || len(rows) != 1 || rows[0].TraceID != next.TraceID {
		t.Fatalf("retention: %d %s %v", len(rows), state, err)
	}
	if _, err := os.Stat(StorePath(root, old)); !os.IsNotExist(err) {
		t.Fatal("whole old mixed file not evicted")
	}
	legacy := mustRecord(t, Input{Producer: ProducerCLI, Revision: testRevision, Task: "legacy append", Outcome: "passed"}, nil)
	if _, err := store.Append(legacy); err != nil {
		t.Fatal(err)
	}
	rows, _, err = store.Read()
	if err != nil || len(rows) != 2 {
		t.Fatalf("mixed append: %d %v", len(rows), err)
	}
}

func TestTypedUnicodeAndMalformedWireBounds(t *testing.T) {
	for _, n := range []int{512, 513} {
		_, err := ParseVerificationArgv(mustJSON(t, []string{"x", strings.Repeat("é", n)}))
		if (err == nil) != (n == 512) {
			t.Fatalf("rune bound %d: %v", n, err)
		}
	}
	for _, raw := range [][]byte{[]byte("[\"x\",\"\xff\"]"), []byte(`["x","\udc00"]`)} {
		if _, err := ParseVerificationArgv(raw); err == nil {
			t.Fatalf("invalid UTF8/surrogate admitted: %q", raw)
		}
	}
}
