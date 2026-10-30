// Package cli is the shared entry of the `corvint-test accept` group and its
// corvint-tests-accept shim (CTF-V0-004), so both produce byte-identical output
// apart from the invocation name in diagnostics and the build identity.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"slices"

	"github.com/Beamfall/corvint/internal/testacceptance"
)

// Run executes one invocation. name is the invoking command, and reentry the fixed arguments
// that select this group's worker when the executable re-enters itself (CTF-V0-006).
func Run(ctx context.Context, name string, reentry []string, args []string, in io.Reader, out io.Writer) error {
	if len(args) == 1 && args[0] == "worker" {
		data, e := io.ReadAll(io.LimitReader(in, testacceptance.InputLimit+1))
		if e != nil {
			return e
		}
		// The environment may carry only the request's declared external-app pairs beyond the
		// safe keys (NEA-V0-013); RunWorker approves the request before running anything.
		var j testacceptance.Job
		decodeErr := testacceptance.Decode(data, &j)
		if !testacceptance.WorkerEnvironmentSafe() && (decodeErr != nil || !testacceptance.WorkerEnvironmentSafeFor(j.Request)) {
			return fmt.Errorf("worker-environment-unsafe")
		}
		if decodeErr != nil {
			return decodeErr
		}
		return json.NewEncoder(out).Encode(testacceptance.RunWorker(ctx, j))
	}
	if len(args) == 0 || (args[0] != "plan" && args[0] != "accept") {
		return fmt.Errorf("usage: %s plan|accept --plan FILE [--approve-plan DIGEST --new PATH... --repeat N --environment LABEL]", name)
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	f.SetOutput(io.Discard)
	path := f.String("plan", "", "operator-owned request")
	approved := f.String("approve-plan", "", "independently reviewed request digest")
	repeat := f.Int("repeat", 0, "repeat count")
	env := f.String("environment", "", "environment label")
	tempRoot := f.String("temp-root", "", "operator-owned canonical temporary output root (accept only)")
	var newFiles paths
	f.Var(&newFiles, "new", "new test file (repeat flag)")
	if e := f.Parse(args[1:]); e != nil || f.NArg() != 0 {
		return fmt.Errorf("flags-invalid")
	}
	file, e := os.Open(*path)
	if e != nil {
		return fmt.Errorf("plan-unreadable")
	}
	defer file.Close()
	data, e := io.ReadAll(io.LimitReader(file, testacceptance.InputLimit+1))
	if e != nil {
		return fmt.Errorf("plan-unreadable")
	}
	var r testacceptance.Request
	if e = testacceptance.Decode(data, &r); e != nil {
		return e
	}
	if args[0] == "plan" {
		if *tempRoot != "" {
			return fmt.Errorf("flags-invalid")
		}
		if e = testacceptance.Validate(r); e != nil {
			return e
		}
		return json.NewEncoder(out).Encode(struct {
			Digest        string `json:"digest"`
			Qualification string `json:"qualification"`
		}{testacceptance.Digest(r), "experimental; provider per-test freshness UNKNOWN"})
	}
	if *repeat != r.Repeat || *env != r.Environment || len(newFiles) == 0 {
		return fmt.Errorf("selection-mismatch")
	}
	expected := []string{}
	for _, t := range r.Tests {
		if !slices.Contains(expected, t.File) {
			expected = append(expected, t.File)
		}
	}
	slices.Sort(expected)
	slices.Sort(newFiles)
	if !slices.Equal(expected, []string(newFiles)) {
		return fmt.Errorf("new-files-mismatch")
	}
	exe, e := os.Executable()
	if e != nil {
		return fmt.Errorf("self-unavailable")
	}
	report, e := testacceptance.ExecuteWithWorker(ctx, r, *approved, exe, reentry, buildIdentity(name), *tempRoot)
	if e != nil {
		return e
	}
	return json.NewEncoder(out).Encode(report)
}

type paths []string

func (p *paths) String() string     { return "" }
func (p *paths) Set(s string) error { *p = append(*p, s); return nil }

// buildIdentity names the executing binary; it is the one report field exempt from the fold's
// byte identity (CTF-V0-002).
func buildIdentity(name string) string {
	revision := "UNKNOWN"
	dirty := "UNKNOWN"
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				revision = s.Value
			}
			if s.Key == "vcs.modified" {
				dirty = s.Value
			}
		}
	}
	return name + " experimental/0 revision=" + revision + " dirty=" + dirty
}
