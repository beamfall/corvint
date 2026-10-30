package opencodequalification

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

func TestCommandInterruption(t *testing.T) {
	t.Run("GOC-V0-008 AHI-032 AHI-179 Go command reaps escaped group descendant", func(t *testing.T) {
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
		// AHI-179: the report carries a fixed-size summary bound by digest to the retained snapshots.
		summary := object(result["observation"])
		retained, e := os.ReadFile(filepath.Join(dir, "interruption.observation.json"))
		if e != nil {
			t.Fatal(e)
		}
		full, e := decode(retained)
		if e != nil {
			t.Fatal(e)
		}
		if _, embedded := summary["processes"]; embedded || summary["processesSHA256"] != hash(retained) || summary["processesFile"] != "interruption.observation.json" || number(summary["processCount"]) != float64(len(array(full["processes"]))) || number(summary["processCount"]) == 0 || summary["absent"] != true || summary["scope"] != full["scope"] {
			t.Fatal(summary)
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

func TestObservedCleanupClassification(t *testing.T) {
	gap := procgroup.SnapshotGapFailurePrefix + "12.94s, beyond the 10s observation bound"
	old := readContention
	readContention = func() contentionSnapshot {
		return contentionSnapshot{load: [3]float64{24, 20, 10}, loadOK: true, cpus: 10, source: "test"}
	}
	t.Cleanup(func() { readContention = old })
	t.Run("AHI-340 clean group with only a snapshot gap is retry-later", func(t *testing.T) {
		err := cleanupError(true, &procgroup.DescendantObservation{Failures: []string{gap}, Processes: make([]procgroup.ObservedProcess, 668)})
		if !IsRetryLater(err) || !strings.Contains(err.Error(), "12.94s") || !strings.Contains(err.Error(), "24.00 on 10 logical CPUs") || !strings.Contains(err.Error(), "below 5.0") || strings.Contains(err.Error(), "Processes:") {
			t.Fatal(err)
		}
	})
	t.Run("AHI-340 survivors and unclean groups stay cleanup incomplete", func(t *testing.T) {
		for name, err := range map[string]error{
			"unclean group":  cleanupError(false, &procgroup.DescendantObservation{Failures: []string{gap}}),
			"survivor":       cleanupError(true, &procgroup.DescendantObservation{Failures: []string{"observed descendant remains after cleanup\n" + gap}}),
			"no failure":     cleanupError(true, &procgroup.DescendantObservation{}),
			"no observation": cleanupError(true, nil),
		} {
			if IsRetryLater(err) || !strings.Contains(err.Error(), "cleanup incomplete") {
				t.Fatal(name, err)
			}
		}
		err := cleanupError(true, &procgroup.DescendantObservation{Failures: []string{"observed descendant remains after cleanup"}, Processes: make([]procgroup.ObservedProcess, 50)})
		if !strings.Contains(err.Error(), "remains after cleanup") || len(err.Error()) > 300 {
			t.Fatal(err)
		}
	})
}
