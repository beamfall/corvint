package appflows

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// V1-2487, PRS-V1-021: a Git read the platform refuses reports the procgroup refusal, never
// "evaluated revision is not a commit", "not a Git checkout" or another input defect.
func TestGitPlatformRefusalIsNotReportedAsARevisionDefect(t *testing.T) {
	saved := runGit
	t.Cleanup(func() { runGit = saved })
	runGit = func(context.Context, procgroup.Spec) procgroup.Observation {
		return procgroup.Observation{ExitStatus: -1, Err: procgroup.ErrPlatformUnsupported}
	}
	root := t.TempDir()
	ctx := context.Background()
	_, resolveErr := ResolveRevision(ctx, root, "HEAD")
	_, loadErr := LoadIntentsAt(ctx, root, "flows", "HEAD")
	for name, err := range map[string]error{"resolve": resolveErr, "load": loadErr} {
		var typed *gokernel.Error
		if !errors.As(err, &typed) || typed.Code != "process-platform-unsupported" {
			t.Fatalf("%s: want code process-platform-unsupported, got %#v", name, err)
		}
		if strings.Contains(err.Error(), "not a commit") || !strings.HasSuffix(err.Error(), ": process-platform-unsupported") {
			t.Fatalf("%s: diagnostic misattributes the platform refusal: %q", name, err)
		}
	}
	if _, _, reason := OpenCheckout(ctx, root, root); reason != "flow Git source unavailable: process-platform-unsupported" {
		t.Fatalf("checkout reason misattributes the platform refusal: %q", reason)
	}
}

// PRS-V1-920 (V1-2683): a docs-maintain maintenance-file read refused by the
// receipt platform boundary is a typed gokernel error the flows CLI keeps,
// not an argument error; other read failures are unchanged.
func TestPRSV1920MaintenanceReadReportsThePlatformRefusal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/page.md", []byte("page\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	saved := readReceiptFile
	t.Cleanup(func() { readReceiptFile = saved })
	readReceiptFile = func(*os.Root, string) ([]byte, error) {
		return nil, fmt.Errorf("open: %w", testvaliditydoc.ErrPlatformUnsupported)
	}
	_, _, err = maintenanceFile(root, "page.md", false)
	var coded *gokernel.Error
	if !errors.As(err, &coded) || coded.Code != testvaliditydoc.PlatformUnsupportedCode || coded.Message != testvaliditydoc.ErrPlatformUnsupported.Error() || !isPlatformRefusal(err) {
		t.Fatalf("got %v, want %s", err, testvaliditydoc.PlatformUnsupportedCode)
	}
	other := errors.New("unreadable")
	readReceiptFile = func(*os.Root, string) ([]byte, error) { return nil, other }
	if _, _, err = maintenanceFile(root, "page.md", false); err != other || isPlatformRefusal(err) {
		t.Fatalf("got %v, want the unchanged read error", err)
	}
}
