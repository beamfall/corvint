package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"strconv"
	"syscall"
	"time"
)

type principal struct{ uid, gid uint32 }

func account(name string) (principal, error) {
	u, e := user.Lookup(name)
	if e != nil {
		return principal{}, e
	}
	uid, e := strconv.ParseUint(u.Uid, 10, 32)
	if e != nil || uid == 0 {
		return principal{}, errors.New("invalid account")
	}
	gid, e := strconv.ParseUint(u.Gid, 10, 32)
	if e != nil || gid == 0 {
		return principal{}, errors.New("invalid account group")
	}
	return principal{uint32(uid), uint32(gid)}, nil
}

// operatorRun is a fixed one-shot broker, not a service or caller-selected
// command/signer. Root transports bounded opaque bytes; only the authority and
// keyless worker parse source capsules or candidate module data.
func operatorRun(handle string) error {
	if os.Geteuid() != 0 {
		return errors.New("independent operator root context required")
	}
	if _, e := decodeHex(handle, 32); e != nil {
		return e
	}
	unlock, lockErr := operatorLock()
	if lockErr != nil {
		return lockErr
	}
	defer unlock()
	authority, e := account("_corvintauthority")
	if e != nil {
		return e
	}
	check, e := account("_corvintcheck")
	if e != nil {
		return e
	}
	if authority.uid == check.uid || authority.gid == check.gid {
		return errors.New("principal separation")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()
	request, e := launchAs(ctx, authority, []string{"authority-prepare", handle}, nil, 40*time.Second)
	if e != nil {
		return e
	}
	built, e := launchAs(ctx, check, []string{"guardian"}, request, 63*time.Second)
	clear(request)
	if e != nil {
		return e
	}
	request, e = launchAs(ctx, authority, []string{"authority-built", handle}, built, 40*time.Second)
	clear(built)
	if e != nil {
		return e
	}
	observed, e := launchAs(ctx, check, []string{"guardian"}, request, 8*time.Second)
	clear(request)
	if e != nil {
		return e
	}
	_, e = launchAs(ctx, authority, []string{"authority-finish", handle}, observed, 40*time.Second)
	clear(observed)
	return e
}
func launchAs(parent context.Context, p principal, args []string, input []byte, limit time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, limit)
	defer cancel()
	exe, e := os.Executable()
	if e != nil {
		return nil, e
	}
	liveR, liveW, e := os.Pipe()
	if e != nil {
		return nil, e
	}
	defer liveR.Close()
	defer liveW.Close()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Env = []string{}
	cmd.Stdin = bytes.NewReader(input)
	cmd.ExtraFiles = []*os.File{liveR}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Credential: &syscall.Credential{Uid: p.uid, Gid: p.gid, Groups: []uint32{p.gid}}}
	cmd.Cancel = func() error { return liveW.Close() }
	cmd.WaitDelay = 3 * time.Second
	var out, diagnostic limitedBuffer
	out.max = 48 << 20
	diagnostic.max = maxOutput
	cmd.Stdout = &out
	cmd.Stderr = &diagnostic
	if e = cmd.Start(); e != nil {
		return nil, e
	}
	liveR.Close()
	pgid := cmd.Process.Pid
	defer syscall.Kill(-pgid, syscall.SIGKILL)
	e = cmd.Wait()
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
	until := time.Now().Add(2 * time.Second)
	for syscall.Kill(-pgid, 0) != syscall.ESRCH {
		if time.Now().After(until) {
			return nil, errors.New("broker cleanup unconfirmed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if e != nil {
		return nil, fmt.Errorf("protected phase failed: %w %s", e, diagnostic.Bytes())
	}
	if out.overflow || diagnostic.overflow {
		return nil, errors.New("broker output overflow")
	}
	return out.Bytes(), nil
}
func watchLiveness() error {
	life := os.NewFile(3, "supervisor-liveness")
	if life == nil {
		return errors.New("missing supervisor")
	}
	go func() { var b [1]byte; _, _ = life.Read(b[:]); _ = syscall.Kill(-syscall.Getpgrp(), syscall.SIGKILL) }()
	return nil
}
