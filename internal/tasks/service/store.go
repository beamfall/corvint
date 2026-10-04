package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Private durable state of one installed program. Every file is closed
// canonical JSON written by checked atomic publication (0600 temporary,
// fsync, rename, directory fsync) under a descriptor-pinned 0700 root.
const (
	ControlName   = "taskman-user-service-control/0"
	PulseName     = "taskman-user-service-pulse/0"
	OperationName = "taskman-user-service-operation/0"
	StatusName    = "taskman-user-service-status/0"

	manifestFile = "manifest.json"
	controlFile  = "control.json"
	pulseFile    = "pulse.json"
	lockFile     = "service.lock"
	operationDir = "operations"

	maxControl    = 4096
	maxPulse      = 4096
	maxManifest   = 128 * wire.KiB
	maxOperation  = 512 * wire.KiB
	maxOperations = 256
	maxHistory    = 16 << 20
)

// Host is the explicit local environment of the service runtime. Tests
// substitute the manager, clock and queue resolution; nothing here reads
// ambient XDG overrides, so a manager-started controller and an operator
// shell resolve the same per-user roots from HOME alone.
type Host struct {
	UID     int
	Home    string
	GOOS    string
	Manager Manager
	Now     func() time.Time
	// QueueID resolves the canonical store's queue identity.
	QueueID func(workRoot string) (string, error)
	// Sleep paces bounded manager readback polls; nil uses time.Sleep.
	Sleep func(time.Duration)
}

func (h Host) home() (string, error) {
	if err := servicePath(h.Home); err != nil {
		return "", wire.Errorf(wire.CodeUnsupported, "/home", "HOME must be an absolute normalized path")
	}
	return h.Home, nil
}

// Manager returns the user manager kind and the bound user domain.
func (h Host) managerDomain() (string, string, error) {
	switch h.GOOS {
	case "darwin":
		return "launchd", "gui/" + strconv.Itoa(h.UID), nil
	case "linux":
		return "systemd-user", "user/" + strconv.Itoa(h.UID), nil
	}
	return "", "", wire.Errorf(wire.CodeUnsupported, "/manager", "the user service supports only darwin launchd and linux systemd --user")
}

// StateRoot is the private per-program registry root, fixed under HOME.
func (h Host) StateRoot(program string) (string, error) {
	if !serviceName.MatchString(program) {
		return "", wire.Errorf(wire.CodeMalformed, "/program", "program name must match [a-z][a-z0-9-]{0,23}")
	}
	home, err := h.home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "corvint-tasks", "service", program), nil
}

