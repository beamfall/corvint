package journal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// LGL-V0-806: every inline record of a decoded receipt carries the receipt
// decode's own canonical encoding, and wire.Parse of that encoding returns
// the record itself, so the walk loses nothing by not parsing it again.
func TestLGLV0806_DecodedRecordIsItsOwnReparse(t *testing.T) {
	repo, _ := prefetchHistory(t, 120)
	names, err := os.ReadDir(filepath.Join(repo.StateDir, "receipts"))
	if err != nil {
		t.Fatal(err)
	}
	inline := 0
	for _, n := range names {
		raw := read(t, filepath.Join(repo.StateDir, "receipts", n.Name()))
		rc, err := snapshot.DecodeReceipt(raw)
		if err != nil {
			t.Fatalf("%s: %v", n.Name(), err)
		}
		for _, p := range rc.Post {
			if p.Record == nil {
				if p.Raw != nil {
					t.Fatalf("%s %s: encoding without a record", n.Name(), p.Path)
				}
				continue
			}
			inline++
			if !bytes.Equal(p.Raw, wire.EncodeFile(*p.Record)) {
				t.Fatalf("%s %s: Raw is not the record's encoding", n.Name(), p.Path)
			}
			v, err := wire.Parse(p.Raw)
			if err != nil || !reflect.DeepEqual(v, *p.Record) {
				t.Fatalf("%s %s: reparse %v, equal %v", n.Name(), p.Path, err, reflect.DeepEqual(v, *p.Record))
			}
		}
	}
	if inline < 100 {
		t.Fatalf("only %d inline records checked", inline)
	}
}

