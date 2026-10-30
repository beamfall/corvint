package dispatch

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The /1 marker adds the publishing host's machine and boot digests
// (CAL-V0-227); a /0 marker, written by an earlier build, records neither and
// is still read, so it stays quarantined until an operator recovery.
const readerMarkerProfile = "taskman-dispatch-reader-lifecycle/1"
const readerMarkerLegacyProfile = "taskman-dispatch-reader-lifecycle/0"
const readerMarkerName = "reader-lifecycle.json"

// ErrReaderQuiescence means reader containment or its restart quarantine is
// unproved. It outranks ordinary cancellation and never permits another tick.
var ErrReaderQuiescence = errors.New("QUIESCENCE_UNPROVED: reader containment or quarantine evidence UNKNOWN")

type readerMarker struct {
	Profile   string `json:"profile"`
	Program   string `json:"program"`
	Run       string `json:"run"`
	Lifecycle string `json:"lifecycle"`
	// Machine and Boot are sha256 digests of the host identity at
	// publication; absent when the host did not report one.
	Machine string `json:"machine,omitempty"`
	Boot    string `json:"boot,omitempty"`
}

// ReaderLeaderProfile names the started reader leader's pid and start
// identity (CAL-V0-226). It is a separate file written after the spawn, so
// the marker itself is never rewritten and evidence a reader leaves in it
// cannot be overwritten. It is live evidence only: it can keep a quarantine
// but never lifts one, because a leader that has gone proves nothing about
// escaped descendants.
const ReaderLeaderProfile = "taskman-dispatch-reader-leader/0"

const readerLeaderName = "reader-leader.json"

// readerClearingSuffix names a marker taken aside by clearReaderMarker. It
// keeps the quarantine until the removal completes (CAL-V0-226).
const readerClearingSuffix = ".clearing"

// ReaderLeader is the decoded leader record.
type ReaderLeader struct {
	Profile     string `json:"profile"`
	Program     string `json:"program"`
	Run         string `json:"run"`
	ReaderPID   int    `json:"readerPid"`
	ReaderStart string `json:"readerStart"`
}

type readerEvidence struct {
	path     string
	raw      []byte
	identity os.FileInfo
	marker   readerMarker
}

// The actual creation-owned handle stays here until result handling ends.
// HOLD retains the slot through Close; Close then takes the terminal UNKNOWN
// exception. Neither this record nor the marker grants recovery authority.
type readerSlot struct {
	command  *exec.Cmd
	owner    *groupreap.Owner
	deadline time.Time
	bound    groupreap.RetirementBound
	result   groupreap.Result
	marker   *readerEvidence
}

// readReaderMarker cannot block on a substituted FIFO or follow a symlink.
// Identity is checked before reading and again at the pathname afterwards.
func readReaderMarker(dir, program string) (*readerEvidence, error) {
	path := filepath.Join(dir, readerMarkerName)
	raw, identity, err := rawReaderMarker(path)
	if err != nil {
		return nil, err
	}
	marker, err := parseReaderMarker(raw, program)
	if err != nil {
		return nil, err
	}
	return &readerEvidence{path: path, raw: raw, identity: identity, marker: marker}, nil
}

