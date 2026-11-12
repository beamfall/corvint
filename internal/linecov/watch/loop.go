// SPDX-License-Identifier: Apache-2.0

// Package watch is the save-driven replan loop that `corvint watch` and
// `linecov watch` share (LPCV-V0-7607, LPCV-V0-1085). A save notification
// replans every current working-tree change against the immutable index
// commit with the same analyzers and fallbacks as a committed range, streams
// the plan and the run result, and reports an event source that stops as an
// explicit error instead of a silent stale answer (LPCV-V0-1087).
package watch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/testid"
)

// Plan is one per-package go test plan: the committed-range plan of
// `linecov select` and the working-tree plan of a save.
type Plan struct {
	Index, Base, Head string
	Reason            string `json:",omitempty"`
	Packages          map[string]linecov.PackagePlan
	Run               []Invocation
	Excluded          []string
	ElapsedMS         int64
}

// Invocation is one `go test` package argument and its -run filter.
// TestIDs names what it runs by canonical test ID (CTI-V1-009): the
// package-level ID of a whole package, one ID per kept top-level test of a
// narrowed one, and none for a compile-only run, which runs no test. The
// package directory is relative to the watch root, the repository root of a
// module at the root.
type Invocation struct {
	Package string
	Run     string   `json:",omitempty"`
	TestIDs []string `json:",omitempty"`
}

// Event is a local observation, never a CI receipt or a proof. Each result
// is streamed before the next save is consumed (LPCV-V0-960). Kind is
// "plan", "result" or "error"; an error event is terminal and names why no
// further save will be replanned (LPCV-V0-1087).
type Event struct {
	Scope        string
	GateEligible bool
	Kind         string
	Commit       string
	Plan         *Plan  `json:",omitempty"`
	LanguagePlan any    `json:",omitempty"`
	Package      string `json:",omitempty"`
	ExitCode     int    `json:",omitempty"`
	// ReplanMillis is the time from the save notification to this plan
	// event: the in-process save-to-answer latency (PRB-V0-1014).
	ReplanMillis float64 `json:",omitempty"`
	Message      string  `json:",omitempty"`
	// Inventory is what Options.Inventory read for this plan: the committed
	// test inventory judged against the working tree (TIV-V1-002). It is
	// read on every save and never refreshed here.
	Inventory any `json:",omitempty"`
}

// Options configures one loop.
type Options struct {
	// Root is the absolute repository root holding the working-tree edits.
	Root  string
	Index *linecov.Index
	// Static, when set, restricts the planned packages to these
	// module-relative directories; nil plans every package of the module.
	Static []string
	// Saves delivers one value per save notification. When it closes the
	// loop ends: with nil when Stopped is nil or reports nil, otherwise with
	// an error event and that error.
	Saves <-chan struct{}
	// Stopped reports why Saves closed, nil for a deliberate stop.
	Stopped func() error
	Out     io.Writer
	// Run executes a plan; nil emits plans only and no result events.
	Run func(context.Context, Plan) error
	// NonGo plans the working tree of an index whose Language is not Go;
	// nil refuses such an index before the first save.
	NonGo func(ctx context.Context, diffs []linecov.FileDiff) (any, error)
	// Inventory, when set, reads the state each plan event reports beside
	// the plan; it must only read.
	Inventory func(ctx context.Context) any
}

// ErrSourceStopped wraps the event source failure the loop ends with.
var ErrSourceStopped = errors.New("save notifications stopped")

// Run serializes saves; one notification always replans all current
// working-tree changes against the immutable index commit. Test failures are
// observations, so the next save can repair them. It returns nil when ctx
// ends or Saves closes deliberately, and the wrapped source error after an
// error event when the event source failed.
func Run(ctx context.Context, o Options) error {
	ix := o.Index
	if ix == nil || o.Out == nil || o.Root == "" {
		return errors.New("watch: root, index and output are required")
	}
	nonGo := ix.Language != "" && ix.Language != "go"
	if nonGo && o.NonGo == nil {
		return fmt.Errorf("watch: index language %q needs a non-Go plan; use linecov watch -command", ix.Language)
	}
	enc := json.NewEncoder(o.Out)
	emit := func(e Event) error {
		e.Scope = "user"
		e.GateEligible = false
		e.Commit = ix.Commit
		return enc.Encode(e)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-o.Saves:
			if !ok {
				if o.Stopped == nil {
					return nil
				}
				err := o.Stopped()
				if err == nil {
					return nil
				}
				err = fmt.Errorf("%w: %v; no further save is replanned and the last plan is stale: fix the cause and start watch again", ErrSourceStopped, err)
				if e := emit(Event{Kind: "error", Message: err.Error()}); e != nil {
					return e
				}
				return err
			}
		}
		start := time.Now()
		diffs, err := linecov.WorkingDiff(ctx, o.Root, ix.Commit)
		if err != nil {
			return replanErr(ctx, err)
		}
		p := Plan{Index: ix.Commit, Base: ix.Commit, Head: "working-tree"}
		var lp any
		if nonGo {
			// Non-Go candidates use the same analyzers. Effective execution remains
			// the whole caller-supplied suite while external qualification is pending.
			if lp, err = o.NonGo(ctx, diffs); err != nil {
				return replanErr(ctx, err)
			}
		} else if err = goPlan(ctx, o, diffs, &p); err != nil {
			return replanErr(ctx, err)
		}
		p.ElapsedMS = time.Since(start).Milliseconds()
		var inventory any
		if o.Inventory != nil {
			inventory = o.Inventory(ctx)
		}
		if err := emit(Event{Kind: "plan", Plan: &p, LanguagePlan: lp, ReplanMillis: linecov.Millis(time.Since(start)), Inventory: inventory}); err != nil {
			return err
		}
		if o.Run == nil {
			continue
		}
		err = o.Run(ctx, p)
		code := 0
		if err != nil {
			code = 1
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				code = ee.ExitCode()
			}
		}
		if err := emit(Event{Kind: "result", ExitCode: code}); err != nil {
			return err
		}
	}
}