// LGL-V0-806: a record the standalone parse would refuse for a decode bound
// or a code point never reaches the walk, because the receipt that holds it
// is refused first with the same code; one just inside every bound reparses
// to itself.
func TestLGLV0806_BoundsRefuseTheReceiptBeforeTheRecord(t *testing.T) {
	nested := func(depth int) string { return strings.Repeat(`{"a":`, depth) + `"x"` + strings.Repeat("}", depth) }
	array := func(n int) string { return `{"a":[` + strings.TrimSuffix(strings.Repeat(`"x",`, n), ",") + `]}` }
	wrap := func(record string) []byte { return []byte(`{"post":[{"record":` + record + `}]}` + "\n") }
	// The record's own first level is depth 4 of the receipt (object, post
	// array, entry, record) and depth 1 alone.
	for _, tc := range []struct {
		name, record string
		refused      string
	}{
		{"deepest record a receipt holds", nested(wire.MaxJSONDepth - 3), ""},
		{"one level deeper", nested(wire.MaxJSONDepth - 2), wire.CodeLimitExceeded},
		{"widest array", array(wire.MaxJSONArrayElements), ""},
		{"one element wider", array(wire.MaxJSONArrayElements + 1), wire.CodeLimitExceeded},
		{"hostile code point", `{"a":"‮"}`, wire.CodeMalformed},
		{"raw bidi override", "{\"a\":\"‮\"}", wire.CodeMalformed},
		{"unsorted keys", `{"b":"x","a":"x"}`, wire.CodeMalformed},
		{"duplicate key", `{"a":"x","a":"y"}`, wire.CodeMalformed},
		{"JSON number", `{"a":1}`, wire.CodeMalformed},
	} {
		doc, err := wire.Parse(wrap(tc.record))
		if tc.refused != "" {
			if wire.CodeOf(err) != tc.refused {
				t.Errorf("%s: receipt parse %v, want %s", tc.name, err, tc.refused)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		entry, _ := doc.Obj.Get("post")
		record, _ := entry.Arr[0].Obj.Get("record")
		again, err := wire.Parse(wire.EncodeFile(record))
		if err != nil || !reflect.DeepEqual(again, record) {
			t.Fatalf("%s: standalone reparse %v", tc.name, err)
		}
	}
	// The node bound counts the whole receipt, so a record over it is
	// refused with the receipt.
	inner := `[` + strings.TrimSuffix(strings.Repeat(`"x",`, wire.MaxJSONArrayElements), ",") + `]`
	rows := wire.MaxJSONNodes/wire.MaxJSONArrayElements + 1
	big := `{"a":[` + strings.TrimSuffix(strings.Repeat(inner+",", rows), ",") + `]}`
	if _, err := wire.Parse(wrap(big)); wire.CodeOf(err) != wire.CodeLimitExceeded || !strings.Contains(err.Error(), "decoded nodes") {
		t.Fatalf("record over the node bound: %v", err)
	}
}

// LGL-V0-805/806: validateRecord given the decoded record returns exactly
// what it returns when it parses the bytes, for valid records and for every
// semantic refusal.
func TestLGLV0806_ValidateRecordMatchesReparse(t *testing.T) {
	_, r := setup(t)
	ticket := func(edit func(*wire.Object)) []byte {
		v, err := wire.Parse(fixture.Ticket("A").Encode())
		if err != nil {
			t.Fatal(err)
		}
		edit(v.Obj)
		return wire.EncodeFile(v)
	}
	attempt := func(id string) []byte {
		return wire.EncodeFile(object("profile", str("taskman-attempt/0"), "queueId", str(fixture.QueueID), "attemptId", str(id)))
	}
	rc := &snapshot.Receipt{Seq: "7", Kind: "MUTATION"}
	cases := []struct {
		path string
		raw  []byte
	}{
		{"intent/tickets/A.json", fixture.Ticket("A").Encode()},
		{"intent/tickets/B.json", fixture.Ticket("A").Encode()},
		{"intent/tickets/A.json", ticket(func(o *wire.Object) { o.Set("queueId", str("queue:other:main")) })},
		{"intent/tickets/A.json", ticket(func(o *wire.Object) { o.Set("title", wire.Null()) })},
		{"intent/tickets/A.json", ticket(func(o *wire.Object) { o.Set("profile", str("taskman-ticket/99")) })},
		{"attempts/queue:acme:main:x1.json", attempt("queue:acme:main:x1")},
		{"attempts/queue:acme:main:x1.json", attempt("queue:acme:main:x2")},
		{"attempts/queue:other:main:x1.json", attempt("queue:other:main:x1")},
		{"effects/k.json", wire.EncodeFile(object("profile", str("taskman-effect/0"), "key", str("other")))},
		{"pinned/p.json", wire.EncodeFile(object("queueId", str(fixture.QueueID)))},
		{"requests/r.json", requestBytes("r", 7, wire.Sum([]byte("m")), false)},
		{"requests/r.json", wire.EncodeFile(object("requestId", str("r")))},
	}
	refused := 0
	for _, tc := range cases {
		v, err := wire.Parse(tc.raw)
		if err != nil {
			t.Fatalf("%s: fixture does not parse: %v", tc.path, err)
		}
		c1, d1, p1, e1 := r.validateRecord(tc.path, tc.raw, rc, nil)
		c2, d2, p2, e2 := r.validateRecord(tc.path, tc.raw, rc, &v)
		if c1 != c2 || !reflect.DeepEqual(d1, d2) || !reflect.DeepEqual(p1, p2) || fmt.Sprint(e1) != fmt.Sprint(e2) {
			t.Fatalf("%s: reparse (%v %v %v) differs from reuse (%v %v %v)", tc.path, c1, p1, e1, c2, p2, e2)
		}
		if e1 != nil {
			refused++
		}
	}
	if refused < 6 {
		t.Fatalf("only %d of %d cases refused; the refusal cases are not exercised", refused, len(cases))
	}
}

// LGL-V0-805/806: a complete audit with reuse returns the audit without it,
// verdicts and records alike, on a valid history and on one whose latest
// receipt was tampered with or posts a record the walk refuses.
func TestLGLV0806_AuditWithReuseMatchesAuditWithout(t *testing.T) {
	audit := func(r Reader, reuse bool) (*Result, error) {
		old := inlinePostReuse
		inlinePostReuse = reuse
		defer func() { inlinePostReuse = old }()
		return r.Audit("intent/tickets/A.json", "intent/tickets/B.json")
	}
	same := func(name string, r Reader) {
		t.Helper()
		without, e1 := audit(r, false)
		with, e2 := audit(r, true)
		if fmt.Sprint(e1) != fmt.Sprint(e2) {
			t.Fatalf("%s: errors differ: without %v, with %v", name, e1, e2)
		}
		if e1 != nil {
			return
		}
		if without.LastSeq != with.LastSeq || without.LastReceiptSha256 != with.LastReceiptSha256 || without.StructuralConsistency != with.StructuralConsistency || without.SemanticCoverage != with.SemanticCoverage || without.ProjectionAgreement != with.ProjectionAgreement || !reflect.DeepEqual(without.Records, with.Records) || !reflect.DeepEqual(without.RequestDigests, with.RequestDigests) {
			t.Fatalf("%s: results differ:\nwithout %+v\nwith    %+v", name, without, with)
		}
	}
	repo, r := prefetchHistory(t, 60)
	same("valid", r)

	// A ticket post naming another file: refused the same way.
	appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/C.json": fixture.Ticket("A").Encode()}, "", false, true, false)
	same("ticket filename identity", r)
	if _, err := audit(r, true); wire.CodeOf(err) != wire.CodeJournalForked {
		t.Fatalf("tampered identity: %v, want %s", err, wire.CodeJournalForked)
	}

	// A record edited in place after the digest was recorded.
	repo2, r2 := prefetchHistory(t, 20)
	names, _ := os.ReadDir(filepath.Join(repo2.StateDir, "receipts"))
	last := filepath.Join(repo2.StateDir, "receipts", names[len(names)-1].Name())
	raw := read(t, last)
	tampered := bytes.Replace(raw, []byte(`"receipt 19"`), []byte(`"receipt 91"`), 1)
	if bytes.Equal(raw, tampered) {
		t.Fatal("fixture body not found")
	}
	fixture.Write(t, last, tampered)
	same("tampered record", r2)
	if _, err := audit(r2, true); err == nil {
		t.Fatal("an audit with reuse accepted a tampered record")
	}
}
