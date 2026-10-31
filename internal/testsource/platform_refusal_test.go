package testsource

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// V1-2487, PRS-V1-021: a Git read the platform refuses reports the procgroup refusal, never
// "the revision is not a commit".
func TestGitPlatformRefusalIsNotReportedAsARevisionDefect(t *testing.T) {
	saved := runGit
	t.Cleanup(func() { runGit = saved })
	runGit = func(context.Context, procgroup.Spec) procgroup.Observation {
		return procgroup.Observation{ExitStatus: -1, Err: procgroup.ErrPlatformUnsupported}
	}
	_, err := OpenRepo(context.Background(), t.TempDir()).Resolve("HEAD")
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != "tests-source-unavailable" || !errors.Is(err, procgroup.ErrPlatformUnsupported) {
		t.Fatalf("want tests-source-unavailable wrapping the platform refusal, got %#v", err)
	}
	if strings.Contains(err.Error(), "not a commit") || !strings.HasSuffix(err.Error(), ": process-platform-unsupported") {
		t.Fatalf("diagnostic misattributes the platform refusal: %q", err)
	}
}
