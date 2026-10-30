package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	ossignal "os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Rolling upgrade (DSG-V0-010..015). A running dispatcher hands its lock,
// ledger and live worker processes to a new build by replacing its own image
// (execve): the process ID, the inherited lock and every worker stay; only
// new launches use the new build. Nothing here changes the ledger format.
const (
	// HandoverProtocol is the handover a build accepts; `dispatch status`
	// names it so `dispatch upgrade --apply` can refuse a build without it.
	HandoverProtocol = "taskman-dispatch-handover/0"
	// MaxMixedBuilds bounds the distinct builds live at once: the running
	// dispatcher's, its workers' and a requested one (DSG-V0-014).
	MaxMixedBuilds = 3
	// BuildUnknown names the build of a worker without a build record.
	BuildUnknown = "UNKNOWN"

	handoverEnv        = "CORVINT_DISPATCH_HANDOVER"
	tasksBinEnv        = "CORVINT_TASKS_BIN"
	upgradeStateName   = "upgrade.json"
	upgradeRequestName = "upgrade-request.json"
	workerBuildName    = "build.json"

	UpgradeStateProfile   = "taskman-dispatch-upgrade-state/0"
	UpgradeRequestProfile = "taskman-dispatch-upgrade-request/0"
	WorkerBuildProfile    = "taskman-dispatch-worker-build/0"
	maxUpgradeRecord      = 64 << 10
)

// Upgrade phases recorded in upgrade.json.
const (
	UpgradeRunning     = "RUNNING"
	UpgradeExec        = "EXEC"
	UpgradeFailed      = "FAILED"
	UpgradeInterrupted = "INTERRUPTED"
)

// ErrUpgradeRequested ends Run between ticks when an upgrade request is
// pending for a dispatcher that accepts handover; the caller then calls
// Handover.
var ErrUpgradeRequested = errors.New("dispatcher upgrade requested")

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Build identifies an executable by path and content digest; an empty
// Sha256 is a digest that could not be read.
type Build struct {
	Path   string `json:"executable"`
	Sha256 string `json:"sha256"`
}

// ID is the digest, or UNKNOWN.
func (b Build) ID() string {
	if b.Sha256 == "" {
		return BuildUnknown
	}
	return b.Sha256
}

var (
	selfOnce  sync.Once
	selfBuild Build
)

// SelfBuild is this process image's executable and digest, read once per
// image. A rolling upgrade expects versioned build paths, so the file at the
// path is the image that runs.
func SelfBuild() Build {
	selfOnce.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			return
		}
		selfBuild.Path = exe
		selfBuild.Sha256, _ = BuildDigest(exe)
	})
	return selfBuild
}

// BuildDigest is the SHA-256 of an executable regular file.
func BuildDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("%s is not an executable regular file", path)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// workerBuild is the per-session build record beside a worker's logs
// (DSG-V0-010).
type workerBuild struct {
	Profile    string `json:"profile"`
	Worker     string `json:"worker"`
	Executable string `json:"executable"`
	Sha256     string `json:"sha256"`
}

func writeWorkerBuild(dir, id string, b Build) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(workerBuild{Profile: WorkerBuildProfile, Worker: id, Executable: b.Path, Sha256: b.Sha256})
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, workerBuildName), append(raw, '\n'))
}

// WorkerBuild reads the build a worker was launched by; a missing or
// unreadable record is the UNKNOWN build.
func WorkerBuild(programDir, worker string) Build {
	b, err := ReadWorkerBuildFile(filepath.Join(programDir, "workers", worker, workerBuildName), worker)
	if err != nil {
		return Build{}
	}
	return b
}

// ReadWorkerBuildFile decodes one worker build record; a newer profile
// version is refused as UNSUPPORTED_VERSION (CAL-V0-131).
func ReadWorkerBuildFile(path, worker string) (Build, error) {
	raw, err := readBounded(path, 4096)
	if err != nil {
		return Build{}, err
	}
	if err := wire.RawProfileVersion("/profile", raw, WorkerBuildProfile); err != nil {
		return Build{}, err
	}
	var r workerBuild
	if json.Unmarshal(raw, &r) != nil || r.Profile != WorkerBuildProfile || r.Worker != worker || (r.Sha256 != "" && !sha256Hex.MatchString(r.Sha256)) {
		return Build{}, errors.New("not a valid " + WorkerBuildProfile + " record")
	}
	return Build{Path: r.Executable, Sha256: r.Sha256}, nil
}

