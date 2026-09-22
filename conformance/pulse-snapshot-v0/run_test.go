package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
	"time"
)

var frozenCaseDigests = []string{
	"edb56e1d350490eede00c713fbf3602847870f13f8ef5fe459e798275afbc2e0",
	"87837aa95af7899f86212b6314c2b2b3aa14175b0edcbeb7587ad9c928a26560",
	"c366e231fbe4f3bb69ac633a62ba18728bb0f03176811275b439c6f5f03f9032",
	"3df4f7719ce579f353901da02d805070cd4b5955385382fa8ffef34eaaa88d80",
	"90ce1a355c75fd4dcf4de2e04460c150b30f0b91b51e96986dd390188ffe6e6b",
	"1679aabbc375a6788e70090b6be6ede208ff8aa2e1fb3d48a263933a4478567a",
	"a45ebb47d707f3f0ee1c046021213d05f4296be6c690e77f00c27d71be464512",
	"149a04938d8fa48e72319d7ed1b7581e00e040c68c7033fc0a5c09b63256f10c",
	"8a35809a23f0c72f9e57349fec2200b588c9b2b85a45d4c07ed23d9d519d35a8",
	"9949bb3c29641ebf16df85cb1839dc0f06aaba63bfc58f59f6eee27196a8df11",
	"da671490700a79c8f5d0f61f0247ecf0bf9677f704f0e4ab3838190a98bce8c1",
	"b06c8c5618e5760a7dec261c83c2e13d6c9d5d047c7f2db2fddcc07eeb878ffe",
}

func sourceDirectory(t testing.TB) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate conformance source")
	}
	return filepath.Dir(filename)
}

func moduleRoot(t testing.TB) string {
	t.Helper()
	return filepath.Clean(filepath.Join(sourceDirectory(t), "..", ".."))
}

func corpusBytes(t testing.TB) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(sourceDirectory(t), "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func validCorpus(t testing.TB) corpusDocument {
	t.Helper()
	document, err := loadCorpus(corpusBytes(t), true)
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func TestFrozenCorpusValidationAndDigests(t *testing.T) {
	raw := corpusBytes(t)
	if len(raw) != 15_116 {
		t.Fatalf("corpus bytes = %d", len(raw))
	}
	fileDigest := sha256.Sum256(raw)
	if got := hex.EncodeToString(fileDigest[:]); got != "2c333d4d8bf6789814bdc436860da46599d7015c751df45a7739117050856354" {
		t.Fatalf("corpus SHA-256 = %s", got)
	}
	document := validCorpus(t)
	if document.caseSetSHA256 != "a364aee5d5fc0a0f3868ee9479b62e573bfa79e9e924c6f7870ed4ed482fd406" {
		t.Fatalf("case-set digest = %s", document.caseSetSHA256)
	}
	if len(document.cases) != len(frozenCaseDigests) {
		t.Fatalf("cases = %d", len(document.cases))
	}
	for index, test := range document.cases {
		if test.id != frozenCaseIDs[index] || test.caseSHA256 != frozenCaseDigests[index] {
			t.Fatalf("case %d = (%s, %s)", index, test.id, test.caseSHA256)
		}
	}
	if err := embeddedSelfTests(document); err != nil {
		t.Fatal(err)
	}
}

func TestCorpusValidatorRejectsHostileAndNoncanonicalInput(t *testing.T) {
	raw := corpusBytes(t)
	duplicateKey := bytes.Replace(raw, []byte(`{"caseSetSha256":`), []byte(`{"stage":"P0-A","caseSetSha256":`), 1)
	noncanonical := append(append([]byte(nil), bytes.TrimSuffix(raw, []byte{'\n'})...), ' ', '\n')
	cases := []struct {
		name      string
		raw       []byte
		expected  string
		canonical bool
	}{
		{"empty", nil, "corpus:invalid-size", true},
		{"oversize", bytes.Repeat([]byte{'x'}, maxCorpusBytes+1), "corpus:invalid-size", true},
		{"invalid-utf8", []byte{'{', 0xff, '}'}, "corpus:invalid-json", true},
		{"duplicate-key", duplicateKey, "corpus:duplicate-key:stage", true},
		{"noncanonical", noncanonical, "corpus:not-canonical-json-plus-lf", true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := loadCorpus(test.raw, test.canonical)
			if err == nil || !strings.Contains(err.Error(), test.expected) {
				t.Fatalf("error = %v, want %q", err, test.expected)
			}
		})
	}
}

