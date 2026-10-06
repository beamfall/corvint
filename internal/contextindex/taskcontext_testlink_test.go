package contextindex

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// testLinkFixture holds one code/test pairing per TCP-V0-015 signal, each
// bound by that signal alone, plus three definers of the prose word `all`.
func testLinkFixture(t *testing.T) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                    "module example.test/link\n\ngo 1.27.0\n",
		"src/render/render.go":      "package render\n\nfunc Render() {}\n",
		"src/render/render_test.go": "package render\n\nfunc TestNothing() {}\n",
		"store/store.go":            "package store\n\nfunc Get() int { return 1 }\n",
		"probe/probe_test.go":       "package probe\n\nimport \"example.test/link/store\"\n\nfunc TestProbe() { _ = store.Get() }\n",
		"codec/frame.go":            "package codec\n\nfunc EncodeFrame() {}\n",
		"spec/wire_test.go":         "package spec\n\n// EncodeFrame is exercised here.\nfunc TestWire() {}\n",
		"queue/drain.go":            "package queue\n\nfunc DrainQueue() {}\n",
		"other/other_test.go":       "package other\n\nfunc TestDrainQueueTwice() {}\n",
		"alpha/alpha.go":            "package alpha\n\nfunc all() {}\n",
		"beta/beta.go":              "package beta\n\nfunc all() {}\n",
		"gamma/gamma.go":            "package gamma\n\nfunc all() {}\n",
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
		{"trace `Render`", "src/render/render_test.go", "tests src/render/render.go: mirrored stem (test counterpart)"},
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

// TestTaskContextRefusesAOnePlainWordTestLink is V1-0343's frozen negative:
// a test whose only link to the anchor is one plain declared word (`down` in
// a comment) gets no test row, and a lexical-only link that TCP-V0-015 still
// admits (one compound name) says to check the test, not to update it.
func TestTaskContextRefusesAOnePlainWordTestLink(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                "module example.test/plain\n\ngo 1.27.0\n",
		"motion/motion.go":      "package motion\n\nfunc down() {}\n",
		"ext/extension_test.go": "package ext\n\n// Scroll down to the footer before asserting.\nfunc TestFooter() {}\n",
		"codec/frame.go":        "package codec\n\nfunc EncodeFrame() {}\n",
		"spec/wire_test.go":     "package spec\n\n// EncodeFrame is exercised here.\nfunc TestWire() {}\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := TaskContext(context.Background(), index, "open motion/motion.go", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if found := contextRowsByKind(t, packet); slices.Contains(found["test"], "ext/extension_test.go") {
		t.Fatalf("test rows = %v, want no row for the one-plain-word link", found["test"])
	}
	packet, err = TaskContext(context.Background(), index, "trace `EncodeFrame`", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range mapsFromAny(packet["results"]) {
		if item["kind"] == "test" && item["id"] == "spec/wire_test.go" {
			if action := item["action"].(string); !strings.HasPrefix(action, "Check this test: it tests codec/frame.go: names EncodeFrame") {
				t.Fatalf("lexical-only test action = %q, want the conditional form", action)
			}
			return
		}
	}
	t.Fatal("the one-compound-name link stays admitted")
}

