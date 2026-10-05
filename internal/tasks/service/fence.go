package service

import (
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

// admitter is the managed main's dispatch.LaunchFence for one opened
// dispatcher: it takes F and admits a launch only while control is RUNNING
// and bound to ident and any legacy stop file is ABSENT. An admitted
// launch keeps F until the dispatcher releases it after recording the
// worker.
func (o RunOptions) admitter(root string, ident wire.Digest, legacy *string) dispatch.LaunchFence {
	return func() (func(), error) {
		unlock, err := o.fence(root)
		if err != nil {
			return nil, err
		}
		c, err := o.readControl(root)
		if err != nil || c.ManifestIdentity != ident || c.Desired != "RUNNING" || (legacy != nil && o.legacyPresence(*legacy) != "ABSENT") {
			unlock()
			return nil, errAdmission
		}
		return unlock, nil
	}
}

// govern applies the managed main's own control transitions under F for
// an observation that may run: a present legacy stop file latches RUNNING
// R to DRAINING R+1, and a DRAINING control whose dispatcher ledger records
// no worker and no pending pool sweep moves to STOPPED. An UNKNOWN legacy
// presence holds new opens; an opened dispatcher keeps supervising while
// its fence refuses launches.
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
			request := "legacy-stop-r" + strconv.FormatInt(c.Revision.Int()+1, 10)
			p := LegacyLatch(*c, "MAIN", request, ControlFacts{FenceHeld: true, Fresh: true, ObservedRevision: c.Revision, Legacy: st})
			if p.Control.Desired == "DRAINING" {
				if err := o.writeControl(root, p.Control); err != nil {
					d.hold = "legacy latch: " + describe(err)
					return d
				}
				c = &p.Control
			}
		}
	}
	d.desired = c.Desired
	if c.Desired == "DRAINING" && drainSettled(d.config, o.Program) {
		next := *c
		next.Revision = wire.CountOf(c.Revision.Int() + 1)
		next.Desired = "STOPPED"
		if err := o.writeControl(root, next); err != nil {
			d.hold = "drain settlement: " + describe(err)
			return d
		}
		d.run, d.desired = false, "STOPPED"
	}
	return d
}

// drainSettled reads the dispatcher's saved ledger: settled means no
// recorded worker and no pending pool sweep. An unreadable ledger is not
// settled.
func drainSettled(c *dispatch.Config, program string) bool {
	l, err := dispatch.LoadLedger(dispatch.ProgramDir(c, program), program)
	return err == nil && dispatch.Settled(l)
}

// boundAfterOpen re-reads control under F after Open took dispatcher
// ownership (L→F): a suppression saved before then is seen here, and one
// saved later is enforced by the launch fence.
func (o RunOptions) boundAfterOpen(root string, ident wire.Digest) bool {
	unlock, err := o.fence(root)
	if err != nil {
		return false
	}
	defer unlock()
	c, err := o.readControl(root)
	return err == nil && c.ManifestIdentity == ident && (c.Desired == "RUNNING" || c.Desired == "DRAINING")
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
