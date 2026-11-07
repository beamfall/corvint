// SPDX-License-Identifier: Apache-2.0
package clierrors_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/clierror"
)

func binaries(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv("CORVINT_CLI_BIN_DIR"); dir != "" {
		return dir
	}
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", dir+string(os.PathSeparator), "./cmd/...")
	cmd.Dir = "../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build shipped binaries: %v %s", err, out)
	}
	return dir
}

// CAP-V0-7700 retains native help on these compatibility entrypoints. Their
// process contracts are checked separately, rather than requiring generated help.
var nativeAnalyzers = map[string]string{
	"corvint-analyzer-c-jni":       "corvint-analyzer-native-bridge/v0",
	"corvint-analyzer-java":        "corvint-analyzer-native-bridge/v0",
	"corvint-analyzer-objective-c": "corvint-analyzer-native-bridge/v0",
	"corvint-analyzer-rust":        "corvint-analyzer-candidate/experimental",
}

var nativeShims = map[string]string{
	"corvint-test-runner":         "runner",
	"corvint-behavior-falsify":    "falsify",
	"corvint-playwright-minimize": "minimize",
	"corvint-tests-accept":        "accept",
}

// TestCAPV07700ShippedHelp launches every retained generated-help command node
// with both help spellings. stdin is absent and cwd has no repository.
func TestCAPV07700ShippedHelp(t *testing.T) {
	dir := binaries(t)
	for _, ref := range capcatalog.TreeReferences() {
		t.Run(ref.Binary, func(t *testing.T) {
			tree, ok := capcatalog.TreeSnapshot(ref.Binary)
			if !ok {
				t.Fatal("missing snapshot")
			}
			binary := filepath.Join(dir, ref.Binary)
			// CAP-V0-9300: frozen analyzer entrypoints use the explicitly
			// selected discovery profile. All help assertions remain identical.
			profile := strings.HasPrefix(ref.Binary, "corvint-analyzer-")
			_, nativeAnalyzer := nativeAnalyzers[ref.Binary]
			_, nativeShim := nativeShims[ref.Binary]
			if !nativeAnalyzer && !nativeShim {
				for _, node := range tree.Nodes {
					for _, flag := range []string{"--help", "-h"} {
						args := append(append([]string{}, node.Path...), flag)
						if profile {
							args = append([]string{"--binary", ref.Binary}, args...)
						}
						executable := binary
						if profile {
							executable = filepath.Join(dir, "corvint-cli")
						}
						ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
						cmd := exec.CommandContext(ctx, executable, args...)
						cmd.Dir = t.TempDir()
						var out, errout bytes.Buffer
						cmd.Stdout = &out
						cmd.Stderr = &errout
						err := cmd.Run()
						cancel()
						if err != nil || errout.Len() != 0 || out.Len() == 0 {
							t.Fatalf("%v: %v stderr=%q stdout=%q", args, err, errout.String(), out.String())
						}
						if ref.Binary == "corvint-tasks" {
							var native map[string]any
							if json.Unmarshal(out.Bytes(), &native) != nil || native["profile"] != "taskman-command-result/0" {
								t.Fatalf("%v corrupted native help JSON: %s", args, out.String())
							}
						}
						text := strings.ToLower(out.String())
						if !strings.Contains(text, "usage") || !strings.Contains(text, "example") || (!strings.Contains(text, "required") && !strings.Contains(text, "inputs")) {
							t.Fatalf("%v lacks usage, example or required inputs: %s", args, out.String())
						}
					}
				}
			}
			if ref.Binary != "corvint" && !nativeAnalyzer {
				flags := []string{"--json"}
				if nativeShim {
					flags = append(flags, "--tree")
				}
				for _, flag := range flags {
					cmd := exec.Command(binary, "help", flag)
					if profile {
						cmd = exec.Command(filepath.Join(dir, "corvint-cli"), "--binary", ref.Binary, "help", flag)
					}
					out, err := cmd.Output()
					if err != nil {
						t.Fatal(err)
					}
					want, _ := json.Marshal(tree)
					var got any
					if json.Unmarshal(out, &got) != nil {
						t.Fatal("non-json tree")
					}
					canonical, _ := json.Marshal(got)
					if !bytes.Equal(want, canonical) { // Compare semantically: field order is an encoding detail here.
						var wanted any
						_ = json.Unmarshal(want, &wanted)
						want, _ = json.Marshal(wanted)
						if !bytes.Equal(want, canonical) {
							t.Fatal("binary catalog differs from snapshot; make capability-catalog")
						}
					}
				}
			}
		})
	}
}

