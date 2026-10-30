// Package sessiontest is the planning-independent half of `corvint test`
// (docs/specs/session-test-runner-v0.md): the declared full scope, the verdict
// and the corvint-test-manifest/0 it derives, the Go and declared-command
// adapters, the shard execution that writes a corvint-test-run/0 receipt, and
// the verify-shards fan-in. The plan itself comes from the caller, which
// compiles it through the same code as `corvint affected --base`.
package sessiontest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/gokernel"
)

const (
	// ManifestProfile names the STR-V0-018 expected-run document.
	ManifestProfile = "corvint-test-manifest/0"
	// ReceiptProfile names the STR-V0-010 per-shard receipt.
	ReceiptProfile = "corvint-test-run/0"
	// MaxUnits bounds the execution units of one run (STR-V0-015).
	MaxUnits = 4096
	// MaxDocumentBytes bounds a manifest and a receipt (STR-V0-010, STR-V0-018).
	MaxDocumentBytes = 1 << 20

	ScopeNarrowed = "NARROWED"
	ScopeFull     = "FULL"
	ScopeNothing  = "NOTHING"

	ResultPass       = "PASS"
	ResultFail       = "FAIL"
	ResultIncomplete = "INCOMPLETE"

	// PlacementUniverse is the default complete-universe placement
	// (STR-V0-007).
	PlacementUniverse = "universe"
	// PlacementBalanced partitions a narrowed run set's own execution units
	// by the advisory cost table (STR-V0-008, experimental). It executes only
	// in the local profile; FULL keeps the universe partition.
	PlacementBalanced = "balanced"
	// PlacementMeasured places every Go package of the run set whole on
	// the shard a corvint-test-shards/0 plan assigns it (STR-V0-054,
	// experimental). It executes only in the local profile.
	PlacementMeasured = "measured"

	// SliceWhole names an unsplit unit; a split one is "<index>/<count>".
	SliceWhole = "whole"
	// SliceSelected names a unit narrowed to the test functions of its own
	// changed test files (STR-V0-063): Flag is -run and Pattern anchors them.
	SliceSelected = "selected"

	AdapterGo       = "go"
	AdapterDeclared = "declared"

	ProfileLocal = "local"
	ProfileCI    = "ci"
)

// PathDigest binds one dirty path: "sha256:<hex>" of a regular file,
// "symlink:sha256:<hex>" of a link target, "directory", or "deleted".
type PathDigest struct {
	Digest string `json:"digest"`
	Path   string `json:"path"`
}

// SourceIdentity is the STR-V0-017 identity of the bytes that execute.
type SourceIdentity struct {
	Head  string       `json:"head"`
	Paths []PathDigest `json:"paths"`
	Tree  string       `json:"tree"`
}

// Unit is one execution unit and its shard (STR-V0-002, STR-V0-007).
type Unit struct {
	Adapter string `json:"adapter"`
	// Dir is set only on a nested-module unit (STR-V0-064): the
	// repository-relative module directory its `go test ./...` runs from.
	Dir     string `json:"dir,omitempty"`
	Flag    string `json:"flag,omitempty"`
	Pattern string `json:"pattern,omitempty"`
	// Reason says why a selected Go unit runs whole or as a selected slice
	// (STR-V0-063); a FULL run and a declared unit carry none.
	Reason string `json:"reason,omitempty"`
	Shard  int    `json:"shard"`
	Slice  string `json:"slice"`
	Unit   string `json:"unit"`
}

// Key is the canonical (unit, slice) pair.
func (u Unit) Key() string { return u.Unit + " " + u.Slice }

// Manifest is the STR-V0-018 corvint-test-manifest/0: everything every shard
// and the fan-in must agree on before anything executes.
type Manifest struct {
	Base           string   `json:"base"`
	Checks         []string `json:"checks"`
	DeclaredDigest string   `json:"declaredDigest"`
	// Excluded records each graph Go unit left out of the root Go universe,
	// as "<unit>: <reason>"; nothing runs for it (STR-V0-069). It is absent
	// when nothing is excluded, so such a manifest keeps its bytes.
	Excluded    []string `json:"excluded,omitempty"`
	GraphDigest string   `json:"graphDigest"`
	Incomplete  []string `json:"incomplete"`
	// NoSuite records each covered language the declaration asserts has no
	// suite, as "<language>: <reason>"; nothing runs for it (STR-V0-004).
	NoSuite         []string       `json:"noSuite"`
	PartitionDigest string         `json:"partitionDigest"`
	Placement       string         `json:"placement"`
	PlanDigest      string         `json:"planDigest"`
	Profile         string         `json:"profile"`
	Reason          string         `json:"reason"`
	RunnerProfile   string         `json:"runnerProfile"`
	Scope           string         `json:"scope"`
	ShardCount      int            `json:"shardCount"`
	Source          SourceIdentity `json:"source"`
	SourceDigest    string         `json:"sourceDigest"`
	Units           []Unit         `json:"units"`
}

