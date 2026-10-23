package main

import (
	"bytes"
	"context"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/procprofile"
)

var profileTimingMeta = regexp.MustCompile(`"_meta":\{"corvint/timing":\{[^}]*\}\},`)

// PRB-V0-010 (GitHub #1600 corpus MCP row): a corpus MCP server that serves a session and exits
// at end of input writes both profiles, and the knobs do not change a response byte.
func TestPRBV0010CorpusServerHonoursProfileKnobs(t *testing.T) {
	root, _ := corpusServerFixture(t, "# Evidence\n\nOriginal source.\n")
	session := strings.Join([]string{
		`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"profile","version":"test"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"corvint.docs_search","arguments":{"query":"Original"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"corvint.docs_info","arguments":{}}}`,
	}, "\n") + "\n"
	arguments := []string{"--root", root, "--artifact", "corpus.json", "--protocol-version", "2025-11-25"}
	dir := t.TempDir()
	cpu, mem := filepath.Join(dir, "cpu.pprof"), filepath.Join(dir, "heap.pprof")
	env := map[string]string{"CORVINT_CPUPROFILE": cpu, "CORVINT_MEMPROFILE": mem}
	var profiled, plain, stderr bytes.Buffer
	if code := runProfiled(context.Background(), arguments, strings.NewReader(session), &profiled, &stderr, func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}); code != 0 {
		t.Fatalf("profiled session: exit=%d stderr=%q", code, &stderr)
	}
	for _, path := range []string{cpu, mem} {
		if err := procprofile.Verify(path); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Count(profiled.String(), "\n") != 3 || strings.Contains(profiled.String(), `"isError":true`) {
		t.Fatalf("profiled session: %d responses or an error result", strings.Count(profiled.String(), "\n"))
	}
	// Responses carry wall-clock timing metadata and may complete in either order; every other
	// byte of every response must match.
	untimed := func(b []byte) []byte {
		lines := strings.SplitAfter(string(profileTimingMeta.ReplaceAll(b, nil)), "\n")
		slices.Sort(lines)
		return []byte(strings.Join(lines, ""))
	}
	if code := run(context.Background(), arguments, strings.NewReader(session), &plain, &stderr); code != 0 || !bytes.Equal(untimed(profiled.Bytes()), untimed(plain.Bytes())) {
		t.Fatalf("knobs changed output (exit %d, %d vs %d bytes)", code, profiled.Len(), plain.Len())
	}
}
