package appmap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/procgroup"
)

// V1-2487, PRS-V1-021: a Git read the platform refuses reports the procgroup refusal, never
// "evaluated revision is not a commit" or another input defect.
func TestGitPlatformRefusalIsNotReportedAsARevisionDefect(t *testing.T) {
	saved := runGit
	t.Cleanup(func() { runGit = saved })
	runGit = func(context.Context, procgroup.Spec) procgroup.Observation {
		return procgroup.Observation{ExitStatus: -1, Err: procgroup.ErrPlatformUnsupported}
	}
	root := t.TempDir()
	ctx := context.Background()
	_, buildErr := Build(ctx, root, "appmap.json", "HEAD")
	_, impactErr := Impact(ctx, &Map{}, nil, "HEAD~1", Options{Root: root})
	_, _, rootErr := openRoot(ctx, root, "app", t.TempDir(), "")
	for name, err := range map[string]error{"build": buildErr, "impact": impactErr, "root": rootErr} {
		var typed *gokernel.Error
		if !errors.As(err, &typed) || typed.Code != "process-platform-unsupported" {
			t.Fatalf("%s: want code process-platform-unsupported, got %#v", name, err)
		}
		if strings.Contains(err.Error(), "not a commit") || !strings.HasSuffix(err.Error(), ": process-platform-unsupported") {
			t.Fatalf("%s: diagnostic misattributes the platform refusal: %q", name, err)
		}
	}
}