// Digest is "sha256:<hex>" of v's canonical JSON.
func Digest(v any) (string, error) {
	encoded, err := gokernel.CanonicalJSON(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// Encode returns the canonical bytes of a manifest or receipt plus a newline,
// refusing a document over MaxDocumentBytes.
func Encode(v any) ([]byte, error) {
	return encodeBounded(v, MaxDocumentBytes)
}

func encodeBounded(v any, limit int) ([]byte, error) {
	encoded, err := gokernel.CanonicalJSON(v)
	if err != nil {
		return nil, err
	}
	if len(encoded)+1 > limit {
		return nil, fmt.Errorf("document exceeds %d bytes", limit)
	}
	return append(encoded, '\n'), nil
}

// decodeStrict reads one canonical document of the named profile: bounded,
// no unknown members, nothing trailing, and byte-identical to its own
// canonical encoding, so its digest has exactly one spelling.
func decodeStrict(name, profile string, v any, profileOf func() string) error {
	return decodeStrictBounded(name, profile, MaxDocumentBytes, v, profileOf)
}

func decodeStrictBounded(name, profile string, limit int, v any, profileOf func() string) error {
	raw, err := readRegular(name, limit)
	if err != nil {
		return err
	}
	return decodeStrictBytes(name, profile, limit, raw, v, profileOf)
}

// readRegular reads at most limit+1 bytes of a regular file. The open neither
// follows a final symlink nor blocks on a FIFO, and the descriptor is checked
// before any read (V1-1635); the caller enforces limit.
func readRegular(name string, limit int) ([]byte, error) {
	file, err := openInput(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", filepath.Base(name))
	}
	return io.ReadAll(io.LimitReader(file, int64(limit)+1))
}

// decodeStrictBytes is decodeStrictBounded over bytes a caller already read
// through a bounded, non-blocking open (STR-V0-055).
func decodeStrictBytes(name, profile string, limit int, raw []byte, v any, profileOf func() string) error {
	if len(raw) > limit {
		return fmt.Errorf("%s exceeds %d bytes", filepath.Base(name), limit)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("%s is not a %s document: %w", filepath.Base(name), profile, err)
	}
	if decoder.More() {
		return fmt.Errorf("%s has trailing data", filepath.Base(name))
	}
	if profileOf() != profile {
		return fmt.Errorf("%s profile is %q, want %q", filepath.Base(name), profileOf(), profile)
	}
	canonical, err := encodeBounded(v, limit)
	if err != nil || !bytes.Equal(canonical, raw) {
		return fmt.Errorf("%s is not canonical %s JSON", filepath.Base(name), profile)
	}
	return nil
}

// ReadManifest reads one canonical manifest and returns it with its digest.
func ReadManifest(name string) (Manifest, string, error) {
	var manifest Manifest
	if err := decodeStrict(name, ManifestProfile, &manifest, func() string { return manifest.Profile }); err != nil {
		return Manifest{}, "", err
	}
	digest, err := Digest(manifest)
	return manifest, digest, err
}

// ReadReceipt reads one canonical receipt.
func ReadReceipt(name string) (Receipt, error) {
	var receipt Receipt
	err := decodeStrict(name, ReceiptProfile, &receipt, func() string { return receipt.Profile })
	return receipt, err
}

// WriteFile replaces name with data through a sibling temporary file, so a
// reader sees the old document or the whole new one.
func WriteFile(name string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(name), "."+filepath.Base(name)+".*")
	if err != nil {
		return err
	}
	_, writeErr := temporary.Write(data)
	closeErr := temporary.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	if err := os.Rename(temporary.Name(), name); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	return nil
}

// SlicedUnits counts the units narrowed to named test functions
// (STR-V0-063).
func (m Manifest) SlicedUnits() int {
	count := 0
	for _, unit := range m.Units {
		if unit.Slice == SliceSelected {
			count++
		}
	}
	return count
}
