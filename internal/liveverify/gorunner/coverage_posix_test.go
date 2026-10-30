//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package gorunner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunCollectsRealUnitCoverage(t *testing.T) {
	tests := []struct {
		name       string
		testBody   string
		wantBlocks bool
	}{
		{"covered", `func TestValue(t *testing.T) { if Value() != 7 { t.Fatal("bad") } }`, true},
		{"zero observed", `func TestValue(t *testing.T) {}`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := realCoveragePlan(t, test.testBody, 30*time.Second)
			result, err := Run(context.Background(), plan)
			if err != nil {
				t.Fatalf("Run: %v; stderr=%s", err, result.Stderr.Data)
			}
			coverage := result.Coverage
			if coverage.Completeness != CoverageComplete || coverage.Mode != "UNIT_COVERPROFILE_SET" ||
				coverage.ArtifactSHA256 == "" || coverage.RawRootSHA256 == "" {
				t.Fatalf("coverage = %+v; result=%+v", coverage, result)
			}
			if len(coverage.Packages) != 1 || coverage.Packages[0] != "example.test/fixture" {
				t.Fatalf("packages = %#v", coverage.Packages)
			}
			raw, err := os.ReadFile(plan.Coverage.ProfilePath)
			if err != nil {
				t.Fatal(err)
			}
			hasObserved := strings.Contains(string(raw), " 1 1\n")
			if hasObserved != test.wantBlocks {
				t.Fatalf("observed block = %v, profile=%s", hasObserved, raw)
			}
			if coverage.Completeness == CoverageAbsent {
				t.Fatal("requested zero coverage collapsed to ABSENT")
			}
		})
	}
}

