package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os/exec"
	"slices"
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Upgrade-check verdicts and reasons (DSG-V0-007).
const (
	UpgradeRolling       = "ROLLING"
	UpgradeDrainRequired = "DRAIN_REQUIRED"
	UpgradeRefused       = "REFUSED"
	UpgradeUnknown       = "UNKNOWN"

	upgradeProbeTimeout = 60 * time.Second
	maxUpgradeProbe     = 4 << 20
)

// dispatchUpgrade is `dispatch upgrade --check NEW_BUILD` (DSG-V0-007): it
// asks NEW_BUILD to read this program's configuration and ledger through its
// own read-only `dispatch status`, and reports whether a restart on that build
// adopts the live workers (ROLLING), needs a drain first (DRAIN_REQUIRED),
// cannot start at all (REFUSED), or could not be decided (UNKNOWN). It never
// writes state, signals a process, or starts a dispatcher.
func dispatchUpgrade(env Env, args []string) *wire.Result {
	cmd := []string{"dispatch", "upgrade"}
	values, msg := dispatchFlags(env.Cwd, args, map[string]bool{"--program": true, "--config": true, "--check": true, "--apply": true}, nil)
	if msg != "" {
		return usage(cmd, msg)
	}
	flag, build := "--check", values["--check"]
	if values["--apply"] != "" {
		if build != "" {
			return usage(cmd, "--check and --apply are exclusive")
		}
		flag, build = "--apply", values["--apply"]
	}
	if build == "" {
		return usage(cmd, "dispatch upgrade requires --check NEW_BUILD or --apply NEW_BUILD (the corvint-tasks executable to switch to)")
	}
	build = commandPath(env.Cwd, build)
	sum, e := dispatch.BuildDigest(build)
	if e != nil {
		return usage(cmd, flag+" must name an executable regular file: "+e.Error())
	}
	c, res, _ := dispatchAuxConfig([]string{"dispatch", "status"}, "status", values)
	if res != nil {
		res.Command = cmd
		return res
	}
	dir := dispatch.ProgramDir(c, values["--program"])
	live := dispatch.StateUnknown
	l, e := dispatch.LoadLedger(dir, values["--program"])
	if e == nil {
		live = strconv.Itoa(len(l.Workers))
	}
	verdict, reason, detail, handover := upgradeProbe(env, build, values, live)
	profile := "taskman-dispatch-upgrade-check/0"
	if flag == "--apply" {
		profile = "taskman-dispatch-upgrade-apply/0"
		verdict, reason, detail = upgradeApply(dir, l, build, sum, verdict, reason, detail, handover)
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(profile))
	o.Set("program", wire.String(values["--program"]))
	o.Set("newBuild", wire.String(build))
	o.Set("newBuildSha256", wire.String(sum))
	o.Set("liveWorkers", wire.String(live))
	o.Set("verdict", wire.String(verdict))
	o.Set("reason", wire.String(reason))
	o.Set("detail", wire.String(prose(detail)))
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Codes: []string{}, Items: []wire.Value{{Kind: wire.KindObject, Obj: o}}}
}

// Apply verdicts (DSG-V0-011).
const (
	UpgradeRequested = "REQUESTED"
)

// upgradeApply is `dispatch upgrade --apply NEW_BUILD` (DSG-V0-011): after a
// ROLLING check it asks the running dispatcher to hand over to NEW_BUILD by
// writing one request file, which the dispatcher consumes between ticks. It
// never signals a process or starts a dispatcher; any refusal writes nothing.
func upgradeApply(dir string, l *dispatch.Ledger, build, sum, verdict, reason, detail string, handover bool) (string, string, string) {
	if verdict != UpgradeRolling {
		return verdict, reason, detail
	}
	if !handover {
		return UpgradeRefused, "NO_HANDOVER", "the new build does not accept " + dispatch.HandoverProtocol + "; stop this dispatcher and start the new build, which adopts the live workers"
	}
	state, pid := dispatch.OwnerProcess(dir)
	if state != "RUNNING" {
		return UpgradeRefused, "NOT_RUNNING", "no dispatcher runs for this program (" + state + "); start the new build directly: it adopts the recorded workers"
	}
	st, err := dispatch.ReadUpgradeState(dir)
	if err != nil || st.PID != pid {
		return UpgradeRefused, "NO_HANDOVER", "the running dispatcher (pid " + strconv.Itoa(pid) + ") does not accept a handover; stop it and start the new build, which adopts the live workers"
	}
	if st.Phase == dispatch.UpgradeExec {
		return UpgradeRefused, "UPGRADE_IN_PROGRESS", "a handover to " + st.Build + " is in progress"
	}
	if l == nil {
		return UpgradeUnknown, "PROBE_FAILED", "the ledger could not be read, so the live builds are unknown"
	}
	builds := dispatch.LiveBuilds(dir, l)
	builds[st.Sha256] = true
	builds[sum] = true
	if len(builds) > dispatch.MaxMixedBuilds {
		return UpgradeRefused, "MIXED_BUILD_LIMIT", strconv.Itoa(len(builds)) + " distinct builds would be live at once, above the limit of " + strconv.Itoa(dispatch.MaxMixedBuilds) + "; wait for workers of an earlier build to finish"
	}
	err = dispatch.WriteUpgradeRequest(dir, dispatch.UpgradeRequest{Build: build, Sha256: sum, RequestedAt: time.Now().UTC().Format(time.RFC3339)})
	switch {
	case errors.Is(err, fs.ErrExist):
		return UpgradeRefused, "REQUEST_PENDING", "an upgrade request is already pending for this program"
	case err != nil:
		return UpgradeUnknown, "REQUEST_UNWRITTEN", "the upgrade request could not be written: " + err.Error()
	}
	return UpgradeRequested, "HANDOVER_REQUESTED", "the running dispatcher (pid " + strconv.Itoa(pid) + ") hands over to the new build after its current tick; its " + strconv.Itoa(len(l.Workers)) + " live worker(s) finish on their own builds; follow the upgrading and upgraded events in dispatch status"
}

