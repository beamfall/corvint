package contextindex

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestFrameRelationSignals(t *testing.T) {
	t.Setenv("CORVINT_CONTEXT_FRAME_RELATION", "1")
	index := testLinkFixture(t)
	cases := []struct{ task, want, reason string }{
		{"TCP-V0-020 mirrored", "src/render/render.go", "mirrored stem"},
		{"TCP-V0-020 imported", "store/store.go", "import edge"},
		{"TCP-V0-020 identifier", "codec/frame.go", "names EncodeFrame"},
	}
	tasks := []string{"open tests/render/render_test.go", "trace probe/probe_test.go:3", "trace spec/wire_test.go:4"}
	for i, item := range cases {
		t.Run(item.task, func(t *testing.T) {
			packet, err := TaskContext(context.Background(), index, tasks[i], "", 5)
			if err != nil {
				t.Fatal(err)
			}
			path, reason := contextReasonOf(t, packet, "test")
			if path != item.want || !strings.Contains(reason, item.reason) {
				t.Fatalf("got %s: %s, want %s / %s", path, reason, item.want, item.reason)
			}
			if contextIsTest(path) {
				t.Fatal("frame relation returned the test itself")
			}
		})
	}
}

func frameRankFixture(t *testing.T) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                   "module example.test/frame\n\ngo 1.27.0\n",
		"AGENTS.md":                "# Instructions\nPreserve project authority.\n",
		"implementation/codec.go":  "package codec\nfunc EncodeFrame() {}\nfunc DecodeFrame() {}\n",
		"implementation/extra.go":  "package codec\nfunc ExpandFrame() {}\n",
		"implementation/final.go":  "package codec\nfunc FinishFrame() {}\n",
		"implementation/unused.go": "package codec\nfunc OtherFrame() {}\n",
		"spec/wire_test.go":        "package codec\nfunc TestWire() { EncodeFrame(); DecodeFrame(); ExpandFrame(); FinishFrame(); OtherFrame() }\n",
		"noise/alpha.go":           "package noise\nfunc NoiseAlpha() {}\n",
		"noise/beta.go":            "package noise\nfunc NoiseBeta() {}\n",
		"noise/gamma.go":           "package noise\nfunc NoiseGamma() {}\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func TestFrameRelationOrderingAndCoverage(t *testing.T) {
	t.Setenv("CORVINT_CONTEXT_FRAME_RELATION", "1")
	index := frameRankFixture(t)
	task := "spec/wire_test.go:4 spec/wire_test.go `NoiseAlpha` `NoiseBeta` `NoiseGamma`"
	t.Run("TCP-V0-020 implementation precedes irrelevant definitions", func(t *testing.T) {
		packet, err := TaskContext(context.Background(), index, task, "", 5)
		if err != nil {
			t.Fatal(err)
		}
		rows := mapsFromAny(packet["results"])
		if rows[0]["id"] != "AGENTS.md" || rows[1]["id"] != "implementation/codec.go" {
			t.Fatalf("unexpected order: %v", rows)
		}
		paths := []string{}
		for _, row := range rows {
			paths = append(paths, row["id"].(string))
		}
		unique := slices.Clone(paths)
		slices.Sort(unique)
		if len(slices.Compact(unique)) != len(paths) {
			t.Fatal("duplicate paths")
		}
		states, withheld := contextUnexamined(t, contextCoverage(t, packet))
		// The lexical fill admits the fourth frame candidate after the test
		// cap. It is not withheld by the test relation; final truncation is
		// disclosed separately by omitted_results.
		if states["test"] != "examined" || contextIntValue(withheld["test"]) != 0 {
			t.Fatalf("test coverage = %s / %v", states["test"], withheld["test"])
		}
	})
	t.Run("TCP-V0-020 final limit preserves governance", func(t *testing.T) {
		packet, err := TaskContext(context.Background(), index, task, "", 1)
		if err != nil {
			t.Fatal(err)
		}
		rows := mapsFromAny(packet["results"])
		if len(rows) != 1 || rows[0]["id"] != "AGENTS.md" {
			t.Fatalf("governance lost: %v", rows)
		}
	})
}

