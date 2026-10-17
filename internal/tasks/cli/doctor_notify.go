package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"sort"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The notify hook (TQD-V0-026) runs on --refresh only, under the cache
// lock, when the finding set differs from the last set it was run for and
// at least minIntervalSeconds have passed since its last run. The cache
// records the set it last ran for, when, and its failure; a set it already
// ran for never runs it again, whether that run succeeded or failed.

const doctorNotifyFailed = "NOTIFY_FAILED"

// doctorNotifyState is the cache's notify member.
type doctorNotifyState struct {
	notifiedSet string
	attemptedAt time.Time
	failure     string
}

// doctorFindingSet digests the sorted finding keys; NOTIFY_FAILED is not
// part of the set it reports on.
func doctorFindingSet(findings []doctorFinding) string {
	keys := make([]string, 0, len(findings))
	for _, f := range findings {
		if f.kind != doctorNotifyFailed {
			k := f
			if k.source == "" {
				k.source = "builtin"
			}
			keys = append(keys, k.key())
		}
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// doctorNotifyStateOf reads the notify member of cache bytes; an absent,
// invalid or foreign cache is the state of the empty finding set, so a
// queue with no findings never notifies on its first refresh.
func doctorNotifyStateOf(raw []byte) doctorNotifyState {
	st := doctorNotifyState{notifiedSet: doctorFindingSet(nil)}
	if raw == nil {
		return st
	}
	v, err := decodeDoctorCache(raw)
	if err != nil {
		return st
	}
	n, ok := v.Obj.Get("notify")
	if !ok || n.Kind != wire.KindObject || n.Obj == nil {
		return st
	}
	get := func(k string) wire.Value { x, _ := n.Obj.Get(k); return x }
	if s := get("notifiedSet"); s.Kind == wire.KindString && len(s.Str) == 64 {
		st.notifiedSet = s.Str
	}
	if t, err := time.Parse(time.RFC3339, get("attemptedAt").Str); err == nil {
		st.attemptedAt = t
	}
	if f := get("failure"); f.Kind == wire.KindString {
		st.failure = f.Str
	}
	return st
}

// doctorHasNotify reports whether cache bytes decode with a notify member.
func doctorHasNotify(raw []byte) bool {
	if raw == nil {
		return false
	}
	v, err := decodeDoctorCache(raw)
	if err != nil {
		return false
	}
	n, ok := v.Obj.Get("notify")
	return ok && n.Kind == wire.KindObject && n.Obj != nil
}

func (s doctorNotifyState) value() wire.Value {
	o := wire.NewObject().Set("notifiedSet", wire.String(s.notifiedSet))
	at := wire.Null()
	if !s.attemptedAt.IsZero() {
		at = wire.String(s.attemptedAt.UTC().Format(time.RFC3339))
	}
	fail := wire.Null()
	if s.failure != "" {
		fail = wire.String(prose(s.failure))
	}
	return wire.ObjectValue(o.Set("attemptedAt", at).Set("failure", fail))
}

// doctorNotifyStep decides and, when due, runs the hook with summary on
// stdin; it returns the new state. A failure is recorded, never returned.
func doctorNotifyStep(cfg *doctorNotifyConfig, prev doctorNotifyState, set string, now time.Time, summary []byte, cwd string) doctorNotifyState {
	if set == prev.notifiedSet || (!prev.attemptedAt.IsZero() && now.Sub(prev.attemptedAt) < cfg.minInterval) {
		return prev
	}
	return doctorNotifyState{notifiedSet: set, attemptedAt: now, failure: doctorRunNotify(cfg.argv, summary, cwd)}
}

// doctorRunNotify runs the hook under the TQD-V0-010 plugin bounds: its own
// process group reaped on exit, the plugin timeout, a capped stdout that is
// otherwise discarded, and the primary worktree as working directory. It
// returns the failure text, empty on success.
func doctorRunNotify(argv []string, stdin []byte, cwd string) string {
	ctx, cancel := context.WithTimeout(context.Background(), doctorPluginTimeout)
	defer cancel()
	c := exec.CommandContext(ctx, argv[0], argv[1:]...)
	c.Dir = cwd
	c.Stdin = bytes.NewReader(stdin)
	out := &doctorLimited{max: doctorPluginOutput}
	c.Stdout = out
	c.WaitDelay = time.Second
	err := doctorPluginRun(c)
	switch {
	case ctx.Err() != nil:
		return fmt.Sprintf("notify hook timed out after %s", doctorPluginTimeout)
	case err != nil:
		return "notify hook failed: " + err.Error()
	case out.over:
		return fmt.Sprintf("notify hook output larger than %d bytes", doctorPluginOutput)
	}
	return ""
}

func doctorNotifyFinding(failure string) doctorFinding {
	return doctorFinding{kind: doctorNotifyFailed, who: "notify", detail: failure, carried: true}
}
