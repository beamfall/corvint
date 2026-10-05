package service

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// fenceFile is F, the short shared control/launch fence (SERVICE500-003).
// It is held only across one control read-modify-write or one admitted
// launch (final control read, spawn and the saved worker record), never
// for a dispatcher lifetime or tick. The lock order is O→F and L→F; F is
// never held while taking O or L.
const fenceFile = "fence.lock"

// fence takes F with a bounded wait. Its holders never wait on anything
// slow, so the wait uses wall time rather than the injectable Host.Sleep.
func (h Host) fence(root string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(root, fenceFile), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	for i := 0; ; i++ {
		if tryLock(f) == nil {
			return func() { f.Close() }, nil
		}
		if i == 500 {
			f.Close()
			return nil, wire.Errorf(wire.CodeLockTimeout, root, "the control fence is held")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// legacyPresence observes the configured legacy stop file: an owned
// regular file is PRESENT and verified ENOENT is ABSENT, both only when
// the parent is a safe directory whose identity did not change across the
// observation; anything else (a symlink, another owner, an error, parent
// drift) is UNKNOWN.
func (h Host) legacyPresence(path string) string {
	parent := filepath.Dir(path)
	before, err := os.Lstat(parent)
	if err != nil || !before.IsDir() || !h.safeAncestors(parent) {
		return LegacyPresence(LegacyObservation{})
	}
	state := "ERROR"
	fi, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		state = "ENOENT"
	case err == nil && fi.Mode().IsRegular():
		if uid, ok := fileOwner(fi); ok && int(uid) == h.UID {
			state = "OWNED_REGULAR"
		}
	}
	after, err := os.Lstat(parent)
	return LegacyPresence(LegacyObservation{ParentVerified: err == nil && os.SameFile(before, after), FileState: state})
}

// legacyPath is the manifest profile's optional legacy stop file.
func legacyPath(m *Manifest) (*string, error) {
	p, err := DecodeProfile(m.ProfileRaw)
	if err != nil {
		return nil, err
	}
	return p.LegacyStopFile, nil
}

var errAdmission = errors.New("launch admission refused")

// intentFile is the durable pre-spawn launch intent (SERVICE500-003): it
// names the admitted worker or pool sweep and the dispatcher instance that
// admitted it, and is written under F before the effect starts. It is
// removed only once the effect's outcome is durable (in the saved ledger,
// or nothing started); while it exists no launch is admitted, a stop is not
// acknowledged and a drain does not settle.
const intentFile = "launch-intent.json"

const maxIntent = 4096

type launchIntent struct {
	Intent string `json:"intent"`
	Token  string `json:"token"`
}

// readIntent reads the launch intent marker: nil when verified absent.
func (h Host) readIntent(root string) (*launchIntent, []byte, error) {
	raw, err := h.readPrivate(filepath.Join(root, intentFile), maxIntent)
	if absent(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var li launchIntent
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&li); err != nil || li.Intent == "" || li.Token == "" {
		return nil, nil, wire.Errorf(wire.CodeUncertainEffect, "/"+intentFile, "launch intent is unreadable")
	}
	return &li, raw, nil
}

// intentState is the marker as Status and Stop report it: ABSENT,
// UNRESOLVED (an admitted effect whose outcome is not yet durable) or
// UNKNOWN.
func (h Host) intentState(root string) string {
	li, _, err := h.readIntent(root)
	switch {
	case err != nil:
		return "UNKNOWN"
	case li == nil:
		return "ABSENT"
	}
	return "UNRESOLVED"
}

// launchControl is the managed main's dispatch.LaunchControl for one opened
// dispatcher, identified by its random token.
type launchControl struct {
	o      RunOptions
	root   string
	ident  wire.Digest
	legacy *string
	token  string
}

func (o RunOptions) control(root string, ident wire.Digest, legacy *string) (*launchControl, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	return &launchControl{o: o, root: root, ident: ident, legacy: legacy, token: hex.EncodeToString(nonce[:])}, nil
}

// Admit takes F and admits one effect only while control is RUNNING and
// bound, no launch intent is unresolved and any legacy stop file is ABSENT.
// A PRESENT legacy stop file latches RUNNING R to DRAINING R+1 under the
// same F. An admitted effect's intent is durable before Admit returns, and
// F is kept until the dispatcher releases it.
func (l *launchControl) Admit(intent string) (func(bool), error) {
	unlock, err := l.o.fence(l.root)
	if err != nil {
		return nil, err
	}
	refuse := func() (func(bool), error) {
		unlock()
		return nil, errAdmission
	}
	if li, _, err := l.o.readIntent(l.root); err != nil || li != nil {
		return refuse()
	}
	c, err := l.o.readControl(l.root)
	if err != nil || c.ManifestIdentity != l.ident || c.Desired != "RUNNING" {
		return refuse()
	}
	if l.legacy != nil {
		if st := l.o.legacyPresence(*l.legacy); st != "ABSENT" {
			if st == "PRESENT" {
				_ = l.o.latchLegacy(l.root, c)
			}
			return refuse()
		}
	}
	raw, err := json.Marshal(launchIntent{Intent: intent, Token: l.token})
	if err != nil || writeAtomic(l.root, intentFile, raw) != nil {
		return refuse()
	}
	sum := wire.Sum(raw)
	return func(recorded bool) {
		if recorded {
			_ = l.o.removeExact(filepath.Join(l.root, intentFile), sum)
		}
		unlock()
	}, nil
}

// Boundary runs between dispatcher ticks under F. A recorded tick resolves
// this dispatcher's own unresolved intent; a settled tick of a bound
// DRAINING control with no unresolved intent saves STOPPED at R+1 and ends
// the dispatcher.
func (l *launchControl) Boundary(recorded, settled bool) bool {
	unlock, err := l.o.fence(l.root)
	if err != nil {
		return false
	}
	defer unlock()
	li, raw, err := l.o.readIntent(l.root)
	if err != nil {
		return false
	}
	if li != nil {
		if !recorded || li.Token != l.token || l.o.removeExact(filepath.Join(l.root, intentFile), wire.Sum(raw)) != nil {
			return false
		}
	}
	if !settled {
		return false
	}
	c, err := l.o.readControl(l.root)
	if err != nil || c.ManifestIdentity != l.ident || c.Desired != "DRAINING" {
		return false
	}
	next := *c
	next.Revision = wire.CountOf(c.Revision.Int() + 1)
	next.Desired = "STOPPED"
	return l.o.writeControl(l.root, next) == nil
}

// latchLegacy saves the legacy stop file's RUNNING R to DRAINING R+1 under
// a held F.
func (h Host) latchLegacy(root string, c *Control) *Control {
	request := "legacy-stop-r" + strconv.FormatInt(c.Revision.Int()+1, 10)
	p := LegacyLatch(*c, "MAIN", request, ControlFacts{FenceHeld: true, Fresh: true, ObservedRevision: c.Revision, Legacy: "PRESENT"})
	if p.Control.Desired != "DRAINING" || h.writeControl(root, p.Control) != nil {
		return nil
	}
	return &p.Control
}

// govern applies the managed main's legacy latch under F for an
// observation that may run: a present legacy stop file latches RUNNING R to
// DRAINING R+1. An UNKNOWN legacy presence holds new opens; an opened
// dispatcher keeps supervising while its control refuses launches. Drain
// settlement is the dispatcher's tick boundary (launchControl.Boundary).
func (o RunOptions) govern(root string, d desiredRun) desiredRun {
	unlock, err := o.fence(root)
	if err != nil {
		d.hold = "control fence: " + describe(err)
		return d
	}
	defer unlock()
	c, err := o.readControl(root)
	if err != nil || c.ManifestIdentity != d.ident {
		d.run, d.hold = false, "control is not bound to the installed manifest"
		return d
	}
	if d.legacy != nil {
		switch st := o.legacyPresence(*d.legacy); {
		case st == "UNKNOWN":
			d.hold = "legacy stop file presence is UNKNOWN"
		case st == "PRESENT" && c.Desired == "RUNNING":
			next := o.latchLegacy(root, c)
			if next == nil {
				d.hold = "legacy latch: control write failed"
				return d
			}
			c = next
		}
	}
	d.desired = c.Desired
	return d
}

// boundAfterOpen re-reads control under F after Open took dispatcher
// ownership (L→F): a suppression saved before then is seen here, and one
// saved later is enforced by the launch control. Holding L, it resolves a
// launch intent left by an earlier dispatcher whose effect the saved ledger
// records; any other left intent stays UNRESOLVED.
func (o RunOptions) boundAfterOpen(root string, ident wire.Digest, c *dispatch.Config) bool {
	unlock, err := o.fence(root)
	if err != nil {
		return false
	}
	defer unlock()
	if li, raw, err := o.readIntent(root); err == nil && li != nil {
		if l, err := dispatch.LoadLedger(dispatch.ProgramDir(c, o.Program), o.Program); err == nil && dispatch.Records(l, li.Intent) {
			_ = o.removeExact(filepath.Join(root, intentFile), wire.Sum(raw))
		}
	}
	ctl, err := o.readControl(root)
	return err == nil && ctl.ManifestIdentity == ident && (ctl.Desired == "RUNNING" || ctl.Desired == "DRAINING")
}

// fencedOwner reports whether the dispatcher owner is this program's
// managed main: the owner lock's live holder is the process of a fresh
// pulse bound to ident. Only the managed main opens a fenced dispatcher.
func (h Host) fencedOwner(root string, m *Manifest, ident wire.Digest) (owner string, fenced bool) {
	owner, pid := dispatch.OwnerProcess(dispatchDir(m))
	if owner != "RUNNING" {
		return owner, false
	}
	st, p := h.pulseState(root, ident)
	return owner, p != nil && p.PID == pid && (st == "RUNNING" || st == "IDLE" || st == "HOLD")
}
