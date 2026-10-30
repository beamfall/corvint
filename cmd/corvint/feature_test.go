package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func runFeatureProcess(t *testing.T, root string, featureArguments ...string) processResult {
	t.Helper()
	arguments := append([]string{"--root", root, "feature"}, featureArguments...)
	candidate := exec.Command(os.Args[0], append([]string{"-test.run=^TestCandidateHelperProcess$", "--"}, arguments...)...)
	candidate.Env = append(os.Environ(), "CORVINT_HELPER_PROCESS=1")
	return execute(t, candidate)
}

func TestFreshProcessFeatureMatchesPythonKnownUnknownLimitsAndBudgets(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	for _, test := range []struct {
		name string
		args []string
	}{
		{"known default", []string{"a-first"}},
		{"unknown default", []string{"no-such-feature-id"}},
		{"known limit one", []string{"--limit", "1", "a-first"}},
		{"known limit fifty", []string{"a-first", "--limit", "50"}},
		{"known minimum budget", []string{"a-first", "--budget-bytes", "1216"}},
		{"known selective budget", []string{"a-first", "--budget-bytes", "1500"}},
		{"known maximum budget", []string{"a-first", "--budget-bytes", "1000000"}},
		{"unknown minimum budget", []string{"no-such-feature-id", "--budget-bytes", "1216"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := runFeatureProcess(t, root, test.args...)
			if candidate.exit != 0 || len(candidate.stderr) != 0 {
				t.Fatalf("candidate exit=%d stderr=%q", candidate.exit, candidate.stderr)
			}
		})
	}
}

func TestFreshProcessFeatureMatchesPythonArgumentFailures(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	for _, test := range []struct {
		name string
		args []string
	}{
		{"missing id", nil},
		{"extra id", []string{"a-first", "extra"}},
		{"malformed id", []string{"Bad_ID"}},
		{"limit precedes id", []string{"Bad_ID", "--limit", "0"}},
		{"noninteger limit", []string{"a-first", "--limit", "nope"}},
		{"minimum limit", []string{"a-first", "--limit", "0"}},
		{"maximum limit", []string{"a-first", "--limit", "51"}},
		{"noninteger budget", []string{"a-first", "--budget-bytes", "nope"}},
		{"low budget", []string{"a-first", "--budget-bytes", "1023"}},
		{"high budget", []string{"a-first", "--budget-bytes", "1000001"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := runFeatureProcess(t, root, test.args...)
			if candidate.exit != 2 || len(candidate.stdout) != 0 || len(candidate.stderr) == 0 {
				t.Fatalf("candidate exit=%d stdout=%q stderr=%q", candidate.exit, candidate.stdout, candidate.stderr)
			}
		})
	}
}

func TestFeatureKnownAndUnknownAreReadOnlyAndDoNotReadStdin(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	before := repositoryBytesDigest(t, root)
	for _, featureID := range []string{"a-first", "no-such-feature-id"} {
		reader := &forbiddenImpactReader{}
		var stdout, stderr bytes.Buffer
		exit := run([]string{"--root", root, "feature", featureID}, reader, &stdout, &stderr)
		if exit != 0 || stderr.Len() != 0 || stdout.Len() == 0 {
			t.Fatalf("id=%s exit=%d stdout=%q stderr=%q", featureID, exit, &stdout, &stderr)
		}
		if reader.reads != 0 {
			t.Fatalf("id=%s stdin reads=%d", featureID, reader.reads)
		}
	}
	if after := repositoryBytesDigest(t, root); before != after {
		t.Fatal("feature changed repository bytes")
	}
}

func TestFreshProcessUnknownFeatureMatchesPythonInMixedWorktree(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	if err := os.WriteFile(filepath.Join(root, "pkg", "main.go"), []byte("package main\n\nfunc StableValue() string { return \"mixed\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidate := runFeatureProcess(t, root, "no-such-feature-id", "--budget-bytes", "1216")
	if !bytes.Contains(candidate.stdout, []byte(`"state":"OUT_OF_SCOPE"`)) || !bytes.Contains(candidate.stdout, []byte(`"state":"mixed-worktree"`)) {
		t.Fatalf("stdout=%s", candidate.stdout)
	}
}

func TestFeatureOutputIsIndependentOfRepositoryLocation(t *testing.T) {
	t.Parallel()
	firstRoot := impactCLIRepository(t)
	secondRoot := filepath.Join(t.TempDir(), "different", "root")
	if err := os.MkdirAll(filepath.Dir(secondRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	clone := exec.Command("git", "clone", "-q", firstRoot, secondRoot)
	if output, err := clone.CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, output)
	}
	first := runFeatureProcess(t, firstRoot, "a-first")
	second := runFeatureProcess(t, secondRoot, "a-first")
	if first.exit != second.exit || !bytes.Equal(first.stdout, second.stdout) || !bytes.Equal(first.stderr, second.stderr) {
		t.Fatalf("location-dependent output\nfirst=%q\nsecond=%q", first.stdout, second.stdout)
	}
	if bytes.Contains(first.stdout, []byte(firstRoot)) || bytes.Contains(second.stdout, []byte(secondRoot)) {
		t.Fatal("feature stdout contains an absolute repository root")
	}
}

func TestFeatureRefusesKnownNonGoCandidateRanking(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	pythonSource := filepath.Join(root, "pkg", "candidate.py")
	if err := os.WriteFile(pythonSource, []byte("def StableValue():\n    return 'python'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"add", "pkg/candidate.py"}, {"commit", "-qm", "add Python candidate"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	candidate := runFeatureProcess(t, root, "a-first")
	if candidate.exit != 2 || len(candidate.stdout) != 0 ||
		!bytes.Contains(candidate.stderr, []byte(`"code": "unsupported-feature-repository"`)) {
		t.Fatalf("candidate exit=%d stdout=%q stderr=%q", candidate.exit, candidate.stdout, candidate.stderr)
	}
	limitOne := runFeatureProcess(t, root, "a-first", "--limit", "1")
	if limitOne.exit != 0 || len(limitOne.stderr) != 0 || len(limitOne.stdout) == 0 {
		t.Fatalf("limit-one result=%#v", limitOne)
	}
	unknown := runFeatureProcess(t, root, "no-such-feature-id")
	if unknown.exit != 0 || len(unknown.stderr) != 0 || !bytes.Contains(unknown.stdout, []byte(`"state":"OUT_OF_SCOPE"`)) {
		t.Fatalf("unknown result=%#v", unknown)
	}
}

func TestFeaturePlatformBoundaryIsTyped(t *testing.T) {
	t.Parallel()
	_, err := parseFeatureArgumentsForPlatform(options{featureLimit: 10}, []string{"a-first"}, "windows")
	var stderr bytes.Buffer
	emitError(&stderr, err)
	if stderr.String() != "{\"code\": \"unsupported-feature-platform\", \"error\": \"native Go feature is qualified only on Darwin and Linux\", \"evidence\": [{\"name\": \"platform\", \"value\": \"windows\"}], \"ok\": false, \"subject\": {\"kind\": \"host-capability\", \"value\": \"native-platform\"}, \"supported_fixes\": [], \"terminal\": \"unsupported-platform\"}\n" {
		t.Fatalf("stderr=%q", &stderr)
	}
}
