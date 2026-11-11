// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNCMV1003ProposeDataMapCommand: `affected --propose-data-map` writes the
// proposal to stdout and no file (NCM-V1-003), and refuses an invalid map
// with data-map-invalid and exit 2 (NCM-V1-007).
func TestNCMV1003ProposeDataMapCommand(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	affectedGit(t, root, "init", "-q")
	recorded := filepath.Join(root, "reads.jsonl")
	body := `{"test":"go:internal/app::TestLoad","path":"config/app.yml"}` + "\n" +
		`{"test":"go:internal/app::TestReload","path":"config/app.yml"}` + "\n" +
		`{"test":"go:internal/web::TestServe","path":"config/app.yml"}` + "\n"
	if err := os.WriteFile(recorded, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(arguments ...string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		code := runContext(context.Background(), append([]string{"--root", root, "affected"}, arguments...), strings.NewReader(""), &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	code, stdout, stderr := run("--propose-data-map", "reads.jsonl")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, want := range []string{`"profile": "corvint-data-map/0"`, `"path": "config/app.yml"`, `"go:internal/web::TestServe"`, "3 test(s) read this path"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "accepted_by") || strings.Contains(stdout, "proof") {
		t.Fatalf("a proposal carries acceptance or proof:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(root, ".corvint")); !os.IsNotExist(err) {
		t.Fatalf("the proposal wrote under .corvint/: %v", err)
	}
	if code, again, _ := run("--propose-data-map=reads.jsonl"); code != 0 || again != stdout {
		t.Fatalf("a second proposal differs (exit %d)", code)
	}
	if code, _, stderr := run("--propose-data-map", "reads.jsonl", "--full"); code != 2 || !strings.Contains(stderr, "takes no other argument") {
		t.Fatalf("an extra argument: exit %d %s", code, stderr)
	}
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint", "data-map.json"), []byte(`{"profile":"corvint-data-map/1","entries":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = run("--propose-data-map", "reads.jsonl")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "data-map-invalid") {
		t.Fatalf("an invalid map: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}
