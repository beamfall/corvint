// SPDX-License-Identifier: Apache-2.0
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/releasecandidate"
)

// ARTIFACT-BIN-V0-509: the installer's core set is the catalog's `core` version set beside
// corvint, so a release candidate carries exactly the apps that must install beside corvint.
func TestCoreSetAppsMatchTheCatalogVersionSet(t *testing.T) {
	catalog, err := loadCatalog("")
	if err != nil {
		t.Fatal(err)
	}
	var members []string
	for _, binary := range catalog.Binaries {
		if binary.VersionSet == "core" && binary.Name != "corvint" {
			members = append(members, binary.Name)
		}
	}
	slices.Sort(members)
	if !slices.Equal(members, releasecandidate.CoreSetApps) {
		t.Fatalf("catalog core set %v; releasecandidate.CoreSetApps %v", members, releasecandidate.CoreSetApps)
	}
	if !slices.IsSorted(releasecandidate.CoreSetApps) {
		t.Fatalf("CoreSetApps is not sorted: %v", releasecandidate.CoreSetApps)
	}
}

// ARTIFACT-BIN-V0-522: the release candidate carries app archives for exactly the catalog's
// targets, so a manifest release-binary writes for those targets assembles into a candidate.
func TestAppPlatformsMatchTheCatalogTargets(t *testing.T) {
	catalog, err := loadCatalog("")
	if err != nil {
		t.Fatal(err)
	}
	var platforms []string
	for _, target := range catalog.Targets {
		platforms = append(platforms, target.GOOS+"_"+target.GOARCH)
	}
	slices.Sort(platforms)
	if !slices.Equal(platforms, releasecandidate.AppPlatforms) {
		t.Fatalf("catalog targets %v; releasecandidate.AppPlatforms %v", platforms, releasecandidate.AppPlatforms)
	}
}

