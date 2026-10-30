//go:build windows

package releasegate

func gitContainmentSupported() bool { return false }

func gitContainmentUnsupportedReason() string {
	return "Windows Git descendant containment is unavailable; release gate is unsupported"
}