func (h Host) unitRoot(manager string) (string, error) {
	home, err := h.home()
	if err != nil {
		return "", err
	}
	if manager == "launchd" {
		return filepath.Join(home, "Library", "LaunchAgents"), nil
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

func (h Host) now() time.Time {
	if h.Now == nil {
		return time.Now()
	}
	return h.Now()
}

func (h Host) sleep(d time.Duration) {
	if h.Sleep == nil {
		time.Sleep(d)
		return
	}
	h.Sleep(d)
}

// privateDir creates (when create is set) and verifies a 0700 directory
// owned by the bound user.
func (h Host) privateDir(path string, create bool) error {
	if create {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	uid, ok := fileOwner(fi)
	if !fi.IsDir() || !ok || int(uid) != h.UID || fi.Mode().Perm()&0o077 != 0 {
		return wire.Errorf(wire.CodeUnsupportedFilesystem, path, "service state directory must be a 0700 directory owned by uid %d", h.UID)
	}
	return nil
}

// readPrivate reads one bounded regular file without following a final
// symlink and requires the bound owner and no group/world write bit.
// fs.ErrNotExist is returned unwrapped-compatible for absence.
func (h Host) readPrivate(path string, max int) ([]byte, error) {
	f, err := safeopen.File(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	uid, ok := fileOwner(fi)
	if !fi.Mode().IsRegular() || !ok || int(uid) != h.UID || fi.Mode().Perm()&0o022 != 0 {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, path, "expected a regular file owned by uid %d without group/world write", h.UID)
	}
	if fi.Size() > int64(max) {
		return nil, wire.Errorf(wire.CodeLimitExceeded, path, "file exceeds %d bytes", max)
	}
	raw, err := io.ReadAll(io.LimitReader(f, int64(max)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > max {
		return nil, wire.Errorf(wire.CodeLimitExceeded, path, "file exceeds %d bytes", max)
	}
	return raw, nil
}

func absent(err error) bool { return errors.Is(err, fs.ErrNotExist) }

// writeAtomic publishes raw at dir/name: exclusive 0600 temporary, fsync,
// rename and directory fsync, all through the pinned directory descriptor.
func writeAtomic(dir, name string, raw []byte) error {
	root, err := safeopen.Root(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	tmp := ".tmp-" + hex.EncodeToString(nonce[:])
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(raw)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = root.Rename(tmp, name)
	}
	if err != nil {
		_ = root.Remove(tmp)
		return err
	}
	d, err := root.Open(".")
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// removeExact removes dir/name only when it is still the owned regular file
// with one of the expected digests; absence is success. The digest is
// checked immediately before the unlink.
func (h Host) removeExact(path string, want ...wire.Digest) error {
	raw, err := h.readPrivate(path, maxManifest)
	if absent(err) {
		return nil
	}
	if err != nil {
		return err
	}
	match := false
	for _, w := range want {
		match = match || wire.Sum(raw) == w
	}
	if !match {
		return wire.Errorf(wire.CodeResourceCollision, path, "file changed since it was published; it is left in place")
	}
	root, err := safeopen.Root(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Remove(filepath.Base(path)); err != nil && !absent(err) {
		return err
	}
	d, err := root.Open(".")
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// lock takes the program's service mutation lock without blocking forever.
func (h Host) lock(root string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(root, lockFile), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	for i := 0; ; i++ {
		err = tryLock(f)
		if err == nil {
			return func() { f.Close() }, nil
		}
		if i == 50 {
			f.Close()
			return nil, wire.Errorf(wire.CodeLockTimeout, root, "another service operation holds the program lock")
		}
		h.sleep(100 * time.Millisecond)
	}
}

func optionalString(s string) wire.Value {
	if s == "" {
		return wire.Null()
	}
	return wire.String(s)
}

// EncodeControl is the closed durable control record (<=4096 bytes).
func EncodeControl(c Control) ([]byte, error) {
	if !validControl(c) || (c.LastRequest == "") != (c.LastRequestSha256 == "") {
		return nil, wire.Errorf(wire.CodeMalformed, "/control", "invalid control record")
	}
	raw := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("profile", wire.String(ControlName)).Set("program", wire.String(c.Program)).Set("manifestSha256", wire.String(string(c.ManifestIdentity))).Set("revision", wire.String(string(c.Revision))).Set("desired", wire.String(c.Desired)).Set("lastRequest", optionalString(c.LastRequest)).Set("lastRequestSha256", optionalString(string(c.LastRequestSha256)))))
	if len(raw) > maxControl {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/control", "control exceeds %d bytes", maxControl)
	}
	return raw, nil
}

func DecodeControl(raw []byte) (Control, error) {
	if len(raw) > maxControl {
		return Control{}, wire.Errorf(wire.CodeLimitExceeded, "/control", "control exceeds %d bytes", maxControl)
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return Control{}, err
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "program", "manifestSha256", "revision", "desired", "lastRequest", "lastRequestSha256")
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ControlName); err != nil {
		return Control{}, err
	}
	c := Control{Program: r.Field("program").String(), ManifestIdentity: r.Field("manifestSha256").Digest(), Revision: r.Field("revision").Count(), Desired: r.Field("desired").Enum("RUNNING", "DRAINING", "STOPPED")}
	if p := r.Field("lastRequest").StringOrNull((*wire.Reader).Identifier); p != nil {
		c.LastRequest = *p
	}
	if d := r.Field("lastRequestSha256").DigestOrNull(); d != nil {
		c.LastRequestSha256 = *d
	}
	if err := r.Err(); err != nil {
		return Control{}, err
	}
	if again, err := EncodeControl(c); err != nil || string(again) != string(raw) {
		return Control{}, wire.Errorf(wire.CodeMalformed, "/control", "control is not canonical")
	}
	return c, nil
}

// Pulse is the controller's liveness record. It is written every 10s and is
// not a completed dispatcher tick.
type Pulse struct {
	Program, Identity, State, Hold string
	ManifestSha256                 wire.Digest
	PID                            int
	At                             int64
}

func EncodePulse(p Pulse) ([]byte, error) {
	if !serviceName.MatchString(p.Program) || p.PID <= 0 || p.At < 0 || serviceText(p.Identity) != nil || (p.State != "RUNNING" && p.State != "IDLE" && p.State != "HOLD") {
		return nil, wire.Errorf(wire.CodeMalformed, "/pulse", "invalid pulse")
	}
	if _, err := wire.ParseDigest("/manifestSha256", string(p.ManifestSha256)); err != nil {
		return nil, err
	}
	raw := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("profile", wire.String(PulseName)).Set("program", wire.String(p.Program)).Set("manifestSha256", wire.String(string(p.ManifestSha256))).Set("pid", wire.String(strconv.Itoa(p.PID))).Set("processIdentity", wire.String(p.Identity)).Set("state", wire.String(p.State)).Set("hold", optionalString(p.Hold)).Set("at", wire.String(strconv.FormatInt(p.At, 10)))))
	if len(raw) > maxPulse {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/pulse", "pulse exceeds %d bytes", maxPulse)
	}
	return raw, nil
}

func DecodePulse(raw []byte) (Pulse, error) {
	if len(raw) > maxPulse {
		return Pulse{}, wire.Errorf(wire.CodeLimitExceeded, "/pulse", "pulse exceeds %d bytes", maxPulse)
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return Pulse{}, err
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "program", "manifestSha256", "pid", "processIdentity", "state", "hold", "at")
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), PulseName); err != nil {
		return Pulse{}, err
	}
	p := Pulse{Program: r.Field("program").String(), ManifestSha256: r.Field("manifestSha256").Digest(), Identity: r.Field("processIdentity").String(), State: r.Field("state").String()}
	if h := r.Field("hold").StringOrNull((*wire.Reader).String); h != nil {
		p.Hold = *h
	}
	pid, perr := strconv.Atoi(r.Field("pid").String())
	at, aerr := strconv.ParseInt(r.Field("at").String(), 10, 64)
	if err := r.Err(); err != nil {
		return Pulse{}, err
	}
	p.PID, p.At = pid, at
	if perr != nil || aerr != nil {
		return Pulse{}, wire.Errorf(wire.CodeMalformed, "/pulse", "pid/at must be decimal")
	}
	if again, err := EncodePulse(p); err != nil || string(again) != string(raw) {
		return Pulse{}, wire.Errorf(wire.CodeMalformed, "/pulse", "pulse is not canonical")
	}
	return p, nil
}

// DecodeManifest decodes the closed manifest and proves it canonical by
// re-rendering every unit from the embedded profile and re-encoding.
func DecodeManifest(raw []byte) (*Manifest, error) {
	if len(raw) > maxManifest {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/manifest", "manifest exceeds %d bytes", maxManifest)
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed("helperExecutables", "config", "profile", "program", "manager", "domain", "uid", "generation", "queueId", "store", "configRoot", "stateRoot", "unitRoot", "manifestPath", "dispatchStateRoot", "executable", "namespace", "profileSha256", "executableSha256", "dispatchConfigSha256", "units", "previous")
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ManifestName); err != nil {
		return nil, err
	}
	if len(r.Field("helperExecutables").Array(8, false)) != 0 {
		return nil, wire.Errorf(wire.CodeUnsupported, "/helperExecutables", "helper units are not supported by this service runtime")
	}
	m := &Manifest{HelperExecutables: map[string]FileFacts{}, Program: r.Field("program").String(), Manager: r.Field("manager").String(), Domain: r.Field("domain").String(), UID: r.Field("uid").Size(), Generation: r.Field("generation").Size(), QueueID: r.Field("queueId").String(), CanonicalStore: r.Field("store").String(), ConfigRoot: r.Field("configRoot").String(), StateRoot: r.Field("stateRoot").String(), UnitRoot: r.Field("unitRoot").String(), Path: r.Field("manifestPath").String(), DispatchStateRoot: r.Field("dispatchStateRoot").String(), Executable: r.Field("executable").String(), Namespace: r.Field("namespace").Digest(), ProfileSha256: r.Field("profileSha256").Digest(), ExecutableSha256: r.Field("executableSha256").Digest(), DispatchConfigSha256: r.Field("dispatchConfigSha256").Digest(), Previous: r.Field("previous").DigestOrNull()}
	m.ProfileRaw = wire.EncodeFile(r.Field("config").Value())
	units := r.Field("units").Array(9, false)
	if err := r.Err(); err != nil {
		return nil, err
	}
	p, err := DecodeProfile(m.ProfileRaw)
	if err != nil {
		return nil, err
	}
	m.Units, err = RenderUnits(*m, *p)
	if err != nil {
		return nil, err
	}
	if len(units) != len(m.Units) {
		return nil, wire.Errorf(wire.CodeMalformed, "/units", "manifest units differ from the rendered profile")
	}
	again, err := m.Encode()
	if err != nil {
		return nil, err
	}
	if string(again) != string(raw) {
		return nil, wire.Errorf(wire.CodeMalformed, "/manifest", "manifest is not the canonical rendering of its profile")
	}
	return m, nil
}

// Operation is one durable original-request journal record. Actions are the
// planned steps; manager argv is never trusted from the record and is
// recomputed from the embedded manifests before each effect.
type Operation struct {
	Kind, RequestID, Program, Phase, PriorDesired string
	RequestSha256                                 wire.Digest
	Previous, Next                                *Manifest
	Actions                                       []Action
	Completed                                     int
	Published                                     []string
}

var operationKinds = map[string]bool{"JOURNAL_INSTALLING": true, "RECHECK_OWNER_STAGE_FSYNC_RENAME": true, "REGISTER": true, "VERIFY_OWNED_REGISTRATION": true, "COMMIT_MANIFEST_AFTER_READBACK": true, "JOURNAL_OLD_OWNED_BYTES": true, "SAVE_STOPPED_UNDER_FENCE": true, "PRESERVE_WORKERS": true, "UNREGISTER": true, "VERIFY_MANAGER_ABSENCE": true, "REMOVE_IF_UNCHANGED": true, "RETAIN_CONTROL_WORKER_DEBT_LOG_STATE": true, "NO_CHANGE_PRESERVE_CONTROL_DEBT_WORKERS": true, "ROLLBACK_REQUIRED": true, "RESTORE_SNAPSHOTTED_OWNED_BYTES": true, "RETAIN_ROLLBACK_RECEIPT": true}

func manifestValue(m *Manifest) (wire.Value, error) {
	if m == nil {
		return wire.Null(), nil
	}
	raw, err := m.Encode()
	if err != nil {
		return wire.Value{}, err
	}
	return wire.Parse(raw)
}

func EncodeOperation(o Operation) ([]byte, error) {
	if (o.Kind != "INSTALL" && o.Kind != "UNINSTALL") || !serviceName.MatchString(o.Program) || o.Completed < 0 || o.Completed > len(o.Actions) || len(o.Actions) > 64 || len(o.Published) > 9 {
		return nil, wire.Errorf(wire.CodeMalformed, "/operation", "invalid operation record")
	}
	switch o.Phase {
	case "INSTALLING", "INSTALLED", "NO_CHANGE", "REMOVING", "REMOVED", "ROLLBACK_REQUIRED", "ROLLED_BACK":
	default:
		return nil, wire.Errorf(wire.CodeMalformed, "/phase", "invalid operation phase")
	}
	if _, err := wire.ParseIdentifier("/requestId", o.RequestID); err != nil || len(o.RequestID) > wire.MaxRequestIDBytes {
		return nil, wire.Errorf(wire.CodeMalformed, "/requestId", "invalid request id")
	}
	if _, err := wire.ParseDigest("/requestSha256", string(o.RequestSha256)); err != nil {
		return nil, err
	}
	prev, err := manifestValue(o.Previous)
	if err != nil {
		return nil, err
	}
	next, err := manifestValue(o.Next)
	if err != nil {
		return nil, err
	}
	actions := []wire.Value{}
	for _, a := range o.Actions {
		if !operationKinds[a.Kind] {
			return nil, wire.Errorf(wire.CodeMalformed, "/actions", "unknown action %s", a.Kind)
		}
		actions = append(actions, wire.ObjectValue(wire.NewObject().Set("kind", wire.String(a.Kind)).Set("label", optionalString(a.Label)).Set("path", optionalString(a.Path))))
	}
	raw := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("profile", wire.String(OperationName)).Set("kind", wire.String(o.Kind)).Set("requestId", wire.String(o.RequestID)).Set("requestSha256", wire.String(string(o.RequestSha256))).Set("program", wire.String(o.Program)).Set("phase", wire.String(o.Phase)).Set("priorDesired", optionalString(o.PriorDesired)).Set("previous", prev).Set("next", next).Set("actions", wire.Array(actions...)).Set("completed", wire.String(strconv.Itoa(o.Completed))).Set("published", wire.Strings(o.Published))))
	if len(raw) > maxOperation {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/operation", "operation exceeds %d bytes", maxOperation)
	}
	return raw, nil
}

