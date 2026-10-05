// SPDX-License-Identifier: AGPL-3.0-or-later

// Package procfs is the Linux procfs raw tuple reader for post-merge /2
// process proofs (PMR-V2-006). It returns exact bracketed bytes only; it
// derives no role, owner, absence or acceptance fact. The offline verifier in
// package postmergeproof binds those bytes to logical nodes.
package procfs

// BirthCapture is one bracketed observation of a process birth: boot and PID
// namespace identity, paired stat bytes around the argv, executable link and
// executable bytes, and the native second-granularity start output.
type BirthCapture struct {
	BootID              []byte
	NamespaceLink       []byte
	NamespaceDevice     uint64
	NamespaceInode      uint64
	StatBefore          []byte
	Cmdline             []byte
	CmdlineAfter        []byte
	ExecutableLink      []byte
	ExecutableLinkAfter []byte
	// Executable holds the full executable bytes read through
	// /proc/PID/exe; a second read is summarized by its digest and length.
	Executable            []byte
	ExecutableAfterSHA256 string
	ExecutableAfterBytes  int64
	StatAfter             []byte
	// NativeStartOutput is the exact `ps -o lstart=` stdout under LC_ALL=C and
	// TZ=UTC; NativeStartTool holds that tool's executable bytes.
	NativeStartOutput []byte
	NativeStartTool   []byte
}

// SweptProcess is one row of a raw /proc PID inventory.
type SweptProcess struct {
	PID  int
	Stat []byte
}

// Sweep is a raw /proc PID inventory. PIDDirectory lists every numeric /proc
// entry as `PID\n` in directory order; rows whose stat read failed are listed
// in ReadFailures and omitted from Processes.
type Sweep struct {
	BootID        []byte
	NamespaceLink []byte
	PIDDirectory  []byte
	Processes     []SweptProcess
	ReadFailures  []string
}

// UnsupportedError reports a host without the Linux amd64/arm64 procfs
// profile. Collection there is NOT_OBSERVED, never an empty observation.
type UnsupportedError struct{}

func (UnsupportedError) Error() string {
	return "postmerge process observation NOT_OBSERVED: process-observation-unsupported"
}

// Outcome is the collection outcome class of an unsupported host.
func (UnsupportedError) Outcome() string { return "NOT_OBSERVED" }

// Code is the stable refusal code of an unsupported host.
func (UnsupportedError) Code() string { return "process-observation-unsupported" }
