//go:build darwin || linux

package analyzerexec

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const maxRequestBytes = 64 << 10

var openStageRoot = func(parent *os.Root, name string) (*os.Root, error) { return parent.OpenRoot(name) }
var removeStageDirectory = func(parent *os.Root, name string) error { return parent.RemoveAll(name) }
var readStageRandom = rand.Read
var closeStageFile = func(file *os.File) error { return file.Close() }
var closeStageRoot = func(root *os.Root) error { return root.Close() }
var restatSourceDirectory = func(root *os.Root, name string) (os.FileInfo, error) { return root.Lstat(name) }

// sealedArtifact contains only retained descriptors. No path to the staged
// executable escapes this package, so a backend cannot re-resolve mutable
// bytes after stage verifies them.
type sealedArtifact struct {
	executable     *os.File
	artifact       *os.File
	parent         *os.Root
	root           *os.Root
	directory      string
	stagingInfo    os.FileInfo
	artifactInfo   os.FileInfo
	workBytes      int64
	digest         string
	artifactDigest string
	launchPath     string
	host           NativePlatform
	target         NativePlatform
}

func validPlatformTuple(platform NativePlatform) bool {
	for _, value := range []string{platform.OS, platform.Architecture, platform.ABI} {
		if len(value) == 0 || len(value) > 128 {
			return false
		}
		for index := range len(value) {
			if value[index] < 0x21 || value[index] > 0x7e {
				return false
			}
		}
	}
	return true
}

func (sealed sealedArtifact) validateForLaunch(ctx context.Context) error {
	if err := contextFailure(ctx); err != nil {
		return err
	}
	if !validPlatformTuple(sealed.host) || !validPlatformTuple(sealed.target) {
		return &Error{Failure: IdentityUnsafe}
	}
	// Darwin has no fexecve. The executable remains in Core's 0700 random
	// staging directory until cleanup, while this retained descriptor and the
	// directory entry must name the same owner-private regular file immediately
	// before sandbox-exec receives its exact path. Same-UID active tampering is
	// outside the approved host trust boundary; all other writers are excluded.
	if err := validateStageDirectory(sealed.parent, sealed.directory, sealed.root); err != nil {
		return err
	}
	entry, err := sealed.root.Lstat(sealed.digest)
	if err != nil || !samePinnedRegular(sealed.stagingInfo, entry) {
		return &Error{Failure: Race}
	}
	if err := validatePinnedDescriptor(ctx, sealed.executable, sealed.stagingInfo, sealed.digest); err != nil {
		return err
	}
	if err := validatePinnedDescriptor(ctx, sealed.artifact, sealed.artifactInfo, sealed.artifactDigest); err != nil {
		return err
	}
	if err := contextFailure(ctx); err != nil {
		return err
	}
	platform, err := NativeExecutablePlatform(sealed.executable)
	if err != nil || !nativeExecutableMatchesHost(platform, sealed.host) {
		return &Error{Failure: IdentityUnsafe}
	}
	if err := contextFailure(ctx); err != nil {
		return err
	}
	return nil
}

// validateStageDirectory requires name in parent to be the owner-private
// directory that root is bound to.
func validateStageDirectory(parent *os.Root, name string, root *os.Root) error {
	directoryEntry, err := parent.Lstat(name)
	if err != nil || !safePrivateDirectory(directoryEntry) {
		return &Error{Failure: Race}
	}
	directoryDescriptor, err := root.Open(".")
	if err != nil {
		return &Error{Failure: Race}
	}
	directoryBound, statErr := directoryDescriptor.Stat()
	closeErr := directoryDescriptor.Close()
	if statErr != nil || closeErr != nil || !safePrivateDirectory(directoryBound) || !samePinnedDirectory(directoryEntry, directoryBound) {
		return &Error{Failure: Race}
	}
	return nil
}

func validatePinnedDescriptor(ctx context.Context, file *os.File, pinned os.FileInfo, expected string) error {
	if err := contextFailure(ctx); err != nil {
		return err
	}
	descriptor, err := file.Stat()
	if err != nil || !samePinnedRegular(pinned, descriptor) {
		return &Error{Failure: Race}
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return &Error{Failure: Race}
	}
	hash := sha256.New()
	count, err := copyWithContext(ctx, hash, io.LimitReader(file, MaxExecutableBytes+1))
	if err != nil || count != descriptor.Size() || hex.EncodeToString(hash.Sum(nil)) != expected {
		if contextErr := contextFailure(ctx); contextErr != nil {
			return contextErr
		}
		return &Error{Failure: Race}
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return &Error{Failure: Race}
	}
	return nil
}

