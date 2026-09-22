package parentverify

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

func bareID(kind, profile string, body []byte) string {
	digest := sha256.New()
	var length32 [4]byte
	var length64 [8]byte
	binary.BigEndian.PutUint32(length32[:], uint32(len(kind)))
	_, _ = digest.Write(length32[:])
	_, _ = digest.Write([]byte(kind))
	binary.BigEndian.PutUint32(length32[:], uint32(len(profile)))
	_, _ = digest.Write(length32[:])
	_, _ = digest.Write([]byte(profile))
	binary.BigEndian.PutUint64(length64[:], uint64(len(body)))
	_, _ = digest.Write(length64[:])
	_, _ = digest.Write(body)
	return hex.EncodeToString(digest.Sum(nil))
}

func prefixedID(kind, profile string, body []byte) string {
	return kind + ":sha256:" + bareID(kind, profile, body)
}
