package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/sharedpath"
	"github.com/Beamfall/corvint/internal/upgradechanges"
)

const changesHelp = `Inspect an experimental offline upgrade checklist (UCL1142-V0).

Usage:
  corvint [--root SOURCE] changes --from BUILD_SHA --to BUILD_SHA
    [--refs-file FILE ...] [--issue-state-file FILE]

Reads only local Git merge evidence; never fetches or writes repository state.
Repeat --refs-file for explicit regular files (no globs or symlink following).
Closes/Fixes/Resolves evidence can suggest "workaround may be removable";
Refs alone is unknown. Each PR lists new_flags added by literal in non-test
cmd/corvint Go source, from the local diff against its first parent.
--issue-state-file takes an operator-supplied export, for example
  gh issue list --state all --limit 1000 --json number,state > issues.json
Corvint never queries GitHub; without the file github_state is NOT_OBSERVED.
JSON includes truncation and NOT_OBSERVED reopen/revert semantics and per-PR
new commands. Review every hint.
A range of more than 512 commits in FROM..TO refuses at once with
changes-range-too-large, naming limit and commits; check it hop by hop over
narrower build pairs. Reaching the 30-second total deadline refuses with
changes-timeout, naming limitMs and elapsedMs.
Refusals: changes-invalid-arguments, changes-build-unavailable,
changes-range-unavailable, changes-range-too-large, changes-timeout,
changes-git-unavailable, changes-refs-unavailable,
changes-issue-state-unavailable.
Contract: docs/specs/upgrade-checklist-v0.md (accepted, decisions 0518 and 0519; V1-1142, V1-1158).

Shared-path change policy (SPC-V0, V1-1591):
  corvint [--root SOURCE] changes check-shared --base REV [--head REV]
    [--config FILE] [--notes FILE]
  corvint [--root SOURCE] changes split-shared --base REV --branch NAME
    [--apply --expect-head SHA] [--config FILE]
check-shared is read-only. It names the commit, path and missing element
(marker, separate-commit, listing, run-citation, clean-merge) of every
finding; exit 1 when there are findings. Merge conflicts confined to the
configured generated files are exempt. Without --notes, listing is NOT_OBSERVED.
split-shared plans the mechanical split of mixed commits on the named,
checked-out local branch; with --apply it rewrites that branch only, keeping
the final tree, author, dates and message, moves it by compare-and-swap on
--expect-head and prints oldHead. It never pushes or fetches. It refuses a
dirty tree, detached HEAD, another branch, merges in range and a live session.
--config defaults to .corvint/shared-paths.json under the root.
Refusals: changes-shared-config-unavailable, changes-shared-notes-unavailable,
changes-shared-range-unavailable, changes-shared-dirty-tree,
changes-shared-detached-head, changes-shared-branch-mismatch,
changes-shared-merge-in-range, changes-shared-live-session,
changes-shared-head-mismatch, changes-shared-tree-mismatch,
changes-shared-shallow-merge-base (check-shared and behind in a shallow clone
whose merge base may lie past the shallow boundary, or with several merge
bases).
Contract: docs/specs/shared-path-change-policy-v0.md.

Shared-path commits a branch is missing (SPC-V0-009..014, V1-1710, V1-1723):
  corvint [--root SOURCE] changes behind --base REF [--head REV]
    [--config FILE] [--ticket ID --stale-hours N] [--format json|doctor-plugin]
Read-only; never fetches. Lists the commits of head..base that change a
configured shared path (commit, subject, committedAt, arrivedVia, arrivedAt,
sharedPaths, headTouches, conflictRisk) and a dry merge of head into base:
clean, generated-only or conflict with the conflicting paths. arrivedVia is
the first-parent commit of base that brought the commit onto base, and
arrivedAt its committer time. JSON records base, baseRef (the ref and when it
was last observed: its reflog or, for a remote-tracking ref, FETCH_HEAD),
head, mergeBase, configDigest and observedAt. Over 512 commits refuses at once
with changes-shared-range-too-large, naming limit and commits. With --ticket
and --stale-hours, a missing shared commit that reached base at least N hours
ago is a SHARED_BASE_BEHIND finding; --format doctor-plugin prints only the
findings as a corvint-tasks doctor --plugins array.
`