// upgradeProbe runs NEW_BUILD's `dispatch status` on this program and reads
// its verdict from the result: a decoded ledger and configuration is
// ROLLING; a configuration it refuses is REFUSED; a ledger it refuses
// UNSUPPORTED_VERSION is DRAIN_REQUIRED while workers are live and REFUSED
// when none are, since a drain cannot help then; anything else is UNKNOWN.
func upgradeProbe(env Env, build string, values map[string]string, live string) (verdict, reason, detail string, handover bool) {
	ctx, cancel := context.WithTimeout(context.Background(), upgradeProbeTimeout)
	defer cancel()
	probe := exec.CommandContext(ctx, build, "dispatch", "status", "--program", values["--program"], "--config", values["--config"], "--events", "0")
	probe.Dir = env.Cwd
	// A descendant that keeps the output pipes open cannot outlast the
	// timeout: the pipes close WaitDelay after the kill.
	probe.WaitDelay = 5 * time.Second
	var out bytes.Buffer
	probe.Stdout, probe.Stderr = &limitedWriter{w: &out, n: maxUpgradeProbe}, io.Discard
	runErr := probe.Run()
	if ctx.Err() != nil || errors.Is(runErr, exec.ErrWaitDelay) {
		return UpgradeUnknown, "PROBE_FAILED", "the new build's dispatch status did not finish within " + upgradeProbeTimeout.String(), false
	}
	res, err := wire.DecodeResult(out.Bytes())
	if err != nil {
		d := "the new build's dispatch status printed no readable result"
		if runErr != nil {
			d += " (" + runErr.Error() + ")"
		}
		return UpgradeUnknown, "PROBE_FAILED", d, false
	}
	warning := ""
	if len(res.Warnings) > 0 {
		warning = res.Warnings[0]
	}
	for _, item := range res.Items {
		if item.Obj == nil {
			continue
		}
		if cf, ok := item.Obj.Get("configFile"); ok && cf.Obj != nil {
			r, _ := cf.Obj.Get("reason")
			return UpgradeRefused, "CONFIG_INVALID", "the new build refuses the configuration: " + r.Str, false
		}
		if u, ok := item.Obj.Get("upgrade"); ok && u.Obj != nil {
			h, _ := u.Obj.Get("handover")
			handover = h.Str == dispatch.HandoverProtocol
		}
	}
	switch {
	case res.Outcome == wire.OutcomeOK:
		return UpgradeRolling, "STATE_COMPATIBLE", "the new build reads this program's configuration and ledger; stop this dispatcher and start the new build: it adopts the " + live + " live worker(s), which finish under the attempts and leases they hold, and new launches use the new build", handover
	case slices.Contains(res.Codes, wire.CodeUnsupportedVersion) && live != "0" && live != dispatch.StateUnknown:
		return UpgradeDrainRequired, "STATE_FORMAT", "the new build refuses this ledger while it records live workers: " + warning, false
	case slices.Contains(res.Codes, wire.CodeUnsupportedVersion):
		return UpgradeRefused, "STATE_FORMAT", "the new build refuses this ledger: " + warning, false
	default:
		return UpgradeUnknown, "PROBE_FAILED", "the new build's dispatch status failed " + slicesText(res.Codes) + ": " + warning, false
	}
}

func slicesText(codes []string) string {
	raw, _ := json.Marshal(codes)
	return string(raw)
}

// limitedWriter keeps at most n bytes and discards the rest.
type limitedWriter struct {
	w io.Writer
	n int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.n > 0 {
		k := min(len(p), l.n)
		if _, err := l.w.Write(p[:k]); err != nil {
			return 0, err
		}
		l.n -= k
	}
	return len(p), nil
}
