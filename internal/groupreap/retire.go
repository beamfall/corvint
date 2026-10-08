package groupreap

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"sync"
	"syscall"
	"time"
)

// OwnerEnvironmentKey carries a fresh random per-phase token. A process that
// still holds it in its initial environment was started by that phase, even
// after it left the leader's process group and was reparented (TRE-V0-025).
const OwnerEnvironmentKey = "CORVINT_TEST_RUNNER_OWNER"

const (
	OwnerAncestry = "ANCESTRY"
	OwnerToken    = "TOKEN"

	retireRounds   = 32
	retireEnvReads = 4096
	retirePoll     = 3 * time.Second
)

// RetiredProcess is one owned process identity: pid plus kernel start time.
type RetiredProcess struct {
	PID    int    `json:"pid"`
	Start  string `json:"start"`
	Owner  string `json:"owner"`
	Detail string `json:"detail,omitempty"`
}

// Retirement retains every owned detached descendant a phase left. Only an
// empty Unretired list and no Problems prove cleanup.
type Retirement struct {
	Retired   []RetiredProcess `json:"retired"`
	Unretired []RetiredProcess `json:"unretired"`
	Problems  []string         `json:"problems"`
}

func (r Retirement) Clean() bool { return len(r.Unretired) == 0 && len(r.Problems) == 0 }

// Merge appends another phase's result.
func (r *Retirement) Merge(o Retirement) {
	r.Retired = append(r.Retired, o.Retired...)
	r.Unretired = append(r.Unretired, o.Unretired...)
	r.Problems = append(r.Problems, o.Problems...)
}

type procRow struct {
	pid, ppid, uid int
	sec            int64
	usec           int
	zombie         bool
}

func (p procRow) start() string { return fmt.Sprintf("%d.%06d", p.sec, p.usec) }

type retirePrimitives struct {
	rows     func() ([]procRow, error)
	identity func(pid int) (row procRow, alive bool, err error)
	environ  func(pid int, entry string) (bool, error)
	signal   func(pid int, sig syscall.Signal) error
	sleep    func(time.Duration)
}

type tracked struct {
	RetiredProcess
	signalled, gone bool
}

// Retirer finds and retires one phase's owned detached descendants. Ownership
// is proved only by ppid ancestry from the identity-verified live leader or by
// the phase token; each identity is re-verified immediately before a signal.
type Retirer struct {
	mu       sync.Mutex
	p        retirePrimitives
	entry    string
	uid      int
	self     int
	sinceSec int64
	leader   int
	leaderID string
	seen     map[string]*tracked
	problems []string
}

// NewRetirer reserves a fresh token; it refuses where retirement is unproven.
func NewRetirer() (*Retirer, error) {
	if !RetirementSupported {
		return nil, fmt.Errorf("detached descendant retirement unsupported on this platform")
	}
	return newRetirer(platformPrimitives())
}

func newRetirer(p retirePrimitives) (*Retirer, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, err
	}
	// Kernel start times have microsecond resolution; a one-second margin
	// keeps a wall-clock step from hiding a child started just after this.
	return &Retirer{p: p, entry: OwnerEnvironmentKey + "=" + hex.EncodeToString(b[:]), uid: os.Getuid(), self: os.Getpid(), sinceSec: time.Now().Unix() - 1, seen: map[string]*tracked{}}, nil
}

// Token is the environment value the phase must carry.
func (r *Retirer) Token() string { return r.entry[len(OwnerEnvironmentKey)+1:] }

// Leader records the started leader's identity for ancestry proof.
func (r *Retirer) Leader(pid int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if row, alive, err := r.p.identity(pid); err == nil && alive {
		r.leader, r.leaderID = pid, row.start()
	}
}

