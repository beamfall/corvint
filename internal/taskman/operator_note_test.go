package taskman

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestONV0001_CoreReaderAdmitsOnlyTheClosedNoteReference: Core decodes a
// legacy ticket and one carrying a valid SET or CLEARED operatorNote
// reference, and refuses a malformed reference or any other unknown member.
func TestONV0001_CoreReaderAdmitsOnlyTheClosedNoteReference(t *testing.T) {
	record := func(note string) error {
		t.Helper()
		v := value(value(testDocument(t, "tickets"), "items").Arr[0], "record")
		if note != "" {
			n, err := wire.Parse([]byte(note))
			if err != nil {
				t.Fatal(err)
			}
			v.Obj.Keys = append(v.Obj.Keys, "operatorNote")
			v.Obj.Values["operatorNote"] = n
		}
		_, err := decodeTicket(v)
		return err
	}
	head, other := strings.Repeat("a", 64), strings.Repeat("b", 64)
	if err := record(""); err != nil {
		t.Fatalf("Core refused a legacy ticket: %v", err)
	}
	for _, good := range []string{
		`{"current":"` + head + `","head":"` + head + `","revision":"1"}`,
		`{"current":null,"head":"` + head + `","revision":"4096"}`,
	} {
		if err := record(good); err != nil {
			t.Fatalf("Core refused %s: %v", good, err)
		}
	}
	for name, bad := range map[string]string{
		"zero revision":    `{"current":null,"head":"` + head + `","revision":"0"}`,
		"over 4096":        `{"current":null,"head":"` + head + `","revision":"4097"}`,
		"current not head": `{"current":"` + other + `","head":"` + head + `","revision":"2"}`,
		"extra member":     `{"current":null,"extra":null,"head":"` + head + `","revision":"2"}`,
		"whole null":       `null`,
		"bad head":         `{"current":null,"head":"not-a-digest","revision":"2"}`,
	} {
		if err := record(bad); err == nil {
			t.Errorf("%s: Core accepted %s", name, bad)
		}
	}
	v := value(value(testDocument(t, "tickets"), "items").Arr[0], "record")
	v.Obj.Keys = append(v.Obj.Keys, "zzUnknown")
	v.Obj.Values["zzUnknown"] = wire.Value{Kind: wire.KindNull}
	if _, err := decodeTicket(v); err == nil {
		t.Error("Core accepted an unknown ticket member")
	}
}
