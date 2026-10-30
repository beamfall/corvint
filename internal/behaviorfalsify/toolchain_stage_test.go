package behaviorfalsify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		if err := os.WriteFile(driver+".new", []byte(script), 0o700); err != nil {
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
	if got := strings.TrimSpace(string(out)); got != "verified "+goroot || evidence.Error != "" {
		t.Fatalf("swap after the check ran %q (%+v), want the verified bytes", got, evidence)
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

// TestBBFV0214LimitToolsRunInPlace confirms the BBF-V0-214 limit (V1-2215): only the Go driver is
// staged from verified bytes. A tool under the pinned GOROOT swapped after the driver's check, and
// an RSpec toolchain swapped after its own digest check, both still run. The test records today's
// behavior so that a fix that stages or refuses them must flip it.
func TestBBFV0214LimitToolsRunInPlace(t *testing.T) {
	t.Cleanup(func() {
		afterToolchainStaged = func(Command) {}
		afterToolchainChecked = func(Command) {}
	})
	replace := func(path, script string) {
		t.Helper()
		if err := os.WriteFile(path+".new", []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(path+".new", path); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("GOROOT tool swapped after the driver check runs", func(t *testing.T) {
		goroot := t.TempDir()
		for _, dir := range []string{"bin", filepath.Join("pkg", "tool")} {
			if err := os.MkdirAll(filepath.Join(goroot, dir), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		driver, tool := filepath.Join(goroot, "bin", "go"), filepath.Join(goroot, "pkg", "tool", "compile")
		replace(driver, "#!/bin/sh\nexec \"$GOROOT/pkg/tool/compile\"\n")
		replace(tool, "#!/bin/sh\necho verified-tool\n")
		plan := Plan{Request: Request{Provider: &ProviderSpec{Name: ProviderGoTest, Toolchain: Command{Path: driver, ExecutableSHA256: mustFileDigest(t, driver)}, Package: ".", Examples: []string{"TestX"}}}}
		afterToolchainStaged = func(Command) { replace(tool, "#!/bin/sh\necho swapped-tool\n") }
		out, evidence := runProvider(context.Background(), plan, nil, t.TempDir(), t.TempDir(), 30*time.Second)
		if got := strings.TrimSpace(string(out)); got != "swapped-tool" || evidence.Error != "" {
			t.Fatalf("GOROOT tool swap ran %q (%+v); the limit no longer holds, update BBF-V0-214 and this test", got, evidence)
		}
	})

	t.Run("RSpec toolchain swapped after its digest check runs", func(t *testing.T) {
		checkout := t.TempDir()
		if err := os.Mkdir(filepath.Join(checkout, "example"), 0o700); err != nil {
			t.Fatal(err)
		}
		toolchain := writeToolchain(t, t.TempDir(), "#!/bin/sh\nprintf verified > "+rspecReportName+"\n")
		plan := Plan{Request: Request{Provider: &ProviderSpec{Name: ProviderRSpec, Toolchain: toolchain, Package: "example", Examples: []string{rspecTotalExample}}}}
		afterToolchainChecked = func(Command) { replace(toolchain.Path, "#!/bin/sh\nprintf swapped > "+rspecReportName+"\n") }
		out, evidence := runProvider(context.Background(), plan, nil, t.TempDir(), checkout, 30*time.Second)
		if string(out) != "swapped" || evidence.Error != "" {
			t.Fatalf("RSpec toolchain swap ran %q (%+v); the limit no longer holds, update BBF-V0-214 and this test", out, evidence)
		}
	})
}