// rawReaderMarker reads at most 4097 bytes of a regular, non-symlinked
// marker whose identity is unchanged from before the open to after the read.
func rawReaderMarker(path string) ([]byte, os.FileInfo, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, nil, errors.New("reader marker is not a regular file")
	}
	f, err := openReaderMarker(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	identity, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !identity.Mode().IsRegular() || !os.SameFile(before, identity) {
		return nil, nil, errors.New("reader marker identity changed")
	}
	raw, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil {
		return nil, nil, err
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(identity, after) {
		return nil, nil, errors.New("reader marker changed while reading")
	}
	return raw, identity, nil
}

// parseReaderMarker is the strict decode of a marker's bytes.
func parseReaderMarker(raw []byte, program string) (readerMarker, error) {
	var marker readerMarker
	if len(raw) > 4096 || !validScalarJSON(raw) {
		return marker, errors.New("malformed reader marker")
	}
	var head struct {
		Profile string `json:"profile"`
	}
	if json.Unmarshal(raw, &head) != nil || head.Profile != readerMarkerLegacyProfile {
		if err := wire.RawProfileVersion("/profile", raw, readerMarkerProfile); err != nil {
			return marker, err
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&marker); err != nil {
		return marker, err
	}
	// Canonical bytes also reject duplicate/case-folded fields and trailing data.
	canonical, _ := json.Marshal(marker)
	legacy := marker.Profile == readerMarkerLegacyProfile
	if !bytes.Equal(raw, canonical) || (marker.Profile != readerMarkerProfile && !legacy) || marker.Program != program || marker.Lifecycle != "UNKNOWN" || len(marker.Run) != 32 {
		return marker, errors.New("malformed reader marker")
	}
	if legacy && (marker.Machine != "" || marker.Boot != "") || !optionalDigest(marker.Machine) || !optionalDigest(marker.Boot) {
		return marker, errors.New("malformed reader host evidence")
	}
	if run, err := hex.DecodeString(marker.Run); err != nil || len(run) != 16 {
		return marker, errors.New("malformed reader run identifier")
	}
	return marker, nil
}

// optionalDigest admits an absent value or a lowercase sha256 hex digest.
func optionalDigest(s string) bool {
	if s == "" {
		return true
	}
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == sha256.Size && hex.EncodeToString(b) == s
}

// ReaderContainment is a pure read. Absence is NOT_OBSERVED, never RELEASED.
// The diagnostic names the sanctioned recovery (CAL-V0-228); a read never
// applies the reboot proof, which only Open takes under the program lock.
func ReaderContainment(dir, program string) (state string, quarantined bool, diagnostic string) {
	mark, err := readReaderMarker(dir, program)
	if errors.Is(err, fs.ErrNotExist) {
		tomb := filepath.Join(dir, readerMarkerName+readerClearingSuffix)
		if _, tombErr := os.Lstat(tomb); errors.Is(tombErr, fs.ErrNotExist) {
			return "NOT_OBSERVED", false, ""
		}
		return "UNKNOWN", true, "an interrupted reader marker removal left " + tomb + "; " + recoverHint(program)
	}
	if err != nil {
		return "UNKNOWN", true, err.Error() + "; " + recoverHint(program)
	}
	process := "no reader pid or start time is recorded"
	if leader, err := readReaderLeader(dir, program); err == nil && leader.Run == mark.marker.Run {
		process = "the leader record names reader pid " + strconv.Itoa(leader.ReaderPID) + ", whose exit alone does not prove its descendants ended"
	}
	why := process + ", and the host has not rebooted since it was written"
	switch _, ok := rebootProof(mark.marker); {
	case ok:
		why = "the host has rebooted since the marker was written; the next dispatch start records that proof and clears it"
	case mark.marker.Machine == "" || mark.marker.Boot == "":
		why = process + ", and it records no host boot identity, so no reboot proof is possible"
	}
	return "UNKNOWN", true, "reader lifecycle unresolved (run " + mark.marker.Run + "); " + why + "; " + recoverHint(program)
}

func recoverHint(program string) string {
	return "after confirming the reader and its descendants exited, run `corvint-tasks dispatch recover-reader --program " + program + " --config FILE --reason TEXT --evidence REF --request-id ID`"
}

func checkReaderQuarantine(dir, program string) error {
	_, held, why := ReaderContainment(dir, program)
	if held {
		return fmt.Errorf("%w: %s", ErrReaderQuiescence, why)
	}
	return nil
}

// Caller holds the program lock. The checked writer includes file Sync and
// Close, but no parent-directory sync/power-loss qualification is claimed.
func publishReaderMarker(dir, program string) (*readerEvidence, error) {
	if err := checkReaderQuarantine(dir, program); err != nil {
		return nil, err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	machine, boot := hostIdentity()
	raw, _ := json.Marshal(readerMarker{Profile: readerMarkerProfile, Program: program, Run: hex.EncodeToString(nonce[:]), Lifecycle: "UNKNOWN", Machine: machine, Boot: boot})
	if err := writeAtomic(filepath.Join(dir, readerMarkerName), raw); err != nil {
		return nil, err
	}
	mark, err := readReaderMarker(dir, program)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(mark.raw, raw) {
		return nil, errors.New("reader marker replaced during publication")
	}
	return mark, nil
}

// publishReaderLeader records the started leader (CAL-V0-226). It replaces
// any earlier leader record and never touches the marker.
func publishReaderLeader(dir, program, run string, pid int, start string) error {
	raw, _ := json.Marshal(ReaderLeader{Profile: ReaderLeaderProfile, Program: program, Run: run, ReaderPID: pid, ReaderStart: start})
	return writeAtomic(filepath.Join(dir, readerLeaderName), raw)
}

// readReaderLeader strictly decodes the leader record; fs.ErrNotExist when
// there is none.
func readReaderLeader(dir, program string) (*ReaderLeader, error) {
	raw, _, err := rawReaderMarker(filepath.Join(dir, readerLeaderName))
	if err != nil {
		return nil, err
	}
	if len(raw) > 4096 || !validScalarJSON(raw) {
		return nil, errors.New("malformed reader leader record")
	}
	if err := wire.RawProfileVersion("/profile", raw, ReaderLeaderProfile); err != nil {
		return nil, err
	}
	var leader ReaderLeader
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&leader); err != nil {
		return nil, err
	}
	canonical, _ := json.Marshal(leader)
	run, runErr := hex.DecodeString(leader.Run)
	if !bytes.Equal(raw, canonical) || leader.Program != program || runErr != nil || len(run) != 16 || leader.ReaderPID <= 0 || leader.ReaderStart == "" || len(leader.ReaderStart) > 64 {
		return nil, errors.New("malformed reader leader record")
	}
	return &leader, nil
}

// ReadReaderLeader reads the program state directory's leader record
// strictly (CAL-V0-229).
func ReadReaderLeader(dir, program string) (*ReaderLeader, error) {
	return readReaderLeader(dir, program)
}

// readerTakeAsideHook is replaced only by tests that change the marker
// between the check and the rename.
var readerTakeAsideHook = func(string) {}

// clearReaderMarker removes the marker only while its identity and bytes
// are exactly the evidence the caller holds. The marker is first renamed
// aside, atomically capturing whatever is at its path, and checked again
// there: a marker replaced or rewritten before the rename is put back, and
// the name it was taken to keeps the quarantine if the process stops
// between the rename and the removal. The leader record goes last; one
// left behind only adds refusals while its process runs.
func clearReaderMarker(mark *readerEvidence, program string) error {
	raw, identity, err := rawReaderMarker(mark.path)
	if err != nil {
		return err
	}
	if !os.SameFile(mark.identity, identity) || !bytes.Equal(mark.raw, raw) {
		return errors.New("reader marker identity or bytes changed; preserved")
	}
	dir := filepath.Dir(mark.path)
	if filepath.Base(mark.path) == readerMarkerName {
		tomb := mark.path + readerClearingSuffix
		if _, err := os.Lstat(tomb); !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("reader marker removal refused: %s exists (%v); preserved", tomb, err)
		}
		readerTakeAsideHook(mark.path)
		if err := os.Rename(mark.path, tomb); err != nil {
			return err
		}
		raw, identity, err = rawReaderMarker(tomb)
		if err != nil || !os.SameFile(mark.identity, identity) || !bytes.Equal(mark.raw, raw) {
			// Put the captured file back unless a new marker took the
			// path; either way the quarantine stays.
			if os.Link(tomb, mark.path) == nil {
				_ = os.Remove(tomb)
			}
			return errors.New("reader marker changed at removal; preserved")
		}
		mark = &readerEvidence{path: tomb, raw: raw, identity: identity, marker: mark.marker}
	}
	if err := os.Remove(mark.path); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dir, readerLeaderName)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("reader marker removed, but the leader record stays: %v", err)
	}
	return nil
}

