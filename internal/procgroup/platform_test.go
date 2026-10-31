package procgroup

import (
	"context"
	"errors"
	"runtime"
	"testing"
)

// V1-2487: only the platform refusal matches ErrPlatformUnsupported, so a caller can report the
// platform limit without mistaking another process failure for it.
func TestProcessErrorMatchesPlatformSentinelOnlyForPlatformRefusal(t *testing.T) {
	if !errors.Is(&processError{Code: "process-platform-unsupported"}, ErrPlatformUnsupported) {
		t.Fatal("platform refusal does not match ErrPlatformUnsupported")
	}
	if errors.Is(&processError{Code: "process-spec-invalid", Cause: errors.New("x")}, ErrPlatformUnsupported) {
		t.Fatal("an invalid spec matched ErrPlatformUnsupported")
	}
}

// V1-2487: on an unsupported platform Run's refusal is ErrPlatformUnsupported with the listed text.
func TestRunPlatformRefusalIsTheSentinel(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the platform refusal is compiled on unsupported targets")
	}
	observation := Run(context.Background(), Spec{})
	if !errors.Is(observation.Err, ErrPlatformUnsupported) || observation.Err.Error() != "process-platform-unsupported" {
		t.Fatalf("platform refusal is not ErrPlatformUnsupported: %v", observation.Err)
	}
}
