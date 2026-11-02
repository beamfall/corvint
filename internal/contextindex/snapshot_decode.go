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
// of the whole file (V1-2237). The trailer hash of the messages and footer
// runs on its own goroutine while the messages decode, and neither side
// copies the file through read(2). Gob copies every message out of its
// reader and each region body is copied out under the guard, so nothing
// decoded aliases the mapping. The mapping is released once both sides
// return, unless the decode handed out lazy bodies (IDX-SNAP-V0-811), which
// then hold it through bodies.lease.
func decodeMappedSnapshotValue(mapping *snapshotMapping, identity repositoryIdentity, engineID string, bodies *snapshotBodies, decode func(*gob.Decoder) error) error {
	defer mapping.release()
	data := mapping.data()
	var messages, regionStart int64
	footerAt := int64(len(data)) - sha256.Size - snapshotFooterBytes
	if err := guardMappedRead(func() (err error) {
		messages, regionStart, err = snapshotLayout(int64(len(data)), data[footerAt:footerAt+snapshotFooterBytes])
		return err
	}); err != nil {
		return err
	}
	bodies.region, bodies.mapping, bodies.messageBytes = data[regionStart:footerAt], mapping, messages
	digests := make(chan bool, 1)
	go func() {
		matched := false
		defer func() { digests <- matched }()
		_ = guardMappedRead(func() error {
			digest := sha256.New()
			digest.Write(data[:messages])
			digest.Write(data[footerAt : footerAt+snapshotFooterBytes])
			matched = bytes.Equal(digest.Sum(nil), data[len(data)-sha256.Size:])
			return nil
		})
	}()
	decodeErr := guardMappedRead(func() error {
		decoder := gob.NewDecoder(bytes.NewReader(data[:messages]))
		if err := decodeSnapshotHeader(decoder, identity, engineID); err != nil {
			return err
		}
		return decode(decoder)
	})
	// Join the hash before the deferred release, on every path.
	matched := <-digests
	if decodeErr != nil {
		bodies.lazy = nil
		return decodeErr
	}
	if !matched {
		bodies.lazy = nil
		return errors.New("snapshot digest mismatch")
	}
	if bodies.lazy != nil {
		bodies.lease = mapping.lease()
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
