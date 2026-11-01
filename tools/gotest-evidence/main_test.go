package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const tree = "6c967b21caa2ffb0324a681f873a891996d82e34"

var prov = Provenance{Tree: tree, Toolchain: "go1.27.3", Platform: "linux/amd64", Command: "go test -json -count=2 ."}

func stream(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func recordFile(t *testing.T, ledger string, data []byte, p Provenance) {
	t.Helper()
	var out, errb bytes.Buffer
	args := []string{"record", "-ledger", ledger, "-tree", p.Tree, "-toolchain", p.Toolchain, "-platform", p.Platform, "-command", p.Command}
	if code := run(args, bytes.NewReader(data), &out, &errb); code != 0 {
		t.Fatalf("record exit %d: %s", code, errb.String())
	}
}

// TestGTEV0822RecordingTwiceIsByteIdentical binds GTE-V0-822: the same stream
// recorded twice adds no record and leaves the ledger byte-identical.
func TestGTEV0822RecordingTwiceIsByteIdentical(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "evidence.jsonl")
	recordFile(t, ledger, stream(t, "count2.json"), prov)
	first, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	added, err := record(ledger, bytes.NewReader(stream(t, "count2.json")), prov)
	if err != nil || added != 0 {
		t.Fatalf("second record added %d: %v", added, err)
	}
	second, _ := os.ReadFile(ledger)
	if !bytes.Equal(first, second) {
		t.Fatal("rerecording changed the ledger bytes")
	}
	records, err := readLedger(ledger)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, r := range records {
		if ids[r.ID] {
			t.Fatalf("duplicate record %s", r.ID)
		}
		ids[r.ID] = true
	}
	// -count=2 over 4 top-level tests and 2 subtests, plus the one package
	// outcome: each repeat is its own ordinal, never a merge.
	if len(records) != 2*(4+2)+1 {
		t.Fatalf("records = %d", len(records))
	}
}

// TestGTEV0821RecordsNameIdentityAndProvenance binds GTE-V0-821 and
// GTE-V0-823: each record names package, test, subtest path, ordinal, tree,
// toolchain, platform and command, and a renamed test is a new identity.
func TestGTEV0821RecordsNameIdentityAndProvenance(t *testing.T) {
	records, err := Derive(stream(t, "count2.json"), prov)
	if err != nil {
		t.Fatal(err)
	}
	var nested []Record
	for _, r := range records {
		if r.Tree != tree || r.Toolchain != "go1.27.3" || r.Platform != "linux/amd64" || r.Command != prov.Command || r.Package != "example.com/flaky" {
			t.Fatalf("provenance %+v", r)
		}
		if r.Test == "TestTable/two/nested" {
			nested = append(nested, r)
		}
	}
	if len(nested) != 2 || nested[0].Ordinal != 1 || nested[1].Ordinal != 2 || nested[0].ID == nested[1].ID || strings.Join(nested[0].Path, "|") != "TestTable|two|nested" || nested[0].Outcome != "pass" {
		t.Fatalf("nested subtest records %+v", nested)
	}
	renamed, err := Derive(bytes.ReplaceAll(stream(t, "count2.json"), []byte(`"TestPass"`), []byte(`"TestPassRenamed"`)), prov)
	if err != nil {
		t.Fatal(err)
	}
	before := map[string]bool{}
	for _, r := range records {
		before[r.Package+" "+r.Test] = true
	}
	for _, r := range renamed {
		if r.Test == "TestPassRenamed" && before[r.Package+" "+r.Test] {
			t.Fatal("renamed test merged into an old identity")
		}
	}
	// Undeclared provenance is NOT_OBSERVED, never guessed.
	bare, err := Derive(stream(t, "count2.json"), Provenance{Tree: tree, Toolchain: notObserved, Platform: notObserved, Command: notObserved})
	if err != nil || bare[0].Toolchain != notObserved || bare[0].Platform != notObserved || bare[0].Command != notObserved {
		t.Fatalf("%+v %v", bare[0], err)
	}
}