// Retire stops every owned process until no new one appears, then kills each
// verified identity and polls for its exit. Results accumulate across calls.
func (r *Retirer) Retire() {
	r.mu.Lock()
	defer r.mu.Unlock()
	leaderAlive := false
	if r.leader != 0 {
		if row, alive, err := r.p.identity(r.leader); err == nil && alive && !row.zombie && row.start() == r.leaderID {
			leaderAlive = true
			// The leader is this process's unreaped child; stop its forking.
			_ = r.p.signal(r.leader, sigStop)
		}
	}
	converged := false
	for round := 0; round < retireRounds; round++ {
		rows, err := r.p.rows()
		if err != nil {
			r.problems = append(r.problems, "process snapshot: "+err.Error())
			return
		}
		fresh := r.owned(rows, leaderAlive)
		if fresh < 0 {
			return
		}
		if fresh == 0 {
			converged = true
			break
		}
	}
	if !converged {
		r.problems = append(r.problems, "owned descendants kept appearing")
	}
	for _, t := range r.seen {
		if !t.signalled || t.gone {
			continue
		}
		if r.verified(t) {
			_ = r.p.signal(t.PID, sigKill)
		}
	}
	deadline := time.Now().Add(retirePoll)
	for {
		pending := 0
		for _, t := range r.seen {
			if t.signalled && !t.gone {
				if r.verified(t) {
					pending++
				} else {
					t.gone = true
				}
			}
		}
		if pending == 0 || time.Now().After(deadline) {
			return
		}
		r.p.sleep(10 * time.Millisecond)
	}
}

// owned stops new owned processes in one snapshot and returns how many were
// new, or -1 after recording a bound problem.
func (r *Retirer) owned(rows []procRow, leaderAlive bool) int {
	byPID := make(map[int]procRow, len(rows))
	for _, row := range rows {
		byPID[row.pid] = row
	}
	fresh, reads := 0, 0
	for _, row := range rows {
		if row.zombie || row.pid == r.self || row.pid == r.leader || row.pid <= 1 || row.sec < r.sinceSec {
			continue
		}
		key := fmt.Sprintf("%d/%s", row.pid, row.start())
		if r.seen[key] != nil {
			continue
		}
		owner := ""
		if leaderAlive && descends(byPID, row, r.leader) {
			owner = OwnerAncestry
		} else if row.uid == r.uid {
			if reads++; reads > retireEnvReads {
				r.problems = append(r.problems, "ownership scan bound exceeded")
				return -1
			}
			if ok, err := r.p.environ(row.pid, r.entry); err == nil && ok {
				owner = OwnerToken
			}
		}
		if owner == "" {
			continue
		}
		t := &tracked{RetiredProcess: RetiredProcess{PID: row.pid, Start: row.start(), Owner: owner}}
		r.seen[key] = t
		fresh++
		if row.uid != r.uid {
			// Proven ours by ancestry but not signallable by this user.
			t.Detail = "foreign-uid descendant"
			continue
		}
		if r.verified(t) && r.p.signal(t.PID, sigStop) == nil {
			t.signalled = true
		} else {
			t.gone = !r.verified(t)
			if !t.gone {
				t.Detail = "stop refused"
			}
		}
	}
	return fresh
}

func descends(byPID map[int]procRow, row procRow, leader int) bool {
	for depth := 0; depth < 128 && row.ppid > 1; depth++ {
		if row.ppid == leader {
			return true
		}
		next, ok := byPID[row.ppid]
		if !ok {
			return false
		}
		row = next
	}
	return false
}

// verified reports whether t's pid still names the same live, non-zombie process.
func (r *Retirer) verified(t *tracked) bool {
	row, alive, err := r.p.identity(t.PID)
	return err == nil && alive && !row.zombie && row.start() == t.Start
}

// Result is the accumulated retirement report, sorted by pid.
func (r *Retirer) Result() Retirement {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := Retirement{Retired: []RetiredProcess{}, Unretired: []RetiredProcess{}, Problems: append([]string{}, r.problems...)}
	for _, t := range r.seen {
		if !t.gone && !r.verified(t) {
			t.gone = true
		}
		if t.gone {
			out.Retired = append(out.Retired, t.RetiredProcess)
			continue
		}
		if t.Detail == "" {
			t.Detail = "still running after SIGKILL"
		}
		out.Unretired = append(out.Unretired, t.RetiredProcess)
	}
	less := func(s []RetiredProcess) func(i, j int) bool {
		return func(i, j int) bool { return s[i].PID < s[j].PID || s[i].PID == s[j].PID && s[i].Start < s[j].Start }
	}
	sort.Slice(out.Retired, less(out.Retired))
	sort.Slice(out.Unretired, less(out.Unretired))
	return out
}

// RunRetiring is Run with r retiring owned detached descendants after the
// leader exits and before it is reaped. A nil r is plain Run.
func RunRetiring(command *exec.Cmd, r *Retirer) error {
	if r == nil {
		return Run(command)
	}
	if err := command.Start(); err != nil {
		return err
	}
	r.Leader(command.Process.Pid)
	return wait(command, r.Retire)
}
