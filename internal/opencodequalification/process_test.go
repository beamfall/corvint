package opencodequalification

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestCommandInterruption(t *testing.T) {
	t.Run("GOC-V0-008 AHI-032 Go command reaps escaped group descendant", func(t *testing.T) {
		if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
			t.Skip("explicitly unsupported process platform")
		}
		dir := t.TempDir()
		binary := filepath.Join(dir, "qualify")
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()
		root, e := filepath.Abs("../..")
		if e != nil {
			t.Fatal(e)
		}
		if _, e = capture(ctx, root, cleanEnvironment(), []string{"go", "build", "-o", binary, "./tools/qualify-opencode"}); e != nil {
			t.Fatal(e)
		}
		config := filepath.Join(dir, "probe.json")
		if e = writeJSON(config, Object{"binary": binary, "root": dir, "interrupt": true}); e != nil {
			t.Fatal(e)
		}
		result, e := interruptedCommand(ctx, dir, []string{binary, "--probe-config", config}, filepath.Join(dir, "interruption"), func() bool {
			x, e := readObject(filepath.Join(dir, "active.json"))
			return e == nil && number(x["child"]) > 0
		})
		if e != nil {
			t.Fatal(e)
		}
		if number(result["exit"]) != 143 || len(array(result["observedDescendants"])) == 0 || len(array(result["survivors"])) != 0 {
			t.Fatal(result)
		}
	})
}
func TestFixedCommandResolution(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	out, e := capture(ctx, t.TempDir(), os.Environ(), []string{"git", "--version"})
	if e != nil || out == "" {
		t.Fatalf("%s %v", out, e)
	}
}
