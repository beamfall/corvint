//go:build !darwin && !linux

package cli

// Host lock and file-table probes are not implemented here; their
// detectors report nothing (TQD-V0-021, TQD-V0-024).
var (
	doctorLockHolder  = func(string) (bool, int, error) { return false, 0, errDoctorUnsupported }
	doctorLockOpeners = func([]string, []int) (map[string][]int, error) { return nil, errDoctorUnsupported }
	doctorFileTable   = func() (uint64, uint64, error) { return 0, 0, errDoctorUnsupported }
)
