// SPDX-License-Identifier: Apache-2.0
// conflict-markers-check refuses unresolved merge markers in governed source.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Intentional fixture paths must be exact files beneath testdata or fixtures;
// a blanket source directory cannot opt out. Add a reason beside each entry.
var fixtures = map[string]string{}

func governed(path string) bool {
	ext := filepath.Ext(path)
	return strings.HasPrefix(path, "docs/") || strings.HasPrefix(path, "script/") ||
		ext == ".go" || ext == ".json" || ext == ".sh" || ext == ".py" || ext == ".js" || ext == ".mjs" || ext == ".cjs"
}

func check(root string, paths []string, allow map[string]string) error {
	for path, reason := range allow {
		if strings.TrimSpace(reason) == "" || filepath.ToSlash(filepath.Clean(path)) != path || strings.ContainsAny(path, "*?[]") || strings.HasSuffix(path, "/") || !(strings.Contains("/"+path, "/testdata/") || strings.Contains("/"+path, "/fixtures/")) {
			return fmt.Errorf("invalid fixture exception: %s", path)
		}
	}
	var findings []string
	for _, path := range paths {
		if !governed(path) || allow[path] != "" {
			continue
		}
		name := filepath.Join(root, filepath.FromSlash(path))
		info, err := os.Lstat(name)
		if os.IsNotExist(err) {
			continue
		} // staged deletion
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		inConflict := false
		for i, line := range bytes.Split(raw, []byte{'\n'}) {
			line = bytes.TrimSuffix(line, []byte{'\r'})
			opener := bytes.HasPrefix(line, []byte("<<<<<<< "))
			closer := bytes.HasPrefix(line, []byte(">>>>>>> "))
			if opener || inConflict && bytes.Equal(line, []byte("=======")) || closer {
				findings = append(findings, fmt.Sprintf("%s:%d: unresolved merge marker", path, i+1))
			}
			if opener {
				inConflict = true
			} else if closer {
				inConflict = false
			}
		}
	}
	if len(findings) > 0 {
		return fmt.Errorf("%s", strings.Join(findings, "\n"))
	}
	return nil
}

func main() {
	if len(os.Args) > 1 {
		fmt.Println("usage: conflict-markers-check\nChecks tracked docs, Go, JSON and script files. Example: go run ./tools/conflict-markers-check")
		if os.Args[1] != "--help" {
			os.Exit(2)
		}
		return
	}
	raw, err := exec.Command("git", "ls-files", "-z").Output()
	if err == nil {
		err = check(".", strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00"), fixtures)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("conflict-markers-check: PASS")
}
