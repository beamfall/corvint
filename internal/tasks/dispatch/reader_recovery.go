package dispatch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ReaderRecoveryProfile is the attestation that lifted one reader quarantine
// (CAL-V0-226, CAL-V0-227). It is written, fsynced and linked into place
// before the marker is removed, and is never removed by Corvint.
const ReaderRecoveryProfile = "taskman-dispatch-reader-recovery/0"

const readerRecoveryDir = "reader-recoveries"

// readerUnannouncedDir, under reader-recoveries/, holds a hard link to each
// operator recovery record the next dispatch start has yet to announce
// (CAL-V0-940); maxAnnouncedRecoveries bounds one start's announcements.
const (
	readerUnannouncedDir   = "unannounced"
	maxAnnouncedRecoveries = 64
)

// Recovery kinds: an operator attestation, or the host-reboot proof.
const (
	RecoveryOperator   = "OPERATOR"
	RecoveryHostReboot = "HOST_REBOOT"
)

// ReaderRecovery is one recorded quarantine recovery. Run is empty when the
// marker was unreadable; the marker's digest and modification time bind the
// record to the exact marker it cleared.
type ReaderRecovery struct {
	Profile       string `json:"profile"`
	Program       string `json:"program"`
	Kind          string `json:"kind"`
	RequestID     string `json:"requestId"`
	Reason        string `json:"reason"`
	Evidence      string `json:"evidence"`
	Run           string `json:"run"`
	MarkerSHA256  string `json:"markerSha256"`
	MarkerModTime string `json:"markerModTime"`
	MarkerBoot    string `json:"markerBoot"`
	CurrentBoot   string `json:"currentBoot"`
	RecordedAt    string `json:"recordedAt"`
}

// ReaderRecoveryRequest is the operator attestation (CAL-V0-226). The CLI
// validates each field's grammar.
type ReaderRecoveryRequest struct {
	RequestID string
	Reason    string
	Evidence  string
}

// hostIdentity is replaced only by tests that simulate a reboot.
var hostIdentity = ReaderHostIdentity

// readerProcessIdentity is replaced only by tests of an unreadable identity.
var readerProcessIdentity = supervisor.ProcessIdentity

// ReaderHostIdentity returns sha256 digests of this host's machine identity
// (bound to its hostname) and of its current boot; each is empty when the
// host does not report it (CAL-V0-227).
func ReaderHostIdentity() (machine, boot string) {
	m, b := hostBootIdentity()
	if m != "" {
		host, _ := os.Hostname()
		machine = digestHex("corvint-reader-machine/0\x00" + host + "\x00" + m)
	}
	if b != "" {
		boot = digestHex("corvint-reader-boot/0\x00" + b)
	}
	return machine, boot
}

func digestHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// rebootProof is the only automatic containment proof (CAL-V0-227): the
// marker names this machine and a boot, and this host now reports a
// different boot. Absent evidence on either side is never a proof.
func rebootProof(m readerMarker) (currentBoot string, ok bool) {
	machine, boot := hostIdentity()
	if m.Machine == "" || m.Boot == "" || machine == "" || boot == "" {
		return "", false
	}
	return boot, machine == m.Machine && boot != m.Boot
}

// resolveReaderByReboot applies the reboot proof. The caller holds the
// program lock. A marker without the proof is left untouched for
// checkReaderQuarantine to refuse; a proof that cannot be recorded is an
// error, and the marker stays.
func resolveReaderByReboot(dir, program string, now time.Time) (*ReaderRecovery, error) {
	mark, err := readReaderMarker(dir, program)
	if err != nil {
		return nil, nil
	}
	current, ok := rebootProof(mark.marker)
	if !ok {
		return nil, nil
	}
	rec := newRecovery(mark, program, RecoveryHostReboot, now)
	rec.Reason = "the host rebooted after the reader marker was written, so the reader and every descendant ended"
	rec.Evidence = "boot " + mark.marker.Boot + " -> " + current
	rec.CurrentBoot = current
	name := "reboot-" + mark.marker.Run + ".json"
	if err := writeRecovery(dir, name, rec); errors.Is(err, fs.ErrExist) {
		// An earlier start recorded this proof and stopped before the
		// removal: the record must attest this exact marker.
		prior, err := readRecovery(dir, name)
		if err != nil {
			return nil, err
		}
		if prior.MarkerSHA256 != rec.MarkerSHA256 || prior.MarkerModTime != rec.MarkerModTime || prior.Kind != RecoveryHostReboot {
			return nil, errors.New("a reboot record for this run attests a different marker")
		}
		rec = *prior
	} else if err != nil {
		return nil, err
	}
	if err := clearReaderMarker(mark, program); err != nil {
		return nil, err
	}
	if err := syncDir(dir); err != nil {
		return nil, err
	}
	return &rec, nil
}