func TestCorpusValidatorRejectsRewrittenAndRehashedCorpus(t *testing.T) {
	parsed, err := parseJSON(corpusBytes(t), "mutation")
	if err != nil {
		t.Fatal(err)
	}
	document := parsed.(map[string]any)
	cases := document["cases"].([]any)
	changed := cases[0].(map[string]any)
	expectedLines := changed["expectedLines"].([]any)
	expectedLines[1] = strings.Replace(expectedLines[1].(string), "already initialized", "previously initialized", 1)
	basis := make(map[string]any, len(changed)-1)
	for key, value := range changed {
		if key != "caseSha256" {
			basis[key] = value
		}
	}
	basisRaw, err := canonical(basis, false)
	if err != nil {
		t.Fatal(err)
	}
	changed["caseSha256"] = digest("case", basisRaw)
	caseDigests := make([]any, len(cases))
	for index, rawCase := range cases {
		caseDigests[index] = rawCase.(map[string]any)["caseSha256"]
	}
	setBasis, err := canonical(caseDigests, false)
	if err != nil {
		t.Fatal(err)
	}
	document["caseSetSha256"] = digest("case-set", setBasis)
	mutated, err := canonical(document, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadCorpus(mutated, true); err == nil || err.Error() != "corpus:frozen-digest-mismatch" {
		t.Fatalf("error = %v", err)
	}
}

func TestCanonicalJSONMatchesPythonEncodingEdges(t *testing.T) {
	value, err := parseJSON([]byte(`{"amp":"<&>","line":"\u2028\u2029","small":1e-5,"whole":1.0}`), "edge")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := canonical(value, true)
	if err != nil {
		t.Fatal(err)
	}
	expected := "{\"amp\":\"<&>\",\"line\":\"\u2028\u2029\",\"small\":1e-05,\"whole\":1.0}\n"
	if string(raw) != expected {
		t.Fatalf("canonical = %q", raw)
	}
}

type processObservation struct {
	exit   int
	stdout []byte
	stderr []byte
}

func observe(command *exec.Cmd) processObservation {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	exitCode := 0
	if err != nil {
		var exitError *exec.ExitError
		if !errorsAs(err, &exitError) {
			return processObservation{-999, stdout.Bytes(), append(stderr.Bytes(), err.Error()...)}
		}
		exitCode = exitError.ExitCode()
	}
	return processObservation{exitCode, append([]byte(nil), stdout.Bytes()...), append([]byte(nil), stderr.Bytes()...)}
}

// errorsAs is a narrow wrapper so helper-process tests do not share candidate code.
func errorsAs(err error, target any) bool {
	value, ok := target.(**exec.ExitError)
	if !ok {
		return false
	}
	exitError, ok := err.(*exec.ExitError)
	if ok {
		*value = exitError
	}
	return ok
}

func TestGoSelfCheckFrozenBytes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := runCLI(nil, &stdout, &stderr)
	expected := "{\"cases\":12,\"mode\":\"self-check\",\"profile\":\"corvint-pulse-snapshot-conformance/0\",\"stage\":\"P0-A\",\"status\":\"PASS\"}\n"
	if exit != 0 || stdout.String() != expected || stderr.Len() != 0 {
		t.Fatalf("result = (%d, %q, %q)", exit, &stdout, &stderr)
	}
}

func helperCommand(mode string, arguments ...string) []string {
	command := []string{os.Args[0], "-test.run=^TestAuthorityHelperProcess$", "authority-helper", mode}
	return append(command, arguments...)
}

func buildBinary(t testing.TB, name, packagePath string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), name)
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-trimpath", "-o", binary, packagePath)
	build.Dir = moduleRoot(t)
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v: %s", packagePath, err, output)
	}
	return binary
}

