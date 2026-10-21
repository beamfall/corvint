package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestREADMEKnownDivergenceCountsMatchManifest pins the README's spelled-out
// knownDivergence case and class counts, and its per-class case lists, to the
// manifest declarations (V1-1224).
func TestREADMEKnownDivergenceCountsMatchManifest(t *testing.T) {
	dir := filepath.Join(moduleRootForTest(t), "conformance", "cli-parity-v0")
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := loadManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(readme)
	words := []string{"", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten",
		"Eleven", "Twelve", "Thirteen", "Fourteen", "Fifteen", "Sixteen", "Seventeen", "Eighteen",
		"Nineteen", "Twenty"}
	classes := map[string]int{}
	cases := 0
	for _, item := range manifest.Cases {
		if item.KnownDivergence == nil {
			continue
		}
		cases++
		classes[item.KnownDivergence.Register]++
		if !strings.Contains(text, "`"+item.ID+"`") {
			t.Errorf("README does not name knownDivergence case %s", item.ID)
		}
	}
	if cases >= len(words) || len(classes) >= len(words) {
		t.Fatalf("extend the number words: cases=%d classes=%d", cases, len(classes))
	}
	want := words[cases] + " case declarations are admitted across " + strings.ToLower(words[len(classes)]) + " distinct divergence classes"
	if !strings.Contains(strings.Join(strings.Fields(text), " "), want) {
		t.Errorf("README count sentence is stale; want %q", want)
	}
	for register := range classes {
		if !strings.Contains(text, "- `"+register+"` /") {
			t.Errorf("README has no inventory entry for %s", register)
		}
	}
}
