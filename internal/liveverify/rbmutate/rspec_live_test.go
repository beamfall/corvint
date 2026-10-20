package rbmutate

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// liveImageVariable names a local container image that carries a real RSpec
// on PATH. The live qualification below runs only when it is set; it never
// pulls or installs anything (`--pull never`, `--network none`).
const liveImageVariable = "CORVINT_TEST_RSPEC_IMAGE"

// liveDirVariable optionally names a host directory the container engine can
// mount; Docker Desktop shares neither /private/tmp nor /var/folders by
// default, so the macOS run points this under the home directory.
const liveDirVariable = "CORVINT_TEST_RSPEC_DIR"

func liveTempDir(t *testing.T) string {
	t.Helper()
	parent := os.Getenv(liveDirVariable)
	if parent == "" {
		return t.TempDir()
	}
	dir, err := os.MkdirTemp(parent, "rbmutate-live-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// dockerCopy is a Copy whose runs execute in a network-less container with
// only the export and its scratch directory mounted, at their host paths.
type dockerCopy struct {
	image   string
	dir     string
	scratch string
}

func (copy dockerCopy) Dir() string     { return copy.dir }
func (copy dockerCopy) Scratch() string { return copy.scratch }
func (dockerCopy) Unsandboxed() string  { return "" }
func (copy dockerCopy) Run(ctx context.Context, timeout time.Duration, dir string, environment []string, argv ...string) (string, bool, error) {
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	arguments := []string{"run", "--rm", "--pull", "never", "--network", "none",
		"-v", copy.dir + ":" + copy.dir, "-v", copy.scratch + ":" + copy.scratch, "-w", dir}
	for _, entry := range environment {
		// The host's search paths mean nothing inside the image.
		name, _, _ := strings.Cut(entry, "=")
		switch name {
		case "PATH", "HOME", "GEM_HOME", "GEM_PATH", "BUNDLE_PATH", "SystemRoot", "USERPROFILE":
			continue
		}
		arguments = append(arguments, "-e", entry)
	}
	// The scratch directory is emptied inside the container: Docker
	// Desktop's file sharing keeps a deleted host file's name unwritable
	// from the VM, so a host-side delete would break the next report.
	arguments = append(arguments, copy.image, "sh", "-c", `find "$0" -mindepth 1 -delete && exec "$@"`, copy.scratch)
	arguments = append(arguments, argv...)
	output, err := exec.CommandContext(runCtx, "docker", arguments...).CombinedOutput()
	if runCtx.Err() != nil && errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return string(output), true, err
	}
	return string(output), false, err
}

// TestFPKV0069LiveRSpecQualification runs the issue #1321 weak-spec fixture
// through a real RSpec (FPK-V0-069 live evidence). Opt in with
// CORVINT_TEST_RSPEC_IMAGE=<local image with rspec on PATH>.
func TestFPKV0069LiveRSpecQualification(t *testing.T) {
	image := os.Getenv(liveImageVariable)
	if image == "" {
		t.Skip(liveImageVariable + " is not set")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker is not available")
	}
	dir, scratch := liveTempDir(t), liveTempDir(t)
	for name, content := range map[string]string{"app/models/scoring.rb": fixtureScoring, "spec/models/scoring_spec.rb": fixtureWeakSpec} {
		target := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	request := Request{
		ChangedPath: "app/models/scoring.rb", TestPath: "spec/models/scoring_spec.rb",
		Lines: bandLines, MaxMutants: 14, Budget: 8 * time.Minute, RSpec: []string{"rspec"},
	}
	copy := dockerCopy{image: image, dir: dir, scratch: scratch}
	if output, _, err := copy.Run(context.Background(), time.Minute, dir, nil, "test", "-f", "spec/models/scoring_spec.rb"); err != nil {
		t.Fatalf("the container cannot see %s (set %s to a shared directory): %v %s", dir, liveDirVariable, err, output)
	}
	report, err := Judge(context.Background(), copy, request)
	if err != nil {
		t.Fatal(err)
	}
	if report.Stage != StageMutants || report.Runner != "rspec" || report.Killed != 6 || report.Survived != 8 {
		t.Fatalf("report: %+v %v", report, outcomes(report))
	}
	got := outcomes(report)
	for key, outcome := range weakSpecOutcomes {
		if got[key] != outcome {
			t.Errorf("%s: got %q, want %q", key, got[key], outcome)
		}
	}
	if report.Baseline == nil || report.Baseline.Examples != 3 || report.Baseline.Failures != 0 {
		t.Fatalf("baseline receipt: %+v", report.Baseline)
	}
	if content, _ := os.ReadFile(filepath.Join(dir, "app/models/scoring.rb")); string(content) != fixtureScoring {
		t.Fatal("the changed file was not restored")
	}
	t.Logf("live: %s (%s)", report.Detail, report.Elapsed)
}
