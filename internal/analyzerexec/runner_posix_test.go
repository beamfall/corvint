//go:build darwin || linux

package analyzerexec

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

var (
	helperOnce sync.Once
	helperPath string
	helperErr  error
)

func nativeHelper(t *testing.T) string {
	t.Helper()
	helperOnce.Do(func() {
		directory, err := os.MkdirTemp("", "analyzerexec-helper-")
		if err != nil {
			helperErr = err
			return
		}
		source := filepath.Join(directory, "main.go")
		program := `package main
import ("bytes"; "io"; "os"; "strconv"; "strings"; "time")
func main() {
	data, _ := io.ReadAll(os.Stdin)
	switch value := string(data); {
	case value == "hang": time.Sleep(time.Hour)
	case strings.HasPrefix(value, "stdout:"):
		n, _ := strconv.Atoi(strings.TrimPrefix(value, "stdout:")); os.Stdout.Write(bytes.Repeat([]byte("o"), n))
	case strings.HasPrefix(value, "stderr:"):
		n, _ := strconv.Atoi(strings.TrimPrefix(value, "stderr:")); os.Stderr.Write(bytes.Repeat([]byte("e"), n))
	default: os.Stdout.Write(data)
	}
}
`
		if err := os.WriteFile(source, []byte(program), 0o600); err != nil {
			helperErr = err
			return
		}
		helperPath = filepath.Join(directory, "helper")
		command := exec.Command("go", "build", "-o", helperPath, source)
		helperErr = command.Run()
		if helperErr == nil {
			helperPath, helperErr = filepath.EvalSymlinks(helperPath)
		}
	})
	if helperErr != nil {
		t.Fatal(helperErr)
	}
	return helperPath
}

func helperDigest(t *testing.T) string {
	t.Helper()
	contents, err := os.ReadFile(nativeHelper(t))
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(contents))
}

func testPlan(t *testing.T, request string) Plan {
	t.Helper()
	helper, digest := nativeHelper(t), helperDigest(t)
	host, err := ActualHostPlatform()
	if err != nil {
		t.Fatal(err)
	}
	plan := Plan{Artifact: helper, ExpectedArtifactSHA256: digest, Executable: helper, ExpectedExecutableSHA256: digest, Host: host, Target: host, RepositoryRoot: t.TempDir(), StagingParent: ownerPrivateTempDir(t), Request: []byte(request), Timeout: 100 * time.Millisecond, MaxStdoutBytes: 64 << 10, MaxStderrBytes: 64 << 10, MaxChildren: 1}
	plan.InvocationBindingSHA256 = InvocationBindingSHA256(plan.Request, plan.Host, plan.Target)
	return plan
}

func ownerPrivateTempDir(t *testing.T) string {
	t.Helper()
	directory, err := os.MkdirTemp("", "analyzerexec-stage-")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	return directory
}

func TestStagePinsNativeBytes(t *testing.T) {
	plan := testPlan(t, `{"request":"fixed"}`)
	if file, err := openRelativeNoFollow(string(filepath.Separator), strings.TrimPrefix(plan.Executable, string(filepath.Separator))); err != nil {
		t.Fatalf("descriptor traversal failed for %q: %v", plan.Executable, err)
	} else {
		file.Close()
	}
	staged, cleanup, err := stage(plan)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	contents, err := io.ReadAll(staged.executable)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(contents)) != plan.ExpectedExecutableSHA256 || staged.artifactDigest != plan.ExpectedArtifactSHA256 || staged.digest != plan.ExpectedExecutableSHA256 {
		t.Fatal("staged bytes do not equal pinned digest")
	}
}

func TestRetainedDescriptorsCloseOnExec(t *testing.T) {
	plan := testPlan(t, "cloexec")
	for _, path := range []string{plan.Artifact, plan.Executable} {
		file, err := openNoFollow(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, file.Fd(), syscall.F_GETFD, 0)
		if errno != 0 || flags&syscall.FD_CLOEXEC == 0 {
			t.Fatalf("retained descriptor leaks across exec: flags=%#x errno=%v", flags, errno)
		}
	}
}

