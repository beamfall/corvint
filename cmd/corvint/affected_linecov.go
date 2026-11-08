package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gateselect"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// affectedLineIndexProfileCode refuses --line-index outside the one form it
// narrows (GLS-V0-010).
const affectedLineIndexProfileCode = "unsupported-affected-line-index-profile"

// affectedLineQualify admits a NARROWED selection (GLS-V0-034); a test may
// replace it to stand for a qualified index.
var affectedLineQualify = linecov.CheckQualification

// affectedLineIndexProfileRefusal admits --line-index only with --base on the
// default JSON plan: the selection narrows a committed range's Go units and
// has no line, summary, by-path, snapshot, provider or Playwright form.
func affectedLineIndexProfileRefusal(invocation affectedInvocation) error {
	if invocation.LineIndex == "" {
		return nil
	}
	var with string
	switch {
	case invocation.Full:
		with = "--full"
	case invocation.Summary:
		with = "--summary"
	case invocation.Format == "text":
		with = "--format text"
	case invocation.ByPath:
		with = "--by-path"
	case len(invocation.PathList) != 0 || invocation.PathsFrom != "":
		with = "--path or --paths-from"
	case invocation.Snapshot != "":
		with = "--snapshot"
	case len(invocation.Providers) != 0:
		with = "--provider"
	case invocation.PlaywrightConfig != "":
		with = "--playwright-config"
	case len(invocation.AppmapImpacts) != 0:
		with = "--appmap-impact"
	case invocation.Base == "":
		with = "no --base"
	default:
		return nil
	}
	return &gokernel.Error{Code: affectedLineIndexProfileCode, Message: "--line-index narrows only the default JSON plan of a --base range, not with " + with + "; run corvint affected --base FULL_COMMIT_ID --line-index FILE"}
}

// resolveAffectedLineTimeout parses --line-test-timeout, which needs
// --line-index, into the line selection's test timeout, the go test default
// without it (GLS-V0-020).
func resolveAffectedLineTimeout(invocation *affectedInvocation) error {
	invocation.lineTimeout = linecov.DefaultTestTimeout
	if invocation.LineExec != "" && invocation.LineIndex == "" {
		return argumentError("--line-exec requires --line-index")
	}
	if invocation.LineTestTimeout == "" {
		return nil
	}
	if invocation.LineIndex == "" {
		return argumentError("--line-test-timeout requires --line-index")
	}
	d, err := time.ParseDuration(invocation.LineTestTimeout)
	if err != nil || d < 0 {
		return argumentError("--line-test-timeout requires a go test -timeout duration such as 10m or 0")
	}
	invocation.lineTimeout = d
	return nil
}

// startAffectedLineRunner reads the runner identity of root's Go environment
// (env, nil for this process's) under the test runtime of the go test
// -timeout timeout and the -exec wrapper exec in the background and returns
// its result.
func startAffectedLineRunner(ctx context.Context, root string, env []string, timeout time.Duration, wrapper string) func() (string, error) {
	type result struct {
		id  string
		err error
	}
	done := make(chan result, 1)
	go func() {
		id, _, err := linecov.RunnerID(ctx, root, env, timeout, wrapper)
		done <- result{id, err}
	}()
	return func() (string, error) {
		r := <-done
		return r.id, r.err
	}
}

// affectedLineInputs is what one line selection reads besides Git.
type affectedLineInputs struct {
	root, index, base, revision string
	worktreeDirty               int
	graph                       *affected.Graph
	plan                        affected.Plan
	runner                      func() (string, error)
	// env is the environment the selected tests run under, whose values the
	// bindings of every excluded test must hold (GLS-V0-018); nil is this
	// process's.
	env []string
}

