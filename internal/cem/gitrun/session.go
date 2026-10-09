package gitrun

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/groupreap"
)

// sessionHeaderLimit bounds one `git cat-file --batch` header line; a real
// `<oid> <type> <size>` header is under 100 bytes.
const sessionHeaderLimit = 512

// Session is one caller-scoped `git cat-file --batch` co-process answering
// requests in order. Its child starts on the first Read and ends at Close, so
// no process outlives the pass that owns the session. It charges no budget:
// the caller reserves one operation per request. A request the session cannot
// answer as one admitted record returns ok=false with the child already
// reaped; the caller then replays its original one-shot invocation, which
// alone classifies that failure. Only the per-operation timeout and
// cancellation are classified here, with Run's codes and messages.
type Session struct {
	mu      sync.Mutex
	closed  bool
	binary  string
	dir     string
	env     []string
	args    []string
	command *exec.Cmd
	stdin   *os.File
	stdout  *os.File
	reader  *bufio.Reader
	stderr  *limitedBuffer
	overrun chan struct{}
	waited  chan error
	// budget and owner are set only for a session of a stable budget.
	budget *Budget
	owner  *groupreap.Owner
}

type sessionReply struct {
	header string
	body   []byte
	ok     bool
}

// Read writes request as one batch line and returns the record's header line
// and body when admit accepts the header line, its fields and the body size. A request
// holding LF or NUL is never written, so the stream cannot desynchronize.
func (s *Session) Read(ctx context.Context, perOp time.Duration, options Options, args []string, request string, admit func(header string, fields []string, size int) bool) (string, []byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || strings.ContainsAny(request, "\n\x00") || !s.start(options, args) {
		return "", nil, false, nil
	}
	if ctx.Err() != nil {
		return "", nil, false, cemcode.New(cemcode.GitCancelled, "Git operation cancelled")
	}
	done := make(chan sessionReply, 1)
	stdin, reader := s.stdin, s.reader
	go func() { done <- exchange(stdin, reader, request, admit) }()
	timer := time.NewTimer(perOp)
	defer timer.Stop()
	select {
	case reply := <-done:
		if _, exceeded := s.stderr.snapshot(); reply.ok && !exceeded {
			return reply.header, reply.body, true, nil
		}
		s.stop(nil)
		return "", nil, false, nil
	case <-s.overrun:
		s.stop(done)
		return "", nil, false, nil
	case <-ctx.Done():
		s.stop(done)
		return "", nil, false, cemcode.New(cemcode.GitCancelled, "Git operation cancelled")
	case <-timer.C:
		s.stop(done)
		return "", nil, false, cemcode.New(cemcode.GitTimeout, "Git operation timed out")
	}
}

// start keeps a running child only when it was started with the same argv,
// directory and environment; any other request restarts it.
func (s *Session) start(options Options, args []string) bool {
	binary := options.Binary
	if binary == "" {
		binary = defaultBinary()
	}
	env := options.Env
	if env == nil {
		env = []string{}
	}
	if s.command != nil && (s.binary != binary || s.dir != options.Dir || !slices.Equal(s.env, env) || !slices.Equal(s.args, args)) {
		s.stop(nil)
	}
	if s.command != nil {
		return true
	}
	stdinRead, stdinWrite, err := os.Pipe()
	if err != nil {
		return false
	}
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		stdinRead.Close()
		stdinWrite.Close()
		return false
	}
	stderrLimit := options.StderrLimit
	if stderrLimit == 0 {
		stderrLimit = DefaultStderrLimit
	}
	overrun := make(chan struct{})
	var overrunOnce sync.Once
	command := exec.Command(binary, args...)
	command.Dir, command.Env = options.Dir, env
	command.Stdin, command.Stdout = stdinRead, stdoutWrite
	s.stderr = newLimitedBuffer(stderrLimit, 0, func() { overrunOnce.Do(func() { close(overrun) }) })
	command.Stderr = s.stderr
	command.WaitDelay = pipeDrainDelay
	containChild(command)
	err = groupreap.StartLive(command)
	stdinRead.Close()
	stdoutWrite.Close()
	if err != nil {
		stdinWrite.Close()
		stdoutRead.Close()
		return false
	}
	waited := make(chan error, 1)
	go func() { waited <- groupreap.Wait(command) }()
	s.binary, s.dir, s.env, s.args = binary, options.Dir, slices.Clone(env), slices.Clone(args)
	s.command, s.stdin, s.stdout, s.overrun, s.waited = command, stdinWrite, stdoutRead, overrun, waited
	s.reader = bufio.NewReaderSize(stdoutRead, sessionHeaderLimit)
	return true
}

