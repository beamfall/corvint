// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gateselect"
)

func TestAffectedRuntimeEdgeWidening_CINV1940(t *testing.T) {
	root := affectedFixtureRepository(t)
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0700); err != nil {
		t.Fatal(err)
	}
	declaration := `{"profile":"corvint-runtime-edges/0","edges":[{"tests":"leaf/leaf_test.go","runs":"python:new.analyzer.unit","reason":"newly analyzable runtime tool"}]}`
	if err := os.WriteFile(filepath.Join(root, ".corvint/runtime-edges.json"), []byte(declaration), 0600); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", ".corvint/runtime-edges.json")
	affectedGit(t, root, "commit", "-qm", "declare runtime edge")
	var first []byte
	for _, cache := range []string{"", filepath.Join(t.TempDir(), "cache")} {
		args := []string{"--path", "leaf/leaf.go"}
		if cache != "" {
			args = append(args, "--cache", cache)
		}
		_, raw, stderr, code := runAffectedCLI(t, root, args...)
		if code != 0 {
			t.Fatalf("exit=%d stderr=%s", code, stderr)
		}
		if first == nil {
			first = raw
		} else if !bytes.Equal(first, raw) {
			t.Fatal("cache changed degraded receipt")
		}
		var plan gateselect.Receipt
		if err := json.Unmarshal(raw, &plan); err != nil {
			t.Fatal(err)
		}
		lines := gateselect.SelectQualified(plan, "example.com/fixture", root)
		verdict := lines[len(lines)-1]
		if !strings.HasPrefix(verdict, "run ") || !strings.Contains(verdict, "example.com/fixture/leaf") || strings.Contains(verdict, "example.com/fixture/solo") {
			t.Fatalf("not narrowed: %v", lines)
		}
		found := false
		for _, unknown := range plan.Plan.Unknown {
			if strings.HasPrefix(unknown.Detail, `runtime:unresolved-edge: tests="leaf/leaf_test.go" runs="python:new.analyzer.unit": runs names no unit`) {
				found = true
			}
		}
		if !found {
			t.Fatalf("degradation missing: %s", raw)
		}
	}
	// The retry must validate the rest of the file, even after the unknown target.
	invalid := strings.Replace(declaration, `}]}`, `},{"tests":"missing_test.go","runs":"go:example.com/fixture/core","reason":"stale holder"}]}`, 1)
	if err := os.WriteFile(filepath.Join(root, ".corvint/runtime-edges.json"), []byte(invalid), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, stderr, code := runAffectedCLI(t, root)
	if code == 0 || !strings.Contains(stderr, "holds no test file") {
		t.Fatalf("invalid retry exit=%d stderr=%s", code, stderr)
	}
}
