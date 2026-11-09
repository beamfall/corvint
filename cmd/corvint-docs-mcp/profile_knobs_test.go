package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/procprofile"
)

// PRB-V0-010: the corvint-docs-mcp entry point honours CORVINT_CPUPROFILE and CORVINT_MEMPROFILE
// without changing its output.
func TestPRBV0010ServerHonoursProfileKnobs(t *testing.T) {
	dir := t.TempDir()
	cpu, mem := filepath.Join(dir, "cpu.pprof"), filepath.Join(dir, "heap.pprof")
	env := map[string]string{"CORVINT_CPUPROFILE": cpu, "CORVINT_MEMPROFILE": mem}
	var profiled, plain, stderr bytes.Buffer
	if code := runProfiled(context.Background(), []string{"--version"}, strings.NewReader(""), &profiled, &stderr, func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}); code != 0 || stderr.Len() != 0 {
		t.Fatalf("profiled --version: exit=%d stderr=%q", code, &stderr)
	}
	for _, path := range []string{cpu, mem} {
		if err := procprofile.Verify(path); err != nil {
			t.Fatal(err)
		}
	}
	if code := run(context.Background(), []string{"--version"}, strings.NewReader(""), &plain, &stderr); code != 0 || !bytes.Equal(profiled.Bytes(), plain.Bytes()) {
		t.Fatalf("knobs changed output: %q vs %q (exit %d)", &profiled, &plain, code)
	}
}
