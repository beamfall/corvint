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
	"github.com/Beamfall/corvint/internal/cliconformance"
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

func TestCLIErrorContracts_ACXV0859(t *testing.T) {
	dir := binaries(t)
	for _, ref := range capcatalog.TreeReferences() {
		t.Run(ref.Binary, func(t *testing.T) {
			binary := filepath.Join(dir, ref.Binary)
			version := exec.Command(binary, "--version")
			versionOut, err := version.Output()
			if err != nil || !strings.Contains(string(versionOut), versionline.Release) || strings.Contains(string(versionOut), "tcp01-unverified") {
				t.Fatalf("version scheme: %v %s", err, versionOut)
			}
			tree, _ := capcatalog.TreeSnapshot(ref.Binary)
			if tree.AllFlags()["--root"] {
				cliconformance.Failure(t, binary, []string{"--root"}, 2)
			}
			for _, args := range [][]string{{"zz-no-such-command"}, {"--zz-no-such-option"}, {"help", "zz-no-such-topic"}} {
				e := cliconformance.Failure(t, binary, args, 2)
				if e.Binary != ref.Binary || e.Help == "" {
					t.Fatalf("missing binary/help attribution: %+v", e)
				}
			}
		})
	}
	for _, tc := range []struct{ binary, typo, want string }{{"corvint", "contxt", "context"}, {"corvint-tasks", "tikcet", "ticket"}} {
		e := cliconformance.Failure(t, filepath.Join(dir, tc.binary), []string{tc.typo}, 2)
		if !strings.Contains(e.Error, "did you mean") || !strings.Contains(e.Error, tc.want) {
			t.Fatalf("typo has no remedy: %+v", e)
		}
	}
	// Plain flag-parser and command errors use the same envelope too.
	cliconformance.Failure(t, filepath.Join(dir, "corvint-companion-release"), nil, 2)
	cliconformance.Failure(t, filepath.Join(dir, "corvint-public-release-check"), nil, 2)
	cliconformance.Failure(t, filepath.Join(dir, "corvint-postmerge-workflow"), []string{"encode-id"}, 2)
	// CAP-V0-7700: an internal recorder must reach its own closed argument
	// parser, rather than being blocked as an unknown public command.
	recorder := cliconformance.Failure(t, filepath.Join(dir, "corvint"), []string{"dogfood-record", "--zz-no-such-option"}, 2)
	if recorder.Code == "unknown-verb" {
		t.Fatal("private dogfood recorder was blocked by public discovery")
	}
	// Exercise the native serializers too, rather than only common preflight.
	for _, args := range [][]string{{"test-validity", "--receipt", "/no/such/clicontract-receipt.json"}, {"ocm", "status", "--map", "/no/such/clicontract-map.json"}, {"prove", "--cem", "/no/such/clicontract-map.json"}} {
		cliconformance.Failure(t, filepath.Join(dir, "corvint"), args, 2)
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
	// An isolated initialized queue is supplied by the native fixture in the focused Tasks tests;
	// the all-binary test also proves codeless serializers are normalized by the shared writer.
}

func TestCAPV07707MessageCodeLint(t *testing.T) {
	if clierror.Validate(clierror.Envelope{Code: "missing-input", Error: "missing-input"}) == nil {
		t.Fatal("message-equals-code lint failed to reject")
	}
}
