package contextindex

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"errors"
	"runtime/debug"
)

// errSnapshotFault is a fault while reading a mapped snapshot, such as a file
// truncated under the mapping; the read is the IDX-SNAP-V0-003 miss.
var errSnapshotFault = errors.New("snapshot mapping fault")

// decodeMappedSnapshotValue is decodeSnapshotValue over a read-only mapping
// of the whole file (V1-2237). It reads the same bytes and makes the same
// header, message and SHA-256 trailer checks, in the same on-disk format;
// only the order of work differs. The trailer hash runs on its own goroutine
// over the mapping while the gob message is decoded, so the hash leaves the
// load's critical path, and neither side copies the file through read(2).
// The mapping is released only after both sides return, and every decoded
// value is a copy (gob copies a message out of its reader before decoding
// it), so nothing returned aliases the mapping.
func decodeMappedSnapshotValue(mapping []byte, identity repositoryIdentity, engineID string, value any) error {
	defer unmapReadOnly(mapping)
	payload := mapping[:len(mapping)-sha256.Size]
	digests := make(chan bool, 1)
	go func() {
		matched := false
		defer func() { digests <- matched }()
		_ = guardMappedRead(func() error {
			sum := sha256.Sum256(payload)
			matched = bytes.Equal(sum[:], mapping[len(payload):])
			return nil
		})
	}()
	decodeErr := guardMappedRead(func() error {
		decoder := gob.NewDecoder(bytes.NewReader(payload))
		if err := decodeSnapshotHeader(decoder, identity, engineID); err != nil {
			return err
		}
		return decoder.Decode(value)
	})
	// Join the hash before the deferred unmap, on every path.
	matched := <-digests
	if decodeErr != nil {
		return decodeErr
	}
	if !matched {
		return errors.New("snapshot digest mismatch")
	}
	return nil
}

// guardMappedRead runs read with memory faults turned into errSnapshotFault
// on this goroutine. Any other panic is re-raised unchanged.
func guardMappedRead(read func() error) (err error) {
	defer debug.SetPanicOnFault(debug.SetPanicOnFault(true))
	defer func() {
		if recovered := recover(); recovered != nil {
			if _, fault := recovered.(interface{ Addr() uintptr }); fault {
				err = errSnapshotFault
				return
			}
			panic(recovered)
		}
	}()
	return read()
}

// guardMapped is guardMappedRead for a read that returns a value; a fault
// returns the zero value with errSnapshotFault (V1-3005).
func guardMapped[T any](read func() (T, error)) (T, error) {
	var value T
	err := guardMappedRead(func() (err error) {
		value, err = read()
		return err
	})
	if errors.Is(err, errSnapshotFault) {
		var zero T
		return zero, err
	}
	return value, err
}
