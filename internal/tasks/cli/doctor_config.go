package cli

import (
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The doctor configuration (TQD-V0-020) is an operator file named by
// --config: thresholds for the operational detectors, the notify hook and
// the dispatcher state directories whose worker sessions and work-state
// health the doctor reads. It is only read.
const (
	doctorConfigProfile  = "taskman-doctor-config/0"
	doctorConfigBytes    = 64 << 10
	doctorMaxDispatchers = 8
	doctorMaxNotifyArgs  = 64
)

// doctorThresholds holds the settable bounds; defaults are TQD-V0-020's.
type doctorThresholds struct {
	lockHeld, slowCommand, noCompletions, sessionWindow time.Duration
	fdPercent, retryStorm, hostErrors, earlyRefusal     int
	// heldIdleTicks and queueDeepTicks are the TQD-V0-043/044 streaks.
	heldIdleTicks, queueDeepTicks int
}

// doctorThresholdKeys lists each threshold member, its default and its
// inclusive bounds; a seconds member is a duration.
var doctorThresholdKeys = []struct {
	key      string
	def, max int64
	seconds  bool
}{
	{"earlyRefusal", 3, 1 << 20, false},
	{"fdPressurePercent", 80, 100, false},
	{"hostErrors", 3, 1 << 20, false},
	{"lockHeldSeconds", 60, 30 * 86400, true},
	{"noCompletionsSeconds", 7200, 30 * 86400, true},
	{"retryStorm", 3, 1 << 20, false},
	{"roleQueueDeepTicks", 3, 1 << 20, false},
	{"runnableHeldIdleTicks", 3, 1 << 20, false},
	{"sessionWindowSeconds", 3600, 7 * 86400, true},
	{"slowCommandSeconds", 60, 30 * 86400, true},
}

type doctorDispatcher struct {
	programDir, format string
}

type doctorNotifyConfig struct {
	argv        []string
	minInterval time.Duration
}

type doctorConfig struct {
	thresholds  doctorThresholds
	notify      *doctorNotifyConfig
	dispatchers []doctorDispatcher
}

func defaultDoctorConfig() *doctorConfig {
	c, _ := decodeDoctorConfig(wire.ObjectValue(wire.NewObject().Set("profile", wire.String(doctorConfigProfile))), "")
	return c
}

// readDoctorConfig reads and decodes the --config file; relative paths in
// it resolve against the file's directory.
func readDoctorConfig(path string) (*doctorConfig, error) {
	raw, err := readBoundedFile("", "--config", path, doctorConfigBytes)
	if err != nil {
		return nil, err
	}
	v, err := wire.ParseInput(raw)
	if err != nil {
		return nil, wire.Errorf(wire.CodeMalformed, "--config", "doctor config: %v", err)
	}
	return decodeDoctorConfig(v, filepath.Dir(path))
}

func doctorConfigErr(where, format string, args ...any) error {
	return wire.Errorf(wire.CodeMalformed, "--config"+where, format, args...)
}

func doctorClosed(v wire.Value, where string, required []string, optional ...string) error {
	if v.Kind != wire.KindObject || v.Obj == nil {
		return doctorConfigErr(where, "not an object")
	}
	allowed := append(append([]string{}, required...), optional...)
	for _, k := range v.Obj.SortedKeys() {
		if !slices.Contains(allowed, k) {
			return doctorConfigErr(where, "unknown member %q", k)
		}
	}
	for _, k := range required {
		if _, ok := v.Obj.Get(k); !ok {
			return doctorConfigErr(where, "missing member %q", k)
		}
	}
	return nil
}

// doctorMember reads an optional member of a possibly absent object.
func doctorMember(v wire.Value, key string) (wire.Value, bool) {
	if v.Kind != wire.KindObject || v.Obj == nil {
		return wire.Value{}, false
	}
	return v.Obj.Get(key)
}

func doctorConfigString(v wire.Value, where string) (string, error) {
	if v.Kind != wire.KindString {
		return "", doctorConfigErr(where, "not a string")
	}
	return wire.ParseProse("--config"+where, v.Str, 1, 4096)
}

func doctorConfigCount(v wire.Value, where string, min, max int64) (int64, error) {
	if v.Kind != wire.KindString {
		return 0, doctorConfigErr(where, "not a decimal string")
	}
	c, err := wire.ParseCount("--config"+where, v.Str)
	if err != nil {
		return 0, err
	}
	if n := c.Int(); n < min || n > max {
		return 0, doctorConfigErr(where, "%d is outside %d..%d", n, min, max)
	}
	return c.Int(), nil
}

func decodeDoctorConfig(v wire.Value, base string) (*doctorConfig, error) {
	if err := wire.ProfileVersion("--config/profile", v, doctorConfigProfile); err != nil {
		return nil, err
	}
	if err := doctorClosed(v, "", []string{"profile"}, "thresholds", "notify", "dispatchers"); err != nil {
		return nil, err
	}
	if p, _ := v.Obj.Get("profile"); p.Kind != wire.KindString || p.Str != doctorConfigProfile {
		return nil, doctorConfigErr("/profile", "not %s", doctorConfigProfile)
	}
	c := &doctorConfig{}
	th, hasTh := v.Obj.Get("thresholds")
	if hasTh {
		keys := make([]string, 0, len(doctorThresholdKeys))
		for _, k := range doctorThresholdKeys {
			keys = append(keys, k.key)
		}
		if err := doctorClosed(th, "/thresholds", nil, keys...); err != nil {
			return nil, err
		}
	}
	for _, k := range doctorThresholdKeys {
		n := k.def
		if x, ok := doctorMember(th, k.key); ok {
			var err error
			if n, err = doctorConfigCount(x, "/thresholds/"+k.key, 1, k.max); err != nil {
				return nil, err
			}
		}
		d := time.Duration(n) * time.Second
		switch k.key {
		case "earlyRefusal":
			c.thresholds.earlyRefusal = int(n)
		case "fdPressurePercent":
			c.thresholds.fdPercent = int(n)
		case "hostErrors":
			c.thresholds.hostErrors = int(n)
		case "lockHeldSeconds":
			c.thresholds.lockHeld = d
		case "noCompletionsSeconds":
			c.thresholds.noCompletions = d
		case "retryStorm":
			c.thresholds.retryStorm = int(n)
		case "roleQueueDeepTicks":
			c.thresholds.queueDeepTicks = int(n)
		case "runnableHeldIdleTicks":
			c.thresholds.heldIdleTicks = int(n)
		case "sessionWindowSeconds":
			c.thresholds.sessionWindow = d
		case "slowCommandSeconds":
			c.thresholds.slowCommand = d
		}
	}
	if n, ok := v.Obj.Get("notify"); ok {
		if err := doctorClosed(n, "/notify", []string{"command"}, "minIntervalSeconds"); err != nil {
			return nil, err
		}
		cmd, _ := n.Obj.Get("command")
		if cmd.Kind != wire.KindArray || len(cmd.Arr) == 0 || len(cmd.Arr) > doctorMaxNotifyArgs {
			return nil, doctorConfigErr("/notify/command", "not an argv array of 1..%d strings", doctorMaxNotifyArgs)
		}
		nc := &doctorNotifyConfig{}
		for i, a := range cmd.Arr {
			s, err := doctorConfigString(a, "/notify/command/"+strconv.Itoa(i))
			if err != nil {
				return nil, err
			}
			nc.argv = append(nc.argv, s)
		}
		if !filepath.IsAbs(nc.argv[0]) {
			return nil, doctorConfigErr("/notify/command/0", "the hook program must be an absolute path")
		}
		if m, ok := n.Obj.Get("minIntervalSeconds"); ok {
			secs, err := doctorConfigCount(m, "/notify/minIntervalSeconds", 0, 7*86400)
			if err != nil {
				return nil, err
			}
			nc.minInterval = time.Duration(secs) * time.Second
		}
		c.notify = nc
	}
	if d, ok := v.Obj.Get("dispatchers"); ok {
		if d.Kind != wire.KindArray || len(d.Arr) == 0 || len(d.Arr) > doctorMaxDispatchers {
			return nil, doctorConfigErr("/dispatchers", "not an array of 1..%d dispatchers", doctorMaxDispatchers)
		}
		for i, e := range d.Arr {
			where := "/dispatchers/" + strconv.Itoa(i)
			if err := doctorClosed(e, where, []string{"programDir"}, "format"); err != nil {
				return nil, err
			}
			sv, _ := e.Obj.Get("programDir")
			dir, err := doctorConfigString(sv, where+"/programDir")
			if err != nil {
				return nil, err
			}
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(base, dir)
			}
			dd := doctorDispatcher{programDir: filepath.Clean(dir)}
			if fv, ok := e.Obj.Get("format"); ok {
				f, err := doctorConfigString(fv, where+"/format")
				if err != nil {
					return nil, err
				}
				if !slices.Contains(dispatch.UsageFormats, f) {
					return nil, doctorConfigErr(where+"/format", "format %q is not one of %v", f, dispatch.UsageFormats)
				}
				dd.format = f
			}
			c.dispatchers = append(c.dispatchers, dd)
		}
	}
	return c, nil
}
