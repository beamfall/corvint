package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func guardian() error {
	raw, e := readBound(os.Stdin, 48<<20)
	if e != nil {
		return e
	}
	var w work
	if e = strictDecode(raw, &w); e != nil {
		return e
	}
	deadline := 5 * time.Second
	rssLimit := 768 * 1024
	if w.Mode == "build" {
		deadline = 60 * time.Second
		rssLimit = 2 * 1024 * 1024
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	parent := os.NewFile(3, "parent-liveness")
	if parent == nil {
		return errors.New("missing parent pipe")
	}
	defer parent.Close()
	go func() { var b [1]byte; _, _ = parent.Read(b[:]); cancel() }()
	lifeR, lifeW, e := os.Pipe()
	if e != nil {
		return e
	}
	defer lifeR.Close()
	defer lifeW.Close()
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	cmd := exec.Command(exe, "worker")
	cmd.Env = []string{}
	cmd.Stdin = bytes.NewReader(raw)
	cmd.ExtraFiles = []*os.File{lifeR}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var out, stderr limitedBuffer
	out.max = 48 << 20
	stderr.max = maxOutput
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if e = cmd.Start(); e != nil {
		return e
	}
	lifeR.Close()
	pgid := cmd.Process.Pid
	defer syscall.Kill(-pgid, syscall.SIGKILL)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	var failure error
	waiting := true
	for waiting {
		select {
		case e = <-done:
			failure = e
			waiting = false
		case <-ctx.Done():
			failure = ctx.Err()
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
			<-done
			waiting = false
		case <-ticker.C:
			if e = checkRSS(ctx, pgid, rssLimit); e != nil {
				failure = e
				_ = syscall.Kill(-pgid, syscall.SIGKILL)
				<-done
				waiting = false
			}
		}
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
	until := time.Now().Add(2 * time.Second)
	for syscall.Kill(-pgid, 0) != syscall.ESRCH {
		if time.Now().After(until) {
			return errors.New("cleanup unconfirmed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if failure != nil {
		return fmt.Errorf("worker failed: %w %s", failure, stderr.Bytes())
	}
	if out.overflow || stderr.overflow {
		return errors.New("worker control overflow")
	}
	var t terminal
	if e = strictDecode(out.Bytes(), &t); e != nil {
		return e
	}
	if t.Nonce != w.Nonce || t.Phase != "completed" {
		return errors.New("invalid worker terminal")
	}
	t.Phase = "reaped"
	_, e = os.Stdout.Write(mustJSONLine(t))
	return e
}
func checkRSS(ctx context.Context, pgid, limit int) error {
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/ps", "-axo", "pgid=,rss=")
	cmd.Env = []string{}
	raw, e := cmd.Output()
	if e != nil {
		return fmt.Errorf("RSS monitor lost: %w", e)
	}
	total := 0
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		g, e := strconv.Atoi(f[0])
		if e != nil {
			return e
		}
		if g != pgid {
			continue
		}
		rss, e := strconv.Atoi(f[1])
		if e != nil {
			return e
		}
		total += rss
	}
	if total > limit {
		return errors.New("RSS threshold exceeded")
	}
	return nil
}
