// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func runTestPlanCLI(t *testing.T, arguments ...string) (int, string, string) {
	t.Helper()
	if len(arguments) > 0 && arguments[0] == "test" &&
		hasTestArg(arguments, "--dry-run") && !hasTestArg(arguments, "--summary") &&
		!hasTestArg(arguments, "--pins") && !hasTestArg(arguments, "--full") {
		arguments = append(arguments, "--full")
	}
	var stdout, stderr bytes.Buffer
	code := run(arguments, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func hasTestArg(arguments []string, wanted string) bool {
	for _, value := range arguments {
		if value == wanted {
			return true
		}
	}
	return false
}

func reuseGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	prefix := []string{
		"-c", "maintenance.autoDetach=false", "-c", "gc.autoDetach=false",
		"-c", "gc.auto=0", "-c", "maintenance.auto=false", "-C", root,
	}
	out, err := fixtureGitCommand(append(prefix, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func staticSnapshot(t *testing.T, root string) string {
	t.Helper()
	var out strings.Builder
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(&out, "%s %d %v %d\n", path, info.Size(), info.Mode(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// After the walk, and without optional locks, so the snapshot never refreshes the index itself.
	out.WriteString(affectedGit(t, root, "--no-optional-locks", "status", "--porcelain",
		"--untracked-files=all", "--ignored"))
	return out.String()
}