func decodeEmbedded(r *wire.Reader) (*Manifest, error) {
	if r.IsNull() {
		return nil, nil
	}
	return DecodeManifest(wire.EncodeFile(r.Value()))
}

func DecodeOperation(raw []byte) (*Operation, error) {
	if len(raw) > maxOperation {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/operation", "operation exceeds %d bytes", maxOperation)
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "kind", "requestId", "requestSha256", "program", "phase", "priorDesired", "previous", "next", "actions", "completed", "published")
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), OperationName); err != nil {
		return nil, err
	}
	o := &Operation{Kind: r.Field("kind").String(), RequestID: r.Field("requestId").String(), RequestSha256: r.Field("requestSha256").Digest(), Program: r.Field("program").String(), Phase: r.Field("phase").String(), Published: r.Field("published").Strings(9, true, (*wire.Reader).String)}
	if p := r.Field("priorDesired").StringOrNull((*wire.Reader).String); p != nil {
		o.PriorDesired = *p
	}
	for _, a := range r.Field("actions").Array(64, true) {
		a.Closed("kind", "label", "path")
		x := Action{Kind: a.Field("kind").String()}
		if l := a.Field("label").StringOrNull((*wire.Reader).String); l != nil {
			x.Label = *l
		}
		if p := a.Field("path").StringOrNull((*wire.Reader).String); p != nil {
			x.Path = *p
		}
		o.Actions = append(o.Actions, x)
	}
	completed, cerr := strconv.Atoi(r.Field("completed").String())
	prev, next := r.Field("previous"), r.Field("next")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if cerr != nil {
		return nil, wire.Errorf(wire.CodeMalformed, "/completed", "completed must be decimal")
	}
	o.Completed = completed
	if o.Previous, err = decodeEmbedded(prev); err != nil {
		return nil, err
	}
	if o.Next, err = decodeEmbedded(next); err != nil {
		return nil, err
	}
	if err := o.bound(); err != nil {
		return nil, err
	}
	if again, err := EncodeOperation(*o); err != nil || string(again) != string(raw) {
		return nil, wire.Errorf(wire.CodeMalformed, "/operation", "operation is not canonical")
	}
	return o, nil
}

