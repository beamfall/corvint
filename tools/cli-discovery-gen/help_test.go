// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestCAPV07700RootInputs(t *testing.T) {
	t.Chdir("../..")
	for binary, want := range map[string][]string{
		"corvint-public-release-check":    {"--bundle-dir DIR", "--source-root DIR", "--scratch DIR", "--output FILE", "--npm-cache DIR", "--browser-cache DIR", "--node FILE", "--python FILE", "--python-sha256 SHA256", "--expected-commit SHA", "--expected-tree SHA"},
		"corvint-release-candidate":       {"--core-dir DIR", "--source-root DIR", "--scratch DIR", "--output-parent DIR", "--version VERSION"},
		"corvint-release-install":         {"--candidate DIR", "--store DIR"},
		"corvint-readiness-record":        {"--candidate DIR", "--source-root DIR", "--evidence-file FILE", "--output FILE"},
		"corvint-release-gate":            {"--commit SHA", "--manifest FILE", "--git FILE", "--git-sha256 SHA256", "--policy FILE", "--policy-sha256 SHA256"},
		"corvint-go-toolchain-receipt":    {"GOROOT"},
		"corvint-remote-provider":         {"--allow-network", "--config FILE"},
		"corvint-mcp":                     {"--root ABSOLUTE_ROOT"},
		"corvint-corpus-http":             {"--index FILE", "--sha256 SHA256"},
		"corvint-corpus-parity":           {"--index FILE", "--sha256 SHA256", "--request FILE"},
		"corvint-postmerge-host-launcher": {"--profile FILE", "--profile-sha256 SHA256", "--request FILE", "--request-sha256 SHA256", "--out DIR"},
	} {
		t.Run(binary, func(t *testing.T) {
			root := rootTree(binary).Nodes[0]
			for _, input := range want {
				if !slices.Contains(root.Required, input) {
					t.Errorf("missing required input %q: %v", input, root.Required)
				}
				if !strings.Contains(root.Example, strings.Fields(input)[0]) {
					t.Errorf("example omits %q: %s", input, root.Example)
				}
			}
			if binary == "corvint-corpus-parity" && len(root.Usage) != 3 {
				t.Errorf("lost conditional modes: %v", root.Usage)
			}
			if binary == "corvint-readiness-record" && len(root.Usage) != 2 {
				t.Errorf("lost build/verify alternatives: %v", root.Usage)
			}
		})
	}
}

func TestCAPV07700UnauthoredInputsRefused(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("cmd/fixture", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("cmd/fixture/main.go", []byte(`package main
import "flag"
func main() { flag.String("input", "", "required only in build mode") }
`), 0600); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if got := recover(); got == nil || !strings.Contains(got.(string), "authored usage") {
			t.Fatalf("missing usage must refuse inference, got %v", got)
		}
	}()
	rootTree("fixture")
}

// CAP-V0-9300: a native receipt control is an alternative to protocol stdin,
// not grounds to infer an analyzer CLI parser or to rewrite its native source.
func TestCLIProfile9300NativeReceiptUsage(t *testing.T) {
	t.Chdir("../..")
	adapter := rootTree("corvint-cli")
	if !slices.Contains(adapter.Sources, "internal/cliprofile") || !slices.Contains(adapter.Nodes[0].Flags, "--binary") {
		t.Fatalf("adapter parser missing from inventory: %+v", adapter)
	}
	for _, flag := range []string{"--binary", "--version", "--json", "--tree", "--help", "-h"} {
		if !adapter.AllFlags()[flag] {
			t.Errorf("adapter control %s missing", flag)
		}
	}
	for _, binary := range []string{"corvint-analyzer-html-css", "corvint-analyzer-structured-data"} {
		tree := rootTree(binary)
		root := tree.Nodes[0]
		if !slices.Contains(root.Required, "stdin: protocol request JSON") || !slices.Contains(root.Usage, binary+" --version") {
			t.Fatalf("%s: %+v", binary, root)
		}
		if binary == "corvint-analyzer-structured-data" && !slices.Contains(root.Usage, binary+" --descriptor") {
			t.Fatalf("missing descriptor: %+v", root)
		}
	}
}