func Run(ctx context.Context, plan Plan) (result Result, err error) {
	if err := validatePlanShape(ctx, plan); err != nil {
		return noStartResult(), err
	}
	started := time.Now()
	runContext, cancel := context.WithTimeout(ctx, plan.Timeout)
	defer cancel()
	roots, err := validatePlan(runContext, plan)
	if err != nil {
		var failure *stagingFailure
		if errors.As(err, &failure) {
			return stageFailureResult(started, err), err
		}
		return noStartResult(), err
	}
	if err := contextFailure(runContext); err != nil {
		cleanupStarted := time.Now()
		failure := stagingFailureFor(err, roots.close(), time.Since(cleanupStarted))
		return stageFailureResult(started, failure), failure
	}
	if !containedBackendSupported() {
		cleanup := roots.close()
		if cleanup != CleanupErrorNone {
			failure := &stagingFailure{cause: &Error{Failure: Unsupported}, cleanup: cleanup}
			return stageFailureResult(started, failure), failure
		}
		return noStartResult(), &Error{Failure: Unsupported}
	}
	sealed, cleanup, err := stageContextRoots(runContext, plan, roots)
	if err != nil {
		return stageFailureResult(started, err), err
	}
	result, err = runContained(runContext, sealed, plan)
	result.Elapsed = time.Since(started)
	cleanupStarted := time.Now()
	cleanupErr := cleanup()
	cleanupElapsed := time.Since(cleanupStarted)
	if cleanupErr != CleanupErrorNone {
		result.Cleanup = cleanupElapsed
		result.CleanupState, result.CleanupError = CleanupFailed, cleanupErr
		if result.Started {
			result.Completed = true
			if result.Termination == TerminationNotRun {
				result.Termination = TerminationFailed
			}
		}
		if err == nil {
			err = &Error{Failure: Process}
		}
		return result, err
	}
	if result.Started {
		result.Cleanup = cleanupElapsed
		result.CleanupState, result.CleanupError = CleanupObserved, CleanupErrorNone
	} else {
		result.Cleanup = 0
		result.CleanupState, result.CleanupError = CleanupNotRun, CleanupErrorNone
	}
	return result, err
}

type stagingFailure struct {
	cause          error
	cleanup        CleanupError
	cleanupElapsed time.Duration
}

func (failure *stagingFailure) Error() string { return failure.cause.Error() }
func (failure *stagingFailure) Unwrap() error { return failure.cause }

func stageFailureResult(started time.Time, err error) Result {
	result := noStartResult()
	result.Elapsed = time.Since(started)
	var failure *stagingFailure
	if errors.As(err, &failure) && failure.cleanup != CleanupErrorNone {
		result.Cleanup = failure.cleanupElapsed
		result.CleanupState = CleanupFailed
		result.CleanupError = failure.cleanup
	}
	return result
}

func noStartResult() Result {
	return Result{CleanupState: CleanupNotRun, Termination: TerminationNotRun, CleanupError: CleanupErrorNone}
}

// Rejection precedence is fixed: structural numeric bounds, identity/root
// containment, then a pre-start cancellation. Runtime stream/timeout/process
// failures are observed only after a process has started.
func validatePlanShape(ctx context.Context, plan Plan) error {
	if ctx == nil || len(plan.Request) == 0 || len(plan.Request) > maxRequestBytes || plan.Timeout <= 0 || plan.Timeout > 100*time.Millisecond || plan.MaxStdoutBytes <= 0 || plan.MaxStderrBytes <= 0 || plan.MaxStdoutBytes > maxRequestBytes || plan.MaxStderrBytes > maxRequestBytes || plan.MaxChildren != 1 || !validPlatformTuple(plan.Host) || !validPlatformTuple(plan.Target) || !validDigest(plan.InvocationBindingSHA256) || plan.InvocationBindingSHA256 != InvocationBindingSHA256(plan.Request, plan.Host, plan.Target) {
		return &Error{Failure: Limit}
	}
	if plan.MemoryBytes != 0 {
		return &Error{Failure: Unsupported}
	}
	return nil
}

