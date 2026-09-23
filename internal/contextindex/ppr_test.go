package contextindex

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
)

const graphFixtureTask = "Why does `AlphaWidget` in alpha/alpha.go break the widget pipeline?"

func graphUnexamined(packet map[string]any) map[string]any {
	for _, item := range mapsFromAny(packet["coverage"].(map[string]any)["unexamined"]) {
		if item["relation"] == contextGraphRelation {
			return item
		}
	}
	return nil
}

func TestContextGraphSlotRanksFromTheTaskAnchors(t *testing.T) {
	index := identGraphFixture(t)
	t.Setenv("CORVINT_CONTEXT_GRAPH", "on")
	packet, err := TaskContext(context.Background(), index, graphFixtureTask, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	results := mapsFromAny(packet["results"])
	t.Run("TCP-V0-031", func(t *testing.T) {
		found := contextRowsByKind(t, packet)
		if !slices.Equal(found[contextGraphRelation], []string{"beta/beta.go", "gamma/gamma.go"}) {
			t.Fatalf("graph rows = %v, want beta then gamma", found[contextGraphRelation])
		}
		seenGraph := false
		for _, item := range results {
			kind := item["kind"].(string)
			seenGraph = seenGraph || kind == contextGraphRelation
			if seenGraph && kind != contextGraphRelation && kind != "lexical" && kind != "documentation" {
				t.Fatalf("a %s row follows a graph row: the graph must not outrank a relation", kind)
			}
		}
		if state := graphUnexamined(packet); state == nil || state["state"] != "examined" {
			t.Fatalf("graph receipt = %v", state)
		}
	})
	t.Run("TCP-V0-032", func(t *testing.T) {
		for _, item := range results {
			if item["id"] != "gamma/gamma.go" {
				continue
			}
			reason := item["evidence"].([]any)[0].(map[string]any)["reason"].(string)
			want := "graph from seed `alpha/alpha.go` (mentioned): `alpha/alpha.go` defines `Quexel`, which `beta/beta.go` names; `beta/beta.go` defines `Brindle`, which `gamma/gamma.go` names"
			if reason != want {
				t.Fatalf("reason = %q\nwant %q", reason, want)
			}
			if !strings.Contains(item["action"].(string), reason) {
				t.Fatalf("action does not carry the hop path: %q", item["action"])
			}
			return
		}
		t.Fatal("no gamma row")
	})
	t.Run("TCP-V0-034", func(t *testing.T) {
		for _, limit := range []int{1, 3, 4} {
			bounded, err := TaskContext(context.Background(), index, graphFixtureTask, "", limit)
			if err != nil {
				t.Fatal(err)
			}
			rows := mapsFromAny(bounded["results"])
			if len(rows) > limit || rows[0]["id"] != "alpha/alpha.go" {
				t.Fatalf("limit %d: %d rows, first %v", limit, len(rows), rows[0]["id"])
			}
		}
		if packet["mutates"] != false {
			t.Fatalf("mutates = %v", packet["mutates"])
		}
	})
}

func TestContextGraphSlotAbstains(t *testing.T) {
	t.Run("TCP-V0-033", func(t *testing.T) {
		index := identGraphFixture(t)
		t.Setenv("CORVINT_CONTEXT_GRAPH", "on")
		packet, err := TaskContext(context.Background(), index, "the widget pipeline stalls", "", 20)
		if err != nil {
			t.Fatal(err)
		}
		if found := contextRowsByKind(t, packet); len(found[contextGraphRelation]) != 0 || graphUnexamined(packet)["state"] != "no-seed" {
			t.Fatalf("no anchor: graph rows %v, receipt %v", found[contextGraphRelation], graphUnexamined(packet))
		}
		index.Vocabulary.IdentGraph = &identGraph{Nodes: index.Vocabulary.IdentGraph.Nodes, Bounded: true, Offsets: []uint32{0}}
		packet, err = TaskContext(context.Background(), index, graphFixtureTask, "", 20)
		if err != nil {
			t.Fatal(err)
		}
		if found := contextRowsByKind(t, packet); len(found[contextGraphRelation]) != 0 || graphUnexamined(packet)["state"] != "graph-bounded" {
			t.Fatalf("bounded graph: graph rows %v, receipt %v", found[contextGraphRelation], graphUnexamined(packet))
		}
		graph := identGraphFixture(t).Vocabulary.IdentGraph
		if _, converged := personalizedPageRank(graph, []contextGraphSeed{{node: 0, anchor: "mentioned"}}, 0); converged {
			t.Fatal("a walk past its push bound ranked")
		}
		unsupported, err := TaskContext(context.Background(), identGraphFixture(t), "Does `httpRetry` call `retry_loop` inside the widget?", "", 20)
		if err != nil {
			t.Fatal(err)
		}
		if len(unsupported["results"].([]any)) != 0 {
			t.Fatalf("TCP-V0-016 abstention changed under the graph flag: %v", contextRowsByKind(t, unsupported))
		}
	})
}

func TestContextGraphDefaultBytes(t *testing.T) {
	t.Run("TCP-V0-034", func(t *testing.T) {
		index := recipeFixtureIndex(t)
		golden, err := os.ReadFile("testdata/context-recipe-default-golden.json")
		if err != nil {
			t.Fatal(err)
		}
		for _, flag := range []string{"", "off", "unknown"} {
			t.Setenv("CORVINT_CONTEXT_GRAPH", flag)
			if newTaskContextCompiler(index, recipeFixtureTask, "").graphRanking {
				t.Fatalf("flag %q enabled the graph slot", flag)
			}
			packet := recipePacket(t, index, "", recipeFixtureTask)
			encoded, err := json.MarshalIndent(packet, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(append(encoded, '\n'), golden) || graphUnexamined(packet) != nil {
				t.Fatalf("flag %q changed golden packet bytes", flag)
			}
		}
	})
}
