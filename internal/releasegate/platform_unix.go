//go:build !windows

package releasegate

// A process group cannot contain a hostile Git helper that calls setsid(2).
// Until a portable job/sandbox primitive is supplied, product scans fail
// closed rather than overclaim descendant containment.
func gitContainmentSupported() bool { return gitContainmentTestOverride }

func gitContainmentUnsupportedReason() string {
	return "Unix process groups cannot contain a hostile Git helper that calls setsid(2); deterministic descendant containment is unavailable"
}
