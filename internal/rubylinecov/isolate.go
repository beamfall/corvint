// SPDX-License-Identifier: Apache-2.0
package rubylinecov

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

// IncompleteError reports a collection that stopped launching isolated runs
// at its budget. Every finished run is retained under State, so the same
// command resumes it (LPCV-V0-7609).
type IncompleteError struct {
	State       string
	Done, Total int
}

func (e *IncompleteError) Error() string {
	return fmt.Sprintf("rubylinecov: incomplete: %d of %d isolated runs retained in %s; rerun the same command to resume", e.Done, e.Total, e.State)
}

// stateBinding names the one collection a state directory may hold.
type stateBinding struct {
	Profile         string   `json:"profile"`
	Collector       string   `json:"collectorSha256"`
	Root            string   `json:"root"`
	Commit          string   `json:"commit"`
	Argv            []string `json:"argv"`
	Timeout         int64    `json:"timeoutNanos"`
	IsolatedTimeout int64    `json:"isolatedTimeoutNanos"`
	// Env is the digest of each bound environment variable, "unset" for a
	// declared one that is not set (see boundEnvironment).
	Env map[string]string `json:"env"`
}

// boundNames and boundPrefixes are the variables that choose the Ruby
// interpreter and its gems: a state directory resumes only under the values
// its first chunk ran with.
var (
	boundNames    = []string{"PATH", "RUBYOPT", "RUBYLIB"}
	boundPrefixes = []string{"GEM_", "BUNDLE_", "RUBY", "RBENV_", "RVM_", "CHRUBY_", "ASDF_RUBY"}
)

// boundEnvironment digests the Ruby and gem variables of environ and every
// declared variable. Values are digested because a declared variable may
// hold a secret.
func boundEnvironment(environ, declared []string) map[string]string {
	bound := map[string]string{}
	for _, name := range append(append([]string{}, boundNames...), declared...) {
		bound[name] = "unset"
	}
	for _, kv := range environ {
		name, value, _ := strings.Cut(kv, "=")
		_, named := bound[name]
		for _, prefix := range boundPrefixes {
			named = named || strings.HasPrefix(name, prefix)
		}
		if named {
			bound[name] = fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(value)))
		}
	}
	return bound
}

// retainedResult is one finished run's terminal result beside its dumps.
type retainedResult struct {
	Exit  int    `json:"exit"`
	Error string `json:"error,omitempty"`
}

// Collect compares each ordinary-order result with exactly one isolated test
// process and unions both reaches. Missing, duplicate and mismatching outcomes
// preserve whole-file fallback (LPCV-V0-962, LPCV-V0-7605). Every run's dumps
// and terminal result are retained under Work (State when set, which a later
// call resumes from); up to Parallel isolated runs execute at once, and the
// merge reads them back in outcome order, so the index does not depend on the
// worker count or on interruptions (LPCV-V0-7609).
func Collect(ctx context.Context, o Options) (*Run, error) {
	if o.State != "" {
		state, err := filepath.Abs(o.State)
		if err != nil {
			return nil, err
		}
		if err := bindState(ctx, state, o); err != nil {
			return nil, err
		}
		o.Work = state
	}
	work, err := filepath.Abs(o.Work)
	if err != nil {
		return nil, err
	}
	o.Work = work
	run, err := retained(ctx, o.Work, o.State != "", "ordinary.json", func() (*Run, error) { return collectOnce(ctx, o) })
	if err != nil {
		return nil, err
	}
	// Every isolated run must report the ordinary-order run's runner: a
	// chunk resumed under another Ruby or framework is refused, and its run
	// is not retained.
	var (
		mu      sync.Mutex
		foreign error
	)
	isolatedRun := func(i int, out Outcome) (*Run, error) {
		one := o
		if o.IsolatedTimeout > 0 {
			one.Timeout = o.IsolatedTimeout
		}
		one.Work = filepath.Join(o.Work, fmt.Sprintf("isolated-%d", i))
		one.onlyName, one.onlyPackage = out.Name, out.Package
		isolated, err := retained(ctx, one.Work, true, "result.json", func() (*Run, error) { return collectOnce(ctx, one) })
		if err == nil && isolated != nil && isolated.Runner != run.Runner {
			_ = os.Remove(filepath.Join(one.Work, "result.json"))
			mu.Lock()
			defer mu.Unlock()
			if foreign == nil {
				foreign = fmt.Errorf("rubylinecov: isolated run %d of %s ran under runner %q, the ordinary-order run under %q; rerun under the collection's own Ruby and gems", i, out.Name, isolated.Runner, run.Runner)
			}
			return nil, foreign
		}
		return isolated, err
	}
	refused := func() error {
		mu.Lock()
		defer mu.Unlock()
		return foreign
	}
	if err := runIsolated(ctx, run.Outcomes, o, func(i int, out Outcome) error {
		_, err := isolatedRun(i, out)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// A failed isolated run is a retained divergent result; a run under
		// another runner stops the collection.
		_ = err
		return refused()
	}); err != nil {
		return nil, err
	}
	merged := isolateArtifacts(run, isolatedRun)
	if err := refused(); err != nil {
		return nil, err
	}
	return merged, nil
}

