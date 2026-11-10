// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/linecov/watch"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

const watchHelp = `Rerun the tests a working-tree save can affect (experimental, LPCV-V0).

Usage:
  corvint [--root PATH] watch --index FILE [--once | --events-stdin] [--dry-run]
      [--packages a,b] [--go-wrapper PATH]

FILE is a Go line index collected at the last recorded commit (linecov
collect, linecov-index/4), resolved from the current directory. On each save
every staged, unstaged and untracked edit under the root is replanned against
that commit with the same analyzers and fallbacks as a committed range
(LPCV-V0-7607): a package keeps whole, narrows to the tests whose recorded
coverage a changed block reaches, compiles only, or is excluded. The plan,
each run's result and a terminal error stream as one JSON object per line on
stdout; every event is user-scoped (Scope "user", GateEligible false) and
never closes a CI gate (LPCV-V0-960, LPCV-V0-1085).

Save events are native and foreground: inotify on Linux, kqueue on macOS
(LPCV-V0-1086); no daemon, no polling. --events-stdin reads one notification
per line from stdin instead, for an editor hook or another system. --once
replans the current working tree once and exits. --dry-run streams plans
only and runs no test. An event source that stops ends the run with a
{"Kind":"error"} event naming the cause and exit 2, never with a stale plan
left as current (LPCV-V0-1087). Each plan event carries ReplanMillis, the
time from the save to the plan.

--packages a,b restricts the plan to these module-relative directories
(default: every package of the module). --go-wrapper PATH prefixes each go
test invocation with PATH (for example script/heavy.sh). A non-Go index is
refused: linecov watch -command runs those suites.

Exit status: 0 when the loop ends by signal or every --once run passed, 1
when a --once run failed, 2 for a refusal or an event source failure.
Writes: nothing under the repository; tests write what they write.

Example:
  corvint watch --index linecov.idx --once --dry-run
`

// watchInvocation recognises `[--root PATH] watch ARGS...`.
func watchInvocation(args []string) (string, []string, bool) {
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
	if len(args) == 0 || args[0] != "watch" {
		return "", nil, false
	}
	return root, args[1:], true
}

func watchRefuse(diagnostic io.Writer, code, message string) int {
	_ = json.NewEncoder(diagnostic).Encode(map[string]any{"code": code, "error": message, "ok": false})
	return 2
}

// runWatch is `corvint watch` (LPCV-V0-1085): the shared loop over a native,
// stdin or one-shot event source.
func runWatch(ctx context.Context, root string, args []string, stdin io.Reader, out, diagnostic io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		_, _ = io.WriteString(out, watchHelp)
		return 0
	}
	f := flag.NewFlagSet("watch", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	index := f.String("index", "", "")
	once := f.Bool("once", false, "")
	eventsStdin := f.Bool("events-stdin", false, "")
	dryRun := f.Bool("dry-run", false, "")
	packages := f.String("packages", "", "")
	wrapper := f.String("go-wrapper", "", "")
	if err := f.Parse(args); err != nil {
		return watchRefuse(diagnostic, "watch-invalid-arguments", err.Error()+"; usage: corvint [--root PATH] watch --index FILE [--once | --events-stdin] [--dry-run] [--packages a,b] [--go-wrapper PATH]")
	}
	if f.NArg() != 0 {
		return watchRefuse(diagnostic, "watch-invalid-arguments", fmt.Sprintf("unexpected argument %q; watch takes options only", f.Arg(0)))
	}
	if *index == "" {
		return watchRefuse(diagnostic, "watch-invalid-arguments", "--index FILE is required: the Go line index collected at the last recorded commit")
	}
	if *once && *eventsStdin {
		return watchRefuse(diagnostic, "watch-invalid-arguments", "--once and --events-stdin exclude each other")
	}
	if root == "" {
		root = "."
	}
	resolved, err := resolveExplicitRoot(root)
	if err != nil {
		emitError(diagnostic, err)
		return 2
	}
	ix, err := linecov.Open(*index)
	if err != nil {
		return watchRefuse(diagnostic, "watch-index-unavailable", fmt.Sprintf("index %s: %v", *index, err))
	}
	defer ix.Close()
	if ix.Language != "" && ix.Language != (golang.Language{}).Name() {
		return watchRefuse(diagnostic, "watch-index-language", fmt.Sprintf("index %s records %s tests; corvint watch plans Go indexes, linecov watch -command runs other suites", *index, ix.Language))
	}
	var source *watch.Source
	switch {
	case *once:
		source = watch.Once()
	case *eventsStdin:
		source = watch.Lines(ctx, stdin)
	default:
		if source, err = watch.Native(ctx, resolved); err != nil {
			return watchRefuse(diagnostic, "watch-events-unavailable", err.Error())
		}
	}
	var static []string
	if *packages != "" {
		static = strings.Split(*packages, ",")
	}
	failed := false
	o := watch.Options{Root: resolved, Index: ix, Static: static, Saves: source.C, Stopped: source.Err, Out: out}
	if !*dryRun {
		o.Run = func(ctx context.Context, p watch.Plan) error {
			err := runWatchPlan(ctx, resolved, ix, p, *wrapper, diagnostic)
			if err != nil {
				failed = true
			}
			return err
		}
	}
	if err := watch.Run(ctx, o); err != nil {
		code := "watch-failed"
		if errors.Is(err, watch.ErrSourceStopped) {
			code = "watch-events-stopped"
		}
		return watchRefuse(diagnostic, code, err.Error())
	}
	if failed {
		return 1
	}
	return 0
}

// runWatchPlan runs each planned invocation under the recorded test runtime
// (GLS-V0-020), through wrapper when named, with native output on the
// diagnostic stream so stdout stays one event per line.
func runWatchPlan(ctx context.Context, root string, ix *linecov.Index, p watch.Plan, wrapper string, diagnostic io.Writer) error {
	var lastErr error
	for _, inv := range p.Run {
		a := watch.TestArgs(ix, inv)
		if wrapper != "" {
			if !filepath.IsAbs(wrapper) && strings.Contains(wrapper, string(filepath.Separator)) {
				wrapper = filepath.Join(root, wrapper)
			}
			a = append([]string{wrapper}, a...)
		}
		cmd := exec.CommandContext(ctx, a[0], a[1:]...)
		cmd.Dir = root
		cmd.Stdout, cmd.Stderr = diagnostic, diagnostic
		cmd.Env = append(os.Environ(), "HEAVY_WHOLE_REASON=corvint watch working-tree plan selects whole package")
		// A cancelled watch kills the wrapper, go test and the test binary
		// together, and a child that keeps the output pipe cannot hold the
		// watch open.
		if err := linecov.RunIsolated(cmd); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