// affectedLineSelection compiles the GLS-V0 line selection of a --base plan.
// Every condition it cannot verify is a FALLBACK that decides no unit, so the
// static plan stands (GLS-V0-011).
func affectedLineSelection(ctx context.Context, in affectedLineInputs) linecov.Selection {
	began := time.Now()
	ix, err := linecov.Open(in.index)
	load := time.Since(began)
	if err != nil {
		reason := linecov.ReasonIndexFormat
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) || strings.Contains(err.Error(), "not a regular file") {
			reason = linecov.ReasonIndexUnreadable
		}
		return affectedLineDone(in.runner, linecov.Fallback(reason, err.Error()), load, 0)
	}
	defer ix.Close()
	identity := &linecov.SelectionIndex{Format: linecov.Format, Commit: ix.Commit, Runner: ix.Runner, Digest: ix.Digest()}
	fallback := func(reason, detail string, gitTime time.Duration) linecov.Selection {
		sel := linecov.Fallback(reason, detail)
		sel.Index, sel.Base, sel.Head = identity, in.base, in.revision
		return affectedLineDone(in.runner, sel, load, gitTime)
	}
	switch {
	case in.worktreeDirty != 0:
		return fallback(linecov.ReasonWorktreeDirty, "the worktree status lists changed paths; the line selection plans committed ranges only", 0)
	case !affectedLineBounded(in.plan):
		return fallback(linecov.ReasonStaticScopeUnknown, "plan.unknown holds an entry outside the AFP-V0-982 allowlist", 0)
	}
	module, err := linecov.ModulePath(in.root)
	if err != nil || module != ix.Module {
		detail := "index module " + ix.Module + ", go.mod module " + module
		if err != nil {
			detail = err.Error()
		}
		return fallback(linecov.ReasonModuleMismatch, detail, 0)
	}
	runner, err := in.runner()
	in.runner = nil
	switch {
	case err != nil:
		return fallback(linecov.ReasonRunnerUnavailable, err.Error(), 0)
	case runner != ix.Runner:
		return fallback(linecov.ReasonRunnerMismatch, "index runner "+ix.Runner+", current "+runner, 0)
	}
	gitBegan := time.Now()
	ancestor := exec.CommandContext(ctx, "git", "-C", in.root, "merge-base", "--is-ancestor", ix.Commit, in.base)
	if err := ancestor.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return fallback(linecov.ReasonNotAncestor, "index commit "+ix.Commit+" is not an ancestor of the base "+in.base, time.Since(gitBegan))
		}
		return fallback(linecov.ReasonNotAncestor, "index commit "+ix.Commit+" is not an ancestor of the base "+in.base+" (or is not in this repository): "+err.Error(), time.Since(gitBegan))
	}
	drift, err := linecov.Diff(ctx, in.root, ix.Commit, in.base)
	if err != nil {
		return fallback(linecov.ReasonGitUnavailable, err.Error(), time.Since(gitBegan))
	}
	change, err := linecov.Diff(ctx, in.root, in.base, in.revision)
	if err != nil {
		return fallback(linecov.ReasonGitUnavailable, err.Error(), time.Since(gitBegan))
	}
	readDrift, err := linecov.BlobReader(ctx, in.root, drift)
	if err != nil {
		return fallback(linecov.ReasonGitUnavailable, err.Error(), time.Since(gitBegan))
	}
	readChange, err := linecov.BlobReader(ctx, in.root, change)
	if err != nil {
		return fallback(linecov.ReasonGitUnavailable, err.Error(), time.Since(gitBegan))
	}
	gitTime := time.Since(gitBegan)
	environ := in.env
	if environ == nil {
		environ = os.Environ()
	}
	readers := affectedLineReaders(in.graph, drift, change)
	var units []linecov.Unit
	for _, s := range in.plan.Selected {
		if dir, ok := linecov.UnitDir(module, s.UnitID); ok {
			units = append(units, linecov.Unit{ID: s.UnitID, Dir: dir, Witness: s.Witness.DirtyPath, StaticRead: readers(s.UnitID)})
		}
	}
	sel := ix.Plan(linecov.PlanInput{
		Base: in.base, Head: in.revision,
		Drift: drift, Change: change, ReadDrift: readDrift, ReadChange: readChange,
		Units: units,
		Heads: func(dirs []string) (map[string][]string, error) { return linecov.HeadTestNames(in.root, dirs) },
		StaticOf: func(paths []string) (bool, map[string]string, error) {
			return affectedLineStatic(in.graph, module, paths)
		},
		Environ: environ,
	})
	if sel.State == linecov.StateNarrowed {
		qualificationBegan := time.Now()
		info, _ := debug.ReadBuildInfo()
		if err := affectedLineQualify(ctx, in.root, in.base, runner, info); err != nil {
			sel.State, sel.Reason, sel.Detail = linecov.StateShadow, linecov.ReasonUnqualified, err.Error()
		}
		gitTime += time.Since(qualificationBegan)
	}
	return affectedLineDone(nil, sel, load, gitTime)
}

// affectedLineStatic is the static plan of the fallback paths on the plan's
// own graph: whether its scope is unknown, and each reached Go unit's
// directory with the dirty path that witnesses it (GLS-V0-013).
func affectedLineStatic(graph *affected.Graph, module string, paths []string) (bool, map[string]string, error) {
	plan := affected.Select(graph, paths)
	reach := map[string]string{}
	for _, s := range plan.Selected {
		if dir, ok := linecov.UnitDir(module, s.UnitID); ok {
			reach[dir] = s.Witness.DirtyPath
		}
	}
	return !affectedLineBounded(plan), reach, nil
}

// affectedLineBounded reports whether a plan bounds its Go units: every
// plan.unknown entry is one the AFP-V0-982 allowlist admits for a narrowed
// run (gateselect.AllowedUnknown), the rule CI and `corvint test` narrow by
// (GLS-V0-011).
func affectedLineBounded(plan affected.Plan) bool {
	for _, u := range plan.Unknown {
		if !gateselect.AllowedUnknown(u.Reason, u.Detail) {
			return false
		}
	}
	return true
}

// affectedLineDone waits out a runner read still in flight, so no goroutine
// outlives the plan, and records the load and Git times.
func affectedLineDone(runner func() (string, error), sel linecov.Selection, load, gitTime time.Duration) linecov.Selection {
	if runner != nil {
		_, _ = runner()
	}
	sel.LoadMillis = linecov.Millis(load)
	sel.GitMillis += linecov.Millis(gitTime)
	return sel
}

// affectedLineReaders reports the units the static plan's reader rules select
// for a Go path of the drift or the change (GLS-V0-013): a unit whose literal,
// glob, unbounded, declared or import-graph reads, embedded files or declared
// test ownership can observe the changed bytes stays whole as static-read,
// since coverage proves execution, not filesystem reads. The rules run on the
// Go paths alone, so a unit the plan reached only through an import or source
// witness is decided by its records. Without a graph every unit is a possible
// reader.
func affectedLineReaders(graph *affected.Graph, diffs ...[]linecov.FileDiff) func(id string) bool {
	if graph == nil {
		return func(string) bool { return true }
	}
	var paths []string
	for _, list := range diffs {
		for _, d := range list {
			if strings.HasSuffix(d.Path, ".go") {
				paths = append(paths, d.Path)
			}
		}
	}
	readers := graph.Readers(paths)
	return func(id string) bool {
		_, ok := readers[id]
		return ok
	}
}