func validatePlan(ctx context.Context, plan Plan) (rootBindings, error) {
	if err := validatePlanShape(ctx, plan); err != nil {
		return rootBindings{}, err
	}
	if !cleanAbsolute(plan.Artifact) || !cleanAbsolute(plan.Executable) || !cleanAbsolute(plan.RepositoryRoot) || !cleanAbsolute(plan.StagingParent) {
		return rootBindings{}, &Error{Failure: IdentityUnsafe}
	}
	if !validDigest(plan.ExpectedArtifactSHA256) || !validDigest(plan.ExpectedExecutableSHA256) {
		return rootBindings{}, &Error{Failure: DigestMismatch}
	}
	if err := contextFailure(ctx); err != nil {
		return rootBindings{}, err
	}
	roots, err := bindPlanRoots(plan)
	if err != nil {
		var failure *stagingFailure
		if errors.As(err, &failure) {
			return rootBindings{}, err
		}
		return rootBindings{}, &Error{Failure: IdentityUnsafe}
	}
	return roots, nil
}

func stage(plan Plan) (sealedArtifact, func() CleanupError, error) {
	return stageContext(context.Background(), plan)
}

func stageContext(ctx context.Context, plan Plan) (sealedArtifact, func() CleanupError, error) {
	if err := contextFailure(ctx); err != nil {
		return sealedArtifact{}, nil, err
	}
	roots, err := validatePlan(ctx, plan)
	if err != nil {
		return sealedArtifact{}, nil, err
	}
	return stageWithCopyRoots(ctx, plan, roots, copyWithContext)
}

type stageCopier func(context.Context, io.Writer, io.Reader) (int64, error)

func stageWithCopy(ctx context.Context, plan Plan, copy stageCopier) (sealedArtifact, func() CleanupError, error) {
	if err := contextFailure(ctx); err != nil {
		return sealedArtifact{}, nil, err
	}
	roots, err := validatePlan(ctx, plan)
	if err != nil {
		return sealedArtifact{}, nil, err
	}
	return stageWithCopyRoots(ctx, plan, roots, copy)
}

func stageContextRoots(ctx context.Context, plan Plan, roots rootBindings) (sealedArtifact, func() CleanupError, error) {
	return stageWithCopyRoots(ctx, plan, roots, copyWithContext)
}

