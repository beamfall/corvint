// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testinventory"
)

// inventoryRepository writes files under a new repository and commits them.
func inventoryRepository(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	writeInventoryFiles(t, root, files)
	gitFixture(t, root, "init", "-q")
	commitAll(t, root, "fixture")
	return root
}

func writeInventoryFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func inventoryCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), args, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func inventoryRefusalCode(t *testing.T, stderr string) string {
	t.Helper()
	var body struct {
		Code string `json:"code"`
		OK   bool   `json:"ok"`
	}
	if err := json.Unmarshal([]byte(stderr), &body); err != nil || body.OK {
		t.Fatalf("stderr is not a refusal: %q", stderr)
	}
	return body.Code
}

// withInventoryExec replaces the executing-lister runner for one test.
func withInventoryExec(t *testing.T, exec testinventory.Exec) {
	t.Helper()
	previous := inventoryRefreshExec
	inventoryRefreshExec = exec
	t.Cleanup(func() { inventoryRefreshExec = previous })
}

// vitestStub is a Vitest executable path for nodeBin to find and an Exec
// that answers `vitest list --json` for web/src/sum.test.ts, first running
// during when it is set.
func vitestStub(t *testing.T, root string, during func()) testinventory.Exec {
	t.Helper()
	writeInventoryFiles(t, root, map[string]string{"web/node_modules/.bin/vitest": "", "web/node_modules/.bin/vitest.cmd": ""})
	return func(_ context.Context, dir string, argv []string) ([]byte, error) {
		if filepath.Base(argv[0]) != "vitest" && filepath.Base(argv[0]) != "vitest.cmd" || strings.Join(argv[1:], " ") != "list --json --config vitest.config.ts" {
			return nil, fmt.Errorf("unexpected argv %v", argv)
		}
		if during != nil {
			during()
		}
		file, _ := json.Marshal(filepath.Join(dir, "src", "sum.test.ts"))
		return []byte(`[{"name":"sum > adds","file":` + string(file) + `}]`), nil
	}
}

// TIV-V1-001, TIV-V1-002, TIV-V1-004: `corvint inventory refresh` on a
// repository with one Go module and one Vitest project writes one record for
// each, keyed by canonical IDs, and reports what it wrote.
func TestTIVV1001InventoryRefreshCLI(t *testing.T) {
	root := inventoryRepository(t, map[string]string{
		".gitignore":           "node_modules/\n",
		"go.mod":               "module example.com/m\n",
		"calc/calc_test.go":    "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {}\n",
		"web/vitest.config.ts": "export default {}\n",
		"web/src/sum.test.ts":  "describe('sum', () => test('adds', () => {}))\n",
	})
	withInventoryExec(t, vitestStub(t, root, nil))
	code, stdout, stderr := inventoryCLI(t, "--root", root, "inventory", "refresh")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var summary struct {
		OK        bool
		Path      string
		Commit    string
		Changed   bool
		Instances []struct {
			Key, Kind string
			Tests     int
		}
		Failures []any
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("stdout %q: %v", stdout, err)
	}
	if !summary.OK || summary.Path != testinventory.Path || !summary.Changed || len(summary.Instances) != 2 || len(summary.Failures) != 0 ||
		summary.Commit != gitOutput(t, root, "rev-parse", "HEAD") {
		t.Fatalf("summary %+v", summary)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(testinventory.Path)))
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := testinventory.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, instance := range inventory.Instances {
		for _, test := range instance.Tests {
			ids[test.ID] = instance.Key + " " + instance.Lister.Kind
		}
	}
	if ids["go:calc::TestAdd"] != "go@. static" || ids["vitest:web/src/sum.test.ts::sum::adds"] != "vitest@web executing" || len(ids) != 2 {
		t.Fatalf("inventory tests %v", ids)
	}
	code, stdout, _ = inventoryCLI(t, "--root", root, "inventory", "refresh", "--runner", "go")
	if code != 0 || !strings.Contains(stdout, `"changed":false`) {
		t.Fatalf("filtered refresh of an unchanged tree: exit %d %s", code, stdout)
	}
	if again, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(testinventory.Path))); !bytes.Equal(again, data) {
		t.Fatal("a --runner refresh dropped or changed the other records")
	}
}

