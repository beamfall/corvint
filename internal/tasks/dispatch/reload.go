package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// ConfigRecord is this run's CAL-V0-127 reload outcome. It exists only after
// the configuration file changed while the dispatcher ran; a restart applies
// the file afresh and drops it.
type ConfigRecord struct {
	AppliedSha256 string         `json:"appliedSha256"`
	AppliedAt     time.Time      `json:"appliedAt"`
	Refused       *ConfigRefusal `json:"refused,omitempty"`
}

// ConfigRefusal is the newest refused file. Sha256 is empty when the file
// could not be read.
type ConfigRefusal struct {
	Sha256 string    `json:"sha256,omitempty"`
	At     time.Time `json:"at"`
	Reason string    `json:"reason"`
}

const maxConfigRefusal = 1024

func validSha256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
}

func (r *ConfigRecord) validate() error {
	if !validSha256Hex(r.AppliedSha256) || r.AppliedAt.IsZero() {
		return errors.New("invalid config record")
	}
	if f := r.Refused; f != nil && ((f.Sha256 != "" && !validSha256Hex(f.Sha256)) || f.At.IsZero() || f.Reason == "" || len(f.Reason) > maxConfigRefusal || !utf8.ValidString(f.Reason)) {
		return errors.New("invalid config refusal")
	}
	return nil
}

// WatchConfig makes each tick re-read the configuration with read before it
// observes the store (CAL-V0-127); initial are the bytes d.Config was decoded
// from. read returns the bytes with the stat of the descriptor it read them
// from, taken before reading. stat, when not nil, describes the named file
// without reading it, so an idle skip (CAL-V0-139) re-reads only after the
// file's stat changed.
func (d *Dispatcher) WatchConfig(read func() ([]byte, fs.FileInfo, error), stat func() (fs.FileInfo, error), initial []byte) {
	sum := sha256.Sum256(initial)
	d.configRead, d.configStat, d.configSha256, d.configAt = read, stat, hex.EncodeToString(sum[:]), d.Now().UTC()
	d.configSeen, d.configSeenSha256 = nil, ""
}

// readConfig reads the configuration file and keeps the bytes' digest with
// the stat of the descriptor they were read from, so a later identical
// stat of the named file can stand for them. A file replaced around the
// read pairs its own stat with its own bytes; a change after the stat only
// makes the next stat differ.
func (d *Dispatcher) readConfig() ([]byte, string, error) {
	d.configSeen, d.configSeenSha256 = nil, ""
	raw, seen, err := d.configRead()
	if err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(raw)
	sum := hex.EncodeToString(digest[:])
	if seen != nil && d.configStat != nil {
		d.configSeen, d.configSeenSha256, d.configSeenAt = seen, sum, d.Now()
	}
	return raw, sum, nil
}

// cachedConfigSum returns the digest of the bytes last read when the named
// file's stat still matches the descriptor they were read from and they were
// read within idleFullEvery (ATR-V0-180, V1-0925). Any stat error, changed
// identity, size, mode or modification time, a clock behind the read, or an
// older read returns false, so the caller reads the file.
func (d *Dispatcher) cachedConfigSum(now time.Time) (string, bool) {
	if d.configStat == nil || d.configSeen == nil || now.Before(d.configSeenAt) || !now.Before(d.configSeenAt.Add(idleFullEvery)) {
		return "", false
	}
	st, err := d.configStat()
	if err != nil || !sameConfigStat(d.configSeen, st) {
		return "", false
	}
	return d.configSeenSha256, true
}

// sameConfigStat reports an unchanged file: same identity, size, mode and
// modification time. A rewrite that keeps all four within the filesystem's
// timestamp granularity is not seen until the next full tick reads the file.
func sameConfigStat(a, b fs.FileInfo) bool {
	return os.SameFile(a, b) && a.Size() == b.Size() && a.Mode() == b.Mode() && a.ModTime().Equal(b.ModTime())
}

// configPending reports whether reloadConfig would act at this tick, so an
// idle skip (CAL-V0-139) cannot delay a CAL-V0-127 reload: the file differs
// from the applied bytes and is not the refusal already recorded, or it
// matches them again after a refusal. An unchanged stat reuses the digest
// of the bytes last read within idleFullEvery and reads nothing; a changed
// stat or an older read reads the file (ATR-V0-180). A read error is
// pending, so the full tick reports it.
func (d *Dispatcher) configPending() bool {
	if d.configRead == nil {
		return false
	}
	sum, same := d.cachedConfigSum(d.Now().UTC())
	if !same {
		var err error
		if _, sum, err = d.readConfig(); err != nil {
			return true
		}
	}
	var refused *ConfigRefusal
	if d.ledger.Config != nil {
		refused = d.ledger.Config.Refused
	}
	if sum == d.configSha256 {
		return refused != nil
	}
	return refused == nil || refused.Sha256 != sum
}

