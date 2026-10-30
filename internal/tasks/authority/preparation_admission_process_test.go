//go:build darwin || linux

package authority

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type admissionEvent struct {
	ID     string `json:"id"`
	Phase  string `json:"phase"`
	Rank   uint64 `json:"rank,omitempty"`
	FD     int    `json:"fd,omitempty"`
	Bytes  int    `json:"bytes,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// Helpers have no descendants. The exec witness replaces this same process.
// Each helper has its own 15s terminal bound in addition to parent cancellation.
func TestGH494PreparationAdmissionProcessHelper(t *testing.T) {
	mode := os.Getenv("GH494_FAIR_HELPER")
	if mode == "" {
		return
	}
	watch := time.AfterFunc(15*time.Second, func() { os.Exit(124) })
	defer watch.Stop()
	id := os.Getenv("GH494_FAIR_ID")
	emit := func(e admissionEvent) {
		e.ID = id
		if err := json.NewEncoder(os.Stdout).Encode(e); err != nil {
			t.Fatal(err)
		}
	}
	if mode == "exec-witness" {
		fd, e := strconv.Atoi(os.Getenv("GH494_FAIR_FD"))
		if e != nil {
			t.Fatal(e)
		}
		var st syscall.Stat_t
		if e = syscall.Fstat(fd, &st); e != syscall.EBADF {
			t.Fatalf("actual slot FD%d after exec: %v (possible FD reuse is not proof)", fd, e)
		}
		emit(admissionEvent{Phase: "exec-closed", FD: fd, Detail: "Fstat=EBADF after actual self-exec"})
		return
	}
	repo, e := intent.Resolve(os.Getenv("GH494_FAIR_ROOT"))
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 14*time.Second)
	defer cancel()
	commands := bufio.NewScanner(os.Stdin)
	command := func() string {
		if !commands.Scan() {
			cancel()
			return "cancel"
		}
		return commands.Text()
	}
	// Keep actual product-owned descriptors above ordinary runtime startup FDs.
	// No ownership descriptor is duplicated or passed in ExtraFiles.
	var padding []*os.File
	if mode == "exec" {
		for i := 0; i < 128; i++ {
			f, e := os.Open(os.DevNull)
			if e != nil {
				t.Fatal(e)
			}
			padding = append(padding, f)
		}
		defer func() {
			for _, f := range padding {
				f.Close()
			}
		}()
	}
	queued := false
	admissionReached = func(phase string, rank uint64) {
		if mode == "registry" && phase == "registry" {
			emit(admissionEvent{Phase: "registry-owned"})
			command()
			return
		}
		if mode == "slot-owned" && phase == "slot-owned" {
			emit(admissionEvent{Phase: "slot-owned"})
			command()
			return
		}
		if mode == "exec" || mode == "partial" {
			return
		}
		if phase == "published" {
			emit(admissionEvent{Phase: phase, Rank: rank})
			if command() == "cancel" {
				cancel()
			}
		}
		if phase == "queued" && !queued {
			queued = true
			emit(admissionEvent{Phase: phase, Rank: rank})
		}
	}
	if mode == "partial" {
		admissionWrite = func(f *os.File, b []byte) (int, error) {
			n, e := f.WriteAt(b[:7], 0)
			if e != nil || n != 7 {
				t.Fatalf("actual partial write %d %v", n, e)
			}
			emit(admissionEvent{Phase: "partial-written", Bytes: n})
			command()
			return n, e
		}
	}
	l, e := AcquirePreparation(ctx, repo, LockOptions{Wait: 13 * time.Second, Poll: time.Millisecond})
	if errors.Is(e, context.Canceled) {
		emit(admissionEvent{Phase: "cancelled", Detail: e.Error()})
		if l != nil {
			t.Fatal("cancelled handle")
		}
		return
	}
	if e != nil {
		t.Fatal(e)
	}
	if mode == "exec" {
		fd := int(l.slot.f.Fd())
		flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFD, 0)
		if errno != 0 || flags&syscall.FD_CLOEXEC == 0 {
			t.Fatalf("product descriptor is not CLOEXEC: %v %d", errno, flags)
		}
		var st syscall.Stat_t
		if e = syscall.Fstat(fd, &st); e != nil {
			t.Fatal(e)
		}
		emit(admissionEvent{Phase: "exec-owned", FD: fd, Detail: fmt.Sprintf("device=%d inode=%d flags=%d", st.Dev, st.Ino, flags)})
		env := []string{}
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "GH494_FAIR_HELPER=") && !strings.HasPrefix(v, "GH494_FAIR_FD=") {
				env = append(env, v)
			}
		}
		env = append(env, "GH494_FAIR_HELPER=exec-witness", "GH494_FAIR_FD="+strconv.Itoa(fd))
		if e = syscall.Exec(os.Args[0], []string{os.Args[0], "-test.run=^TestGH494PreparationAdmissionProcessHelper$"}, env); e != nil {
			t.Fatal(e)
		}
		return
	}
	emit(admissionEvent{Phase: "entered", Rank: func() uint64 {
		r, e := preparationRank(l.slot)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}()})
	if command() != "release" {
		t.Fatal("expected release")
	}
	if e = l.Close(); e != nil {
		t.Fatal(e)
	}
	emit(admissionEvent{Phase: "retired"})
}

type admissionChild struct {
	t          *testing.T
	cmd        *exec.Cmd
	cancel     context.CancelFunc
	input      *os.File
	events     chan admissionEvent
	readerDone chan struct{}
	joined     bool
	mu         sync.Mutex
	raw        []admissionEvent
	output     []string
	readErr    error
}

func admissionStart(t *testing.T, repo *intent.Repository, id, mode string) *admissionChild {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGH494PreparationAdmissionProcessHelper$")
	env := []string{}
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "GH494_FAIR_HELPER=") && !strings.HasPrefix(v, "GH494_FAIR_ROOT=") && !strings.HasPrefix(v, "GH494_FAIR_ID=") {
			env = append(env, v)
		}
	}
	cmd.Env = append(env, "GH494_FAIR_HELPER="+mode, "GH494_FAIR_ROOT="+repo.PrimaryWorktree, "GH494_FAIR_ID="+id)
	// Explicit pipes are closed and reader joined before fixture teardown.
	in, write, e := os.Pipe()
	if e != nil {
		cancel()
		t.Fatal(e)
	}
	read, out, e := os.Pipe()
	if e != nil {
		cancel()
		in.Close()
		write.Close()
		t.Fatal(e)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, out
	c := &admissionChild{t: t, cmd: cmd, cancel: cancel, input: write, events: make(chan admissionEvent, 32), readerDone: make(chan struct{})}
	if e = cmd.Start(); e != nil {
		cancel()
		in.Close()
		write.Close()
		read.Close()
		out.Close()
		t.Fatal(e)
	}
	in.Close()
	out.Close()
	go func() {
		defer close(c.readerDone)
		defer close(c.events)
		defer read.Close()
		scan := bufio.NewScanner(read)
		scan.Buffer(make([]byte, 4096), 256*1024)
		for scan.Scan() {
			line := scan.Text()
			c.mu.Lock()
			if len(c.output) < 128 {
				c.output = append(c.output, line)
			}
			c.mu.Unlock()
			var event admissionEvent
			if json.Unmarshal([]byte(line), &event) == nil && event.Phase != "" {
				c.mu.Lock()
				c.raw = append(c.raw, event)
				c.mu.Unlock()
				c.events <- event
			}
		}
		c.readErr = scan.Err()
	}()
	t.Cleanup(func() {
		if !c.joined {
			c.cmd.Process.Kill()
			c.join(false)
		}
		c.cancel()
	})
	return c
}

func (c *admissionChild) send(command string) {
	c.t.Helper()
	if _, e := fmt.Fprintln(c.input, command); e != nil {
		c.t.Fatal(e)
	}
}

func (c *admissionChild) wait(phase string) admissionEvent {
	c.t.Helper()
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	for {
		select {
		case e, ok := <-c.events:
			if !ok {
				c.mu.Lock()
				text := strings.Join(c.output, "\n")
				c.mu.Unlock()
				c.t.Fatalf("child ended before %s: %s", phase, text)
			}
			if e.Phase == phase {
				return e
			}
		case <-timer.C:
			c.t.Fatal("child boundary timeout: " + phase)
		}
	}
}

func (c *admissionChild) join(success bool) {
	c.t.Helper()
	if c.joined {
		c.t.Fatal("duplicate child join")
	}
	c.input.Close()
	e := c.cmd.Wait()
	c.joined = true
	c.cancel()
	<-c.readerDone
	if success && e != nil {
		c.mu.Lock()
		out := strings.Join(c.output, "\n")
		c.mu.Unlock()
		c.t.Fatalf("child exit %v: %s", e, out)
	}
	if !success && e == nil {
		c.t.Error("killed child exited successfully")
	}
	if c.readErr != nil {
		c.t.Error(c.readErr)
	}
}

func (c *admissionChild) kill() {
	c.t.Helper()
	if e := c.cmd.Process.Kill(); e != nil {
		c.t.Fatal(e)
	}
	c.join(false)
}
func (c *admissionChild) release() { c.send("release"); c.wait("retired"); c.join(true) }

func admissionAudit(t *testing.T, r *fixture.Repo) {
	t.Helper()
	q, e := wire.ParseQueueID("", fixture.QueueID)
	if e != nil {
		t.Fatal(e)
	}
	a, e := (journal.Reader{Source: journal.Native{StateDir: r.StateDir, PrimaryWorktree: r.Root}, QueueID: q, PrimaryWorktree: r.Root}).Audit("intent/queue.json")
	if e != nil || a == nil || a.StructuralConsistency != "CONSISTENT" || a.ProjectionAgreement != "AGREES" {
		t.Fatalf("native canonical audit: %+v %v", a, e)
	}
}

func TestGH494PreparationAdmissionProcess(t *testing.T) {
	names := []string{"SeparateOpen", "KilledOwner", "ExecClosedFD", "RegisteredOrder", "CancelHead", "CancelMiddle", "CrashPartialRecord", "CrashPublished", "CrashRegistry", "CrashServing"}
	cases := map[string]any{}
	var filesystem any
	for _, name := range names {
		proof := map[string]any{"status": "FAIL", "cleanup": "NOT_PROVED", "canonicalBeforeAfterEqual": false}
		ok := t.Run(name, func(t *testing.T) {
			r, repo := preparationOpenFixture(t)
			fs, e := Qualify(repo.CommonDir)
			if e != nil {
				t.Fatal(e)
			}
			filesystem = map[string]any{"supported": true, "qualification": fs}
			beforeState, beforeIntent := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
			var children []*admissionChild
			start := func(id, mode string) *admissionChild {
				c := admissionStart(t, repo, id, mode)
				children = append(children, c)
				return c
			}
			reached := []any{}
			switch name {
			case "SeparateOpen":
				admissionSeparateOpen(t, filepath.Join(repo.CommonDir, preparationSlotName(0)))
				reached = append(reached, "same-process independent open WOULD_BLOCK; original close; second acquisition")
			case "KilledOwner":
				c := start("owner", "slot-owned")
				reached = append(reached, c.wait("slot-owned"))
				p, e := os.OpenFile(filepath.Join(repo.CommonDir, preparationSlotName(0)), os.O_RDWR, 0600)
				if e != nil {
					t.Fatal(e)
				}
				defer p.Close()
				if e = withFD(p, flockExclusiveNB); !isWouldBlock(e) {
					t.Fatal("parent did not observe live ownership", e)
				}
				c.kill()
				if e = withFD(p, flockExclusiveNB); e != nil {
					t.Fatal("owner lock retained after joined kill", e)
				}
				if e = p.Close(); e != nil {
					t.Fatal(e)
				}
				reached = append(reached, "parent WOULD_BLOCK; captured child kill/join; same-descriptor acquisition and close")
			case "ExecClosedFD":
				c := start("exec", "exec")
				owned := c.wait("exec-owned")
				closed := c.wait("exec-closed")
				c.join(true)
				if owned.FD != closed.FD || owned.FD < 128 {
					t.Fatal("controlled descriptor witness differs", owned, closed)
				}
				reached = append(reached, owned, closed)
				proof["closedFDWitness"] = true
				l, e := AcquirePreparation(context.Background(), repo, LockOptions{Wait: time.Second})
				if e != nil {
					t.Fatal(e)
				}
				if e = l.Close(); e != nil {
					t.Fatal(e)
				}
			case "RegisteredOrder", "CancelHead", "CancelMiddle":
				a := start("A", "normal")
				a.wait("published")
				a.send("go")
				first := a.wait("entered")
				reached = append(reached, first)
				b := start("B", "normal")
				bp := b.wait("published")
				reached = append(reached, bp)
				c := start("C", "normal")
				cp := c.wait("published")
				reached = append(reached, cp)
				if !(first.Rank < bp.Rank && bp.Rank < cp.Rank) {
					t.Fatal("live ranks not ordered")
				}
				if name == "CancelHead" {
					b.send("cancel")
					reached = append(reached, b.wait("cancelled"))
					b.join(true)
					c.send("go")
					reached = append(reached, c.wait("queued"))
					a.release()
					reached = append(reached, c.wait("entered"))
					c.release()
					break
				}
				if name == "CancelMiddle" {
					d := start("D", "normal")
					dp := d.wait("published")
					c.send("cancel")
					reached = append(reached, c.wait("cancelled"))
					c.join(true)
					if dp.Rank <= cp.Rank {
						t.Fatal("successor not behind cancelled middle")
					}
					fresh := start("E", "normal")
					ep := fresh.wait("published")
					reached = append(reached, dp, ep)
					if ep.Rank <= dp.Rank {
						t.Fatal("fresh caller overtook successor")
					}
					// A and B still own slots00/01, so the abandoned middle slot02
					// must now contain E's rank, without deleting or replacing its name.
					raw, e := os.ReadFile(filepath.Join(repo.CommonDir, preparationSlotName(2)))
					if e != nil || !reflect.DeepEqual(raw, admissionRecord(ep.Rank)) {
						t.Fatal("cancelled middle slot not reclaimed", e)
					}
					fresh.send("go")
					reached = append(reached, fresh.wait("queued"))
					d.send("go")
					reached = append(reached, d.wait("queued"))
					a.release()
					b.send("go")
					reached = append(reached, b.wait("entered"))
					b.release()
					reached = append(reached, d.wait("entered"))
					d.release()
					reached = append(reached, fresh.wait("entered"))
					fresh.release()
					break
				}
				a.release()
				a2 := start("A2", "normal")
				ap := a2.wait("published")
				if ap.Rank <= cp.Rank {
					t.Fatal("fresh A2 not tail")
				}
				reached = append(reached, ap)
				// Later callers actually scan and wait before B becomes runnable.
				c.send("go")
				reached = append(reached, c.wait("queued"))
				a2.send("go")
				reached = append(reached, a2.wait("queued"))
				b.send("go")
				be := b.wait("entered")
				reached = append(reached, be)
				b.release()
				ce := c.wait("entered")
				reached = append(reached, ce)
				c.release()
				ae := a2.wait("entered")
				reached = append(reached, ae)
				a2.release()
				order := []string{first.ID, be.ID, ce.ID, ae.ID}
				if !reflect.DeepEqual(order, []string{"A", "B", "C", "A2"}) {
					t.Fatal(order)
				}
				proof["entryOrder"] = order
			default:
				mode, phase := "normal", "published"
				if name == "CrashPartialRecord" {
					mode, phase = "partial", "partial-written"
				}
				if name == "CrashRegistry" {
					mode, phase = "registry", "registry-owned"
				}
				c := start("crash", mode)
				event := c.wait(phase)
				reached = append(reached, event)
				if name == "CrashPartialRecord" {
					if event.Bytes <= 0 || event.Bytes >= 16 {
						t.Fatal("strict prefix not reached")
					}
					proof["partialWriteReached"] = true
				}
				if name == "CrashServing" {
					c.send("go")
					reached = append(reached, c.wait("entered"))
				}
				registryPath := filepath.Join(repo.CommonDir, preparationRegistryName)
				registryBefore, e := os.Stat(registryPath)
				if e != nil {
					t.Fatal(e)
				}
				c.kill()
				l, e := AcquirePreparation(context.Background(), repo, LockOptions{Wait: time.Second, Poll: time.Millisecond})
				if e != nil {
					t.Fatal("successor after joined crash", e)
				}
				if e = l.Close(); e != nil {
					t.Fatal(e)
				}
				registryAfter, e := os.Stat(registryPath)
				if e != nil || !os.SameFile(registryBefore, registryAfter) {
					t.Fatal("registry identity changed on recovery", e)
				}
				reached = append(reached, "joined owner death; same registry inode; successor acquired and retired gate+slot")
			}
			raw := []any{}
			for _, c := range children {
				if !c.joined || c.cmd.ProcessState == nil {
					t.Fatal("child not joined")
				}
				c.mu.Lock()
				raw = append(raw, map[string]any{"pid": c.cmd.Process.Pid, "joined": true, "exit": c.cmd.ProcessState.String(), "events": append([]admissionEvent(nil), c.raw...), "output": append([]string(nil), c.output...)})
				c.mu.Unlock()
			}
			admissionAudit(t, r)
			if !reflect.DeepEqual(beforeState, fixture.TreeSnapshot(t, r.StateDir)) || !reflect.DeepEqual(beforeIntent, fixture.TreeSnapshot(t, r.IntentDir)) {
				t.Fatal("scheduler changed canonical journal/intent")
			}
			proof["reached"], proof["children"], proof["cleanup"], proof["canonicalBeforeAfterEqual"] = reached, raw, "PROVED", true
		})
		if ok {
			proof["status"] = "PASS"
		}
		cases[name] = proof
		raw, e := json.Marshal(proof)
		if e != nil || len(raw) > 256*1024 {
			t.Fatal("raw case exceeds bound", e)
		}
		t.Logf("GH494_FAIR_CASE %s %s", name, raw)
		if dir := os.Getenv("GH494_FAIR_EVIDENCE_DIR"); dir != "" {
			if e = os.MkdirAll(dir, 0700); e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(dir, name+".json"), raw, 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
	status := "PASS"
	if t.Failed() {
		status = "FAIL"
	}
	summary := map[string]any{"status": status, "platform": runtime.GOOS, "filesystem": filesystem, "cases": cases}
	raw, e := json.Marshal(summary)
	if e != nil || len(raw) > 256*1024 {
		t.Fatal("summary exceeds bound", e)
	}
	t.Logf("GH494_FAIR_SUMMARY %s", raw)
	if dir := os.Getenv("GH494_FAIR_EVIDENCE_DIR"); dir != "" {
		if e = os.WriteFile(filepath.Join(dir, "process-summary.json"), raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
