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
	err = command.Start()
	stdinRead.Close()
	stdoutWrite.Close()
	if err != nil {
		stdinWrite.Close()
		stdoutRead.Close()
		return false
	}
	waited := make(chan error, 1)
	go func() { waited <- command.Wait() }()
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
// holds their other ends, waits for that exchange, and sweeps the group after
// the reap like Run.
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
	killDescendants(s.command)
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
		killDescendants(s.command)
	case <-timer.C:
		killGroupThenReap(s.command, s.waited)
	}
	s.release()
}
