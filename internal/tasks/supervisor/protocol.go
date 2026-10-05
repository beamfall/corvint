package supervisor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const MaxCapsule = 1 << 20

type Capsule struct {
	Profile          string   `json:"profile"`
	Effect           string   `json:"effect"`
	Executable       string   `json:"executable"`
	ExecutableSHA256 string   `json:"executableSha256"`
	Argv             []string `json:"argv"`
	Env              []string `json:"env"`
	Directory        string   `json:"directory"`
	Prompt           string   `json:"prompt"`
	// Host selects the result vocabulary (CAL-V0-074); absent is Codex, so
	// Codex capsule bytes are unchanged.
	Host string `json:"host,omitempty"`
}
type Boot struct {
	Effect  string `json:"effect"`
	PID     int    `json:"pid"`
	Started string `json:"started"`
}
type Ack struct {
	Boot     Boot `json:"boot"`
	Accepted bool `json:"accepted"`
}

func Digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func decode(raw []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}
func ReadBounded(path string, max int) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() || st.Size() > int64(max) {
		return nil, fmt.Errorf("nonregular/oversized protocol file")
	}
	b, e := io.ReadAll(io.LimitReader(f, int64(max)+1))
	if len(b) > max {
		return nil, fmt.Errorf("protocol limit")
	}
	return b, e
}

// Publish links a synced private temporary inode into an absent final name.
// A collision never authorizes overwriting a boot or acknowledgment.
func Publish(dir, name string, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	if len(b) > MaxCapsule {
		return fmt.Errorf("protocol bound")
	}
	f, e := os.CreateTemp(dir, ".prepare-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	if e = os.Link(tmp, filepath.Join(dir, name)); e != nil {
		return e
	}
	d, e := os.Open(dir)
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}

// PrelaunchError is a Run refusal made before the lane leader is forked: no
// host process exists or can exist for it. Every other Run error may follow a
// spawn, so only this one lets a caller settle the stage as NO_EXEC
// (CAL-V0-074).
type PrelaunchError struct{ Err error }

func (e *PrelaunchError) Error() string { return "launch refused: " + e.Err.Error() }
func (e *PrelaunchError) Unwrap() error { return e.Err }

// MaxRuntime bounds a pinned runtime's bytes.
const MaxRuntime = 256 << 20

func ValidateCapsule(c Capsule) error {
	_, _, e := validateCapsule(c)
	return e
}

// validateCapsule returns the verified runtime bytes and the status of the
// descriptor they were read from, so the lane leader can execute exactly the
// object it checked.
func validateCapsule(c Capsule) ([]byte, os.FileInfo, error) {
	if c.Profile != "taskman-codex-supervisor/0" || len(c.Effect) != 64 || !filepath.IsAbs(c.Executable) || !filepath.IsAbs(c.Directory) || len(c.Argv) > 64 || len(c.Env) > 64 || len(c.Prompt) > MaxCapsule/2 {
		return nil, nil, fmt.Errorf("capsule profile/bounds")
	}
	host, ok := HostVocabulary(c.Host)
	if !ok || c.Host == HostCodex {
		return nil, nil, fmt.Errorf("capsule host unsupported")
	}
	for _, want := range host.DetachedEnv {
		if !lastEnv(c.Env, want) {
			return nil, nil, fmt.Errorf("detached host environment absent")
		}
	}
	b, st, e := readRuntime(c.Executable)
	if e != nil {
		return nil, nil, e
	}
	if Digest(b) != c.ExecutableSHA256 {
		return nil, nil, fmt.Errorf("runtime executable digest changed")
	}
	return b, st, nil
}

// LaunchableExecutable is the launch-time check on a pinned runtime path:
// an absolute path naming a regular file with an execute bit, never a
// symlink, whose bounded bytes and permission bits it returns. The type,
// mode and bytes all come from one descriptor opened without following a
// final symlink, so no second path lookup can substitute another file.
// Admission and ValidateCapsule share it, so an admitted program is never
// refused at launch for the executable's type (CAL-V0-074).
func LaunchableExecutable(path string) ([]byte, os.FileMode, error) {
	b, st, e := readRuntime(path)
	if e != nil {
		return nil, 0, e
	}
	return b, st.Mode().Perm(), nil
}

func readRuntime(path string) ([]byte, os.FileInfo, error) {
	if !filepath.IsAbs(path) {
		return nil, nil, fmt.Errorf("runtime path is not absolute")
	}
	f, e := openExecutable(path)
	if e != nil {
		// The descriptor refusal decides; the second lookup only names it.
		if st, le := os.Lstat(path); le == nil && st.Mode()&os.ModeSymlink != 0 {
			if target, te := filepath.EvalSymlinks(path); te == nil {
				return nil, nil, fmt.Errorf("runtime %s is a symlink; pin its target %s", path, target)
			}
			return nil, nil, fmt.Errorf("runtime %s is a symlink", path)
		}
		return nil, nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return nil, nil, e
	}
	if !st.Mode().IsRegular() || st.Mode()&0111 == 0 {
		return nil, nil, fmt.Errorf("runtime is not regular executable")
	}
	if st.Size() > MaxRuntime {
		return nil, nil, fmt.Errorf("runtime exceeds %d bytes", MaxRuntime)
	}
	b, e := io.ReadAll(io.LimitReader(f, MaxRuntime+1))
	if e != nil {
		return nil, nil, e
	}
	if len(b) > MaxRuntime {
		return nil, nil, fmt.Errorf("runtime exceeds %d bytes", MaxRuntime)
	}
	return b, st, nil
}

// lastEnv reports whether the effective (last) assignment of want's key in
// env is exactly want.
func lastEnv(env []string, want string) bool {
	key, _, _ := strings.Cut(want, "=")
	got := ""
	for _, kv := range env {
		if k, _, _ := strings.Cut(kv, "="); k == key {
			got = kv
		}
	}
	return got == want
}
func await(ctx context.Context, path string, v any) error {
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		b, e := ReadBounded(path, MaxCapsule)
		if e == nil {
			return decode(b, v)
		}
		if !os.IsNotExist(e) {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

const MaxHostOutput = 16 << 10

type Outcome struct {
	Result                         HostResult
	Class, SessionID, OutputSHA256 string
	Clean                          bool
	Stdout, Stderr                 []byte
	Boot                           Boot
}
type Journal func(phase string, boot Boot, out *Outcome) error

func RecoveryBoot(dir, effect string, retained Boot) (Boot, error) {
	raw, e := ReadBounded(filepath.Join(dir, "boot"), MaxCapsule)
	if e != nil {
		return Boot{}, e
	}
	var boot Boot
	if e = decode(raw, &boot); e != nil {
		return Boot{}, e
	}
	if boot.Effect != effect || boot.PID <= 0 || boot.Started == "" {
		return Boot{}, fmt.Errorf("recovery boot binding differs")
	}
	if retained.PID > 0 && (boot.PID != retained.PID || boot.Started != retained.Started) {
		return Boot{}, fmt.Errorf("recovery leader differs")
	}
	return boot, nil
}
