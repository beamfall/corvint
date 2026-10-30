//go:build linux

package supervisor

import (
	"bufio"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// psRows is `ps -axo pid=,ppid=,pgid=,stat=` parsed into the same rows the
// /proc reader returns, the oracle CAL-V0-136 names.
func psRows(t *testing.T) map[int]processRow {
	t.Helper()
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,pgid=,stat=").Output()
	if err != nil {
		t.Skipf("ps is not usable on this host: %v", err)
	}
	rows := map[int]processRow{}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 4 {
			continue
		}
		pid, e1 := strconv.Atoi(f[0])
		ppid, e2 := strconv.Atoi(f[1])
		pgid, e3 := strconv.Atoi(f[2])
		if e1 != nil || e2 != nil || e3 != nil {
			t.Fatalf("ps row %q", sc.Text())
		}
		rows[pid] = processRow{pid: pid, ppid: ppid, pgid: pgid, zombie: strings.HasPrefix(f[3], "Z")}
	}
	return rows
}

// CAL-V0-363, V1-0911: on a real Linux /proc, not a fake root, the reader
// returns the same pid, parent, group and zombie facts as ps for a live
// tree: a worker in its own process group, an unreaped zombie child, and an
// orphan that was reparented when its parent exited.
func TestCALV0363_ProcRowsAgreeWithPsOnLinux(t *testing.T) {
	sleeper := exec.Command("sleep", "60")
	sleeper.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := sleeper.Start(); err != nil {
		t.Skipf("sleep is not usable: %v", err)
	}
	t.Cleanup(func() { sleeper.Process.Kill(); sleeper.Wait() })

	zombie := exec.Command("true")
	if err := zombie.Start(); err != nil {
		t.Skipf("true is not usable: %v", err)
	}
	t.Cleanup(func() { zombie.Wait() }) // reaped only at the end: it stays a zombie meanwhile

	// The shell prints its background child's pid and exits at once, so the
	// child is reparented to init or the nearest subreaper.
	orphanOut, err := exec.Command("sh", "-c", "sleep 60 >/dev/null 2>&1 </dev/null & echo $!").Output()
	if err != nil {
		t.Skipf("sh is not usable: %v", err)
	}
	orphan, err := strconv.Atoi(strings.TrimSpace(string(orphanOut)))
	if err != nil {
		t.Fatalf("orphan pid %q", orphanOut)
	}
	t.Cleanup(func() { syscall.Kill(orphan, syscall.SIGKILL) })

	deadline := time.Now().Add(5 * time.Second)
	for {
		raw, _ := os.ReadFile("/proc/" + strconv.Itoa(zombie.Process.Pid) + "/stat")
		if row, err := procStatRow(raw); err == nil && row.zombie {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the unreaped child never became a zombie")
		}
		time.Sleep(10 * time.Millisecond)
	}

	got := map[int]processRow{}
	rows, err := processRows()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		got[r.pid] = r
	}
	want := psRows(t)
	for name, pid := range map[string]int{"sleeper": sleeper.Process.Pid, "zombie": zombie.Process.Pid, "orphan": orphan, "self": os.Getpid()} {
		g, ok1 := got[pid]
		w, ok2 := want[pid]
		if !ok1 || !ok2 || g != w {
			t.Errorf("%s %d: /proc %+v (%v), ps %+v (%v)", name, pid, g, ok1, w, ok2)
		}
	}
	if !got[zombie.Process.Pid].zombie || got[sleeper.Process.Pid].pgid != sleeper.Process.Pid || got[orphan].ppid == 0 {
		t.Errorf("rows %+v %+v %+v", got[zombie.Process.Pid], got[sleeper.Process.Pid], got[orphan])
	}
	// Unrelated host processes may change between the two separate reads,
	// so their differences are reported for diagnosis and never asserted.
	agree, differ := 0, 0
	for pid, g := range got {
		if w, ok := want[pid]; ok {
			if g == w {
				agree++
			} else {
				differ++
			}
		}
	}
	t.Logf("unrelated processes: %d agree and %d differ between /proc and ps", agree, differ)
}

// BenchmarkCALV0363_PsFork is the cost of the scan CAL-V0-136 replaced on
// Linux: one `ps` fork per scan, for the same table as
// BenchmarkCALV0136_ProcRowsOfThisHost.
func BenchmarkCALV0363_PsFork(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := exec.Command("ps", "-axo", "pid=,ppid=,pgid=,stat=").Output(); err != nil {
			b.Skipf("ps is not usable: %v", err)
		}
	}
}
