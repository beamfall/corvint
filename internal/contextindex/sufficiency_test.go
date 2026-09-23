package contextindex

import (
	"context"
	"reflect"
	"testing"
)

// TestContextSufficiency is table-driven over hand-built span sets, forged
// ones included: the set reads satisfied only when every anchor's evidence is
// in a selected span's own pinned lines, and every other anchor is named.
func TestContextSufficiency(t *testing.T) {
	t.Run("TCP-V0-028", func(t *testing.T) {
		files := map[string]string{"assets/logo.png": "\x89PNG\x00\x00\x00\x01binary"}
		for path, content := range spanFixtureFiles {
			files[path] = content
		}
		index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
		if err != nil {
			t.Fatal(err)
		}
		if _, indexed := index.Sources["assets/logo.png"]; indexed {
			t.Fatal("fixture precondition: the binary file must be tracked but not indexed")
		}
		const named = "Why does `crankOnce` in pkg/core/engine.go overflow?"
		engine := func(start, end int) contextSpan {
			return contextSpan{role: "core", path: "pkg/core/engine.go", start: start, end: end}
		}
		for _, test := range []struct {
			name, task, verdict string
			spans               []contextSpan
			missing             []string
		}{
			{"span carries every anchor", named, sufficiencySatisfied, []contextSpan{engine(8, 10)}, []string{}},
			{"no span selected", named, sufficiencyInsufficient, nil, []string{"pkg/core/engine.go", "crankOnce"}},
			{"span on the path lacks the name", named, sufficiencyInsufficient, []contextSpan{engine(1, 3)}, []string{"crankOnce"}},
			{"forged range past the end", named, sufficiencyInsufficient, []contextSpan{engine(8, 99)}, []string{"crankOnce"}},
			{"forged inverted range", named, sufficiencyInsufficient, []contextSpan{engine(9, 8)}, []string{"crankOnce"}},
			{"span on another path", named, sufficiencyInsufficient,
				[]contextSpan{{role: "core", path: "pkg/app/main.go", start: 1, end: 8}}, []string{"pkg/core/engine.go", "crankOnce"}},
			{"name nothing indexed carries", "Why does `zzUnseenThing` break?", sufficiencyUnknown, nil, []string{"zzUnseenThing"}},
			{"no anchors", "why is it slow", sufficiencyUnknown, []contextSpan{engine(8, 10)}, []string{}},
			{"satisfied plus unknown", "Why does `crankOnce` call `zzUnseenThing`?", sufficiencyUnknown,
				[]contextSpan{engine(8, 10)}, []string{"zzUnseenThing"}},
			{"tracked path with no indexed source", "Why is assets/logo.png blurry?", sufficiencyUnknown,
				[]contextSpan{engine(8, 10)}, []string{"assets/logo.png"}},
		} {
			t.Run(test.name, func(t *testing.T) {
				compiler := newTaskContextCompiler(index, test.task, "")
				compiler.answerability = compiler.answer()
				ranker := newSpanRanker(compiler, nil)
				verdict := compiler.sufficiency(test.spans, ranker.lines)
				packet := verdict.packet()
				missing := make([]string, 0)
				for _, name := range packet["missing"].([]any) {
					missing = append(missing, name.(string))
				}
				if verdict.verdict != test.verdict || !reflect.DeepEqual(missing, test.missing) {
					t.Fatalf("verdict %q missing %v, want %q %v; anchors %#v", verdict.verdict, missing, test.verdict, test.missing, verdict.anchors)
				}
				for _, anchor := range verdict.anchors {
					if verdict.verdict == sufficiencySatisfied && anchor.state != sufficiencySatisfied {
						t.Fatalf("satisfied set holds a %s anchor: %#v", anchor.state, anchor)
					}
				}
				if packet["missing_total"] != len(test.missing) || packet["anchors_total"] != len(verdict.anchors) {
					t.Fatalf("totals = %v / %v", packet["missing_total"], packet["anchors_total"])
				}
			})
		}
	})
}
