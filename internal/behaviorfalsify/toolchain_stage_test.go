package behaviorfalsify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
)

// TestBBFV0214ToolchainRunsVerifiedBytes swaps the pinned Go driver after its digest check: the
// run uses the verified bytes with GOROOT pinned, a swap before staging is refused, and a GOROOT
// without pkg/tool or a declared GOROOT key is refused (BBF-V0-214).
func TestBBFV0214ToolchainRunsVerifiedBytes(t *testing.T) {
	goroot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(goroot, "pkg", "tool"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(goroot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	driver := filepath.Join(goroot, "bin", "go")
	write := func(script string) {
		t.Helper()
		if err := execfile.WriteFile(driver+".new", []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(driver+".new", driver); err != nil {
			t.Fatal(err)
		}
	}
	write("#!/bin/sh\necho \"verified $GOROOT\"\n")
	toolchain := Command{Path: driver, ExecutableSHA256: mustFileDigest(t, driver)}
	plan := Plan{Request: Request{Provider: &ProviderSpec{Name: ProviderGoTest, Toolchain: toolchain, Package: ".", Examples: []string{"TestX"}}}}
	run := func() ([]byte, ProcessEvidence) {
		return runProvider(context.Background(), plan, nil, t.TempDir(), t.TempDir(), 30*time.Second)
	}
	t.Cleanup(func() { afterToolchainStaged = func(Command) {} })

	afterToolchainStaged = func(Command) { write("#!/bin/sh\necho swapped\n") }
	out, evidence := run()
	// GOROOT is the private copy of the pinned root (BBF-V0-820), never the original.
	if got := strings.TrimSpace(string(out)); !strings.HasPrefix(got, "verified /") || !strings.HasSuffix(got, string(filepath.Separator)+"goroot") || got == "verified "+goroot || evidence.Error != "" {
		t.Fatalf("swap after the check ran %q (%+v), want the verified bytes over the private GOROOT", got, evidence)
	}

	afterToolchainStaged = func(Command) {}
	if out, evidence := run(); evidence.Error != "command executable drift" || len(out) != 0 {
		t.Fatalf("swapped driver = %q %+v, want drift refusal", out, evidence)
	}

	write("#!/bin/sh\necho \"verified $GOROOT\"\n")
	if err := os.Remove(filepath.Join(goroot, "pkg", "tool")); err != nil {
		t.Fatal(err)
	}
	if _, evidence := run(); evidence.Error != "provider toolchain GOROOT unavailable" {
		t.Fatalf("GOROOT without pkg/tool = %+v", evidence)
	}

	request := providerRequest(t, []ControlSpec{providerControl(t, "kill", "CHECKOUT_TOTAL_OFFSET")})
	request.Provider.PerturbationEnvKeys = append(request.Provider.PerturbationEnvKeys, "GOROOT")
	if _, err := BuildPlan(request); err == nil || !strings.Contains(err.Error(), "provider boundary") {
		t.Fatalf("declared GOROOT = %v, want boundary refusal", err)
	}
}

// TestBBFV0820ToolsRunFromVerifiedCopies: a tool under the pinned GOROOT, or a standard
// library file, swapped after the private GOROOT is staged does not run, and one swapped between
// the driver's check and staging is refused; an RSpec toolchain swapped after its digest check does
// not run either (V1-2215). Before BBF-V0-820 the swapped GOROOT tool and RSpec toolchain ran.
func TestBBFV0820ToolsRunFromVerifiedCopies(t *testing.T) {
	t.Cleanup(func() {
		afterToolchainStaged = func(Command) {}
		afterToolchainChecked = func(Command) {}
	})
	replace := func(path, script string) {
		t.Helper()
		if err := execfile.WriteFile(path+".new", []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(path+".new", path); err != nil {
			t.Fatal(err)
		}
	}
	fixture := func(t *testing.T) (Plan, string, string) {
		goroot := t.TempDir()
		for _, dir := range []string{"bin", filepath.Join("pkg", "tool"), filepath.Join("src", "fmt")} {
			if err := os.MkdirAll(filepath.Join(goroot, dir), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		driver, tool := filepath.Join(goroot, "bin", "go"), filepath.Join(goroot, "pkg", "tool", "compile")
		replace(driver, "#!/bin/sh\nexec \"$GOROOT/pkg/tool/compile\"\n")
		replace(tool, "#!/bin/sh\necho verified-tool $(cat \"$GOROOT/src/fmt/print.go\")\n")
		replace(filepath.Join(goroot, "src", "fmt", "print.go"), "verified-library")
		// A top-level member file is copied before any member directory creates the private root.
		replace(filepath.Join(goroot, "VERSION"), "go0.0.0")
		plan := Plan{Request: Request{Provider: &ProviderSpec{Name: ProviderGoTest, Toolchain: Command{Path: driver, ExecutableSHA256: mustFileDigest(t, driver)}, Package: ".", Examples: []string{"TestX"}}}}
		return plan, tool, filepath.Join(goroot, "src", "fmt", "print.go")
	}

	t.Run("GOROOT tool and library swapped after staging do not run", func(t *testing.T) {
		plan, tool, library := fixture(t)
		afterToolchainStaged = func(Command) {
			replace(tool, "#!/bin/sh\necho swapped-tool\n")
			replace(library, "swapped-library")
		}
		out, evidence := runProvider(context.Background(), plan, nil, t.TempDir(), t.TempDir(), 30*time.Second)
		if got := strings.TrimSpace(string(out)); got != "verified-tool verified-library" || evidence.Error != "" {
			t.Fatalf("ran %q (%+v), want the verified tool over the verified library", got, evidence)
		}
	})

	t.Run("GOROOT swapped between the check and staging is refused", func(t *testing.T) {
		for name, swap := range map[string]func(tool, library string){
			"tool":    func(tool, _ string) { replace(tool, "#!/bin/sh\necho swapped-tool\n") },
			"library": func(_, library string) { replace(library, "swapped-library") },
		} {
			plan, tool, library := fixture(t)
			afterToolchainChecked = func(Command) { swap(tool, library) }
			out, evidence := runProvider(context.Background(), plan, nil, t.TempDir(), t.TempDir(), 30*time.Second)
			if evidence.Error != errGOROOTChanged.Error() || evidence.Started || len(out) != 0 {
				t.Fatalf("%s swap: ran %q (%+v), want %q before anything starts", name, out, evidence, errGOROOTChanged)
			}
		}
	})

	t.Run("GOROOT symbolic link is refused", func(t *testing.T) {
		plan, tool, _ := fixture(t)
		if err := os.Symlink(tool, filepath.Join(filepath.Dir(tool), "link")); err != nil {
			t.Fatal(err)
		}
		if _, evidence := runProvider(context.Background(), plan, nil, t.TempDir(), t.TempDir(), 30*time.Second); evidence.Error != errGOROOTUnsupported.Error() {
			t.Fatalf("symbolic link under pkg/tool: %+v", evidence)
		}
	})

	t.Run("RSpec toolchain swapped after its digest check does not run", func(t *testing.T) {
		checkout := t.TempDir()
		if err := os.Mkdir(filepath.Join(checkout, "example"), 0o700); err != nil {
			t.Fatal(err)
		}
		toolchain := writeToolchain(t, t.TempDir(), "#!/bin/sh\nprintf verified > "+rspecReportName+"\n")
		plan := Plan{Request: Request{Provider: &ProviderSpec{Name: ProviderRSpec, Toolchain: toolchain, Package: "example", Examples: []string{rspecTotalExample}}}}
		afterToolchainChecked = func(Command) { replace(toolchain.Path, "#!/bin/sh\nprintf swapped > "+rspecReportName+"\n") }
		out, evidence := runProvider(context.Background(), plan, nil, t.TempDir(), checkout, 30*time.Second)
		if evidence.Error != "command changed before staging" || len(out) != 0 {
			t.Fatalf("RSpec toolchain swapped after its check: ran %q (%+v), want a staging refusal", out, evidence)
		}
	})
}

// TestBBFV0820StagingHonorsTheCommandBudget: the GOROOT digest and copy run inside the control's
// command budget. A run cancelled during staging refuses before anything starts, and the driver
// gets only what staging left of the budget, not the whole budget again (Codex review of
// linux-w2).
func TestBBFV0820StagingHonorsTheCommandBudget(t *testing.T) {
	t.Cleanup(func() { afterToolchainChecked = func(Command) {} })
	fixture := func(t *testing.T, driverScript string) Plan {
		goroot := t.TempDir()
		for _, dir := range []string{"bin", filepath.Join("pkg", "tool")} {
			if err := os.MkdirAll(filepath.Join(goroot, dir), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		driver := filepath.Join(goroot, "bin", "go")
		if err := execfile.WriteFile(driver, []byte(driverScript), 0o700); err != nil {
			t.Fatal(err)
		}
		return Plan{Request: Request{Provider: &ProviderSpec{Name: ProviderGoTest, Toolchain: Command{Path: driver, ExecutableSHA256: mustFileDigest(t, driver)}, Package: ".", Examples: []string{"TestX"}}}}
	}

	t.Run("cancelled during staging", func(t *testing.T) {
		plan := fixture(t, "#!/bin/sh\necho ran\n")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		afterToolchainChecked = func(Command) { cancel() }
		out, evidence := runProvider(ctx, plan, nil, t.TempDir(), t.TempDir(), 30*time.Second)
		if evidence.Error != errBudgetExhausted.Error() || evidence.Started || len(out) != 0 {
			t.Fatalf("ran %q (%+v), want %q before anything starts", out, evidence, errBudgetExhausted)
		}
	})

	t.Run("the driver gets only the remaining budget", func(t *testing.T) {
		// Staging spends 1.5 s of a 3 s budget; the 2 s driver fits the whole budget but not
		// what remains, so it must time out.
		plan := fixture(t, "#!/bin/sh\nexec /bin/sleep 2\n")
		afterToolchainChecked = func(Command) { time.Sleep(1500 * time.Millisecond) }
		start := time.Now()
		_, evidence := runProvider(context.Background(), plan, nil, t.TempDir(), t.TempDir(), 3*time.Second)
		if !evidence.TimedOut {
			t.Fatalf("driver given the whole budget after staging (%+v, %v elapsed), want a timeout inside the budget", evidence, time.Since(start))
		}
	})
}