// RecoverReader is the operator recovery (CAL-V0-226). It refuses while a
// dispatcher holds the program lock, records the attestation durably, and
// only then removes the exact marker it read. A replay of the same request
// ID finishes an interrupted removal or returns the original record.
func RecoverReader(c *Config, program string, req ReaderRecoveryRequest, now time.Time) (*ReaderRecovery, bool, error) {
	if !platformSupported {
		return nil, false, wire.Errorf(wire.CodeUnsupported, "recover-reader", "dispatch is supported only on darwin and linux")
	}
	if !ValidName(program) {
		return nil, false, wire.Errorf(wire.CodeMalformed, "--program", "program name must match [a-z][a-z0-9-]{0,23}")
	}
	if req.RequestID == "" || req.Reason == "" || req.Evidence == "" {
		return nil, false, wire.Errorf(wire.CodeMalformed, "recover-reader", "requires --request-id, --reason and --evidence")
	}
	dir := ProgramDir(c, program)
	if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil, false, wire.Errorf(wire.CodeMalformed, "--program", "no reader quarantine for program %s: %s does not exist", program, dir)
	}
	lock, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, false, err
	}
	defer lock.Close()
	if err := lockExclusive(lock); err != nil {
		return nil, false, wire.Errorf(wire.CodeResourceCollision, "--program", "a dispatcher for program %s holds %s (%v); stop it before recovering its reader", program, dir, err)
	}
	name := "operator-" + digestHex(req.RequestID)[:32] + ".json"
	prior, err := readRecovery(dir, name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, false, err
	}
	same := prior != nil && prior.Program == program && prior.Kind == RecoveryOperator && prior.RequestID == req.RequestID && prior.Reason == req.Reason && prior.Evidence == req.Evidence
	mark, err := quarantineEvidence(dir, program)
	if errors.Is(err, fs.ErrNotExist) {
		switch {
		case same:
			return prior, true, nil
		case prior != nil:
			return nil, false, wire.Errorf(wire.CodeRequestIDConflict, "--request-id", "request ID %s recorded a different recovery", req.RequestID)
		}
		return nil, false, wire.Errorf(wire.CodeMalformed, "--program", "no reader quarantine for program %s", program)
	}
	if err != nil {
		return nil, false, err
	}
	if err := readerLeaderLive(dir, program); err != nil {
		return nil, false, err
	}
	rec := newRecovery(mark, program, RecoveryOperator, now)
	rec.RequestID, rec.Reason, rec.Evidence = req.RequestID, req.Reason, req.Evidence
	replayed := false
	switch {
	case prior != nil && (!same || prior.MarkerSHA256 != rec.MarkerSHA256 || prior.MarkerModTime != rec.MarkerModTime):
		return nil, false, wire.Errorf(wire.CodeRequestIDConflict, "--request-id", "request ID %s recorded a different recovery; use a new request ID for this quarantine", req.RequestID)
	case prior != nil:
		rec, replayed = *prior, true
	default:
		if err := writeRecovery(dir, name, rec); err != nil {
			return nil, false, err
		}
	}
	// CAL-V0-940: the next start announces the recovery; the link precedes
	// the removal, so a recovery that lifted a quarantine is never silent.
	if err := linkUnannounced(dir, name); err != nil {
		return nil, false, err
	}
	if err := clearReaderMarker(mark, program); err != nil {
		return nil, false, err
	}
	if err := syncDir(dir); err != nil {
		return nil, false, err
	}
	return &rec, replayed, nil
}

