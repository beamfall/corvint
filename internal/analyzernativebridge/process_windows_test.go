//go:build windows

package analyzernativebridge

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

const (
	createSuspended           = 0x00000004
	jobObjectExtendedLimits   = 9
	jobObjectLimitKillOnClose = 0x00002000
	errorAccessDenied         = syscall.Errno(5)
	errorInvalidParameter     = syscall.Errno(87)
)

func configureProcessGroup(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.CreationFlags |= createSuspended
}

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	createJobObject          = kernel32.NewProc("CreateJobObjectW")
	assignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	setInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	closeHandle              = kernel32.NewProc("CloseHandle")
	openProcess              = kernel32.NewProc("OpenProcess")
	waitForSingleObject      = kernel32.NewProc("WaitForSingleObject")
	ntResumeProcess          = syscall.NewLazyDLL("ntdll.dll").NewProc("NtResumeProcess")
	jobs                     sync.Map
)

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
	_                       [8 - unsafe.Sizeof(uintptr(0))]byte
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

func attachProcessTree(command *exec.Cmd) error {
	if command.Process == nil {
		return fmt.Errorf("missing process")
	}
	job, _, err := createJobObject.Call(0, 0)
	if job == 0 {
		return err
	}
	info := jobExtendedLimitInformation{}
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnClose
	if ok, _, err := setInformationJobObject.Call(job, jobObjectExtendedLimits, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info)); ok == 0 {
		closeWindowsHandle(job)
		return err
	}
	var attachErr error
	if err := command.Process.WithHandle(func(process uintptr) {
		if ok, _, callErr := assignProcessToJobObject.Call(job, process); ok == 0 {
			attachErr = callErr
			return
		}
		if status, _, _ := ntResumeProcess.Call(process); status != 0 {
			attachErr = fmt.Errorf("NtResumeProcess status=0x%x", status)
		}
	}); err != nil {
		closeWindowsHandle(job)
		return err
	}
	if attachErr != nil {
		closeWindowsHandle(job)
		return attachErr
	}
	jobs.Store(command.Process.Pid, job)
	return nil
}

func releaseProcessTree(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	if job, ok := jobs.LoadAndDelete(command.Process.Pid); ok {
		closeWindowsHandle(job.(uintptr))
	}
}

func trackedProcessTrees() int {
	count := 0
	jobs.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

func killProcessTree(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	if job, ok := jobs.LoadAndDelete(command.Process.Pid); ok {
		closeWindowsHandle(job.(uintptr))
		return
	}
	_ = command.Process.Kill()
}

func waitDescendantReaped(pid int, timeout time.Duration) bool {
	handle, _, openErr := openProcess.Call(0x00100000, 0, uintptr(pid)) // SYNCHRONIZE
	if handle == 0 {
		return processOpenMeansReaped(handle, openErr)
	}
	defer closeWindowsHandle(handle)
	wait, _, _ := waitForSingleObject.Call(handle, uintptr(timeout.Milliseconds()))
	return wait == 0
}

func processOpenMeansReaped(handle uintptr, err error) bool {
	return handle == 0 && errors.Is(err, errorInvalidParameter)
}

func closeWindowsHandle(handle uintptr) { _, _, _ = closeHandle.Call(handle) }

func TestWindowsJobObjectABI(t *testing.T) {
	command := &exec.Cmd{}
	configureProcessGroup(command)
	if command.SysProcAttr == nil || command.SysProcAttr.CreationFlags&createSuspended == 0 {
		t.Fatal("process was not configured for pre-assignment suspension")
	}
	pointerBytes := unsafe.Sizeof(uintptr(0))
	wantBasic, wantExtended := uintptr(64), uintptr(144)
	if pointerBytes == 4 {
		wantBasic, wantExtended = 48, 112
	}
	if got := unsafe.Sizeof(jobObjectBasicLimitInformation{}); got != wantBasic {
		t.Fatalf("basic bytes=%d want=%d", got, wantBasic)
	}
	if got := unsafe.Sizeof(ioCounters{}); got != 48 {
		t.Fatalf("io counters bytes=%d", got)
	}
	if got := unsafe.Offsetof(jobExtendedLimitInformation{}.IoInfo); got != wantBasic {
		t.Fatalf("io offset=%d want=%d", got, wantBasic)
	}
	if got := unsafe.Sizeof(jobExtendedLimitInformation{}); got != wantExtended {
		t.Fatalf("extended bytes=%d want=%d", got, wantExtended)
	}
}

func TestWindowsOpenProcessErrorsAreNotReaped(t *testing.T) {
	if processOpenMeansReaped(0, errorAccessDenied) {
		t.Fatal("access denied was classified as reaped")
	}
	if !processOpenMeansReaped(0, errorInvalidParameter) {
		t.Fatal("absent process was not classified as reaped")
	}
	if processOpenMeansReaped(1, errorInvalidParameter) {
		t.Fatal("live handle was classified as reaped")
	}
}
