package dispatch

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

const readerMarkerProfile = "taskman-dispatch-reader-lifecycle/0"
const readerMarkerName = "reader-lifecycle.json"

// ErrReaderQuiescence means reader containment or its restart quarantine is
// unproved. It outranks ordinary cancellation and never permits another tick.
var ErrReaderQuiescence = errors.New("QUIESCENCE_UNPROVED: reader containment or quarantine evidence UNKNOWN")

type readerMarker struct {
	Profile   string `json:"profile"`
	Program   string `json:"program"`
	Run       string `json:"run"`
	Lifecycle string `json:"lifecycle"`
}

type readerEvidence struct {
	path     string
	raw      []byte
	identity os.FileInfo
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
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, errors.New("reader marker is not a regular file")
	}
	f, err := openReaderMarker(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	identity, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !identity.Mode().IsRegular() || !os.SameFile(before, identity) {
		return nil, errors.New("reader marker identity changed")
	}
	raw, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil {
		return nil, err
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(identity, after) {
		return nil, errors.New("reader marker changed while reading")
	}
	var marker readerMarker
	if len(raw) > 4096 || !validScalarJSON(raw) {
		return nil, errors.New("malformed reader marker")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&marker); err != nil {
		return nil, err
	}
	// Canonical bytes also reject duplicate/case-folded fields and trailing data.
	canonical, _ := json.Marshal(marker)
	if !bytes.Equal(raw, canonical) || marker.Profile != readerMarkerProfile || marker.Program != program || marker.Lifecycle != "UNKNOWN" || len(marker.Run) != 32 {
		return nil, errors.New("malformed reader marker")
	}
	if run, err := hex.DecodeString(marker.Run); err != nil || len(run) != 16 {
		return nil, errors.New("malformed reader run identifier")
	}
	return &readerEvidence{path: path, raw: raw, identity: identity}, nil
}

// ReaderContainment is a pure read. Absence is NOT_OBSERVED, never RELEASED.
func ReaderContainment(dir, program string) (state string, quarantined bool, diagnostic string) {
	_, err := readReaderMarker(dir, program)
	if errors.Is(err, fs.ErrNotExist) {
		return "NOT_OBSERVED", false, ""
	}
	if err != nil {
		return "UNKNOWN", true, err.Error()
	}
	return "UNKNOWN", true, "reader lifecycle unresolved; no automatic recovery"
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
	raw, _ := json.Marshal(readerMarker{readerMarkerProfile, program, hex.EncodeToString(nonce[:]), "UNKNOWN"})
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

func clearReaderMarker(mark *readerEvidence, program string) error {
	current, err := readReaderMarker(filepath.Dir(mark.path), program)
	if err != nil {
		return err
	}
	if !os.SameFile(mark.identity, current.identity) || !bytes.Equal(mark.raw, current.raw) {
		return errors.New("reader marker identity or bytes changed; preserved")
	}
	return os.Remove(mark.path)
}

func (d *Dispatcher) poisonReader(err error) error {
	if d.readerErr == nil {
		d.readerErr = fmt.Errorf("%w: %v", ErrReaderQuiescence, err)
	}
	return d.readerErr
}

func readerRetirementDeadline(commandDeadline, trigger time.Time) time.Time {
	deadline := trigger.Add(time.Second)
	if commandDeadline.Add(time.Second).Before(deadline) {
		deadline = commandDeadline.Add(time.Second)
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
	ctx, cancel := context.WithTimeout(parent, stateTimeout)
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
		start = groupreap.Start
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
	select {
	case <-slot.owner.Exited():
	case <-ctx.Done():
	}
	commandDeadline, _ := ctx.Deadline()
	slot.deadline = readerRetirementDeadline(commandDeadline, time.Now())
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
	if err := checkReaderQuarantine(dir, program); err != nil {
		return nil, err
	}
	d := &Dispatcher{Program: program, Config: c, dir: dir, lock: lock}
	return d.stateCommand(ctx, c.WorkRoot, argv)
}