// ACC-V0-002: traversal descriptors close without waiting for finalizers.
func TestSourceWalkClosesDescriptors(t *testing.T) {
	directory := t.TempDir()
	relative := filepath.Join("one", "two", "three", "executable")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(directory, relative)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, relative), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	previous := debug.SetGCPercent(-1)
	defer func() { debug.SetGCPercent(previous); runtime.GC() }()
	before := openDescriptorCount(t)
	file, err := openRelativeNoFollow(directory, relative)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	after := openDescriptorCount(t)
	t.Logf("depth=3 descriptors before=%d after=%d delta=%d", before, after, after-before)
	if after != before {
		t.Fatalf("source walk retained %d descriptors", after-before)
	}
}

// ACC-V0-002: a failed post-open Lstat must also close the bound directory file.
func TestSourceWalkLstatFailureClosesBoundDescriptor(t *testing.T) {
	directory := t.TempDir()
	relative := filepath.Join("one", "two", "three", "executable")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(directory, relative)), 0o700); err != nil {
		t.Fatal(err)
	}
	original := restatSourceDirectory
	defer func() { restatSourceDirectory = original }()
	injected := false
	restatSourceDirectory = func(root *os.Root, name string) (os.FileInfo, error) {
		if err := os.Rename(filepath.Join(directory, "one"), filepath.Join(directory, "moved")); err != nil {
			t.Fatal(err)
		}
		info, err := original(root, name)
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("post-open Lstat did not fail with ENOENT: %v", err)
		}
		injected = true
		return info, err
	}
	runtime.GC()
	previous := debug.SetGCPercent(-1)
	defer func() { debug.SetGCPercent(previous); runtime.GC() }()
	before := openDescriptorCount(t)
	file, err := openRelativeNoFollow(directory, relative)
	if file != nil {
		file.Close()
		t.Fatal("source walk returned a file after directory removal")
	}
	if !injected || err == nil || err.Error() != "component race" {
		t.Fatalf("post-open Lstat failure did not reject the walk: injected=%t err=%v", injected, err)
	}
	after := openDescriptorCount(t)
	t.Logf("post-open Lstat ENOENT descriptors before=%d after=%d delta=%d", before, after, after-before)
	if after != before {
		t.Fatalf("failed source walk retained %d descriptors", after-before)
	}
}