func TestBuiltRunnerIsRelocatable(t *testing.T) {
	runner := buildBinary(t, "pulse-conformance", "./conformance/pulse-snapshot-v0")
	command := exec.Command(runner)
	command.Dir = t.TempDir()
	result := observe(command)
	expected := "{\"cases\":12,\"mode\":\"self-check\",\"profile\":\"corvint-pulse-snapshot-conformance/0\",\"stage\":\"P0-A\",\"status\":\"PASS\"}\n"
	if result.exit != 0 || string(result.stdout) != expected || len(result.stderr) != 0 {
		t.Fatalf("result = (%d, %q, %q)", result.exit, result.stdout, result.stderr)
	}
}

func TestGoExecutableFrozenBytes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sentinel"), []byte("unchanged\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	command := helperCommand("good", "--root", root)
	var stdout, stderr bytes.Buffer
	exit := runCLI(append([]string{"--command"}, command...), &stdout, &stderr)
	expected := "{\"cases\":12,\"mode\":\"executable\",\"profile\":\"corvint-pulse-snapshot-conformance/0\",\"stage\":\"P0-A\",\"status\":\"PASS\"}\n"
	if exit != 0 || stdout.String() != expected || stderr.Len() != 0 {
		t.Fatalf("result = (%d, %q, %q)", exit, &stdout, &stderr)
	}
}

func TestSuppliedCorvintPulseRunsAllCasesWithoutRepositoryMutation(t *testing.T) {
	binary := buildBinary(t, "corvint-pulse", "./cmd/corvint-pulse")
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o750); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		".git/config": "[core]\n\trepositoryformatversion = 0\n",
		"source.go":   "package fixture\n",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(content), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	before, err := snapshotRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runCLI([]string{"--command", binary, "--root", root, "serve", "--stdio"}, &stdout, &stderr)
	if exitCode != 0 || stderr.Len() != 0 || !bytes.Contains(stdout.Bytes(), []byte(`"mode":"executable"`)) {
		t.Fatalf("runner = (%d, %q, %q)", exitCode, &stdout, &stderr)
	}
	after, err := snapshotRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("repository changed: %s != %s", before, after)
	}
}

func TestSeededTranscriptFaultsAreRejected(t *testing.T) {
	document := validCorpus(t)
	tests := []struct {
		name     string
		mode     string
		expected string
		timeout  time.Duration
	}{
		{"stdout", "bad-stdout", "duplicate-initialize:stdout-byte-mismatch", 3 * time.Second},
		{"stderr", "bad-stderr", "duplicate-initialize:unexpected-stderr", 3 * time.Second},
		{"exit", "bad-exit", "duplicate-initialize:exit-code:7!=0", 3 * time.Second},
		{"reused-session", "reused-session", "command:session-id-reused-across-processes", 3 * time.Second},
		{"timeout", "timeout", "duplicate-initialize:cannot-execute:TimeoutExpired", 100 * time.Millisecond},
		{"newline-amplification", "newline-amplification", "duplicate-initialize:invalid-output-framing", 3 * time.Second},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			prior := caseTimeout
			caseTimeout = test.timeout
			defer func() { caseTimeout = prior }()
			err := executeCases(document, helperCommand(test.mode))
			if err == nil || err.Error() != test.expected {
				t.Fatalf("error = %v, want %q", err, test.expected)
			}
		})
	}
}

