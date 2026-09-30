// corvint-tests-accept is an optional experimental local companion.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Beamfall/corvint/internal/testacceptance"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"slices"
	"syscall"
)

func run(ctx context.Context, args []string, in io.Reader, out io.Writer) error {
	if len(args) == 1 && args[0] == "worker" {
		if !testacceptance.WorkerEnvironmentSafe() {
			return fmt.Errorf("worker-environment-unsafe")
		}
		data, e := io.ReadAll(io.LimitReader(in, testacceptance.InputLimit+1))
		if e != nil {
			return e
		}
		var j testacceptance.Job
		if e = testacceptance.Decode(data, &j); e != nil {
			return e
		}
		return json.NewEncoder(out).Encode(testacceptance.RunWorker(ctx, j))
	}
	if len(args) == 0 || (args[0] != "plan" && args[0] != "accept") {
		return fmt.Errorf("usage: corvint-tests-accept plan|accept --plan FILE [--approve-plan DIGEST --new PATH... --repeat N --environment LABEL]")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	f.SetOutput(io.Discard)
	path := f.String("plan", "", "operator-owned request")
	approved := f.String("approve-plan", "", "independently reviewed request digest")
	repeat := f.Int("repeat", 0, "repeat count")
	env := f.String("environment", "", "environment label")
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
	report, e := testacceptance.Execute(ctx, r, *approved, exe, buildIdentity())
	if e != nil {
		return e
	}
	return json.NewEncoder(out).Encode(report)
}

type paths []string

func (p *paths) String() string     { return "" }
func (p *paths) Set(s string) error { *p = append(*p, s); return nil }
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if e := run(ctx, os.Args[1:], os.Stdin, os.Stdout); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(2)
	}
}

func buildIdentity() string {
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
	return "corvint-tests-accept experimental/0 revision=" + revision + " dirty=" + dirty
}
