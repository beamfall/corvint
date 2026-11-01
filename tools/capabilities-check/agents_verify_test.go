package main

import (
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dogfoodflow"
)

// ASD-V0-905 (V1-3446): every command of the AGENTS.md Verify block is one a
// dogfood VERIFY file admits, so a lane can copy the block as it is; the
// nested module runs through go -C, not a subshell.
func TestASDV0905AgentsVerifyBlockIsAdmitted(t *testing.T) {
	data, err := os.ReadFile("../../AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	marker := "each in a form a dogfood `VERIFY` file accepts (`ASD-V0-905`):\n\n```sh\n"
	start := strings.Index(text, marker)
	if start < 0 {
		t.Fatal("AGENTS.md has no ASD-V0-905 Verify block")
	}
	block := text[start+len(marker):]
	block = block[:strings.Index(block, "```")]
	lines := strings.Split(strings.TrimSpace(block), "\n")
	if len(lines) < 4 || !strings.Contains(block, "go -C interop/cem01-go test") {
		t.Fatalf("Verify block = %q", block)
	}
	for _, line := range lines {
		if err := dogfoodflow.ValidateVerifyLine(line); err != nil {
			t.Errorf("AGENTS.md Verify line %q is refused: %v", line, err)
		}
	}
}
