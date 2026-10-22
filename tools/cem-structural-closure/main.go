// Command cem-structural-closure digests the parser closure of one Go
// toolchain: the standard-library source the CEM structural predicates
// depend on (docs/specs/cem-stable-v1.md, CEM-V1-050, decision 0551).
//
//	go run ./tools/cem-structural-closure --goroot "$(go1.27.3 env GOROOT)"
//
// The closure is the union of `go list -deps` over the structural roots for
// every listed platform, run with the named toolchain's own go command, minus
// the runtime and operating-system packages, whose behaviour the structural
// predicates do not observe. A package's listing is one line per regular file
// directly in its GOROOT/src directory, `_test.go` files excepted:
// "<importpath>/<name> <sha256>". The closure digest is the sha256 of every
// package listing, in import-path order. A toolchain whose closure digest
// equals the qualified tuple's inherits that tuple's corpus qualification
// without a corpus run; any other digest needs the full corpus.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const profile = "cem-structural-parser-closure/1"

// roots are the packages the native and portable structural predicates
// import for parsing, scanning, printing and formatting.
var roots = []string{"go/ast", "go/format", "go/parser", "go/printer", "go/scanner", "go/token"}

var platforms = []string{"darwin/arm64", "linux/amd64", "linux/arm64", "windows/amd64"}

// excludedPackages and excludedTrees (a package and every package below it)
// are the runtime, scheduler, system call and file-system packages. They
// change across patch releases for reasons the predicates cannot observe: the
// predicates read only in-memory bytes and never touch time, files, goroutines
// or the environment. The lexical path packages stay in: go/scanner cleans and
// joins `//line` file names with path/filepath.
var excludedPackages = map[string]bool{
	"runtime": true, "os": true, "syscall": true, "time": true, "sync": true,
	"sync/atomic": true, "unsafe": true, "io/fs": true,
}

var excludedTrees = []string{
	"internal/runtime", "internal/syscall", "internal/trace",
	"internal/abi", "internal/asan", "internal/msan", "internal/race",
	"internal/coverage/rtcov", "internal/chacha8rand", "internal/cpu",
	"internal/goarch", "internal/goos", "internal/goexperiment",
	"internal/profilerecord", "internal/synctest", "internal/bisect",
	"internal/godebug", "internal/godebugs", "internal/testlog", "internal/poll",
	"internal/oserror", "internal/sync",
	"internal/unsafeheader",
}

type packageDigest struct {
	ImportPath string `json:"importPath"`
	Files      int    `json:"files"`
	Sha256     string `json:"sha256"`
}

type closure struct {
	Profile       string          `json:"profile"`
	GoVersion     string          `json:"goVersion"`
	Roots         []string        `json:"roots"`
	Platforms     []string        `json:"platforms"`
	Excluded      []string        `json:"excluded"`
	Packages      []packageDigest `json:"packages"`
	PackageCount  int             `json:"packageCount"`
	FileCount     int             `json:"fileCount"`
	ClosureSha256 string          `json:"closureSha256"`
}

func main() {
	goroot := flag.String("goroot", "", "GOROOT of the toolchain to digest (required)")
	flag.Parse()
	if *goroot == "" {
		fmt.Fprintln(os.Stderr, "cem-structural-closure: --goroot is required")
		os.Exit(2)
	}
	c, err := digestToolchain(*goroot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cem-structural-closure: %v\n", err)
		os.Exit(1)
	}
	out, _ := json.MarshalIndent(c, "", "  ")
	os.Stdout.Write(append(out, '\n'))
}

// digestToolchain lists the closure with the toolchain's own go command, from
// a scratch directory outside any module so no go.mod selects another
// toolchain.
func digestToolchain(goroot string) (closure, error) {
	version, err := os.ReadFile(filepath.Join(goroot, "VERSION"))
	if err != nil {
		return closure{}, err
	}
	scratch, err := os.MkdirTemp("", "cem-structural-closure-")
	if err != nil {
		return closure{}, err
	}
	defer os.RemoveAll(scratch)
	deps := map[string]bool{}
	for _, platform := range platforms {
		goos, goarch, _ := strings.Cut(platform, "/")
		cmd := exec.Command(filepath.Join(goroot, "bin", "go"), append([]string{"list", "-deps"}, roots...)...)
		cmd.Dir = scratch
		cmd.Env = append(os.Environ(), "GOROOT="+goroot, "GOTOOLCHAIN=local", "GOFLAGS=", "GO111MODULE=",
			"CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch, "GOWORK=off")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return closure{}, fmt.Errorf("go list for %s: %v: %s", platform, err, stderr.String())
		}
		for _, line := range strings.Fields(string(out)) {
			deps[line] = true
		}
	}
	list := make([]string, 0, len(deps))
	for p := range deps {
		list = append(list, p)
	}
	return digestClosure(filepath.Join(goroot, "src"), strings.SplitN(string(version), "\n", 2)[0], list)
}

func excluded(importPath string) bool {
	if excludedPackages[importPath] {
		return true
	}
	for _, tree := range excludedTrees {
		if importPath == tree || strings.HasPrefix(importPath, tree+"/") {
			return true
		}
	}
	return false
}

// digestClosure digests the included packages of deps under src.
func digestClosure(src, goVersion string, deps []string) (closure, error) {
	c := closure{Profile: profile, GoVersion: goVersion, Roots: roots, Platforms: platforms}
	for p := range excludedPackages {
		c.Excluded = append(c.Excluded, p)
	}
	for _, tree := range excludedTrees {
		c.Excluded = append(c.Excluded, tree+"/...")
	}
	sort.Strings(c.Excluded)
	sort.Strings(deps)
	whole := sha256.New()
	for _, importPath := range deps {
		if excluded(importPath) {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(src, filepath.FromSlash(importPath)))
		if err != nil {
			return closure{}, err
		}
		var listing bytes.Buffer
		files := 0
		for _, e := range entries {
			if !e.Type().IsRegular() || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(importPath), e.Name()))
			if err != nil {
				return closure{}, err
			}
			sum := sha256.Sum256(raw)
			fmt.Fprintf(&listing, "%s/%s %s\n", importPath, e.Name(), hex.EncodeToString(sum[:]))
			files++
		}
		sum := sha256.Sum256(listing.Bytes())
		whole.Write(listing.Bytes())
		c.Packages = append(c.Packages, packageDigest{ImportPath: importPath, Files: files, Sha256: hex.EncodeToString(sum[:])})
		c.FileCount += files
	}
	c.PackageCount = len(c.Packages)
	c.ClosureSha256 = hex.EncodeToString(whole.Sum(nil))
	return c, nil
}
