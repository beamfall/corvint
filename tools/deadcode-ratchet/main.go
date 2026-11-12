// SPDX-License-Identifier: Apache-2.0

// deadcode-ratchet runs golang.org/x/tools/cmd/deadcode at one pinned version
// for six GOOS/GOARCH configurations, keeps the functions every run reports,
// and compares them with the shrink-only baseline in baseline.json
// (DCR-V0-001 to DCR-V0-005, docs/specs/deadcode-ratchet-v0.md). It is a
// development tool behind `make deadcode` and `make deadcode-baseline`; no
// product package imports it.
//
// Usage, from the module root:
//
//	go run ./tools/deadcode-ratchet -version v0.52.0 [-out DIR] [-write-baseline]
package main

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	analyzerModule  = "golang.org/x/tools"
	analyzerPackage = "golang.org/x/tools/cmd/deadcode"
	defaultBaseline = "tools/deadcode-ratchet/baseline.json"
)

func main() {
	var (
		version  = flag.String("version", "", "exact version of "+analyzerModule+" to install (required, for example v0.52.0)")
		baseline = flag.String("baseline", defaultBaseline, "baseline file, relative to the module root")
		out      = flag.String("out", "", "directory that keeps each configuration's analyzer JSON (default: a scratch directory removed on exit)")
		write    = flag.Bool("write-baseline", false, "rewrite the baseline from the report instead of comparing (make deadcode-baseline)")
	)
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `Usage: go run ./tools/deadcode-ratchet -version VERSION [-out DIR] [-write-baseline]

Installs %s@VERSION for the host into a scratch directory, runs it with -test
-json for linux, darwin and windows on amd64 and arm64 over every package of
the module (dot directories included), keeps the functions all six runs report
and compares them with the baseline. Exit 0 when they match, 1 otherwise.

Example: go run ./tools/deadcode-ratchet -version v0.52.0

`, analyzerPackage)
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := runMain(ctx, *version, *baseline, *out, *write)
	stop()
	os.Exit(code)
}

func runMain(ctx context.Context, version, baselinePath, outDir string, write bool) int {
	fail := func(format string, args ...any) int {
		fmt.Fprintf(os.Stderr, "deadcode-ratchet: "+format+"\n", args...)
		return 1
	}
	if !validVersion(version) {
		return fail("-version %q is not an exact module version (want vMAJOR.MINOR.PATCH)", version)
	}
	root, err := os.Getwd()
	if err != nil {
		return fail("%v", err)
	}
	modBefore, err := moduleFiles(root)
	if err != nil {
		return fail("run from the module root: %v", err)
	}
	var old []baselineEntry
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(baselinePath)))
	switch {
	case err == nil:
		if old, err = decodeBaseline(data); err != nil {
			return fail("%s: %v", baselinePath, err)
		}
	case write && errors.Is(err, os.ErrNotExist):
	default:
		return fail("%v", err)
	}

	scratch, err := os.MkdirTemp("", "corvint-deadcode-")
	if err != nil {
		return fail("%v", err)
	}
	defer os.RemoveAll(scratch)
	if outDir == "" {
		outDir = filepath.Join(scratch, "runs")
	}
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return fail("%v", err)
	}

	start := time.Now()
	bin, err := install(ctx, scratch, version)
	if err != nil {
		return fail("cannot install %s@%s: %v; no other analyzer is run", analyzerPackage, version, err)
	}
	patterns, err := packagePatterns(root)
	if err != nil {
		return fail("%v", err)
	}
	runs := map[config]run{}
	for _, c := range configs {
		t := time.Now()
		r, err := analyze(ctx, bin, root, c, patterns, filepath.Join(outDir, c.file()))
		if err != nil {
			return fail("configuration %s: %v; no verdict", c, err)
		}
		runs[c] = r
		fmt.Fprintf(os.Stderr, "deadcode-ratchet: %s reports %d functions (%.1fs)\n", c, len(r), time.Since(t).Seconds())
	}
	report, err := intersect(runs)
	if err != nil {
		return fail("%v", err)
	}
	modAfter, err := moduleFiles(root)
	if err != nil {
		return fail("%v", err)
	}
	if !bytes.Equal(modBefore, modAfter) {
		return fail("go.mod or go.sum changed during the run")
	}
	fmt.Fprintf(os.Stderr, "deadcode-ratchet: %d functions unreachable in all %d configurations (%s, %.1fs)\n",
		len(report), len(configs), analyzerPackage+"@"+version, time.Since(start).Seconds())

	if write {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(baselinePath)), encodeBaseline(report, old), 0o644); err != nil {
			return fail("%v", err)
		}
		fmt.Fprintf(os.Stderr, "deadcode-ratchet: wrote %d entries to %s; set wantBaselineEntries in baseline_test.go to %d\n", len(report), baselinePath, len(report))
		return 0
	}
	lines := verdict(report, old, baselinePath)
	for _, l := range lines {
		fmt.Println(l)
	}
	if len(lines) > 0 {
		return fail("%d finding(s) against %s (%d entries)", len(lines), baselinePath, len(old))
	}
	fmt.Printf("deadcode: OK, the report equals the %d-entry baseline\n", len(old))
	return 0
}

