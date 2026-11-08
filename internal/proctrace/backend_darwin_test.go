// SPDX-License-Identifier: Apache-2.0

package proctrace

import (
	"context"
	"reflect"
	"testing"
)

// This request would execute a shell if a backend incorrectly launched it.
// No runner or tracing tool is needed to test the deliberate absence path.
func darwinRequest() Request {
	return Request{
		Executable:   "/bin/sh",
		Args:         []string{"-c", "exit 29"},
		Dir:          "/",
		Repositories: []Repository{{ID: "primary", Root: "/"}},
		EventCap:     100,
		PathCap:      100,
	}
}

func assertDarwinAbsent(t *testing.T, r Result) {
	t.Helper()
	if r.Backend != "absent" || r.Outcome != (Outcome{ExitCode: -1}) || r.Events != 0 || r.Duration != 0 || len(r.Outside) != 0 {
		t.Fatalf("unqualified backend fabricated execution or observations: %+v", r)
	}
	assertDarwinComponent(t, "reads", r.Reads)
	assertDarwinComponent(t, "listings", r.Listings)
	assertDarwinComponent(t, "lookups", r.Lookups)
	assertDarwinComponent(t, "writes", r.Writes)
	assertDarwinComponent(t, "execs", r.Execs)
	assertDarwinComponent(t, "external", r.External)
}

func assertDarwinComponent[T any](t *testing.T, name string, c Component[T]) {
	t.Helper()
	want := []string{"darwin-mechanism-unqualified", "unsupported-platform"}
	if c.State != Absent || len(c.Entries) != 0 || !reflect.DeepEqual(c.Reasons, want) {
		t.Fatalf("%s must remain absent with mechanism evidence: %+v", name, c)
	}
}

func TestRSPV0021DarwinAbsentComponents(t *testing.T) {
	q := darwinRequest()
	if reason := validate(q); reason != "" {
		t.Fatalf("positive fixture is invalid: %s", reason)
	}
	assertDarwinAbsent(t, New().Trace(context.Background(), q))
}

type darwinUnusedIO struct{ t *testing.T }

func (s darwinUnusedIO) Read([]byte) (int, error) {
	s.t.Fatal("absent backend consumed command stdin")
	return 0, nil
}

func (s darwinUnusedIO) Write([]byte) (int, error) {
	s.t.Fatal("absent backend wrote command output")
	return 0, nil
}

func TestRSPV0021DarwinNoUnobservedExecution(t *testing.T) {
	q := darwinRequest()
	q.Stdin, q.Stdout, q.Stderr = darwinUnusedIO{t}, darwinUnusedIO{t}, darwinUnusedIO{t}
	assertDarwinAbsent(t, New().Trace(context.Background(), q))
}

func TestRSPV0022DarwinValidCapsRemainAbsent(t *testing.T) {
	for _, cap := range []int{1, 100} {
		q := darwinRequest()
		q.EventCap, q.PathCap = cap, cap
		assertDarwinAbsent(t, New().Trace(context.Background(), q))
	}
}

func TestRSPV0022DarwinMalformedRequestsRemainAbsent(t *testing.T) {
	cases := map[string]func(*Request){
		"relative-executable": func(q *Request) { q.Executable = "sh" },
		"nul-executable":      func(q *Request) { q.Executable = "/bin/sh\x00" },
		"relative-directory":  func(q *Request) { q.Dir = "." },
		"zero-event-cap":      func(q *Request) { q.EventCap = 0 },
		"negative-path-cap":   func(q *Request) { q.PathCap = -1 },
		"missing-repository":  func(q *Request) { q.Repositories = nil },
		"unknown-outside": func(q *Request) {
			q.Outside = []OutsideClass{{Class: "unknown", Root: "/"}}
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			q := darwinRequest()
			change(&q)
			if validate(q) == "" {
				t.Fatal("negative fixture did not reach malformed request")
			}
			assertDarwinAbsent(t, New().Trace(context.Background(), q))
		})
	}
}

func TestRSPV0023DarwinDeterministicAbsence(t *testing.T) {
	q := darwinRequest()
	first := New().Trace(context.Background(), q)
	second := New().Trace(context.Background(), q)
	assertDarwinAbsent(t, first)
	assertDarwinAbsent(t, second)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("absence depends on unobserved execution: %+v != %+v", first, second)
	}
}

func TestRSPV0023DarwinUnknownContextRemainsAbsent(t *testing.T) {
	q := darwinRequest()
	if reason := validate(q); reason != "" {
		t.Fatalf("context fixture is invalid: %s", reason)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if cancelled.Err() != context.Canceled {
		t.Fatal("negative fixture did not cancel its context")
	}
	for _, ctx := range []context.Context{nil, cancelled} {
		assertDarwinAbsent(t, New().Trace(ctx, q))
	}
	assertDarwinAbsent(t, New().Trace(context.Background(), Request{}))
}