// exchange performs one request/record round trip. Only a body-bearing
// `<name> <type> <size>` header can be admitted; `missing`, `ambiguous` and
// every other bodiless answer is not.
func exchange(stdin io.Writer, reader *bufio.Reader, request string, admit func(string, []string, int) bool) sessionReply {
	if _, err := io.WriteString(stdin, request+"\n"); err != nil {
		return sessionReply{}
	}
	line, err := reader.ReadSlice('\n')
	if err != nil {
		return sessionReply{}
	}
	header := string(line[:len(line)-1])
	fields := strings.Fields(header)
	if len(fields) != 3 {
		return sessionReply{}
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil || size < 0 || !admit(header, fields, size) {
		return sessionReply{}
	}
	body := make([]byte, size+1)
	if _, err := io.ReadFull(reader, body); err != nil || body[size] != '\n' {
		return sessionReply{}
	}
	return sessionReply{header: header, body: body[:size], ok: true}
}

// stop kills the child's group before reaping it, closes both pipes so a
// pending exchange fails even when a descendant that left the group still
// holds their other ends, and waits for that exchange.
func (s *Session) stop(pending <-chan sessionReply) {
	if s.command == nil {
		return
	}
	killGroupThenReap(s.command, s.waited)
	s.stdin.Close()
	s.stdout.Close()
	if pending != nil {
		<-pending
	}
	s.release()
}

func (s *Session) release() {
	s.stdin.Close()
	s.stdout.Close()
	s.command, s.stdin, s.stdout, s.reader, s.waited = nil, nil, nil, nil, nil
}

// Close ends the session: the child sees EOF on stdin and exits; a child that
// does not exit within DefaultPerOpTimeout is killed with its group.
func (s *Session) Close() {
	if s.budget != nil {
		s.CloseContext(context.Background())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.command == nil {
		return
	}
	s.stdin.Close()
	timer := time.NewTimer(DefaultPerOpTimeout)
	defer timer.Stop()
	select {
	case <-s.waited:
	case <-timer.C:
		killGroupThenReap(s.command, s.waited)
	}
	s.release()
}

// NewSession returns a session whose child is owned by a groupreap.Owner and
// bounded by this stable budget's one clock.
func (b *Budget) NewSession() *Session { return &Session{budget: b} }

// ReadReserved is Read for a stable session. The request runs under the
// absolute deadline of its reservation. ok=false with a nil error means the
// owned child was retired with observed cleanup and the caller replays the
// same reservation as a one-shot child; a failed cleanup is an error.
func (s *Session) ReadReserved(ctx context.Context, reservation Reservation, options Options, args []string, request string, admit func(header string, fields []string, size int) bool) (string, []byte, bool, error) {
	st := s.budget.stable
	event := Event{Ordinal: reservation.Ordinal, Session: true}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || strings.ContainsAny(request, "\n\x00") {
		return "", nil, false, nil
	}
	if st.terminal(ctx) {
		return "", nil, false, cemcode.New(cemcode.GitCancelled, "Git operation cancelled")
	}
	options.Binary, args = st.rewrite(reservation.Ordinal, false, options.Binary, args)
	started, err := s.startOwned(ctx, options, args, event)
	if err != nil || !started {
		return "", nil, false, err
	}
	event.Name = "op-start"
	st.event(event)
	done := make(chan sessionReply, 1)
	stdin, reader := s.stdin, s.reader
	go func() { done <- exchange(stdin, reader, request, admit) }()
	expired := func() bool { return !st.now().Before(reservation.Deadline) }
	opExpired, stopWatch := st.until(expired)
	defer stopWatch()
	var outer, timedOut bool
	var pending <-chan sessionReply = done
	select {
	case reply := <-done:
		if _, exceeded := s.stderr.snapshot(); reply.ok && !exceeded {
			return reply.header, reply.body, true, nil
		}
		pending = nil
		s.owner.Stop()
	case <-s.overrun:
		s.owner.Stop()
	case <-ctx.Done():
		outer, timedOut = st.retire(ctx, s.owner, event, expired)
	case <-opExpired:
		outer, timedOut = st.retire(ctx, s.owner, event, expired)
	}
	if err := s.finishOwned(ctx, event, pending); err != nil {
		return "", nil, false, err
	}
	if outer {
		return "", nil, false, cemcode.New(cemcode.GitCancelled, "Git operation cancelled")
	}
	if timedOut {
		return "", nil, false, cemcode.New(cemcode.GitTimeout, "Git operation timed out")
	}
	return "", nil, false, nil
}

// startOwned is start under an owner. A child running with other parameters
// is retired first; its unobserved cleanup is an error and nothing is started.
func (s *Session) startOwned(ctx context.Context, options Options, args []string, event Event) (bool, error) {
	st := s.budget.stable
	env := options.Env
	if env == nil {
		env = []string{}
	}
	if s.command != nil && (s.binary != options.Binary || s.dir != options.Dir || !slices.Equal(s.env, env) || !slices.Equal(s.args, args)) {
		s.owner.Stop()
		if err := s.finishOwned(ctx, event, nil); err != nil {
			return false, err
		}
	}
	if s.command != nil {
		return true, nil
	}
	st.mu.Lock()
	held := st.held
	st.mu.Unlock()
	if held || !groupreap.OwnerAvailable() {
		return false, containment()
	}
	stdinRead, stdinWrite, err := os.Pipe()
	if err != nil {
		return false, nil
	}
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		stdinRead.Close()
		stdinWrite.Close()
		return false, nil
	}
	stderrLimit := options.StderrLimit
	if stderrLimit == 0 {
		stderrLimit = DefaultStderrLimit
	}
	overrun := make(chan struct{})
	var overrunOnce sync.Once
	command := exec.Command(options.Binary, args...)
	command.Dir, command.Env = options.Dir, env
	command.Stdin, command.Stdout = stdinRead, stdoutWrite
	s.stderr = newLimitedBuffer(stderrLimit, 0, func() { overrunOnce.Do(func() { close(overrun) }) })
	command.Stderr = s.stderr
	command.WaitDelay = pipeDrainDelay
	owner, err := groupreap.StartWith(command, st.primitives(event.Ordinal, true))
	stdinRead.Close()
	stdoutWrite.Close()
	if err != nil {
		stdinWrite.Close()
		stdoutRead.Close()
		return false, nil
	}
	s.binary, s.dir, s.env, s.args = options.Binary, options.Dir, slices.Clone(env), slices.Clone(args)
	s.command, s.owner, s.stdin, s.stdout, s.overrun = command, owner, stdinWrite, stdoutRead, overrun
	s.reader = bufio.NewReaderSize(stdoutRead, sessionHeaderLimit)
	return true, nil
}

// finishOwned completes the owner lifecycle of the session child, closes both
// pipes so a pending exchange ends, and forgets the child. A HOLD keeps the
// owner handle inside the owner and is reported as a containment refusal.
func (s *Session) finishOwned(ctx context.Context, event Event, pending <-chan sessionReply) error {
	result := s.budget.stable.finish(ctx, s.owner, event)
	s.stdin.Close()
	s.stdout.Close()
	if pending != nil {
		<-pending
	}
	s.release()
	s.owner = nil
	if result.State != groupreap.Released {
		return containment()
	}
	return nil
}

// CloseContext ends a stable session and reports whether its cleanup was
// observed. The child sees EOF on stdin and may finish within ten seconds of
// remaining outer time; its still-owned group is then retired and reaped. A
// caller cancellation or outer expiry moves the close to the single emergency
// allowance. A legacy session has no cleanup status and returns nil.
func (s *Session) CloseContext(ctx context.Context) error {
	if s.budget == nil {
		s.Close()
		return nil
	}
	st := s.budget.stable
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.command == nil {
		if s.budget.Held() {
			return containment()
		}
		return nil
	}
	event := Event{Session: true}
	event.Name = "session-close-start"
	st.event(event)
	s.stdin.Close()
	start := st.now()
	soft, stop := st.until(func() bool {
		return st.terminal(ctx) || !st.now().Before(start.Add(DefaultPerOpTimeout))
	})
	select {
	case <-s.owner.Exited():
	case <-soft:
		s.owner.Stop()
	}
	stop()
	return s.finishOwned(ctx, event, nil)
}
