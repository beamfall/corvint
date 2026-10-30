package golang

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// This file reads the project-owned gated root-locator declaration
// (TSP-V0-003). A declared package's root-locating code runs only after it
// admits a protected document under an absolute gate outside the repository,
// so a test that links it reads the repository only on a host whose gate
// names that checkout. The package keeps its own reason, but its dependents
// are no longer root-locator dependents (AFP-V0-012 rule (d)). The package's
// own test proves the gate (internal/authoritystore's
// TestRootLocatorGated_TSP_V0_003); a dirty declaration widens the plan
// (affected.GatedRootLocatorsPath).

// FrontierGatedLocatorsInvalid reports a gated root-locator declaration that
// is present but unreadable, invalid, or no longer matches the package it
// names. Nothing is then declared, and every plan widens.
const FrontierGatedLocatorsInvalid = "go:gated-root-locators-invalid"

// GatedLocatorsProfile is the declaration's closed profile name.
const GatedLocatorsProfile = "corvint-gated-root-locators/0"

const (
	maxGatedLocatorBytes    = 64 << 10
	maxGatedLocatorPackages = 64
	maxGatedLocatorReason   = 512
)

// GatedLocator is one declared package: the absolute gate its root locating
// requires and the project's stated reason.
type GatedLocator struct {
	Gate   string `json:"gate"`
	Reason string `json:"reason"`
}

type gatedLocatorFile struct {
	Profile  string                  `json:"profile"`
	Packages map[string]GatedLocator `json:"packages"`
}

// readGatedLocators returns the declared packages by directory: nil with no
// declaration, an error for one that is unreadable, oversized, not a regular
// file, or invalid in any member. A gate is a clean absolute path other than
// the filesystem root.
func readGatedLocators(root *affected.Source) (map[string]GatedLocator, error) {
	body, err := root.ReadBounded(affected.GatedRootLocatorsPath, maxGatedLocatorBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var declared gatedLocatorFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if declared.Profile != GatedLocatorsProfile || declared.Packages == nil || len(declared.Packages) > maxGatedLocatorPackages {
		return nil, fmt.Errorf("%s: profile must be %s with at most %d packages", affected.GatedRootLocatorsPath, GatedLocatorsProfile, maxGatedLocatorPackages)
	}
	for directory, entry := range declared.Packages {
		if !affected.ValidRelativePath(directory) || directory == "." {
			return nil, fmt.Errorf("%s: package %q", affected.GatedRootLocatorsPath, directory)
		}
		if !path.IsAbs(entry.Gate) || path.Clean(entry.Gate) != entry.Gate || entry.Gate == "/" || !utf8.ValidString(entry.Gate) || strings.ContainsFunc(entry.Gate, unicode.IsControl) {
			return nil, fmt.Errorf("%s: package %q gate %q", affected.GatedRootLocatorsPath, directory, entry.Gate)
		}
		if entry.Reason == "" || len(entry.Reason) > maxGatedLocatorReason || !utf8.ValidString(entry.Reason) || strings.ContainsFunc(entry.Reason, unicode.IsControl) {
			return nil, fmt.Errorf("%s: package %q reason", affected.GatedRootLocatorsPath, directory)
		}
	}
	return declared.Packages, nil
}

// applyGatedLocators clears LocatesRoot on each declared unit, given each
// unit's directory. A declared directory must hold an observed package whose
// non-test code locates the root and whose non-test sources carry the gate as
// a string literal; any failure declares nothing and widens every plan.
func applyGatedLocators(root *affected.Source, units []affected.Unit, directories []string, frontier map[string]bool) {
	declared, err := readGatedLocators(root)
	if err != nil {
		frontier[FrontierGatedLocatorsInvalid] = true
		return
	}
	matched := map[int]bool{}
	for index, directory := range directories {
		entry, ok := declared[directory]
		if !ok {
			continue
		}
		if !units[index].LocatesRoot || !carriesLiteral(root, units[index].Sources, entry.Gate) {
			frontier[FrontierGatedLocatorsInvalid] = true
			return
		}
		matched[index] = true
	}
	if len(matched) != len(declared) {
		frontier[FrontierGatedLocatorsInvalid] = true
		return
	}
	for index := range matched {
		units[index].LocatesRoot = false
	}
}

// carriesLiteral reports whether a non-test source holds value as an
// interpreted string literal. An unreadable source reports false.
func carriesLiteral(root *affected.Source, sources []string, value string) bool {
	literal := []byte(strconv.Quote(value))
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		body, err := root.Read(source)
		if err != nil {
			return false
		}
		if bytes.Contains(body, literal) {
			return true
		}
	}
	return false
}