func TestCoverageRejectsFailClosedFilesystemAndProfileCases(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T, Plan) CoverageObservation
	}{
		{"missing target", func(t *testing.T, plan Plan) CoverageObservation {
			return preparedCapture(t, plan).finish(cleanCoverageResult(), nil)
		}},
		{"pre-existing target", func(t *testing.T, plan Plan) CoverageObservation {
			if err := os.WriteFile(plan.Coverage.ProfilePath, []byte("mode: set\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			result, err := Run(context.Background(), plan)
			if !errors.Is(err, ErrInvalidPlan) || result.Started {
				t.Fatalf("result=%+v error=%v", result, err)
			}
			return result.Coverage
		}},
		{"symlinked target", func(t *testing.T, plan Plan) CoverageObservation {
			if err := os.Symlink(filepath.Join(plan.Coverage.Directory, "missing"), plan.Coverage.ProfilePath); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			result, err := Run(context.Background(), plan)
			if !errors.Is(err, ErrInvalidPlan) || result.Started {
				t.Fatalf("result=%+v error=%v", result, err)
			}
			return result.Coverage
		}},
		{"hard-linked target", func(t *testing.T, plan Plan) CoverageObservation {
			capture := preparedCapture(t, plan)
			outside := filepath.Join(filepath.Dir(plan.Coverage.Directory), "outside.coverprofile")
			if err := os.WriteFile(outside, validProfile(plan), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Link(outside, plan.Coverage.ProfilePath); err != nil {
				t.Skipf("hard links unavailable: %v", err)
			}
			return capture.finish(cleanCoverageResult(), nil)
		}},
		{"replaced file", func(t *testing.T, plan Plan) CoverageObservation {
			capture := preparedCapture(t, plan)
			writeValidProfile(t, plan)
			time.Sleep(20 * time.Millisecond)
			moved := plan.Coverage.ProfilePath + ".old"
			if err := os.Rename(plan.Coverage.ProfilePath, moved); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(plan.Coverage.ProfilePath, validProfile(plan), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(moved); err != nil {
				t.Fatal(err)
			}
			return capture.finish(cleanCoverageResult(), nil)
		}},
		{"malformed profile", func(t *testing.T, plan Plan) CoverageObservation {
			capture := preparedCapture(t, plan)
			if err := os.WriteFile(plan.Coverage.ProfilePath, []byte("mode: set\nnot-a-block\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			return capture.finish(cleanCoverageResult(), nil)
		}},
		{"mixed mode", func(t *testing.T, plan Plan) CoverageObservation {
			capture := preparedCapture(t, plan)
			raw := append(validProfile(plan), []byte("mode: count\n")...)
			if err := os.WriteFile(plan.Coverage.ProfilePath, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			return capture.finish(cleanCoverageResult(), nil)
		}},
		{"invalid count", func(t *testing.T, plan Plan) CoverageObservation {
			capture := preparedCapture(t, plan)
			raw := []byte("mode: set\n" + plan.Coverage.Packages[0].Files[0].ProfilePath + ":3.20,3.30 1 2\n")
			if err := os.WriteFile(plan.Coverage.ProfilePath, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			return capture.finish(cleanCoverageResult(), nil)
		}},
		{"unmapped file", func(t *testing.T, plan Plan) CoverageObservation {
			capture := preparedCapture(t, plan)
			raw := []byte("mode: set\nexample.test/fixture/other.go:3.20,3.30 1 1\n")
			if err := os.WriteFile(plan.Coverage.ProfilePath, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			return capture.finish(cleanCoverageResult(), nil)
		}},
		{"post-source mutation", func(t *testing.T, plan Plan) CoverageObservation {
			capture := preparedCapture(t, plan)
			writeValidProfile(t, plan)
			if err := os.WriteFile(plan.Coverage.Packages[0].Files[0].SourcePath, []byte("package fixture\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			return capture.finish(cleanCoverageResult(), nil)
		}},
		{"oversized profile", func(t *testing.T, plan Plan) CoverageObservation {
			capture := preparedCapture(t, plan)
			file, err := os.Create(plan.Coverage.ProfilePath)
			if err != nil {
				t.Fatal(err)
			}
			if err := file.Truncate(MaxCoverageBytes + 1); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			return capture.finish(cleanCoverageResult(), nil)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := staticCoveragePlan(t)
			coverage := test.run(t, plan)
			if coverage.Completeness != CoverageRejected {
				t.Fatalf("coverage = %+v", coverage)
			}
		})
	}
}

func TestCoverageRejectsEscapingTarget(t *testing.T) {
	plan := staticCoveragePlan(t)
	plan.Coverage.ProfilePath = filepath.Join(filepath.Dir(plan.Coverage.Directory), "escaped.coverprofile")
	result, err := Run(context.Background(), plan)
	if !errors.Is(err, ErrInvalidPlan) || result.Coverage.Completeness != CoverageRejected || result.Started {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestCoverageDirtyTerminalIsNeverComplete(t *testing.T) {
	plan := realCoveragePlan(t, `func TestValue(t *testing.T) { t.Fatal("expected") }`, 30*time.Second)
	result, err := Run(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode == 0 || result.Coverage.Completeness != CoveragePartial {
		t.Fatalf("result = %+v", result)
	}

	plan = realCoveragePlan(t, `func TestValue(t *testing.T) { time.Sleep(30 * time.Second) }`, 200*time.Millisecond)
	testPath := filepath.Join(plan.WorkingDirectory, "fixture_test.go")
	raw, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatal(err)
	}
	raw = bytesReplaceImport(raw)
	if err := os.WriteFile(testPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err = Run(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if !result.TimedOut || result.Coverage.Completeness == CoverageComplete || result.Coverage.Completeness == CoverageAbsent {
		t.Fatalf("timeout result = %+v", result)
	}
}

func TestCoverageTerminalFailuresArePartial(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Result)
	}{
		{"nonzero", func(result *Result) { result.ExitCode = 1 }},
		{"crash", func(result *Result) { result.Exited, result.ExitCode = false, -1 }},
		{"timeout", func(result *Result) { result.TimedOut = true }},
		{"cancellation", func(result *Result) { result.Cancelled = true }},
		{"missing metadata", func(result *Result) { result.Stdout.RawSHA256 = "" }},
		{"truncation", func(result *Result) { result.OutputLimitExceeded = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := staticCoveragePlan(t)
			capture := preparedCapture(t, plan)
			writeValidProfile(t, plan)
			result := cleanCoverageResult()
			test.edit(&result)
			coverage := capture.finish(result, nil)
			if coverage.Completeness != CoveragePartial {
				t.Fatalf("coverage = %+v", coverage)
			}
		})
	}
}

func realCoveragePlan(t *testing.T, testBody string, timeout time.Duration) Plan {
	t.Helper()
	plan := staticCoveragePlan(t)
	repository := plan.WorkingDirectory
	if err := os.WriteFile(filepath.Join(repository, "go.mod"), []byte("module example.test/fixture\n\ngo 1.27.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	testSource := "package fixture\n\nimport \"testing\"\n\n" + testBody + "\n"
	if err := os.WriteFile(filepath.Join(repository, "fixture_test.go"), []byte(testSource), 0o600); err != nil {
		t.Fatal(err)
	}
	goRoot, err := filepath.EvalSymlinks(runtime.GOROOT())
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(filepath.Dir(repository), "cache")
	home := filepath.Join(filepath.Dir(repository), "home")
	moduleCache := filepath.Join(filepath.Dir(repository), "module-cache")
	for _, path := range []string{cache, home, moduleCache} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	plan.GoExecutable = filepath.Join(goRoot, "bin", "go")
	plan.Environment = []EnvironmentVariable{
		{Name: "GOCACHE", Value: cache}, {Name: "GOENV", Value: "off"}, {Name: "GOMODCACHE", Value: moduleCache},
		{Name: "GOPROXY", Value: "off"}, {Name: "GOROOT", Value: goRoot}, {Name: "GOSUMDB", Value: "off"},
		{Name: "GOTOOLCHAIN", Value: "local"}, {Name: "HOME", Value: home},
	}
	plan.RetainOutput = true
	plan.Timeout = timeout
	return plan
}

func staticCoveragePlan(t *testing.T) Plan {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository := filepath.Join(base, "repository")
	directory := filepath.Join(base, "coverage")
	for _, path := range []string{repository, directory} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(repository, "fixture.go")
	sourceBytes := []byte("package fixture\n\nfunc Value() int { return 7 }\n")
	if err := os.WriteFile(source, sourceBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(sourceBytes)
	executable, err := filepath.EvalSymlinks(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	return Plan{
		GoExecutable: executable, WorkingDirectory: repository, Packages: []string{"example.test/fixture"},
		OutputLimitBytes: MaxOutputBytes, Timeout: 30 * time.Second,
		Coverage: &CoverageRequest{
			Directory: directory, ProfilePath: filepath.Join(directory, "unit.coverprofile"), RepositoryRoot: repository,
			Mode: CoverageModeSet, Packages: []CoveragePackage{{ImportPath: "example.test/fixture", Files: []CoverageSourceFile{{
				ProfilePath: "example.test/fixture/fixture.go", SourcePath: source, RawSHA256: hex.EncodeToString(digest[:]),
			}}}},
		},
	}
}

func preparedCapture(t *testing.T, plan Plan) *coverageCapture {
	t.Helper()
	capture, err := prepareCoverage(plan.Coverage)
	if err != nil {
		t.Fatal(err)
	}
	return capture
}

func writeValidProfile(t *testing.T, plan Plan) {
	t.Helper()
	if err := os.WriteFile(plan.Coverage.ProfilePath, validProfile(plan), 0o600); err != nil {
		t.Fatal(err)
	}
}

func validProfile(plan Plan) []byte {
	return []byte("mode: set\n" + plan.Coverage.Packages[0].Files[0].ProfilePath + ":3.20,3.30 1 1\n")
}

func cleanCoverageResult() Result {
	empty := sha256.Sum256(nil)
	return Result{
		Started: true, Exited: true, ExitCode: 0, ProcessCleanupDone: true,
		Containment: ContainmentProcessGroupBestEffort,
		Stdout:      StreamResult{Drained: true, RawSHA256: hex.EncodeToString(empty[:])},
		Stderr:      StreamResult{Drained: true, RawSHA256: hex.EncodeToString(empty[:])},
	}
}

func bytesReplaceImport(raw []byte) []byte {
	return []byte(strings.Replace(string(raw), "import \"testing\"", "import (\n\t\"testing\"\n\t\"time\"\n)", 1))
}