func TestFrameRelationScopeAndDeterminism(t *testing.T) {
	index := frameRankFixture(t)
	cases := []struct{ name, task, subject string }{
		{"TCP-V0-020 no named test is unchanged", "trace `EncodeFrame`", ""},
		{"TCP-V0-020 explicit subject is unchanged", "spec/wire_test.go", "implementation/codec.go"},
	}
	t.Run("TCP-V0-020 absent path creates no frame", func(t *testing.T) {
		t.Setenv("CORVINT_CONTEXT_FRAME_RELATION", "1")
		compiler := newTaskContextCompiler(index, "absent_test.go", "")
		rows, active := compiler.frameRelationRows()
		if active || len(rows) != 0 {
			t.Fatalf("absent frame produced evidence: %v", rows)
		}
	})
	t.Run("TCP-V0-020 absent and unknown flags ignore eligible frame", func(t *testing.T) {
		for _, flag := range []string{"", "unknown"} {
			t.Setenv("CORVINT_CONTEXT_FRAME_RELATION", flag)
			compiler := newTaskContextCompiler(index, "spec/wire_test.go", "")
			rows, active := compiler.frameRelationRows()
			if active || len(rows) != 0 {
				t.Fatalf("inactive flag %q produced frame evidence", flag)
			}
		}
	})
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			var previous []byte
			for _, flag := range []string{"", "unknown", "1"} {
				t.Setenv("CORVINT_CONTEXT_FRAME_RELATION", flag)
				packet, err := TaskContext(context.Background(), index, item.task, item.subject, 20)
				if err != nil {
					t.Fatal(err)
				}
				encoded, _ := json.Marshal(packet)
				if previous != nil && !bytes.Equal(previous, encoded) {
					t.Fatalf("out-of-scope flag %q changed packet", flag)
				}
				previous = encoded
			}
		})
	}
	t.Run("TCP-V0-020 deterministic identifier aggregation", func(t *testing.T) {
		t.Setenv("CORVINT_CONTEXT_FRAME_RELATION", "1")
		var previous []byte
		for range 20 {
			packet, err := TaskContext(context.Background(), index, "spec/wire_test.go:4", "", 5)
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(packet)
			if previous != nil && !bytes.Equal(previous, encoded) {
				t.Fatal("same input produced different packets")
			}
			previous = encoded
		}
	})
}

func TestFrameRelationAmbiguityAndCap(t *testing.T) {
	t.Setenv("CORVINT_CONTEXT_FRAME_RELATION", "1")
	files := map[string]string{"go.mod": "module example.test/cap\n\ngo 1.27.0\n"}
	for _, name := range []string{"alpha", "bravo", "charlie", "delta"} {
		files[name+"/wire_test.go"] = "package cap\nfunc TestWire() { " + name + "Frame() }\n"
		files["src/"+name+".go"] = "package cap\nfunc " + name + "Frame() {}\n"
	}
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("TCP-V0-020 ambiguous basename does not invent a frame", func(t *testing.T) {
		compiler := newTaskContextCompiler(index, "wire_test.go:3", "")
		if frames := compiler.namedTestFrames(); len(frames) != 0 {
			t.Fatalf("ambiguous basename resolved: %v", frames)
		}
	})
	t.Run("TCP-V0-020 bounded anchors disclose omitted scope", func(t *testing.T) {
		compiler := newTaskContextCompiler(index, "delta/wire_test.go bravo/wire_test.go alpha/wire_test.go charlie/wire_test.go", "")
		rows, active := compiler.frameRelationRows()
		if !active || compiler.relationState["test"] != "capped" || !compiler.slotOmitted {
			t.Fatal("anchor cap omitted its uncertainty")
		}
		paths := make([]string, 0, len(rows))
		for _, row := range rows {
			paths = append(paths, row.path)
		}
		if !slices.Equal(paths, []string{"src/alpha.go", "src/bravo.go", "src/charlie.go"}) {
			t.Fatalf("first-three sorted anchors = %v", paths)
		}
	})
}
