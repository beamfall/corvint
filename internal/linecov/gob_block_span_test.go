// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"bytes"
	"encoding/gob"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testevidence"
)

// V1-5371: every gob index format Decode still reads (linecov-index/2, linecov-index/3 and the
// RSP-V0-008 evidence extension of linecov-index/4) refuses a block whose Start is after its End,
// as the compact linecov-index/4 decoder does by construction, rather than admitting an inverted
// range that a line lookup would silently never match.
func TestDecodeRefusesInvertedBlockInGobFormats(t *testing.T) {
	inverted := []File{{Path: "src", OID: strings.Repeat("2", 40), Blocks: []Block{{Start: 5, StartCol: 1, End: 3, EndCol: 2, Tests: []int32{0}}}}}
	tests := []Test{{Package: "p", Name: "TestA"}}
	type oldIndex struct {
		Format, Language, Module, Commit, Runner string
		Tests                                    []Test
		Files                                    []File
	}
	cases := map[string]any{
		FormatV2: oldIndex{FormatV2, "go", "module", strings.Repeat("a", 40), "runner", tests, inverted},
		FormatV3: oldIndex{FormatV3, "go", "module", strings.Repeat("a", 40), "runner", tests, inverted},
	}
	r := evidenceFixture()
	cases[Format+" evidence extension"] = &Index{Format: Format, Module: "module", Commit: r.Commit, Runner: "runner", Tests: tests, Files: inverted, Evidence: []testevidence.Record{r}}
	for name, index := range cases {
		t.Run(name, func(t *testing.T) {
			var data bytes.Buffer
			if err := gob.NewEncoder(&data).Encode(index); err != nil {
				t.Fatal(err)
			}
			ix, err := Decode(&data)
			if err == nil {
				t.Fatalf("decoded an inverted block: %+v", ix.Files[0].Blocks[0])
			}
			if !strings.Contains(err.Error(), "block span") {
				t.Fatalf("refusal does not name the block span: %v", err)
			}
		})
	}
	// The same shape with Start <= End still decodes, so the refusal is the span alone.
	valid := []File{{Path: "src", OID: strings.Repeat("2", 40), Blocks: []Block{{Start: 3, StartCol: 1, End: 5, EndCol: 2, Tests: []int32{0}}}}}
	var data bytes.Buffer
	if err := gob.NewEncoder(&data).Encode(oldIndex{FormatV3, "go", "module", strings.Repeat("a", 40), "runner", tests, valid}); err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(&data); err != nil {
		t.Fatalf("valid linecov-index/3 refused: %v", err)
	}
}