func openDescriptorCount(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func TestNativeExecutablePlatformRequiresCompleteNativeObject(t *testing.T) {
	for _, value := range [][]byte{{0x7f, 'E', 'L', 'F'}, {0xcf, 0xfa, 0xed, 0xfe}, make([]byte, 64)} {
		path := filepath.Join(t.TempDir(), "forged-native")
		if err := os.WriteFile(path, value, 0o500); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		_, parseErr := NativeExecutablePlatform(file)
		file.Close()
		if parseErr == nil {
			t.Fatal("incomplete native object accepted")
		}
	}
	helper, err := os.Open(nativeHelper(t))
	if err != nil {
		t.Fatal(err)
	}
	defer helper.Close()
	candidate, err := NativeExecutablePlatform(helper)
	if err != nil {
		t.Fatal(err)
	}
	host, err := ActualHostPlatform()
	if err != nil || !nativeExecutableMatchesHost(candidate, host) {
		t.Fatalf("actual native header mismatch: candidate=%#v host=%#v err=%v", candidate, host, err)
	}
}

func TestLaunchValidationAcceptsPinnedNativeExecutableWithoutToolchainProvenance(t *testing.T) {
	directory := t.TempDir()
	source, binary := filepath.Join(directory, "native.c"), filepath.Join(directory, "native")
	if err := os.WriteFile(source, []byte("int main(void) { return 0; }"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("cc", "-o", binary, source).CombinedOutput(); err != nil {
		t.Fatalf("build C positive control: %v: %s", err, output)
	}
	binary, err := filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(contents))
	host, hostErr := ActualHostPlatform()
	if hostErr != nil {
		t.Fatal(hostErr)
	}
	plan := Plan{Artifact: binary, ExpectedArtifactSHA256: digest, Executable: binary, ExpectedExecutableSHA256: digest, Host: host, Target: host, RepositoryRoot: t.TempDir(), StagingParent: ownerPrivateTempDir(t), Request: []byte("native"), Timeout: 100 * time.Millisecond, MaxStdoutBytes: 64 << 10, MaxStderrBytes: 64 << 10, MaxChildren: 1}
	plan.InvocationBindingSHA256 = InvocationBindingSHA256(plan.Request, plan.Host, plan.Target)
	staged, cleanup, err := stageContext(context.Background(), plan)
	if err != nil {
		t.Fatalf("valid native stage rejected before provenance check: %v", err)
	}
	defer cleanup()
	if err := staged.validateForLaunch(context.Background()); err != nil {
		t.Fatalf("pinned native executable validation err=%v", err)
	}
}

func TestNativeABITupleControlsHostAcceptance(t *testing.T) {
	plan := testPlan(t, "native-abi")
	plan.Host.ABI += "-forged"
	plan.InvocationBindingSHA256 = InvocationBindingSHA256(plan.Request, plan.Host, plan.Target)
	if _, cleanup, err := stageContext(context.Background(), plan); cleanup != nil || !Is(err, Unsupported) {
		t.Fatalf("forged ABI err=%v hasCleanup=%t", err, cleanup != nil)
	}
}

func TestCancelledStagingDoesNotCreateResidue(t *testing.T) {
	plan := testPlan(t, "cancelled-stage")
	before, err := os.ReadDir(plan.StagingParent)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, cleanup, err := stageContext(ctx, plan)
	if cleanup != nil {
		cleanup()
	}
	if !Is(err, Cancelled) {
		t.Fatalf("cancelled staging error=%v", err)
	}
	after, err := os.ReadDir(plan.StagingParent)
	if err != nil || len(before) != len(after) {
		t.Fatalf("cancelled staging retained residue: before=%d after=%d err=%v", len(before), len(after), err)
	}
	plan.Timeout = time.Nanosecond
	result, err := Run(context.Background(), plan)
	if !Is(err, Timeout) || result.Started || result.CleanupState != CleanupNotRun || !result.ValidCleanupObservation() {
		t.Fatalf("staging timeout was not a closed no-start result: %#v %v", result, err)
	}
}

func TestLaunchValidationHonorsPreLaunchCancellation(t *testing.T) {
	staged, cleanup, err := stageContext(context.Background(), testPlan(t, "cancelled-pre-launch"))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := staged.validateForLaunch(ctx); !Is(err, Cancelled) {
		t.Fatalf("pre-launch cancellation err=%v", err)
	}
}

func TestStageRandomnessFailureIsTyped(t *testing.T) {
	previous := readStageRandom
	defer func() { readStageRandom = previous }()
	readStageRandom = func([]byte) (int, error) { return 0, errors.New("entropy unavailable") }
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, _, err := newStageRoot(context.Background(), root); !Is(err, Process) {
		t.Fatalf("staging entropy err=%v", err)
	}
}

func TestStageHashPassRatchet(t *testing.T) {
	plan := testPlan(t, "hash-pass-ratchet")
	info, err := os.Stat(plan.Executable)
	if err != nil {
		t.Fatal(err)
	}
	type counter struct{ bytes int64 }
	copy := func(counter *counter) stageCopier {
		return func(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
			count, err := copyWithContext(ctx, destination, source)
			counter.bytes += count
			return count, err
		}
	}
	current := &counter{}
	started := time.Now()
	staged, cleanup, err := stageWithCopy(context.Background(), plan, copy(current))
	if err != nil {
		t.Logf("private staging error: %T", err)
		t.Fatal(err)
	}
	defer cleanup()
	wantCurrent := info.Size() * 2 // artifact hash + source-to-stage hash/copy
	if current.bytes != wantCurrent {
		t.Fatalf("current staging read %d bytes, want exactly %d", current.bytes, wantCurrent)
	}
	restored := &counter{bytes: current.bytes}
	if _, err := copy(restored)(context.Background(), io.Discard, staged.executable); err != nil {
		t.Fatal(err)
	}
	if restored.bytes-current.bytes != info.Size() {
		t.Fatalf("restored staged rehash did not add one executable pass: current=%d restored=%d size=%d", current.bytes, restored.bytes, info.Size())
	}
	t.Logf("private stage read ratchet: current=%d restored=%d elapsed=%s", current.bytes, restored.bytes, time.Since(started))
}

func TestProductionStageWorkRatchet(t *testing.T) {
	plan := testPlan(t, "production-work-ratchet")
	info, err := os.Stat(plan.Executable)
	if err != nil {
		t.Fatal(err)
	}
	staged, cleanup, err := stageContext(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if staged.workBytes != info.Size()*2 {
		t.Fatalf("production stage work=%d want=%d", staged.workBytes, info.Size()*2)
	}
}

func TestStageRejectsPostCopyHardLinkRaces(t *testing.T) {
	for _, test := range []struct {
		name, target string
		copyPass     int
	}{
		{"artifact", "artifact", 1},
		{"executable", "executable", 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := testPlan(t, "hard-link-race")
			target := plan.Artifact
			if test.target == "executable" {
				target = plan.Executable
			}
			link := filepath.Join(t.TempDir(), "race-link")
			pass := 0
			copy := func(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
				copied, err := copyWithContext(ctx, destination, source)
				pass++
				if pass == test.copyPass {
					if linkErr := os.Link(target, link); linkErr != nil {
						return copied, linkErr
					}
				}
				return copied, err
			}
			_, cleanup, err := stageWithCopy(context.Background(), plan, copy)
			if cleanup != nil {
				_ = cleanup()
			}
			if !Is(err, Race) {
				t.Fatalf("post-copy hard-link race err=%v", err)
			}
		})
	}
}

func TestLaunchValidationRetainsDescriptorAndRejectsPostStageMutation(t *testing.T) {
	t.Run("same-owner write", func(t *testing.T) {
		plan := testPlan(t, "post-stage-write")
		staged, cleanup, err := stageContext(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		stagePath := filepath.Join(plan.StagingParent, staged.directory, staged.digest)
		if entry, err := os.Lstat(stagePath); err != nil || !samePinnedRegular(staged.stagingInfo, entry) {
			t.Fatalf("staged executable path lost descriptor identity: entry=%#v err=%v", entry, err)
		}
		if err := os.Chmod(stagePath, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(stagePath, []byte("replaced"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := staged.validateForLaunch(context.Background()); !Is(err, Race) {
			t.Fatalf("post-stage write err=%v, want race", err)
		}
	})
	t.Run("post-stage hard link", func(t *testing.T) {
		plan := testPlan(t, "post-stage-link")
		staged, cleanup, err := stageContext(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		stagePath := filepath.Join(plan.StagingParent, staged.directory, staged.digest)
		if err := os.Remove(stagePath); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(nativeHelper(t), stagePath); err != nil {
			t.Fatal(err)
		}
		if err := staged.validateForLaunch(context.Background()); !Is(err, Race) {
			t.Fatalf("post-stage hard link err=%v, want race", err)
		}
	})
	t.Run("directory replacement cannot redirect descriptor", func(t *testing.T) {
		plan := testPlan(t, "post-stage-replace")
		staged, cleanup, err := stageContext(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		stageRoot := filepath.Join(plan.StagingParent, staged.directory)
		moved := filepath.Join(t.TempDir(), "moved-stage")
		if err := os.Rename(stageRoot, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("/dev/null", stageRoot); err != nil {
			t.Fatal(err)
		}
		if err := staged.validateForLaunch(context.Background()); !Is(err, Race) {
			t.Fatalf("descriptor seal accepted replaced launch directory path: %v", err)
		}
	})
}

func TestStagingRootRejectsSymlinkBeforeAnyCopy(t *testing.T) {
	plan := testPlan(t, "staging-swap")
	boundParent, repository := t.TempDir(), t.TempDir()
	link := filepath.Join(t.TempDir(), "staging")
	if err := os.Symlink(boundParent, link); err != nil {
		t.Fatal(err)
	}
	plan.RepositoryRoot, plan.StagingParent = repository, link
	if _, cleanup, err := stageContext(context.Background(), plan); cleanup != nil || !Is(err, IdentityUnsafe) {
		t.Fatalf("symlink staging root err=%v hasCleanup=%t", err, cleanup != nil)
	}
}

func TestStagingRootMustBeOwnerPrivate(t *testing.T) {
	plan := testPlan(t, "private-staging")
	if err := os.Chmod(plan.StagingParent, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, cleanup, err := stageContext(context.Background(), plan); cleanup != nil || !Is(err, IdentityUnsafe) {
		t.Fatalf("shared staging root err=%v hasCleanup=%t", err, cleanup != nil)
	}
}

func TestRootBindingCleanupFailureRetainsNoStartEvidence(t *testing.T) {
	plan := testPlan(t, "root-bind-cleanup")
	link := filepath.Join(t.TempDir(), "staging-link")
	if err := os.Symlink(plan.StagingParent, link); err != nil {
		t.Fatal(err)
	}
	plan.StagingParent = link
	previous := closeStageRoot
	defer func() { closeStageRoot = previous }()
	closeStageRoot = func(*os.Root) error { return errors.New("injected root close failure") }
	result, err := Run(context.Background(), plan)
	if !Is(err, IdentityUnsafe) || result.Started || result.CleanupState != CleanupFailed || result.CleanupError != CleanupErrorClose || !result.ValidCleanupObservation() {
		t.Fatalf("root-binding cleanup evidence=%#v err=%v", result, err)
	}
}

func TestOpenRootCleanupFailureRetainsNoStartEvidence(t *testing.T) {
	oldOpen, oldRemove := openStageRoot, removeStageDirectory
	defer func() { openStageRoot, removeStageDirectory = oldOpen, oldRemove }()
	openStageRoot = func(*os.Root, string) (*os.Root, error) { return nil, errors.New("open root") }
	removeStageDirectory = func(*os.Root, string) error { return errors.New("remove root") }
	started := time.Now()
	_, cleanup, err := stageContext(context.Background(), testPlan(t, "open-root-cleanup"))
	if cleanup != nil {
		_ = cleanup()
	}
	result := stageFailureResult(started, err)
	if !Is(err, Process) || result.Started || result.CleanupState != CleanupFailed || result.CleanupError != CleanupErrorRemove || !result.ValidCleanupObservation() {
		t.Fatalf("open-root cleanup evidence=%#v err=%v", result, err)
	}
}

func TestStageFailureRetainsCleanupResidueEvidence(t *testing.T) {
	err := &stagingFailure{cause: &Error{Failure: DigestMismatch}, cleanup: CleanupErrorRemove, cleanupElapsed: time.Millisecond}
	result := stageFailureResult(time.Now().Add(-2*time.Millisecond), err)
	if !Is(err, DigestMismatch) || result.Started || result.CleanupState != CleanupFailed || result.CleanupError != CleanupErrorRemove || !result.ValidCleanupObservation() {
		t.Fatalf("staging residue evidence lost: %#v %v", result, err)
	}
	t.Logf("private staging cleanup error: class=%s", result.CleanupError)
}

func TestStageCleanupReportsDescriptorFailure(t *testing.T) {
	staged, cleanup, err := stage(testPlan(t, "cleanup"))
	if err != nil {
		t.Fatal(err)
	}
	if err := staged.executable.Close(); err != nil {
		t.Fatal(err)
	}
	if got := cleanup(); got != CleanupErrorClose {
		t.Fatalf("cleanup error=%q, want %q", got, CleanupErrorClose)
	}
}

func TestEveryPrestartDescriptorCloseFailureIsRetained(t *testing.T) {
	type resources struct {
		executable, source, artifact *os.File
		root, parent                 *os.Root
		bindings                     rootBindings
	}
	for _, target := range []string{"executable", "stage-root", "source", "artifact", "parent", "repository-root", "staging-root"} {
		t.Run(target, func(t *testing.T) {
			var value resources
			var failedFile *os.File
			var failedRoot *os.Root
			switch target {
			case "executable", "source", "artifact":
				file, err := os.CreateTemp(t.TempDir(), "descriptor")
				if err != nil {
					t.Fatal(err)
				}
				failedFile = file
				defer file.Close()
				switch target {
				case "executable":
					value.executable = file
				case "source":
					value.source = file
				case "artifact":
					value.artifact = file
				}
			default:
				root, err := os.OpenRoot(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				failedRoot = root
				defer root.Close()
				switch target {
				case "stage-root":
					value.root = root
				case "parent":
					value.parent = root
				case "repository-root":
					value.bindings.repository.root = root
				case "staging-root":
					value.bindings.staging.root = root
				}
			}
			previousFile, previousRoot := closeStageFile, closeStageRoot
			defer func() { closeStageFile, closeStageRoot = previousFile, previousRoot }()
			closeStageFile = func(file *os.File) error {
				if file == failedFile {
					return errors.New("injected prestart file close failure")
				}
				return previousFile(file)
			}
			closeStageRoot = func(root *os.Root) error {
				if root == failedRoot {
					return errors.New("injected prestart root close failure")
				}
				return previousRoot(root)
			}
			cleanup := cleanupStage(value.executable, value.root, value.source, value.artifact, value.parent, "", value.bindings)
			failure := stagingFailureFor(&Error{Failure: Process}, cleanup, time.Nanosecond)
			result := stageFailureResult(time.Now().Add(-time.Millisecond), failure)
			if cleanup != CleanupErrorClose || result.Started || result.CleanupState != CleanupFailed || result.CleanupError != CleanupErrorClose || !result.ValidCleanupObservation() {
				t.Fatalf("prestart %s close evidence=%#v cleanup=%s", target, result, cleanup)
			}
		})
	}
}

func TestInvocationBindingRejectsTargetOrRequestSubstitution(t *testing.T) {
	for _, mutate := range []func(*Plan){
		func(plan *Plan) { plan.Target.Architecture = "other" },
		func(plan *Plan) { plan.Request = append([]byte(nil), []byte("other")...) },
	} {
		plan := testPlan(t, "bound-request")
		mutate(&plan)
		result, err := Run(context.Background(), plan)
		if !Is(err, Limit) || result.Started || !result.ValidCleanupObservation() {
			t.Fatalf("substituted invocation result=%#v err=%v", result, err)
		}
	}
}

func TestRunRejectsUnsafeExecutableInputs(t *testing.T) {
	plan := testPlan(t, "request")
	plan.ExpectedExecutableSHA256 = strings.Repeat("0", sha256.Size*2)
	_, cleanup, err := stage(plan)
	if cleanup != nil {
		cleanup()
	}
	if !Is(err, DigestMismatch) {
		t.Fatalf("digest err = %v", err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(plan.Executable, link); err != nil {
		t.Fatal(err)
	}
	plan.Executable = link
	plan.ExpectedExecutableSHA256 = helperDigest(t)
	_, cleanup, err = stage(plan)
	if cleanup != nil {
		cleanup()
	}
	if !Is(err, IdentityUnsafe) {
		t.Fatalf("symlink err = %v", err)
	}
}

func TestStagePinsArtifactAndExecutableIndependently(t *testing.T) {
	plan := testPlan(t, "request")
	artifact := filepath.Join(t.TempDir(), "artifact")
	contents, err := os.ReadFile(plan.Executable)
	if err != nil {
		t.Fatal(err)
	}
	contents = append(contents, []byte("artifact-binding")...)
	if err := os.WriteFile(artifact, contents, 0o500); err != nil {
		t.Fatal(err)
	}
	artifact, err = filepath.EvalSymlinks(artifact)
	if err != nil {
		t.Fatal(err)
	}
	plan.Artifact = artifact
	plan.ExpectedArtifactSHA256 = fmt.Sprintf("%x", sha256.Sum256(contents))
	staged, cleanup, err := stage(plan)
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
	if staged.artifactDigest != plan.ExpectedArtifactSHA256 || staged.digest != plan.ExpectedExecutableSHA256 {
		t.Fatalf("staged=%#v", staged)
	}
	plan.ExpectedArtifactSHA256 = strings.Repeat("0", sha256.Size*2)
	if _, cleanup, err := stage(plan); cleanup != nil || !Is(err, DigestMismatch) {
		t.Fatalf("artifact err=%v", err)
	}
	plan = testPlan(t, "request")
	plan.ExpectedExecutableSHA256 = strings.Repeat("0", sha256.Size*2)
	if _, cleanup, err := stage(plan); cleanup != nil || !Is(err, DigestMismatch) {
		t.Fatalf("executable err=%v", err)
	}
}

func TestStageReusesVerifiedContentAddressedEntry(t *testing.T) {
	plan := testPlan(t, "reuse")
	first, cleanup, err := stage(plan)
	if err != nil {
		t.Fatal(err)
	}
	firstInfo := first.stagingInfo
	if got := cleanup(); got != CleanupErrorNone {
		t.Fatalf("cleanup=%s", got)
	}
	entries, err := os.ReadDir(plan.StagingParent)
	if err != nil || len(entries) != 1 || entries[0].Name() != publishedStageName(plan.ExpectedExecutableSHA256) {
		t.Fatalf("staging parent after cleanup=%v err=%v, want only the content-addressed entry", entries, err)
	}
	info, err := os.Stat(plan.Executable)
	if err != nil {
		t.Fatal(err)
	}
	var read int64
	counted := func(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
		count, err := copyWithContext(ctx, destination, source)
		read += count
		return count, err
	}
	second, cleanup, err := stageWithCopy(context.Background(), plan, counted)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if !samePinnedRegular(firstInfo, second.stagingInfo) || read != info.Size()*2 || second.validateForLaunch(context.Background()) != nil {
		t.Fatalf("reuse did not pin the verified entry: same=%t read=%d size=%d", samePinnedRegular(firstInfo, second.stagingInfo), read, info.Size())
	}
}

func TestStageRemovesDivergentReusedEntry(t *testing.T) {
	plan := testPlan(t, "divergent-reuse")
	_, cleanup, err := stage(plan)
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
	directory := filepath.Join(plan.StagingParent, publishedStageName(plan.ExpectedExecutableSHA256))
	stagePath := filepath.Join(directory, plan.ExpectedExecutableSHA256)
	if err := os.Chmod(stagePath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagePath, []byte("replaced"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(stagePath, 0o500); err != nil {
		t.Fatal(err)
	}
	if _, cleanup, err := stage(plan); cleanup != nil || !Is(err, Race) {
		t.Fatalf("divergent reused entry err=%v hasCleanup=%t", err, cleanup != nil)
	}
	if _, err := os.Lstat(directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("divergent entry survived: %v", err)
	}
	if _, cleanup, err := stage(plan); err != nil {
		t.Fatalf("restage after divergent entry: %v", err)
	} else {
		cleanup()
	}
}

func TestStageKeepsReusedEntryWhenCancelled(t *testing.T) {
	plan := testPlan(t, "cancelled-reuse")
	_, cleanup, err := stage(plan)
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	cancelDuringStaging := func(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
		count, err := copyWithContext(ctx, destination, source)
		if calls++; calls == 2 {
			cancel()
		}
		return count, err
	}
	if _, cleanup, err := stageWithCopy(ctx, plan, cancelDuringStaging); cleanup != nil || !Is(err, Cancelled) {
		t.Fatalf("cancelled reuse err=%v hasCleanup=%t", err, cleanup != nil)
	}
	directory := filepath.Join(plan.StagingParent, publishedStageName(plan.ExpectedExecutableSHA256))
	if _, err := os.Lstat(directory); err != nil {
		t.Fatalf("cancelled reuse removed the verified entry: %v", err)
	}
}

func TestRunRejectsRepositoryAndStagingContainment(t *testing.T) {
	if !containedBackendSupported() {
		// Containment is checked while staging; without a backend Run refuses before
		// staging, which TestUnsupportedBackendDoesNotStageOrReplaceStatus pins.
		t.Skip("no contained backend on this platform")
	}
	plan := testPlan(t, "request")
	plan.RepositoryRoot = filepath.Dir(plan.Executable)
	if result, err := Run(context.Background(), plan); !Is(err, IdentityUnsafe) || result.Started || !result.ValidCleanupObservation() {
		t.Fatalf("repository containment result=%#v err=%v", result, err)
	}
	plan = testPlan(t, "request")
	plan.StagingParent = plan.RepositoryRoot
	if result, err := Run(context.Background(), plan); !Is(err, IdentityUnsafe) || result.Started || !result.ValidCleanupObservation() {
		t.Fatalf("staging containment result=%#v err=%v", result, err)
	}
}

// TestRunRejectsCaseAliasedContainment pins containment by directory identity:
// on a case-insensitive volume a repository root spelled with different case
// names the same directory, so the lexical path comparison alone admits an
// executable or staging parent that lives inside the repository.
func TestRunRejectsCaseAliasedContainment(t *testing.T) {
	plan := testPlan(t, "request")
	helperDirectory := filepath.Dir(plan.Executable)
	alias := filepath.Join(filepath.Dir(helperDirectory), strings.ToUpper(filepath.Base(helperDirectory)))
	original, originalErr := os.Lstat(helperDirectory)
	aliased, aliasErr := os.Lstat(alias)
	if originalErr != nil || aliasErr != nil || alias == helperDirectory || !os.SameFile(original, aliased) {
		t.Skip("volume is case-sensitive")
	}
	plan.RepositoryRoot = alias
	if result, err := Run(context.Background(), plan); !Is(err, IdentityUnsafe) || result.Started || !result.ValidCleanupObservation() {
		t.Fatalf("case-aliased repository containment result=%#v err=%v", result, err)
	}
	plan = testPlan(t, "request")
	repository := filepath.Join(plan.RepositoryRoot, "repo")
	plan.StagingParent = filepath.Join(repository, "stage")
	if err := os.MkdirAll(plan.StagingParent, 0o700); err != nil {
		t.Fatal(err)
	}
	plan.RepositoryRoot = filepath.Join(filepath.Dir(repository), "REPO")
	if result, err := Run(context.Background(), plan); !Is(err, IdentityUnsafe) || result.Started || !result.ValidCleanupObservation() {
		t.Fatalf("case-aliased staging containment result=%#v err=%v", result, err)
	}
}

func TestUnavailableNoStartPathsRetainCleanupNotRun(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, test := range []struct {
		name string
		ctx  context.Context
		plan Plan
		want Failure
	}{
		{"invalid plan", context.Background(), Plan{}, Limit},
		{"cancelled plan", cancelled, testPlan(t, "request"), Cancelled},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := Run(test.ctx, test.plan)
			if !Is(err, test.want) || result.Started || result.CleanupState != CleanupNotRun || !result.ValidCleanupObservation() {
				t.Fatalf("result=%#v err=%v", result, err)
			}
		})
	}
}

func TestUnsupportedBackendDoesNotStageOrReplaceStatus(t *testing.T) {
	if containedBackendSupported() {
		t.Skip("contained Darwin backend stages before launch")
	}
	plan := testPlan(t, "unsupported")
	plan.ExpectedArtifactSHA256 = strings.Repeat("0", sha256.Size*2)
	result, err := Run(context.Background(), plan)
	if !Is(err, Unsupported) || result.Started || result.CleanupState != CleanupNotRun || !result.ValidCleanupObservation() {
		t.Fatalf("unsupported backend lost no-start status: %#v %v", result, err)
	}
}

func TestPlanRejectionPrecedence(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, test := range []struct {
		name   string
		mutate func(*Plan)
		want   Failure
	}{
		{"numeric limit before identity and cancellation", func(plan *Plan) { plan.MaxChildren = 2; plan.Executable = "relative" }, Limit},
		{"identity before digest and cancellation", func(plan *Plan) { plan.Executable = "relative"; plan.ExpectedExecutableSHA256 = "not-a-digest" }, IdentityUnsafe},
		{"digest before cancellation", func(plan *Plan) { plan.ExpectedExecutableSHA256 = "not-a-digest" }, DigestMismatch},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := testPlan(t, "request")
			test.mutate(&plan)
			result, err := Run(cancelled, plan)
			if !Is(err, test.want) || result.Started || !result.ValidCleanupObservation() {
				t.Fatalf("result=%#v err=%v want=%s", result, err, test.want)
			}
		})
	}
}

func TestCleanupObservationIsClosed(t *testing.T) {
	if (Result{}).ValidCleanupObservation() {
		t.Fatal("empty cleanup observation accepted")
	}
	if (Result{CleanupState: CleanupObserved}).ValidCleanupObservation() {
		t.Fatal("unstarted observed cleanup accepted")
	}
	if (Result{Started: true, CleanupState: CleanupNotRun}).ValidCleanupObservation() {
		t.Fatal("started not-run cleanup accepted")
	}
	if !(Result{Started: true, Completed: true, Termination: TerminationExited, CleanupState: CleanupObserved, CleanupError: CleanupErrorNone}).ValidCleanupObservation() {
		t.Fatal("observed zero cleanup rejected")
	}
	if (Result{Started: true, Completed: true, Termination: TerminationExited, Elapsed: -time.Nanosecond, CleanupState: CleanupObserved}).ValidCleanupObservation() {
		t.Fatal("negative started elapsed accepted")
	}
	if (Result{Started: true, Completed: true, Termination: TerminationExited, Cleanup: -time.Nanosecond, CleanupState: CleanupObserved}).ValidCleanupObservation() {
		t.Fatal("negative observed cleanup accepted")
	}
	if (Result{Started: true, Completed: true, Termination: TerminationExited, CleanupState: CleanupRejected}).ValidCleanupObservation() {
		t.Fatal("backend-rejected cleanup accepted")
	}
	if !(Result{Started: true, Completed: true, Termination: TerminationFailed, CleanupState: CleanupFailed, CleanupError: CleanupErrorRemove}).ValidCleanupObservation() {
		t.Fatal("typed failed cleanup rejected")
	}
	for _, result := range []Result{
		{Started: true, Completed: true, Termination: Termination("UNKNOWN"), CleanupState: CleanupObserved, CleanupError: CleanupErrorNone},
		{Started: true, Completed: true, Termination: TerminationExited, CleanupState: CleanupFailed, CleanupError: CleanupError("UNKNOWN")},
	} {
		if result.ValidCleanupObservation() {
			t.Fatalf("accepted open receipt enum: %#v", result)
		}
	}
}