func TestRepositoryMutationIsDetected(t *testing.T) {
	document := validCorpus(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sentinel"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := executeCases(document, helperCommand("mutate", "--root", root))
	if err == nil || err.Error() != repositoryMutation {
		t.Fatalf("error = %v", err)
	}
}

func TestResolvedRootIsBoundIntoExecutedArgumentVector(t *testing.T) {
	document := validCorpus(t)
	parent := t.TempDir()
	rootA := filepath.Join(parent, "root-a")
	rootB := filepath.Join(parent, "root-b")
	link := filepath.Join(parent, "current")
	for _, root := range []string{rootA, rootB} {
		if err := os.Mkdir(root, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(rootA, link); err != nil {
		t.Fatal(err)
	}
	err := executeCases(document, helperCommand("root-swap", link, rootB, "--root", link))
	if err == nil || err.Error() != repositoryMutation {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(rootA, "mutation")); err != nil {
		t.Fatalf("resolved root was not executed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(rootB, "mutation")); !os.IsNotExist(err) {
		t.Fatalf("swapped symlink root was executed: %v", err)
	}
}

func TestSnapshotRejectsOutsideSymlinkSwapWithoutReadingTarget(t *testing.T) {
	root := t.TempDir()
	victim := filepath.Join(root, "victim")
	outside := filepath.Join(t.TempDir(), "outside-secret")
	if err := os.WriteFile(victim, []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("must-not-be-read"), 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(outside, 0o600)
	changed := false
	_, err := snapshotRepositoryWithHook(root, func(path, phase string) {
		if changed || path != "victim" || phase != "after-lstat" {
			return
		}
		changed = true
		if removeErr := os.Remove(victim); removeErr != nil {
			t.Errorf("remove victim: %v", removeErr)
			return
		}
		if linkErr := os.Symlink(outside, victim); linkErr != nil {
			t.Errorf("swap victim: %v", linkErr)
		}
	})
	if !changed {
		t.Fatal("swap hook did not run")
	}
	if err == nil || err.Error() != repositoryMutation {
		t.Fatalf("error = %v", err)
	}
}

func makeFIFO(t *testing.T, path string) {
	t.Helper()
	mkfifo, err := exec.LookPath("mkfifo")
	if err != nil {
		t.Skip("mkfifo is unavailable")
	}
	if output, err := exec.Command(mkfifo, path).CombinedOutput(); err != nil {
		t.Fatalf("mkfifo: %v: %s", err, output)
	}
}

func TestSnapshotRejectsFIFOSwapWithoutBlocking(t *testing.T) {
	mkfifo, err := exec.LookPath("mkfifo")
	if err != nil {
		t.Skip("mkfifo is unavailable")
	}
	root := t.TempDir()
	victim := filepath.Join(root, "victim")
	if err := os.WriteFile(victim, []byte("regular"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		changed := false
		_, err := snapshotRepositoryWithHook(root, func(path, phase string) {
			if changed || path != "victim" || phase != "after-lstat" {
				return
			}
			changed = true
			if removeErr := os.Remove(victim); removeErr != nil {
				result <- removeErr
				return
			}
			if output, commandErr := exec.Command(mkfifo, victim).CombinedOutput(); commandErr != nil {
				result <- fmt.Errorf("mkfifo: %w: %s", commandErr, output)
			}
		})
		result <- err
	}()
	select {
	case err := <-result:
		if err == nil || err.Error() != repositoryMutation {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(2 * time.Second):
		// Pair with a blocking reader if the implementation regresses to a
		// path-based FIFO open, then fail without leaving a test goroutine.
		writer, _ := os.OpenFile(victim, os.O_WRONLY, 0)
		if writer != nil {
			_ = writer.Close()
		}
		t.Fatal("snapshot blocked opening a swapped FIFO")
	}
}

func TestSnapshotRejectsExistingFIFOWithoutBlocking(t *testing.T) {
	root := t.TempDir()
	makeFIFO(t, filepath.Join(root, "pipe"))
	started := time.Now()
	_, err := snapshotRepository(root)
	if err == nil || err.Error() != repositoryUnsafe {
		t.Fatalf("error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("FIFO rejection took %s", elapsed)
	}
}

func TestLinkedWorktreeGitStateMutationIsDetected(t *testing.T) {
	document := validCorpus(t)
	root := t.TempDir()
	gitDirectory := t.TempDir()
	commonDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: "+gitDirectory+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDirectory, "commondir"), []byte(commonDirectory+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(gitDirectory, "index")
	if err := os.WriteFile(indexPath, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := executeCases(document, helperCommand("mutate-external", indexPath, "--root", root))
	if err == nil || err.Error() != repositoryMutation {
		t.Fatalf("error = %v", err)
	}
}

func TestProcessGroupCleanupReapsDescendant(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows runner has leader-only fallback until native job-object qualification")
	}
	document := validCorpus(t)
	temporary := t.TempDir()
	ready := filepath.Join(temporary, "ready")
	survived := filepath.Join(temporary, "survived")
	started := time.Now()
	_, err := executeCase(document.cases[0], helperCommand("descendant", ready, survived))
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("descendant cleanup took %s", elapsed)
	}
	if _, err := os.Stat(ready); err != nil {
		t.Fatalf("descendant did not start: %v", err)
	}
	time.Sleep(2200 * time.Millisecond)
	if _, err := os.Stat(survived); !os.IsNotExist(err) {
		t.Fatalf("descendant survived process-group cleanup: %v", err)
	}
}

func TestTimeoutReapsDescendant(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows runner has leader-only fallback until native job-object qualification")
	}
	document := validCorpus(t)
	temporary := t.TempDir()
	ready := filepath.Join(temporary, "ready")
	survived := filepath.Join(temporary, "survived")
	prior := caseTimeout
	caseTimeout = 100 * time.Millisecond
	defer func() { caseTimeout = prior }()
	_, err := executeCase(document.cases[0], helperCommand("timeout-descendant", ready, survived))
	if err == nil || err.Error() != "duplicate-initialize:cannot-execute:TimeoutExpired" {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(ready); err != nil {
		t.Fatalf("descendant did not start: %v", err)
	}
	time.Sleep(2200 * time.Millisecond)
	if _, err := os.Stat(survived); !os.IsNotExist(err) {
		t.Fatalf("descendant survived timeout cleanup: %v", err)
	}
}

func TestRunnerGoroutinesTerminate(t *testing.T) {
	document := validCorpus(t)
	if _, err := executeCase(document.cases[0], helperCommand("good")); err != nil {
		t.Fatal(err)
	}
	profile := pprof.Lookup("goroutineleak")
	if profile == nil {
		t.Fatal("Go 1.27 goroutineleak profile is unavailable")
	}
	var report bytes.Buffer
	if err := profile.WriteTo(&report, 2); err != nil {
		t.Fatal(err)
	}
	if count := profile.Count(); count != 0 {
		t.Fatalf("runner leaked %d goroutines:\n%s", count, &report)
	}
}

func TestInterruptReapsDescendant(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX signal regression")
	}
	runner := buildBinary(t, "pulse-conformance", "./conformance/pulse-snapshot-v0")
	temporary := t.TempDir()
	ready := filepath.Join(temporary, "ready")
	survived := filepath.Join(temporary, "survived")
	helper := helperCommand("interrupt-descendant", ready, survived)
	command := exec.Command(runner, append([]string{"--command"}, helper...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			_ = command.Process.Kill()
			t.Fatal("descendant did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := command.Process.Signal(os.Interrupt); err != nil {
		_ = command.Process.Kill()
		t.Fatal(err)
	}
	waited := make(chan error, 1)
	go func() { waited <- command.Wait() }()
	select {
	case err := <-waited:
		if exitError, ok := err.(*exec.ExitError); !ok || exitError.ExitCode() != 1 {
			t.Fatalf("runner exit = %v; stdout=%q stderr=%q", err, &stdout, &stderr)
		}
	case <-time.After(3 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("runner did not stop after interrupt")
	}
	time.Sleep(2200 * time.Millisecond)
	if _, err := os.Stat(survived); !os.IsNotExist(err) {
		t.Fatalf("descendant survived interrupt cleanup: %v", err)
	}
}

func TestEmptyCommandFailureBytes(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runCLI([]string{"--command"}, &stdout, &stderr)
	expected := "{\"error\":\"command:explicit-executable-required\",\"profile\":\"corvint-pulse-snapshot-conformance/0\",\"stage\":\"P0-A\",\"status\":\"FAIL\"}\n"
	if exitCode != 1 || stdout.Len() != 0 || stderr.String() != expected {
		t.Fatalf("result = (%d, %q, %q)", exitCode, &stdout, &stderr)
	}
}

func helperArguments() []string {
	for index, argument := range os.Args {
		if (argument == "authority-helper" || argument == "authority-descendant") && index < len(os.Args) {
			return os.Args[index:]
		}
	}
	return nil
}

func helperExit(code int) {
	os.Exit(code)
}

func TestAuthorityHelperProcess(t *testing.T) {
	arguments := helperArguments()
	if len(arguments) < 2 || arguments[0] != "authority-helper" {
		return
	}
	mode := arguments[1]
	request, err := io.ReadAll(os.Stdin)
	if err != nil {
		helperExit(90)
	}
	document, err := loadCorpus(corpusBytes(t), true)
	if err != nil {
		helperExit(91)
	}
	var selected *corpusCase
	for index := range document.cases {
		candidate := &document.cases[index]
		if string(request) == strings.Join(candidate.requestLines, "\n")+"\n" {
			selected = candidate
			break
		}
	}
	if selected == nil {
		helperExit(92)
	}
	if mode == "timeout" {
		time.Sleep(10 * time.Second)
		helperExit(93)
	}
	if mode == "newline-amplification" {
		_, _ = os.Stdout.Write(bytes.Repeat([]byte{'\n'}, 1000))
		helperExit(0)
	}
	session := fmt.Sprintf("%016x%016x", os.Getpid(), time.Now().UnixNano())
	if mode == "reused-session" {
		session = strings.Repeat("S", 16)
	}
	if mode == "mutate" {
		root := commandRoot(arguments)
		if root == "" || os.WriteFile(filepath.Join(root, "mutation"), []byte("changed"), 0o600) != nil {
			helperExit(94)
		}
	}
	if mode == "mutate-external" {
		if len(arguments) < 3 || os.WriteFile(arguments[2], []byte("changed"), 0o600) != nil {
			helperExit(95)
		}
	}
	if mode == "root-swap" {
		if len(arguments) < 6 {
			helperExit(96)
		}
		if os.Remove(arguments[2]) != nil || os.Symlink(arguments[3], arguments[2]) != nil {
			helperExit(97)
		}
		root := commandRoot(arguments)
		if root == "" || os.WriteFile(filepath.Join(root, "mutation"), []byte("changed"), 0o600) != nil {
			helperExit(98)
		}
	}
	if mode == "descendant" || mode == "timeout-descendant" || mode == "interrupt-descendant" {
		if len(arguments) < 4 {
			helperExit(99)
		}
		child := exec.Command(os.Args[0], "-test.run=^TestAuthorityDescendantProcess$", "authority-descendant", arguments[2], arguments[3])
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			helperExit(100)
		}
		deadline := time.Now().Add(time.Second)
		for {
			if _, err := os.Stat(arguments[2]); err == nil {
				break
			}
			if time.Now().After(deadline) {
				helperExit(101)
			}
			time.Sleep(5 * time.Millisecond)
		}
		if mode != "descendant" {
			time.Sleep(10 * time.Second)
			helperExit(102)
		}
	}
	output, err := expectedStdout(*selected, session)
	if err != nil {
		helperExit(103)
	}
	if mode == "bad-stdout" {
		output = bytes.Replace(output, []byte("session-already-initialized"), []byte("session-already-initialised"), 1)
	}
	if _, err := os.Stdout.Write(output); err != nil {
		helperExit(104)
	}
	if mode == "bad-stderr" {
		_, _ = os.Stderr.WriteString("seeded stderr\n")
	}
	if mode == "bad-exit" {
		helperExit(7)
	}
	helperExit(selected.expectedExit)
}

func TestAuthorityDescendantProcess(t *testing.T) {
	arguments := helperArguments()
	if len(arguments) != 3 || arguments[0] != "authority-descendant" {
		return
	}
	if err := os.WriteFile(arguments[1], []byte("ready"), 0o600); err != nil {
		helperExit(80)
	}
	time.Sleep(2 * time.Second)
	if err := os.WriteFile(arguments[2], []byte("survived"), 0o600); err != nil {
		helperExit(81)
	}
	helperExit(0)
}