// linkUnannounced hard-links the recovery record name into
// reader-recoveries/unannounced/ with the directory fsynced. An existing
// link is kept, so a replay that finishes an interrupted removal links
// nothing twice (CAL-V0-940).
func linkUnannounced(dir, name string) error {
	recDir := filepath.Join(dir, readerRecoveryDir)
	pending := filepath.Join(recDir, readerUnannouncedDir)
	if err := os.MkdirAll(pending, 0o700); err != nil {
		return err
	}
	if err := syncDir(recDir); err != nil {
		return err
	}
	if err := os.Link(filepath.Join(recDir, name), filepath.Join(pending, name)); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	return syncDir(pending)
}

// announceReaderRecoveries emits one reader-recovered event per linked
// recovery record, in name order and at most maxAnnouncedRecoveries per
// start, and removes each link only after its event line is appended. A
// linked record that does not decode stays linked and is reported by an
// alert (CAL-V0-940). The caller holds the program lock and has passed the
// quarantine check.
func (d *Dispatcher) announceReaderRecoveries() {
	pending := filepath.Join(d.dir, readerRecoveryDir, readerUnannouncedDir)
	f, err := os.Open(pending)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		d.emit(Event{Kind: "alert", Message: "unannounced reader recoveries not read: " + err.Error()})
		return
	}
	entries, err := f.ReadDir(maxAnnouncedRecoveries)
	f.Close()
	if err != nil && !errors.Is(err, io.EOF) {
		d.emit(Event{Kind: "alert", Message: "unannounced reader recoveries not read: " + err.Error()})
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	removed := false
	for _, name := range names {
		rec, err := ReadReaderRecovery(d.dir, name)
		if err != nil {
			d.emit(Event{Kind: "alert", Message: fmt.Sprintf("reader recovery %s is linked for announcement but unreadable; it stays linked: %v", name, err)})
			continue
		}
		e := Event{Kind: "reader-recovered", Message: fmt.Sprintf("reader run %s quarantine lifted by %s attestation recorded at %s: %s (%s); recorded in %s", orText(rec.Run, "unknown"), rec.Kind, rec.RecordedAt, rec.Reason, rec.Evidence, readerRecoveryDir),
			Detail: map[string]string{"kind": rec.Kind, "run": rec.Run, "requestId": rec.RequestID, "recordedAt": rec.RecordedAt}}
		if d.record(e) != nil {
			continue
		}
		if os.Remove(filepath.Join(pending, name)) == nil {
			removed = true
		}
	}
	if removed {
		_ = syncDir(pending)
	}
}

// readerLeaderLive applies the live evidence, which outranks the
// attestation whatever the marker's state: a recorded leader that still runs
// with its start identity, or whose identity or record cannot be read,
// keeps the quarantine. A record of a newer version is UNSUPPORTED_VERSION.
func readerLeaderLive(dir, program string) error {
	leader, err := readReaderLeader(dir, program)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if wire.CodeOf(err) == wire.CodeUnsupportedVersion {
		return err
	}
	if err != nil {
		return wire.Errorf(wire.CodeQuiescenceUnproved, "--program", "reader leader record %s cannot be read (%v); the quarantine stays; inspect it by hand", filepath.Join(dir, readerLeaderName), err)
	}
	id, err := readerProcessIdentity(leader.ReaderPID)
	if err != nil {
		return wire.Errorf(wire.CodeQuiescenceUnproved, "--program", "reader pid %d identity cannot be read (%v); the quarantine stays", leader.ReaderPID, err)
	}
	// A Linux btime one second off the recorded start is the same reader;
	// a matching start tick with btime further off is unknown, so the
	// quarantine stays (PSR-V0-227).
	switch supervisor.CompareIdentity(id, leader.ReaderStart) {
	case supervisor.IdentitySame:
		return wire.Errorf(wire.CodeQuiescenceUnproved, "--program", "reader pid %d (run %s) is still running; stop it and its descendants before recovering", leader.ReaderPID, leader.Run)
	case supervisor.IdentityUnknown:
		return wire.Errorf(wire.CodeQuiescenceUnproved, "--program", "reader pid %d (run %s) identity is uncertain: its start tick matches but its boot time differs; the quarantine stays", leader.ReaderPID, leader.Run)
	}
	return nil
}