// install builds the analyzer for the host (GOOS and GOARCH unset) into dir
// and checks the binary's build info names the pinned module version.
func install(ctx context.Context, dir, version string) (string, error) {
	cmd := exec.CommandContext(ctx, "go", "install", analyzerPackage+"@"+version)
	cmd.Dir = dir // outside the module: go.mod and go.sum are not read or written
	cmd.Env = append(without(os.Environ(), "GOOS", "GOARCH", "GOBIN", "GOFLAGS"), "GOBIN="+dir)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	bin := filepath.Join(dir, "deadcode")
	if _, err := os.Stat(bin + ".exe"); err == nil {
		bin += ".exe"
	}
	info, err := buildinfo.ReadFile(bin)
	if err != nil {
		return "", err
	}
	if info.Main.Path != analyzerModule || info.Main.Version != version {
		return "", fmt.Errorf("installed binary is %s@%s", info.Main.Path, info.Main.Version)
	}
	return bin, nil
}

// analyze runs the analyzer for one configuration, keeps its raw JSON in
// dest and returns the parsed run.
func analyze(ctx context.Context, bin, root string, c config, patterns []string, dest string) (run, error) {
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cmd := exec.CommandContext(ctx, bin, append([]string{"-test", "-json"}, patterns...)...)
	cmd.Dir = root
	cmd.Env = append(without(os.Environ(), "GOOS", "GOARCH", "CGO_ENABLED", "GOFLAGS"),
		"GOOS="+c.GOOS, "GOARCH="+c.GOARCH, "CGO_ENABLED=0")
	cmd.Stdout, cmd.Stderr = f, os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("analyzer: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		return nil, err
	}
	return parseRun(data)
}

// packagePatterns is ./... plus each package directory under a top-level
// directory whose name begins with a dot, which ./... skips. Directories
// under testdata or vendor, or inside a nested module, are left out as the go
// command would leave them out.
func packagePatterns(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z", "--", ":(glob).*/**/*.go")
	cmd.Dir = root
	listed, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	dirs := map[string]bool{}
	for _, name := range strings.Split(string(listed), "\x00") {
		if name == "" {
			continue
		}
		dir := path.Dir(name)
		if skipDir(root, dir) {
			continue
		}
		dirs["./"+dir] = true
	}
	out := []string{"./..."}
	extra := make([]string, 0, len(dirs))
	for d := range dirs {
		extra = append(extra, d)
	}
	sort.Strings(extra)
	return append(out, extra...), nil
}

func skipDir(root, dir string) bool {
	for _, seg := range strings.Split(dir, "/") {
		if seg == "testdata" || seg == "vendor" || strings.HasPrefix(seg, "_") {
			return true
		}
	}
	for d := dir; d != "."; d = path.Dir(d) {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(d), "go.mod")); err == nil {
			return true
		}
	}
	return false
}

func moduleFiles(root string) ([]byte, error) {
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, err
	}
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return append(append(mod, 0), sum...), nil
}

func without(env []string, names ...string) []string {
	out := env[:0:0]
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		drop := false
		for _, n := range names {
			if name == n {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, kv)
		}
	}
	return out
}
