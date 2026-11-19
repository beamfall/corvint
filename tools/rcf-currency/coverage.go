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

type releaseTarget struct {
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
	Suffix string `json:"binarySuffix"`
}

type docsBuild struct {
	pkg     string
	targets []releaseTarget
}

// The live test exercises its own main package. Only one product mapping in
// the supported catalog format can bind its evidence to the shipped program.
func qualificationBuild(raw []byte) (docsBuild, bool) {
	var catalog struct {
		Schema      string            `json:"schema"`
		LegalFiles  []string          `json:"legalFiles"`
		VersionSets map[string]string `json:"versionSets"`
		Targets     []releaseTarget   `json:"targets"`
		Binaries    []struct {
			Name       string `json:"name"`
			Package    string `json:"package"`
			Role       string `json:"role"`
			VersionSet string `json:"versionSet"`
			Decision   string `json:"decision"`
			Stamp      string `json:"stamp"`
		} `json:"binaries"`
	}
	if jsonv2.Unmarshal(raw, &catalog, jsonv2.RejectUnknownMembers(true)) != nil ||
		catalog.Schema != "corvint.release-binaries/1" || len(catalog.Targets) == 0 {
		return docsBuild{}, false
	}
	var pkg string
	for _, binary := range catalog.Binaries {
		if binary.Name == "corvint-docs" {
			if pkg != "" || binary.Role != "product" || binary.Package != qualificationPackage {
				return docsBuild{}, false
			}
			pkg = binary.Package
		}
	}
	return docsBuild{pkg, catalog.Targets}, pkg != ""
}

// A clean candidate may reuse a pin only if every shipped build's local inputs
// are modeled and owned. An incomplete proof takes the usual qualification path.
func (g gate) productionCovered(build docsBuild, patterns []string, pin, candidate string) bool {
	raw, err := g.git("ls-tree", "-rz", "--format=%(objectmode) %(path)", candidate)
	if err != nil {
		return false
	}
	regular := make(map[string]bool)
	for entry := range strings.SplitSeq(string(raw), "\x00") {
		mode, path, ok := strings.Cut(entry, " ")
		if ok && (mode == "100644" || mode == "100755") {
			regular[path] = true
		}
	}
	owned := func(path string) bool { return regular[path] && qualificationOwns(patterns, path) }
	if !owned(releaseCatalog) || !owned("go.mod") {
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
	return build.covered(root, owned, deleted)
}

func (build docsBuild) covered(root string, owned func(string) bool, deleted []string) bool {
	// go list omits implicit command inputs, including the automatic PGO
	// profile. Inspect the directory too, including ignored checkout files.
	dir := strings.TrimPrefix(build.pkg, "./")
	entries, err := os.ReadDir(filepath.Join(root, dir))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() == "testdata" {
			continue
		}
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".go") || !owned(dir+"/"+entry.Name()) {
			return false
		}
	}
	for _, target := range build.targets {
		if target.GOOS == "" || target.GOARCH == "" {
			return false
		}
		cmd := exec.Command("go", "list", "-deps", "-json", "-buildvcs=false", build.pkg)
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
		if err != nil || !coveredPackages(root, dir, list, owned, deleted) {
			return false
		}
	}
	return true
}

func coveredPackages(root, commandDir string, list []byte, owned func(string) bool, deleted []string) bool {
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
		if dir == commandDir && pkg.Name == "main" {
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
				if !owned(path) {
					return false
				}
			}
		}
	}
}

func qualificationOwns(patterns []string, path string) bool {
	return slices.ContainsFunc(patterns, func(p string) bool { return affected.OwnerPatternMatches(p, path) })
}
