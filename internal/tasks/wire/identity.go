package wire

import (
	"crypto/sha256"
	"encoding/hex"
)

// Sum returns the SHA-256 of raw bytes as a Digest. A file's chain digest is
// the digest of its complete raw bytes, trailing LF included (§2).
func Sum(raw []byte) Digest {
	h := sha256.Sum256(raw)
	return Digest(hex.EncodeToString(h[:]))
}

// ContentID computes the WQO §4.3 content identity
// SHA-256(kind || 0x00 || profile || 0x00 || canonicalBody) where the body is
// the canonical encoding without the trailing LF.
func ContentID(kind, profile string, canonicalBody []byte) Digest {
	h := sha256.New()
	h.Write([]byte(kind))
	h.Write([]byte{0})
	h.Write([]byte(profile))
	h.Write([]byte{0})
	h.Write(canonicalBody)
	return Digest(hex.EncodeToString(h.Sum(nil)))
}

// RecordIdentity renders `<kind>:sha256:<Digest>`.
func RecordIdentity(kind string, d Digest) string {
	return kind + ":sha256:" + string(d)
}