// quarantineEvidence is the marker an operator attests. A marker an
// interrupted removal left aside comes first, as its exact bytes; then a
// strictly decoded marker, or the exact bytes of a malformed one (Run
// empty). A missing, non-regular or oversized marker is not attested, and a
// newer marker version is UNSUPPORTED_VERSION (CAL-V0-229).
func quarantineEvidence(dir, program string) (*readerEvidence, error) {
	path := filepath.Join(dir, readerMarkerName+readerClearingSuffix)
	if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
		path = filepath.Join(dir, readerMarkerName)
		mark, err := readReaderMarker(dir, program)
		if err == nil || errors.Is(err, fs.ErrNotExist) || wire.CodeOf(err) == wire.CodeUnsupportedVersion {
			return mark, err
		}
	}
	raw, identity, rawErr := rawReaderMarker(path)
	if rawErr != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, path, "reader marker cannot be attested: %v; inspect it by hand", rawErr)
	}
	if len(raw) > 4096 {
		return nil, wire.Errorf(wire.CodeLimitExceeded, path, "reader marker exceeds 4096 bytes; inspect it by hand")
	}
	return &readerEvidence{path: path, raw: raw, identity: identity}, nil
}

func newRecovery(mark *readerEvidence, program, kind string, now time.Time) ReaderRecovery {
	sum := sha256.Sum256(mark.raw)
	return ReaderRecovery{
		Profile: ReaderRecoveryProfile, Program: program, Kind: kind,
		Run: mark.marker.Run, MarkerSHA256: hex.EncodeToString(sum[:]),
		MarkerModTime: mark.identity.ModTime().UTC().Format(time.RFC3339Nano),
		MarkerBoot:    mark.marker.Boot, RecordedAt: now.UTC().Format(time.RFC3339),
	}
}

// writeRecovery creates the record exclusively: the bytes are fsynced in a
// temporary file, linked to the final name (fs.ErrExist when it exists) and
// the directory is fsynced, all before any marker removal.
func writeRecovery(dir, name string, rec ReaderRecovery) error {
	recDir := filepath.Join(dir, readerRecoveryDir)
	if err := os.MkdirAll(recDir, 0o700); err != nil {
		return err
	}
	if err := syncDir(dir); err != nil {
		return err
	}
	raw, _ := json.Marshal(rec)
	tmp, err := os.CreateTemp(recDir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(raw); err != nil {
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
	if err := os.Link(tmp.Name(), filepath.Join(recDir, name)); err != nil {
		return err
	}
	return syncDir(recDir)
}

// readRecovery strictly decodes one record.
// ReadReaderRecovery reads one attestation record from the program state
// directory's reader-recoveries/ by file name, strictly (CAL-V0-229).
func ReadReaderRecovery(dir, name string) (*ReaderRecovery, error) {
	if name != filepath.Base(name) || !strings.HasSuffix(name, ".json") {
		return nil, wire.Errorf(wire.CodeMalformed, name, "reader recovery name must be a plain .json file name")
	}
	return readRecovery(dir, name)
}

func readRecovery(dir, name string) (*ReaderRecovery, error) {
	path := filepath.Join(dir, readerRecoveryDir, name)
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("reader recovery %s is not a regular file", path)
	}
	f, err := openReaderMarker(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 32769))
	if err != nil {
		return nil, err
	}
	var rec ReaderRecovery
	if len(raw) > 32768 || !validScalarJSON(raw) {
		return nil, fmt.Errorf("malformed reader recovery %s", path)
	}
	if err := wire.RawProfileVersion("/profile", raw, ReaderRecoveryProfile); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&rec); err != nil {
		return nil, fmt.Errorf("malformed reader recovery %s: %v", path, err)
	}
	if canonical, _ := json.Marshal(rec); !bytes.Equal(raw, canonical) || rec.Profile != ReaderRecoveryProfile {
		return nil, fmt.Errorf("malformed reader recovery %s", path)
	}
	return &rec, nil
}