func changesInvocation(args []string) (string, []string, bool) {
	root := ""
	for len(args) > 0 {
		if args[0] == "--root" {
			if !rootPreambleValue(args, 1) {
				return "", nil, false
			}
			root = args[1]
			args = args[2:]
			continue
		}
		if strings.HasPrefix(args[0], "--root=") {
			root = strings.TrimPrefix(args[0], "--root=")
			args = args[1:]
			continue
		}
		break
	}
	if len(args) == 0 || args[0] != "changes" {
		return "", nil, false
	}
	return root, args[1:], true
}
func runChanges(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	refuse := func(code string) int {
		_ = json.NewEncoder(diagnostic).Encode(struct {
			Code string `json:"code"`
			OK   bool   `json:"ok"`
		}{Code: code})
		return 2
	}
	if len(args) > 0 && (args[0] == "check-shared" || args[0] == "split-shared") {
		return runSharedPath(ctx, root, args[0], args[1:], out, refuse)
	}
	if len(args) > 0 && args[0] == "behind" {
		return runChangesBehind(ctx, root, args[1:], out, diagnostic, refuse)
	}
	f := flag.NewFlagSet("changes", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	from, to, issueState := "", "", ""
	files := []string{}
	setOnce := func(dst *string) func(string) error {
		return func(v string) error {
			if *dst != "" || v == "" {
				return errChangesArgument{}
			}
			*dst = v
			return nil
		}
	}
	f.Func("from", "installed base build", setOnce(&from))
	f.Func("to", "installed target build", setOnce(&to))
	f.Func("issue-state-file", "operator-supplied issue state export", setOnce(&issueState))
	f.Func("refs-file", "explicit adopter file", func(v string) error {
		if v == "" || len(files) >= 16 {
			return errChangesArgument{}
		}
		files = append(files, v)
		return nil
	})
	if f.Parse(args) != nil || f.NArg() != 0 || from == "" || to == "" {
		return refuse("changes-invalid-arguments")
	}
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return refuse("changes-git-unavailable")
		}
	}
	report, err := upgradechanges.Compile(ctx, upgradechanges.Request{Root: root, From: from, To: to, Files: files, IssueStateFile: issueState})
	var tooLarge *upgradechanges.RangeTooLargeError
	var timeout *upgradechanges.TimeoutError
	switch {
	case errors.As(err, &tooLarge):
		return refuseWith(diagnostic, map[string]any{"code": err.Error(), "ok": false, "limit": tooLarge.Limit, "commits": tooLarge.Commits})
	case errors.As(err, &timeout):
		return refuseWith(diagnostic, map[string]any{"code": err.Error(), "ok": false, "limitMs": timeout.LimitMS, "elapsedMs": timeout.ElapsedMS})
	case err != nil:
		return refuse(err.Error())
	}
	if json.NewEncoder(out).Encode(report) != nil {
		return refuse("changes-output-unavailable")
	}
	return 0
}

// refuseWith writes a coded refusal that also names its bound (UCL1142-V0-008) and exits 2.
func refuseWith(diagnostic io.Writer, refusal map[string]any) int {
	_ = json.NewEncoder(diagnostic).Encode(refusal)
	return 2
}

// runSharedPath runs check-shared (exit 0 clean, 1 findings) or split-shared
// (exit 0); every refusal exits 2 (SPC-V0-002, SPC-V0-004).
func runSharedPath(ctx context.Context, root, command string, args []string, out io.Writer, refuse func(string) int) int {
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	base, head, config, notes, branch, expect := "", "", "", "", "", ""
	setOnce := func(dst *string) func(string) error {
		return func(v string) error {
			if *dst != "" || v == "" {
				return errChangesArgument{}
			}
			*dst = v
			return nil
		}
	}
	f.Func("base", "base revision", setOnce(&base))
	f.Func("config", "shared-path configuration", setOnce(&config))
	apply := false
	if command == "check-shared" {
		f.Func("head", "head revision", setOnce(&head))
		f.Func("notes", "notes file that lists shared-path commits", setOnce(&notes))
	} else {
		f.Func("branch", "local branch to rewrite", setOnce(&branch))
		f.Func("expect-head", "expected branch head", setOnce(&expect))
		f.BoolVar(&apply, "apply", false, "rewrite the branch")
	}
	if f.Parse(args) != nil || f.NArg() != 0 || base == "" || (command == "split-shared" && (branch == "" || apply != (expect != ""))) {
		return refuse(sharedpath.CodeInvalidArguments)
	}
	if root == "" {
		var err error
		if root, err = os.Getwd(); err != nil {
			return refuse(sharedpath.CodeGitUnavailable)
		}
	}
	under := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(root, p)
	}
	if config == "" {
		config = sharedpath.DefaultConfig
	}
	c, err := sharedpath.LoadConfig(under(config))
	if err != nil {
		return refuse(err.Error())
	}
	var result any
	code := 0
	if command == "check-shared" {
		report, err := sharedpath.Check(ctx, sharedpath.CheckRequest{Root: root, Base: base, Head: head, NotesFile: under(notes), Config: c})
		if err != nil {
			return refuse(err.Error())
		}
		if !report.OK {
			code = 1
		}
		result = report
	} else {
		split, err := sharedpath.SplitShared(ctx, sharedpath.SplitRequest{Root: root, Base: base, Branch: branch, ExpectHead: expect, Apply: apply, Config: c})
		if err != nil {
			return refuse(err.Error())
		}
		result = split
	}
	if json.NewEncoder(out).Encode(result) != nil {
		return refuse("changes-output-unavailable")
	}
	return code
}

type errChangesArgument struct{}

func (errChangesArgument) Error() string { return "changes-invalid-arguments" }
