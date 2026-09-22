package trace

import (
	"bytes"
	"reflect"
	"testing"
)

// FuzzDecodeStoreRoundTripsEncode feeds arbitrary store bytes to DecodeStore.
// It must refuse rather than panic, and every store it accepts must re-encode
// row by row to a store DecodeStore accepts as the same records, whose own
// encoding is byte-identical: a verified row is a fixed point of Encode.
func FuzzDecodeStoreRoundTripsEncode(f *testing.F) {
	tracked := []string{"a.go", "z.go"}
	inputs := []Input{
		{Revision: testRevision, Task: "café ☃ \U0001F600 <&>", OpenedPaths: []string{"z.go", "a.go"},
			ChangedPaths: []string{"a.go"}, Verification: []string{"go test ./...", "git diff --check"}, Outcome: "passed"},
		{Revision: testRevision, Task: "edge \x7f \b\f\n\r\t end", Outcome: "blocked"},
	}
	var store []byte
	for _, input := range inputs {
		record, err := NewRecord(input, tracked)
		if err != nil {
			f.Fatal(err)
		}
		row, err := Encode(record)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(row)
		store = append(store, row...)
	}
	f.Add(store)
	f.Fuzz(func(t *testing.T, data []byte) {
		records, err := DecodeStore(data, testRevision, tracked)
		if err != nil {
			return
		}
		var encoded []byte
		for _, record := range records {
			row, err := Encode(record)
			if err != nil {
				t.Fatalf("decoded record %+v from %q does not encode: %v", record, data, err)
			}
			encoded = append(encoded, row...)
		}
		again, err := DecodeStore(encoded, testRevision, tracked)
		if err != nil || !reflect.DeepEqual(again, records) {
			t.Fatalf("store %q re-encodes as %q, which decodes to %+v (%v), want %+v", data, encoded, again, err, records)
		}
		var reencoded []byte
		for _, record := range again {
			row, _ := Encode(record)
			reencoded = append(reencoded, row...)
		}
		if !bytes.Equal(reencoded, encoded) {
			t.Fatalf("encoding of %q is not a fixed point: %q", encoded, reencoded)
		}
	})
}
