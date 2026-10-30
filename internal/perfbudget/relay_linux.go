//go:build linux

package perfbudget

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"slices"
	"syscall"
	"time"
)

// rssRelayEnv marks a process started as the peak-RSS relay (PRB-V0-060).
const rssRelayEnv = "CORVINT_PERF_BUDGET_RSS_RELAY"

// Go's os/exec starts a child with clone(CLONE_VFORK|CLONE_VM), so until exec the child runs in
// the parent's memory, and Linux copies that memory's high-water mark into the child's maxrss at
// exec. A measured process started by the harness would report the harness's resident set
// (#1976: six scenarios read the same ~437 MiB). The harness therefore starts each measured
// process through a relay, a fresh copy of its own executable that holds only the Go runtime: the
// relay starts the measured process, waits for it, and reports its maxrss and wall time on fd 3,
// so the measured peak carries at most the relay's own few MiB (PRB-V0-060).
func init() {
	if os.Getenv(rssRelayEnv) == "1" {
		os.Exit(relayMain(os.Args[1:]))
	}
}

// relayReport is what the relay writes on fd 3.
type relayReport struct {
	StartError string `json:"start_error,omitempty"`
	WallUS     int64  `json:"wall_us"`
	MaxRSSKiB  int64  `json:"max_rss_kib"`
	RelayKiB   int64  `json:"relay_kib"`
}

// relayMain runs argv with the relay's stdio and working directory and the relay's environment
// without rssRelayEnv, reports it on fd 3 and exits with its exit code (128+signal when it was
// killed). The measured process inherits the relay's process group, so the harness's group sweep
// reaches it.
func relayMain(argv []string) int {
	// fd 3 is the harness's report pipe: close it on exec, so neither the measured process nor a
	// descendant that outlives the group sweep holds the harness's read open (V1-3079).
	syscall.CloseOnExec(3)
	report := os.NewFile(3, "rss-relay")
	write := func(r relayReport) {
		if report != nil {
			_ = json.NewEncoder(report).Encode(r)
		}
	}
	if len(argv) == 0 {
		write(relayReport{StartError: "no command"})
		return 127
	}
	var self syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &self)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = slices.DeleteFunc(os.Environ(), func(kv string) bool { return kv == rssRelayEnv+"=1" })
	start := time.Now()
	if err := cmd.Start(); err != nil {
		write(relayReport{StartError: err.Error(), RelayKiB: self.Maxrss})
		return 127
	}
	_ = cmd.Wait()
	wall := time.Since(start)
	r := relayReport{WallUS: wall.Microseconds(), RelayKiB: self.Maxrss}
	if usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok && usage != nil {
		r.MaxRSSKiB = usage.Maxrss
	}
	write(r)
	if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal())
	}
	return cmd.ProcessState.ExitCode()
}

// relay is one measured process started through the relay (see relayMain).
type relay struct {
	target string
	r, w   *os.File
	report *relayReport
}

// startRelayed rewrites cmd, before it starts, to run its command through the relay, and returns
// the relay whose result is read once cmd has been waited for. A cmd whose command did not
// resolve is left as it is, so it fails to start as before. Without its own executable the
// harness cannot relay, and the measured peak is not observed.
func startRelayed(cmd *exec.Cmd) (*relay, error) {
	if cmd.Err != nil {
		return nil, nil
	}
	self, err := os.Executable()
	if err != nil {
		return nil, errNoRelay
	}
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	rl := &relay{target: cmd.Path, r: r, w: w}
	cmd.Args = append([]string{self, cmd.Path}, cmd.Args[1:]...)
	cmd.Path = self
	env := cmd.Env
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = append(slices.Clone(env), rssRelayEnv+"=1")
	cmd.ExtraFiles = []*os.File{w}
	return rl, nil
}

// started closes the harness's copy of the report pipe's write end once cmd has started.
func (rl *relay) started() {
	if rl != nil && rl.w != nil {
		_ = rl.w.Close()
		rl.w = nil
	}
}

// result reads the relay's report after cmd was waited for; nil when the relay wrote none.
func (rl *relay) result() *relayReport {
	if rl == nil {
		return nil
	}
	rl.started()
	if rl.report != nil {
		return rl.report
	}
	// One report line is all the relay writes; stop there rather than wait for end of file.
	raw, _ := bufio.NewReaderSize(io.LimitReader(rl.r, 4096), 4096).ReadBytes('\n')
	_ = rl.r.Close()
	var r relayReport
	if json.Unmarshal(raw, &r) != nil {
		return nil
	}
	rl.report = &r
	return rl.report
}
