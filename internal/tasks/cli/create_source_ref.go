package cli

import (
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Subject paths absent from the base (docs/specs/corvint-tasks-cli-v0.md,
// TCL-V0-743; V1-3696): a ticket whose declared touchPaths exist only on an
// unlanded branch sends a lane based on origin/main searching for code it
// cannot find. ticket create checks each touch path at the origin base and,
// when one is absent, requires the body to record where the code lives
// (a "Source ref: BRANCH@SHA" line) or --creates-paths, the declaration that
// the ticket creates those paths.

// sourceRefLine is the body line that records the subject's branch and
// commit; the commit is a full 40-hex SHA so it names immutable content.
var sourceRefLine = regexp.MustCompile(`(?mi)^source ref:[ \t]*([^\s@]+)@([0-9a-f]{40})[ \t]*$`)

// createSubjectGuard is TCL-V0-743: the warnings to add to the create
// result, or the MALFORMED refusal when a touch path is absent at the base
// and the body records no source ref and --creates-paths is not given.
func createSubjectGuard(env Env, cmd []string, flags mutateFlags, payload wire.Value) ([]string, *wire.Result) {
	paths := payloadTouchPaths(payload)
	if len(paths) == 0 || flags.createsPaths {
		return nil, nil
	}
	body := ""
	if v, ok := payload.Obj.Get("body"); ok && v.Kind == wire.KindString {
		body = v.Str
	}
	var (
		warnings, absent, notAtSource []string
		base, baseSha, branch, sha    string
		replay                        bool
	)
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		warnings, absent, notAtSource, base, baseSha, replay = nil, nil, nil, "", "", false
		if !rc.journalAbsent {
			idx := journal.RequestIndex{Reader: journal.Reader{Source: journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.PrimaryWorktree}, QueueID: rc.store.Queue.QueueID, PrimaryWorktree: rc.repo.PrimaryWorktree}}
			if _, found, err := idx.Lookup(flags.requestID); err == nil && found {
				replay = true
				return nil
			}
		}
		git := func(args ...string) (string, bool) { return remoteGit(rc.repo.CommonDir, args...) }
		for _, ref := range []string{"refs/remotes/origin/HEAD", "refs/remotes/origin/main"} {
			if commit, ok := git("rev-parse", "--verify", "-q", ref+"^{commit}"); ok && commit != "" {
				base, baseSha = strings.TrimPrefix(ref, "refs/remotes/"), commit
				break
			}
		}
		if rc.repo.CommonDir == "" || baseSha == "" {
			warnings = append(warnings, "touchPaths were not checked against the base: this repository has no origin/HEAD or origin/main remote-tracking ref, so subject presence is NOT_OBSERVED (TCL-V0-743)")
			return nil
		}
		absent = pathsAbsentAt(git, baseSha, paths)
		if len(absent) == 0 {
			return nil
		}
		m := sourceRefLine.FindStringSubmatch(body)
		if m == nil {
			return nil
		}
		branch, sha = m[1], m[2]
		if _, ok := git("cat-file", "-e", sha+"^{commit}"); !ok {
			warnings = append(warnings, "source ref "+branch+"@"+sha+" is not in this clone, so its paths "+strings.Join(absent, ", ")+" were not checked there (NOT_OBSERVED); the ticket records it (TCL-V0-743)")
			return nil
		}
		notAtSource = pathsAbsentAt(git, sha, absent)
		return nil
	})
	if wire.CodeOf(err) == wire.CodeRedoPending {
		return nil, nil
	}
	if err != nil {
		return nil, failure(cmd, rc, err)
	}
	if replay || len(absent) == 0 {
		return warnings, nil
	}
	if sha == "" {
		return nil, malformedUsage(cmd, "touchPaths "+strings.Join(absent, ", ")+" are absent from the base "+base+" at "+baseSha+", so a lane based on it cannot find the subject; nothing was written. "+
			"When the code lives on an unlanded branch, add the body line `Source ref: BRANCH@FULL_SHA` naming it (a lane then orders the ticket after that branch lands); when the ticket creates these paths, pass --creates-paths (TCL-V0-743)")
	}
	if len(notAtSource) > 0 {
		return nil, malformedUsage(cmd, "touchPaths "+strings.Join(notAtSource, ", ")+" are absent from both the base "+base+" at "+baseSha+" and the recorded source ref "+branch+"@"+sha+"; nothing was written. "+
			"Name the commit that holds them, or pass --creates-paths when the ticket creates them (TCL-V0-743)")
	}
	return append(warnings, "touchPaths "+strings.Join(absent, ", ")+" are absent from the base "+base+" at "+baseSha+" and present at the recorded source ref "+branch+"@"+sha+"; order the ticket after that branch lands (TCL-V0-743)"), nil
}

// payloadTouchPaths is the payload's effects.touchPaths.
func payloadTouchPaths(payload wire.Value) []string {
	if payload.Kind != wire.KindObject || payload.Obj == nil {
		return nil
	}
	effects, ok := payload.Obj.Get("effects")
	if !ok || effects.Kind != wire.KindObject || effects.Obj == nil {
		return nil
	}
	v, ok := effects.Obj.Get("touchPaths")
	if !ok || v.Kind != wire.KindArray {
		return nil
	}
	var paths []string
	for _, p := range v.Arr {
		if p.Kind == wire.KindString {
			paths = append(paths, p.Str)
		}
	}
	return paths
}

// pathsAbsentAt is each path with no tree entry at commit. A directory key
// ends in "/" and is present when anything lies under it. A git failure
// counts as present, so the guard never refuses on an observation it could
// not make.
func pathsAbsentAt(git func(...string) (string, bool), commit string, paths []string) []string {
	var absent []string
	for _, p := range paths {
		out, ok := git("ls-tree", "--name-only", commit, "--", strings.TrimSuffix(p, "/"))
		if ok && out == "" {
			absent = append(absent, p)
		}
	}
	return absent
}