// TIV-V1-010: a test file edited while the Vitest lister runs refuses with
// inventory-refresh-drift and keeps the previous inventory.
func TestTIVV1010InventoryRefreshDriftCLI(t *testing.T) {
	root := inventoryRepository(t, map[string]string{
		".gitignore":           "node_modules/\n",
		"web/vitest.config.ts": "export default {}\n",
		"web/src/sum.test.ts":  "describe('sum', () => test('adds', () => {}))\n",
	})
	withInventoryExec(t, vitestStub(t, root, nil))
	if code, _, stderr := inventoryCLI(t, "--root", root, "inventory", "refresh"); code != 0 {
		t.Fatalf("first refresh: %s", stderr)
	}
	target := filepath.Join(root, filepath.FromSlash(testinventory.Path))
	old, _ := os.ReadFile(target)
	withInventoryExec(t, vitestStub(t, root, func() {
		writeInventoryFiles(t, root, map[string]string{"web/src/sum.test.ts": "test('edited', () => {})\n"})
	}))
	code, stdout, stderr := inventoryCLI(t, "--root", root, "inventory", "refresh")
	if code != 2 || stdout != "" || inventoryRefusalCode(t, stderr) != testinventory.ReasonDrift {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if now, _ := os.ReadFile(target); !bytes.Equal(now, old) {
		t.Fatal("drift refusal changed the inventory")
	}
}

// TIV-V1-011: a refresh whose inventory would exceed 8 MiB refuses with
// inventory-too-large and keeps the previous inventory.
func TestTIVV1011InventoryRefreshTooLargeCLI(t *testing.T) {
	t.Parallel()
	root := inventoryRepository(t, map[string]string{
		"go.mod":      "module example.com/m\n",
		"p/a_test.go": "package p\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n",
	})
	if code, _, stderr := inventoryCLI(t, "--root", root, "inventory", "refresh"); code != 0 {
		t.Fatalf("first refresh: %s", stderr)
	}
	target := filepath.Join(root, filepath.FromSlash(testinventory.Path))
	old, _ := os.ReadFile(target)
	long := strings.Repeat("X", 240)
	for file := 0; file < 3; file++ {
		var source strings.Builder
		source.WriteString("package p\n\nimport \"testing\"\n\n")
		for test := 0; test < 9500; test++ {
			fmt.Fprintf(&source, "func Test%s%d_%d(t *testing.T) {}\n", long, file, test)
		}
		writeInventoryFiles(t, root, map[string]string{fmt.Sprintf("p/big%d_test.go", file): source.String()})
	}
	code, stdout, stderr := inventoryCLI(t, "--root", root, "inventory", "refresh")
	if code != 2 || stdout != "" || inventoryRefusalCode(t, stderr) != testinventory.ReasonTooLarge {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if now, _ := os.ReadFile(target); !bytes.Equal(now, old) {
		t.Fatal("oversize refusal changed the inventory")
	}
}

// The verb has help with usage and an example and refuses bad arguments
// with exit 2 and a typed diagnostic on stderr only.
func TestTIVV1002InventoryHelpAndRefusals(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"inventory", "--help"}, {"inventory", "refresh", "--help"}, {"help", "inventory"}} {
		code, stdout, stderr := inventoryCLI(t, args...)
		if code != 0 || stderr != "" || !strings.Contains(stdout, "Usage:\n  corvint [--root PATH] inventory refresh") || !strings.Contains(stdout, "Example:\n  corvint inventory refresh") {
			t.Fatalf("%v: exit %d stdout %q stderr %q", args, code, stdout, stderr)
		}
	}
	for _, args := range [][]string{{"inventory"}, {"inventory", "frob"}, {"inventory", "refresh", "--runner", "cobol"}, {"inventory", "refresh", "--root", "../x"}, {"inventory", "refresh", "extra"}} {
		code, stdout, stderr := inventoryCLI(t, args...)
		if code != 2 || stdout != "" || inventoryRefusalCode(t, stderr) != "inventory-invalid-arguments" {
			t.Fatalf("%v: exit %d stdout %q stderr %q", args, code, stdout, stderr)
		}
	}
}
