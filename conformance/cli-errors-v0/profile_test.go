// SPDX-License-Identifier: Apache-2.0
package clierrors_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/clierror"
	"github.com/Beamfall/corvint/internal/versionline"
)

// CAP-V0-9300, ACX-V0-9301, CAP-V0-9302: every binary and every node
// exercises the additive process boundary, alongside the native controls.
func TestCLIProfile9300EveryNode(t *testing.T) {
	dir := binaries(t)
	adapter := filepath.Join(dir, "corvint-cli")
	for _, ref := range capcatalog.TreeReferences() {
		if ref.Binary == "corvint-cli" {
			continue
		} // Its own CLI is checked by ShippedHelp.
		t.Run(ref.Binary, func(t *testing.T) {
			tree, ok := capcatalog.TreeSnapshot(ref.Binary)
			if !ok {
				t.Fatal("missing tree")
			}
			for _, node := range tree.Nodes {
				for _, flag := range []string{"--help", "-h"} {
					args := append([]string{"--binary", ref.Binary}, node.Path...)
					code, out, errout := launch(t, adapter, t.TempDir(), append(args, flag)...)
					if code != 0 || errout != "" || !strings.Contains(strings.ToLower(out), "usage") || !strings.Contains(strings.ToLower(out), "required") || !strings.Contains(strings.ToLower(out), "example") {
						t.Fatalf("%v: exit=%d stdout=%q stderr=%q", args, code, out, errout)
					}
				}
			}
			code, out, errout := launch(t, adapter, t.TempDir(), "--binary", ref.Binary, "help", "--json")
			want, _ := json.Marshal(tree)
			var got any
			if code != 0 || errout != "" || json.Unmarshal([]byte(out), &got) != nil {
				t.Fatalf("catalog: %d %q %q", code, out, errout)
			}
			var expected any
			_ = json.Unmarshal(want, &expected)
			canonical, _ := json.Marshal(got)
			want, _ = json.Marshal(expected)
			if string(canonical) != string(want) {
				t.Fatal("profile catalog differs from frozen native tree")
			}
			code, out, errout = launch(t, adapter, t.TempDir(), "--binary", ref.Binary, unknownVerb)
			var refusal clierror.Envelope
			if code != 2 || out != "" || json.Unmarshal([]byte(errout), &refusal) != nil || clierror.Validate(refusal) != nil || refusal.Code != "unknown-verb" || !strings.Contains(refusal.Error, unknownVerb) {
				t.Fatalf("refusal: %d %q %q", code, out, errout)
			}
			code, out, errout = launch(t, adapter, t.TempDir(), "--binary", ref.Binary, "--version", "--json")
			var version struct {
				Version string `json:"version"`
			}
			if code != 0 || errout != "" || json.Unmarshal([]byte(out), &version) != nil || version.Version != versionline.Development {
				t.Fatalf("release version: %d %q %q", code, out, errout)
			}
		})
	}
	for _, binary := range []string{"corvint", "corvint-tasks"} {
		word := "contxt"
		if binary == "corvint-tasks" {
			word = "tikcet"
		}
		code, out, errout := launch(t, adapter, t.TempDir(), "--binary", binary, word)
		if code != 2 || out != "" || !strings.Contains(errout, "did you mean") {
			t.Fatalf("suggestion: %d %q %q", code, out, errout)
		}
	}
}

// ACX-V0-9301: the twelve restored analyzers keep exact native help refusals,
// including Swift's nonzero exit and the two specialized profile identifiers.
func TestCLIProfile9301NativeAnalyzers(t *testing.T) {
	dir := binaries(t)
	for _, ref := range capcatalog.TreeReferences() {
		if !strings.HasPrefix(ref.Binary, "corvint-analyzer-") {
			continue
		}
		profile, status := "corvint-analyzer-candidate/experimental", 0
		if p, ok := nativeAnalyzers[ref.Binary]; ok {
			profile = p
		}
		if ref.Binary == "corvint-analyzer-structured-data" {
			profile = "corvint-structured-data/experimental-v1"
		}
		if ref.Binary == "corvint-analyzer-sql-native" {
			profile = "corvint-analyzer-candidate/sqlite-3.51.0-source-v1"
		}
		if ref.Binary == "corvint-analyzer-swift" {
			status = 1
		}
		want := `{"profile":"` + profile + `","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n"
		for _, arg := range []string{"--help", "-h", unknownVerb} {
			code, out, errout := launch(t, filepath.Join(dir, ref.Binary), t.TempDir(), arg)
			if code != status || out != want || errout != "" {
				t.Fatalf("%s %s: %d %q %q", ref.Binary, arg, code, out, errout)
			}
		}
	}
}