// runIsolated launches each non-pending outcome's isolated run, up to
// o.Parallel at once, and stops launching once o.Budget has elapsed.
// Retained results are read back without a launch.
func runIsolated(ctx context.Context, outcomes []Outcome, o Options, launch func(int, Outcome) error) error {
	workers := max(o.Parallel, 1)
	began := time.Now()
	var (
		mu       sync.Mutex
		firstErr error
		wg       sync.WaitGroup
	)
	sem := make(chan struct{}, workers)
	done, total := 0, 0
	for i, out := range outcomes {
		if out.Status == "pending" {
			continue
		}
		total++
		dir := filepath.Join(o.Work, fmt.Sprintf("isolated-%d", i))
		if _, err := os.Stat(filepath.Join(dir, "result.json")); err == nil {
			done++
			continue
		}
		if o.Budget > 0 && time.Since(began) >= o.Budget {
			continue
		}
		sem <- struct{}{}
		mu.Lock()
		stop := firstErr != nil
		mu.Unlock()
		if stop {
			<-sem
			break
		}
		wg.Add(1)
		go func(i int, out Outcome) {
			defer wg.Done()
			defer func() { <-sem }()
			err := launch(i, out)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			if err == nil {
				done++
			}
		}(i, out)
	}
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	if done < total {
		return &IncompleteError{State: o.Work, Done: done, Total: total}
	}
	return nil
}

