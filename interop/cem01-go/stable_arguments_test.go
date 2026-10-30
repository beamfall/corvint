package main

// CEM-V1-115..117 portable conformance, paired with
// TestCEMV1115ArgumentRefusalsNative in internal/cem/cli: the same argv gives
// the same UNSUPPORTED stage and code on both stable entry points, and a
// relative path option is refused with the absolute-path fix beside the
// result. The portable ABI has no global --root option.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const (
	stableArgumentBase   = "467cbb73c1102cd7df0299cd74a6bef96ee3a9a4"
	stableArgumentTarget = "1f181cd0843318b72bb6679cd19878c46eb95ada"
)

func TestCEMV1115ArgumentRefusalsPortable(t *testing.T) {
	neutral := neutralCaseFiles(t)
	keys := filepath.Join(t.TempDir(), "trusted-keys.json")
	if err := os.WriteFile(keys, []byte(`{"profile":"corvint-trusted-keys/1","keys":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	mapPath := filepath.Join(neutral, "stable.json")
	full := func(repository, artifacts string, extra ...string) []string {
		a := []string{"--repository", repository, "--map", mapPath, "--expected-base", stableArgumentBase, "--target", stableArgumentTarget}
		if artifacts != "" {
			a = append(a, "--artifacts", artifacts)
		}
		return append(a, extra...)
	}
	for _, c := range []struct {
		name  string
		argv  []string
		stage string
		code  string
		fix   string
	}{
		{"missing map", []string{"--repository", "/nonexistent-repository", "--expected-base", stableArgumentBase, "--target", stableArgumentTarget, "--artifacts", neutral}, "arguments", "invalid-arguments", ""},
		{"relative repository", full("relative/repository", neutral), "arguments", "invalid-arguments", "--repository"},
		{"repeated repository with a relative value", append([]string{"--repository", "/nonexistent-repository"}, full("relative/repository", neutral)...), "arguments", "invalid-arguments", "--repository"},
		{"trusted keys with no keys", full("/nonexistent-repository", neutral, "--trusted-keys", keys), "arguments", "invalid-arguments", ""},
		{"relative artifacts", full("/nonexistent-repository", "relative/artifacts"), "authority-arguments", "invalid-arguments", "--artifacts"},
		{"missing artifacts", full("/nonexistent-repository", ""), "authority-arguments", "artifacts-required", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			for entry, execute := range map[string]func(context.Context, []string) (stableResult, int){"verify-stable": executeStableCLI, "verify-stable-canonical": executeStableCanonicalCLI} {
				r, exit := execute(context.Background(), c.argv)
				encoded, _ := json.Marshal(r)
				var result map[string]any
				if err := json.Unmarshal(encoded, &result); err != nil {
					t.Fatal(err)
				}
				if exit != 2 || result["outcome"] != "UNSUPPORTED" || result["accept"] != nil || result["assurance"] != nil || result["stage"] != c.stage || result["code"] != c.code {
					t.Fatalf("%s: exit %d %s, want UNSUPPORTED %s/%s exit 2", entry, exit, encoded, c.stage, c.code)
				}
				fix := stableRelativePathFix(r, c.argv)
				want := ""
				if c.fix != "" {
					want = "invalid-arguments: relative path option " + c.fix + "; fix: pass each as an absolute path such as \"$PWD/<path>\"; a relative path is refused, never resolved"
				}
				if fix != want {
					t.Fatalf("%s: fix %q, want %q", entry, fix, want)
				}
			}
		})
	}
	// The absolute form passes both argument stages and names no fix.
	argv := full("/nonexistent-repository", neutral)
	r, _ := executeStableCanonicalCLI(context.Background(), argv)
	if r.Stage == "arguments" || r.Stage == "authority-arguments" || stableRelativePathFix(r, argv) != "" {
		t.Fatalf("absolute form stopped at %s", r.Stage)
	}
}
