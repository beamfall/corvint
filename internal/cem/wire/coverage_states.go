package wire

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
)

// CoverageStatesProfile names the additive coverage-states document of
// CAX-V0-010. A cem/1.0 map keeps its closed two-state coverage witness
// (TCQ-V0-052); this separate document, bound to the exact map bytes, is
// where a third state, not-instrumented, is recorded. A cem/1.0 consumer that
// never reads it loses nothing it relied on.
const CoverageStatesProfile = "cem-coverage-states/1"

// CoverageNotInstrumented is the coverage-states value for an uncovered hunk
// whose path no block of the coverprofile names at all (CAX-V0-010). It is
// never a cem/1.0 witness state.
const CoverageNotInstrumented = "not-instrumented"

// CoverageStates is one cem-coverage-states/1 document. Members are in
// canonical (sorted) order so EncodeCoverageStates is deterministic.
type CoverageStates struct {
	Hunks         []CoverageStateHunk `json:"hunks"`
	MapSha256     string              `json:"mapSha256"`
	Profile       string              `json:"profile"`
	ProfileSha256 string              `json:"profileSha256"`
	TestRun       string              `json:"testRun"`
}

// CoverageStateHunk names one map hunk, in map order, and its state.
type CoverageStateHunk struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// EncodeCoverageStates writes the canonical bytes: sorted members, no HTML
// escaping, no insignificant whitespace, one trailing newline.
func EncodeCoverageStates(states CoverageStates) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(states); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// ValidateCoverageStates checks a cem-coverage-states/1 document against the
// exact cem/1.0 map bytes it names (CAX-V0-011): closed, canonical bytes; the
// profile name; mapSha256 equal to the map's SHA-256; one entry per map hunk
// with the same id in the same order; the map's witness on every hunk with the
// document's profileSha256 and testRun; and each state agreeing with that
// witness, so covered is only covered and uncovered is uncovered or
// not-instrumented. Any failure refuses the whole document.
func ValidateCoverageStates(document, mapBytes []byte) (*CoverageStates, error) {
	var states CoverageStates
	if err := jsonv2.Unmarshal(document, &states, jsonv2.RejectUnknownMembers(true)); err != nil {
		return nil, fmt.Errorf("coverage states: %w", err)
	}
	canonical, err := EncodeCoverageStates(states)
	if err != nil || !bytes.Equal(canonical, document) {
		return nil, errors.New("coverage states: document is not in canonical form or lacks a member")
	}
	if states.Profile != CoverageStatesProfile {
		return nil, fmt.Errorf("coverage states: profile %q is not %s", states.Profile, CoverageStatesProfile)
	}
	sum := sha256.Sum256(mapBytes)
	if states.MapSha256 != hex.EncodeToString(sum[:]) {
		return nil, errors.New("coverage states: mapSha256 does not name the supplied map bytes")
	}
	if len(states.ProfileSha256) != 64 || !isLowerHex(states.ProfileSha256) {
		return nil, errors.New("coverage states: profileSha256 must be 64 lowercase hex characters")
	}
	if err := ValidateTestRun(states.TestRun); err != nil {
		return nil, fmt.Errorf("coverage states: %w", err)
	}
	stable, err := ParseStable(mapBytes)
	if err != nil {
		return nil, fmt.Errorf("coverage states: the named map is not cem/1.0: %w", err)
	}
	hunks := stable.Change.Hunks
	if len(states.Hunks) != len(hunks) {
		return nil, fmt.Errorf("coverage states: %d entries for %d map hunks", len(states.Hunks), len(hunks))
	}
	for index, entry := range states.Hunks {
		hunk := hunks[index]
		if entry.ID != hunk.ID {
			return nil, fmt.Errorf("coverage states: entry %d names %q, the map hunk is %q", index, entry.ID, hunk.ID)
		}
		witness := hunk.Coverage
		if witness == nil || witness.ProfileSha256 != states.ProfileSha256 || witness.TestRun != states.TestRun {
			return nil, fmt.Errorf("coverage states: hunk %s has no witness from this profile and test run", hunk.ID)
		}
		agrees := entry.State == witness.State ||
			(entry.State == CoverageNotInstrumented && witness.State == CoverageUncovered)
		if !agrees {
			return nil, fmt.Errorf("coverage states: hunk %s state %q disagrees with its %q witness", hunk.ID, entry.State, witness.State)
		}
	}
	return &states, nil
}