func (d *Dispatcher) poisonReader(err error) error {
	if d.readerErr == nil {
		d.readerErr = fmt.Errorf("%w: %v", ErrReaderQuiescence, err)
	}
	return d.readerErr
}

// readerRetirementAllowance is the fixed one-second reader retirement
// allowance. Only a test that does not exercise the allowance replaces it, per
// dispatcher, so a loaded host cannot turn its cancellation into a HOLD
// (V1-1392); the product value never changes.
func (d *Dispatcher) readerRetirementAllowance() time.Duration {
	if d.readerAllowance > 0 {
		return d.readerAllowance
	}
	return time.Second
}

func readerRetirementDeadline(commandDeadline, trigger time.Time, allowance time.Duration) time.Time {
	deadline := trigger.Add(allowance)
	if commandDeadline.Add(allowance).Before(deadline) {
		deadline = commandDeadline.Add(allowance)
	}
	return deadline
}

func (d *Dispatcher) stateCommand(parent context.Context, dir string, argv []string) (map[string]commandState, error) {
	if d.readerErr != nil {
		return nil, d.readerErr
	}
	if err := parent.Err(); err != nil {
		return nil, err
	}
	if d.reader != nil {
		return nil, d.poisonReader(errors.New("reader slot already occupied"))
	}
	if !groupreap.OwnerAvailable() {
		return nil, groupreap.ErrOwnerUnavailable
	}
	limit := stateTimeout
	if w := d.Config.WorkState; w != nil {
		limit = w.Timeout()
	}
	ctx, cancel := context.WithTimeout(parent, limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir, cmd.WaitDelay = dir, time.Second
	var out bounded
	cmd.Stdout = &out
	mark, err := publishReaderMarker(d.dir, d.Program)
	if err != nil {
		return nil, d.poisonReader(err)
	}
	slot := &readerSlot{command: cmd, marker: mark}
	d.reader = slot
	start := d.readerStart
	if start == nil {
		// A reader command may fork, and on Darwin a group SIGKILL can miss
		// a member forked alongside it, so the owner repeats it while a
		// live member remains before the reap (PGO-V0-017).
		host := d.readerHost
		host.RepeatKillWhileLive = true
		start = func(c *exec.Cmd) (*groupreap.Owner, error) { return groupreap.StartWith(c, host) }
	}
	slot.owner, err = start(cmd)
	if err != nil {
		// The creation API returns failure only before a child is owned.
		if clearErr := clearReaderMarker(mark, d.Program); clearErr != nil {
			return nil, d.poisonReader(clearErr)
		}
		d.reader = nil
		return nil, err
	}
	// The leader's pid and start identity become live evidence that keeps
	// an operator recovery refused while it runs (CAL-V0-226). The marker
	// is not rewritten; a failed leader write only loses that evidence.
	if cmd.Process != nil {
		if id, idErr := supervisor.ProcessIdentity(cmd.Process.Pid); idErr == nil && id != "" && len(id) <= 64 {
			_ = publishReaderLeader(d.dir, d.Program, mark.marker.Run, cmd.Process.Pid, id)
		}
	}
	select {
	case <-slot.owner.Exited():
	case <-ctx.Done():
	}
	commandDeadline, _ := ctx.Deadline()
	slot.deadline = readerRetirementDeadline(commandDeadline, time.Now(), d.readerRetirementAllowance())
	retirement, stop := context.WithDeadline(context.Background(), slot.deadline)
	defer stop()
	slot.bound = groupreap.RetirementBound{Done: retirement.Done(), Expired: func() bool { return !time.Now().Before(slot.deadline) }}
	slot.result = slot.owner.FinishBounded(slot.bound)
	if slot.result.State != groupreap.Released {
		// Output and Go watcher joins are UNKNOWN: never inspect out here.
		return nil, d.poisonReader(fmt.Errorf("owner=%s, retirement=%v, wait=%v; joins UNKNOWN", slot.result.State, slot.result.Err, slot.result.WaitErr))
	}
	var states map[string]commandState
	err = slot.result.WaitErr
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) && parent.Err() == nil {
		// CAL-V0-274: the overrun killed the command; name the cause, not
		// the signal.
		err = fmt.Errorf("timed out after %ds", int(limit/time.Second))
	}
	if err == nil {
		if out.over {
			err = fmt.Errorf("output exceeds %d bytes", maxStateCommand)
		} else {
			states, err = decodeCommandStates(out.buf.Bytes())
		}
	}
	// RELEASED plus result handling precedes removal. A failed clear poisons
	// this invocation even though the recorded owner itself was released.
	if clearErr := clearReaderMarker(mark, d.Program); clearErr != nil {
		return nil, d.poisonReader(clearErr)
	}
	d.reader = nil
	return states, err
}

// Direct ReadStates callers use the same lock/marker boundary. The dispatcher
// passes its own slot instead; no reader uses a transient unquarantined owner.
func standaloneStateCommand(ctx context.Context, c *Config, argv []string) (map[string]commandState, error) {
	const program = "work-state-reader"
	if !platformSupported {
		return nil, groupreap.ErrOwnerUnavailable
	}
	dir := ProgramDir(c, program)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	if err := lockExclusive(lock); err != nil {
		return nil, err
	}
	if _, err := resolveReaderByReboot(dir, program, time.Now()); err != nil {
		return nil, fmt.Errorf("%w: the reboot proof was not recorded: %v", ErrReaderQuiescence, err)
	}
	if err := checkReaderQuarantine(dir, program); err != nil {
		return nil, err
	}
	d := &Dispatcher{Program: program, Config: c, dir: dir, lock: lock}
	return d.stateCommand(ctx, c.WorkRoot, argv)
}