// bound requires every action and published label to name exactly one
// unit or manifest path of the embedded manifests.
func (o *Operation) bound() error {
	units := map[string]string{}
	paths := map[string]bool{}
	for _, m := range []*Manifest{o.Previous, o.Next} {
		if m == nil {
			continue
		}
		if m.Program != o.Program {
			return wire.Errorf(wire.CodeMalformed, "/operation", "manifest program differs from the operation")
		}
		paths[m.Path] = true
		for _, u := range m.Units {
			units[u.Label] = u.Path
			paths[u.Path] = true
		}
	}
	for _, a := range o.Actions {
		if a.Label != "" {
			if _, ok := units[a.Label]; !ok {
				return wire.Errorf(wire.CodeMalformed, "/actions", "action names a foreign label")
			}
		}
		if a.Path != "" && !paths[a.Path] {
			return wire.Errorf(wire.CodeMalformed, "/actions", "action names a foreign path")
		}
	}
	for _, l := range o.Published {
		if _, ok := units[l]; !ok {
			return wire.Errorf(wire.CodeMalformed, "/published", "published label is foreign")
		}
	}
	return nil
}

func (o *Operation) finished() bool {
	switch o.Phase {
	case "INSTALLED", "NO_CHANGE", "REMOVED", "ROLLED_BACK":
		return true
	}
	return false
}

