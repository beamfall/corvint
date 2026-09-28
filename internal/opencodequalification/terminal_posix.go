//go:build darwin || linux

package opencodequalification

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"syscall"
	"time"
	"unsafe"
)

func ioctl(fd, request uintptr, arg unsafe.Pointer) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, request, uintptr(arg))
	if e != 0 {
		return e
	}
	return nil
}
func runTerminal(ctx context.Context, path string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cfg, e := readObject(path)
	if e != nil {
		return e
	}
	fd, e := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if e != nil {
		return e
	}
	master := os.NewFile(uintptr(fd), "/dev/ptmx")
	defer master.Close()
	name, e := unlockPTY(master.Fd())
	if e != nil {
		return e
	}
	slave, e := os.OpenFile(name, os.O_RDWR|syscall.O_NOCTTY, 0)
	if e != nil {
		return e
	}
	defer slave.Close()
	resize := func(w, h int) error {
		size := [4]uint16{uint16(h), uint16(w), 0, 0}
		return ioctl(master.Fd(), syscall.TIOCSWINSZ, unsafe.Pointer(&size))
	}
	if e = resize(160, 48); e != nil {
		return e
	}
	env := []string{}
	for _, v := range array(cfg["env"]) {
		env = append(env, str(v))
	}
	child, e := os.StartProcess(str(cfg["host"]), []string{str(cfg["host"]), "--standalone"}, &os.ProcAttr{Dir: str(cfg["directory"]), Env: env, Files: []*os.File{slave, slave, slave}, Sys: &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}})
	if e != nil {
		return e
	}
	slave.Close()
	waited := make(chan struct{})
	go func() { _, _ = child.Wait(); close(waited) }()
	copied := make(chan struct{})
	go func() { _, _ = io.Copy(os.Stdout, master); close(copied) }()
	commands := make(chan Object)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		defer close(commands)
		d := json.NewDecoder(os.Stdin)
		for {
			var v Object
			if d.Decode(&v) != nil {
				return
			}
			select {
			case commands <- v:
			case <-ctx.Done():
				return
			}
		}
	}()
	defer func() {
		_ = syscall.Kill(-child.Pid, syscall.SIGTERM)
		select {
		case <-waited:
		case <-time.After(2 * time.Second):
			_ = syscall.Kill(-child.Pid, syscall.SIGKILL)
			<-waited
		}
		_ = syscall.Kill(-child.Pid, syscall.SIGKILL)
		master.Close()
		<-copied
		cancel()
		os.Stdin.Close()
		<-readDone
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-waited:
			return nil
		case v, ok := <-commands:
			if !ok || truth(v["stop"]) {
				return nil
			}
			if s, ok := v["send"].(string); ok {
				if _, e = master.Write([]byte(s)); e != nil {
					return e
				}
			}
			if dims := array(v["resize"]); len(dims) == 2 {
				if e = resize(int(number(dims[0])), int(number(dims[1]))); e != nil {
					return e
				}
				_ = syscall.Kill(child.Pid, syscall.SIGWINCH)
			}
		}
	}
}