func stageWithCopyRoots(ctx context.Context, plan Plan, roots rootBindings, copy stageCopier) (sealedArtifact, func() CleanupError, error) {
	if err := contextFailure(ctx); err != nil {
		return stageFailureFrom(err, roots, nil, nil, nil, "")
	}
	artifact, artifactBytes, err := verifyAndOpen(ctx, plan.Artifact, plan.ExpectedArtifactSHA256, roots.repository, copy)
	if err != nil {
		return stageFailureFrom(err, roots, nil, nil, nil, "")
	}
	source, opened, err := openPinnedRegular(ctx, plan.Executable, roots.repository)
	if err != nil {
		return stageFailureFrom(err, roots, nil, artifact, nil, "")
	}
	host, hostErr := ActualHostPlatform()
	candidate, candidateErr := NativeExecutablePlatform(source)
	if hostErr != nil || candidateErr != nil {
		return stageFailureFrom(&Error{Failure: IdentityUnsafe}, roots, source, artifact, nil, "")
	}
	if err := contextFailure(ctx); err != nil {
		return stageFailureFrom(err, roots, source, artifact, nil, "")
	}
	if host != plan.Host || !nativeExecutableMatchesHost(candidate, host) {
		return stageFailureFrom(&Error{Failure: Unsupported}, roots, source, artifact, nil, "")
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return stageFailureFrom(&Error{Failure: Race}, roots, source, artifact, nil, "")
	}
	parent := roots.staging.root
	roots.staging.root = nil
	// Staging is content-addressed and reused (decision 0148): Darwin charges
	// a fresh inode's first exec far more than the 100 ms plan cap, so a
	// verified entry persists under its digest name and is re-verified here
	// and again immediately before launch.
	published := publishedStageName(plan.ExpectedExecutableSHA256)
	_, lookupErr := parent.Lstat(published)
	reuse := lookupErr == nil
	if !reuse && !errors.Is(lookupErr, os.ErrNotExist) {
		return stageFailureFrom(&Error{Failure: Race}, roots, source, artifact, parent, "")
	}
	directory, residue := published, published
	var root *os.Root
	if reuse {
		root, err = openStageRoot(parent, published)
	} else {
		directory, root, err = newStageRoot(ctx, parent)
		residue = directory
	}
	if err != nil {
		return stageFailureFrom(stageFailure(err), roots, source, artifact, parent, residue)
	}
	var executable *os.File
	cleanup := func() CleanupError {
		return cleanupStage(executable, root, source, artifact, parent, residue, roots)
	}
	fail := func(err error) (sealedArtifact, func() CleanupError, error) {
		cleanupStarted := time.Now()
		return sealedArtifact{}, nil, &stagingFailure{cause: err, cleanup: cleanup(), cleanupElapsed: time.Since(cleanupStarted)}
	}
	// keepEntry fails without removing a reused entry: the failure lies in
	// the caller's context or the registry source, never in the entry.
	keepEntry := func(err error) (sealedArtifact, func() CleanupError, error) {
		if reuse {
			residue = ""
		}
		return fail(err)
	}
	stageName := plan.ExpectedExecutableSHA256
	var sink io.Writer = io.Discard
	var destination *os.File
	if !reuse {
		destination, err = root.OpenFile(stageName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o500)
		if err != nil {
			return fail(&Error{Failure: Process})
		}
		sink = destination
	}
	hash := sha256.New()
	stagingBytes, copyErr := copy(ctx, io.MultiWriter(sink, hash), io.LimitReader(source, MaxExecutableBytes+1))
	destinationInfo, destinationStatErr, closeErr := closeStageDestination(destination)
	after, statErr := source.Stat()
	if contextErr := contextFailure(ctx); contextErr != nil {
		return keepEntry(contextErr)
	}
	if copyErr != nil || closeErr != nil || destinationStatErr != nil || statErr != nil || !samePinnedRegular(opened, after) {
		return keepEntry(&Error{Failure: Race})
	}
	if actual := hex.EncodeToString(hash.Sum(nil)); actual != plan.ExpectedExecutableSHA256 {
		return keepEntry(&Error{Failure: DigestMismatch})
	}
	executable, err = root.OpenFile(stageName, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return fail(&Error{Failure: Race})
	}
	staged, err := executable.Stat()
	if reuse {
		destinationInfo = staged
	}
	artifactInfo, artifactStatErr := artifact.Stat()
	destinationPath, destinationPathErr := root.Lstat(stageName)
	if err != nil || artifactStatErr != nil || destinationPathErr != nil || !samePinnedRegular(destinationInfo, staged) || !samePinnedRegular(destinationInfo, destinationPath) || staged.Mode().Perm() != 0o500 {
		return fail(&Error{Failure: Race})
	}
	if err := validateStageDirectory(parent, directory, root); err != nil {
		return fail(err)
	}
	if reuse {
		if err := validatePinnedDescriptor(ctx, executable, staged, stageName); err != nil {
			if contextFailure(ctx) != nil {
				return keepEntry(err)
			}
			return fail(err)
		}
	}
	if !reuse {
		if err := parent.Rename(directory, published); err != nil {
			return fail(&Error{Failure: Race})
		}
		directory = published
	}
	residue = ""
	if err := contextFailure(ctx); err != nil {
		return fail(err)
	}
	return sealedArtifact{executable: executable, artifact: artifact, parent: parent, root: root, directory: directory, stagingInfo: staged, artifactInfo: artifactInfo, workBytes: artifactBytes + stagingBytes, digest: plan.ExpectedExecutableSHA256, artifactDigest: plan.ExpectedArtifactSHA256, launchPath: filepath.Join(roots.staging.path, directory, stageName), host: plan.Host, target: plan.Target}, cleanup, nil
}

// publishedStageName names the one owner-private directory that holds the
// verified staged executable for digest.
func publishedStageName(digest string) string { return "corvint-analyzer-sha256-" + digest }

func closeStageDestination(destination *os.File) (os.FileInfo, error, error) {
	if destination == nil {
		return nil, nil, nil
	}
	info, statErr := destination.Stat()
	return info, statErr, destination.Close()
}

