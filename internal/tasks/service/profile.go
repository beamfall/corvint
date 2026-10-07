// Package service is the experimental, opt-in taskman-user-service/0 profile.
// profile.go and render.go are pure models over supplied facts. store.go,
// observe.go, manager.go, lifecycle.go and run.go are its local runtime: the
// private per-user registry, descriptor-anchored observations, the original
// request operation journal, bounded launchctl/systemctl calls behind the
// Manager interface, and the foreground managed main. Nothing here is
// enabled by ordinary dispatch; platform qualification is separate evidence.
package service

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

const ProfileName = "taskman-user-service/0"
const ManifestName = "taskman-user-service-manifest/0"

var serviceName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)
var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Helper struct {
	ID, Cwd                     string
	Argv                        []string
	Env                         map[string]string
	Foreground, StopWithProgram bool
}
type Profile struct {
	Executable, DispatchConfig, WorkRoot string
	LegacyStopFile                       *string
	Helpers                              []Helper
}

func serviceText(s string) error {
	if _, err := wire.ParseProse("/", s, 1, 4096); err != nil {
		return err
	}
	for _, c := range s {
		if unicode.IsControl(c) {
			return fmt.Errorf("control character")
		}
	}
	return nil
}
func servicePath(s string) error {
	if err := serviceText(s); err != nil {
		return err
	}
	if !filepath.IsAbs(s) || filepath.Clean(s) != s {
		return fmt.Errorf("absolute normalized path required")
	}
	return nil
}
func under(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// temporaryRoots are host temporary or volatile roots: macOS per-user
// temporary folders and Linux tmpfs/runtime directories included.
var temporaryRoots = []string{"/tmp", "/private/tmp", "/var/tmp", "/private/var/tmp", "/var/folders", "/private/var/folders", "/dev/shm", "/run/user"}

func nonTemporary(s string) bool {
	for _, root := range temporaryRoots {
		if under(s, root) {
			return false
		}
	}
	return true
}
func (p Profile) Validate() error {
	for _, v := range []string{p.Executable, p.DispatchConfig, p.WorkRoot} {
		if err := servicePath(v); err != nil {
			return err
		}
		if !nonTemporary(v) {
			return fmt.Errorf("temporary production path")
		}
	}
	if p.LegacyStopFile != nil {
		if err := servicePath(*p.LegacyStopFile); err != nil {
			return err
		}
		if !nonTemporary(*p.LegacyStopFile) {
			return fmt.Errorf("temporary legacy path")
		}
	}
	if len(p.Helpers) > 8 {
		return fmt.Errorf("helper capacity")
	}
	seen := map[string]bool{}
	total := 0
	for _, h := range p.Helpers {
		if !serviceName.MatchString(h.ID) || seen[h.ID] || !h.Foreground || !h.StopWithProgram {
			return fmt.Errorf("helper identity/foreground/stop contract")
		}
		seen[h.ID] = true
		if err := servicePath(h.Cwd); err != nil {
			return err
		}
		if !nonTemporary(h.Cwd) || len(h.Argv) == 0 || len(h.Argv) > 64 || len(h.Env) > 64 {
			return fmt.Errorf("helper bounds")
		}
		if err := servicePath(h.Argv[0]); err != nil {
			return err
		}
		if !nonTemporary(h.Argv[0]) {
			return fmt.Errorf("temporary helper executable")
		}
		for _, a := range h.Argv {
			if err := serviceText(a); err != nil {
				return err
			}
			total += len(a)
		}
		for k, v := range h.Env {
			if !envName.MatchString(k) {
				return fmt.Errorf("environment key")
			}
			if v != "" {
				if err := serviceText(v); err != nil {
					return err
				}
			}
			total += len(k) + len(v)
		}
	}
	if total > 16*wire.KiB {
		return fmt.Errorf("helper argv/environment exceeds16KiB")
	}
	return nil
}
func (p Profile) Encode() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	hs := []wire.Value{}
	for _, h := range p.Helpers {
		env := wire.NewObject()
		for k, v := range h.Env {
			env.Set(k, wire.String(v))
		}
		hs = append(hs, wire.ObjectValue(wire.NewObject().Set("id", wire.String(h.ID)).Set("argv", wire.Strings(h.Argv)).Set("env", wire.ObjectValue(env)).Set("cwd", wire.String(h.Cwd)).Set("foreground", wire.Bool(h.Foreground)).Set("stopWithProgram", wire.Bool(h.StopWithProgram))))
	}
	raw := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("profile", wire.String(ProfileName)).Set("executable", wire.String(p.Executable)).Set("dispatchConfig", wire.String(p.DispatchConfig)).Set("workRoot", wire.String(p.WorkRoot)).Set("legacyStopFile", wire.StringOrNull(p.LegacyStopFile)).Set("helpers", wire.Array(hs...))))
	if len(raw) > 64*wire.KiB {
		return nil, fmt.Errorf("profile exceeds64KiB")
	}
	return raw, nil
}
func DecodeProfile(raw []byte) (*Profile, error) {
	if len(raw) > 64*wire.KiB {
		return nil, fmt.Errorf("profile exceeds64KiB")
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	if err := wire.ProfileVersion("/profile", v, ProfileName); err != nil {
		return nil, err
	}
	r.Closed("profile", "executable", "dispatchConfig", "workRoot", "legacyStopFile", "helpers")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err = wire.CheckProfile("/profile", r.Field("profile").String(), ProfileName); err != nil {
		return nil, err
	}
	p := &Profile{Executable: r.Field("executable").String(), DispatchConfig: r.Field("dispatchConfig").String(), WorkRoot: r.Field("workRoot").String(), LegacyStopFile: r.Field("legacyStopFile").StringOrNull((*wire.Reader).String), Helpers: []Helper{}}
	for _, x := range r.Field("helpers").Array(8, true) {
		x.Closed("id", "argv", "env", "cwd", "foreground", "stopWithProgram")
		h := Helper{ID: x.Field("id").String(), Cwd: x.Field("cwd").String(), Foreground: x.Field("foreground").Bool(), StopWithProgram: x.Field("stopWithProgram").Bool(), Argv: []string{}, Env: map[string]string{}}
		for _, a := range x.Field("argv").Array(64, true) {
			h.Argv = append(h.Argv, a.String())
		}
		ev := x.Field("env")
		value := ev.Value()
		if value.Kind != wire.KindObject || value.Obj == nil {
			ev.Fail(wire.CodeMalformed, "environment object required")
		} else {
			if len(value.Obj.Keys) > 64 {
				ev.Fail(wire.CodeLimitExceeded, "environment capacity")
			}
			for _, k := range value.Obj.Keys {
				h.Env[k] = ev.Field(k).String()
			}
		}
		p.Helpers = append(p.Helpers, h)
	}
	if err = r.Err(); err != nil {
		return nil, err
	}
	if err = p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// FileFacts are supplied resolved observations; validation never stats/chowns.
type FileFacts struct {
	State, Path, DeclaredPath                                string
	Owner                                                    wire.Size
	Mode                                                     uint32
	Regular, Executable, SafeAncestors, NoReplacementSymlink bool
	Sha256                                                   wire.Digest
}
type InstallationFacts struct {
	State, Manager, Domain, Program, QueueID, ConfigRoot, StateRoot, UnitRoot, ManifestPath, DispatchStateRoot, CanonicalStore string
	UID, Generation                                                                                                            wire.Size
	Previous                                                                                                                   *wire.Digest
	Executable, DispatchConfig                                                                                                 FileFacts
	RootsSafe, ManagerReachable                                                                                                bool
	ConfigWorkRoot, ConfigStateRoot                                                                                            string
	HelperExecutables                                                                                                          map[string]FileFacts
}
type Unit struct {
	Label, HelperID, Path string
	Raw                   []byte
}
type Manifest struct {
	Program, Manager, Domain, QueueID, ConfigRoot, StateRoot, UnitRoot, Path, DispatchStateRoot, CanonicalStore, Executable string
	UID, Generation                                                                                                         wire.Size
	Previous                                                                                                                *wire.Digest
	Namespace, ProfileSha256, ExecutableSha256, DispatchConfigSha256                                                        wire.Digest
	Units                                                                                                                   []Unit
	ProfileRaw                                                                                                              []byte
	HelperExecutables                                                                                                       map[string]FileFacts
}

func BuildManifest(p Profile, f InstallationFacts) (*Manifest, error) {
	raw, err := p.Encode()
	if err != nil {
		return nil, err
	}
	if f.State != "VERIFIED" || !f.RootsSafe || !f.ManagerReachable || !serviceName.MatchString(f.Program) {
		return nil, fmt.Errorf("installation facts unknown/foreign")
	}
	if _, err = wire.ParseSize("/uid", string(f.UID)); err != nil {
		return nil, err
	}
	if _, err = wire.ParseSize("/generation", string(f.Generation)); err != nil || f.Generation == "0" {
		return nil, fmt.Errorf("generation invalid")
	}
	if _, err = wire.ParseQueueID("/queueId", f.QueueID); err != nil {
		return nil, err
	}
	if f.Previous != nil {
		if _, err = wire.ParseDigest("/previous", string(*f.Previous)); err != nil {
			return nil, err
		}
	}
	for _, x := range []string{f.ConfigRoot, f.StateRoot, f.UnitRoot, f.ManifestPath, f.DispatchStateRoot, f.CanonicalStore} {
		if err = servicePath(x); err != nil || !nonTemporary(x) {
			return nil, fmt.Errorf("unsafe installation path")
		}
	}
	if !under(f.ManifestPath, f.StateRoot) || !under(p.DispatchConfig, f.ConfigRoot) || f.CanonicalStore != p.WorkRoot || f.ConfigWorkRoot != p.WorkRoot || f.ConfigStateRoot != f.DispatchStateRoot {
		return nil, fmt.Errorf("root/store mismatch")
	}
	if f.Manager == "launchd" {
		if f.Domain != "gui/"+string(f.UID) {
			return nil, fmt.Errorf("user GUI domain required")
		}
	} else if f.Manager == "systemd-user" {
		if f.Domain != "user/"+string(f.UID) {
			return nil, fmt.Errorf("user systemd domain required")
		}
	} else {
		return nil, fmt.Errorf("unsupported manager")
	}
	for i, x := range []FileFacts{f.Executable, f.DispatchConfig} {
		if x.State != "VERIFIED" || !x.Regular || !x.SafeAncestors || !x.NoReplacementSymlink || x.Mode&022 != 0 || !(x.Owner == f.UID || x.Owner == "0") {
			return nil, fmt.Errorf("file ownership/identity unknown")
		}
		if err = servicePath(x.Path); err != nil || !nonTemporary(x.Path) {
			return nil, fmt.Errorf("resolved file path")
		}
		if _, err = wire.ParseDigest("/sha256", string(x.Sha256)); err != nil {
			return nil, err
		}
		if i == 0 && x.DeclaredPath != p.Executable {
			return nil, fmt.Errorf("declared executable binding")
		}
		if i == 1 && x.DeclaredPath != p.DispatchConfig {
			return nil, fmt.Errorf("declared config binding")
		}
		if i == 0 && !x.Executable {
			return nil, fmt.Errorf("resolved executable required")
		}
		if i == 1 && x.Path != p.DispatchConfig {
			return nil, fmt.Errorf("config snapshot differs")
		}
	}
	if len(f.HelperExecutables) != len(p.Helpers) {
		return nil, fmt.Errorf("helper executable observations incomplete")
	}
	for _, h := range p.Helpers {
		x, ok := f.HelperExecutables[h.ID]
		if !ok || x.State != "VERIFIED" || x.DeclaredPath != h.Argv[0] || !x.Regular || !x.Executable || !x.SafeAncestors || !x.NoReplacementSymlink || x.Mode&022 != 0 || (x.Owner != f.UID && x.Owner != "0") {
			return nil, fmt.Errorf("helper executable ownership")
		}
		if err = servicePath(x.Path); err != nil || !nonTemporary(x.Path) {
			return nil, fmt.Errorf("helper resolved path")
		}
		if _, err = wire.ParseDigest("/helperSha256", string(x.Sha256)); err != nil {
			return nil, err
		}
	}
	m := &Manifest{HelperExecutables: map[string]FileFacts{}, ProfileRaw: append([]byte(nil), raw...), Program: f.Program, Manager: f.Manager, Domain: f.Domain, UID: f.UID, Generation: f.Generation, Previous: f.Previous, QueueID: f.QueueID, ConfigRoot: f.ConfigRoot, StateRoot: f.StateRoot, UnitRoot: f.UnitRoot, Path: f.ManifestPath, DispatchStateRoot: f.DispatchStateRoot, CanonicalStore: f.CanonicalStore, Executable: f.Executable.Path, ProfileSha256: wire.Sum(raw), ExecutableSha256: f.Executable.Sha256, DispatchConfigSha256: f.DispatchConfig.Sha256}
	for id, fact := range f.HelperExecutables {
		m.HelperExecutables[id] = fact
	}
	m.Namespace = manifestNamespace(*m)
	m.Units, err = RenderUnits(*m, p)
	if err != nil {
		return nil, err
	}
	if _, err = m.Encode(); err != nil {
		return nil, err
	}
	return m, nil
}
func (m Manifest) Encode() ([]byte, error) {
	if !serviceName.MatchString(m.Program) || m.Generation == "0" {
		return nil, fmt.Errorf("manifest identity")
	}
	if _, err := wire.ParseSize("/uid", string(m.UID)); err != nil {
		return nil, err
	}
	if _, err := wire.ParseSize("/generation", string(m.Generation)); err != nil {
		return nil, err
	}
	if _, err := wire.ParseQueueID("/queue", m.QueueID); err != nil {
		return nil, err
	}
	for _, d := range []wire.Digest{m.Namespace, m.ProfileSha256, m.ExecutableSha256, m.DispatchConfigSha256} {
		if _, err := wire.ParseDigest("/digest", string(d)); err != nil {
			return nil, err
		}
	}
	if m.Previous != nil {
		if _, err := wire.ParseDigest("/previous", string(*m.Previous)); err != nil {
			return nil, err
		}
	}
	for _, path := range []string{m.Path, m.ConfigRoot, m.StateRoot, m.UnitRoot, m.DispatchStateRoot, m.CanonicalStore, m.Executable} {
		if err := servicePath(path); err != nil || !nonTemporary(path) {
			return nil, fmt.Errorf("manifest path")
		}
	}
	if !under(m.Path, m.StateRoot) || (m.Manager == "launchd" && m.Domain != "gui/"+string(m.UID)) || (m.Manager == "systemd-user" && m.Domain != "user/"+string(m.UID)) || (m.Manager != "launchd" && m.Manager != "systemd-user") {
		return nil, fmt.Errorf("manifest user domain/root")
	}
	if m.Namespace != manifestNamespace(m) {
		return nil, fmt.Errorf("manifest namespace identity")
	}
	cfg, err := DecodeProfile(m.ProfileRaw)
	if err != nil {
		return nil, err
	}
	again, err := cfg.Encode()
	if err != nil || !bytesEqual(m.ProfileRaw, again) || wire.Sum(m.ProfileRaw) != m.ProfileSha256 {
		return nil, fmt.Errorf("manifest canonical profile binding")
	}
	expected, err := RenderUnits(m, *cfg)
	if err != nil || len(expected) != len(m.Units) {
		return nil, fmt.Errorf("manifest unit rendering")
	}
	for i, u := range expected {
		v := m.Units[i]
		if u.Label != v.Label || u.HelperID != v.HelperID || u.Path != v.Path || !bytesEqual(u.Raw, v.Raw) {
			return nil, fmt.Errorf("manifest unit bytes/identity")
		}
	}
	if cfg.WorkRoot != m.CanonicalStore {
		return nil, fmt.Errorf("manifest config/store")
	}
	value, err := wire.Parse(m.ProfileRaw)
	if err != nil {
		return nil, err
	}

	if len(m.Units) < 1 || len(m.Units) > 9 {
		return nil, fmt.Errorf("unit capacity")
	}
	units := []wire.Value{}
	seen := map[string]bool{}
	for _, u := range m.Units {
		if seen[u.Label] || len(u.Raw) == 0 || len(u.Raw) > 64*wire.KiB || !under(u.Path, m.UnitRoot) {
			return nil, fmt.Errorf("invalid unit")
		}
		seen[u.Label] = true
		units = append(units, wire.ObjectValue(wire.NewObject().Set("label", wire.String(u.Label)).Set("helperId", wire.String(u.HelperID)).Set("path", wire.String(u.Path)).Set("sha256", wire.String(string(wire.Sum(u.Raw))))))
	}
	helperFacts := []wire.Value{}
	if len(m.HelperExecutables) != len(cfg.Helpers) {
		return nil, fmt.Errorf("manifest helper identity count")
	}
	for _, h := range cfg.Helpers {
		x, ok := m.HelperExecutables[h.ID]
		if !ok || x.DeclaredPath != h.Argv[0] || x.State != "VERIFIED" || !x.Regular || !x.Executable || !x.SafeAncestors || !x.NoReplacementSymlink || x.Mode&022 != 0 || (x.Owner != m.UID && x.Owner != "0") {
			return nil, fmt.Errorf("manifest helper identity")
		}
		if _, err := wire.ParseDigest("/sha256", string(x.Sha256)); err != nil {
			return nil, err
		}
		if err := servicePath(x.Path); err != nil || !nonTemporary(x.Path) {
			return nil, fmt.Errorf("manifest helper path")
		}
		helperFacts = append(helperFacts, wire.ObjectValue(wire.NewObject().Set("id", wire.String(h.ID)).Set("declaredPath", wire.String(x.DeclaredPath)).Set("resolvedPath", wire.String(x.Path)).Set("sha256", wire.String(string(x.Sha256)))))
	}
	o := wire.NewObject().Set("helperExecutables", wire.Array(helperFacts...)).Set("config", value).Set("profile", wire.String(ManifestName)).Set("program", wire.String(m.Program)).Set("manager", wire.String(m.Manager)).Set("domain", wire.String(m.Domain)).Set("uid", wire.String(string(m.UID))).Set("generation", wire.String(string(m.Generation))).Set("queueId", wire.String(m.QueueID)).Set("store", wire.String(m.CanonicalStore)).Set("configRoot", wire.String(m.ConfigRoot)).Set("stateRoot", wire.String(m.StateRoot)).Set("unitRoot", wire.String(m.UnitRoot)).Set("manifestPath", wire.String(m.Path)).Set("dispatchStateRoot", wire.String(m.DispatchStateRoot)).Set("executable", wire.String(m.Executable)).Set("namespace", wire.String(string(m.Namespace))).Set("profileSha256", wire.String(string(m.ProfileSha256))).Set("executableSha256", wire.String(string(m.ExecutableSha256))).Set("dispatchConfigSha256", wire.String(string(m.DispatchConfigSha256))).Set("units", wire.Array(units...))
	prev := wire.Null()
	if m.Previous != nil {
		prev = wire.String(string(*m.Previous))
	}
	o.Set("previous", prev)
	raw := wire.EncodeFile(wire.ObjectValue(o))
	if len(raw) > 128*wire.KiB {
		return nil, fmt.Errorf("manifest capacity")
	}
	return raw, nil
}

func bytesEqual(a, b []byte) bool { return string(a) == string(b) }

func manifestNamespace(m Manifest) wire.Digest {
	seed := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("program", wire.String(m.Program)).Set("uid", wire.String(string(m.UID))).Set("queue", wire.String(m.QueueID)).Set("store", wire.String(m.CanonicalStore)).Set("stateRoot", wire.String(m.StateRoot)).Set("configRoot", wire.String(m.ConfigRoot)).Set("unitRoot", wire.String(m.UnitRoot)).Set("manifest", wire.String(m.Path)).Set("domain", wire.String(m.Domain)).Set("executable", wire.String(m.Executable)).Set("executableSha256", wire.String(string(m.ExecutableSha256))).Set("dispatchConfigSha256", wire.String(string(m.DispatchConfigSha256))).Set("profileSha256", wire.String(string(m.ProfileSha256)))))
	return wire.Sum(seed)
}
