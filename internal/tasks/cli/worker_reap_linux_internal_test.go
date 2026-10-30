//go:build linux

package cli

import (
	"errors"
	"syscall"
)

// pidfd system call numbers, the same on every Linux architecture Go
// supports (internal/groupreap uses the same ones).
const (
	sysPidfdSendSignal = 424
	sysPidfdOpen       = 434
)

// pidfdPin pins a process through a pidfd: the kernel keeps it naming the
// process it was opened on, and pidfd_send_signal reports ESRCH once that
// process has been reaped instead of reaching a later holder of its pid.
type pidfdPin struct{ fd int }

// openPin opens a pidfd on the process holding pid now. A kernel without
// pidfds fails closed: nothing is pinned, so nothing is signalled.
func openPin(pid int) (pin, bool, error) {
	r, _, errno := syscall.Syscall(sysPidfdOpen, uintptr(pid), 0, 0)
	switch errno {
	case 0:
		return &pidfdPin{fd: int(r)}, false, nil
	case syscall.ESRCH:
		return nil, true, nil
	case syscall.ENOSYS:
		return nil, false, errPinUnsupported
	}
	return nil, false, errno
}

func (p *pidfdPin) send(sig syscall.Signal) error {
	if _, _, errno := syscall.Syscall6(sysPidfdSendSignal, uintptr(p.fd), uintptr(sig), 0, 0, 0, 0); errno != 0 {
		return errno
	}
	return nil
}

func (p *pidfdPin) pinned() (bool, error) {
	switch err := p.send(0); {
	case err == nil:
		return true, nil
	case errors.Is(err, syscall.ESRCH):
		return false, nil
	default:
		return false, err
	}
}

func (p *pidfdPin) signal(sig syscall.Signal) (bool, error) {
	switch err := p.send(sig); {
	case err == nil:
		return false, nil
	case errors.Is(err, syscall.ESRCH):
		return true, nil
	default:
		return false, err
	}
}

func (p *pidfdPin) close() { _ = syscall.Close(p.fd) }
