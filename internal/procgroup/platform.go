package procgroup

import "errors"

// ErrPlatformUnsupported is the refusal Run reports on a platform without a qualified process
// implementation. Callers match it with errors.Is to report the platform limit instead of
// misattributing the failure to their input (PRS-V1-021, PRS-V1-840, V1-2487).
var ErrPlatformUnsupported = errors.New("process-platform-unsupported")

func (failure *processError) Is(target error) bool {
	return target == ErrPlatformUnsupported && failure.Code == "process-platform-unsupported"
}
