//go:build darwin || linux

// Command pi-tui-fixture drives native Pi TUI prompts through a pseudo-terminal and reaps the
// complete owned PTY process group. It is the Pi integration tests' PTY driver, written in Go so
// those tests invoke no Python interpreter (GOC-V0-008).
//
// Usage: pi-tui-fixture [-protected] command [args...]
//
// The default drives one offline prompt; -protected drives prompts, /reload and /new session
// replacement against the protected runtime, numbering its fixture tokens from
// CORVINT_PI_TUI_FIRST. CORVINT_PI_PTY_WITNESS, when set, receives the PTY process-group id.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// interrupted is the signal that stopped the drive; the process exits 128 plus its number.
type interrupted syscall.Signal

func (i interrupted) Error() string { return syscall.Signal(i).String() }

// driver types the next input once the TUI output so far calls for it.
type driver interface {
	step(s *session) error
}

type session struct {
	pid     int
	master  *os.File
	chunks  chan []byte
	output  []byte
	start   time.Time
	stopped bool
	reaped  bool
	status  syscall.WaitStatus
}

func main() {
	protected := flag.Bool("protected", false, "drive reload and session replacement against the protected runtime")
	flag.Parse()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	err := run(*protected, flag.Args(), signals)
	var stop interrupted
	if errors.As(err, &stop) {
		os.Exit(128 + int(stop))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "pi-tui-fixture:", err)
		os.Exit(1)
	}
}

func run(protected bool, argv []string, signals <-chan os.Signal) error {
	d, limit, err := newDriver(protected)
	if err != nil {
		return err
	}
	s, err := start(argv)
	if err != nil {
		return err
	}
	defer s.cleanup()
	if err := s.witness(); err != nil {
		return err
	}
	return s.drive(d, limit, signals)
}

func newDriver(protected bool) (driver, time.Duration, error) {
	if !protected {
		return &plain{}, 25 * time.Second, nil
	}
	first, err := strconv.Atoi(os.Getenv("CORVINT_PI_TUI_FIRST"))
	if err != nil {
		return nil, 0, fmt.Errorf("CORVINT_PI_TUI_FIRST: %w", err)
	}
	return &protectedDriver{first: first}, 40 * time.Second, nil
}

// start runs argv as the session leader of a new 30x100 PTY.
func start(argv []string) (*session, error) {
	if len(argv) == 0 {
		return nil, errors.New("usage: pi-tui-fixture [-protected] command [args...]")
	}
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return nil, err
	}
	fd, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	master := os.NewFile(uintptr(fd), "/dev/ptmx")
	name, err := unlockPTY(uintptr(fd))
	if err != nil {
		master.Close()
		return nil, err
	}
	replica, err := os.OpenFile(name, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, err
	}
	defer replica.Close()
	size := [4]uint16{30, 100, 0, 0}
	if err := ioctl(replica.Fd(), syscall.TIOCSWINSZ, unsafe.Pointer(&size)); err != nil {
		master.Close()
		return nil, err
	}
	process, err := os.StartProcess(path, argv, &os.ProcAttr{
		Env:   os.Environ(),
		Files: []*os.File{replica, replica, replica},
		Sys:   &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0},
	})
	if err != nil {
		master.Close()
		return nil, err
	}
	s := &session{pid: process.Pid, master: master, chunks: make(chan []byte), start: time.Now()}
	go s.read()
	return s, nil
}

func (s *session) read() {
	for {
		chunk := make([]byte, 16384)
		n, err := s.master.Read(chunk)
		if n > 0 {
			s.chunks <- chunk[:n]
		}
		if err != nil {
			return
		}
	}
}

func (s *session) witness() error {
	path := os.Getenv("CORVINT_PI_PTY_WITNESS")
	if path == "" {
		return nil
	}
	return os.WriteFile(path, []byte(strconv.Itoa(s.pid)), 0o666)
}

