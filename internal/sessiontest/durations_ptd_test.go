// SPDX-License-Identifier: Apache-2.0

package sessiontest

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testduration"
)

// PTD-V1-001, PTD-V1-003, PTD-V1-007: Finish gives every outcome a durationSource, NOT_MEASURED
// where the adapter measured nothing, and lists an invalid time under durationInvalid; the receipt
// round-trips canonically through the strict reader.
func TestPTDV1001ReceiptDurationFieldsRoundTrip(t *testing.T) {
	m, digest := twoShardManifest(t)
	three := int64(3)
	measured := Outcome{DurationGranularity: testduration.GranularityFile, DurationMs: &three, DurationSource: testduration.SourceGo, Outcome: OutcomePass, Slice: SliceWhole, Unit: "go:m/a"}
	receipt := shardReceipt(m, digest, 0, measured)
	if got := receipt.Invocations[0].Outcomes[0]; got.DurationMs == nil || *got.DurationMs != 3 || got.DurationSource != testduration.SourceGo || got.DurationGranularity != "file" {
		t.Fatalf("measured outcome %+v", got)
	}
	unmeasured := shardReceipt(m, digest, 0, Outcome{Outcome: OutcomePass, Slice: SliceWhole, Unit: "go:m/a"})
	if got := unmeasured.Invocations[0].Outcomes[0]; got.DurationMs != nil || got.DurationSource != testduration.NotMeasured {
		t.Fatalf("unmeasured outcome %+v", got)
	}
	invalid := timedOutcome(Outcome{Outcome: OutcomePass, Slice: SliceWhole, Unit: "go:m/a"}, ptr(testduration.FromSeconds(-1, testduration.SourceGo)))
	bad := shardReceipt(m, digest, 0, invalid)
	if got := bad.Invocations[0].Outcomes[0]; got.DurationMs != nil || got.DurationSource != testduration.NotMeasured || !slices.Equal(bad.DurationInvalid, []string{"go:m/a"}) {
		t.Fatalf("invalid outcome %+v, durationInvalid %v", got, bad.DurationInvalid)
	}
	dir := t.TempDir()
	for name, r := range map[string]Receipt{"measured": receipt, "invalid": bad} {
		encoded, err := Encode(r)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, encoded, 0o600); err != nil {
			t.Fatal(err)
		}
		read, err := ReadReceipt(path)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		again, _ := Encode(read)
		if !bytes.Equal(again, encoded) {
			t.Fatalf("%s does not round-trip", name)
		}
	}
	encoded, _ := Encode(receipt)
	if !strings.Contains(string(encoded), `"durationGranularity":"file","durationMs":3,"durationSource":"go-test2json","outcome":"PASS"`) {
		t.Fatalf("encoded outcome %s", encoded)
	}
}

// PTD-V1-001: the fields are add-only. A receipt written before them still reads strictly and keeps
// its bytes, and a reader that knows only outcome, slice and unit ignores them.
func TestPTDV1001OlderReceiptsAndReaders(t *testing.T) {
	m, digest := twoShardManifest(t)
	legacy := NewReceipt(m, digest, 0, "corvint test")
	legacy.Invocations = []Invocation{{Adapter: AdapterGo, AdapterManifest: AdapterManifestProfile, Argv: []string{"go"}, Env: []string{}, Outcomes: []Outcome{{Outcome: OutcomePass, Slice: SliceWhole, Unit: "go:m/a"}}}}
	legacy.Result, legacy.Summary = ResultPass, "selected units passed: 1 of 1"
	encoded, err := Encode(legacy)
	if err != nil || bytes.Contains(encoded, []byte("duration")) {
		t.Fatalf("legacy encoding %v %s", err, encoded)
	}
	path := filepath.Join(t.TempDir(), "legacy.json")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadReceipt(path); err != nil {
		t.Fatalf("a receipt without duration fields no longer reads: %v", err)
	}
	three := int64(3)
	current := shardReceipt(m, digest, 0, Outcome{DurationGranularity: "file", DurationMs: &three, DurationSource: testduration.SourceGo, Outcome: OutcomePass, Slice: SliceWhole, Unit: "go:m/a"})
	raw, _ := Encode(current)
	var older struct {
		Invocations []struct {
			Outcomes []struct{ Outcome, Slice, Unit string } `json:"outcomes"`
		} `json:"invocations"`
	}
	if err := json.Unmarshal(raw, &older); err != nil || older.Invocations[0].Outcomes[0].Unit != "go:m/a" || older.Invocations[0].Outcomes[0].Outcome != OutcomePass {
		t.Fatalf("an older reader cannot read the receipt: %v %+v", err, older)
	}
}

// PTD-V1-002, PTD-V1-004: the Go adapter reads each package's Elapsed from its terminal
// package-level event, not from a test's, and records it as a file-granularity duration.
func TestPTDV1002GoPackageElapsed(t *testing.T) {
	stream := strings.Join([]string{
		`{"Action":"pass","Package":"m/a","Test":"TestA","Elapsed":0.001}`,
		`{"Action":"pass","Package":"m/a","Elapsed":0.004}`,
		`{"Action":"fail","Package":"m/b","Elapsed":1.5}`,
		`{"Action":"skip","Package":"m/c"}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	outcomes, elapsed := parseTestEventsTimed(strings.NewReader(stream), &out)
	if outcomes["m/a"] != OutcomePass || outcomes["m/b"] != OutcomeFail || outcomes["m/c"] != OutcomeNoTests {
		t.Fatalf("outcomes %v", outcomes)
	}
	if len(elapsed) != 2 || elapsed["m/a"] != 0.004 || elapsed["m/b"] != 1.5 {
		t.Fatalf("elapsed %v", elapsed)
	}
	got := timedOutcome(Outcome{Outcome: OutcomePass, Slice: SliceWhole, Unit: "go:m/a"}, ptr(testduration.FromSeconds(elapsed["m/a"], testduration.SourceGo)))
	if got.DurationMs == nil || *got.DurationMs != 4 || got.DurationSource != testduration.SourceGo || got.DurationGranularity != testduration.GranularityFile {
		t.Fatalf("timed outcome %+v", got)
	}
}

func ptr(measure testduration.Measure) *testduration.Measure { return &measure }
