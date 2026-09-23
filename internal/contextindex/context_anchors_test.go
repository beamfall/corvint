package contextindex

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// anchorFixtureIndex holds, per anchor class, one source carrying the literal
// verbatim and one carrying only its split tokens.
func anchorFixtureIndex(t *testing.T) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":          "module example.test/anchors\n\ngo 1.27.0\n",
		"AGENTS.md":       "# Standing instructions\n\nRead before editing.\n",
		"dial.go":         "package sample\n\nvar errDial = errors.New(\"connection refused: dial tcp\")\n",
		"dial_split.go":   "package sample\n\n// connection refused dial tcp\n",
		"client.go":       "package sample\n\nconst endpoint = \"https://api.example.test/v1/items\"\n",
		"client_split.go": "package sample\n\n// https api example test v1 items\n",
		"state.rs":        "fn main() { let s = Status::Active; }\n",
		"state_split.rs":  "// Status Active\n",
		"config.go":       "package sample\n\nvar port = get(\"server.http.port\")\n",
		"config_split.go": "package sample\n\n// server http port\n",
		"config_super.go": "package sample\n\n// xserver.http.port\n",
		"trace.txt":       "goroutine 1 [running]:\nmain.run()\n\tcmd/app/main.go:42 +0x1d\n",
		"trace_split.txt": "cmd/app/main.go line 42\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func anchorHitsByPath(hits []lexicalHit) map[string][]anchorHit {
	found := map[string][]anchorHit{}
	for _, hit := range hits {
		found[hit.path] = hit.anchors
	}
	return found
}

func TestContextAnchorClassesMatchVerbatim(t *testing.T) {
	t.Run("TCP-V0-022", func(t *testing.T) {
		t.Setenv("CORVINT_CONTEXT_ANCHORS", "on")
		index := anchorFixtureIndex(t)
		cases := []struct {
			class, task, literal, verbatim string
			split                          []string
		}{
			{"error", `Fix the "connection refused: dial tcp" error in the client`, "connection refused: dial tcp", "dial.go", []string{"dial_split.go"}},
			{"url", "Why does https://api.example.test/v1/items return 404?", "https://api.example.test/v1/items", "client.go", []string{"client_split.go"}},
			{"enum", "Handle Status::Active in the state machine", "Status::Active", "state.rs", []string{"state_split.rs"}},
			{"config", "The key server.http.port is ignored on reload", "server.http.port", "config.go", []string{"config_split.go", "config_super.go"}},
			{"frame", "Panic at cmd/app/main.go:42 when the queue drains", "cmd/app/main.go:42", "trace.txt", []string{"trace_split.txt"}},
		}
		for _, item := range cases {
			t.Run(item.class, func(t *testing.T) {
				anchors := taskAnchors(item.task)
				if !reflect.DeepEqual(anchors, []taskAnchor{{class: item.class, literal: item.literal}}) {
					t.Fatalf("anchors = %#v", anchors)
				}
				found := anchorHitsByPath(newTaskContextCompiler(index, item.task, "").lexicalHits())
				if !reflect.DeepEqual(found[item.verbatim], []anchorHit{{literal: item.literal, count: 1}}) {
					t.Fatalf("%s anchors = %#v", item.verbatim, found[item.verbatim])
				}
				for _, path := range item.split {
					if len(found[path]) != 0 {
						t.Fatalf("%s credited with %#v", path, found[path])
					}
				}
			})
		}
	})
}

func TestContextAnchorsExtractionBounds(t *testing.T) {
	t.Run("TCP-V0-022", func(t *testing.T) {
		// A URL inside a quoted string is one error anchor; a bare file name
		// is not a config key; e.g. and i.e. are under the minimum; the
		// per-task cap and the duplicate rule hold.
		anchors := taskAnchors(`See "open https://x.test/a failed", e.g. parser.go and i.e. log.level then log.level`)
		want := []taskAnchor{{class: "error", literal: "open https://x.test/a failed"}, {class: "config", literal: "log.level"}}
		if !reflect.DeepEqual(anchors, want) {
			t.Fatalf("anchors = %#v, want %#v", anchors, want)
		}
		many := make([]string, 0, contextAnchorCap+4)
		for index := range contextAnchorCap + 4 {
			many = append(many, "key"+strings.Repeat("x", index+1)+".value")
		}
		if got := len(taskAnchors(strings.Join(many, " "))); got != contextAnchorCap {
			t.Fatalf("cap = %d, want %d", got, contextAnchorCap)
		}
		if countAnchor("a.b a.bc xa.b a.b", "a.b") != 2 {
			t.Fatal("whole-anchor rule miscounted")
		}
	})
}

func TestContextAnchorsExplainAndNeverOutrankAuthority(t *testing.T) {
	t.Run("TCP-V0-022", func(t *testing.T) {
		t.Setenv("CORVINT_CONTEXT_ANCHORS", "on")
		index := anchorFixtureIndex(t)
		packet, err := TaskContext(context.Background(), index, "Why does https://api.example.test/v1/items return 404?", "", 20)
		if err != nil {
			t.Fatal(err)
		}
		results := mapsFromAny(packet["results"])
		if len(results) < 2 || results[0]["id"] != "AGENTS.md" {
			t.Fatalf("authority row is not first: %#v", results)
		}
		var anchorRow map[string]any
		for _, row := range results {
			if row["id"] == "client.go" {
				anchorRow = row
			}
		}
		if anchorRow == nil {
			t.Fatalf("no anchor row: %#v", results)
		}
		entry := mapsFromAny(anchorRow["evidence"])[0]
		reason := entry["reason"].(string)
		if anchorRow["kind"] != "lexical" || anchorRow["score"].(int) >= results[0]["score"].(int) || entry["authority"] != "vocabulary" {
			t.Fatalf("anchor row outranks authority: %#v", anchorRow)
		}
		if !strings.HasPrefix(reason, "anchor: `https://api.example.test/v1/items` x1 verbatim; ") {
			t.Fatalf("reason = %q", reason)
		}
	})
}

func TestContextAnchorsDefaultBytes(t *testing.T) {
	t.Run("TCP-V0-022", func(t *testing.T) {
		index := recipeFixtureIndex(t)
		golden, err := os.ReadFile("testdata/context-recipe-default-golden.json")
		if err != nil {
			t.Fatal(err)
		}
		for _, flag := range []string{"", "off", "unknown"} {
			t.Setenv("CORVINT_CONTEXT_ANCHORS", flag)
			if compiler := newTaskContextCompiler(index, `"quoted literal" https://x.test/a`, ""); compiler.anchors != nil {
				t.Fatalf("flag %q extracted anchors: %#v", flag, compiler.anchors)
			}
			packet := recipePacket(t, index, "", recipeFixtureTask)
			encoded, err := json.MarshalIndent(packet, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(append(encoded, '\n'), golden) {
				t.Fatalf("flag %q changed golden packet bytes", flag)
			}
		}
	})
}
