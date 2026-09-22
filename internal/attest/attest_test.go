package attest

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const fakeProveDocument = `{"mutates":false,"ok":true,"packet":{"a":1,"z":"last"},"profile":"falsifiable-packet/0","proof":{"rows":[{"result":"h1","status":"PASS"}]},"revision":"deadbeefcafef00d","state":"READY","tool":"prove"}`

func fakeSubjects() []Subject {
	return []Subject{
		{Name: ".corvint/change.cem.json", Digest: map[string]string{"sha256": "abc123"}},
		{Name: "git:deadbeefcafef00d", Digest: map[string]string{"gitCommit": "deadbeefcafef00d"}},
	}
}

func TestStatementIsByteStableAcrossCalls(t *testing.T) {
	first, err := Statement([]byte(fakeProveDocument), fakeSubjects())
	if err != nil {
		t.Fatalf("Statement: %v", err)
	}
	second, err := Statement([]byte(fakeProveDocument), fakeSubjects())
	if err != nil {
		t.Fatalf("Statement (second call): %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("Statement is not byte-stable:\n%s\nvs\n%s", first, second)
	}
}

func TestStatementFieldsAndKeyOrder(t *testing.T) {
	out, err := Statement([]byte(fakeProveDocument), fakeSubjects())
	if err != nil {
		t.Fatalf("Statement: %v", err)
	}
	if bytes.ContainsAny(out, " \n\t") {
		t.Fatalf("Statement output has insignificant whitespace: %s", out)
	}
	if bytes.HasSuffix(out, []byte("\n")) {
		t.Fatalf("Statement output has a trailing newline")
	}

	// Top-level keys must be sorted: _type, predicate, predicateType, subject.
	wantPrefix := `{"_type":"https://in-toto.io/Statement/v1","predicate":`
	if !bytes.HasPrefix(out, []byte(wantPrefix)) {
		t.Fatalf("Statement output does not start with sorted top-level keys: %s", out)
	}

	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("decode Statement output: %v", err)
	}
	if decoded["_type"] != "https://in-toto.io/Statement/v1" {
		t.Errorf("_type = %v", decoded["_type"])
	}
	if decoded["predicateType"] != PredicateType {
		t.Errorf("predicateType = %v, want %v", decoded["predicateType"], PredicateType)
	}
	if PredicateType != "https://corvint-context.dev/attestation/falsifiable-packet/0" {
		t.Errorf("PredicateType = %v", PredicateType)
	}

	var wantPredicate any
	if err := json.Unmarshal([]byte(fakeProveDocument), &wantPredicate); err != nil {
		t.Fatalf("decode fake prove document: %v", err)
	}
	gotPredicate, err := json.Marshal(decoded["predicate"])
	if err != nil {
		t.Fatalf("re-marshal decoded predicate: %v", err)
	}
	wantPredicateBytes, err := json.Marshal(wantPredicate)
	if err != nil {
		t.Fatalf("re-marshal want predicate: %v", err)
	}
	if !jsonEqual(t, gotPredicate, wantPredicateBytes) {
		t.Errorf("predicate does not equal the input document semantically:\ngot  %s\nwant %s", gotPredicate, wantPredicateBytes)
	}

	subjects, ok := decoded["subject"].([]any)
	if !ok || len(subjects) != 2 {
		t.Fatalf("subject = %#v, want 2 entries", decoded["subject"])
	}
	first, _ := subjects[0].(map[string]any)
	if first["name"] != ".corvint/change.cem.json" {
		t.Errorf("subject[0].name = %v, want preserved order", first["name"])
	}
	second, _ := subjects[1].(map[string]any)
	if second["name"] != "git:deadbeefcafef00d" {
		t.Errorf("subject[1].name = %v, want preserved order", second["name"])
	}
}

func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("unmarshal a: %v", err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("unmarshal b: %v", err)
	}
	am, _ := json.Marshal(av)
	bm, _ := json.Marshal(bv)
	return bytes.Equal(am, bm)
}

func TestStatementRejectsNonObject(t *testing.T) {
	_, err := Statement([]byte(`["not", "an", "object"]`), nil)
	if err == nil {
		t.Fatal("Statement accepted a non-object prove document")
	}
}

func TestStatementRejectsWrongProfile(t *testing.T) {
	document := `{"profile":"something-else","revision":"deadbeef"}`
	_, err := Statement([]byte(document), nil)
	if err == nil {
		t.Fatal("Statement accepted a document with the wrong profile")
	}
}

