package main

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// dogfoodEventProbeKey carries the measurements one adapter invocation's dogfood event takes as it
// runs (AHI-800, V1-1496): the phase it is in, and the opening repository observation's duration,
// changed-path count and revision once that observation finishes. The event only stores values it
// already has; reading the probe costs no repository work.
type dogfoodEventProbeKey struct{}

type dogfoodEventProbe struct {
	mu             sync.Mutex
	start          time.Time
	limit          time.Duration
	phase          string
	frozen         bool
	frozenAt       time.Duration
	statusDone     bool
	statusElapsed  time.Duration
	changedPaths   int
	commitRevision string
}

// dogfoodEventMeasurement is a probe's frozen copy.
type dogfoodEventMeasurement struct {
	Phase          string
	Elapsed, Limit time.Duration
	StatusDone     bool
	StatusElapsed  time.Duration
	ChangedPaths   int
	CommitRevision string
}

// withDogfoodEventProbe gives ctx a probe, keeping one an outer caller already attached.
func withDogfoodEventProbe(ctx context.Context) context.Context {
	if _, ok := ctx.Value(dogfoodEventProbeKey{}).(*dogfoodEventProbe); ok {
		return ctx
	}
	return context.WithValue(ctx, dogfoodEventProbeKey{}, &dogfoodEventProbe{})
}

func dogfoodEventProbeOf(ctx context.Context) *dogfoodEventProbe {
	probe, _ := ctx.Value(dogfoodEventProbeKey{}).(*dogfoodEventProbe)
	return probe
}

// begin starts a fresh measurement for one event read under limit.
func (probe *dogfoodEventProbe) begin(limit time.Duration) {
	if probe == nil {
		return
	}
	probe.mu.Lock()
	defer probe.mu.Unlock()
	probe.start, probe.limit, probe.phase, probe.frozen, probe.frozenAt = time.Now(), limit, "input-read", false, 0
	probe.statusDone, probe.statusElapsed, probe.changedPaths, probe.commitRevision = false, 0, 0, ""
}

// enter records the phase the read is in. A frozen probe keeps the phase the deadline found.
func (probe *dogfoodEventProbe) enter(phase string) {
	if probe == nil {
		return
	}
	probe.mu.Lock()
	defer probe.mu.Unlock()
	if !probe.frozen && !probe.start.IsZero() {
		probe.phase = phase
	}
}

// observed records the finished opening repository observation (its git status read).
func (probe *dogfoodEventProbe) observed(changed int, revision string) {
	if probe == nil {
		return
	}
	probe.mu.Lock()
	defer probe.mu.Unlock()
	if probe.frozen || probe.start.IsZero() {
		return
	}
	probe.statusDone, probe.statusElapsed, probe.changedPaths, probe.commitRevision = true, time.Since(probe.start), changed, revision
}

// freeze stops the probe at the moment the event reports its expiry, so a read the event abandoned
// cannot move the phase that ran out of time.
func (probe *dogfoodEventProbe) freeze() {
	if probe == nil {
		return
	}
	probe.mu.Lock()
	defer probe.mu.Unlock()
	if !probe.frozen && !probe.start.IsZero() {
		probe.frozen, probe.frozenAt = true, time.Since(probe.start)
	}
}

func (probe *dogfoodEventProbe) measurement() (dogfoodEventMeasurement, bool) {
	if probe == nil {
		return dogfoodEventMeasurement{}, false
	}
	probe.mu.Lock()
	defer probe.mu.Unlock()
	if probe.start.IsZero() {
		return dogfoodEventMeasurement{}, false
	}
	elapsed := time.Since(probe.start)
	if probe.frozen {
		elapsed = probe.frozenAt
	}
	return dogfoodEventMeasurement{Phase: probe.phase, Elapsed: elapsed, Limit: probe.limit, StatusDone: probe.statusDone,
		StatusElapsed: probe.statusElapsed, ChangedPaths: probe.changedPaths, CommitRevision: probe.commitRevision}, true
}

// hostLoadMeasureBound is the most time the deadline notice spends reading the host load average
// (AHI-800). The read is one in-process file or sysctl read; a read that has not returned by then
// is reported as not measured, so the measurement cannot push the hook past its deadline.
const hostLoadMeasureBound = 20 * time.Millisecond

// dogfoodHostLoadKey carries a per-invocation replacement for hostLoadAverage; tests set it.
type dogfoodHostLoadKey struct{}

type hostLoadFunc = func() (float64, bool)

// boundedHostLoad returns the 1-minute load average when the OS exposes it within
// hostLoadMeasureBound, and ok false otherwise.
func boundedHostLoad(ctx context.Context) (float64, bool) {
	read, ok := ctx.Value(dogfoodHostLoadKey{}).(hostLoadFunc)
	if !ok {
		read = hostLoadAverage
	}
	type sample struct {
		load float64
		ok   bool
	}
	done := make(chan sample, 1)
	go func() {
		load, ok := read()
		done <- sample{load, ok}
	}()
	timer := time.NewTimer(hostLoadMeasureBound)
	defer timer.Stop()
	select {
	case got := <-done:
		return got.load, got.ok
	case <-timer.C:
		return 0, false
	}
}

