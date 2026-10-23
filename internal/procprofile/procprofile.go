// Package procprofile brackets one Corvint process with the operator profile knobs
// CORVINT_CPUPROFILE and CORVINT_MEMPROFILE (performance-profiling-and-read-budget-v0.md,
// PRB-V0-001..019). It is applied once at each binary's entry point, so every command and
// server it dispatches inherits the knobs. The files are local diagnostics: the knobs never
// change what a command reads, returns, or mutates.
package procprofile

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

const (
	// CPUSuffix and MemSuffix are the runtimeenv suffixes of the two knobs.
	CPUSuffix = "CPUPROFILE"
	MemSuffix = "MEMPROFILE"
	// PIDPlaceholder in a knob value is replaced by the process ID, so a child process that
	// inherits the environment writes its own file instead of truncating its parent's.
	PIDPlaceholder = "{pid}"
)

// Knobs names the environment variables, in the order help and the catalog list them.
var Knobs = []string{"CORVINT_" + CPUSuffix, "CORVINT_" + MemSuffix}

// Error is a profile knob failure; Code is profile-invalid, profile-unavailable or
// profile-write-failed.
type Error struct {
	Code, Message string
	Err           error
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }
func (e *Error) Unwrap() error { return e.Err }

func fail(code, format string, args ...any) error {
	wrapped := fmt.Errorf(format, args...)
	return &Error{Code: code, Message: wrapped.Error(), Err: errors.Unwrap(wrapped)}
}

// Session is one process's open profiles. Stop is idempotent and safe to call from the
// signal watcher and the normal exit path at once: the second caller waits for the first.
type Session struct {
	once     sync.Once
	done     chan struct{}
	cpu, mem *os.File
	cpuOut   *recordingWriter
	err      error
}

// recordingWriter is the CPU profiler's destination. runtime/pprof drops its writer's errors,
// so the first one is kept here for Stop to report (PRB-V0-007). While Start holds mu the
// profiler cannot write; a refused start sets discard so the profiler's final flush never
// reaches the file it is leaving intact.
type recordingWriter struct {
	mu      sync.Mutex
	w       io.Writer
	discard bool
	err     error
}

func (r *recordingWriter) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.discard {
		return len(p), nil
	}
	if r.err != nil {
		return 0, r.err
	}
	n, err := r.w.Write(p)
	if err == nil && n < len(p) {
		err = io.ErrShortWrite
	}
	r.err = err
	return n, err
}

func (r *recordingWriter) failure() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

// Test hooks: truncateFile is (*os.File).Truncate and startCPUProfile is pprof.StartCPUProfile.
var (
	truncateFile    = func(f *os.File, size int64) error { return f.Truncate(size) }
	startCPUProfile = pprof.StartCPUProfile
)

