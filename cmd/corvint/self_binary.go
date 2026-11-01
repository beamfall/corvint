package main

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime/debug"
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
	paths, count := diffPaths(output.Bytes(), output.Len() == selfBinaryDiffBytes)
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

// sharedOutputStream reports whether stdout and stderr are the same pipe or
// regular file, as after 2>&1, so a line written to stderr would land inside
// the stdout document (AFP-V0-871). A terminal is never shared here: a person
// reading both streams parses nothing.
func sharedOutputStream(stdout, stderr io.Writer) bool {
	out, ok := stdout.(*os.File)
	if !ok {
		return false
	}
	errFile, ok := stderr.(*os.File)
	if !ok {
		return false
	}
	outInfo, err := out.Stat()
	if err != nil || outInfo.Mode()&os.ModeCharDevice != 0 {
		return false
	}
	errInfo, err := errFile.Stat()
	return err == nil && os.SameFile(outInfo, errInfo)
}