func operationName(request string) string {
	sum := wire.Sum([]byte(request))
	return string(sum)[:32] + ".json"
}

// operations reads the bounded journal history. Any unreadable record is an
// error: unfinished evidence is never skipped.
func (h Host) operations(root string) ([]*Operation, error) {
	dir := filepath.Join(root, operationDir)
	entries, err := os.ReadDir(dir)
	if absent(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(entries) > maxOperations {
		return nil, wire.Errorf(wire.CodeLimitExceeded, dir, "operation history exceeds %d records", maxOperations)
	}
	out := []*Operation{}
	total := 0
	for _, e := range entries {
		if len(e.Name()) > 0 && e.Name()[0] == '.' {
			continue
		}
		raw, err := h.readPrivate(filepath.Join(dir, e.Name()), maxOperation)
		if err != nil {
			return nil, err
		}
		total += len(raw)
		o, err := DecodeOperation(raw)
		if err != nil {
			return nil, err
		}
		if operationName(o.RequestID) != e.Name() {
			return nil, wire.Errorf(wire.CodeMalformed, dir, "operation file name does not bind its request")
		}
		out = append(out, o)
	}
	if total > maxHistory {
		return nil, wire.Errorf(wire.CodeLimitExceeded, dir, "operation history exceeds %d bytes", maxHistory)
	}
	return out, nil
}

func (h Host) saveOperation(root string, o *Operation) error {
	raw, err := EncodeOperation(*o)
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(root, operationDir), operationName(o.RequestID), raw)
}

func (h Host) readControl(root string) (*Control, error) {
	raw, err := h.readPrivate(filepath.Join(root, controlFile), maxControl)
	if err != nil {
		return nil, err
	}
	c, err := DecodeControl(raw)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (h Host) writeControl(root string, c Control) error {
	raw, err := EncodeControl(c)
	if err != nil {
		return err
	}
	return writeAtomic(root, controlFile, raw)
}

func (h Host) readManifest(root string) (*Manifest, []byte, error) {
	raw, err := h.readPrivate(filepath.Join(root, manifestFile), maxManifest)
	if err != nil {
		return nil, nil, err
	}
	m, err := DecodeManifest(raw)
	if err != nil {
		return nil, nil, err
	}
	return m, raw, nil
}

func describe(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprint(err)
}
