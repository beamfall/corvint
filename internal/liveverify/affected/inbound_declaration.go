// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"bytes"
	json "encoding/json/v2"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ShellSuffixes are the script endings admitted by the shell caller contract.
var ShellSuffixes = []string{".sh", ".bash", ".zsh", ".ksh", ".bats"}

// ShellForeignCallersProfile is the declaration's closed profile name.
const ShellForeignCallersProfile = "corvint-shell-foreign-callers/0"

const (
	MaxShellForeignCallerBytes = 64 << 10
	maxForeignCallerScripts    = 64
	maxForeignCallers          = 256
	// maxForeignCallerReason is 1024 bytes (V1-4062, TSP-V0-820): the
	// longest declared reason reached 512 bytes with 24 files.
	maxForeignCallerReason = 1024
)

// ShellForeignCallerEntry is one declared script: every file naming it, split into
// the callers that run it and the mentions that only name it, and the
// project's stated reason.
type ShellForeignCallerEntry struct {
	Callers  []string `json:"callers"`
	Mentions []string `json:"mentions,omitempty"`
	Reason   string   `json:"reason"`
}

type foreignCallersFile struct {
	Profile string                             `json:"profile"`
	Scripts map[string]ShellForeignCallerEntry `json:"scripts"`
}

// ParseShellForeignCallers applies the canonical shell declaration validation
// to immutable bytes. Revision closure and the shell reader share this contract;
// tree completeness remains the shell plugin's responsibility. AZU-V0-8904.
func ParseShellForeignCallers(body []byte) (map[string]ShellForeignCallerEntry, error) {
	var declared foreignCallersFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if err := ValidateShellForeignCallers(declared.Profile, declared.Scripts); err != nil {
		return nil, err
	}
	return declared.Scripts, nil
}

// ValidateShellForeignCallers is the canonical member validation for current
// and historical declarations. The shell reader retains its original decoder
// types because JSON semantic errors include those type names. AZU-V0-8904.
func ValidateShellForeignCallers(profile string, scripts map[string]ShellForeignCallerEntry) error {
	if profile != ShellForeignCallersProfile || len(scripts) == 0 || len(scripts) > maxForeignCallerScripts {
		return fmt.Errorf("%s: profile must be %s with 1 to %d scripts", ShellForeignCallersPath, ShellForeignCallersProfile, maxForeignCallerScripts)
	}
	for script, entry := range scripts {
		if !ValidRelativePath(script) || !slices.ContainsFunc(ShellSuffixes, func(suffix string) bool { return strings.HasSuffix(script, suffix) }) {
			return fmt.Errorf("%s: script %q", ShellForeignCallersPath, script)
		}
		named := append(slices.Clone(entry.Callers), entry.Mentions...)
		if len(named) == 0 || len(named) > maxForeignCallers || !shellCallerSortedUnique(entry.Callers) || !shellCallerSortedUnique(entry.Mentions) {
			return fmt.Errorf("%s: script %q callers and mentions must be 1 to %d sorted unique paths", ShellForeignCallersPath, script, maxForeignCallers)
		}
		for _, file := range named {
			if !ValidRelativePath(file) || file == script || slices.Contains(entry.Callers, file) && slices.Contains(entry.Mentions, file) {
				return fmt.Errorf("%s: script %q file %q", ShellForeignCallersPath, script, file)
			}
		}
		if entry.Reason == "" || len(entry.Reason) > maxForeignCallerReason || !utf8.ValidString(entry.Reason) || strings.ContainsFunc(entry.Reason, unicode.IsControl) {
			return fmt.Errorf("%s: script %q reason has %d bytes; want non-empty valid UTF-8 without control characters, at most %d bytes", ShellForeignCallersPath, script, len(entry.Reason), maxForeignCallerReason)
		}
	}
	return nil
}

func shellCallerSortedUnique(paths []string) bool {
	return slices.IsSorted(paths) && len(slices.Compact(slices.Clone(paths))) == len(paths)
}
