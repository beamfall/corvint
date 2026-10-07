//go:build darwin && (arm64 || amd64)

package supervisor

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func psRows(t *testing.T) map[int]processRow {
	t.Helper()
	raw, err := exec.Command("/bin/ps", "-axo", "pid=,ppid=,pgid=,stat=").Output()
	if err != nil {
		t.Fatal(err)
	}
	rows := map[int]processRow{}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) != 4 {
			continue
		}
		pid, _ := strconv.Atoi(f[0])
		ppid, _ := strconv.Atoi(f[1])
		pgid, _ := strconv.Atoi(f[2])
		rows[pid] = processRow{pid: pid, ppid: ppid, pgid: pgid, zombie: strings.HasPrefix(f[3], "Z")}
	}
	return rows
}

// CAL-V0-136: the native kern.proc.all snapshot reports the facts ps
// reports: exactly for this process, a child leading its own group and an
// unreaped zombie, and for the processes both snapshots share.
func TestCALV0136_NativeProcessRowsMatchPS(t *testing.T) {
	leader := exec.Command("/bin/sleep", "30")
	leader.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := leader.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { leader.Process.Kill(); leader.Wait() }()
	zombie := exec.Command("/usr/bin/true")
	if err := zombie.Start(); err != nil {
		t.Fatal(err)
	}
	defer zombie.Wait()
	want := map[int]processRow{
		os.Getpid():        {pid: os.Getpid(), ppid: os.Getppid(), pgid: syscall.Getpgrp()},
		leader.Process.Pid: {pid: leader.Process.Pid, ppid: os.Getpid(), pgid: leader.Process.Pid},
		zombie.Process.Pid: {pid: zombie.Process.Pid, ppid: os.Getpid(), pgid: syscall.Getpgrp(), zombie: true},
	}
	var native []processRow
	for i := 0; ; i++ {
		rows, err := processRows()
		if err != nil {
			t.Fatal(err)
		}
		got := map[int]processRow{}
		for _, r := range rows {
			got[r.pid] = r
		}
		ok := true
		for pid, w := range want {
			ok = ok && got[pid] == w
		}
		if ok {
			native = rows
			break
		}
		if i > 200 {
			t.Fatalf("native rows %v, want %v", []processRow{got[os.Getpid()], got[leader.Process.Pid], got[zombie.Process.Pid]}, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
	ps := psRows(t)
	if _, ok := ps[0]; ok {
		t.Fatal("ps reported pid 0")
	}
	shared, differ := 0, 0
	for _, r := range native {
		if r.pid == 0 {
			t.Fatal("native rows include pid 0")
		}
		if p, ok := ps[r.pid]; ok {
			shared++
			if p != r {
				differ++
			}
		}
	}
	if shared < len(ps)/2 || differ > shared/20 {
		t.Fatalf("native and ps snapshots disagree: shared %d of %d, differ %d", shared, len(ps), differ)
	}
	for pid, w := range want {
		if ps[pid] != w {
			t.Fatalf("ps row %v, want %v", ps[pid], w)
		}
	}
}

// BenchmarkCALV0136_ProcessRows guards the per-scan cost of the detached
// host escape scan (escapeScanEvery).
func BenchmarkCALV0136_ProcessRows(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := processRows(); err != nil {
			b.Fatal(err)
		}
	}
}
