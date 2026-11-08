// SPDX-License-Identifier: Apache-2.0
package runnerprotocol

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

type Limits struct {
	Message, Output, Stderr int64
	Hello, Request          time.Duration
}

func (l Limits) normalized() (Limits, error) {
	if l.Message == 0 {
		l.Message = 1 << 20
	}
	if l.Output == 0 {
		l.Output = 256 << 20
	}
	if l.Stderr == 0 {
		l.Stderr = 64 << 10
	}
	if l.Hello == 0 {
		l.Hello = 5 * time.Second
	}
	if l.Request == 0 {
		l.Request = 60 * time.Second
	}
	if l.Message < 1 || l.Message > 8<<20 || l.Output < 1 || l.Output > 1<<30 || l.Stderr < 0 || l.Stderr > 64<<10 || l.Hello < 0 || l.Hello > 5*time.Second || l.Request < 0 || l.Request > 600*time.Second {
		return l, fmt.Errorf("limits-invalid")
	}
	return l, nil
}

type Hello struct {
	Protocol string              `json:"protocol"`
	Plugin   string              `json:"plugin"`
	Version  string              `json:"version"`
	Runners  []string            `json:"runners"`
	Pieces   map[string][]string `json:"pieces"`
	Features []string            `json:"features"`
}
type Options struct {
	Repository, RunDir, CoreVersion string
	Limits                          Limits
}

// Session is sequential and owns one process for all runners named by its pin.
// Close is mandatory. No native evidence is released before Close succeeds.
type Session struct {
	mu            sync.Mutex
	options       Options
	pin           Pin
	hello         Hello
	limits        Limits
	command       *exec.Cmd
	input, output *os.File
	reader        *bufio.Reader
	wait          chan error
	cancel        context.CancelFunc
	counter       int
	bytes         atomic.Int64
	stderr        diagnostic
	failure       string
	closed        bool
	before        string
	staged        map[string][]registry.NativeEvidence
}
type diagnostic struct {
	mu      sync.Mutex
	data    []byte
	session *Session
}

