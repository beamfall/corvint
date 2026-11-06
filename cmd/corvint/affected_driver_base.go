package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// affectedDriverContent reads both revisions of each dirty driver input, so
// the driver claimant can seed only the targets or jobs a hunk touches
// (AZU-V0-001): the base blob at the measured commit, in one bounded
// `git cat-file --batch`, and the head file from the planned tree. A path it
// cannot read either side of gets no revision, and the claimant then seeds
// every target or job of that file.
func affectedDriverContent(ctx context.Context, gitExecutable, root, tree, base string, dirty []string) map[string]affected.DriverRevision {
	paths := []string{}
	for _, relative := range dirty {
		if (affected.DriverInput(relative) || relative == affected.ShellForeignCallersPath) && !strings.ContainsAny(relative, "\r\n") && len(paths) < playwrightBaseMaxPaths {
			paths = append(paths, relative)
		}
	}
	if len(paths) == 0 || base == "" {
		return nil
	}
	deadline, cancel := context.WithTimeout(ctx, affectedRevisionDeadline)
	defer cancel()
	var request strings.Builder
	for _, relative := range paths {
		request.WriteString(base + ":" + relative + "\n")
	}
	command := hermeticGitCommand(deadline, gitExecutable, root, "cat-file", "--batch")
	command.Stdin = strings.NewReader(request.String())
	stdout, err := command.StdoutPipe()
	if err != nil || command.Start() != nil {
		return nil
	}
	parsed := map[string]playwrightBaseBlob{}
	output := bufio.NewReader(stdout)
	for _, relative := range paths {
		entry, keep, err := readPlaywrightBaseBlob(output)
		if err != nil {
			break
		}
		if keep {
			parsed[relative] = entry
		}
	}
	_, _ = io.Copy(io.Discard, stdout)
	if command.Wait() != nil {
		return nil
	}
	revisions := map[string]affected.DriverRevision{}
	for relative, entry := range parsed {
		head, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(relative)))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			revisions[relative] = affected.DriverRevision{Base: entry.body, BaseExists: entry.exists}
		case err == nil && len(head) <= playwrightBaseMaxBytes:
			revisions[relative] = affected.DriverRevision{Base: entry.body, Head: head, BaseExists: entry.exists, HeadExists: true}
		}
	}
	return revisions
}