func TestTaskContextSelectedLexicalPairs(t *testing.T) {
	files := map[string]string{"go.mod": "module example.test/pairs\n\ngo 1.27.0\n", "AGENTS.md": "Follow repository instructions.\n"}
	for i := 0; i < 5; i++ {
		files[fmt.Sprintf("source%d.go", i)] = fmt.Sprintf("package fixture\n// Record response status when flushing.\nfunc Action%d() {}\n", i)
		files[fmt.Sprintf("source%d_test.go", i)] = fmt.Sprintf("package fixture\nfunc TestAction%d() {}\n", i)
	}
	for i := 0; i < 12; i++ {
		files[fmt.Sprintf("unrelated%02d_test.go", i)] = fmt.Sprintf("package fixture\n// response status\nfunc TestUnrelated%d() {}\n", i)
	}
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("TCP-V0-004 counterpart admission respects limit and anchor", func(t *testing.T) {
		for _, limit := range []int{1, 2, 4, 8, 12, 30} {
			packet, err := TaskContext(context.Background(), index, "record response status when flushing", "", limit)
			if err != nil {
				t.Fatal(err)
			}
			rows := mapsFromAny(packet["results"])
			if len(rows) > limit || rows[0]["id"] != "AGENTS.md" {
				t.Fatalf("limit %d: %v", limit, rows)
			}
			present := map[string]bool{}
			for _, row := range rows {
				present[row["id"].(string)] = true
			}
			pairs := 0
			for _, row := range rows {
				if row["kind"] != "pair" {
					continue
				}
				pairs++
				evidence := mapsFromAny(row["evidence"])[0]
				anchor := strings.TrimPrefix(evidence["reason"].(string), "test counterpart of ")
				if !present[anchor] || evidence["authority"] != "test-convention" {
					t.Fatalf("orphaned or mislabeled counterpart: %v", row)
				}
			}
			sources, counterparts := 0, 0
			for _, row := range rows {
				id := row["id"].(string)
				switch {
				case strings.HasPrefix(id, "source") && strings.HasSuffix(id, "_test.go"):
					counterparts++
				case strings.HasPrefix(id, "source"):
					sources++
				}
			}
			// The test slot may hold one counterpart on its own (limit 2 carries
			// AGENTS.md and source0_test.go); the pair pass never exceeds the
			// selected sources.
			if pairs > sources {
				t.Fatalf("limit %d: %d pairs for %d selected sources", limit, pairs, sources)
			}
			// AGENTS.md, the five sources and six unrelated tests fill limit 12;
			// every source outranks every unrelated test, so all five
			// counterparts (one already held by the test slot) displace the
			// five weakest unrelated tests.
			if (limit == 12 || limit == 30) && counterparts != 5 {
				t.Fatalf("limit %d: rank-relative displacement admitted %d of 5 counterparts: %v", limit, counterparts, rows)
			}
		}
	})
	t.Run("TCP-V0-011 pair shortage remains visible", func(t *testing.T) {
		// Limit 8 holds AGENTS.md, five sources and two tests, so three
		// counterparts have no weaker unrelated test to displace.
		packet, err := TaskContext(context.Background(), index, "record response status when flushing", "", 8)
		if err != nil {
			t.Fatal(err)
		}
		states, withheld := contextUnexamined(t, contextCoverage(t, packet))
		if states["pair"] != "examined" || contextIntValue(withheld["pair"]) < 1 {
			t.Fatalf("missing pair shortage: %v / %v", states, withheld)
		}
	})
}

func TestTaskContextLexicalPairPromotion(t *testing.T) {
	t.Run("TCP-V0-004 existing lexical counterpart precedes unrelated tests", func(t *testing.T) {
		compiler := newTaskContextCompiler(testLinkFixture(t), "render", "")
		rows := []contextRow{{kind: "lexical", path: "src/render/render.go"}, {kind: "lexical", path: "probe/probe_test.go"}, {kind: "lexical", path: "src/render/render_test.go"}}
		for _, row := range rows {
			compiler.chosen[row.path] = struct{}{}
		}
		got := compiler.admitLexicalPairs(rows, 3)
		if len(got) != 3 || got[1].path != "src/render/render_test.go" || got[1].kind != "pair" || got[2].path != "probe/probe_test.go" {
			t.Fatalf("promotion: %v", got)
		}
	})
}

func TestTaskContextLexicalStrengthOrder(t *testing.T) {
	t.Run("TCP-V0-014 BM25 strength survives subjectless final ordering", func(t *testing.T) {
		root := impactRepositoryWithFiles(t, map[string]string{"go.mod": "module example.test/lexical\n\ngo 1.27.0\n", "a.go": "package fixture\n// response\nfunc Alpha() {}\n", "z.go": "package fixture\n// record response status when flushing\nfunc Zeta() {}\n"})
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		packet, err := TaskContext(context.Background(), index, "record response status when flushing", "", 2)
		if err != nil {
			t.Fatal(err)
		}
		rows := mapsFromAny(packet["results"])
		if len(rows) != 2 || rows[0]["id"] != "z.go" || rows[1]["id"] != "a.go" {
			t.Fatalf("lexical order: %v", rows)
		}
	})
}
