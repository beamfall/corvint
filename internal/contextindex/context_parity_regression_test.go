package contextindex

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestContextColdBuildRetainsTestImportWinner(t *testing.T) {
	cases := []struct {
		name, source, alpha, zeta, definition, plain, imported string
	}{
		{"Go", "impl/service.go", "tests/alpha_test.go", "tests/zeta_test.go", "package impl\nfunc HandleFrame() {}\n", "package tests\nfunc TestAlpha() { HandleFrame() }\n", "package tests\nimport _ \"example.test/parity/impl\"\nfunc TestZeta() { HandleFrame() }\n"},
		{"Python", "impl/service.py", "tests/test_alpha.py", "tests/test_zeta.py", "def HandleFrame():\n    pass\n", "def test_alpha():\n    HandleFrame()\n", "import impl.service\ndef test_zeta():\n    HandleFrame()\n"},
		{"TypeScript", "impl/service.ts", "tests/alpha.test.ts", "tests/zeta.test.ts", "export function HandleFrame() {}\n", "function test_alpha() { HandleFrame(); }\n", "import '../impl/service';\nfunction test_zeta() { HandleFrame(); }\n"},
	}
	for _, item := range cases {
		t.Run("TCP-V0-015 "+item.name, func(t *testing.T) {
			t.Setenv("CORVINT_CONTEXT_TERMS", "")
			t.Setenv("CORVINT_CONTEXT_FRAME_RELATION", "")
			root := impactRepositoryWithFiles(t, map[string]string{
				"go.mod":         "module example.test/parity\n\ngo 1.27.0\n",
				"other/other.go": "package other\nfunc Elsewhere() {}\n",
				item.source:      item.definition, item.alpha: item.plain, item.zeta: item.imported,
			})
			full, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			linker := newTaskContextCompiler(full, item.source, "").newTestLinker()
			candidates := linker.candidates(item.source)
			if len(candidates) < 2 || candidates[0].path != item.zeta || !candidates[0].imports {
				t.Fatalf("fixture must rank the later import witness first: %+v", candidates)
			}
			for _, subject := range []string{"", "other/other.go"} {
				cold, err := BuildContext(context.Background(), root, subject)
				if err != nil {
					t.Fatal(err)
				}
				want := contextParityPacket(t, full, item.source, subject)
				got := contextParityPacket(t, cold, item.source, subject)
				if !bytes.Equal(got, want) {
					t.Fatalf("cold/full packet differs for subject %q:\nfull %s\ncold %s", subject, want, got)
				}
			}
		})
	}
}

func TestContextEqualIDFTestEvidenceIsStable(t *testing.T) {
	t.Run("TCP-V0-004 TCP-V0-007", func(t *testing.T) {
		t.Setenv("CORVINT_CONTEXT_TERMS", "")
		t.Setenv("CORVINT_CONTEXT_FRAME_RELATION", "")
		root := impactRepositoryWithFiles(t, map[string]string{
			"go.mod":               "module example.test/ties\n\ngo 1.27.0\n",
			"impl/engine.go":       "package impl\nfunc AlphaFrame() {}\nfunc BetaFrame() {}\n",
			"checks/probe_test.go": "package checks\nfunc TestProbe() { BetaFrame(); AlphaFrame() }\n",
		})
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		table := index.vocabulary()
		lowA, highA, okA := table.Words.find("AlphaFrame")
		lowB, highB, okB := table.Words.find("BetaFrame")
		if !okA || !okB || highA-lowA != highB-lowB || highA-lowA != 2 {
			t.Fatal("fixture must give the two names identical two-file IDF")
		}
		var expected []byte
		for run := 0; run < 64; run++ {
			compiler := newTaskContextCompiler(index, "checks/probe_test.go", "")
			candidates := compiler.newTestLinker().candidates("checks/probe_test.go")
			if len(candidates) != 1 || candidates[0].rarest != "AlphaFrame" {
				t.Fatalf("run %d: lexical equal-IDF witness must be AlphaFrame: %+v", run, candidates)
			}
			packet := contextParityPacket(t, index, "checks/probe_test.go", "")
			if !strings.Contains(string(packet), "rarest AlphaFrame") {
				t.Fatalf("packet did not carry the tied witness: %s", packet)
			}
			if run == 0 {
				expected = packet
			} else if !bytes.Equal(packet, expected) {
				t.Fatalf("run %d changed complete packet bytes", run)
			}
		}
	})
}

func contextParityPacket(t *testing.T, index *Index, task, subject string) []byte {
	t.Helper()
	packet, err := TaskContext(context.Background(), index, task, subject, 20)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
