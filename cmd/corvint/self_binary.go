package main

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// selfBinaryStaleCode is the degradation a Corvint binary reports when it runs
// inside its own repository and the Go source it was built from differs from
// HEAD there, so its selections and evidence may not be the ones current
// source gives (V1-3216, AFP-V0-846).
const selfBinaryStaleCode = "self-binary-stale"

// selfBinaryUnverifiedCode is the degradation reported instead when that
// comparison could not complete: the stamped commit is absent here, Git
// failed or the deadline expired (AFP-V0-846).
const selfBinaryUnverifiedCode = "self-binary-unverified"

const (
	selfBinaryDiffDeadline = 2 * time.Second
	selfBinaryDiffBytes    = 64 * 1024
	selfBinaryPathsShown   = 3
	// selfBinaryGraphDirectories bounds the package directories the main
	// build-graph walk visits; past it every changed path counts (TSP-V0-722).
	selfBinaryGraphDirectories = 4096
)

// selfBuildInfo is this binary's build information, and
// readExecutableBuildInfo another executable's; tests replace them.
var (
	selfBuildInfo           = func() (*debug.BuildInfo, bool) { return debug.ReadBuildInfo() }
	readExecutableBuildInfo = buildinfo.ReadFile
)

// binaryFreshness is a stale or unverified binary's degradation code and
// detail; Code is "" for a fresh binary or one no stamp or repository lets
// Corvint judge.
type binaryFreshness struct {
	Code   string
	Detail string
}

// selfBinaryStale judges this binary against root (AFP-V0-846).
func selfBinaryStale(ctx context.Context, root string) binaryFreshness {
	info, ok := selfBuildInfo()
	if !ok {
		return binaryFreshness{}
	}
	return binaryStaleAgainst(ctx, root, info)
}

// executableStale is selfBinaryStale for the executable at path, read with
// debug/buildinfo without running it.
func executableStale(ctx context.Context, root, path string) binaryFreshness {
	info, err := readExecutableBuildInfo(path)
	if err != nil {
		return binaryFreshness{}
	}
	return binaryStaleAgainst(ctx, root, info)
}

// binaryStaleAgainst compares the Git commit stamped into info with HEAD at
// root: equal commits are fresh after one rev-parse; otherwise one bounded
// tree diff of the non-test Go source and module files decides. It judges
// nothing for a binary without a Git stamp (a -buildvcs=false or release
// build), or for a repository whose module is not the binary's own.
func binaryStaleAgainst(ctx context.Context, root string, info *debug.BuildInfo) binaryFreshness {
	commit := producerFrom(version, build, info).Commit
	if commit == producerUnknown || info.Main.Path == "" {
		return binaryFreshness{}
	}
	if module, err := golang.ReadModulePath(root); err != nil || module != info.Main.Path {
		return binaryFreshness{}
	}
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return binaryFreshness{}
	}
	deadline, cancel := context.WithTimeout(ctx, selfBinaryDiffDeadline)
	defer cancel()
	unverified := binaryFreshness{Code: selfBinaryUnverifiedCode, Detail: fmt.Sprintf("this binary was built from commit %s, which could not be compared with HEAD here (the commit is absent, Git failed or the %s deadline expired); if the commit is absent, rebuild it from this checkout (go install ./cmd/corvint)", shortRevision(commit), selfBinaryDiffDeadline)}
	head, err := hermeticGitCommand(deadline, gitExecutable, root, "rev-parse", "--verify", "--quiet", "HEAD^{commit}").Output()
	if err != nil {
		return unverified
	}
	if strings.TrimSpace(string(head)) == commit {
		return binaryFreshness{}
	}
	command := hermeticGitCommand(deadline, gitExecutable, root, "diff", "--name-only", "-z", "--no-renames", commit, "HEAD", "--",
		":(glob)**/*.go", ":(exclude,glob)**/*_test.go", ":(exclude,glob)**/testdata/**", "go.mod", "go.sum")
	var output bytes.Buffer
	command.Stdout = &limitedWriter{buffer: &output, limit: selfBinaryDiffBytes}
	if err := command.Run(); err != nil {
		return unverified
	}
	truncated := output.Len() == selfBinaryDiffBytes
	paths, count := diffPaths(output.Bytes(), truncated)
	if !truncated && len(paths) != 0 {
		if built, ok := mainBuildDirectories(deadline, root, info.Main.Path, info.Path); ok {
			paths = inMainBuild(paths, built)
			count = fmt.Sprint(len(paths))
		}
	}
	if len(paths) == 0 {
		return binaryFreshness{}
	}
	shown := paths[:min(len(paths), selfBinaryPathsShown)]
	more := ""
	if len(paths) > len(shown) {
		more = fmt.Sprintf(" (+%d more)", len(paths)-len(shown))
	}
	return binaryFreshness{Code: selfBinaryStaleCode, Detail: fmt.Sprintf("this binary was built from commit %s and %s Go source paths differ at HEAD: %s%s; rebuild it from this checkout (go install ./cmd/corvint)", shortRevision(commit), count, strings.Join(shown, ", "), more)}
}

