package service

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Manager runs one bounded user-manager command. Implementations never
// receive argv from a journal: callers recompute it from a decoded manifest.
type Manager interface {
	Run(ctx context.Context, argv []string) (exit int, out []byte, err error)
}

const (
	managerTimeout = 30 * time.Second
	managerOutput  = 64 * wire.KiB
)

// ExecManager executes launchctl or systemctl from fixed system paths with
// a minimal environment, a 30s timeout and bounded output.
type ExecManager struct{}

func managerBinary(name string) (string, error) {
	var candidates []string
	switch name {
	case "launchctl":
		candidates = []string{"/bin/launchctl"}
	case "systemctl":
		candidates = []string{"/usr/bin/systemctl", "/bin/systemctl"}
	default:
		return "", wire.Errorf(wire.CodeUnsupported, "/manager", "unsupported manager command")
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.Mode().IsRegular() && fi.Mode()&0o111 != 0 {
			return c, nil
		}
	}
	return "", wire.Errorf(wire.CodeCapabilityUnavailable, "/manager", "%s is not installed at a fixed system path", name)
}

type boundedBuffer struct {
	bytes.Buffer
	max int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if room := b.max - b.Len(); room > 0 {
		if len(p) > room {
			b.Buffer.Write(p[:room])
		} else {
			b.Buffer.Write(p)
		}
	}
	return len(p), nil
}

func (ExecManager) Run(ctx context.Context, argv []string) (int, []byte, error) {
	if len(argv) == 0 {
		return -1, nil, wire.Errorf(wire.CodeMalformed, "/manager", "empty manager argv")
	}
	bin, err := managerBinary(argv[0])
	if err != nil {
		return -1, nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, managerTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, argv[1:]...)
	env := []string{"LANG=C", "PATH=/usr/bin:/bin:/usr/sbin:/sbin"}
	for _, k := range []string{"HOME", "USER", "XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	cmd.Env = env
	out := &boundedBuffer{max: managerOutput}
	cmd.Stdout, cmd.Stderr = out, out
	err = cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) && ctx.Err() == nil {
		return exit.ExitCode(), out.Bytes(), nil
	}
	if err != nil {
		return -1, out.Bytes(), wire.Errorf(wire.CodeUncertainEffect, "/manager", "%s did not complete: %v", argv[0], err)
	}
	return 0, out.Bytes(), nil
}

// Registration states observed from the user manager.
const (
	regPresent = "PRESENT"
	regAbsent  = "ABSENT"
	regUnknown = "UNKNOWN"
)

// launchctl print exits 113 for a label absent from the domain.
const launchdAbsentExit = 113

func (h Host) run(argv []string) (int, []byte, error) {
	if h.Manager == nil {
		return -1, nil, wire.Errorf(wire.CodeCapabilityUnavailable, "/manager", "no user manager is configured")
	}
	return h.Manager.Run(context.Background(), argv)
}

// reachable proves the user manager domain answers before any effect.
func (h Host) reachable(m *Manifest) bool {
	argv := []string{"launchctl", "print", m.Domain}
	if m.Manager == "systemd-user" {
		argv = []string{"systemctl", "--user", "show", "-p", "Version"}
	}
	exit, _, err := h.run(argv)
	return err == nil && exit == 0
}

// query observes one label's registration in the bound domain. Anything
// the manager does not state exactly is UNKNOWN.
func (h Host) query(m *Manifest, u Unit) string {
	if m.Manager == "launchd" {
		exit, _, err := h.run([]string{"launchctl", "print", m.Domain + "/" + u.Label})
		switch {
		case err != nil:
			return regUnknown
		case exit == 0:
			return regPresent
		case exit == launchdAbsentExit:
			return regAbsent
		}
		return regUnknown
	}
	exit, out, err := h.run([]string{"systemctl", "--user", "show", "-p", "LoadState", "-p", "ActiveState", "-p", "UnitFileState", "-p", "FragmentPath", u.Label + ".service"})
	if err != nil || exit != 0 {
		return regUnknown
	}
	props := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			props[k] = v
		}
	}
	switch {
	case props["LoadState"] == "not-found":
		return regAbsent
	case props["UnitFileState"] == "enabled" && props["FragmentPath"] == u.Path:
		return regPresent
	case props["UnitFileState"] == "disabled" && (props["ActiveState"] == "inactive" || props["ActiveState"] == "failed"):
		return regAbsent
	}
	return regUnknown
}

// await polls the registration a bounded number of times for want.
func (h Host) await(m *Manifest, u Unit, want string) bool {
	for i := 0; i < 5; i++ {
		if h.query(m, u) == want {
			return true
		}
		h.sleep(200 * time.Millisecond)
	}
	return false
}

// effect runs the manifest-derived register/unregister command for u.
func (h Host) effect(m *Manifest, u Unit, remove bool) error {
	if m.Manager == "systemd-user" && !remove {
		if exit, _, err := h.run([]string{"systemctl", "--user", "daemon-reload"}); err != nil || exit != 0 {
			return wire.Errorf(wire.CodeUncertainEffect, "/manager", "systemctl --user daemon-reload failed")
		}
	}
	a := managerAction(*m, u, remove)
	exit, out, err := h.run(a.Argv)
	if err != nil {
		return err
	}
	if exit != 0 {
		return wire.Errorf(wire.CodeUncertainEffect, "/manager", "%s exited %d: %s", strings.Join(a.Argv, " "), exit, strings.TrimSpace(string(out)))
	}
	if m.Manager == "systemd-user" && remove {
		if exit, _, err := h.run([]string{"systemctl", "--user", "daemon-reload"}); err != nil || exit != 0 {
			return wire.Errorf(wire.CodeUncertainEffect, "/manager", "systemctl --user daemon-reload failed")
		}
	}
	return nil
}