func verifyAndOpen(ctx context.Context, path, expected string, repository rootBinding, copy stageCopier) (*os.File, int64, error) {
	file, opened, err := openPinnedRegular(ctx, path, repository)
	if err != nil {
		return nil, 0, err
	}
	hash := sha256.New()
	copied, copyErr := copy(ctx, hash, io.LimitReader(file, MaxExecutableBytes+1))
	after, statErr := file.Stat()
	if contextErr := contextFailure(ctx); contextErr != nil {
		file.Close()
		return nil, 0, contextErr
	}
	if copyErr != nil || statErr != nil || !samePinnedRegular(opened, after) {
		file.Close()
		return nil, 0, &Error{Failure: Race}
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		file.Close()
		return nil, 0, &Error{Failure: DigestMismatch}
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return nil, 0, &Error{Failure: Race}
	}
	return file, copied, nil
}

func openPinnedRegular(ctx context.Context, path string, repository rootBinding) (*os.File, os.FileInfo, error) {
	if !cleanAbsolute(path) || within(repository, path) {
		return nil, nil, &Error{Failure: IdentityUnsafe}
	}
	file, err := openRelativeNoFollow(string(filepath.Separator), strings.TrimPrefix(path, string(filepath.Separator)))
	if err != nil {
		return nil, nil, &Error{Failure: IdentityUnsafe}
	}
	opened, err := file.Stat()
	if err != nil || !safeRegular(opened) || opened.Size() > MaxExecutableBytes {
		_ = file.Close()
		return nil, nil, &Error{Failure: IdentityUnsafe}
	}
	if err := contextFailure(ctx); err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	return file, opened, nil
}

// openRelativeNoFollow holds each parent directory descriptor while opening
// the next component. It therefore never performs a resolve-then-reopen walk.
func openRelativeNoFollow(repository, relative string) (*os.File, error) {
	root, err := os.OpenRoot(repository)
	if err != nil {
		return nil, err
	}
	defer func() { root.Close() }()
	components := strings.Split(relative, string(filepath.Separator))
	for _, component := range components[:len(components)-1] {
		if component == "" || component == "." || component == ".." {
			return nil, errors.New("unsafe path component")
		}
		before, statErr := root.Lstat(component)
		if statErr != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("unsafe path component")
		}
		next, openErr := root.OpenRoot(component)
		if openErr != nil {
			return nil, openErr
		}
		bound, boundErr := next.Open(".")
		after, afterErr := restatSourceDirectory(root, component)
		if boundErr != nil || afterErr != nil {
			if bound != nil {
				bound.Close()
			}
			next.Close()
			return nil, errors.New("component race")
		}
		boundInfo, boundStatErr := bound.Stat()
		closeErr := bound.Close()
		if boundStatErr != nil || closeErr != nil || !sameOpenedDirectory(before, boundInfo) || !sameOpenedDirectory(before, after) {
			next.Close()
			return nil, errors.New("component race")
		}
		root.Close()
		root = next
	}
	name := components[len(components)-1]
	if name == "" || name == "." || name == ".." {
		return nil, errors.New("unsafe path component")
	}
	before, statErr := root.Lstat(name)
	if statErr != nil || !safeRegular(before) || before.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("unsafe path component")
	}
	file, openErr := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if openErr != nil {
		return nil, openErr
	}
	opened, openedErr := file.Stat()
	after, afterErr := root.Lstat(name)
	if openedErr != nil || afterErr != nil || !samePinnedRegular(before, opened) || !samePinnedRegular(before, after) {
		file.Close()
		return nil, errors.New("component race")
	}
	return file, nil
}