// retained returns the run whose terminal result dir holds, or runs it and
// retains the result. A run ended by cancellation retains nothing.
func retained(ctx context.Context, dir string, reuse bool, name string, collect func() (*Run, error)) (*Run, error) {
	file := filepath.Join(dir, name)
	if reuse {
		if data, err := os.ReadFile(file); err == nil {
			var r retainedResult
			if err := json.Unmarshal(data, &r); err != nil {
				return nil, fmt.Errorf("rubylinecov: retained result %s: %w", file, err)
			}
			if r.Error != "" {
				return nil, errors.New(r.Error)
			}
			run, err := read(filepath.Join(dir, "dumps"))
			if err != nil {
				return nil, fmt.Errorf("rubylinecov: retained dumps %s: %w", dir, err)
			}
			run.exit = r.Exit
			return run, nil
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	run, err := collect()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	r := retainedResult{}
	if err != nil {
		r.Error = err.Error()
	} else {
		r.Exit = run.exit
	}
	data, merr := json.Marshal(r)
	if merr != nil {
		return nil, merr
	}
	tmp := file + ".tmp"
	if werr := os.WriteFile(tmp, data, 0o644); werr != nil {
		return nil, werr
	}
	if werr := os.Rename(tmp, file); werr != nil {
		return nil, werr
	}
	return run, err
}

// bindState creates or checks state's binding to this collection.
func bindState(ctx context.Context, state string, o Options) error {
	head, err := exec.CommandContext(ctx, "git", "-C", o.Root, "rev-parse", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("rubylinecov: git rev-parse: %w", err)
	}
	root, err := filepath.Abs(o.Root)
	if err != nil {
		return err
	}
	want := stateBinding{Profile: "rubylinecov-state/2", Collector: fmt.Sprintf("%x", sha256.Sum256(script)), Root: root, Commit: strings.TrimSpace(string(head)), Argv: o.Argv, Timeout: int64(o.Timeout), IsolatedTimeout: int64(o.IsolatedTimeout), Env: boundEnvironment(os.Environ(), o.Env)}
	file := filepath.Join(state, "state.json")
	if data, err := os.ReadFile(file); err == nil {
		var got stateBinding
		if err := json.Unmarshal(data, &got); err != nil {
			return fmt.Errorf("rubylinecov: state %s belongs to another collection (unreadable binding: %v)", state, err)
		}
		if differ := environmentDiffers(got.Env, want.Env); len(differ) > 0 && got.Profile == want.Profile {
			return fmt.Errorf("rubylinecov: state %s belongs to another collection: its environment differs in %s; resume under the environment its first chunk ran with", state, strings.Join(differ, ", "))
		}
		if !reflect.DeepEqual(got, want) {
			return fmt.Errorf("rubylinecov: state %s belongs to another collection (state profile, collector, root, commit, argv, deadlines or declared environment differ)", state)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(state, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(state)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("rubylinecov: state %s is not empty and has no binding", state)
	}
	data, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, append(data, '\n'), 0o644)
}

// environmentDiffers names the variables whose binding differs.
func environmentDiffers(got, want map[string]string) []string {
	var names []string
	for name, value := range want {
		if got[name] != value {
			names = append(names, name)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// isolateArtifacts shares the same comparison for live and core-captured runs.
func isolateArtifacts(run *Run, isolatedRun func(int, Outcome) (*Run, error)) *Run {
	run.isolated = true
	records := map[string]*record{}
	for i := range run.records {
		r := &run.records[i]
		records[r.pkg+"\x00"+r.name] = r
	}
	for i := range run.Outcomes {
		out := &run.Outcomes[i]
		if out.Status == "pending" {
			continue
		}
		isolated, err := isolatedRun(i, *out)
		divergent := err != nil || isolated == nil || len(isolated.Outcomes) != 1
		if !divergent {
			a := isolated.Outcomes[0]
			divergent = a.Package != out.Package || a.Name != out.Name || a.Status != out.Status || a.Recorded != out.Recorded
		}
		if divergent {
			out.Recorded = false
			out.Reason = "isolation-divergent"
			continue
		}
		if !out.Recorded {
			continue
		}
		target := records[out.Package+"\x00"+out.Name]
		if target == nil {
			out.Recorded = false
			out.Reason = "isolation-divergent: missing record"
			continue
		}
		// A test whose ordinary-order reach exceeds its isolated reach depends on
		// earlier tests (shared state): selecting it alone would not reach the
		// changed code, so the file keeps whole-file fallback.
		if !reachCovered(target, isolated.records, out.Package, out.Name) {
			out.Recorded = false
			out.Reason = "isolation-divergent: ordinary-order reach exceeds isolated reach"
			continue
		}
		for _, r := range isolated.records {
			if r.pkg == out.Package && r.name == out.Name {
				for p, ls := range r.lines {
					target.lines[p] = append(target.lines[p], ls...)
				}
				for p, ms := range r.methods {
					target.methods[p] = append(target.methods[p], ms...)
				}
			}
		}
		mergeShapes(run.files, isolated.files)
	}
	complete := map[string]bool{}
	for _, o := range run.Outcomes {
		if o.Recorded {
			complete[o.Package+"\x00"+o.Name] = true
		}
	}
	var kept []record
	for _, r := range run.records {
		if r.pkg == Load || complete[r.pkg+"\x00"+r.name] {
			kept = append(kept, r)
		}
	}
	run.records = kept
	return run
}

// reachCovered reports whether every line and method the ordinary-order record
// reached was also reached by the isolated run of the same test.
func reachCovered(ordinary *record, isolated []record, pkg, name string) bool {
	lines := map[string]map[int32]bool{}
	methods := map[string]map[span]bool{}
	for _, r := range isolated {
		if r.pkg != pkg || r.name != name {
			continue
		}
		for p, ls := range r.lines {
			if lines[p] == nil {
				lines[p] = map[int32]bool{}
			}
			for _, l := range ls {
				lines[p][l] = true
			}
		}
		for p, ms := range r.methods {
			if methods[p] == nil {
				methods[p] = map[span]bool{}
			}
			for _, m := range ms {
				methods[p][m] = true
			}
		}
	}
	for p, ls := range ordinary.lines {
		for _, l := range ls {
			if !lines[p][l] {
				return false
			}
		}
	}
	for p, ms := range ordinary.methods {
		for _, m := range ms {
			if !methods[p][m] {
				return false
			}
		}
	}
	return true
}
