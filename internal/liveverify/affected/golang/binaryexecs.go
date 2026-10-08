package golang

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// This file records the command edges of AFP-V0-037: a package that runs the
// built binary of a command (a package main in the repository) depends on the
// command's whole build, though no import edge says so. An edge comes from a
// path literal of the package that names the command's directory, as in
// `go build ./cmd/corvint`, or from the project-owned declaration
// affected.BinaryExecsPath for a package that runs a binary it is handed.

// FrontierBinaryExecsInvalid reports a command-edge declaration that is present
// but unreadable or invalid. No declared edge is then kept, so a plan may miss
// a declared runner and widens instead.
const FrontierBinaryExecsInvalid = "go:test-binary-execs-invalid"

// BinaryExecsProfile is the declaration's closed profile name.
const BinaryExecsProfile = "corvint-test-binary-execs/0"

const (
	maxBinaryExecBytes    = 1 << 20
	maxBinaryExecPackages = 4096
	maxBinaryExecEntries  = 256
)

type binaryExecsFile struct {
	Profile  string              `json:"profile"`
	Packages map[string][]string `json:"packages"`
}

// applyBinaryExecs sets each unit's command edges, given each unit's
// directory and module and the command unit in each command directory: the
// literal edges always, and the declared edges unless the declaration is
// unreadable or invalid, which widens every plan instead.
func applyBinaryExecs(root *affected.Source, units []affected.Unit, directories []string, owners map[string]module, commands map[string]string, frontier map[string]bool) {
	edges := make([]map[string]bool, len(units))
	for index := range units {
		edges[index] = literalExecs(units[index], directories[index], owners[directories[index]].dir, commands)
	}
	declared, err := readBinaryExecs(root)
	var declaredEdges map[int][]string
	if err == nil {
		declaredEdges, err = matchBinaryExecs(declared, directories, commands)
	}
	if err != nil {
		frontier[FrontierBinaryExecsInvalid] = true
	}
	for index := range units {
		for _, id := range declaredEdges[index] {
			edges[index][id] = true
		}
		if len(edges[index]) != 0 {
			units[index].Execs = sortedKeys(edges[index])
		}
	}
}

// literalExecs names the commands, other than the unit itself, whose directory
// one of the unit's path tokens names exactly.
func literalExecs(unit affected.Unit, directory, moduleDir string, commands map[string]string) map[string]bool {
	edges := map[string]bool{}
	for _, value := range unit.PathTokens {
		target, ok := commandDirectory(value, directory, moduleDir)
		if !ok {
			continue
		}
		if id, command := commands[target]; command && id != unit.ID {
			edges[id] = true
		}
	}
	return edges
}

// commandDirectory resolves a path token to the repository-relative directory
// it names: an anchored token or a plain one against the module's directory
// (`go build ./cmd/x` runs there), a climbing one against the unit's own
// directory. A token naming no directory below the module root names none.
func commandDirectory(value, directory, moduleDir string) (string, bool) {
	var target string
	switch {
	case strings.HasPrefix(value, "/"):
		target = path.Join(moduleDir, value[1:])
	case value == ".." || strings.HasPrefix(value, "../"):
		target = path.Join(directory, value)
	default:
		target = path.Join(moduleDir, value)
	}
	if target == "." || target == moduleDir || target == ".." || strings.HasPrefix(target, "../") {
		return "", false
	}
	return target, true
}

// readBinaryExecs returns the declared command directories by package
// directory: nil with no declaration, an error for one that is unreadable,
// oversized, not a regular file, or invalid in any member.
func readBinaryExecs(root *affected.Source) (map[string][]string, error) {
	body, err := root.ReadBounded(affected.BinaryExecsPath, maxBinaryExecBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var declared binaryExecsFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if declared.Profile != BinaryExecsProfile || declared.Packages == nil || len(declared.Packages) > maxBinaryExecPackages {
		return nil, fmt.Errorf("%s: profile must be %s with at most %d packages", affected.BinaryExecsPath, BinaryExecsProfile, maxBinaryExecPackages)
	}
	for directory, entries := range declared.Packages {
		if !affected.ValidRelativePath(directory) || len(entries) == 0 || len(entries) > maxBinaryExecEntries {
			return nil, fmt.Errorf("%s: package %q", affected.BinaryExecsPath, directory)
		}
		previous := ""
		for _, entry := range entries {
			if !affected.ValidRelativePath(entry) || entry <= previous || entry == directory {
				return nil, fmt.Errorf("%s: package %q entry %q", affected.BinaryExecsPath, directory, entry)
			}
			previous = entry
		}
	}
	return declared.Packages, nil
}

// matchBinaryExecs returns the declared command units by unit index. Every
// declared directory must hold an observed package and every entry an observed
// command; otherwise no declared edge is kept.
func matchBinaryExecs(declared map[string][]string, directories []string, commands map[string]string) (map[int][]string, error) {
	edges := make(map[int][]string, len(declared))
	for index, directory := range directories {
		entries, ok := declared[directory]
		if !ok {
			continue
		}
		for _, entry := range entries {
			id, command := commands[entry]
			if !command {
				return nil, fmt.Errorf("%s: package %q entry %q holds no command", affected.BinaryExecsPath, directory, entry)
			}
			edges[index] = append(edges[index], id)
		}
	}
	if len(edges) != len(declared) {
		return nil, fmt.Errorf("%s declares a directory that holds no Go package", affected.BinaryExecsPath)
	}
	return edges, nil
}