func openNoFollow(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func copyWithContext(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
	buffer := make([]byte, 32<<10)
	var copied int64
	for {
		if err := contextFailure(ctx); err != nil {
			return copied, err
		}
		count, readErr := source.Read(buffer)
		if count > 0 {
			written, writeErr := destination.Write(buffer[:count])
			copied += int64(written)
			if writeErr != nil {
				return copied, writeErr
			}
			if written != count {
				return copied, io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			return copied, nil
		}
		if readErr != nil {
			return copied, readErr
		}
	}
}

func contextFailure(ctx context.Context) error {
	if ctx == nil || ctx.Err() == nil {
		return nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return &Error{Failure: Timeout}
	}
	return &Error{Failure: Cancelled}
}

func newStageRoot(ctx context.Context, parent *os.Root) (string, *os.Root, error) {
	for range 8 {
		if err := contextFailure(ctx); err != nil {
			return "", nil, err
		}
		bytes := make([]byte, 12)
		if err := readRandom(readStageRandom, bytes); err != nil {
			return "", nil, &Error{Failure: Process}
		}
		if err := contextFailure(ctx); err != nil {
			return "", nil, err
		}
		name := "corvint-analyzer-" + hex.EncodeToString(bytes)
		if err := parent.Mkdir(name, 0o700); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", nil, err
		}
		if err := contextFailure(ctx); err != nil {
			return name, nil, err
		}
		root, err := openStageRoot(parent, name)
		if err != nil {
			return name, nil, err
		}
		return name, root, nil
	}
	return "", nil, errors.New("staging collision")
}

type randomReader func([]byte) (int, error)

func (reader randomReader) Read(value []byte) (int, error) { return reader(value) }

func readRandom(read func([]byte) (int, error), value []byte) error {
	_, err := io.ReadFull(randomReader(read), value)
	return err
}

type rootBinding struct {
	root *os.Root
	path string
	info os.FileInfo
}

type rootBindings struct{ repository, staging rootBinding }

func bindPlanRoots(plan Plan) (rootBindings, error) {
	repository, err := bindOwnerRoot(plan.RepositoryRoot, false)
	if err != nil {
		return rootBindings{}, err
	}
	staging, err := bindOwnerRoot(plan.StagingParent, true)
	if err != nil {
		cleanup := cleanupStage(nil, nil, nil, nil, nil, "", rootBindings{repository: repository})
		return rootBindings{}, stagingFailureFor(&Error{Failure: IdentityUnsafe}, cleanup, 0)
	}
	roots := rootBindings{repository: repository, staging: staging}
	if within(repository, staging.path) {
		cleanup := roots.close()
		return rootBindings{}, stagingFailureFor(&Error{Failure: IdentityUnsafe}, cleanup, 0)
	}
	return roots, nil
}

func bindOwnerRoot(path string, private bool) (rootBinding, error) {
	if strings.HasPrefix(path, "/System/Volumes/Data/") || path == "/System/Volumes/Data" {
		return rootBinding{}, errors.New("Darwin Data volume alias")
	}
	info, err := os.Lstat(path)
	if err != nil || !safeDirectory(info) || private && !safePrivateDirectory(info) {
		return rootBinding{}, errors.New("unsafe owner directory")
	}
	canonicalPath, err := filepath.EvalSymlinks(path)
	if err != nil || !cleanAbsolute(canonicalPath) {
		return rootBinding{}, errors.New("unsafe owner directory")
	}
	canonicalInfo, err := os.Lstat(canonicalPath)
	if err != nil || !samePinnedDirectory(info, canonicalInfo) {
		return rootBinding{}, errors.New("root alias")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return rootBinding{}, err
	}
	descriptor, err := root.Open(".")
	if err != nil {
		_ = root.Close()
		return rootBinding{}, err
	}
	bound, statErr := descriptor.Stat()
	closeErr := descriptor.Close()
	pathAfter, pathErr := os.Lstat(path)
	canonicalAfter, canonicalErr := os.Lstat(canonicalPath)
	if statErr != nil || closeErr != nil || pathErr != nil || canonicalErr != nil || !samePinnedDirectory(info, bound) || !samePinnedDirectory(info, pathAfter) || !samePinnedDirectory(info, canonicalAfter) || private && !safePrivateDirectory(bound) {
		_ = root.Close()
		return rootBinding{}, errors.New("root race")
	}
	return rootBinding{root: root, path: canonicalPath, info: info}, nil
}

func (roots rootBindings) close() CleanupError {
	return cleanupStage(nil, nil, nil, nil, nil, "", roots)
}

func stageFailureFrom(cause error, roots rootBindings, source, artifact *os.File, parent *os.Root, directory string) (sealedArtifact, func() CleanupError, error) {
	cleanupStarted := time.Now()
	cleanup := cleanupStage(nil, nil, source, artifact, parent, directory, roots)
	return sealedArtifact{}, nil, stagingFailureFor(cause, cleanup, time.Since(cleanupStarted))
}

func stagingFailureFor(cause error, cleanup CleanupError, elapsed time.Duration) error {
	if cleanup == CleanupErrorNone {
		return cause
	}
	return &stagingFailure{cause: cause, cleanup: cleanup, cleanupElapsed: elapsed}
}

func cleanupStage(executable *os.File, root *os.Root, source, artifact *os.File, parent *os.Root, directory string, roots rootBindings) CleanupError {
	result := CleanupErrorNone
	closeFile := func(file *os.File) {
		if file != nil && closeStageFile(file) != nil && result == CleanupErrorNone {
			result = CleanupErrorClose
		}
	}
	closeRoot := func(value *os.Root) {
		if value != nil && closeStageRoot(value) != nil && result == CleanupErrorNone {
			result = CleanupErrorClose
		}
	}
	closeFile(executable)
	closeRoot(root)
	closeFile(source)
	closeFile(artifact)
	if parent != nil && directory != "" && removeStageDirectory(parent, directory) != nil && result == CleanupErrorNone {
		result = CleanupErrorRemove
	}
	closeRoot(parent)
	closeRoot(roots.repository.root)
	closeRoot(roots.staging.root)
	return result
}

func stageFailure(err error) error {
	var typed *Error
	if errors.As(err, &typed) {
		return err
	}
	return &Error{Failure: Process}
}

func cleanAbsolute(path string) bool { return filepath.IsAbs(path) && filepath.Clean(path) == path }

// within reports whether path names root or a descendant of it, lexically or by
// directory identity. A case-insensitive volume admits differently spelled
// aliases of one directory that the lexical comparison alone does not see.
func within(root rootBinding, path string) bool {
	rel, err := filepath.Rel(root.path, path)
	if err == nil && rel != ".." && !bytes.HasPrefix([]byte(rel), []byte(".."+string(filepath.Separator))) {
		return true
	}
	for directory := path; ; directory = filepath.Dir(directory) {
		if info, err := os.Lstat(directory); err == nil && os.SameFile(root.info, info) {
			return true
		}
		if filepath.Dir(directory) == directory {
			return false
		}
	}
}

func safeDirectory(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && info.IsDir() && info.Mode().Perm()&0o022 == 0 && stat.Uid == uint32(os.Geteuid())
}

func safePrivateDirectory(info os.FileInfo) bool {
	return safeDirectory(info) && info.Mode().Perm()&0o077 == 0 && info.Mode().Perm()&0o700 == 0o700
}

func safeRegular(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 && info.Mode().Perm()&0o022 == 0 && stat.Nlink == 1 && stat.Uid == uint32(os.Geteuid())
}

func samePinnedRegular(want, got os.FileInfo) bool {
	if want == nil || got == nil || !safeRegular(want) || !safeRegular(got) || !os.SameFile(want, got) || want.Size() != got.Size() || want.Mode() != got.Mode() {
		return false
	}
	wantStat, wantOK := want.Sys().(*syscall.Stat_t)
	gotStat, gotOK := got.Sys().(*syscall.Stat_t)
	return wantOK && gotOK && wantStat.Dev == gotStat.Dev && wantStat.Ino == gotStat.Ino && wantStat.Nlink == gotStat.Nlink && wantStat.Uid == gotStat.Uid
}

func samePinnedDirectory(want, got os.FileInfo) bool {
	if want == nil || got == nil || !safeDirectory(want) || !safeDirectory(got) || !os.SameFile(want, got) || want.Mode() != got.Mode() {
		return false
	}
	wantStat, wantOK := want.Sys().(*syscall.Stat_t)
	gotStat, gotOK := got.Sys().(*syscall.Stat_t)
	return wantOK && gotOK && wantStat.Dev == gotStat.Dev && wantStat.Ino == gotStat.Ino && wantStat.Uid == gotStat.Uid
}

func sameOpenedDirectory(want, got os.FileInfo) bool {
	if want == nil || got == nil || !want.IsDir() || !got.IsDir() || want.Mode()&os.ModeSymlink != 0 || got.Mode()&os.ModeSymlink != 0 || !os.SameFile(want, got) || want.Mode() != got.Mode() {
		return false
	}
	wantStat, wantOK := want.Sys().(*syscall.Stat_t)
	gotStat, gotOK := got.Sys().(*syscall.Stat_t)
	return wantOK && gotOK && wantStat.Dev == gotStat.Dev && wantStat.Ino == gotStat.Ino
}