// Start opens the files the knobs name and starts the CPU profile. An unset or empty knob is
// off. A destination inside a Git worktree or Git metadata is refused before either file is
// opened, so profiling never creates or truncates repository content. Both files are opened
// before anything runs, so an unwritable path refuses up front instead of losing the profile
// at exit. Every fallible startup step (opening, the same-file check, starting the profiler,
// and a size-preserving truncation probe) precedes the first truncation, so a refusal leaves
// an existing file intact and removes a file it created.
func Start(lookup runtimeenv.Lookup) (*Session, error) {
	cpuPath, err := knobPath(lookup, CPUSuffix)
	if err != nil {
		return nil, err
	}
	memPath, err := knobPath(lookup, MemSuffix)
	if err != nil {
		return nil, err
	}
	if cpuPath != "" && memPath != "" {
		cpuAbs, cpuErr := filepath.Abs(cpuPath)
		memAbs, memErr := filepath.Abs(memPath)
		if cpuErr == nil && memErr == nil && cpuAbs == memAbs {
			return nil, fail("profile-invalid", "CORVINT_CPUPROFILE and CORVINT_MEMPROFILE name the same file %q", cpuPath)
		}
	}
	for _, knob := range []struct{ name, path string }{{"CORVINT_MEMPROFILE", memPath}, {"CORVINT_CPUPROFILE", cpuPath}} {
		if knob.path != "" {
			if err := checkOutsideRepository(knob.name, knob.path); err != nil {
				return nil, err
			}
		}
	}
	s := &Session{done: make(chan struct{})}
	var created []string
	refuse := func(err error) (*Session, error) {
		s.closeFiles()
		for _, path := range created {
			_ = os.Remove(path)
		}
		return nil, err
	}
	open := func(path, knob string) (*os.File, error) {
		_, statErr := os.Stat(path)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o666)
		if err != nil {
			return nil, fail("profile-unavailable", "%s: %w", knob, err)
		}
		if errors.Is(statErr, os.ErrNotExist) {
			created = append(created, path)
		}
		return file, nil
	}
	if memPath != "" {
		if s.mem, err = open(memPath, "CORVINT_MEMPROFILE"); err != nil {
			return refuse(err)
		}
	}
	if cpuPath != "" {
		if s.cpu, err = open(cpuPath, "CORVINT_CPUPROFILE"); err != nil {
			return refuse(err)
		}
	}
	if s.cpu != nil && s.mem != nil {
		cpuInfo, cpuErr := s.cpu.Stat()
		memInfo, memErr := s.mem.Stat()
		if cpuErr == nil && memErr == nil && os.SameFile(cpuInfo, memInfo) {
			return refuse(fail("profile-invalid", "CORVINT_CPUPROFILE and CORVINT_MEMPROFILE name the same file %q", cpuPath))
		}
	}
	if s.cpu != nil {
		// Held until truncation ends: the profiler cannot write into a file not yet truncated.
		s.cpuOut = &recordingWriter{w: s.cpu}
		s.cpuOut.mu.Lock()
		if err := startCPUProfile(s.cpuOut); err != nil {
			s.cpuOut.mu.Unlock()
			return refuse(fail("profile-unavailable", "CORVINT_CPUPROFILE: %w", err))
		}
	}
	err = s.truncate()
	if s.cpuOut != nil {
		s.cpuOut.discard = err != nil
		s.cpuOut.mu.Unlock()
		if err != nil {
			pprof.StopCPUProfile()
		}
	}
	if err != nil {
		return refuse(err)
	}
	return s, nil
}

// truncate empties the open regular files. Each is first truncated to its own size, a
// non-destructive probe of the same permission, so a file that cannot be truncated refuses
// before any file loses content. A device or pipe (such as /dev/null) has nothing to truncate.
func (s *Session) truncate() error {
	var regular []*os.File
	for _, file := range []struct {
		f    *os.File
		knob string
	}{{s.mem, "CORVINT_MEMPROFILE"}, {s.cpu, "CORVINT_CPUPROFILE"}} {
		if file.f == nil {
			continue
		}
		info, err := file.f.Stat()
		if err != nil {
			return fail("profile-unavailable", "%s: %w", file.knob, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if err := truncateFile(file.f, info.Size()); err != nil {
			return fail("profile-unavailable", "%s: %w", file.knob, err)
		}
		regular = append(regular, file.f)
	}
	for _, f := range regular {
		if err := truncateFile(f, 0); err != nil {
			return fail("profile-unavailable", "%s: %w", f.Name(), err)
		}
	}
	return nil
}

// checkOutsideRepository refuses a destination inside a Git worktree or Git metadata
// (PRB-V0-013): a profile written there would create or truncate repository content before a
// read runs and could turn a clean read into a refusal. The destination is resolved through
// symbolic links, including a dangling final link that opening would follow, and every
// ancestor of the resolved file is checked.
func checkOutsideRepository(knob, path string) error {
	resolved, err := resolveDestination(path)
	if err != nil {
		return fail("profile-unavailable", "%s: %w", knob, err)
	}
	for dir := filepath.Dir(resolved); ; {
		if filepath.Base(dir) == ".git" || isGitDir(dir) {
			return fail("profile-invalid", "%s %q is inside Git metadata %q; choose a path outside every repository", knob, path, dir)
		}
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return fail("profile-invalid", "%s %q is inside the Git worktree %q; choose a path outside every repository", knob, path, dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}

// isGitDir reports whether dir has a Git directory's layout (a bare or separated Git dir).
func isGitDir(dir string) bool {
	head, err := os.Lstat(filepath.Join(dir, "HEAD"))
	if err != nil || !head.Mode().IsRegular() {
		return false
	}
	for _, sub := range []string{"objects", "refs"} {
		if info, err := os.Stat(filepath.Join(dir, sub)); err != nil || !info.IsDir() {
			return false
		}
	}
	return true
}

// maxDestinationLinks bounds link resolution, as the kernel's own loop limit does.
const maxDestinationLinks = 40

// resolveDestination is the absolute, symlink-free path that opening path would write.
func resolveDestination(path string) (string, error) {
	current, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	for range maxDestinationLinks {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			return resolved, nil
		}
		info, err := os.Lstat(current)
		switch {
		case err == nil && info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(current)
			if err != nil {
				return "", err
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(current), target)
			}
			current = target
			continue
		case err != nil && !errors.Is(err, os.ErrNotExist):
			return "", err
		}
		// A file not yet created: its directory must resolve.
		dir, err := filepath.EvalSymlinks(filepath.Dir(current))
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, filepath.Base(current)), nil
	}
	return "", fmt.Errorf("%s: too many symbolic links", path)
}

func knobPath(lookup runtimeenv.Lookup, suffix string) (string, error) {
	value := runtimeenv.Resolve(lookup, suffix)
	if value == "" {
		return "", nil
	}
	if strings.ContainsRune(value, 0) {
		return "", fail("profile-invalid", "CORVINT_%s contains a NUL byte", suffix)
	}
	return strings.ReplaceAll(value, PIDPlaceholder, strconv.Itoa(os.Getpid())), nil
}

// Active reports whether either knob opened a file.
func (s *Session) Active() bool { return s != nil && (s.cpu != nil || s.mem != nil) }

// Stop flushes the CPU profile, writes the heap profile after a collection, and closes both
// files. Only the first call writes; every call returns the first call's error.
func (s *Session) Stop() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		defer close(s.done)
		var errs []error
		if s.cpu != nil {
			pprof.StopCPUProfile() // returns once the profiler has written its last byte
			if err := s.cpuOut.failure(); err != nil {
				errs = append(errs, fail("profile-write-failed", "CORVINT_CPUPROFILE: %w", err))
			}
			if err := s.cpu.Close(); err != nil {
				errs = append(errs, fail("profile-write-failed", "CORVINT_CPUPROFILE: %w", err))
			}
		}
		if s.mem != nil {
			runtime.GC()
			if err := pprof.Lookup("heap").WriteTo(s.mem, 0); err != nil {
				errs = append(errs, fail("profile-write-failed", "CORVINT_MEMPROFILE: %w", err))
			}
			if err := s.mem.Close(); err != nil {
				errs = append(errs, fail("profile-write-failed", "CORVINT_MEMPROFILE: %w", err))
			}
		}
		s.err = errors.Join(errs...)
	})
	return s.err
}

