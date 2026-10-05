package taskman

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestERGV0009_CoreReaderAdmitsOnlyTheClosedReviewReferences: Core decodes a
// ticket carrying valid ERG-V0-009 gate references (alone or beside an
// operator note) and refuses a malformed map or reference.
func TestERGV0009_CoreReaderAdmitsOnlyTheClosedReviewReferences(t *testing.T) {
	head := strings.Repeat("a", 64)
	record := func(reviews string, note bool) error {
		t.Helper()
		v := value(value(testDocument(t, "tickets"), "items").Arr[0], "record")
		if note {
			n, _ := wire.Parse([]byte(`{"current":null,"head":"` + head + `","revision":"1"}`))
			v.Obj.Keys = append(v.Obj.Keys, "operatorNote")
			v.Obj.Values["operatorNote"] = n
		}
		r, err := wire.Parse([]byte(reviews))
		if err != nil {
			t.Fatal(err)
		}
		v.Obj.Keys = append(v.Obj.Keys, "externalReviews")
		v.Obj.Values["externalReviews"] = r
		_, err = decodeTicket(v)
		return err
	}
	ref := func(g, r string) string {
		return `{"generation":"` + g + `","head":"` + head + `","revision":"` + r + `"}`
	}
	for _, good := range []string{`{"g1":` + ref("1", "1") + `}`, `{"g1":` + ref("2", "4096") + `,"g2":` + ref("1", "3") + `}`} {
		for _, note := range []bool{false, true} {
			if err := record(good, note); err != nil {
				t.Fatalf("Core refused %s (note %t): %v", good, note, err)
			}
		}
	}
	for name, bad := range map[string]string{
		"empty map":        `{}`,
		"not an object":    `null`,
		"zero generation":  `{"g1":` + ref("0", "1") + `}`,
		"generation > rev": `{"g1":` + ref("3", "2") + `}`,
		"over 4096":        `{"g1":` + ref("1", "4097") + `}`,
		"extra member":     `{"g1":{"extra":null,"generation":"1","head":"` + head + `","revision":"1"}}`,
		"bad head":         `{"g1":{"generation":"1","head":"nope","revision":"1"}}`,
		"bad gate label":   `{"":` + ref("1", "1") + `}`,
	} {
		if err := record(bad, false); err == nil {
			t.Errorf("%s: Core accepted %s", name, bad)
		}
	}
}
