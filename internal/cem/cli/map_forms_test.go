// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// CEM-V1-7400: both verbs accept either spelling of an in-repository map.
func TestCEMV17400MapForms(t *testing.T) {
	f := materializeStableRead(t)
	for _, path := range []string{"change.cem.json", f.mapPath} {
		exit, envelope, refusal := runCoreRead(t, f.repo, "verify", "--map", path, "--expected-base", f.expectedBase, "--target", f.target, "--artifacts", f.artifacts)
		if refusal != nil || exit != 0 {
			t.Fatalf("verify %s: exit=%d envelope=%v refusal=%v", path, exit, envelope, refusal)
		}
		var out, stderr bytes.Buffer
		code, handled := RunStableInvocation(context.Background(), []string{"cem", "verify-stable", "--repository", f.repo, "--map", path, "--expected-base", f.expectedBase, "--target", f.target, "--artifacts", f.artifacts}, &out, &stderr)
		var result map[string]any
		_ = json.Unmarshal(out.Bytes(), &result)
		if !handled || code != 0 {
			t.Fatalf("verify-stable %s: exit=%d output=%s stderr=%s", path, code, out.String(), stderr.String())
		}
	}
}

// CEM-V1-7400 keeps traversal out of the relative alias.
func TestCEMV17400MapTraversal(t *testing.T) {
	f := materializeStableRead(t)
	for _, path := range []string{"../change.cem.json", "sub/../change.cem.json"} {
		var out, stderr bytes.Buffer
		code, _ := RunStableInvocation(context.Background(), []string{"cem", "verify-stable", "--repository", f.repo, "--map", path, "--expected-base", f.expectedBase, "--target", f.target, "--artifacts", f.artifacts}, &out, &stderr)
		var result map[string]any
		_ = json.Unmarshal(out.Bytes(), &result)
		if code != 2 || result["code"] != "invalid-arguments" {
			t.Fatalf("%s: %s", path, out.String())
		}
	}
}

// CEM-V1-7400: decide absolute-map admission by the resolved parent identity;
// an in-repository alias is accepted, while a Git-directory alias is refused.
func TestCEMV17400AbsoluteAliases(t *testing.T) {
	f := materializeStableRead(t)
	alias := filepath.Join(t.TempDir(), "repo-alias")
	if err := os.Symlink(f.repo, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	path := filepath.Join(alias, "change.cem.json")
	code, _, refusal := runCoreRead(t, f.repo, "verify", "--map", path, "--expected-base", f.expectedBase, "--target", f.target, "--artifacts", f.artifacts)
	if code != 0 || refusal != nil {
		t.Fatalf("in-tree alias: code=%d refusal=%v", code, refusal)
	}
	data, err := os.ReadFile(f.mapPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.repo, ".git", "map.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	metadata := filepath.Join(f.repo, "metadata-alias")
	if err := os.Symlink(".git", metadata); err != nil {
		t.Fatal(err)
	}
	code, _, refusal = runCoreRead(t, f.repo, "verify", "--map", filepath.Join(metadata, "map.json"), "--expected-base", f.expectedBase, "--target", f.target, "--artifacts", f.artifacts)
	if code != 2 || refusal["code"] != "invalid-arguments" {
		t.Fatalf("metadata alias: code=%d refusal=%v", code, refusal)
	}
}