func TestStatementRejectsMissingRevision(t *testing.T) {
	document := `{"profile":"falsifiable-packet/0"}`
	_, err := Statement([]byte(document), nil)
	if err == nil {
		t.Fatal("Statement accepted a document with no revision")
	}
}

func TestStatementRejectsEmptyRevision(t *testing.T) {
	document := `{"profile":"falsifiable-packet/0","revision":""}`
	_, err := Statement([]byte(document), nil)
	if err == nil {
		t.Fatal("Statement accepted a document with an empty revision")
	}
}

func TestStatementRejectsInvalidJSON(t *testing.T) {
	_, err := Statement([]byte(`{not json`), nil)
	if err == nil {
		t.Fatal("Statement accepted invalid JSON")
	}
}

// TestStatementRejectsTrailingCloseDelimiter pins the refusal of bytes after
// the document: json.Decoder.More reports false before a stray '}' or ']', so
// only a second Decode reaching io.EOF proves the document ended.
func TestStatementRejectsTrailingCloseDelimiter(t *testing.T) {
	for _, tail := range []string{"}", "]"} {
		if _, err := Statement([]byte(fakeProveDocument+tail), fakeSubjects()); err == nil {
			t.Fatalf("Statement accepted a prove document followed by %q", tail)
		}
	}
}

// TestStatementRejectsRepeatedMemberName pins that the predicate is the
// document unchanged: a last-wins decoder would sign one value of a repeated
// name and silently drop the other, at the top level or nested.
func TestStatementRejectsRepeatedMemberName(t *testing.T) {
	for _, document := range []string{
		`{"profile":"other","profile":"falsifiable-packet/0","revision":"deadbeef"}`,
		`{"packet":{"a":1,"a":2},"profile":"falsifiable-packet/0","revision":"deadbeef"}`,
	} {
		if _, err := Statement([]byte(document), fakeSubjects()); err == nil {
			t.Errorf("Statement accepted a repeated member name in %s", document)
		}
	}
}

// TestStatementRejectsInvalidUTF8 pins that no string is signed under a name it
// was not given: an invalid UTF-8 byte in the prove document or in a subject
// name is refused rather than replaced with U+FFFD, so two distinct byte names
// cannot sign as one.
func TestStatementRejectsInvalidUTF8(t *testing.T) {
	document := `{"packet":{"a":"` + "\xff" + `"},"profile":"falsifiable-packet/0","revision":"deadbeef"}`
	if _, err := Statement([]byte(document), fakeSubjects()); err == nil {
		t.Error("Statement accepted invalid UTF-8 in the prove document")
	}
	subjects := []Subject{{Name: "map\xff.cem.json", Digest: map[string]string{"sha256": "abc123"}}}
	if _, err := Statement([]byte(fakeProveDocument), subjects); err == nil {
		t.Error("Statement accepted invalid UTF-8 in a subject name")
	}
	subjects = []Subject{{Name: "map.cem.json", Digest: map[string]string{"sha\xff": "abc123"}}}
	if _, err := Statement([]byte(fakeProveDocument), subjects); err == nil {
		t.Error("Statement accepted invalid UTF-8 in a subject digest algorithm")
	}
	if _, err := CEMStatement("map\xff.cem.json", cemFixture(t)); err == nil {
		t.Error("CEMStatement accepted invalid UTF-8 in the name")
	}
}

func TestStatementGoldenBytes(t *testing.T) {
	out, err := Statement([]byte(fakeProveDocument), fakeSubjects())
	if err != nil {
		t.Fatalf("Statement: %v", err)
	}
	golden := filepath.Join("testdata", "statement.golden.json")
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}
	if !bytes.Equal(out, want) {
		t.Errorf("Statement output does not match golden file %s:\ngot:  %s\nwant: %s", golden, out, want)
	}
}

// TestCanonicalStringEscaping pins the FPK-V0-015 canonical string form: a
// space and HTML characters stay literal, controls and U+2028/U+2029 are
// escaped.
func TestCanonicalStringEscaping(t *testing.T) {
	got, err := canonicalMarshal("a b<&>\u2028\u2029\x01")
	if err != nil {
		t.Fatalf("canonicalMarshal: %v", err)
	}
	want := `"a b<&>\u2028\u2029\u0001"`
	if string(got) != want {
		t.Errorf("canonicalMarshal = %q, want %q", got, want)
	}
}
