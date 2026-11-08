// SPDX-License-Identifier: Apache-2.0
package testevidence

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash"
	"sort"
	"strings"
)

// Snapshot keys both names and repositories; missing entries are unknown, not
// evidence of unchanged content. Executable digests are independently checked.
type PathKey struct{ Repository, Path string }
type Snapshot struct {
	Blobs       map[PathKey]string
	Executables map[PathKey]string
	// ChangedPins names sibling pins that moved since collection (RSP-V0-049).
	ChangedPins map[string]bool
}
type Freshness struct {
	Fresh  bool
	Reason string
}

// Fresh checks immutable content and exact runner/execution identities
// (RSP-V0-010, RSP-V0-053). It is not a selection or admission decision: absent
// components must still widen in the consumer, and diff drift rules still apply.
func (r Record) Fresh(runner RunnerIdentity, execution ExecutionIdentity, snapshot Snapshot) Freshness {
	stale := func(reason string) Freshness { return Freshness{Reason: reason} }
	if err := r.Validate(); err != nil {
		return stale(err.Error())
	}
	if r.Runner != runner {
		return stale("runner-identity-drift")
	}
	if r.Execution != execution {
		return stale("execution-identity-drift")
	}
	check := func(b Blob) string {
		if snapshot.ChangedPins[b.Path.Repository] {
			return "sibling-pin-drift"
		}
		if snapshot.Blobs[PathKey{b.Path.Repository, b.Path.Resolved}] != b.OID {
			return "blob-drift-or-missing"
		}
		if b.Path.Lexical != b.Path.Resolved && snapshot.Blobs[PathKey{b.Path.Repository, b.Path.Lexical}] != b.LexicalOID {
			return "link-drift-or-missing"
		}
		return ""
	}
	var blobs []Blob
	for _, f := range r.Executed.Items {
		blobs = append(blobs, f.Blob)
	}
	for _, f := range r.Regions.Items {
		blobs = append(blobs, f.Blob)
	}
	blobs = append(blobs, r.Loaded.Items...)
	blobs = append(blobs, r.Reads.Items...)
	for _, e := range r.Execs.Items {
		blobs = append(blobs, e.Blob)
		if snapshot.Executables[PathKey{e.Blob.Path.Repository, e.Blob.Path.Resolved}] != e.Digest {
			return stale("executable-digest-drift-or-missing")
		}
	}
	for _, b := range blobs {
		if reason := check(b); reason != "" {
			return stale(reason)
		}
	}
	// RSP-V0-049: probes and listings can be the only edges into a sibling.
	for _, paths := range [][]Path{r.Listings.Items, r.Lookups.Items} {
		for _, p := range paths {
			if snapshot.ChangedPins[p.Repository] {
				return stale("sibling-pin-drift")
			}
		}
	}
	for _, w := range r.Writes.Items {
		if snapshot.ChangedPins[w.Path.Repository] {
			return stale("sibling-pin-drift")
		}
	}
	return Freshness{Fresh: true}
}

// EnvironmentDigest uses HMAC-SHA256 for each value and for the complete set
// (RSP-V0-058). Even an empty environment changes when the key rotates. The core
// supplies its versioned non-semantic list; a nil value is uncapturable.
func EnvironmentDigest(key []byte, environment map[string]*string, nonSemantic []string) (string, error) {
	if len(key) < 32 {
		return "", errors.New("environment-key-missing-or-short")
	}
	ignored := map[string]bool{}
	for _, name := range nonSemantic {
		if name == "" || strings.ContainsAny(name, "=\x00") {
			return "", errors.New("environment-exclusion-invalid")
		}
		ignored[name] = true
	}
	names := make([]string, 0, len(environment))
	for name, value := range environment {
		if name == "" || strings.ContainsAny(name, "=\x00") {
			return "", errors.New("environment-name-invalid")
		}
		if ignored[name] {
			continue
		}
		if value == nil {
			return "", errors.New("environment-value-uncapturable")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	h := hmac.New(sha256.New, key)
	framed(h, []byte("corvint-test-evidence/1/environment"))
	for _, name := range names {
		v := hmac.New(sha256.New, key)
		framed(v, []byte("value"))
		framed(v, []byte(name))
		framed(v, []byte(*environment[name]))
		framed(h, []byte(name))
		framed(h, v.Sum(nil))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func framed(h hash.Hash, b []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(b)))
	h.Write(size[:])
	h.Write(b)
}
