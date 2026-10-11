package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"strings"

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
Refusals: changes-invalid-arguments, changes-build-unavailable,
changes-range-unavailable, changes-git-unavailable, changes-refs-unavailable,
changes-issue-state-unavailable.
Contract: docs/specs/upgrade-checklist-v0.md (proposed, V1-1142, V1-1158).
`

func changesInvocation(args []string) (string, []string, bool) {
	root := ""
	for len(args) > 0 {
		if args[0] == "--root" && len(args) > 1 {
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
	if err != nil {
		return refuse(err.Error())
	}
	if json.NewEncoder(out).Encode(report) != nil {
		return refuse("changes-output-unavailable")
	}
	return 0
}

type errChangesArgument struct{}

func (errChangesArgument) Error() string { return "changes-invalid-arguments" }
