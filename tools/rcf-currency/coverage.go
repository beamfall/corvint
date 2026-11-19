// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/conformance/release-artifact-v0/archivebuild"
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
func qualificationBuild(raw []byte) (docsBuild, error) {
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
	if err := jsonv2.Unmarshal(raw, &catalog, jsonv2.RejectUnknownMembers(true)); err != nil {
		return docsBuild{}, err
	}
	if catalog.Schema != "corvint.release-binaries/1" {
		return docsBuild{}, fmt.Errorf("unsupported catalog schema %q", catalog.Schema)
	}
	if len(catalog.Targets) == 0 {
		return docsBuild{}, fmt.Errorf("catalog has no release targets")
	}
	seen := map[[2]string]bool{}
	for _, target := range catalog.Targets {
		pair := [2]string{target.GOOS, target.GOARCH}
		if target.GOOS == "" || target.GOARCH == "" || seen[pair] {
			return docsBuild{}, fmt.Errorf("target %q must be a unique goos/goarch pair", strings.Join(pair[:], "/"))
		}
		seen[pair] = true
	}
	var pkg string
	for _, binary := range catalog.Binaries {
		if binary.Name != "corvint-docs" {
			continue
		}
		if pkg != "" {
			return docsBuild{}, fmt.Errorf("duplicate corvint-docs product")
		}
		if binary.Role != "product" {
			return docsBuild{}, fmt.Errorf("corvint-docs role %q must be product", binary.Role)
		}
		if binary.Package != qualificationPackage {
			return docsBuild{}, fmt.Errorf("corvint-docs package %q must match live runner %q", binary.Package, qualificationPackage)
		}
		pkg = binary.Package
	}
	if pkg == "" {
		return docsBuild{}, fmt.Errorf("missing corvint-docs product")
	}
	return docsBuild{pkg, catalog.Targets}, nil
}

type productionInput struct {
	target          releaseTarget
	dir, kind, path string
}

// A clean candidate may reuse a pin only if every shipped build's local inputs
// are modeled and owned. An incomplete proof takes the usual qualification path.
func (g gate) productionCovered(build docsBuild, patterns []string, pin, candidate, scratch, candidateRoot string) bool {
	pinnedRoot := filepath.Join(scratch, "qualified")
	if err := g.checkout(pin, pinnedRoot); err != nil {
		return false
	}
	removed, err := g.git("diff", "--diff-filter=D", "--name-only", "-z", "--no-renames", pin, candidate, "--")
	if err != nil {
		return false
	}
	deleted := strings.Split(strings.TrimSuffix(string(removed), "\x00"), "\x00")
	pinned, ok := (gate{root: pinnedRoot}).productionInputs(build, patterns, nil, "")
	if !ok {
		return false
	}
	current, ok := (gate{root: candidateRoot}).productionInputs(build, patterns, deleted, g.root)
	return ok && maps.Equal(pinned, current)
}

// Read each inventory from its own detached checkout. Track the package, target
// and use of each path so a file moving between input sets cannot disappear in a union.
func (g gate) productionInputs(build docsBuild, patterns, deleted []string, callerRoot string) (map[productionInput]bool, bool) {
	raw, err := g.git("ls-tree", "-rz", "--format=%(objectmode) %(path)", "HEAD")
	if err != nil {
		return nil, false
	}
	regular := map[string]bool{}
	for entry := range strings.SplitSeq(string(raw), "\x00") {
		mode, path, ok := strings.Cut(entry, " ")
		if !ok {
			continue
		}
		regular[path] = mode == "100644" || mode == "100755"
		driverInput := strings.HasPrefix(path, "tools/release-binary/") || strings.HasPrefix(path, "conformance/release-artifact-v0/archivebuild/")
		if driverInput && (!regular[path] || !qualificationOwns(patterns, path)) {
			return nil, false
		}
	}
	owned := func(path string) bool { return regular[path] && qualificationOwns(patterns, path) }
	if !owned(releaseCatalog) || !owned("go.mod") {
		return nil, false
	}
	if callerRoot != "" && !build.commandCovered(callerRoot, owned) {
		return nil, false
	}
	inputs := map[productionInput]bool{}
	return inputs, build.covered(g.root, owned, deleted, inputs)
}

func (build docsBuild) commandCovered(root string, owned func(string) bool) bool {
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
	return true
}

func (build docsBuild) covered(root string, owned func(string) bool, deleted []string, inputs map[productionInput]bool) bool {
	if !build.commandCovered(root, owned) {
		return false
	}
	dir := strings.TrimPrefix(build.pkg, "./")
	for _, target := range build.targets {
		cmd := exec.Command("go", "list", archivebuild.ReleaseTrimpath, "-deps", "-json", "-buildvcs=false", build.pkg)
		cmd.Dir = root
		cmd.Env = archivebuild.ReleaseEnvironment(target.GOOS, target.GOARCH)
		list, err := commandOutput(cmd, "go list")
		if err != nil || !coveredPackages(root, dir, list, owned, deleted, target, inputs) {
			return false
		}
	}
	return true
}

func coveredPackages(root, commandDir string, list []byte, owned func(string) bool, deleted []string, target releaseTarget, inputs map[productionInput]bool) bool {
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
		for kind, files := range map[string][]string{"GoFiles": pkg.GoFiles, "EmbedFiles": pkg.EmbedFiles, "IgnoredGoFiles": pkg.IgnoredGoFiles} {
			for _, file := range files {
				if !filepath.IsLocal(file) {
					return false
				}
				path := filepath.ToSlash(filepath.Join(dir, file))
				inputs[productionInput{target, dir, kind, path}] = true
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
