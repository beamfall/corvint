package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestContextLSPOffKeepsTheGoldenAndOnDegrades pins TCP-V0-043 and
// TCP-V0-045: every value but `gopls` prints the base golden bytes, and
// `gopls` with a server that fails adds only an `external` member whose one
// provider row is unavailable with its reason; every other member is
// unchanged.
func TestContextLSPOffKeepsTheGoldenAndOnDegrades(t *testing.T) {
	root := evidenceSummaryRepository(t)
	arguments := []string{"--root", root, "context", "--task", evidenceSummaryTask, "--subject", "cache/demux.go"}
	golden, err := os.ReadFile("testdata/context-default-wire.golden")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "off", "on", "GOPLS", "1"} {
		t.Setenv("CORVINT_CONTEXT_LSP", value)
		if _, stdout, stderr := runContextCommand(t, arguments...); !bytes.Equal(stdout, golden) {
			t.Fatalf("CORVINT_CONTEXT_LSP=%q changed the wire: %s %s", value, stdout, stderr)
		}
	}
	fake := t.TempDir()
	if err := os.WriteFile(filepath.Join(fake, "gopls"), []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CORVINT_CONTEXT_LSP", "gopls")
	code, stdout, stderr := runContextCommand(t, arguments...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	packet := decodeObject(t, stdout)
	external, _ := packet["external"].(map[string]any)
	providers, _ := external["providers"].([]any)
	if len(providers) != 1 {
		t.Fatalf("one provider row: %v", external)
	}
	row := providers[0].(map[string]any)
	if row["state"] != "unavailable" || row["reason"] != "gopls session failed" || row["source"] != lspSource {
		t.Fatalf("a failing server is a visible unavailable row: %v", row)
	}
	if external["query"].(map[string]any)["sha256"] == "" {
		t.Fatal("the query digest is reported")
	}
	delete(packet, "external")
	if !reflect.DeepEqual(packet, decodeObject(t, golden)) {
		t.Fatal("the external member must leave every other member unchanged")
	}
}
