package dispatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// DrainSignalProfile is the ATR-V0-058 drain-pending signal's profile.
const DrainSignalProfile = "taskman-dispatch-drain-pending/0"

const (
	drainSignalFile = "drain-pending.json"
	maxDrainSignal  = 1024
	// DrainNotice is the signal's fixed instruction to a live worker.
	DrainNotice = "the dispatcher's service is draining: finish the current step, release the lease and end; no new work will be launched"
)

// DrainObserver is an optional LaunchControl extension that reports whether
// the control is draining (ATR-V0-058). An error leaves the signal as it is.
type DrainObserver interface {
	Draining() (bool, error)
}

// DrainSignal is the drain-pending signal file.
type DrainSignal struct {
	Profile string `json:"profile"`
	Program string `json:"program"`
	Since   string `json:"since"`
	Notice  string `json:"notice"`
}

// DrainSignalPath is the signal file under the program's state directory.
// Every worker receives it as CORVINT_DISPATCH_DRAIN_FILE.
func DrainSignalPath(dir string) string { return filepath.Join(dir, drainSignalFile) }

// ReadDrainSignal reads the signal at path. A missing file is
// fs.ErrNotExist; another version is UNSUPPORTED_VERSION (CAL-V0-131).
func ReadDrainSignal(path string) (*DrainSignal, error) {
	raw, err := readBounded(path, maxDrainSignal)
	if err != nil {
		return nil, err
	}
	if err := wire.RawProfileVersion("/profile", raw, DrainSignalProfile); err != nil {
		return nil, err
	}
	var s DrainSignal
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	if _, err := time.Parse(time.RFC3339, s.Since); err != nil || s.Program == "" || s.Notice == "" {
		return nil, errors.New("drain signal is incomplete")
	}
	return &s, nil
}

// syncDrainSignal writes the signal while the dispatcher's control is
// draining and removes it otherwise (ATR-V0-058). An uncontrolled
// dispatcher never drains. A failed observation keeps the file as it is.
func (d *Dispatcher) syncDrainSignal() {
	draining := false
	if o, ok := d.control.(DrainObserver); ok {
		var err error
		if draining, err = o.Draining(); err != nil {
			return
		}
	}
	path := DrainSignalPath(d.dir)
	if !draining {
		if err := os.Remove(path); err == nil {
			d.emit(Event{Kind: "alert", Message: "the service drain was lifted; the drain-pending signal " + path + " is removed", Detail: map[string]string{"drain": "LIFTED"}})
		} else if !errors.Is(err, fs.ErrNotExist) {
			d.drainAlertOnce("the drain-pending signal " + path + " could not be removed: " + err.Error())
		}
		return
	}
	if _, err := os.Stat(path); err == nil {
		return
	}
	raw, _ := json.Marshal(DrainSignal{Profile: DrainSignalProfile, Program: d.Program, Since: d.Now().UTC().Format(time.RFC3339), Notice: DrainNotice})
	if err := writeAtomic(path, append(raw, '\n')); err != nil {
		d.drainAlertOnce("the drain-pending signal " + path + " could not be written: " + err.Error())
		return
	}
	d.drainAlerted = false
	d.emit(Event{Kind: "alert", Message: "the service is draining; live workers can read the drain-pending signal " + path, Detail: map[string]string{"drain": "PENDING"}})
}

func (d *Dispatcher) drainAlertOnce(msg string) {
	if !d.drainAlerted {
		d.drainAlerted = true
		d.emit(Event{Kind: "alert", Message: msg})
	}
}
