package repro

import (
	"encoding/json"
	"strings"
	"testing"
)

func validBundle() Bundle {
	exit := 0
	stdout := []byte("hello\n")
	return Bundle{
		Profile:    Profile,
		RecordedBy: Build{Version: "1.0.0-rc.3", DisplayVersion: "1.0.0-rc.3.dev", Build: "774", Commit: strings.Repeat("a", 40), CommitState: "clean"},
		Command:    Command{Arguments: []string{"impact", "--path", "README.md"}},
		Inputs:     Inputs{Repository: Repository{Commit: strings.Repeat("b", 40), Tree: strings.Repeat("c", 40)}, Files: []InputFile{{Path: "in/x.json", SHA256: Digest([]byte("x"))}}},
		Observed:   Observe(0, stdout, nil),
		Expectations: []Expectation{
			{ID: "e1", Kind: ExitCode, Exit: &exit},
			{ID: "e2", Kind: StdoutContains, Text: "hello"},
			{ID: "e3", Kind: StdoutNotContains, Text: "panic"},
			{ID: "e4", Kind: StdoutSame},
			{ID: "e5", Kind: StderrContains, Text: "warn"},
		},
	}
}

// TestRPBV0001BundleRoundTripsThroughClosedSchema: an encoded bundle decodes to itself, and an
// unknown member, trailing data or another profile is refused.
func TestRPBV0001BundleRoundTripsThroughClosedSchema(t *testing.T) {
	bundle := validBundle()
	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("valid bundle refused: %v", err)
	}
	again, _ := json.Marshal(decoded)
	if string(again) != string(encoded) {
		t.Fatalf("round trip changed the bundle:\n%s\n%s", encoded, again)
	}
	for name, data := range map[string]string{
		"unknown member": strings.Replace(string(encoded), `"profile"`, `"extra":1,"profile"`, 1),
		"trailing data":  string(encoded) + "{}",
		"other profile":  strings.Replace(string(encoded), Profile, "corvint-repro/9", 1),
		"not json":       "nope",
	} {
		if _, err := Decode([]byte(data)); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

// TestRPBV0002BundleBoundsAndPins: each malformed or unbounded member is refused.
func TestRPBV0002BundleBoundsAndPins(t *testing.T) {
	for name, mutate := range map[string]func(*Bundle){
		"short commit":       func(b *Bundle) { b.Inputs.Repository.Commit = "abc" },
		"missing tree":       func(b *Bundle) { b.Inputs.Repository.Tree = "" },
		"absolute input":     func(b *Bundle) { b.Inputs.Files[0].Path = "/etc/passwd" },
		"escaping input":     func(b *Bundle) { b.Inputs.Files[0].Path = "../x" },
		"unclean input":      func(b *Bundle) { b.Inputs.Files[0].Path = "a/./b" },
		"bad digest":         func(b *Bundle) { b.Inputs.Files[0].SHA256 = "zz" },
		"repeated input":     func(b *Bundle) { b.Inputs.Files = append(b.Inputs.Files, b.Inputs.Files[0]) },
		"nil files":          func(b *Bundle) { b.Inputs.Files = nil },
		"no arguments":       func(b *Bundle) { b.Command.Arguments = nil },
		"root argument":      func(b *Bundle) { b.Command.Arguments = []string{"--root", "x", "impact"} },
		"recursive repro":    func(b *Bundle) { b.Command.Arguments = []string{"repro", "run", "x"} },
		"no expectations":    func(b *Bundle) { b.Expectations = nil },
		"repeated id":        func(b *Bundle) { b.Expectations[1].ID = "e1" },
		"unknown kind":       func(b *Bundle) { b.Expectations[0].Kind = "exit-was" },
		"exit without value": func(b *Bundle) { b.Expectations[0].Exit = nil },
		"contains no text":   func(b *Bundle) { b.Expectations[1].Text = "" },
		"stdout digest lies": func(b *Bundle) { b.Observed.Stdout = "other\n" },
		"incomplete build":   func(b *Bundle) { b.RecordedBy.Commit = "" },
	} {
		bundle := validBundle()
		mutate(&bundle)
		if err := bundle.Validate(); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

// TestRPBV0005ObservationKeepsBoundedText: oversized output keeps its full digest and length and
// a truncated prefix.
func TestRPBV0005ObservationKeepsBoundedText(t *testing.T) {
	big := []byte(strings.Repeat("x", MaxStdoutBytes+10))
	observation := Observe(3, big, []byte(strings.Repeat("e", MaxStderrBytes+1)))
	if !observation.StdoutTruncated || len(observation.Stdout) != MaxStdoutBytes || observation.StdoutBytes != len(big) || observation.StdoutSHA256 != Digest(big) {
		t.Fatalf("stdout bound: truncated %v len %d", observation.StdoutTruncated, len(observation.Stdout))
	}
	if !observation.StderrTruncated || len(observation.Stderr) != MaxStderrBytes || observation.ExitCode != 3 {
		t.Fatalf("stderr bound: %+v", observation.StderrTruncated)
	}
	bundle := validBundle()
	bundle.Observed = observation
	if err := bundle.Validate(); err != nil {
		t.Fatalf("truncated observation refused: %v", err)
	}
}

// TestRPBV0007EvaluateJudgesEachExpectation: every kind passes and fails on its own evidence.
func TestRPBV0007EvaluateJudgesEachExpectation(t *testing.T) {
	bundle := validBundle()
	statuses := func(results []Result) string {
		out := []string{}
		for _, result := range results {
			out = append(out, result.ID+"="+result.Status)
		}
		return strings.Join(out, ",")
	}
	if got := statuses(Evaluate(bundle, 0, []byte("hello\n"), []byte("warn: x"))); got != "e1=PASS,e2=PASS,e3=PASS,e4=PASS,e5=PASS" {
		t.Fatalf("matching replay: %s", got)
	}
	if got := statuses(Evaluate(bundle, 1, []byte("panic: hello"), nil)); got != "e1=FAIL,e2=PASS,e3=FAIL,e4=FAIL,e5=FAIL" {
		t.Fatalf("diverging replay: %s", got)
	}
	results := Evaluate(bundle, 1, nil, nil)
	if results[0].Expected != "0" || results[0].Observed != "1" || results[1].Observed != "absent" {
		t.Fatalf("results do not name expected and observed: %+v", results)
	}
}