func (d *diagnostic) Write(b []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(b)
	remaining := int(d.session.limits.Stderr) - len(d.data)
	if remaining > len(b) {
		remaining = len(b)
	}
	if remaining > 0 {
		d.data = append(d.data, b[:remaining]...)
	}
	if d.session.bytes.Add(int64(n)) > d.session.limits.Output {
		d.session.cancel()
	}
	return n, nil
}
func (s *Session) Diagnostics() string {
	s.stderr.mu.Lock()
	defer s.stderr.mu.Unlock()
	return string(s.stderr.data)
}
func Open(ctx context.Context, resolved Resolved, options Options) (*Session, error) {
	if options.Repository == "" || options.RunDir == "" {
		return nil, fmt.Errorf("session-paths-required")
	}
	limits, err := options.Limits.normalized()
	if err != nil {
		return nil, err
	}
	if resolved.path == "" || resolved.pin.Protocol != Protocol {
		return nil, fmt.Errorf("missing-executable")
	}
	options.Repository, err = filepath.Abs(options.Repository)
	if err != nil {
		return nil, err
	}
	options.Repository, err = filepath.EvalSymlinks(options.Repository)
	if err != nil {
		return nil, err
	}
	options.RunDir, err = filepath.Abs(options.RunDir)
	if err != nil {
		return nil, err
	}
	if options.CoreVersion == "" {
		return nil, fmt.Errorf("core-version-missing")
	}
	resolvedRun, resolveErr := filepath.EvalSymlinks(options.RunDir)
	if resolveErr != nil || resolvedRun != options.RunDir {
		return nil, fmt.Errorf("run-directory-unavailable")
	}
	runInfo, statErr := os.Stat(options.RunDir)
	if statErr != nil || !runInfo.IsDir() {
		return nil, fmt.Errorf("run-directory-unavailable")
	}
	if within(options.Repository, options.RunDir) {
		return nil, fmt.Errorf("run-directory-inside-repository")
	}
	before, err := repositorySnapshot(ctx, options.Repository)
	if err != nil {
		return nil, fmt.Errorf("repository-status-unavailable:%w", err)
	}
	executable, err := executableFile(resolved)
	if err != nil {
		return nil, err
	}
	defer executable.Close()
	child, cancel := context.WithCancel(ctx)
	s := &Session{options: options, pin: resolved.Pin(), limits: limits, cancel: cancel, before: before, staged: map[string][]registry.NativeEvidence{}, wait: make(chan error, 1)}
	s.stderr.session = s
	cmd := exec.CommandContext(child, executablePath())
	cmd.ExtraFiles = []*os.File{executable}
	cmd.Dir = options.Repository
	// A plugin receives no credentials or ambient executable search path.
	cmd.Env = []string{"LANG=C.UTF-8", "LC_ALL=C.UTF-8"}
	cmd.Stderr = &s.stderr
	cmd.WaitDelay = 2 * time.Second
	groupreap.Contain(cmd)
	s.command = cmd
	input, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	s.input = input.(*os.File)
	// Own stdout outside exec.Cmd.Wait so the last frame remains readable.
	read, write, err := os.Pipe()
	if err != nil {
		s.input.Close()
		cancel()
		return nil, err
	}
	s.output = read
	cmd.Stdout = write
	s.reader = bufio.NewReaderSize(read, int(limits.Message)+1)
	if err = cmd.Start(); err != nil {
		write.Close()
		read.Close()
		s.input.Close()
		cancel()
		return nil, fmt.Errorf("launch-failure:%w", err)
	}
	write.Close()
	go func() { s.wait <- groupreap.Wait(cmd) }()
	err = s.call(ctx, "hello", map[string]any{"protocols": []string{Protocol}, "core": options.CoreVersion}, &s.hello)
	if err == nil {
		err = s.validateHello()
	}
	if err != nil {
		s.fail(err)
		s.Close(context.Background())
		return nil, err
	}
	return s, nil
}
func (s *Session) validateHello() error {
	h := s.hello
	if h.Protocol != Protocol {
		return fmt.Errorf("version-mismatch")
	}
	if h.Plugin != s.pin.Name || h.Version == "" || !sameStrings(h.Runners, s.pin.Runners) {
		return fmt.Errorf("undeclared-runner")
	}
	if len(h.Pieces) != len(h.Runners) || !unique(h.Runners) || !unique(h.Features) {
		return fmt.Errorf("schema-violation:hello")
	}
	for _, r := range h.Runners {
		pieces, ok := h.Pieces[r]
		if !ok || !unique(pieces) {
			return fmt.Errorf("schema-violation:pieces")
		}
	}
	return nil
}
func unique(xs []string) bool {
	seen := map[string]bool{}
	for _, x := range xs {
		if x == "" || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func (s *Session) fail(err error) error {
	if s.failure == "" {
		s.failure = err.Error()
	}
	s.staged = nil
	s.cancel()
	return fmt.Errorf("%s", s.failure)
}
func (s *Session) FullReason() string { s.mu.Lock(); defer s.mu.Unlock(); return s.failure }
func (s *Session) Supports(feature string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failure == "" && contains(s.hello.Features, feature)
}

// RequireFeature guards additive opt-in use without adding undeclared members
// to the version-1 envelope. Unknown features cannot change the wire schema.
func (s *Session) RequireFeature(feature string) error {
	if !s.Supports(feature) {
		return fmt.Errorf("feature-not-negotiated:%s", feature)
	}
	return nil
}
func (s *Session) call(ctx context.Context, method string, params, out any) error {
	if s.failure != "" {
		return errors.New(s.failure)
	}
	if s.closed {
		return fmt.Errorf("session-closed")
	}
	payload, err := json.Marshal(params)
	if err != nil {
		return s.fail(err)
	}
	var checked any
	if err = validatePayload(method, payload, false, &checked); err != nil {
		return s.fail(err)
	}
	s.counter++
	id := strconv.Itoa(s.counter)
	frame, err := json.Marshal(request{ID: id, Method: method, Params: payload})
	if err != nil {
		return s.fail(err)
	}
	if int64(len(frame)) > s.limits.Message {
		return s.fail(fmt.Errorf("oversize-message"))
	}
	duration := s.limits.Request
	if method == "hello" {
		duration = s.limits.Hello
	}
	if method == "shutdown" && duration > 2*time.Second {
		duration = 2 * time.Second
	}
	deadline := time.Now().Add(duration)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	stop := context.AfterFunc(ctx, s.cancel)
	defer stop()
	if err = ctx.Err(); err != nil {
		return s.fail(fmt.Errorf("timeout:%w", err))
	}
	if err = s.input.SetWriteDeadline(deadline); err != nil {
		return s.fail(fmt.Errorf("unsupported-deadline:%w", err))
	}
	if err = s.output.SetReadDeadline(deadline); err != nil {
		return s.fail(fmt.Errorf("unsupported-deadline:%w", err))
	}
	if _, err = s.input.Write(append(frame, '\n')); err != nil {
		return s.ioFailure(err)
	}
	line, err := s.reader.ReadSlice('\n')
	s.bytes.Add(int64(len(line)))
	if s.bytes.Load() > s.limits.Output {
		return s.fail(fmt.Errorf("session-output-cap"))
	}
	if len(line) > int(s.limits.Message)+1 || errors.Is(err, bufio.ErrBufferFull) {
		return s.fail(fmt.Errorf("oversize-message"))
	}
	if err != nil {
		return s.ioFailure(err)
	}
	if err = decodeResponse(line[:len(line)-1], id, method, out); err != nil {
		return s.fail(err)
	}
	return nil
}
func (s *Session) ioFailure(err error) error {
	if s.bytes.Load() > s.limits.Output {
		return s.fail(fmt.Errorf("session-output-cap"))
	}
	if errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return s.fail(fmt.Errorf("timeout"))
	}
	return s.fail(fmt.Errorf("crash:%w", err))
}
func (s *Session) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		if s.failure != "" {
			return errors.New(s.failure)
		}
		return nil
	}
	timer := time.AfterFunc(2*time.Second, s.cancel)
	if s.failure == "" {
		var result map[string]any
		_ = s.call(ctx, "shutdown", map[string]any{}, &result)
	}
	s.input.Close()
	// A process which ignores EOF or shutdown is killed, with its group, at 2s.
	err := <-s.wait
	timer.Stop()
	s.cancel()
	// Refuse unsolicited/trailing frames, including buffered output after shutdown.
	s.output.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	trailing, readErr := s.reader.ReadByte()
	_ = trailing
	if readErr == nil {
		s.fail(fmt.Errorf("protocol-error:trailing-output"))
	} else if !errors.Is(readErr, io.EOF) {
		s.fail(fmt.Errorf("protocol-error:output-not-closed"))
	}
	s.output.Close()
	// The stderr sink cancels the process at the cap, so the cap outranks the
	// resulting wait error.
	if s.bytes.Load() > s.limits.Output {
		s.fail(fmt.Errorf("session-output-cap"))
	}
	if err != nil && s.failure == "" {
		s.fail(fmt.Errorf("crash:%w", err))
	}
	checkCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	after, snapshotErr := repositorySnapshot(checkCtx, s.options.Repository)
	if snapshotErr != nil {
		s.fail(fmt.Errorf("repository-status-unavailable:%w", snapshotErr))
	} else if after != s.before {
		s.fail(fmt.Errorf("repository-mutation"))
	}
	s.closed = true
	if s.failure != "" {
		return errors.New(s.failure)
	}
	return nil
}
func (s *Session) Plugin(runner string) (registry.RunnerPlugin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !contains(s.hello.Runners, runner) {
		return nil, s.fail(fmt.Errorf("undeclared-runner"))
	}
	return &adapter{session: s, runner: runner}, nil
}
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !filepath.IsAbs(rel) && !hasParent(rel)
}
func hasParent(p string) bool { return len(p) >= 3 && p[:3] == ".."+string(filepath.Separator) }
func (s *Session) allowedPath(path string) bool {
	if path == "" {
		return false
	}
	if !filepath.IsAbs(path) {
		if filepath.Clean(path) != path || hasParent(path) || path == ".." {
			return false
		}
		path = filepath.Join(s.options.Repository, path)
	} else if !within(s.options.RunDir, path) {
		return false
	}
	// Resolve existing paths and ancestors, refusing symlink escapes even when
	// an output leaf has not yet been created by the core.
	for p := path; ; p = filepath.Dir(p) {
		if _, err := os.Lstat(p); err == nil {
			resolved, e := filepath.EvalSymlinks(p)
			return e == nil && (within(s.options.Repository, resolved) || within(s.options.RunDir, resolved))
		}
		if filepath.Dir(p) == p {
			return false
		}
	}
}

var _ registry.RunnerPlugin = (*adapter)(nil)

type adapter struct {
	session *Session
	runner  string
}

func (a *adapter) Pieces() testrunner.RunnerPieces {
	s := a.session
	s.mu.Lock()
	defer s.mu.Unlock()
	p := testrunner.AbsentRunnerPieces(a.runner)
	if s.failure != "" {
		p.Inventory.Reason = s.failure
		return p
	}
	names := s.hello.Pieces[a.runner]
	if contains(names, "inventory") {
		p.Inventory = testrunner.RunnerPiece{Present: true}
	}
	if contains(names, "test-evidence") {
		p.TestEvidence = testrunner.RunnerPiece{Present: true}
	}
	if contains(names, "qualification") {
		p.Qualification = testrunner.RunnerPiece{Present: true}
	}
	return p
}
