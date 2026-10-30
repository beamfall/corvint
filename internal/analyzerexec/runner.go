// Package analyzerexec launches one digest-pinned staged native analyzer.
// It accepts no command string, PATH lookup, inherited environment, repository
// working directory, network API, or retry policy.
package analyzerexec

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"time"
)

const MaxExecutableBytes = 64 << 20

type Failure string

const (
	IdentityUnsafe Failure = "identity-unsafe"
	DigestMismatch Failure = "digest-mismatch"
	Race           Failure = "race"
	Unsupported    Failure = "unsupported"
	Limit          Failure = "limit"
	Timeout        Failure = "timeout"
	Cancelled      Failure = "cancelled"
	Process        Failure = "process"
)

type Error struct{ Failure Failure }

func (e *Error) Error() string { return string(e.Failure) }

func Is(err error, want Failure) bool {
	var failure *Error
	return errors.As(err, &failure) && failure.Failure == want
}

type Plan struct {
	// Artifact is the owner-private containing release artifact. Its bytes are
	// independently pinned to the registry's ArtifactDigest.
	Artifact               string
	ExpectedArtifactSHA256 string
	// Executable is the staged native executable. Its bytes are independently
	// pinned to the selected release's HostBinaryDigest.
	Executable               string
	ExpectedExecutableSHA256 string
	// Host is the signed release tuple for the executable. Target is the
	// separately resolved compilation-unit tuple carried by the request. The
	// runner validates both as closed tuples and never infers either from the
	// other.
	Host   NativePlatform
	Target NativePlatform
	// InvocationBindingSHA256 binds the exact request bytes to both tuples so
	// no adapter can substitute a target between Core resolution and launch.
	InvocationBindingSHA256 string
	// RepositoryRoot and StagingParent are Core-owned roots. Neither originates
	// in an analyzer request, lock, profile, or the ambient environment.
	RepositoryRoot string
	StagingParent  string
	Request        []byte
	Timeout        time.Duration
	MaxStdoutBytes int
	MaxStderrBytes int
	MemoryBytes    uint64
	MaxChildren    uint64
}

func InvocationBindingSHA256(request []byte, host, target NativePlatform) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte("corvint/analyzerexec/invocation/v1"))
	var size [8]byte
	write := func(value []byte) {
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write(value)
	}
	write(request)
	for _, value := range []string{host.OS, host.Architecture, host.ABI, target.OS, target.Architecture, target.ABI} {
		write([]byte(value))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

type Result struct {
	Stdout       []byte
	Stderr       []byte
	Started      bool
	Completed    bool
	Elapsed      time.Duration
	Cleanup      time.Duration
	CleanupState CleanupObservation
	Termination  Termination
	CleanupError CleanupError
}

// CleanupObservation distinguishes an observed zero-duration cleanup from a
// path which never launched and therefore has no cleanup measurement.
type CleanupObservation string

const (
	CleanupNotRun   CleanupObservation = "NOT_RUN"
	CleanupObserved CleanupObservation = "OBSERVED"
	CleanupFailed   CleanupObservation = "FAILED"
	// CleanupRejected records that Core rejected a malformed started-backend
	// observation. It is receipt-only: this state itself proves a process did
	// start and is not a valid backend cleanup observation.
	CleanupRejected CleanupObservation = "REJECTED"
)

// Termination is a closed, path- and error-free account of the launched
// process. It is carried into Core's canonical terminal receipt.
type Termination string

const (
	TerminationNotRun    Termination = "NOT_RUN"
	TerminationExited    Termination = "EXITED"
	TerminationTimedOut  Termination = "TIMED_OUT"
	TerminationCancelled Termination = "CANCELLED"
	TerminationFailed    Termination = "FAILED"
)

// CleanupError retains only the operation class. Raw platform errors and
// staging paths are deliberately never returned to Core receipts.
type CleanupError string

const (
	CleanupErrorNone   CleanupError = "NONE"
	CleanupErrorClose  CleanupError = "CLOSE"
	CleanupErrorRemove CleanupError = "REMOVE"
)

func (r Result) ValidCleanupObservation() bool {
	if r.Elapsed < 0 || r.Cleanup < 0 {
		return false
	}
	switch r.Termination {
	case TerminationNotRun, TerminationExited, TerminationTimedOut, TerminationCancelled, TerminationFailed:
	default:
		return false
	}
	switch r.CleanupError {
	case CleanupErrorNone, CleanupErrorClose, CleanupErrorRemove:
	default:
		return false
	}
	if !r.Started {
		if r.Completed || r.Termination != TerminationNotRun {
			return false
		}
		if r.CleanupState == CleanupNotRun {
			return r.Cleanup == 0 && r.CleanupError == CleanupErrorNone
		}
		return r.CleanupState == CleanupFailed && r.CleanupError != CleanupErrorNone
	}
	if !r.Completed || r.Termination == TerminationNotRun {
		return false
	}
	return r.CleanupState == CleanupObserved && r.CleanupError == CleanupErrorNone || r.CleanupState == CleanupFailed && r.CleanupError != CleanupErrorNone
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