// ARTIFACT-BIN-V0-508, -002: every stamped product binary declares the variables the release
// builder stamps (main.build and main.sourceCommit; main.version too when it is released outside
// the core set), so the -X stamps bind and --version reports them. A `stamp: none` product may
// only be an analyzer, whose --version is a protocol receipt owned by its contract; since the
// plan withholds the analyzers (ARTIFACT-BIN-V0-520) no released binary is unstamped.
func TestProductBinariesDeclareTheVersionStamp(t *testing.T) {
	catalog, err := loadCatalog("")
	if err != nil {
		t.Fatal(err)
	}
	root := repositoryRoot(t)
	for _, binary := range catalog.Binaries {
		if binary.Role != "product" {
			continue
		}
		if binary.Stamp == "none" {
			if !strings.HasPrefix(binary.Name, "corvint-analyzer-") {
				t.Errorf("%s: only analyzers are released unstamped", binary.Name)
			}
			continue
		}
		if err := declaredStampVariables(root, binary); err != nil {
			t.Errorf("%v", err)
		}
		declared := map[string]bool{}
		entries, err := os.ReadDir(filepath.Join(root, binary.Package))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, binary.Package, entry.Name()), nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			for _, declaration := range file.Decls {
				general, ok := declaration.(*ast.GenDecl)
				if !ok || general.Tok != token.VAR {
					continue
				}
				for _, spec := range general.Specs {
					for _, name := range spec.(*ast.ValueSpec).Names {
						declared[name.Name] = true
					}
				}
			}
		}
		for _, name := range binary.stampVariables() {
			if !declared[name] {
				t.Errorf("%s: main package does not declare %s (ARTIFACT-BIN-V0-508)", binary.Name, name)
			}
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// ARTIFACT-BIN-V0-509 / V1-5237: an install made from released archives runs `corvint docs`.
// The real corvint and corvint-docs are built for the host, archived through the release
// assembler, extracted into one directory as docs/INSTALL.md says, and `corvint docs --help`
// and a forwarded docs verb run there; the same extraction without corvint-docs is the named
// NOT_INSTALLED gap, which is what a one-binary release shipped.
func TestInstalledCoreSetRunsCorvintDocs(t *testing.T) {
	if testing.Short() {
		t.Skip("builds corvint and corvint-docs")
	}
	if runtime.GOOS == "windows" {
		t.Skip("tar.gz install path")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	root := repositoryRoot(t)
	catalog, err := loadCatalog("")
	if err != nil {
		t.Fatal(err)
	}
	target := Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	legal, err := loadLegalFiles(root, catalog)
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	archives := map[string][]byte{}
	for _, name := range []string{"corvint", "corvint-docs"} {
		binary, _ := catalog.find(name)
		built := filepath.Join(scratch, name)
		command := exec.Command("go", "build", "-trimpath", "-ldflags=-X main.build=7 -X main.sourceCommit="+strings.Repeat("7", 40), "-o", built, binary.Package)
		command.Dir = root
		command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "CGO_ENABLED=0")
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", name, err, out)
		}
		raw, err := os.ReadFile(built)
		if err != nil {
			t.Fatal(err)
		}
		archive, err := assembleArchive(name, target, name, raw, legal)
		if err != nil {
			t.Fatal(err)
		}
		archives[name] = archive
	}
	fixture := t.TempDir()
	for _, arguments := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t",
		"-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false",
		"-c", "maintenance.auto=false", "-c", "gc.auto=0",
		"commit", "-qm", "fixture", "--allow-empty"}} {
		if err := os.WriteFile(filepath.Join(fixture, "README.md"), []byte("# Fixture\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("git", arguments...)
		command.Dir = fixture
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, out)
		}
	}
	// PATH holds git alone, so a corvint-docs can only be found beside corvint.
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	toolsOnPath := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(toolsOnPath, "git")); err != nil {
		t.Fatal(err)
	}
	run := func(install string, arguments ...string) (int, string) {
		command := exec.Command(filepath.Join(install, "corvint"), arguments...)
		command.Env = []string{"PATH=" + toolsOnPath, "HOME=" + t.TempDir()}
		var output bytes.Buffer
		command.Stdout, command.Stderr = &output, &output
		err := command.Run()
		code := 0
		var exit *exec.ExitError
		if err != nil {
			if !asExit(err, &exit) {
				t.Fatal(err)
			}
			code = exit.ExitCode()
		}
		return code, output.String()
	}

	set := t.TempDir()
	extractInto(t, set, archives["corvint"], "corvint_"+target.GOOS+"_"+target.GOARCH)
	extractInto(t, set, archives["corvint-docs"], "corvint-docs_"+target.GOOS+"_"+target.GOARCH)
	if code, out := run(set, "--version"); code != 0 || !strings.HasPrefix(out, "Corvint ") || !strings.Contains(out, "build 7, commit "+strings.Repeat("7", 40)) {
		t.Fatalf("installed corvint --version: exit %d %q", code, out)
	}
	if code, out := run(set, "docs", "--help"); code != 0 || !strings.Contains(out, "Usage:") {
		t.Fatalf("installed corvint docs --help: exit %d %q", code, out)
	}
	if code, out := run(set, "--root", fixture, "docs", "citations", "check"); code != 0 || !strings.Contains(out, "docs citations check:") {
		t.Fatalf("installed corvint docs citations check (forwarded to corvint-docs): exit %d %q", code, out)
	}

	alone := t.TempDir()
	extractInto(t, alone, archives["corvint"], "corvint_"+target.GOOS+"_"+target.GOARCH)
	if code, out := run(alone, "--root", fixture, "docs", "citations", "check"); code != 2 || !strings.Contains(out, `"code": "not-installed"`) {
		t.Fatalf("corvint alone did not report the NOT_INSTALLED gap: exit %d %q", code, out)
	}

	// CMS-V0-002: a corvint-docs beside corvint at another version is refused by name.
	skewed := t.TempDir()
	extractInto(t, skewed, archives["corvint"], "corvint_"+target.GOOS+"_"+target.GOARCH)
	if err := execfile.WriteFile(filepath.Join(skewed, "corvint-docs"), []byte("#!/bin/sh\nprintf 'corvint-docs 9.9.9\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if code, out := run(skewed, "--root", fixture, "docs", "citations", "check"); code != 2 || !strings.Contains(out, `"code": "version-mismatch"`) {
		t.Fatalf("corvint beside a corvint-docs at another version: exit %d %q", code, out)
	}
}

func asExit(err error, exit **exec.ExitError) bool {
	e, ok := err.(*exec.ExitError)
	if ok {
		*exit = e
	}
	return ok
}

// extractInto unpacks one release archive's root directory into install, the way the
// INSTALL.md core-set step places every executable in one directory.
func extractInto(t *testing.T, install string, archive []byte, root string) {
	t.Helper()
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimPrefix(header.Name, root+"/")
		if name == header.Name || strings.Contains(name, "/") {
			t.Fatalf("archive member %s is not under %s", header.Name, root)
		}
		content, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if name == "SHA256SUMS" {
			name = root + ".SHA256SUMS"
		}
		if err := os.WriteFile(filepath.Join(install, name), content, os.FileMode(header.Mode)); err != nil {
			t.Fatal(err)
		}
	}
}