// replanErr is a replan failure's result: nil once ctx is cancelled, because
// a watch stopped while it replans ends as one stopped while it waits does.
func replanErr(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return nil
	}
	return err
}

// goPlan fills p for a Go index: every static package keeps whole, narrows to
// the tests the changed blocks reach, compiles only, or is excluded, under
// the index's fallbacks. A changed runner identity keeps every package whole.
func goPlan(ctx context.Context, o Options, diffs []linecov.FileDiff, p *Plan) error {
	ix := o.Index
	dirs := o.Static
	var err error
	if dirs == nil {
		_, dirs, err = Packages(ctx, o.Root)
		if err != nil {
			return err
		}
	}
	heads, err := linecov.HeadTests(o.Root, dirs)
	if err != nil {
		return err
	}
	effect := ix.Analyze(diffs, linecov.Reader(ctx, o.Root, ix.Commit))
	id, e := RunnerLike(ctx, o.Root, ix.Runner, "")
	if e != nil || id != ix.Runner {
		effect.Fallback = "runner identity changed"
	}
	p.Packages, p.Reason = ix.Narrow(os.Environ(), dirs, heads, linecov.Effect{}, effect)
	for _, d := range dirs {
		pp := p.Packages[d]
		switch pp.Mode {
		case linecov.Whole:
			p.Run = append(p.Run, Invocation{Package: "./" + d, TestIDs: testIDs(d, nil)})
		case linecov.Tests:
			p.Run = append(p.Run, Invocation{Package: "./" + d, Run: "^(" + strings.Join(pp.Tests, "|") + ")$", TestIDs: testIDs(d, pp.Tests)})
		case linecov.Compile:
			p.Run = append(p.Run, Invocation{Package: "./" + d, Run: "^$"})
		}
	}
	return nil
}

// testIDs is the canonical test IDs of package directory dir: the
// package-level ID when tests is empty, else one ID per top-level test. A
// directory that is not a clean relative path names no ID.
func testIDs(dir string, tests []string) []string {
	pkg, err := testid.New(testid.Go, dir)
	if err != nil {
		return nil
	}
	if len(tests) == 0 {
		return []string{pkg.String()}
	}
	ids := make([]string, 0, len(tests))
	for _, test := range tests {
		ids = append(ids, pkg.Child(test).String())
	}
	return ids
}

// TestArgs is the go test argv of one planned invocation. It runs under the
// test runtime the index's runner identity records, -json for
// -test.v=test2json and that deadline, so a test branching on
// testing.Verbose or t.Deadline executes as it did during collection
// (GLS-V0-020). An identity naming no runtime already forces the whole plan.
func TestArgs(ix *linecov.Index, inv Invocation) []string {
	deadline, ok := linecov.RunnerTestTimeout(ix.Runner)
	if !ok {
		deadline = ix.TestTimeout
		if deadline <= 0 {
			deadline = linecov.DefaultTestTimeout
		}
	}
	a := []string{"go", "test", "-count=1", "-json", "-timeout=" + deadline.String()}
	if inv.Run != "" {
		a = append(a, "-run", inv.Run)
	}
	return append(a, inv.Package)
}

// RunnerLike is the current runner identity of root under the test runtime
// the identity want names, for a check of an index made under want that
// compares everything else.
func RunnerLike(ctx context.Context, root, want, wrapper string) (string, error) {
	timeout, ok := linecov.RunnerTestTimeout(want)
	if !ok {
		return "", fmt.Errorf("runner %q names no test runtime; recollect the index", want)
	}
	id, _, err := linecov.RunnerID(ctx, root, nil, timeout, wrapper)
	return id, err
}

// Packages lists the module path and the module-relative directories of the
// module's packages at root. Cancelling ctx stops the listing.
func Packages(ctx context.Context, root string) (module string, dirs []string, err error) {
	if module, err = goOutput(ctx, root, "list", "-m"); err != nil {
		return "", nil, err
	}
	out, err := goOutput(ctx, root, "list", "./...")
	if err != nil {
		return "", nil, err
	}
	for _, p := range strings.Fields(out) {
		switch {
		case p == module:
			dirs = append(dirs, ".")
		case strings.HasPrefix(p, module+"/"):
			dirs = append(dirs, strings.TrimPrefix(p, module+"/"))
		}
	}
	return module, dirs, nil
}

func goOutput(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("go %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