// diffPaths splits NUL-separated diff output. When the output bound truncated
// it, the last path may be partial: it is dropped and the count reads
// "at least N".
func diffPaths(output []byte, truncated bool) ([]string, string) {
	paths := strings.FieldsFunc(string(output), func(r rune) bool { return r == 0 })
	if truncated && len(paths) > 0 {
		paths = paths[:len(paths)-1]
		return paths, fmt.Sprintf("at least %d", len(paths))
	}
	return paths, fmt.Sprint(len(paths))
}

func shortRevision(revision string) string { return revision[:min(len(revision), 12)] }

// limitedWriter keeps the first limit bytes and discards the rest, so an
// unexpectedly large diff cannot grow memory.
type limitedWriter struct {
	buffer *bytes.Buffer
	limit  int
}

func (writer *limitedWriter) Write(data []byte) (int, error) {
	if room := writer.limit - writer.buffer.Len(); room > 0 {
		writer.buffer.Write(data[:min(len(data), room)])
	}
	return len(data), nil
}

// mainBuildDirectories lists the repository-relative directories of the
// packages the main package mainPath reaches through imports inside module, as
// they stand on disk at root: every non-test .go file counts whatever its build
// constraints, so the set covers every platform's build (TSP-V0-722). It
// answers ok false, and the caller then counts every changed path, when the main
// package is not in module or has no Go file here, a go.work file or a go.mod
// replace directive may bring in other source, a file does not parse, the
// directory bound is exceeded or the deadline expires.
func mainBuildDirectories(ctx context.Context, root, module, mainPath string) (map[string]bool, bool) {
	start, ok := moduleDirectory(module, mainPath)
	if !ok {
		return nil, false
	}
	manifest, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || bytes.Contains(manifest, []byte("replace")) {
		return nil, false
	}
	if _, err := os.Lstat(filepath.Join(root, "go.work")); !os.IsNotExist(err) {
		return nil, false
	}
	seen := map[string]bool{start: true}
	queue := []string{start}
	fileSet := token.NewFileSet()
	for len(queue) != 0 {
		if ctx.Err() != nil || len(seen) > selfBinaryGraphDirectories {
			return nil, false
		}
		directory := queue[0]
		queue = queue[1:]
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(directory)))
		if err != nil {
			return nil, false
		}
		files := 0
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || entry.IsDir() {
				continue
			}
			files++
			file, err := parser.ParseFile(fileSet, filepath.Join(root, filepath.FromSlash(directory), name), nil, parser.ImportsOnly)
			if err != nil {
				return nil, false
			}
			for _, spec := range file.Imports {
				value, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					return nil, false
				}
				if dependency, inside := moduleDirectory(module, value); inside && !seen[dependency] {
					seen[dependency] = true
					queue = append(queue, dependency)
				}
			}
		}
		if files == 0 && directory == start {
			return nil, false
		}
	}
	return seen, true
}

// moduleDirectory is the repository-relative directory of importPath inside
// module, whose go.mod is at the root.
func moduleDirectory(module, importPath string) (string, bool) {
	if importPath == module {
		return ".", true
	}
	rest, inside := strings.CutPrefix(importPath, module+"/")
	return rest, inside && rest != ""
}

// inMainBuild keeps the changed paths the main build compiles: the module files
// and the Go files of a directory in built.
func inMainBuild(paths []string, built map[string]bool) []string {
	kept := paths[:0:0]
	for _, changed := range paths {
		if changed == "go.mod" || changed == "go.sum" || built[path.Dir(changed)] {
			kept = append(kept, changed)
		}
	}
	return kept
}