// ExitRecord is a worker leader's exit carried across a handover.
type ExitRecord struct {
	Code   int `json:"code"`
	Signal int `json:"signal"`
}

// UpgradeState is upgrade.json: the last handover phase of this program's
// dispatcher (DSG-V0-011..015).
type UpgradeState struct {
	Profile    string                `json:"profile"`
	Phase      string                `json:"phase"`
	PID        int                   `json:"pid"`
	Identity   string                `json:"identity"`
	Build      string                `json:"build"`
	Sha256     string                `json:"sha256"`
	From       string                `json:"from,omitempty"`
	FromSha256 string                `json:"fromSha256,omitempty"`
	At         string                `json:"at"`
	Detail     string                `json:"detail,omitempty"`
	Exits      map[string]ExitRecord `json:"exits,omitempty"`
}

// ReadUpgradeState reads upgrade.json; fs.ErrNotExist when absent.
func ReadUpgradeState(programDir string) (*UpgradeState, error) {
	raw, err := readBounded(filepath.Join(programDir, upgradeStateName), maxUpgradeRecord)
	if err != nil {
		return nil, err
	}
	if err := wire.RawProfileVersion("/profile", raw, UpgradeStateProfile); err != nil {
		return nil, err
	}
	var st UpgradeState
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("%s: %w", upgradeStateName, err)
	}
	if st.Profile != UpgradeStateProfile {
		return nil, fmt.Errorf("%s: profile %q is not %s", upgradeStateName, st.Profile, UpgradeStateProfile)
	}
	return &st, nil
}

func writeUpgradeState(programDir string, st *UpgradeState) error {
	st.Profile = UpgradeStateProfile
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(programDir, upgradeStateName), append(raw, '\n'))
}

// UpgradeRequest is upgrade-request.json, written by `dispatch upgrade
// --apply` and consumed once by the running dispatcher.
type UpgradeRequest struct {
	Profile     string `json:"profile"`
	Build       string `json:"build"`
	Sha256      string `json:"sha256"`
	RequestedAt string `json:"requestedAt"`
}

// UpgradeRequestPath is where a pending request waits.
func UpgradeRequestPath(programDir string) string {
	return filepath.Join(programDir, upgradeRequestName)
}