// reloadConfig applies a changed, valid configuration file at the tick
// boundary. An unreadable or invalid file, or one changing stateDir or
// workRoot, is refused once per distinct content: the applied configuration
// stays active and the refusal is recorded for status. Running workers are
// never stopped by a reload; a removed role's workers stay supervised by the
// ledger and only stop launching.
func (d *Dispatcher) reloadConfig() {
	if d.configRead == nil {
		return
	}
	now := d.Now().UTC()
	rec := d.ledger.Config
	// ATR-V0-180: an unchanged stat stands for the bytes last read when they
	// are the applied bytes or the refusal already recorded; anything else
	// is read, decoded and checked as before.
	var raw []byte
	sum, cached := d.cachedConfigSum(now)
	refused := rec != nil && rec.Refused != nil && rec.Refused.Sha256 != "" && rec.Refused.Sha256 == sum
	var err error
	switch {
	case cached && refused:
		return // already refused and reported
	case !cached || sum != d.configSha256:
		raw, sum, err = d.readConfig()
	}
	if err == nil && sum == d.configSha256 {
		if rec != nil && rec.Refused != nil {
			rec.Refused = nil
			d.emit(Event{Kind: "config", Message: "configuration file matches the applied configuration again; the earlier refusal no longer applies", Detail: map[string]string{"outcome": "RESTORED", "sha256": sum}})
		}
		return
	}
	var c *Config
	if err != nil {
		err = fmt.Errorf("read: %w", err)
	} else if c, err = DecodeConfig(raw); err == nil && (c.StateDir != d.Config.StateDir || c.WorkRoot != d.Config.WorkRoot) {
		err = errors.New("stateDir and workRoot cannot change while the dispatcher runs; restart it to apply them")
	}
	if rec == nil {
		rec = &ConfigRecord{AppliedSha256: d.configSha256, AppliedAt: d.configAt}
	}
	if err != nil {
		reason := boundUTF8(err.Error(), maxConfigRefusal)
		if f := rec.Refused; f != nil && f.Sha256 == sum && (sum != "" || f.Reason == reason) {
			return // already refused and reported
		}
		rec.Refused = &ConfigRefusal{Sha256: sum, At: now, Reason: reason}
		d.ledger.Config = rec
		shown := sum
		if shown == "" {
			shown = StateUnknown
		}
		d.emit(Event{Kind: "alert", Message: "configuration change refused; the applied configuration stays active: " + reason, Detail: map[string]string{"config": "REFUSED", "sha256": shown}})
		return
	}
	var removed []string
	for _, r := range d.Config.Roles {
		if !slices.ContainsFunc(c.Roles, func(n Role) bool { return n.Name == r.Name }) {
			removed = append(removed, r.Name)
		}
	}
	under := map[string]*Config{}
	for _, w := range d.ledger.Workers {
		under[w.ID] = d.launchConfig(w)
	}
	d.launchedUnder = under
	d.Config, d.configSha256, d.configAt = c, sum, now
	rec.AppliedSha256, rec.AppliedAt, rec.Refused = sum, now, nil
	d.ledger.Config = rec
	d.reconcileEscalation()
	// CAL-V0-068 across a reload: the recorded level is kept so a reload
	// cannot bypass a throttle, while pending dwell restarts under the new
	// thresholds. Pressure state exists only while configured.
	switch {
	case c.Pressure == nil:
		d.ledger.Pressure = nil
	case d.ledger.Pressure == nil:
		d.ledger.Pressure = &PressureRecord{State: PressureState{Unknown: true}, Held: []HeldLaunch{}}
	default:
		st := &d.ledger.Pressure.State
		st.PendingLevel, st.PendingTicks = st.Level, 0
	}
	d.reconcileInfraRetry()
	msg := "applied the changed configuration at the tick boundary"
	detail := map[string]string{"outcome": "APPLIED", "sha256": sum}
	if len(removed) > 0 {
		msg += fmt.Sprintf("; removed role(s) %s launch nothing further and their %d running worker(s) continue", strings.Join(removed, ", "), d.workersOf(removed))
		detail["removedRoles"] = strings.Join(removed, ",")
	}
	d.emit(Event{Kind: "config", Message: msg, Detail: detail})
	d.warnRoleShadows()
}

// warnRoleShadows appends one CAL-V0-290 `config` warning event per
// shadowed role of the applied configuration, at start and after an applied
// reload. It changes no roster decision.
func (d *Dispatcher) warnRoleShadows() {
	for _, s := range d.Config.RoleShadows() {
		d.emit(Event{Kind: "config", Role: s.Role, Message: s.Message(), Detail: map[string]string{"outcome": "ROLE_SHADOWED", "shadowedBy": s.ShadowedBy}})
	}
}

func (d *Dispatcher) workersOf(roles []string) int {
	n := 0
	for _, w := range d.ledger.Workers {
		if slices.Contains(roles, w.Role) {
			n++
		}
	}
	return n
}

// memberEpisode is when the dispatcher first observed a lane member in one
// state episode (CAL-V0-129).
type memberEpisode struct {
	pool, state, changed string
	since                time.Time
}

// stampMemberAges sets each observed member's Age from the dispatcher clock.
// A changed state or change sequence starts a new episode; members no longer
// observed are forgotten. The clock is in memory, so a restart starts every
// episode again, which only delays a lane launch.
func (d *Dispatcher) stampMemberAges(obs *Observation, now time.Time) {
	next := make(map[string]memberEpisode, len(obs.Members))
	for i := range obs.Members {
		m := &obs.Members[i]
		key := laneKey(m.Pool, m.Member)
		e, ok := d.memberSince[key]
		if !ok || e.state != m.State || e.changed != m.Changed || now.Before(e.since) {
			e = memberEpisode{pool: m.Pool, state: m.State, changed: m.Changed, since: now}
		}
		next[key] = e
		m.Age = now.Sub(e.since)
	}
	d.memberSince = next
}
