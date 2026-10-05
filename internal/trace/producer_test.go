package trace

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The bases and IDs below were computed independently of this package as
// SHA-256 over spec-authored ASCII JSON (LTPM-V0-015).
func TestLTPMV0015ProducerGoldens(t *testing.T) {
	for producer, id := range map[string]string{
		ProducerCLI:     "aa70524b494072760a705416db035298d0fb2826ccf258f08729a2babd4ec314",
		ProducerDogfood: "03d151002d1059594ab0ad80b0e3fa623355207550572d8d0e0be288181182a6",
		ProducerPiTool:  "9c22f327f9f9b31b00a04ea9f4ee7bab2888ecd3cb2b33d0efc51215b38ecde3",
	} {
		t.Run("LTPM-V0-015 "+producer, func(t *testing.T) {
			record, err := NewRecord(Input{Producer: producer, Revision: testRevision, Task: "producer golden", Verification: []string{"go test ./..."}, Outcome: "passed"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			basis := `{"changed_paths":[],"opened_paths":[],"outcome":"passed","producer":"` + producer + `","revision":"0123456789abcdef0123456789abcdef01234567","schema_version":3,"task":"producer golden","verification":["go test ./..."]}`
			got, err := canonicalRecord(record, false)
			if err != nil || string(got) != basis || record.TraceID != id || record.SchemaVersion != SchemaVersionV3 || record.ProducerName() != producer {
				t.Fatalf("basis=%s id=%s record=%+v err=%v", got, record.TraceID, record, err)
			}
			assertStoreRoundTrip(t, record)
		})
	}
	t.Run("LTPM-V0-015 typed verification", func(t *testing.T) {
		record, err := NewRecord(Input{Producer: ProducerPiTool, Revision: testRevision, Task: "typed golden", Outcome: "failed", Verification: []string{"go vet"}, VerificationArgv: [][]string{{"printf", "a b"}}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		const basis = `{"changed_paths":[],"opened_paths":[],"outcome":"failed","producer":"pi-tool","revision":"0123456789abcdef0123456789abcdef01234567","schema_version":3,"task":"typed golden","verification":[{"argv":["printf","a b"],"kind":"argv"},{"command":"go vet","kind":"command"}]}`
		got, err := canonicalRecord(record, false)
		if err != nil || string(got) != basis || record.TraceID != "1706468367fa27a291f91c265c20ce41738873a6b4c06dc5a4dd4f0a6cfc28e7" || record.Verification != nil {
			t.Fatalf("basis=%s id=%s err=%v", got, record.TraceID, err)
		}
		assertStoreRoundTrip(t, record)
	})
}

func assertStoreRoundTrip(t *testing.T, record Record) {
	t.Helper()
	row, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := DecodeStore(row, testRevision, nil)
	if err != nil || len(rows) != 1 || !reflect.DeepEqual(rows[0], record) {
		t.Fatalf("roundtrip: %+v %v", rows, err)
	}
	if typed, err := DecodeTyped(row, testRevision); err != nil || !reflect.DeepEqual(typed, record) {
		t.Fatalf("typed reader: %+v %v", typed, err)
	}
}

func TestLTPMV0015ProducerRefusals(t *testing.T) {
	for _, producer := range []string{"", ProducerUnknown, "CLI", "agent"} {
		if _, err := NewRecord(Input{Producer: producer, Revision: testRevision, Task: "task", Outcome: "passed"}, nil); err == nil {
			t.Errorf("NewRecord admitted producer %q", producer)
		}
	}
	record, err := NewRecord(Input{Producer: ProducerCLI, Revision: testRevision, Task: "producer golden", Verification: []string{"go test ./..."}, Outcome: "passed"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	row, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	mutate := func(change func(map[string]any)) []byte {
		var fields map[string]any
		if err := json.Unmarshal(row, &fields); err != nil {
			t.Fatal(err)
		}
		change(fields)
		encoded, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		return append(encoded, '\n')
	}
	// Re-marshalling the unchanged row is admitted, so each refusal below is
	// caused by its own change.
	if _, err := DecodeStore(mutate(func(map[string]any) {}), testRevision, nil); err != nil {
		t.Fatalf("control row refused: %v", err)
	}
	for name, change := range map[string]func(map[string]any){
		"missing producer":    func(fields map[string]any) { delete(fields, "producer") },
		"stored UNKNOWN":      func(fields map[string]any) { fields["producer"] = ProducerUnknown },
		"unknown producer":    func(fields map[string]any) { fields["producer"] = "agent" },
		"non-string producer": func(fields map[string]any) { fields["producer"] = 1 },
		"schema-2 producer":   func(fields map[string]any) { fields["schema_version"] = 2 },
		"schema-1 producer":   func(fields map[string]any) { fields["schema_version"] = 1 },
		"extra member":        func(fields map[string]any) { fields["model"] = "x" },
		"mixed verification": func(fields map[string]any) {
			fields["verification"] = []any{"go test ./...", map[string]any{"kind": "argv", "argv": []any{"x"}}}
		},
		"typed without argv": func(fields map[string]any) {
			fields["verification"] = []any{map[string]any{"kind": "command", "command": "go test ./..."}}
		},
		"changed producer, id": func(fields map[string]any) { fields["producer"] = ProducerDogfood },
	} {
		if _, err := DecodeStore(mutate(change), testRevision, nil); err == nil {
			t.Errorf("%s: store admitted the row", name)
		}
		if _, err := DecodeTyped(mutate(change), testRevision); err == nil {
			t.Errorf("%s: typed reader admitted the row", name)
		}
	}
}

func TestLTPMV0015LegacyRowsReadAsUnknown(t *testing.T) {
	command, err := NewRecord(Input{Producer: ProducerDogfood, Revision: testRevision, Task: "legacy", Verification: []string{"go test ./..."}, Outcome: "passed"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	typed, err := NewRecord(Input{Producer: ProducerDogfood, Revision: testRevision, Task: "legacy typed", VerificationArgv: [][]string{{"go", "test"}}, Outcome: "passed"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	records := []Record{asLegacy(t, command), asLegacy(t, typed), command}
	var store []byte
	for _, record := range records {
		row, err := Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		store = append(store, row...)
	}
	decoded, err := DecodeStore(store, testRevision, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := RecordProducers(decoded); !reflect.DeepEqual(got, []string{ProducerUnknown, ProducerUnknown, ProducerDogfood}) {
		t.Fatalf("producers %v", got)
	}
	if decoded[0].SchemaVersion != SchemaVersion || decoded[1].SchemaVersion != SchemaVersionV2 || decoded[0].Producer != "" || decoded[1].Producer != "" {
		t.Fatalf("legacy rows changed: %+v", decoded[:2])
	}
	if decoded[0].TraceID == command.TraceID {
		t.Fatal("legacy and schema-3 rows share an identity")
	}
	t.Run("LTPM-V0-016 count and exclude without mutation", func(t *testing.T) {
		before := append([]Record(nil), decoded...)
		counts := CountProducers(RecordProducers(decoded))
		if !reflect.DeepEqual(counts, map[string]int{ProducerCLI: 0, ProducerDogfood: 1, ProducerPiTool: 0, ProducerUnknown: 2}) {
			t.Fatalf("counts %v", counts)
		}
		kept := WithoutProducers(decoded, []string{ProducerUnknown})
		if len(kept) != 1 || kept[0].TraceID != command.TraceID || !reflect.DeepEqual(decoded, before) {
			t.Fatalf("kept %+v, input %+v", kept, decoded)
		}
		if !ReportableProducer(ProducerUnknown) || ReportableProducer("agent") || ValidProducer(ProducerUnknown) {
			t.Fatal("reportable producer set drifted")
		}
	})
}

// LTPM-V0-015: migration re-identifies a row for its commit and keeps the
// producer inside the new identity.
func TestLTPMV0015MigrationKeepsProducer(t *testing.T) {
	record, err := NewRecord(Input{Producer: ProducerPiTool, Revision: testRevision, Task: "migrated producer", Outcome: "passed"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	row, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	const commit = "fedcba9876543210fedcba9876543210fedcba98"
	migrated, err := TransformLegacy(row, testRevision, commit, nil)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := DecodeStore(migrated, commit, nil)
	if err != nil || len(rows) != 1 || rows[0].ProducerName() != ProducerPiTool || rows[0].SchemaVersion != SchemaVersionV3 || rows[0].TraceID == record.TraceID {
		t.Fatalf("migrated %+v %v", rows, err)
	}
}
