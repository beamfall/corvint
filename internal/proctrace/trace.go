// SPDX-License-Identifier: Apache-2.0

// Package proctrace observes test process trees. It has no evidence-store dependency
// and is deliberately not connected to any command. Results are observations, not
// authority to narrow a test plan.
// RSP-V0-020, RSP-V0-006 and RSP-V0-024: isolated service and parity observations.
package proctrace

import (
	"context"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type State string

const (
	Present    State = "present"
	Incomplete State = "incomplete"
	Absent     State = "absent"
)

// Component names match the evidence contract; these are plain Go values, not a wire format.
type Component[T any] struct {
	State   State
	Reasons []string
	Entries []T
}
type Path struct {
	Repository         string
	Path               string
	ResolvedRepository string
	ResolvedPath       string
}
type Write struct {
	Path Path
	Kind string
}
type Exec struct {
	Path         Path
	Digest       string
	Instrumented bool
}
type Result struct {
	Backend  string
	Reads    Component[Path]
	Listings Component[Path]
	Lookups  Component[Path]
	Writes   Component[Write]
	Execs    Component[Exec]
	External Component[string]
	Outside  map[string]int
	Outcome  Outcome
	Duration time.Duration
	Events   int
}
type Outcome struct {
	Started  bool
	ExitCode int
	Signal   int
	Error    string
}
type Repository struct{ ID, Root string }

// OutsideClass is an explicitly declared toolchain, cache or temporary root.
// Undeclared outside reads are incomplete; no outside absolute paths leave this service.
type OutsideClass struct{ Class, Root string }
type Request struct {
	Executable     string   // absolute; never resolved through PATH
	Args           []string // arguments excluding argv[0]
	Dir            string
	Env            []string     // nil inherits the caller environment
	Repositories   []Repository // first is the primary repository
	Outside        []OutsideClass
	EventCap       int
	PathCap        int
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}
type Backend interface {
	Trace(context.Context, Request) Result
}

func New() Backend { return platformBackend{} }

func emptyResult(backend string) Result {
	return Result{Backend: backend, Reads: Component[Path]{State: Present}, Listings: Component[Path]{State: Present}, Lookups: Component[Path]{State: Present}, Writes: Component[Write]{State: Present}, Execs: Component[Exec]{State: Present}, External: Component[string]{State: Present}, Outside: map[string]int{}, Outcome: Outcome{ExitCode: -1}}
}
func mark[T any](c *Component[T], state State, reason string) {
	c.State = state
	for _, s := range c.Reasons {
		if s == reason {
			return
		}
	}
	c.Reasons = append(c.Reasons, reason)
	sort.Strings(c.Reasons)
}
func (r *Result) unavailable(state State, reason string) {
	mark(&r.Reads, state, reason)
	mark(&r.Listings, state, reason)
	mark(&r.Lookups, state, reason)
	mark(&r.Writes, state, reason)
	mark(&r.Execs, state, reason)
	mark(&r.External, state, reason)
}
func validate(q Request) string {
	if !filepath.IsAbs(q.Executable) || !filepath.IsAbs(q.Dir) || strings.ContainsRune(q.Executable+q.Dir, 0) {
		return "invalid-command"
	}
	if q.EventCap <= 0 || q.PathCap <= 0 {
		return "invalid-cap"
	}
	if len(q.Repositories) == 0 {
		return "missing-repository"
	}
	ids := map[string]bool{}
	roots := map[string]bool{}
	for _, r := range q.Repositories {
		if r.ID == "" || ids[r.ID] || !filepath.IsAbs(r.Root) || strings.ContainsRune(r.Root, 0) {
			return "invalid-repository"
		}
		root := filepath.Clean(r.Root)
		if roots[root] {
			return "ambiguous-repository"
		}
		ids[r.ID] = true
		roots[root] = true
	}
	for _, c := range q.Outside {
		if c.Class != "toolchain" && c.Class != "cache" && c.Class != "temporary" {
			return "invalid-outside-class"
		}
		if !filepath.IsAbs(c.Root) || strings.ContainsRune(c.Root, 0) {
			return "invalid-outside-root"
		}
	}
	return ""
}

// Qualification retains both outcomes and measured costs for a caller-owned runner.
// A mismatch disables all trace components. The caller must retain this record and
// use its admitted runner identity; the service neither persists nor enables runners.
type Qualification struct {
	Runner                                     string
	Untraced, Traced                           Outcome
	UntracedDuration, TracedDuration, Overhead time.Duration
	Enabled                                    bool
	Reason                                     string
}

func Compare(runner string, untraced Outcome, duration time.Duration, traced *Result) Qualification {
	q := Qualification{Runner: runner, Untraced: untraced, UntracedDuration: duration}
	if traced == nil {
		q.Reason = "missing-trace"
		return q
	}
	q.Traced = traced.Outcome
	q.TracedDuration = traced.Duration
	q.Overhead = traced.Duration - duration
	switch {
	case runner == "" || duration < 0 || traced.Duration < 0:
		q.Reason = "invalid-parity-input"
	case !untraced.Started || !traced.Outcome.Started || untraced.Error != "" || traced.Outcome.Error != "":
		q.Reason = "parity-unobserved"
	case untraced != traced.Outcome:
		q.Reason = "outcome-mismatch"
	default:
		q.Enabled = true
	}
	if !q.Enabled {
		traced.unavailable(Absent, q.Reason)
	}
	return q
}
