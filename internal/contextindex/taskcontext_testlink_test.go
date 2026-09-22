package contextindex

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// testLinkFixture holds one code/test pairing per TCP-V0-015 signal, each
// bound by that signal alone, plus three definers of the prose word `all`.
func testLinkFixture(t *testing.T) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                      "module example.test/link\n\ngo 1.27.0\n",
		"src/render/render.go":        "package render\n\nfunc Render() {}\n",
		"tests/render/render_test.go": "package render\n\nfunc TestNothing() {}\n",
		"store/store.go":              "package store\n\nfunc Get() int { return 1 }\n",
		"probe/probe_test.go":         "package probe\n\nimport \"example.test/link/store\"\n\nfunc TestProbe() { _ = store.Get() }\n",
		"codec/frame.go":              "package codec\n\nfunc EncodeFrame() {}\n",
		"spec/wire_test.go":           "package spec\n\n// EncodeFrame is exercised here.\nfunc TestWire() {}\n",
		"queue/drain.go":              "package queue\n\nfunc DrainQueue() {}\n",
		"other/other_test.go":         "package other\n\nfunc TestDrainQueueTwice() {}\n",
		"alpha/alpha.go":              "package alpha\n\nfunc all() {}\n",
		"beta/beta.go":                "package beta\n\nfunc all() {}\n",
		"gamma/gamma.go":              "package gamma\n\nfunc all() {}\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func contextReasonOf(t *testing.T, packet map[string]any, kind string) (string, string) {
	t.Helper()
	for _, item := range mapsFromAny(packet["results"]) {
		if item["kind"] != kind {
			continue
		}
		evidence := mapsFromAny(item["evidence"])
		return item["id"].(string), evidence[0]["reason"].(string)
	}
	return "", ""
}

// TestTaskContextLinksTestsByEachSignal is TCP-V0-015's one-fixture-per-signal
// check: the test row names the signal that bound it, in both directions.
func TestTaskContextLinksTestsByEachSignal(t *testing.T) {
	index := testLinkFixture(t)
	cases := []struct{ task, path, reason string }{
		{"trace `Render`", "tests/render/render_test.go", "tests src/render/render.go: mirrored stem (test counterpart in the mirrored directory)"},
		{"trace `Get`", "probe/probe_test.go", "tests store/store.go: import edge"},
		{"trace `EncodeFrame`", "spec/wire_test.go", "tests codec/frame.go: names EncodeFrame (idf "},
		{"trace `DrainQueue`", "other/other_test.go", "tests queue/drain.go: test name TestDrainQueueTwice"},
		{"open other/other_test.go", "queue/drain.go", "is tested by other/other_test.go: test name TestDrainQueueTwice"},
	}
	for _, item := range cases {
		packet, err := TaskContext(context.Background(), index, item.task, "", 20)
		if err != nil {
			t.Fatal(err)
		}
		path, reason := contextReasonOf(t, packet, "test")
		if path != item.path || !strings.HasPrefix(reason, item.reason) {
			t.Fatalf("%q test row = %s %q, want %s %q", item.task, path, reason, item.path, item.reason)
		}
	}
}

// TestTaskContextReservesOneTestSlot pins the one-slot reservation and its
// TCP-V0-011 accounting: the named test anchors queue/drain.go, the lexical
// test anchor within the limit (path term `test`) yields one more source, and
// at a limit where the lexical fill (path term `go` reaches every Go file)
// stops short of it, that one is withheld.
func TestTaskContextReservesOneTestSlot(t *testing.T) {
	packet, err := TaskContext(context.Background(), testLinkFixture(t), "open other/other_test.go", "", 5)
	if err != nil {
		t.Fatal(err)
	}
	found := contextRowsByKind(t, packet)
	if !slices.Equal(found["test"], []string{"queue/drain.go"}) {
		t.Fatalf("test rows = %v, want the named anchor's source alone", found["test"])
	}
	states, withheld := contextUnexamined(t, contextCoverage(t, packet))
	if states["test"] != "examined" || contextIntValue(withheld["test"]) != 1 {
		t.Fatalf("test relation = %s / %v, want examined with one withheld", states["test"], withheld["test"])
	}
}

// TestTaskContextDefinitionSlotSkipsBacktickedProse: a backticked English
// function word admits no definition row (TCP-V0-015's definition-slot rule).
func TestTaskContextDefinitionSlotSkipsBacktickedProse(t *testing.T) {
	packet, err := TaskContext(context.Background(), testLinkFixture(t), "run `all` checks", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if found := contextRowsByKind(t, packet); len(found["definition"]) != 0 {
		t.Fatalf("definition rows = %v, want none for a backticked stop word", found["definition"])
	}
	if !definitionEligible(taskIdentifier{name: "Render", weight: 3}) || definitionEligible(taskIdentifier{name: "all", weight: 3}) {
		t.Fatal("a backticked code word stays eligible and a backticked stop word does not")
	}
	if slices.Contains(taskLexicalTerms("run `all` checks"), "all") {
		t.Fatal("`all` is a stop word for the lexical terms too")
	}
}
