package attest

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"reflect"
	"testing"
)

// FuzzStatementPredicateIsTheDocumentCanonically feeds prove-document bytes to
// Statement. It must refuse rather than panic, and an accepted statement must
// be valid JSON whose predicate decodes to exactly the tree the prove document
// decodes to, in a canonical form that re-encoding leaves byte-identical.
func FuzzStatementPredicateIsTheDocumentCanonically(f *testing.F) {
	f.Add([]byte(`{"profile":"falsifiable-packet/0","revision":"abc","n":1.50,"s":"a <&>","a":[null,true,{"z":{},"y":[]}]}`))
	f.Add([]byte(`{"profile":"falsifiable-packet/0","revision":"r","k":"\ud800"}`))
	subjects := []Subject{{Name: "packet.json", Digest: map[string]string{"sha256": "00"}}}
	f.Fuzz(func(t *testing.T, document []byte) {
		statement, err := Statement(document, subjects)
		if err != nil {
			return
		}
		if !jsontext.Value(statement).IsValid() {
			t.Fatalf("statement %q for %q is not valid JSON", statement, document)
		}
		decoded := decodeWithNumbers(t, statement)
		predicate := decoded.(map[string]any)["predicate"]
		if want := decodeWithNumbers(t, document); !reflect.DeepEqual(predicate, want) {
			t.Fatalf("predicate %#v differs from the document %#v", predicate, want)
		}
		again, err := canonicalMarshal(decoded)
		if err != nil || !bytes.Equal(again, statement) {
			t.Fatalf("statement %q re-encodes as %q (%v)", statement, again, err)
		}
	})
}

func decodeWithNumbers(t *testing.T, data []byte) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("decode %q: %v", data, err)
	}
	return value
}
