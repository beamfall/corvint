package jstestreceipt

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

const FreshDependencyFiles = 512

// FreshDependencyManifest is the operator-approved complete local CJS input
// set. Roots bind every installed file, including previously unloaded modules;
// Files additionally pins the admitted test/config/server/observer inputs.
// Native libraries, ESM/dynamic remote loading and hostile mutation containment
// remain outside the trusted source-direct tuple's qualification.
type FreshDependencyManifest struct {
	Roots []string          `json:"roots"`
	Files map[string]string `json:"files"`
}

type FreshDependencyObservation struct {
	Files    map[string]string `json:"files"`
	Failures []string          `json:"failures"`
}

func FreshDependencyDigest(m *FreshDependencyManifest) string {
	b, _ := json.Marshal(m)
	return Sha256Hex(b)
}

func FreshDependencyPath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && path != "/" && len(path) <= 4096 && !strings.ContainsAny(path, "\x00\r\n") && !secretscreen.MatchString(path)
}

func FreshManifestComplete(m *FreshDependencyManifest) bool {
	if m == nil || len(m.Roots) < 1 || len(m.Roots) > 8 || len(m.Files) < 1 || len(m.Files) > FreshDependencyFiles || !slices.IsSorted(m.Roots) {
		return false
	}
	for i, root := range m.Roots {
		if !FreshDependencyPath(root) || (i > 0 && m.Roots[i-1] == root) {
			return false
		}
		found := false
		for path := range m.Files {
			if strings.HasPrefix(path, root+"/") {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	for path, digest := range m.Files {
		if !FreshDependencyPath(path) || !RawDigestPattern.MatchString(digest) {
			return false
		}
	}
	return true
}

func FreshDependenciesMatch(m *FreshDependencyManifest, o *FreshDependencyObservation) bool {
	return FreshManifestComplete(m) && o != nil && len(o.Failures) == 0 && reflect.DeepEqual(m.Files, o.Files)
}

func freshImportsBound(imports map[string]string, required string, m *FreshDependencyManifest) bool {
	if m == nil || len(imports) == 0 || len(imports) > FreshDependencyFiles || imports[required] == "" {
		return false
	}
	for path, digest := range imports {
		if !RawDigestPattern.MatchString(digest) || m.Files[path] != digest {
			return false
		}
	}
	return true
}
