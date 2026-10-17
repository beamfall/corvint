package cli

import (
	"fmt"
	"sort"
	"time"
)

// doctorChurnSeconds is the SESSION_CHURN short-session bound (ATR-V0-062).
const doctorChurnSeconds = 300

// doctorChurnRemedy is the SESSION_CHURN operator guidance.
const doctorChurnRemedy = "Sessions on this ticket end within minutes: inspect the worker output, then fix the launch, refine the ticket or set shortSessions in the dispatch configuration."

type doctorSession struct {
	secs  int
	seq   uint64
	start time.Time
}

// doctorSessionChurn finds each candidate ticket whose newest ended
// generations, at least doctorThreshold in a row, each lasted less than
// doctorChurnSeconds from the receipt that began it to the receipt that
// ended it (ATR-V0-062). A generation whose start precedes the scan has an
// unknown length and ends the run. A ticket already reported as
// NO_PROGRESS_HANDOFF in prior is not reported again.
func doctorSessionChurn(scan *doctorScan, candidate func(string) bool, prior []doctorFinding) []doctorFinding {
	reported := map[string]bool{}
	for _, f := range prior {
		reported[f.who] = reported[f.who] || f.kind == "NO_PROGRESS_HANDOFF"
	}
	start := map[string]time.Time{}
	lastGen := map[string]string{}
	ended := map[string]bool{}
	runs := map[string][]doctorSession{}
	var order []string
	for _, ev := range scan.events {
		for i, a := range ev.attempts {
			gen := fmt.Sprint(a.Generation)
			key := a.AttemptID + "/" + gen
			prev, seen := lastGen[a.AttemptID]
			lastGen[a.AttemptID] = gen
			began := ev.fresh[i] || seen && prev != gen
			if _, ok := start[key]; !ok && began {
				start[key] = ev.at
			}
			if a.Live() || a.RetryAccounting == nil || ended[key] {
				continue
			}
			ended[key] = true
			t := a.TicketID.Raw
			if _, ok := runs[t]; !ok {
				order = append(order, t)
			}
			s := doctorSession{secs: -1, seq: ev.seq}
			if at, ok := start[key]; ok {
				s.secs, s.start = int(ev.at.Sub(at).Seconds()), at
			}
			runs[t] = append(runs[t], s)
		}
	}
	var out []doctorFinding
	for _, t := range order {
		if !candidate(t) || reported[t] {
			continue
		}
		gens := runs[t]
		i := len(gens)
		for i > 0 && gens[i-1].secs >= 0 && gens[i-1].secs < doctorChurnSeconds {
			i--
		}
		run := gens[i:]
		if len(run) < doctorThreshold {
			continue
		}
		secs := make([]int, len(run))
		f := doctorFinding{kind: "SESSION_CHURN", who: t, firstSeen: run[0].start, remedy: doctorChurnRemedy}
		for j, s := range run {
			secs[j] = s.secs
			f.seqs = append(f.seqs, s.seq)
		}
		sort.Ints(secs)
		median := secs[len(secs)/2]
		if len(secs)%2 == 0 {
			median = (secs[len(secs)/2-1] + median) / 2
		}
		f.detail = fmt.Sprintf("%d consecutive sessions shorter than %ds, median %ds, first seen %s", len(run), doctorChurnSeconds, median, run[0].start.UTC().Format(time.RFC3339))
		out = append(out, f)
	}
	return out
}
