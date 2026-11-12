package rubytestreceipt

import (
	"errors"
	"slices"
	"strings"
	"time"
)

// Runtime identity basis and closed unknown reasons (LPCV-V0-133).
const (
	RuntimeBasis = "exec-prefix-probe"

	RuntimeProbeFailed  = "probe-failed"
	RuntimeNotReported  = "not-reported"
	RuntimeNotInsidePfx = "not-observable-inside-prefix"
	// Image unknown reasons of a Corvint formatter run (LPCV-V0-138).
	RuntimeRunContainerUnknown = "run-container-unknown"
	RuntimeHostInspectUnavail  = "host-inspect-unavailable"
	RuntimeNotInspectable      = "container-not-inspectable"

	HostInspectTimeout = time.Minute

	RuntimeProbeTimeout = 5 * time.Minute
	RuntimeProbeOutput  = 64 << 10
	RuntimeProbeMarker  = "corvint-runtime-identity"
)

// VersionValue admits a bounded version string, the runnerVersion charset.
func VersionValue(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '.' || c == '+' || c == '-') {
			return false
		}
	}
	return true
}

// ImageDigest admits a docker image ID: sha256: and 64 lowercase hex digits.
func ImageDigest(value string) bool {
	digest, ok := strings.CutPrefix(value, "sha256:")
	return ok && BlobPattern("sha256", digest)
}

func ValidateRuntime(runtime *Runtime) error {
	if runtime == nil {
		return nil
	}
	if runtime.Basis != RuntimeBasis {
		return errors.New("Ruby provider runtime identity has an unknown basis")
	}
	containerValue := func(value string) bool { return BlobPattern("sha256", value) }
	probeReasons := []string{RuntimeProbeFailed, RuntimeNotReported, RuntimeNotInsidePfx}
	// Without a run container the image is never observed: any value is a
	// claim. With one, the image is the host inspection of it (LPCV-V0-138).
	imageValid, imageReasons := func(string) bool { return false }, []string{RuntimeNotInsidePfx}
	checks := []struct {
		fact    RuntimeFact
		valid   func(string) bool
		reasons []string
	}{
		{runtime.Ruby, VersionValue, probeReasons},
		{runtime.Bundler, VersionValue, probeReasons},
		{runtime.ProbeContainer, containerValue, probeReasons},
	}
	if runtime.RunContainer != nil {
		checks = append(checks, struct {
			fact    RuntimeFact
			valid   func(string) bool
			reasons []string
		}{*runtime.RunContainer, containerValue, []string{RuntimeNotReported}})
		imageReasons = []string{RuntimeRunContainerUnknown, RuntimeHostInspectUnavail, RuntimeNotInspectable}
		if runtime.RunContainer.Value != "" {
			imageValid = ImageDigest
		}
	}
	checks = append(checks, struct {
		fact    RuntimeFact
		valid   func(string) bool
		reasons []string
	}{runtime.Image, imageValid, imageReasons})
	for _, check := range checks {
		switch {
		case (check.fact.Value == "") == (check.fact.Unknown == ""):
			return errors.New("Ruby provider runtime fact must carry exactly one of value and unknown")
		case check.fact.Value != "" && !check.valid(check.fact.Value):
			return errors.New("Ruby provider runtime fact value is malformed")
		case check.fact.Unknown != "" && !slices.Contains(check.reasons, check.fact.Unknown):
			return errors.New("Ruby provider runtime fact reason is outside the closed set")
		}
	}
	return nil
}