// deadlineMeasuredLines is AHI-800's measured part of a deadline notice: the phase that ran out of
// time, the measurements already taken, and the dominant cause, named only when a measurement
// supports it. A large worktree is supported when the opening git status finished but used at
// least half the event's limit; host load when the 1-minute load average is at least the CPU count.
func deadlineMeasuredLines(ctx context.Context) string {
	measured, ok := dogfoodEventProbeOf(ctx).measurement()
	load, loadOK := boundedHostLoad(ctx)
	cpus := runtime.NumCPU()
	lines := []string{}
	if ok {
		lines = append(lines, fmt.Sprintf("Phase that ran out of time: %s, after %d ms of the %d ms limit.", measured.Phase, measured.Elapsed.Milliseconds(), measured.Limit.Milliseconds()))
	} else {
		lines = append(lines, "Phase that ran out of time: not measured.")
	}
	status := "git status: did not finish before the limit, not measured."
	if ok && measured.StatusDone {
		status = fmt.Sprintf("git status: finished in %d ms with %d changed paths.", measured.StatusElapsed.Milliseconds(), measured.ChangedPaths)
	}
	hostLoad := "Host load: not measured on this OS."
	if loadOK {
		hostLoad = fmt.Sprintf("Host load: 1-minute load average %.2f on %d CPUs.", load, cpus)
	}
	lines = append(lines, "Measured: "+status+" "+hostLoad)
	causes := []string{}
	if loadOK && load >= float64(cpus) {
		causes = append(causes, "host load")
	}
	if ok && measured.StatusDone && measured.Limit > 0 && measured.StatusElapsed*2 >= measured.Limit {
		causes = append(causes, "large worktree")
	}
	cause := "not measured"
	if len(causes) != 0 {
		cause = strings.Join(causes, " and ")
	}
	return "\n" + strings.Join(lines, "\n") + "\nDominant cause: " + cause + "."
}

// reducedPacketProfile names AHI-801's bounded packet: what a prompt-context event delivers when
// its dogfood event ran out of time, instead of no Corvint context at all.
const reducedPacketProfile = "corvint-reduced-packet/0"

// reducedPacketReasons are the expiry rejections AHI-801 answers with a reduced packet.
var reducedPacketReasons = map[string]string{
	claudeDeadlineReason:      "dogfood-event-deadline",
	claudeSnapshotStaleReason: "dogfood-event-index-snapshot-stale",
}

// withReducedPacket appends AHI-801's reduced packet to an expired SessionStart or UserPromptSubmit
// notice, after its unchanged frame and remediation: trusted adapter guidance (the compaction
// disclosure on a compact start) and a framed packet that is marked degraded, carries the typed
// degradation, names the evidence as NOT_PRODUCED, and reports the opening observation's revision
// and changed-path count only when that observation finished. It reads nothing new and writes
// nothing. Output that would not fit the adapter output limit keeps the notice alone.
func withReducedPacket(ctx context.Context, root, event, reason string, normalized map[string]any, guidance string, output map[string]any) map[string]any {
	code, covered := reducedPacketReasons[reason]
	if !covered || (event != "session-start" && event != "user-prompt") {
		return output
	}
	hook, ok := output["hookSpecificOutput"].(map[string]any)
	text, isText := hook["additionalContext"].(string)
	if !ok || !isText {
		return output
	}
	packet := map[string]any{"profile": reducedPacketProfile, "event": event, "degraded": true, "degradation": code, "evidence": "NOT_PRODUCED"}
	if source, ok := normalized["startSource"].(string); ok {
		packet["startSource"] = source
	}
	repository := map[string]any{"status": "NOT_OBSERVED"}
	if measured, ok := dogfoodEventProbeOf(ctx).measurement(); ok && measured.StatusDone && measured.CommitRevision != "" {
		repository = map[string]any{"status": "OBSERVED", "commitRevision": measured.CommitRevision, "changedPathCount": measured.ChangedPaths}
	}
	packet["repository"] = repository
	packet["recovery"] = []string{"corvint", "--root", root, "query", "--task", "Corvint hook check"}
	raw, _ := json.Marshal(packet)
	framed, err := repoenvelope.Frame(string(raw))
	if err != nil {
		return output
	}
	label := "\nCorvint reduced packet (degraded; the full packet did not fit the hook time limit):\n"
	extended := map[string]any{}
	for key, value := range output {
		extended[key] = value
	}
	extended["hookSpecificOutput"] = map[string]any{"hookEventName": hook["hookEventName"], "additionalContext": text + label + guidance + framed}
	if !withinAdapterOutputLimit(extended) {
		return output
	}
	return extended
}
