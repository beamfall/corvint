// SPDX-License-Identifier: Apache-2.0

package sessiontest

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/testduration"
	"github.com/Beamfall/corvint/internal/testid"
)

// RVD-V1-004, PTD-V1-002, PTD-V1-003: a corvint watch save's run becomes a corvint-test-run/0
// receipt with one go invocation per planned package, its outcome and Elapsed read by the Go
// adapter's parser, a package the stream never ended NOT_RUN, and the result FAIL when a package
// failed. The receipt encodes through Encode as corvint test's does.
func TestRVDV1004WatchReceipt(t *testing.T) {
	stream := strings.Join([]string{
		`{"Action":"start","Package":"example.com/m/p"}`,
		`{"Action":"run","Package":"example.com/m/p","Test":"TestPass"}`,
		`{"Action":"pass","Package":"example.com/m/p","Test":"TestPass","Elapsed":0}`,
		`{"Action":"output","Package":"example.com/m/p","Output":"ok  \texample.com/m/p\t0.012s\n"}`,
		`{"Action":"pass","Package":"example.com/m/p","Elapsed":0.012}`,
	}, "\n") + "\n"
	var echoed bytes.Buffer
	passing := ReadGoEvents(strings.NewReader(stream), &echoed)
	if !strings.Contains(echoed.String(), "ok  \texample.com/m/p") {
		t.Fatalf("echo %q", echoed.String())
	}
	failing := ReadGoEvents(strings.NewReader(`{"Action":"fail","Package":"example.com/m/q","Elapsed":1.5}`+"\n"), &bytes.Buffer{})
	source := SourceIdentity{Head: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40), Paths: []PathDigest{{Path: "p/p.go", Digest: "sha256:" + strings.Repeat("c", 64)}}}
	at := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	receipt := WatchReceipt(source, []WatchRun{
		{Argv: []string{"go", "test", "-json", "-run", "^(TestPass)$", "./p"}, Package: "example.com/m/p", Selected: true, Runner: "/usr/bin/go", Version: "go1.27.3", Exit: 0, ElapsedMilliseconds: 40, Events: passing},
		{Argv: []string{"go", "test", "-json", "./q"}, Package: "example.com/m/q", Exit: 1, ElapsedMilliseconds: 1600, Events: failing},
		{Argv: []string{"go", "test", "-json", "./r"}, Package: "example.com/m/r", Exit: 2, Events: GoEvents{}},
	}, at, testid.GoModuleDir("example.com/m", "."))
	if receipt.Profile != ReceiptProfile || receipt.Tool != ToolWatch || receipt.Source.Head != source.Head || receipt.Revision != source.Head ||
		receipt.Scope != ScopeNarrowed || receipt.CompletedAt != "2026-11-11T10:00:00Z" || len(receipt.Invocations) != 3 || receipt.Result != ResultIncomplete {
		t.Fatalf("receipt %+v", receipt)
	}
	p, q, r := receipt.Invocations[0].Outcomes[0], receipt.Invocations[1].Outcomes[0], receipt.Invocations[2].Outcomes[0]
	if p.Unit != "go:example.com/m/p" || p.Slice != SliceSelected || p.Outcome != OutcomePass || p.DurationMs == nil || *p.DurationMs != 12 ||
		p.DurationSource != testduration.SourceGo || p.DurationGranularity != testduration.GranularityFile {
		t.Fatalf("passing outcome %+v", p)
	}
	if q.Unit != "go:example.com/m/q" || q.Slice != SliceWhole || q.Outcome != OutcomeFail || q.DurationMs == nil || *q.DurationMs != 1500 {
		t.Fatalf("failing outcome %+v", q)
	}
	if r.Outcome != OutcomeNotRun || r.DurationMs != nil || r.DurationSource != testduration.NotMeasured {
		t.Fatalf("unended outcome %+v", r)
	}
	// CTI-V1-009: each outcome carries the testId corvint test stamps for the same unit.
	for _, outcome := range []Outcome{p, q, r} {
		if want := UnitTestID(outcome.Unit, testid.GoModuleDir("example.com/m", ".")); want == "" || outcome.TestID != want {
			t.Fatalf("outcome %s testId %q, want %q", outcome.Unit, outcome.TestID, want)
		}
	}
	if receipt.Invocations[0].Adapter != AdapterGo || receipt.Invocations[0].AdapterManifest != AdapterManifestProfile || receipt.Invocations[1].Exit != 1 {
		t.Fatalf("invocations %+v", receipt.Invocations)
	}
	encoded, err := Encode(receipt)
	if err != nil || !bytes.Contains(encoded, []byte(`"tool":"corvint watch"`)) {
		t.Fatalf("encode %v %s", err, encoded)
	}
	two := WatchReceipt(source, []WatchRun{{Package: "example.com/m/p", Events: passing}}, at, nil)
	if two.Result != ResultPass || len(two.Incomplete) != 0 || two.Invocations[0].Outcomes[0].TestID != "" {
		t.Fatalf("passing receipt %+v", two)
	}
}