func TestACXV07707ProtocolWire(t *testing.T) {
	dir := binaries(t)
	// ACP-011 / ACX-V0-7707: unexpected argv must reach native dispatch,
	// including malformed help and arguments following a valid version flag.
	analyzers := map[string]string{"corvint-analyzer-go": "corvint-analyzer-candidate/experimental"}
	for name, profile := range nativeAnalyzers {
		analyzers[name] = profile
	}
	for name, profile := range analyzers {
		for _, args := range [][]string{{"bogus"}, {"bogus", "--help"}, {"help", "bogus"}, {"--version", "extra"}} {
			code, out, errout := launch(t, filepath.Join(dir, name), t.TempDir(), args...)
			want := `{"profile":"` + profile + `","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n"
			if code != 0 || errout != "" || out != want {
				t.Fatalf("%s %v: exit=%d stdout=%q stderr=%q; want native frame %q", name, args, code, out, errout, want)
			}
		}
	}
	// The excluded analyzers refuse even help/catalog requests natively.
	for name, profile := range nativeAnalyzers {
		for _, args := range [][]string{{"--help"}, {"-h"}, {"help", "--json"}, {"help", "--tree"}} {
			code, out, errout := launch(t, filepath.Join(dir, name), t.TempDir(), args...)
			want := `{"profile":"` + profile + `","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n"
			if code != 0 || errout != "" || out != want {
				t.Fatalf("%s %v: exit=%d stdout=%q stderr=%q; want native frame %q", name, args, code, out, errout, want)
			}
		}
	}
	// ACX-V0-7707: an RPC error with an explicit --root is protocol traffic,
	// never a CLI failure. The old argv-length escape corrupted this response.
	root := t.TempDir()
	init := exec.Command("git", "init", "-q", root)
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("RPC fixture: %v %s", err, out)
	}
	rpc := exec.Command(filepath.Join(dir, "corvint-docs-mcp"), "--root", root)
	rpc.Stdin = strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"zz-no-such-method"}` + "\n")
	var rpcOut, rpcErr bytes.Buffer
	rpc.Stdout = &rpcOut
	rpc.Stderr = &rpcErr
	if err := rpc.Run(); err != nil || rpcErr.Len() != 0 {
		t.Fatalf("RPC process: %v %s %s", err, rpcOut.Bytes(), rpcErr.Bytes())
	}
	var response map[string]any
	if json.Unmarshal(bytes.TrimSpace(rpcOut.Bytes()), &response) != nil || response["jsonrpc"] != "2.0" || response["error"] == nil || response["binary"] != nil {
		t.Fatalf("RPC error was rewritten: %s", rpcOut.Bytes())
	}
}

func TestCAPV07707MessageCodeLint(t *testing.T) {
	if clierror.Validate(clierror.Envelope{Code: "missing-input", Error: "missing-input"}) == nil {
		t.Fatal("message-equals-code lint failed to reject")
	}
}

// TestCAPV07700NativeShimHelp pins the excluded shims' native help bytes,
// stream ownership and status. These are compatibility controls, not generated help.
func TestCAPV07700NativeShimHelp(t *testing.T) {
	dir := binaries(t)
	rows := map[string]struct {
		exit        int
		out, errout string
	}{
		"corvint-test-runner": {0, `usage: corvint-test-runner <command> [flags] (experimental runner companion)

commands:
  runners    list runner profile IDs as JSON (no executable pin)
  profiles   alias of runners (no executable pin)
  plan       derive a plan; requires --executable and --executable-sha256
  run        execute an approved plan; requires --executable and --executable-sha256

Run "corvint-test-runner <command> --help" for its flags.
`, ""},
		"corvint-behavior-falsify":    {1, "", "explicit --experimental required; positional arguments forbidden\n"},
		"corvint-playwright-minimize": {1, "", "explicit --experimental required; positional arguments forbidden\n"},
		"corvint-tests-accept":        {2, "", "usage: corvint-tests-accept plan|accept --plan FILE [--approve-plan DIGEST --new PATH... --repeat N --environment LABEL]\n"},
	}
	for name, row := range rows {
		for _, flag := range []string{"--help", "-h"} {
			code, out, errout := launch(t, filepath.Join(dir, name), t.TempDir(), flag)
			if code != row.exit || out != row.out || errout != row.errout {
				t.Fatalf("%s %s: exit=%d stdout=%q stderr=%q; want exit=%d stdout=%q stderr=%q", name, flag, code, out, errout, row.exit, row.out, row.errout)
			}
		}
	}
}