func (s *Session) closeFiles() {
	for _, f := range []*os.File{s.cpu, s.mem} {
		if f != nil {
			_ = f.Close()
		}
	}
}

// StopOnDone writes the profiles as soon as ctx ends, which is how a SIGINT or SIGTERM caught
// by signal.NotifyContext reaches the files even if the command keeps running afterwards. The
// watcher exits when Stop runs first.
func (s *Session) StopOnDone(ctx context.Context) {
	if !s.Active() {
		return
	}
	go func() {
		select {
		case <-ctx.Done():
			_ = s.Stop() // Run's exit path reports the same first-call error.
		case <-s.done:
		}
	}()
}

// Run brackets one process body: it starts the session, flushes it when signalled (ctx done),
// runs body, and stops the session before returning body's status. A knob that cannot be
// honoured is reported and refuses with status 2 before body runs; a write failure at exit is
// reported and keeps body's status, because the diagnostic never changes the command's result.
func Run(ctx context.Context, lookup runtimeenv.Lookup, report func(error), body func() int) int {
	session, err := Start(lookup)
	if err != nil {
		report(err)
		return 2
	}
	session.StopOnDone(ctx)
	// Deferred, so a panicking body still leaves complete profiles behind.
	defer func() {
		if err := session.Stop(); err != nil {
			report(err)
		}
	}()
	return body()
}

// Reporter writes one "<name>: <error>" line to w.
func Reporter(w io.Writer, name string) func(error) {
	return func(err error) { _, _ = fmt.Fprintf(w, "%s: %v\n", name, err) }
}

// maxVerifiedBytes bounds Verify's decompressed read.
const maxVerifiedBytes = 256 << 20

// Verify reports whether path holds a non-empty gzip-compressed pprof profile, as both knobs
// write; it reads at most 256 MiB of decompressed bytes.
func Verify(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("%s is not a gzip pprof profile: %w", path, err)
	}
	n, err := io.Copy(io.Discard, io.LimitReader(reader, maxVerifiedBytes))
	if err != nil {
		return fmt.Errorf("%s is truncated: %w", path, err)
	}
	if n == 0 {
		return fmt.Errorf("%s is an empty profile", path)
	}
	return nil
}
