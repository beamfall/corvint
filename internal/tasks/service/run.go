package service

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Controller is the in-process dispatcher a managed main runs. Close never
// stops workers: the next dispatcher adopts them.
type Controller interface {
	Run(ctx context.Context, ticks int) error
	Close() error
}

// RunOptions configure the internal `service run` entry point.
type RunOptions struct {
	Host
	Program, Manifest string
	// Executable is this process's resolved executable path.
	Executable string
	Open       func(program string, c *dispatch.Config) (Controller, error)
	// Poll is the control/pin observation interval; Pulse the liveness
	// write interval; Retry the delay after a failed or busy Open.
	Poll, Pulse, Retry time.Duration
	Out                io.Writer
}

// desiredRun is one observation of whether the dispatcher may run.
type desiredRun struct {
	ident  wire.Digest
	config *dispatch.Config
	run    bool
	hold   string
}

type exeStamp struct {
	path        string
	size        int64
	mod         time.Time
	sha         wire.Digest
	dev, inode  uint64
	initialized bool
}

// Run is the foreground managed main. It runs the existing dispatcher
// in-process only while control is RUNNING and bound to the installed
// manifest, and the pinned executable and dispatch config bytes match.
// Any other observation idles or holds without exiting, so the manager's
// keepalive never becomes a restart loop. It returns when ctx ends.
func Run(ctx context.Context, o RunOptions) error {
	root, err := o.StateRoot(o.Program)
	if err != nil {
		return err
	}
	if o.Manifest != filepath.Join(root, manifestFile) {
		return wire.Errorf(wire.CodeUnsupported, "/manifest", "manifest must be the registry manifest %s", filepath.Join(root, manifestFile))
	}
	if o.Poll <= 0 {
		o.Poll = 2 * time.Second
	}
	if o.Pulse <= 0 {
		o.Pulse = 10 * time.Second
	}
	if o.Retry <= 0 {
		o.Retry = 30 * time.Second
	}
	self, _ := supervisor.ProcessIdentity(os.Getpid())
	var (
		ctl      Controller
		cancel   context.CancelFunc
		done     chan error
		runIdent wire.Digest
		retryAt  time.Time
		lastHold string
		stamp    exeStamp
		pulseAt  time.Time
	)
	stop := func() {
		if ctl == nil {
			return
		}
		cancel()
		<-done
		_ = ctl.Close()
		ctl, cancel, done = nil, nil, nil
	}
	defer stop()
	ticker := time.NewTicker(o.Poll)
	defer ticker.Stop()
	for {
		d := o.observe(root, &stamp)
		if ctl != nil {
			select {
			case err := <-done:
				_ = ctl.Close()
				ctl, cancel, done = nil, nil, nil
				d.hold = "dispatcher ended: " + describe(err)
				retryAt = o.now().Add(o.Retry)
			default:
				if !d.run || d.ident != runIdent {
					stop()
				}
			}
		}
		if ctl == nil && d.run && d.hold == "" && !o.now().Before(retryAt) {
			c, err := o.Open(o.Program, d.config)
			if err != nil {
				d.hold = "dispatcher open: " + describe(err)
				retryAt = o.now().Add(o.Retry)
			} else {
				runCtx, cf := context.WithCancel(ctx)
				ctl, cancel, done, runIdent = c, cf, make(chan error, 1), d.ident
				go func(c Controller, ch chan error) { ch <- c.Run(runCtx, 0) }(c, done)
			}
		}
		if d.hold == "" && ctl == nil && d.run {
			d.hold = lastHold
		}
		lastHold = d.hold
		if d.ident != "" && (o.now().Sub(pulseAt) >= o.Pulse || pulseAt.IsZero()) {
			state := "IDLE"
			switch {
			case d.hold != "":
				state = "HOLD"
			case ctl != nil:
				state = "RUNNING"
			}
			p := Pulse{Program: o.Program, ManifestSha256: d.ident, PID: os.Getpid(), Identity: self, State: state, Hold: truncate(d.hold, 1024), At: o.now().Unix()}
			if raw, err := EncodePulse(p); err == nil && writeAtomic(root, pulseFile, raw) == nil {
				pulseAt = o.now()
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// observe reads manifest, control and pins. Any unreadable or drifted
// input yields run=false with a hold reason; STOPPED yields an idle
// observation without a hold.
func (o RunOptions) observe(root string, stamp *exeStamp) desiredRun {
	m, raw, err := o.readManifest(root)
	if err != nil {
		return desiredRun{hold: "manifest: " + describe(err)}
	}
	d := desiredRun{ident: wire.Sum(raw)}
	if m.UID != wire.SizeOf(uint64(o.UID)) || m.Program != o.Program {
		d.hold = "manifest binds another user or program"
		return d
	}
	c, err := o.readControl(root)
	if err != nil || c.ManifestIdentity != d.ident {
		d.hold = "control is not bound to the installed manifest"
		return d
	}
	if c.Desired != "RUNNING" {
		return d
	}
	if sha, err := o.executableSha(stamp); err != nil || sha != m.ExecutableSha256 {
		d.hold = "executable differs from the installed pin"
		return d
	}
	p, err := DecodeProfile(m.ProfileRaw)
	if err != nil {
		d.hold = "profile: " + describe(err)
		return d
	}
	cfgRaw, err := o.readBound(p.DispatchConfig, dispatch.MaxConfig)
	if err != nil || wire.Sum(cfgRaw) != m.DispatchConfigSha256 {
		d.hold = "dispatch config differs from the installed pin"
		return d
	}
	cfg, err := dispatch.DecodeConfig(cfgRaw)
	if err != nil || cfg.StateDir != m.DispatchStateRoot || cfg.WorkRoot != m.CanonicalStore {
		d.hold = "dispatch config does not bind the installed roots"
		return d
	}
	d.config, d.run = cfg, true
	return d
}

// readBound reads a pinned input (owned by the user or root) without
// following a final symlink.
func (o RunOptions) readBound(path string, max int) ([]byte, error) {
	_, raw, err := o.observeFile(path, max)
	return raw, err
}

// executableSha rehashes this process's executable only when its stat
// identity changed since the last observation.
func (o RunOptions) executableSha(s *exeStamp) (wire.Digest, error) {
	fi, err := os.Stat(o.Executable)
	if err != nil {
		return "", err
	}
	dev, inode := fileID(fi)
	if s.initialized && s.path == o.Executable && s.size == fi.Size() && s.mod.Equal(fi.ModTime()) && s.dev == dev && s.inode == inode {
		return s.sha, nil
	}
	f, _, err := o.observeFile(o.Executable, 0)
	if err != nil {
		return "", err
	}
	*s = exeStamp{path: o.Executable, size: fi.Size(), mod: fi.ModTime(), sha: f.Sha256, dev: dev, inode: inode, initialized: true}
	return f.Sha256, nil
}