// WriteUpgradeRequest publishes a request complete or not at all, and
// refuses when one is already pending (fs.ErrExist).
func WriteUpgradeRequest(programDir string, r UpgradeRequest) error {
	// A pending request is refused before anything is written.
	if _, err := os.Lstat(UpgradeRequestPath(programDir)); err == nil {
		return fs.ErrExist
	}
	r.Profile = UpgradeRequestProfile
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(programDir, ".tmp-upgrade-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Link(tmp.Name(), UpgradeRequestPath(programDir))
}

func readUpgradeRequest(programDir string) (*UpgradeRequest, error) {
	return ReadUpgradeRequestFile(UpgradeRequestPath(programDir))
}

// ReadUpgradeRequestFile decodes one upgrade request; a newer profile
// version is refused as UNSUPPORTED_VERSION (CAL-V0-131).
func ReadUpgradeRequestFile(path string) (*UpgradeRequest, error) {
	raw, err := readBounded(path, 4096)
	if err != nil {
		return nil, err
	}
	if err := wire.RawProfileVersion("/profile", raw, UpgradeRequestProfile); err != nil {
		return nil, err
	}
	var r UpgradeRequest
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	if r.Profile != UpgradeRequestProfile || !filepath.IsAbs(r.Build) || !sha256Hex.MatchString(r.Sha256) {
		return nil, errors.New("not a valid " + UpgradeRequestProfile + " record")
	}
	return &r, nil
}

// LiveBuilds is the set of distinct builds the recorded workers launched
// by, keyed by Build.ID.
func LiveBuilds(programDir string, l *Ledger) map[string]bool {
	out := map[string]bool{}
	for _, w := range l.Workers {
		out[WorkerBuild(programDir, w.ID).ID()] = true
	}
	return out
}

// EnableUpgrade makes this dispatcher accept handover requests: args is the
// argv after the executable that a new build is started with. It records the
// RUNNING phase, which `dispatch upgrade --apply` requires, and keeps the
// last handover's from-build for status.
func (d *Dispatcher) EnableUpgrade(args []string) error {
	d.upgradeArgs = slices.Clone(args)
	self := SelfBuild()
	id, err := supervisor.ProcessIdentity(os.Getpid())
	if err != nil {
		return err
	}
	st := &UpgradeState{Phase: UpgradeRunning, PID: os.Getpid(), Identity: id, Build: self.Path, Sha256: self.Sha256, At: d.Now().UTC().Format(time.RFC3339)}
	if d.upgradedFrom != nil {
		st.From, st.FromSha256 = d.upgradedFrom.Path, d.upgradedFrom.Sha256
	}
	return writeUpgradeState(d.dir, st)
}

// upgradePending reports a request for a dispatcher that accepts one.
func (d *Dispatcher) upgradePending() bool {
	if d.upgradeArgs == nil {
		return false
	}
	_, err := os.Lstat(UpgradeRequestPath(d.dir))
	return err == nil
}

// Handover consumes the pending request (DSG-V0-012). It verifies the
// requested build again, quiesces this run as Close would except that the
// lock stays held, records the EXEC phase and the exits observed so far, and
// replaces this process image with the new build, passing it the lock
// descriptor. It returns only when no handover happened; this dispatcher then
// keeps running on its own build and Run may be called again.
func (d *Dispatcher) Handover(ctx context.Context) error {
	refuse := func(code, msg string) error {
		d.emit(Event{Kind: "alert", Message: "upgrade refused: " + msg, Detail: map[string]string{"code": code}})
		return errors.New(msg)
	}
	req, readErr := readUpgradeRequest(d.dir)
	if err := os.Remove(UpgradeRequestPath(d.dir)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		// A request that cannot be consumed would end every Run at once.
		d.upgradeArgs = nil
		return refuse("UPGRADE_REQUEST_STUCK", fmt.Sprintf("the request could not be removed, so this run accepts no further upgrade: %v", err))
	}
	if readErr != nil {
		return refuse("UPGRADE_REQUEST_INVALID", fmt.Sprintf("the request is unreadable: %v", readErr))
	}
	if sum, err := BuildDigest(req.Build); err != nil || sum != req.Sha256 {
		return refuse("UPGRADE_BUILD_CHANGED", fmt.Sprintf("%s no longer has the requested digest %s", req.Build, req.Sha256))
	}
	// The new image reads the configuration afresh; one that moved stateDir
	// or workRoot would leave this run's workers behind, so it is refused.
	if d.configRead != nil {
		raw, _, err := d.configRead()
		var c *Config
		if err == nil {
			c, err = DecodeConfig(raw)
		}
		if err == nil && (c.StateDir != d.Config.StateDir || c.WorkRoot != d.Config.WorkRoot) {
			err = errors.New("stateDir or workRoot changed")
		}
		if err != nil {
			return refuse("UPGRADE_CONFIG_CHANGED", fmt.Sprintf("the configuration the new build would start with is not usable here: %v", err))
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if d.readerErr != nil {
		return refuse("UPGRADE_READER_UNKNOWN", d.readerErr.Error())
	}
	joinErr := d.stopPoolSweep()
	d.closeRetireScan()
	for id, ch := range d.exits {
		select {
		case c := <-ch:
			d.codes[id] = c
		default:
		}
	}
	self := SelfBuild()
	pid := os.Getpid()
	identity, err := supervisor.ProcessIdentity(pid)
	if err != nil {
		return refuse("UPGRADE_IDENTITY_UNKNOWN", err.Error())
	}
	st := &UpgradeState{Phase: UpgradeExec, PID: pid, Identity: identity, Build: req.Build, Sha256: req.Sha256, From: self.Path, FromSha256: self.Sha256, At: d.Now().UTC().Format(time.RFC3339), Exits: map[string]ExitRecord{}}
	for _, w := range d.ledger.Workers {
		if c, ok := d.codes[w.ID]; ok {
			st.Exits[w.ID] = ExitRecord{Code: c.code, Signal: c.signal}
		}
	}
	d.emit(Event{Kind: "upgrading", Message: fmt.Sprintf("handing over to build %s (%s); %d live worker(s) keep running on their own builds", req.Build, short(req.Sha256), len(d.ledger.Workers)), Detail: map[string]string{"from": self.Path, "fromSha256": self.Sha256, "to": req.Build, "toSha256": req.Sha256}})
	if err := errors.Join(joinErr, d.ledger.save(d.dir)); err != nil {
		return refuse("UPGRADE_LEDGER_UNSAVED", err.Error())
	}
	if err := writeUpgradeState(d.dir, st); err != nil {
		return refuse("UPGRADE_STATE_UNWRITTEN", err.Error())
	}
	fail := func(err error) error {
		st.Phase, st.Detail, st.Exits = UpgradeFailed, err.Error(), nil
		st.Build, st.Sha256 = self.Path, self.Sha256
		_ = writeUpgradeState(d.dir, st)
		d.emit(Event{Kind: "alert", Message: fmt.Sprintf("upgrade to %s failed; this dispatcher keeps running on %s: %v", req.Build, self.Path, err), Detail: map[string]string{"code": "UPGRADE_FAILED"}})
		return err
	}
	fd, err := dupForExec(d.lock)
	if err != nil {
		return fail(err)
	}
	env := slices.DeleteFunc(os.Environ(), func(kv string) bool { return strings.HasPrefix(kv, handoverEnv+"=") })
	env = append(env, handoverEnv+"="+strconv.Itoa(fd))
	argv := append([]string{req.Build}, d.upgradeArgs...)
	// A stop signal from here on takes its default action: the process ends
	// and the next start reports the interrupted upgrade (DSG-V0-015). One
	// that arrived before the reset is honoured by not executing.
	ossignal.Reset(os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	if ctx.Err() != nil {
		closeFD(fd)
		st.Phase, st.Detail, st.Exits = UpgradeRunning, "stop requested before the handover", nil
		st.Build, st.Sha256 = self.Path, self.Sha256
		_ = writeUpgradeState(d.dir, st)
		return ctx.Err()
	}
	exec := d.execImage
	if exec == nil {
		exec = execImage
	}
	err = exec(req.Build, argv, env)
	closeFD(fd)
	return fail(err)
}

// inheritLock takes the lock a handing-over image passed (DSG-V0-013). It
// returns nil, nil when no handover is named. The variable is removed so no
// worker inherits it, and the descriptor is marked close-on-exec before any
// launch.
func inheritLock(dir string) (*os.File, error) {
	v, ok := os.LookupEnv(handoverEnv)
	if !ok {
		return nil, nil
	}
	_ = os.Unsetenv(handoverEnv)
	fd, err := strconv.Atoi(v)
	if err != nil || fd < 3 {
		return nil, fmt.Errorf("%s=%q names no descriptor", handoverEnv, v)
	}
	path := filepath.Join(dir, "lock")
	f := os.NewFile(uintptr(fd), path)
	if f == nil {
		return nil, fmt.Errorf("descriptor %d is not open", fd)
	}
	markCloseOnExec(fd)
	got, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	want, err := os.Stat(path)
	if err != nil || !os.SameFile(got, want) {
		f.Close()
		return nil, fmt.Errorf("descriptor %d is not %s", fd, path)
	}
	if err := lockExclusive(f); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// resumeUpgrade runs in Open after adoption events (DSG-V0-013/015). A
// handed-over image restores the carried exits, waits for its inherited
// workers and reports the mixed-build window; any other start that finds an
// EXEC phase reports the interrupted upgrade once. A request left by a
// stopped run is discarded: the start that follows supersedes it.
func (d *Dispatcher) resumeUpgrade(inherited bool, inheritErr error) {
	if inheritErr != nil {
		d.emit(Event{Kind: "alert", Message: "the handed-over lock could not be verified, so it was taken afresh: " + inheritErr.Error(), Detail: map[string]string{"code": "UPGRADE_LOCK_UNVERIFIED"}})
	}
	// A handed-over image keeps a request: the old image consumed its own
	// before exec, so any present one was made for this run.
	if _, err := os.Lstat(UpgradeRequestPath(d.dir)); err == nil && !inherited {
		if os.Remove(UpgradeRequestPath(d.dir)) == nil {
			d.emit(Event{Kind: "alert", Message: "discarded an upgrade request left by a stopped dispatcher; request it again for this run", Detail: map[string]string{"code": "UPGRADE_REQUEST_DISCARDED"}})
		}
	}
	st, err := ReadUpgradeState(d.dir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			d.emit(Event{Kind: "alert", Message: "upgrade record unreadable: " + err.Error(), Detail: map[string]string{"code": "UPGRADE_STATE_UNREADABLE"}})
		}
		if inherited {
			d.adoptExits(nil)
		}
		return
	}
	if st.Phase != UpgradeExec {
		if inherited {
			d.adoptExits(nil)
		}
		return
	}
	if !inherited || st.PID != os.Getpid() {
		st.Phase, st.Detail, st.Exits = UpgradeInterrupted, fmt.Sprintf("the handover from pid %d to %s did not complete; this start adopted the workers", st.PID, st.Build), nil
		_ = writeUpgradeState(d.dir, st)
		d.emit(Event{Kind: "alert", Message: fmt.Sprintf("an upgrade to %s was interrupted before the new build took over; this start adopted %d recorded worker(s), whose leases and attempts are unchanged", st.Build, len(d.ledger.Workers)), Detail: map[string]string{"code": "UPGRADE_INTERRUPTED", "to": st.Build}})
		return
	}
	d.adoptExits(st.Exits)
	d.upgradedFrom = &Build{Path: st.From, Sha256: st.FromSha256}
	self := SelfBuild()
	old, ends := 0, time.Time{}
	for _, w := range d.ledger.Workers {
		if WorkerBuild(d.dir, w.ID).ID() == self.ID() && self.Sha256 != "" {
			continue
		}
		old++
		if r := d.launchConfig(w).roleNamed(w.Role); r != nil {
			if e := w.Started.Add(time.Duration(r.WallSeconds) * time.Second); e.After(ends) {
				ends = e
			}
		}
	}
	d.mixedBuilds = old > 0
	detail := map[string]string{"from": st.From, "fromSha256": st.FromSha256, "to": self.Path, "toSha256": self.Sha256, "oldBuildWorkers": strconv.Itoa(old)}
	if !ends.IsZero() {
		detail["windowEndsBy"] = ends.UTC().Format(time.RFC3339)
	}
	d.emit(Event{Kind: "upgraded", Message: fmt.Sprintf("now running build %s; %d worker(s) launched by an earlier build finish on it, at the latest by their roles' wall caps", self.Path, old), Detail: detail})
}

// adoptExits waits for the workers an earlier image of this process
// launched, so their exits are observed (DSG-V0-013). A worker that is not
// this process's child, shares its pid with another record, or already has
// a carried exit is left as adoption leaves it.
func (d *Dispatcher) adoptExits(carried map[string]ExitRecord) {
	pids := map[int]int{}
	for _, w := range d.ledger.Workers {
		pids[w.PID]++
	}
	for _, w := range d.ledger.Workers {
		if c, ok := carried[w.ID]; ok {
			d.codes[w.ID] = exitStatus{code: c.Code, signal: c.Signal}
			continue
		}
		if pids[w.PID] != 1 {
			continue
		}
		if id, err := supervisor.ProcessIdentity(w.PID); err == nil && id != "" && !supervisor.SameProcess(id, w.LeaderIdentity) {
			continue
		}
		if ch := waitChild(w.PID); ch != nil {
			d.exits[w.ID] = ch
		}
	}
}

// settleMixedBuilds emits upgrade-settled once no recorded worker runs on a
// build other than this image's (DSG-V0-014).
func (d *Dispatcher) settleMixedBuilds() {
	if !d.mixedBuilds {
		return
	}
	self := SelfBuild().ID()
	for _, w := range d.ledger.Workers {
		if WorkerBuild(d.dir, w.ID).ID() != self || self == BuildUnknown {
			return
		}
	}
	d.mixedBuilds = false
	d.emit(Event{Kind: "upgrade-settled", Message: "every worker launched by an earlier build has finished; the earlier build may be removed", Detail: map[string]string{"build": SelfBuild().Path}})
}

func short(sum string) string {
	if len(sum) > 12 {
		return sum[:12]
	}
	return sum
}
