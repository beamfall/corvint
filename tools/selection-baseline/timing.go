// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// stepOrder is the order of a replayed change's steps (ASC-V0-782); universe
// runs only for a FULL manifest under SELECTION_CONTRACT_CHECK=1.
var stepOrder = []string{"checkout", "affected", "selector", "test", "guard", "universe"}

// stepTimes holds one replayed change's wall time per step (ASC-V0-782). It is
// written only to stderr: never to a Result, the results JSON or a cache record,
// so a timing can change no verdict, key or report byte.
type stepTimes struct {
	steps map[string]time.Duration
	total time.Duration
}

// mark records the step that began at start and returns the next step's start.
func (s *stepTimes) mark(step string, start time.Time) time.Time {
	now := time.Now()
	if s.steps == nil {
		s.steps = map[string]time.Duration{}
	}
	s.steps[step] += now.Sub(start)
	s.total += now.Sub(start)
	return now
}

func (s *stepTimes) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "total=%.1fs", s.total.Seconds())
	for _, step := range stepOrder {
		if d, ok := s.steps[step]; ok {
			fmt.Fprintf(&b, " %s=%.1fs", step, d.Seconds())
		}
	}
	return b.String()
}

// timingSummary is the one line that attributes a replay's time (ASC-V0-782):
// the replayed count, workers and wall seconds, each step's seconds summed over
// the replayed changes and its share of their sum, and the five slowest changes.
func timingSummary(changes []change, timings []*stepTimes, workers int, wall time.Duration) string {
	sums := map[string]time.Duration{}
	var all time.Duration
	type slow struct {
		id    string
		total time.Duration
	}
	var replayed []slow
	for i, t := range timings {
		if t == nil {
			continue
		}
		for step, d := range t.steps {
			sums[step] += d
		}
		all += t.total
		replayed = append(replayed, slow{changes[i].ID, t.total})
	}
	var b strings.Builder
	fmt.Fprintf(&b, "selection-baseline: TIMING replayed %d of %d changes on %d workers in %.1fs wall; step seconds summed over changes:", len(replayed), len(changes), workers, wall.Seconds())
	for _, step := range stepOrder {
		if d, ok := sums[step]; ok {
			share := 0.0
			if all > 0 {
				share = 100 * d.Seconds() / all.Seconds()
			}
			fmt.Fprintf(&b, " %s=%.1fs (%.0f%%)", step, d.Seconds(), share)
		}
	}
	sort.SliceStable(replayed, func(i, j int) bool { return replayed[i].total > replayed[j].total })
	b.WriteString("; slowest:")
	for i, r := range replayed {
		if i == 5 {
			break
		}
		fmt.Fprintf(&b, " %s=%.1fs", r.id, r.total.Seconds())
	}
	return b.String()
}
