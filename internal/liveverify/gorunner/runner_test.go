package gorunner

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestValidatePlanBuildsOnlyFixedArgv(t *testing.T) {
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	plan := Plan{
		GoExecutable:     executable,
		WorkingDirectory: root,
		Environment: []EnvironmentVariable{
			{Name: "A", Value: "one"},
			{Name: "B_2", Value: "two"},
		},
		Packages:         []string{"example.com/a", "example.com/b"},
		OutputLimitBytes: 4_096,
		Timeout:          time.Second,
	}
	argv, environment, err := validatePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if expected := []string{"test", "-json", "-count=1", "-vet=off", "example.com/a", "example.com/b"}; !reflect.DeepEqual(argv, expected) {
		t.Fatalf("argv = %#v, want %#v", argv, expected)
	}
	if expected := []string{"A=one", "B_2=two"}; !reflect.DeepEqual(environment, expected) {
		t.Fatalf("environment = %#v, want %#v", environment, expected)
	}
}

func TestValidatePlanAddsCoverageOnlyWhenRequested(t *testing.T) {
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
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
	source := filepath.Join(repository, "a.go")
	if err := os.WriteFile(source, []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan := Plan{
		GoExecutable: executable, WorkingDirectory: repository,
		Packages: []string{"example.com/a"}, OutputLimitBytes: 1, Timeout: time.Second,
		Coverage: &CoverageRequest{
			Directory: directory, ProfilePath: filepath.Join(directory, "unit.coverprofile"), RepositoryRoot: repository,
			Mode: CoverageModeSet, Packages: []CoveragePackage{{ImportPath: "example.com/a", Files: []CoverageSourceFile{{
				ProfilePath: "example.com/a/a.go", SourcePath: source, RawSHA256: digestString("package a\n"),
			}}}},
		},
	}
	argv, _, err := validatePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"test", "-json", "-count=1", "-vet=off", "-covermode=set", "-coverprofile=" + plan.Coverage.ProfilePath, "example.com/a"}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv = %#v, want %#v", argv, want)
	}

	escaping := *plan.Coverage
	escaping.ProfilePath = filepath.Join(base, "escaped.coverprofile")
	plan.Coverage = &escaping
	if _, _, err := validatePlan(plan); !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("escaping coverage target error = %v", err)
	}

	oversized := escaping
	oversized.ProfilePath = filepath.Join(directory, strings.Repeat("x", MaxArgvBytes))
	plan.Coverage = &oversized
	if _, _, err := validatePlan(plan); !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("coverage argv byte-bound error = %v", err)
	}
}

func TestValidatePlanEnforcesArgvElementBound(t *testing.T) {
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	packages := make([]string, MaxPackages)
	for index := range packages {
		packages[index] = "example.com/p" + strconv.Itoa(index+1_000_000)
	}
	plan := Plan{
		GoExecutable:     executable,
		WorkingDirectory: t.TempDir(),
		Packages:         packages,
		OutputLimitBytes: 1,
		Timeout:          time.Second,
	}
	argv, _, err := validatePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(argv) + 1; got != 4_096 {
		t.Fatalf("argv elements including executable = %d", got)
	}
	plan.Packages = append(plan.Packages, "example.com/z")
	if _, _, err := validatePlan(plan); !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("oversize argv error = %v", err)
	}
}

func TestValidatePlanRejectsUnadmittedPackages(t *testing.T) {
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	base := Plan{
		GoExecutable:     executable,
		WorkingDirectory: t.TempDir(),
		OutputLimitBytes: 1,
		Timeout:          time.Second,
	}
	tests := [][]string{
		nil,
		{"./...", "example.com/a"},
		{"example.com/b", "example.com/a"},
		{"example.com/a", "example.com/a"},
		{"-run=TestX"},
		{"example.com/a@v1.2.3"},
		{"example.com/a,b"},
		{"example.com/../b"},
		{"example.com/..."},
		{"example.com/*"},
		{"example.com/a\\b"},
		{"example.com/a b"},
	}
	for _, packages := range tests {
		plan := base
		plan.Packages = packages
		if _, _, err := validatePlan(plan); !errors.Is(err, ErrInvalidPlan) {
			t.Errorf("packages %#v: error = %v", packages, err)
		}
	}
}

func TestValidatePlanRejectsFinalComponentSymlinks(t *testing.T) {
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	realDirectory := t.TempDir()
	linkedExecutable := filepath.Join(t.TempDir(), "go-link")
	if err := os.Symlink(executable, linkedExecutable); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	base := Plan{
		GoExecutable:     executable,
		WorkingDirectory: realDirectory,
		Packages:         []string{"./..."},
		OutputLimitBytes: 1,
		Timeout:          time.Second,
	}
	linkedPlan := base
	linkedPlan.GoExecutable = linkedExecutable
	if _, _, err := validatePlan(linkedPlan); !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("executable symlink error = %v", err)
	}
	linkedDirectory := filepath.Join(t.TempDir(), "cwd-link")
	if err := os.Symlink(realDirectory, linkedDirectory); err != nil {
		t.Fatal(err)
	}
	linkedPlan = base
	linkedPlan.WorkingDirectory = linkedDirectory
	if _, _, err := validatePlan(linkedPlan); !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("cwd symlink error = %v", err)
	}
}

func TestValidatePlanRejectsImplicitOrAmbiguousEnvironment(t *testing.T) {
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	base := Plan{
		GoExecutable:     executable,
		WorkingDirectory: t.TempDir(),
		Packages:         []string{"./..."},
		OutputLimitBytes: 1,
		Timeout:          time.Second,
	}
	tests := [][]EnvironmentVariable{
		{{Name: "B", Value: "one"}, {Name: "A", Value: "two"}},
		{{Name: "A", Value: "one"}, {Name: "A", Value: "two"}},
		{{Name: "lower", Value: "one"}},
		{{Name: "A=B", Value: "one"}},
		{{Name: "A", Value: "bad\x00value"}},
	}
	for _, environment := range tests {
		plan := base
		plan.Environment = environment
		if _, _, err := validatePlan(plan); !errors.Is(err, ErrInvalidPlan) {
			t.Errorf("environment %#v: error = %v", environment, err)
		}
	}
}
