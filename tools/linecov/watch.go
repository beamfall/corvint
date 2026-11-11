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
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/linecov/watch"
)

// WatchEvent is a local observation, never a CI receipt or a proof. Each
// result is streamed before the next save is consumed (LPCV-V0-960). The
// loop and its event sources are internal/linecov/watch, shared with
// `corvint watch` (LPCV-V0-1085).
type WatchEvent = watch.Event

func watchCommand(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	root := fs.String("root", ".", "repository containing working-tree edits")
	index := fs.String("index", "", "index at the last recorded commit (required)")
	static := fs.String("packages", "", "comma-separated static packages or test files")
	stdin := fs.Bool("events-stdin", false, "consume one save notification per line from editor stdin")
	once := fs.Bool("once", false, "run current working-tree plan once and exit")
	wrapper := fs.String("go-wrapper", "", "optional path to a wrapper receiving go test arguments")
	command := fs.String("command", "", "non-Go suite argv JSON; whole-suite fallback until qualified")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: linecov watch -root DIR -index INDEX [-once|-events-stdin] [-packages a,b] [-go-wrapper PATH]\nExample: linecov watch -root . -index /tmp/go.idx -go-wrapper ./script/heavy.sh\nRuns in the foreground, streaming user-scoped plans and native test output; no CI gate eligibility.\nNative save events: inotify on Linux, kqueue on macOS; elsewhere -events-stdin or -once.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *index == "" {
		return errors.New("watch: -index required")
	}
	f, err := os.Open(*index)
	if err != nil {
		return err
	}
	ix, err := linecov.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	*root = abs
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	var source *watch.Source
	switch {
	case *once:
		source = watch.Once()
	case *stdin:
		source = watch.Lines(ctx, os.Stdin)
	default:
		if source, err = watch.Native(ctx, *root); err != nil {
			return err
		}
	}
	var argv []string
	if *command != "" {
		if err = json.Unmarshal([]byte(*command), &argv); err != nil || len(argv) == 0 {
			return errors.New("watch: -command must be a nonempty argv array")
		}
	}
	o := watch.Options{Root: *root, Index: ix, Static: split(*static), Saves: source.C, Stopped: source.Err, Out: os.Stdout}
	o.NonGo = func(ctx context.Context, diffs []linecov.FileDiff) (any, error) {
		return languageWorkingPlan(ctx, *root, ix, split(*static), diffs)
	}
	o.Run = func(ctx context.Context, p Plan) error {
		if ix.Language != "" && ix.Language != "go" {
			if len(argv) == 0 {
				return errors.New("watch: -command required for a non-Go suite")
			}
			cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
			cmd.Dir = *root
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stdout
			return cmd.Run()
		}
		return runGoPlan(ctx, *root, ix, p, *wrapper)
	}
	return watch.Run(ctx, o)
}

// runGoPlan runs each planned invocation under the recorded test runtime,
// through wrapper when one is named, streaming native output; the last
// failure is the result.
func runGoPlan(ctx context.Context, root string, ix *linecov.Index, p Plan, wrapper string) error {
	var lastErr error
	for _, inv := range p.Run {
		a := watchTestArgs(ix, inv)
		if wrapper != "" {
			a = append([]string{wrapper}, a...)
		}
		cmd := exec.CommandContext(ctx, a[0], a[1:]...)
		cmd.Dir = root
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stdout
		cmd.Env = append(os.Environ(), "HEAVY_WHOLE_REASON=linecov working-tree plan selects whole package")
		if err := cmd.Run(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// watchTestArgs is the go test argv of one planned invocation (GLS-V0-020).
func watchTestArgs(ix *linecov.Index, inv Invocation) []string { return watch.TestArgs(ix, inv) }

// watchSaves runs the shared loop with a plain notification channel, for a
// Go index; it returns nil when the channel closes.
func watchSaves(ctx context.Context, root string, ix *linecov.Index, static []string, saves <-chan struct{}, out io.Writer, run func(context.Context, Plan) error) error {
	return watch.Run(ctx, watch.Options{Root: root, Index: ix, Static: static, Saves: saves, Out: out, Run: run})
}
