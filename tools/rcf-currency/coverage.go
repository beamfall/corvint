// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

const releaseCatalog = "tools/release-binary/binaries.json"

// A clean candidate may reuse a pin only if every shipped build's local inputs
// are modeled and owned. An incomplete proof takes the usual qualification path.
func (g gate) productionCovered(patterns []string, pin, candidate string) bool {
	// Changes to either build description can alter the closure itself.
	if !qualificationOwns(patterns, releaseCatalog) || !qualificationOwns(patterns, "go.mod") {
		return false
	}
	raw, err := g.git("show", candidate+":"+releaseCatalog)
	if err != nil {
		return false
	}
	var catalog struct {
		Targets []struct {
			GOOS   string `json:"goos"`
			GOARCH string `json:"goarch"`
		} `json:"targets"`
	}
	if jsonv2.Unmarshal(raw, &catalog) != nil || len(catalog.Targets) == 0 {
		return false
	}
	root, err := filepath.Abs(g.root)
	if err != nil {
		return false
	}
	// A deleted source or globbed embed can disappear from go list. Keep
	// deletions under a reachable package in scope even if its owner was absent.
	removed, err := g.git("diff", "--diff-filter=D", "--name-only", "-z", "--no-renames", pin, candidate, "--")
	if err != nil {
		return false
	}
	deleted := strings.Split(strings.TrimSuffix(string(removed), "\x00"), "\x00")
	for _, target := range catalog.Targets {
		if target.GOOS == "" || target.GOARCH == "" {
			return false
		}
		cmd := exec.Command("go", "list", "-deps", "-json", "-buildvcs=false", "./cmd/corvint-docs")
		cmd.Dir = root
		// Match the release builder's pure-Go, offline environment. In
		// particular host build tags or an overlay must not narrow the closure.
		cmd.Env = []string{"CGO_ENABLED=0", "GOENV=off", "GOFLAGS=-mod=readonly", "GOPROXY=off", "GOSUMDB=off",
			"GOTOOLCHAIN=local", "GOWORK=off", "GOOS=" + target.GOOS, "GOARCH=" + target.GOARCH}
		for _, key := range []string{"HOME", "PATH", "TMPDIR", "TEMP", "TMP", "GOCACHE", "GOMODCACHE", "GOPATH", "GOROOT", "SystemRoot", "USERPROFILE", "GOMAXPROCS"} {
			if value, ok := os.LookupEnv(key); ok {
				cmd.Env = append(cmd.Env, key+"="+value)
			}
		}
		list, err := commandOutput(cmd, "go list")
		if err != nil || !coveredPackages(root, list, patterns, deleted) {
			return false
		}
	}
	return true
}

func coveredPackages(root string, list []byte, patterns, deleted []string) bool {
	dec := jsontext.NewDecoder(bytes.NewReader(list))
	foundCommand := false
	for {
		var pkg struct {
			Standard                            bool
			Dir, Name                           string
			Module                              struct{ Main bool }
			GoFiles, EmbedFiles, IgnoredGoFiles []string
			Other                               map[string]jsontext.Value `json:",embed"`
		}
		err := jsonv2.UnmarshalDecode(dec, &pkg)
		if err == io.EOF {
			return foundCommand
		}
		if err != nil {
			return false
		}
		if pkg.Standard {
			continue
		}
		// Only ordinary Go files and resolved embeds are modeled inputs.
		// Admit known descriptive and test-only metadata. Any other field
		// (including native sources, load errors or a new input kind) widens.
		for key := range pkg.Other {
			switch key {
			case "DepOnly", "Deps", "Doc", "EmbedPatterns", "ImportPath", "Imports", "Match", "Root",
				"Stale", "StaleReason", "Target", "DefaultGODEBUG",
				"TestGoFiles", "TestImports", "TestEmbedPatterns", "TestEmbedFiles",
				"XTestGoFiles", "XTestImports", "XTestEmbedPatterns", "XTestEmbedFiles":
			default:
				return false
			}
		}
		dir, err := filepath.Rel(root, pkg.Dir)
		if err != nil || !filepath.IsLocal(dir) || !pkg.Module.Main || len(pkg.GoFiles) == 0 {
			return false
		}
		dir = filepath.ToSlash(dir)
		if dir == "cmd/corvint-docs" && pkg.Name == "main" {
			foundCommand = true
		}
		for _, path := range deleted {
			if path != "" && (dir == "." || strings.HasPrefix(path, dir+"/")) {
				return false
			}
		}
		for _, files := range [][]string{pkg.GoFiles, pkg.EmbedFiles, pkg.IgnoredGoFiles} {
			for _, file := range files {
				if !filepath.IsLocal(file) {
					return false
				}
				path := filepath.ToSlash(filepath.Join(dir, file))
				if !qualificationOwns(patterns, path) {
					return false
				}
			}
		}
	}
}

func qualificationOwns(patterns []string, path string) bool {
	return slices.ContainsFunc(patterns, func(p string) bool { return affected.OwnerPatternMatches(p, path) })
}
