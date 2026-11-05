// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bufio"
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
	"strings"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

// WatchEvent is a local observation, never a CI receipt or a proof. Each
// result is streamed before the next save is consumed (LPCV-V0-960).
type WatchEvent struct {
	Scope        string
	GateEligible bool
	Kind         string
	Commit       string
	Plan         *Plan         `json:",omitempty"`
	LanguagePlan *LanguagePlan `json:",omitempty"`
	Package      string        `json:",omitempty"`
	ExitCode     int           `json:",omitempty"`
}

func watch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	root := fs.String("root", ".", "repository containing working-tree edits")
	index := fs.String("index", "", "index at the last recorded commit (required)")
	static := fs.String("packages", "", "comma-separated static packages or test files")
	stdin := fs.Bool("events-stdin", false, "consume one save notification per line from editor stdin")
	once := fs.Bool("once", false, "run current working-tree plan once and exit")
	wrapper := fs.String("go-wrapper", "", "optional path to a wrapper receiving go test arguments")
	command := fs.String("command", "", "non-Go suite argv JSON; whole-suite fallback until qualified")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: linecov watch -root DIR -index INDEX [-once|-events-stdin] [-packages a,b] [-go-wrapper PATH]\nExample: linecov watch -root . -index /tmp/go.idx -go-wrapper ./script/heavy.sh\nRuns in the foreground, streaming user-scoped plans and native test output; no CI gate eligibility.")
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
	var notifications <-chan struct{}
	if *once {
		ch := make(chan struct{}, 1)
		ch <- struct{}{}
		close(ch)
		notifications = ch
	} else if *stdin {
		ch := make(chan struct{})
		notifications = ch
		go func() {
			defer close(ch)
			s := bufio.NewScanner(os.Stdin)
			for s.Scan() {
				select {
				case ch <- struct{}{}:
				case <-ctx.Done():
					return
				}
			}
		}()
	} else {
		notifications, err = saveNotifications(ctx, *root)
		if err != nil {
			return err
		}
	}
	var argv []string
	if *command != "" {
		if err = json.Unmarshal([]byte(*command), &argv); err != nil || len(argv) == 0 {
			return errors.New("watch: -command must be a nonempty argv array")
		}
	}
	return watchSaves(ctx, *root, ix, split(*static), notifications, os.Stdout, func(ctx context.Context, p Plan) error {
		if ix.Language != "" && ix.Language != "go" {
			if len(argv) == 0 {
				return errors.New("watch: -command required for a non-Go suite")
			}
			cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
			cmd.Dir = *root
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stdout
			return cmd.Run()
		}
		var lastErr error
		for _, inv := range p.Run {
			deadline := ix.TestTimeout
			if deadline <= 0 {
				deadline = 10 * time.Minute
			}
			a := []string{"go", "test", "-count=1", "-timeout=" + deadline.String()}
			if inv.Run != "" {
				a = append(a, "-run", inv.Run)
			}
			a = append(a, inv.Package)
			if *wrapper != "" {
				a = append([]string{*wrapper}, a...)
			}
			cmd := exec.CommandContext(ctx, a[0], a[1:]...)
			cmd.Dir = *root
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stdout
			cmd.Env = append(os.Environ(), "HEAVY_WHOLE_REASON=linecov working-tree plan selects whole package")
			if err := cmd.Run(); err != nil {
				lastErr = err
			}
		}
		return lastErr
	})
}

// watchSaves serializes saves; one notification always replans all current
// working-tree changes against the immutable index commit. Test failures are
// observations, so the next save can repair them.
func watchSaves(ctx context.Context, root string, ix *linecov.Index, static []string, saves <-chan struct{}, out io.Writer, run func(context.Context, Plan) error) error {
	enc := json.NewEncoder(out)
	emit := func(e WatchEvent) error {
		e.Scope = "user"
		e.GateEligible = false
		e.Commit = ix.Commit
		return enc.Encode(e)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-saves:
			if !ok {
				return nil
			}
		}
		diffs, err := linecov.WorkingDiff(ctx, root, ix.Commit)
		if err != nil {
			return err
		}
		p := Plan{Index: ix.Commit, Base: ix.Commit, Head: "working-tree"}
		var lp *LanguagePlan
		if ix.Language != "" && ix.Language != "go" {
			// Non-Go candidates use the same analyzers. Effective execution remains
			// the whole caller-supplied suite while external qualification is pending.
			v, err := languageWorkingPlan(ctx, root, ix, static, diffs)
			if err != nil {
				return err
			}
			lp = &v
		} else {
			dirs := static
			if dirs == nil {
				_, dirs, err = packages(root)
				if err != nil {
					return err
				}
			}
			heads, err := linecov.HeadTests(root, dirs)
			if err != nil {
				return err
			}
			effect := ix.Analyze(diffs, linecov.Reader(ctx, root, ix.Commit))
			id, _, e := runner(root)
			if e != nil || id != ix.Runner {
				effect.Fallback = "runner identity changed"
			}
			p.Packages, p.Reason = ix.Narrow(dirs, heads, linecov.Effect{}, effect)
			for _, d := range dirs {
				pp := p.Packages[d]
				switch pp.Mode {
				case linecov.Whole:
					p.Run = append(p.Run, Invocation{Package: "./" + d})
				case linecov.Tests:
					p.Run = append(p.Run, Invocation{Package: "./" + d, Run: "^(" + strings.Join(pp.Tests, "|") + ")$"})
				case linecov.Compile:
					p.Run = append(p.Run, Invocation{Package: "./" + d, Run: "^$"})
				}
			}
		}
		if err := emit(WatchEvent{Kind: "plan", Plan: &p, LanguagePlan: lp}); err != nil {
			return err
		}
		err = run(ctx, p)
		code := 0
		if err != nil {
			code = 1
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				code = ee.ExitCode()
			}
		}
		if err := emit(WatchEvent{Kind: "result", ExitCode: code}); err != nil {
			return err
		}
	}
}