func (s *session) drive(d driver, limit time.Duration, signals <-chan os.Signal) error {
	for time.Since(s.start) < limit {
		if err := d.step(s); err != nil {
			return err
		}
		select {
		case number := <-signals:
			return interrupted(number.(syscall.Signal))
		case chunk := <-s.chunks:
			s.output = append(s.output, chunk...)
		case <-time.After(50 * time.Millisecond):
		}
		if len(s.output) > 1<<20 {
			return errors.New("TUI output bound")
		}
		if err := s.poll(); err != nil {
			return err
		}
		if !s.reaped {
			continue
		}
		if !s.stopped || !s.status.Exited() || s.status.ExitStatus() != 0 {
			return errors.New("TUI exited before successful prompt/shutdown: " + s.tail())
		}
		fmt.Println("Native Pi TUI prompt and clean shutdown passed")
		return nil
	}
	return errors.New("TUI deadline: " + s.tail())
}

func (s *session) send(input string) error {
	_, err := s.master.Write([]byte(input))
	return err
}

func (s *session) seen(text string) bool {
	return bytes.Contains(s.output, []byte(text))
}

func (s *session) tail() string {
	return strings.ToValidUTF8(string(s.output[max(0, len(s.output)-3000):]), "�")
}

// poll reaps the session leader without blocking.
func (s *session) poll() error {
	if s.reaped {
		return nil
	}
	pid, err := syscall.Wait4(s.pid, &s.status, syscall.WNOHANG, nil)
	s.reaped = pid == s.pid
	return err
}

func (s *session) alive() bool {
	return syscall.Kill(-s.pid, 0) != syscall.ESRCH
}

// cleanup terminates the whole PTY process group, escalating to SIGKILL after one second.
func (s *session) cleanup() {
	syscall.Kill(-s.pid, syscall.SIGTERM)
	until := time.Now().Add(time.Second)
	for s.alive() && time.Now().Before(until) {
		s.poll()
		time.Sleep(10 * time.Millisecond)
	}
	if s.alive() {
		syscall.Kill(-s.pid, syscall.SIGKILL)
	}
	if !s.reaped {
		syscall.Wait4(s.pid, &s.status, 0, nil)
	}
	s.master.Close()
}

// plain sends one prompt after a second and ends the TUI once the fixture responds.
type plain struct {
	sent bool
}

func (d *plain) step(s *session) error {
	if !d.sent && time.Since(s.start) > time.Second {
		d.sent = true
		return s.send("inspect main.go\r")
	}
	if !s.stopped && s.seen("fixture response") {
		s.stopped = true
		return s.send("\x04")
	}
	return nil
}

// protectedDriver prompts, reloads, prompts, replaces the session, prompts, then ends the TUI.
type protectedDriver struct {
	first      int
	phase      int
	sent       bool
	readyAt    time.Time
	responseAt time.Time
}

func (d *protectedDriver) step(s *session) error {
	if d.readyAt.IsZero() && s.seen("fixture") {
		d.readyAt = time.Now()
	}
	if !d.sent && !d.readyAt.IsZero() && time.Since(d.readyAt) > 500*time.Millisecond {
		d.sent = true
		if err := s.send("inspect main.go\r"); err != nil {
			return err
		}
	}
	token := "PROTECTED_NATIVE_PI_OK_" + strconv.Itoa(d.first+d.phase/2)
	if d.phase%2 == 0 && d.phase < 5 && d.responseAt.IsZero() && s.seen(token) {
		d.responseAt = time.Now()
	}
	// A displayed token precedes agent_settled. Wait past the bounded Stop child deadline.
	ready := !d.responseAt.IsZero() && time.Since(d.responseAt) > 2200*time.Millisecond
	switch {
	case d.phase == 0 && ready:
		return d.advance(s, "/reload\r")
	case d.phase == 1 && s.seen("Reloaded keybindings"):
		return d.advance(s, "inspect One after reload\r")
	case d.phase == 2 && ready:
		return d.advance(s, "/new\r")
	case d.phase == 3 && s.seen("New session started"):
		return d.advance(s, "inspect One after replacement\r")
	case d.phase == 4 && ready:
		d.phase = 5
		s.stopped = true
		return s.send("\x04")
	}
	return nil
}

// advance moves to the next phase, forgetting the output and response the last one saw.
func (d *protectedDriver) advance(s *session, input string) error {
	d.phase++
	s.output = s.output[:0]
	d.responseAt = time.Time{}
	return s.send(input)
}

// ioctl converts arg inside the Syscall call so the runtime keeps its referent alive and fixed.
func ioctl(fd, request uintptr, arg unsafe.Pointer) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, uintptr(arg)); errno != 0 {
		return errno
	}
	return nil
}
