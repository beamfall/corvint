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
func ValidateCapsule(c Capsule) error {
	if c.Profile != "taskman-codex-supervisor/0" || len(c.Effect) != 64 || !filepath.IsAbs(c.Executable) || !filepath.IsAbs(c.Directory) || len(c.Argv) > 64 || len(c.Env) > 64 || len(c.Prompt) > MaxCapsule/2 {
		return fmt.Errorf("capsule profile/bounds")
	}
	if _, ok := HostVocabulary(c.Host); !ok || c.Host == HostCodex {
		return fmt.Errorf("capsule host unsupported")
	}
	b, _, e := LaunchableExecutable(c.Executable)
	if e != nil {
		return e
	}
	if Digest(b) != c.ExecutableSHA256 {
		return fmt.Errorf("runtime executable digest changed")
	}
	return nil
}

// LaunchableExecutable is the launch-time check on a pinned runtime path:
// an absolute path naming a regular file with an execute bit, never a
// symlink, whose bounded bytes and permission bits it returns. Admission and
// ValidateCapsule share it, so an admitted program is never refused at launch
// for the executable's type (CAL-V0-074).
func LaunchableExecutable(path string) ([]byte, os.FileMode, error) {
	if !filepath.IsAbs(path) {
		return nil, 0, fmt.Errorf("runtime path is not absolute")
	}
	st, e := os.Lstat(path)
	if e != nil {
		return nil, 0, e
	}
	if st.Mode()&os.ModeSymlink != 0 {
		if target, e := filepath.EvalSymlinks(path); e == nil {
			return nil, 0, fmt.Errorf("runtime %s is a symlink; pin its target %s", path, target)
		}
		return nil, 0, fmt.Errorf("runtime %s is a symlink", path)
	}
	if !st.Mode().IsRegular() || st.Mode()&0111 == 0 {
		return nil, 0, fmt.Errorf("runtime is not regular executable")
	}
	b, e := ReadBounded(path, 256<<20)
	if e != nil {
		return nil, 0, e
	}
	return b, st.Mode().Perm(), nil
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
