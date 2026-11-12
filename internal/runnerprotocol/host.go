// SPDX-License-Identifier: Apache-2.0
package runnerprotocol

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// Plan-line vocabulary of the run-level adapter host (RSP-V0-063).
const (
	ModeFull                 = "full"
	RunOK                    = "ok"
	RunFail                  = "fail"
	RunnerAdapterUnavailable = "runner-adapter-unavailable"
	ReasonUnpinned           = "unpinned"
	ReasonNotQualified       = "runner-not-qualified"
	ReasonSessionOpenFailed  = "session-open-failed"
	ReasonRunnerClaimedTwice = "runner-claimed-by-two-pins"
)

// Line is the host's plan line for one required external runner. Mode is
// always full: the host never admits qualification (RSP-V0-030).
type Line struct {
	Runner    string `json:"runner"`
	Mode      string `json:"mode"`
	Reason    string `json:"reason"`
	Run       string `json:"run"`
	RunReason string `json:"run_reason,omitempty"`
}

// HostInput is everything the host may consult. Plan carries the trusted pins
// in CI (RSP-V0-028, RSP-V0-042); CorePath reports the runners that keep a
// pre-plugin execution path and may be nil (none).
type HostInput struct {
	Required       []string
	Plan           DiscoveryPlan
	Store, Release Discovery
	Options        Options
	CorePath       func(runner string) bool
}

// Host owns at most one session per required pin for one run (RSP-V0-029).
// Close is mandatory and is the only producer of plan lines. A Host is not
// safe for concurrent use: one goroutine opens it, calls Plugin and closes
// it, as the sessions it owns are sequential.
type Host struct {
	input    HostInput
	required []string
	reasons  map[string]string   // pre-session causes, by runner
	sessions map[string]*Session // by pin name
	byRunner map[string]*Session
	closed   bool
	lines    []Line
}

// OpenHost resolves and launches only the pins a required runner names. It
// never fails as a whole: every refusal is retained as that runner's cause.
func OpenHost(ctx context.Context, in HostInput) *Host {
	h := &Host{input: in, reasons: map[string]string{}, sessions: map[string]*Session{}, byRunner: map[string]*Session{}}
	seen := map[string]bool{}
	for _, r := range in.Required {
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		// Compiled-in runners are outside the host (RSP-V0-025).
		if _, compiled := registry.Lookup(r); compiled || r == "script" || r == "make" {
			continue
		}
		h.required = append(h.required, r)
	}
	sort.Strings(h.required)
	if in.Plan.FullAll != "" {
		for _, r := range h.required {
			h.reasons[r] = in.Plan.FullAll
		}
		return h
	}
	claims := map[string]int{}
	for _, pin := range in.Plan.Pins {
		for _, r := range pin.Runners {
			claims[r]++
		}
	}
	for _, r := range h.required {
		switch reason := in.Plan.Full[r]; {
		case reason != "" && reason != "candidate-pins-changed":
			// unsupported-schema or any other discovery cause: launch nothing.
			h.reasons[r] = reason
		case claims[r] == 0:
			h.reasons[r] = ReasonUnpinned
		case claims[r] > 1:
			h.reasons[r] = ReasonRunnerClaimedTwice
		}
	}
	for _, pin := range in.Plan.Pins {
		if _, done := h.sessions[pin.Name]; done || !h.needs(pin) {
			continue
		}
		resolved, err := Resolve(pin, in.Store, in.Release)
		var s *Session
		if err == nil {
			s, err = Open(ctx, resolved, in.Options)
		}
		h.sessions[pin.Name] = s
		for _, r := range pin.Runners {
			if !contains(h.required, r) || h.reasons[r] != "" {
				continue
			}
			if err != nil {
				h.reasons[r] = reasonCode(err)
				continue
			}
			h.byRunner[r] = s
		}
	}
	return h
}

func (h *Host) needs(pin Pin) bool {
	for _, r := range pin.Runners {
		if contains(h.required, r) && h.reasons[r] == "" {
			return true
		}
	}
	return false
}

// Sessions reports how many plugin processes the host launched.
func (h *Host) Sessions() int {
	n := 0
	for _, s := range h.sessions {
		if s != nil {
			n++
		}
	}
	return n
}

// Plugin returns the SPI adapter for a required runner whose session opened.
func (h *Host) Plugin(runner string) (registry.RunnerPlugin, error) {
	if h.closed {
		return nil, fmt.Errorf("host-closed")
	}
	s := h.byRunner[runner]
	if s == nil {
		if reason := h.reasons[runner]; reason != "" {
			return nil, fmt.Errorf("%s:%s", RunnerAdapterUnavailable, reason)
		}
		return nil, fmt.Errorf("%s:not-required", RunnerAdapterUnavailable)
	}
	return s.Plugin(runner)
}

// Close shuts every session, then emits one line per required runner sorted
// by name. A required runner is never omitted and never silently passes.
func (h *Host) Close(ctx context.Context) []Line {
	if h.closed {
		return append([]Line(nil), h.lines...)
	}
	h.closed = true
	names := make([]string, 0, len(h.sessions))
	for name := range h.sessions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if s := h.sessions[name]; s != nil {
			_ = s.Close(ctx)
		}
	}
	for _, r := range h.required {
		line := Line{Runner: r, Mode: ModeFull, Reason: h.reasons[r], Run: RunFail}
		if line.Reason == "" {
			s := h.byRunner[r]
			if s == nil {
				// Only a caller-built plan (for example two pins sharing a
				// name) can leave a pinned runner without a session.
				line.Reason = "adapter-session-missing"
			} else if failure := s.FullReason(); failure != "" {
				line.Reason = reasonCode(fmt.Errorf("%s", failure))
			} else if h.input.Plan.Full[r] != "" {
				line.Reason = h.input.Plan.Full[r]
				line.Run = RunOK
			} else {
				line.Reason = ReasonNotQualified
				line.Run = RunOK
			}
		}
		if line.Run == RunFail && h.input.CorePath != nil && h.input.CorePath(r) {
			line.Run = RunOK
		}
		if line.Run == RunFail {
			line.RunReason = RunnerAdapterUnavailable
		}
		h.lines = append(h.lines, line)
	}
	return append([]Line(nil), h.lines...)
}

var reasonPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// reasonCode keeps the stable failure code and drops wrapped OS detail, such as
// store paths, which would otherwise leak into plan output. A failure that does
// not start with a code (for example an unwrapped lstat error) is reported as
// session-open-failed (RSP-V0-063).
func reasonCode(err error) string {
	code, _, _ := strings.Cut(err.Error(), ":")
	if !reasonPattern.MatchString(code) {
		return ReasonSessionOpenFailed
	}
	return code
}
