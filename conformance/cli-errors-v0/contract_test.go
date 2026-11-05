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
	"github.com/Beamfall/corvint/internal/versionline"
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

// TestCAPV07700ShippedHelp launches every shipped binary and every command node,
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
			for _, node := range tree.Nodes {
				for _, flag := range []string{"--help", "-h"} {
					args := append(append([]string{}, node.Path...), flag)
					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					cmd := exec.CommandContext(ctx, binary, args...)
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
			if ref.Binary != "corvint" {
				cmd := exec.Command(binary, "help", "--json")
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
		})
	}
}

func TestACXV07707ProtocolWire(t *testing.T) {
	dir := binaries(t)
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
