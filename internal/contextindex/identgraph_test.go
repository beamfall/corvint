package contextindex

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

// identGraphFixture is a two-hop chain: alpha defines Quexel, which beta
// names; beta defines Brindle, which gamma names. The fill files carry the
// task's prose terms so the lexical fill has rows for the graph to follow.
func identGraphFixture(t *testing.T) *Index {
	t.Helper()
	files := map[string]string{
		"go.mod":         "module example.test/graph\n\ngo 1.27.0\n",
		"alpha/alpha.go": "package alpha\n\n// AlphaWidget breaks the widget pipeline.\ntype AlphaWidget struct{}\n\nfunc Quexel() int { return 1 }\n",
		"beta/beta.go":   "package beta\n\nfunc Brindle() int { return Quexel() }\n",
		"gamma/gamma.go": "package gamma\n\nfunc Gorpish() int { return Brindle() }\n",
		"delta/delta.go": "package delta\n\nfunc Lonesome() {}\n",
	}
	for number := 1; number <= 6; number++ {
		files[fmt.Sprintf("fill/fill%d.go", number)] = fmt.Sprintf("package fill\n\n// widget pipeline note %d\nfunc Filler%d() {}\n", number, number)
	}
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	return index
}

// graphEdge finds the edge from one path to another, or fails.
func graphEdge(t *testing.T, index *Index, from, to string) (uint32, string, bool) {
	t.Helper()
	table, graph := index.Vocabulary, index.Vocabulary.IdentGraph
	source, _ := table.sourceID(from)
	target, _ := table.sourceID(to)
	low, high := graph.edges(uint32(source))
	for edge := low; edge < high; edge++ {
		if graph.Targets[edge] == uint32(target) {
			name, refers := graph.name(graph.Labels[edge])
			return graph.Weights[edge], name, refers
		}
	}
	t.Fatalf("no edge %s -> %s", from, to)
	return 0, "", false
}

func TestIdentGraphIsDerivedDeterministicallyFromTheIndex(t *testing.T) {
	t.Run("TCP-V0-030", func(t *testing.T) {
		index := identGraphFixture(t)
		graph := index.Vocabulary.IdentGraph
		if graph == nil || graph.Bounded || graph.check(len(index.Vocabulary.Paths)) != nil {
			t.Fatalf("compile left no valid graph: %+v", graph)
		}
		if weight, name, refers := graphEdge(t, index, "beta/beta.go", "alpha/alpha.go"); weight != 1 || name != "Quexel" || !refers {
			t.Fatalf("beta -> alpha = %d %q refers=%v", weight, name, refers)
		}
		if _, name, refers := graphEdge(t, index, "alpha/alpha.go", "beta/beta.go"); name != "Quexel" || refers {
			t.Fatalf("alpha -> beta = %q refers=%v, want the defining direction", name, refers)
		}
		if _, name, _ := graphEdge(t, index, "gamma/gamma.go", "beta/beta.go"); name != "Brindle" {
			t.Fatalf("gamma -> beta = %q", name)
		}
		lonesome, _ := index.Vocabulary.sourceID("delta/delta.go")
		if low, high := graph.edges(uint32(lonesome)); low != high {
			t.Fatalf("delta has %d edges, want none", high-low)
		}
		encoded, err := graph.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		shuffled := *index
		shuffled.Symbols = append([]Symbol(nil), index.Symbols...)
		rand.New(rand.NewSource(7)).Shuffle(len(shuffled.Symbols), func(left, right int) {
			shuffled.Symbols[left], shuffled.Symbols[right] = shuffled.Symbols[right], shuffled.Symbols[left]
		})
		again, err := shuffled.buildIdentGraph(index.Vocabulary).MarshalBinary()
		if err != nil || !bytes.Equal(encoded, again) {
			t.Fatalf("symbol order changed the graph encoding: %v", err)
		}
		var decoded identGraph
		if err := decoded.UnmarshalBinary(encoded); err != nil || !reflect.DeepEqual(&decoded, graph) {
			t.Fatalf("round trip differs: %v", err)
		}
	})
}

func TestIdentGraphRefusesDamagedEncodings(t *testing.T) {
	t.Run("TCP-V0-030", func(t *testing.T) {
		index := identGraphFixture(t)
		sources := len(index.Vocabulary.Paths)
		encoded, err := index.Vocabulary.IdentGraph.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		var truncated identGraph
		if truncated.UnmarshalBinary(encoded[:len(encoded)-1]) == nil {
			t.Fatal("a truncated encoding decoded")
		}
		damaged := *index.Vocabulary.IdentGraph
		damaged.Targets = append([]uint32(nil), damaged.Targets...)
		damaged.Targets[0] = uint32(sources)
		if damaged.check(sources) == nil {
			t.Fatal("an edge past the last node passed the check")
		}
		if index.Vocabulary.IdentGraph.check(sources+1) == nil {
			t.Fatal("a graph over different paths passed the check")
		}
	})
}

func TestIdentGraphBounds(t *testing.T) {
	t.Run("TCP-V0-033", func(t *testing.T) {
		table := &TermTable{Paths: make([]string, identGraphMaxNodes+1)}
		if graph := (&Index{}).buildIdentGraph(table); !graph.Bounded || len(graph.Targets) != 0 {
			t.Fatalf("a tree past the node bound built edges: bounded=%v", graph.Bounded)
		}
		files := map[string]string{"go.mod": "module example.test/common\n\ngo 1.27.0\n"}
		for number := 0; number <= identGraphMaxDefiners; number++ {
			files[fmt.Sprintf("p%d/common.go", number)] = fmt.Sprintf("package p%d\n\nfunc Commonplace() {}\n", number)
		}
		files["user/user.go"] = "package user\n\nfunc Uses() { Commonplace() }\n"
		index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
		if err != nil {
			t.Fatal(err)
		}
		if graph := index.Vocabulary.IdentGraph; len(graph.Targets) != 0 {
			t.Fatalf("a name with %d definers joined files: %d edges", identGraphMaxDefiners+1, len(graph.Targets))
		}
	})
}

func TestIdentGraphNameEligibility(t *testing.T) {
	t.Run("TCP-V0-030", func(t *testing.T) {
		for name, want := range map[string]bool{
			"Quexel": true, "retry_loop": true, "httpRetry": true, "V2Name": true,
			"seed": false, "unexamined": false, "Foo": false, "naïve_x": false, "9lives": false,
		} {
			if identGraphName(name) != want {
				t.Errorf("identGraphName(%q) = %v, want %v", name, !want, want)
			}
		}
	})
}
