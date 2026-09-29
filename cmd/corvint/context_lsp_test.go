package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
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

// TCP-V0-051: explicit selection is closed and overrides the legacy environment.
func TestContextLSPExplicitSelection(t *testing.T) {
	t.Run("TCP-V0-051 explicit selection", func(t *testing.T) {
		root := evidenceSummaryRepository(t)
		args := []string{"--root", root, "context", "--task", evidenceSummaryTask, "--subject", "cache/demux.go"}
		t.Setenv("CORVINT_CONTEXT_LSP", "off")
		_, baseline, _ := runContextCommand(t, args...)
		t.Setenv("CORVINT_CONTEXT_LSP", "gopls")
		if code, out, err := runContextCommand(t, append(args, "--lsp", "off")...); code != 0 || !bytes.Equal(out, baseline) {
			t.Fatalf("off: %d %s %s", code, out, err)
		}
		for _, extra := range [][]string{{"--lsp", ""}, {"--lsp", "GOPLS"}, {"--lsp", "other"}, {"--lsp", "gopls", "--lsp", "off"}, {"--lsp", "gopls", "--summary"}, {"--expand", "bad", "--lsp", "off"}} {
			if code, _, _ := runContextCommand(t, append(args, extra...)...); code != 2 {
				t.Fatalf("accepted %v", extra)
			}
		}
		fake := filepath.Join(t.TempDir(), "gopls")
		if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 3\n"), 0755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", filepath.Dir(fake)+string(os.PathListSeparator)+os.Getenv("PATH"))
		t.Setenv("CORVINT_CONTEXT_LSP", "off")
		if code, out, err := runContextCommand(t, append(args, "--lsp=gopls")...); code != 0 || decodeObject(t, out)["external"] == nil {
			t.Fatalf("explicit enable: %d %s %s", code, out, err)
		}
	})
}

// TCP-V0-052: a checkout change during a server session withholds the packet.
func TestContextLSPDriftWithholdsPacket(t *testing.T) {
	t.Run("TCP-V0-052 repository drift", func(t *testing.T) {
		git, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		for _, mutation := range []string{"printf '\\n// changed\\n' >> cache/demux.go", "'" + git + "' -c core.hooksPath=/dev/null commit -q --allow-empty -m drift"} {
			t.Run(mutation, func(t *testing.T) {
				root := evidenceSummaryRepository(t)
				fake := filepath.Join(t.TempDir(), "gopls")
				if err := os.WriteFile(fake, []byte("#!/bin/sh\n"+mutation+"\nexit 3\n"), 0755); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", filepath.Dir(fake)+string(os.PathListSeparator)+os.Getenv("PATH"))
				code, out, stderr := runContextCommand(t, "--root", root, "context", "--task", evidenceSummaryTask, "--subject", "cache/demux.go", "--lsp", "gopls")
				if code != 2 || len(out) != 0 || !strings.Contains(string(stderr), "repository changed during LSP") {
					t.Fatalf("drift: %d %s %s", code, out, stderr)
				}
			})
		}
	})
}
