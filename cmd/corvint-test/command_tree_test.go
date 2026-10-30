package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/clitree"
)

// TestCAPV0042EveryNodeAnswersHelp runs the built binary for every command-tree node with --help
// and -h: each exits 0 and prints that node's own usage, flags, required inputs and example, or for
// the e2e group and its steps the CCE-V0-118 step help with the node's required inputs.
func TestCAPV0042EveryNodeAnswersHelp(t *testing.T) {
	bin := buildCompanion(t, ".", "corvint-test")
	tree := commandTree()
	if problems := clitree.Problems(tree); len(problems) > 0 {
		t.Fatalf("tree problems: %v", problems)
	}
	if len(tree.Nodes) < 20 {
		t.Fatalf("tree has %d nodes", len(tree.Nodes))
	}
	for _, node := range tree.Nodes {
		want := clitree.Render(tree, node)
		for _, flag := range []string{"--help", "-h"} {
			t.Run(node.Command+" "+flag, func(t *testing.T) {
				stdout, stderr, code := runBinary(t, bin, append(slices.Clone(node.Path), flag)...)
				if code != 0 || stderr != "" {
					t.Fatalf("exit %d, stderr %q", code, stderr)
				}
				if node.Path[0] == "e2e" {
					// The e2e group and its steps print the CCE-V0-118 step help, which
					// TestCCEV0118SubcommandHelp pins; here each must name its own command.
					parts := []string{"corvint-test " + strings.Join(node.Path, " "), "corvint-test help --json",
						"\nRequired: ", "\nExample:\n  " + node.Example}
					if len(node.Path) > 1 {
						parts = []string{"corvint-test " + strings.Join(node.Path, " "), "Usage:", "Example:", "\nRequired: "}
					}
					for _, required := range node.Required {
						parts = append(parts, required)
					}
					for _, part := range parts {
						if !strings.Contains(stdout, part) {
							t.Fatalf("e2e help lacks %q:\n%s", part, stdout)
						}
					}
					return
				}
				if stdout != want {
					t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, want)
				}
				for _, part := range []string{"Usage:", "Flags:", "Required:", "Example:", node.Example} {
					if !strings.Contains(stdout, part) {
						t.Fatalf("help lacks %q:\n%s", part, stdout)
					}
				}
			})
		}
	}
	if stdout, _, code := runBinary(t, bin, "help", "e2e", "prove"); code != 0 || !strings.Contains(stdout, "corvint-test e2e prove --base REV") {
		t.Fatalf("help e2e prove: exit %d, %q", code, stdout)
	}
	if _, stderr, code := runBinary(t, bin, "help", "e2e", "nope"); code != 2 || !strings.Contains(stderr, "unknown command") {
		t.Fatalf("help e2e nope: exit %d, %q", code, stderr)
	}
	if stdout, _, code := runBinary(t, bin, "--help"); code != 0 || !strings.Contains(stdout, "corvint-test help --json") {
		t.Fatalf("--help: exit %d, %q", code, stdout)
	}
}

// TestCAPV0045TreeSnapshotIsCurrent keeps the committed snapshot the capabilities check reads equal
// to `corvint-test help --json` (regenerate with `make capability-catalog`).
func TestCAPV0045TreeSnapshotIsCurrent(t *testing.T) {
	var got bytes.Buffer
	if handled, err := helpRequest([]string{"help", "--json"}, &got); !handled || err != nil {
		t.Fatalf("help --json: handled=%t err=%v", handled, err)
	}
	want, err := os.ReadFile("../../internal/capcatalog/trees/corvint-test.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatal("internal/capcatalog/trees/corvint-test.json is stale; run `make capability-catalog`")
	}
}

func runBinary(t *testing.T, bin string, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}
