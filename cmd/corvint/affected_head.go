// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// affectedHeadCode is the error code of a --head plan that cannot be
// materialized from Git objects alone (AFP-V0-9989).
const affectedHeadCode = "unsupported-affected-head"

// affectedHeadReads names why every refused --head combination is refused.
const affectedHeadReads = "--head plans the commit pair from Git objects alone and reads no file, worktree or local record"

// affectedHeadOptionRefusal refuses --head without --base and with any option whose
// input lies outside Git objects (AFP-V0-9989). It runs before anything is
// read, so a refused call clones nothing.
func affectedHeadOptionRefusal(invocation affectedInvocation) error {
	if invocation.Head == "" {
		return nil
	}
	if invocation.Base == "" {
		return argumentError("--head requires --base: it plans the committed range BASE..HEAD of two commits; pass --base BASE --head HEAD")
	}
	for _, conflict := range []struct {
		set  bool
		name string
	}{
		{len(invocation.PathList) != 0, "--path"},
		{invocation.PathsFrom != "", "--paths-from"},
		{invocation.Worktree, "--worktree"},
		{invocation.Snapshot != "", "--snapshot"},
		{invocation.RequireClean, "--require-clean"},
		{invocation.FailFastOrder, "--fail-fast-order"},
		{invocation.ByPath, "--by-path"},
		{len(invocation.Providers) != 0, "--provider or --provider-command"},
		{len(invocation.Checkouts) != 0, "--repository"},
		{len(invocation.RepositoryBases) != 0, "--repository-base"},
		{len(invocation.AppmapImpacts) != 0, "--appmap-impact"},
		{invocation.PlaywrightConfig != "", "--playwright-config"},
		{invocation.PlaywrightDiscovery != "", "--playwright-discovery"},
		{len(invocation.PlaywrightEnv) != 0, "--playwright-env"},
		{invocation.LineIndex != "", "--line-index"},
		{invocation.LineExec != "", "--line-exec"},
		{invocation.LineTestTimeout != "", "--line-test-timeout"},
	} {
		if conflict.set {
			return argumentError("--head cannot be combined with " + conflict.name + ": " + affectedHeadReads + "; drop " + conflict.name + " or plan the checkout with --base alone")
		}
	}
	return nil
}

// affectedHeadError is the unsupported-affected-head refusal with its cause.
func affectedHeadError(head, cause string) error {
	return &gokernel.Error{Code: affectedHeadCode, Message: "--head " + head + " cannot be planned from Git objects alone: " + cause + "; no plan was written"}
}

// materializeAffectedHead checks head out, detached, in a private local clone
// of root's objects under the temporary directory, outside the repository, and
// returns that checkout and its removal (AFP-V0-9989). Only Git objects and
// refs are read from root: never its working tree, index or untracked files.
// A tree entry whose content is not a blob of this repository (a gitlink or a
// symlink) is refused rather than resolved on the host.
func materializeAffectedHead(ctx context.Context, root, head string) (string, func(), error) {
	git := gitstatus.Executable()
	listing, err := hermeticGitCommand(ctx, git, root, "ls-tree", "-r", "-z", "--full-tree", head).Output()
	if err != nil {
		return "", nil, affectedHeadError(head, "its tree could not be listed")
	}
	for entry := range bytes.SplitSeq(bytes.TrimSuffix(listing, []byte{0}), []byte{0}) {
		header, path, _ := strings.Cut(string(entry), "\t")
		switch mode, _, _ := strings.Cut(header, " "); mode {
		case "160000":
			return "", nil, affectedHeadError(head, "its tree holds the gitlink "+pythonRepr(path)+", whose content is not in this repository's objects")
		case "120000":
			return "", nil, affectedHeadError(head, "its tree holds the symlink "+pythonRepr(path)+", whose target is not a Git object")
		}
	}
	scratch, err := os.MkdirTemp("", "corvint-affected-head-")
	if err != nil {
		return "", nil, affectedHeadError(head, "no private scratch directory could be created")
	}
	cleanup := func() { _ = os.RemoveAll(scratch) }
	resolvedRoot, rootErr := filepath.EvalSymlinks(root)
	resolvedScratch, scratchErr := filepath.EvalSymlinks(scratch)
	relative, relErr := filepath.Rel(resolvedRoot, resolvedScratch)
	if rootErr != nil || scratchErr != nil || relErr != nil || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		cleanup()
		return "", nil, affectedHeadError(head, "the temporary directory lies inside the repository")
	}
	checkout := filepath.Join(scratch, "checkout")
	// A local copy of the objects, never --shared: the alternates denial of
	// the Git readers (CEM-V1-323) would refuse a borrowed object store and
	// widen the plan away from the --base plan of the same checkout.
	clone := hermeticGitCommand(ctx, git, root, "clone", "--quiet", "--no-checkout", "--", resolvedRoot, checkout)
	// The hermetic environment allows no transport; this clone needs exactly
	// the local one, and the later entry wins.
	clone.Env = append(clone.Env, "GIT_ALLOW_PROTOCOL=file")
	if err := clone.Run(); err != nil {
		cleanup()
		return "", nil, affectedHeadError(head, "the repository's objects could not be cloned into private scratch")
	}
	if err := hermeticGitCommand(ctx, git, checkout, "-c", "core.symlinks=false", "-c", "advice.detachedHead=false", "checkout", "--quiet", "--detach", head, "--").Run(); err != nil {
		cleanup()
		return "", nil, affectedHeadError(head, "it could not be checked out in private scratch")
	}
	return checkout, cleanup, nil
}
