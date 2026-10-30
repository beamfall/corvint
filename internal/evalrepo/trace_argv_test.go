package evalrepo

import (
	"bytes"
	"encoding/json"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/trace"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTypedTraceFixtureAdmission(t *testing.T) {
	revision, scored := strings.Repeat("a", 40), strings.Repeat("b", 40)
	row, err := trace.NewRecord(trace.Input{Producer: trace.ProducerCLI, Revision: revision, Task: "prior task", Outcome: "passed", VerificationArgv: [][]string{{"printf", "a b"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := trace.Encode(row)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fixture.json")
	index := &contextindex.Index{CommitRevision: scored}
	write := func(raw []byte) {
		t.Helper()
		data := append([]byte(`{"schema_version":1,"repositories":[{"scored_revision":"`+scored+`","traces":[`), raw...)
		data = append(data, []byte(`]}]}`)...)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		t.Fatal(err)
	}
	write(pretty.Bytes())
	got, err := loadTraceFixture(path, index, nil)
	if err != nil || len(got.traces) != 1 || got.traces[0].TraceID != row.TraceID {
		t.Fatalf("admission: %+v %v", got, err)
	}
	write(bytes.Replace(raw, []byte(`"kind":"argv"`), []byte(`"kind":"argv","kind":"argv"`), 1))
	if _, err := loadTraceFixture(path, index, nil); err == nil {
		t.Fatal("duplicate typed field admitted through fixture decoder")
	}
}

func TestTypedFixturePreservesUnselectedV1Validation(t *testing.T) {
	scored, other := strings.Repeat("a", 40), strings.Repeat("b", 40)
	path := filepath.Join(t.TempDir(), "fixture.json")
	for _, row := range []string{`{"schema_version":1,"unknown":true}`, `{"schema_version":"1"}`, `{"schema_version":1,"verification":[1]}`} {
		data := `{"schema_version":1,"repositories":[{"scored_revision":"` + scored + `","traces":[]},{"scored_revision":"` + other + `","traces":[` + row + `]}]}`
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadTraceFixture(path, &contextindex.Index{CommitRevision: scored}, nil); err == nil {
			t.Fatalf("unselected malformed v1 row admitted: %s", row)
		}
	}
}
