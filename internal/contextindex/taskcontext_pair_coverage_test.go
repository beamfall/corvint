package contextindex

import (
	"context"
	"slices"
	"testing"
)

func TestTaskContextReportsNamedPathPairScope(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		counterpart bool
	}{
		{name: "TCP-V0-011 named path without counterpart is examined"},
		{name: "TCP-V0-011 named path with counterpart is examined", counterpart: true},
	} {
		files := map[string]string{"widget.go": "package widget\n"}
		want := []string{"mentioned widget.go"}
		if testCase.counterpart {
			files["widget_test.go"] = "package widget\n"
			want = append(want, "pair widget_test.go")
		}
		t.Run(testCase.name, func(t *testing.T) {
			root := impactRepositoryWithFiles(t, files)
			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			packet, err := TaskContext(context.Background(), index, "widget.go", "", 20)
			if err != nil {
				t.Fatal(err)
			}
			if got := contextPairs(t, packet); !slices.Equal(got, want) {
				t.Fatalf("results = %v, want %v", got, want)
			}
			states, withheld := contextUnexamined(t, contextCoverage(t, packet))
			if states["pair"] != "examined" || contextIntValue(withheld["pair"]) != 0 {
				t.Fatalf("pair scope = %v / %v, want examined / 0", states["pair"], withheld["pair"])
			}
		})
	}
}

// Isolate the mention slot so later lexical or test admission cannot mask the
// two materialised counterparts held back by its three-row cap (TCP-V0-011).
func TestTaskContextReportsNamedPathPairSlotOmissions(t *testing.T) {
	t.Run("TCP-V0-011 named path reports capped counterparts", func(t *testing.T) {
		index := &Index{Tracked: map[string]struct{}{
			"widget.go": {}, "a/widget_test.go": {}, "b/widget_test.go": {},
			"c/widget_test.go": {}, "d/widget_test.go": {},
		}}
		compiler := newTaskContextCompiler(index, "widget.go", "")
		compiler.markState("subject-absent", "pair")
		rows := compiler.takeSlot(nil, compiler.mentionRows(), contextMentionCap)
		got := make([]string, 0, len(rows))
		for _, row := range rows {
			got = append(got, row.kind+" "+row.path)
		}
		want := []string{"mentioned widget.go", "pair a/widget_test.go", "pair b/widget_test.go"}
		if !slices.Equal(got, want) {
			t.Fatalf("results = %v, want %v", got, want)
		}
		if len(compiler.candidates["pair"]) != 4 {
			t.Fatalf("materialised pairs = %v, want four counterparts", compiler.candidates["pair"])
		}
		states, withheld := contextUnexamined(t, map[string]any{"unexamined": compiler.unexamined()})
		if states["pair"] != "examined" || contextIntValue(withheld["pair"]) != 2 {
			t.Fatalf("pair scope = %v / %v, want examined / 2", states["pair"], withheld["pair"])
		}
	})
}
