package stepnegation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// TestKey is the canonical test identity: the worktree-relative spec file,
// the exact full title (describe titles and test title joined by " > "), and
// the project. It names the retained file and seeds every marker.
type TestKey struct {
	File      string `json:"file"`
	FullTitle string `json:"fullTitle"`
	Project   string `json:"project"`
}

// Digest is the SHA-256 of the key's canonical JSON encoding.
func (key TestKey) Digest() string {
	data, _ := json.Marshal(key)
	return digestHex(data)
}

// Key returns the canonical key of a binding's test identity.
func (binding TestBinding) Key() TestKey {
	return TestKey{File: binding.File, FullTitle: binding.FullTitle, Project: binding.Project}
}

// Marker derives the deterministic fault marker of LPCV-V0-061: the prefix
// corvint-fault- plus the first 8 lowercase hex digits of the SHA-256 of the
// canonical test identity, step title, witnessed assertion location and fault
// kind. It never depends on the plan, whose digest is computed afterwards.
func Marker(key TestKey, stepTitle, assertionLocation, kind string) string {
	data, _ := json.Marshal(struct {
		Test      TestKey `json:"test"`
		Step      string  `json:"step"`
		Assertion string  `json:"assertion"`
		Kind      string  `json:"kind"`
	}{key, stepTitle, assertionLocation, kind})
	return "corvint-fault-" + digestHex(data)[:8]
}

// Screen returns a retained-safe string: the value itself, or its SHA-256
// when internal/secretscreen flags it (LPCV-V0-069).
func Screen(value string) string {
	if secretscreen.MatchString(value) {
		return "sha256:" + digestHex([]byte(value))
	}
	return value
}

func digestHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