// TestGTEV0824UnfinishedTestIsNotObserved binds GTE-V0-824: a test that ran
// and never reached a terminal event is NOT_OBSERVED, and a stream that is not
// wholly go test -json events, or a ledger with an unverified line, is refused
// without a write.
func TestGTEV0824UnfinishedTestIsNotObserved(t *testing.T) {
	s := stream(t, "flaky-fail.json")
	cut := bytes.Index(s, []byte(`"Action":"pass","Package":"example.com/flaky","Test":"TestPass"`))
	if cut < 0 {
		t.Fatal("fixture shape")
	}
	cut = bytes.LastIndexByte(s[:cut], '\n') + 1
	records, err := Derive(s[:cut], prov)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Test != "TestPass" || records[0].Outcome != notObserved || records[0].Elapsed != nil {
		t.Fatalf("killed run records %+v", records)
	}
	dir := t.TempDir()
	ledger := filepath.Join(dir, "evidence.jsonl")
	for name, bad := range map[string][]byte{
		"prefixed log":   append([]byte("2026-10-31T00:00:00Z "), s...),
		"stray brace":    []byte(`{"Action":"pass","Package":"p","Test":"TestX"}}` + "\n"),
		"two values":     []byte(`{"Action":"pass","Package":"p","Test":"TestX"} {"Action":"fail","Package":"p","Test":"TestX"}` + "\n"),
		"unknown action": []byte(`{"Action":"explode","Package":"p"}` + "\n"),
		"empty":          {},
	} {
		if _, err := record(ledger, bytes.NewReader(bad), prov); err == nil {
			t.Fatalf("%s stream recorded", name)
		}
		if _, err := os.Stat(ledger); !os.IsNotExist(err) {
			t.Fatalf("%s stream wrote the ledger", name)
		}
	}
	for _, p := range []Provenance{{Tree: "HEAD", Toolchain: "go", Platform: notObserved, Command: "x"}, {Tree: tree, Toolchain: "go", Platform: "linux", Command: "x"}, {Tree: tree, Toolchain: "go\nx", Platform: notObserved, Command: "x"}} {
		if _, err := Derive(s, p); err == nil {
			t.Fatalf("provenance %+v admitted", p)
		}
	}
	recordFile(t, ledger, s, prov)
	data, _ := os.ReadFile(ledger)
	if err := os.WriteFile(ledger, bytes.Replace(data, []byte(`"outcome":"fail"`), []byte(`"outcome":"pass"`), 1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readLedger(ledger); err == nil {
		t.Fatal("tampered ledger line verified")
	}
	// A verified record followed by trailing bytes is not one record.
	lines := bytes.SplitAfter(data, []byte("\n"))
	for _, tail := range []string{"}", " garbage", `{"schema":"x"}`} {
		trailing := append(bytes.TrimSuffix(append([]byte{}, lines[0]...), []byte("\n")), tail+"\n"...)
		if err := os.WriteFile(ledger, append(trailing, bytes.Join(lines[1:], nil)...), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readLedger(ledger); err == nil {
			t.Fatalf("ledger record followed by %q verified", tail)
		}
	}
}

// TestGTEV0825SummaryMeasuresFlakes binds GTE-V0-825: summary counts outcomes
// per identity across runs and marks flaky only a pass and a fail on the same
// tree, toolchain and platform.
func TestGTEV0825SummaryMeasuresFlakes(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "evidence.jsonl")
	recordFile(t, ledger, stream(t, "count2.json"), prov)
	other := prov
	other.Command = "FLAKY_FAIL=1 go test -json -count=1 ."
	recordFile(t, ledger, stream(t, "flaky-fail.json"), other)
	elsewhere := prov
	elsewhere.Tree = strings.Repeat("e", 40)
	renamed := bytes.ReplaceAll(stream(t, "count2.json"), []byte(`"Test":"TestPass"`), []byte(`"Test":"TestElsewhere"`))
	recordFile(t, ledger, renamed, elsewhere)
	var out bytes.Buffer
	if code := run([]string{"summary", "-ledger", ledger}, nil, &out, &out); code != 0 {
		t.Fatal(out.String())
	}
	want := map[string]string{
		"TestTable/two/nested": "4\t1\t0\t0\t3\ttrue",
		"TestFail":             "0\t5\t0\t0\t3\tfalse",
		"TestPass":             "3\t0\t0\t0\t2\tfalse",
		"TestElsewhere":        "2\t0\t0\t0\t1\tfalse",
	}
	for _, line := range strings.Split(out.String(), "\n") {
		f := strings.SplitN(line, "\t", 3)
		if len(f) == 3 && want[f[1]] != "" {
			if f[2] != want[f[1]] {
				t.Fatalf("%s: %q, want %q", f[1], f[2], want[f[1]])
			}
			delete(want, f[1])
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing rows %v in\n%s", want, out.String())
	}
	// A pass on one tree and a fail on another is a change, not a flake.
	split := filepath.Join(t.TempDir(), "split.jsonl")
	recordFile(t, split, stream(t, "count2.json"), prov)
	recordFile(t, split, stream(t, "flaky-fail.json"), elsewhere)
	out.Reset()
	run([]string{"summary", "-ledger", split}, nil, &out, &out)
	if !strings.Contains(out.String(), "\tTestTable/two/nested\t2\t1\t0\t0\t2\tfalse\n") {
		t.Fatalf("cross-tree outcome marked flaky:\n%s", out.String())
	}
	// Undeclared toolchain and platform are never the same environment: the
	// same pass and fail on one tree are unknown, not flaky.
	undeclared := filepath.Join(t.TempDir(), "undeclared.jsonl")
	bare := Provenance{Tree: tree, Toolchain: notObserved, Platform: notObserved, Command: notObserved}
	recordFile(t, undeclared, stream(t, "count2.json"), bare)
	recordFile(t, undeclared, stream(t, "flaky-fail.json"), bare)
	out.Reset()
	run([]string{"summary", "-ledger", undeclared}, nil, &out, &out)
	if !strings.Contains(out.String(), "\tTestTable/two/nested\t2\t1\t0\t0\t2\tunknown\n") || strings.Contains(out.String(), "\ttrue\n") {
		t.Fatalf("undeclared environments joined as one:\n%s", out.String())
	}
}

// TestGTEV0820HelpNamesUsageInputsAndExample binds GTE-V0-820's help surface.
func TestGTEV0820HelpNamesUsageInputsAndExample(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"--help"}, nil, &out, &out); code != 0 || !strings.Contains(out.String(), "-ledger FILE -tree SHA") || !strings.Contains(out.String(), "example:") {
		t.Fatalf("%d %s", code, out.String())
	}
	if code := run([]string{"record", "-tree", tree}, nil, &out, &out); code != 2 {
		t.Fatalf("record without -ledger exit %d", code)
	}
}
