package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// affectedPathDirectoryForms names the accepted `--path` forms in a directory refusal (AFP-V0-231).
const affectedPathDirectoryForms = "pass a file, or a directory tracked at HEAD as `dir` or `dir/`, as a canonical repository-relative path"

// trimAffectedPathSlash drops one trailing slash from a non-root `--path` value (AFP-V0-231).
func trimAffectedPathSlash(value string) string {
	if len(value) > 1 && strings.HasSuffix(value, "/") && !strings.HasSuffix(value, "//") {
		return strings.TrimSuffix(value, "/")
	}
	return value
}

// affectedTreeEntries lists `git ls-tree` entries (type, path) for one literal path at HEAD.
func affectedTreeEntries(root string, recursive bool, path string) ([][2]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	arguments := []string{"--no-optional-locks", "--literal-pathspecs", "-C", root, "ls-tree", "-z", "--full-tree"}
	if recursive {
		arguments = append(arguments, "-r")
	}
	command := exec.CommandContext(ctx, gitstatus.Executable(), append(arguments, "HEAD", "--", path)...)
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C"}
	var stdout bytes.Buffer
	command.Stdout = &stdout
	if err := command.Run(); err != nil {
		return nil, err
	}
	var entries [][2]string
	for _, record := range strings.Split(stdout.String(), "\x00") {
		if record == "" {
			continue
		}
		meta, name, found := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !found || len(fields) != 3 {
			return nil, fmt.Errorf("unreadable ls-tree record")
		}
		entries = append(entries, [2]string{fields[1], name})
	}
	return entries, nil
}

// affectedUntrackedUnder lists untracked, non-ignored files under one directory (AFP-V0-231).
func affectedUntrackedUnder(root, directory string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, gitstatus.Executable(), "--no-optional-locks", "--literal-pathspecs", "-C", root, "ls-files", "-z", "--others", "--exclude-standard", "--", directory+"/")
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C"}
	var stdout bytes.Buffer
	command.Stdout = &stdout
	if err := command.Run(); err != nil {
		return nil, err
	}
	var names []string
	for _, name := range strings.Split(stdout.String(), "\x00") {
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

// expandAffectedPathDirectories replaces each `--path` that names a directory tracked at HEAD with
// its tracked files. A directory that is untracked at HEAD, or one holding a submodule, is refused
// rather than dropped or guessed (AFP-V0-231).
func expandAffectedPathDirectories(invocation *affectedInvocation) error {
	if len(invocation.PathList) == 0 {
		return nil
	}
	var expanded []string
	for _, path := range invocation.PathList {
		entries, err := affectedTreeEntries(invocation.Root, false, path)
		if err == nil && len(entries) == 1 && entries[0][0] == "tree" {
			files, listErr := affectedTreeEntries(invocation.Root, true, path+"/")
			if listErr != nil || len(files) == 0 {
				return argumentError("--path " + pythonRepr(path) + " is a directory whose tracked files could not be listed; " + affectedPathDirectoryForms)
			}
			untracked, untrackedErr := affectedUntrackedUnder(invocation.Root, path)
			if untrackedErr != nil {
				return argumentError("--path " + pythonRepr(path) + " is a directory whose untracked files could not be checked; " + affectedPathDirectoryForms)
			}
			if len(untracked) != 0 {
				shown := untracked
				if len(shown) > 5 {
					shown = shown[:5]
				}
				return argumentError(fmt.Sprintf("--path %s holds %d untracked file(s) the HEAD tree does not list (%s); commit or remove them, since a --path plan is made at a clean HEAD", pythonRepr(path), len(untracked), strings.Join(shown, ", ")))
			}
			for _, file := range files {
				if file[0] != "blob" || !affected.ValidRelativePath(file[1]) {
					return argumentError("--path " + pythonRepr(path) + " holds a non-file or non-canonical entry " + pythonRepr(file[1]) + "; pass the files explicitly")
				}
				expanded = append(expanded, file[1])
			}
			continue
		}
		if err == nil && len(entries) == 1 && entries[0][0] == "commit" {
			return argumentError("--path " + pythonRepr(path) + " is a submodule; " + affectedPathDirectoryForms)
		}
		if info, statErr := os.Lstat(filepath.Join(invocation.Root, filepath.FromSlash(path))); statErr == nil && info.IsDir() {
			return argumentError("--path " + pythonRepr(path) + " is a directory that is not tracked at HEAD; " + affectedPathDirectoryForms)
		}
		expanded = append(expanded, path)
	}
	invocation.PathList = affected.NormalizePaths(expanded)
	if len(invocation.PathList) > affectedMaxPathList {
		return argumentError(fmt.Sprintf("--path accepts at most %d paths after directory expansion; narrow the directory", affectedMaxPathList))
	}
	return nil
}
