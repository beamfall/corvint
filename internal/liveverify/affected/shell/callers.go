package shell

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// This file reads and verifies the project's shell foreign-caller
// declaration (TSP-V0-005). A declared script's foreign callers are the files
// that run it by a path the plugin cannot resolve to an edge; its mentions are
// the files that name it without running it (a transcript pattern, prose in a
// comment). The listing is verified, not trusted: it holds only while every
// candidate file that names the script's file name is listed as one or the
// other and every listed file names it. Only the caller/mention split is the
// project's own claim, made per file with the script's reason. A verified
// script then carries its callers (affected.Unit.Callers), so the graph makes
// their units its dependents, and FrontierForeignInvocation no longer bears
// on it. A missing, invalid or stale declaration declares nothing, so every
// script keeps the frontier.

// FrontierForeignCallersInvalid reports a foreign-caller declaration that is
// present but unreadable, invalid, or no longer complete. Nothing is then
// declared.
const FrontierForeignCallersInvalid = "shell:foreign-callers-invalid"

// ForeignCallersProfile is the declaration's closed profile name.
const ForeignCallersProfile = "corvint-shell-foreign-callers/0"

const (
	maxForeignCallerBytes   = 64 << 10
	maxForeignCallerScripts = 64
	maxForeignCallers       = 256
	maxForeignCallerReason  = 512
)

// ForeignCallers is one declared script: every file naming it, split into
// the callers that run it and the mentions that only name it, and the
// project's stated reason.
type ForeignCallers struct {
	Callers  []string `json:"callers"`
	Mentions []string `json:"mentions,omitempty"`
	Reason   string   `json:"reason"`
}

type foreignCallersFile struct {
	Profile string                    `json:"profile"`
	Scripts map[string]ForeignCallers `json:"scripts"`
}

// readForeignCallers returns the declared scripts by path: nil with no
// declaration, an error for one that is unreadable, oversized, not a regular
// file or invalid in any member. The caller and mention lists are each
// sorted and unique, disjoint, together non-empty and of clean relative paths
// other than the script.
func readForeignCallers(root *affected.Source) (map[string]ForeignCallers, error) {
	body, err := root.ReadBounded(affected.ShellForeignCallersPath, maxForeignCallerBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var declared foreignCallersFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if declared.Profile != ForeignCallersProfile || len(declared.Scripts) == 0 || len(declared.Scripts) > maxForeignCallerScripts {
		return nil, fmt.Errorf("%s: profile must be %s with 1 to %d scripts", affected.ShellForeignCallersPath, ForeignCallersProfile, maxForeignCallerScripts)
	}
	for script, entry := range declared.Scripts {
		if !affected.ValidRelativePath(script) || !hasAnySuffix(script, Suffixes) {
			return nil, fmt.Errorf("%s: script %q", affected.ShellForeignCallersPath, script)
		}
		named := append(slices.Clone(entry.Callers), entry.Mentions...)
		if len(named) == 0 || len(named) > maxForeignCallers || !sortedUnique(entry.Callers) || !sortedUnique(entry.Mentions) {
			return nil, fmt.Errorf("%s: script %q callers and mentions must be 1 to %d sorted unique paths", affected.ShellForeignCallersPath, script, maxForeignCallers)
		}
		for _, file := range named {
			if !affected.ValidRelativePath(file) || file == script || slices.Contains(entry.Callers, file) && slices.Contains(entry.Mentions, file) {
				return nil, fmt.Errorf("%s: script %q file %q", affected.ShellForeignCallersPath, script, file)
			}
		}
		if entry.Reason == "" || len(entry.Reason) > maxForeignCallerReason || !utf8.ValidString(entry.Reason) || strings.ContainsFunc(entry.Reason, unicode.IsControl) {
			return nil, fmt.Errorf("%s: script %q reason", affected.ShellForeignCallersPath, script)
		}
	}
	return declared.Scripts, nil
}

func sortedUnique(paths []string) bool {
	return slices.IsSorted(paths) && len(slices.Compact(slices.Clone(paths))) == len(paths)
}

// callerCandidate reports a file whose text could run a script: source text
// of the shell or another planned language, a Make file, or a YAML file such
// as a workflow.
func callerCandidate(name string) bool {
	base := path.Base(name)
	return hasAnySuffix(name, Suffixes) || hasAnySuffix(name, ForeignSourceSuffixes) ||
		base == "Makefile" || base == "makefile" || base == "GNUmakefile" ||
		strings.HasSuffix(name, ".mk") || strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")
}

// applyForeignCallers verifies the declaration against the source universe
// and, when it holds, gives each declared script unit its callers and drops
// FrontierForeignInvocation from it. A declared script must be an observed
// non-test shell unit. The declaration holds only when, for each script, the
// candidate files other than the script whose text holds the script's file
// name are exactly its callers and mentions. Any failure declares nothing and names
// FrontierForeignCallersInvalid.
func applyForeignCallers(root *affected.Source, units []affected.Unit, frontier map[string]bool) {
	declared, err := readForeignCallers(root)
	if declared == nil && err == nil {
		return
	}
	if err != nil {
		frontier[FrontierForeignCallersInvalid] = true
		return
	}
	indexes := map[string]int{}
	for index, unit := range units {
		if len(unit.Tests) == 0 && len(unit.Sources) == 1 {
			if _, ok := declared[unit.Sources[0]]; ok {
				indexes[unit.Sources[0]] = index
			}
		}
	}
	if len(indexes) != len(declared) || !foreignCallersComplete(root, declared) {
		frontier[FrontierForeignCallersInvalid] = true
		return
	}
	for script, index := range indexes {
		unit := &units[index]
		unit.Callers = slices.Clone(declared[script].Callers)
		unit.Frontier = slices.DeleteFunc(unit.Frontier, func(reason string) bool { return reason == FrontierForeignInvocation })
		if len(unit.Frontier) == 0 {
			unit.Frontier, unit.FrontierBears = nil, ""
		}
	}
}

// foreignCallersComplete reports whether every declared script's callers and
// mentions are exactly the candidate files, other than the script, whose text holds the
// script's file name. An unreadable candidate fails it.
func foreignCallersComplete(root *affected.Source, declared map[string]ForeignCallers) bool {
	candidates, err := root.Files(callerCandidate)
	if err != nil {
		return false
	}
	found := make(map[string][]string, len(declared))
	for _, candidate := range candidates {
		body, err := root.Read(candidate)
		if err != nil {
			return false
		}
		for script := range declared {
			if candidate != script && bytes.Contains(body, []byte(path.Base(script))) {
				found[script] = append(found[script], candidate)
			}
		}
	}
	for script, entry := range declared {
		named, listed := found[script], append(slices.Clone(entry.Callers), entry.Mentions...)
		slices.Sort(named)
		slices.Sort(listed)
		if !slices.Equal(named, listed) {
			return false
		}
	}
	return true
}
