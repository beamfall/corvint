package changee2e

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// Runtime coverage schema, input cap and hit basis (CCE-V0-045..047).
const (
	CoverageSchema   = "e2e-runtime-coverage/0"
	MaxCoverageBytes = 4 << 20
	CoverageBasis    = "runtime-coverage"
)

type runtimeCoverage struct {
	Schema   string `json:"schema"`
	Revision string `json:"revision"`
	Screens  []struct {
		Screen string   `json:"screen"`
		Paths  []string `json:"paths"`
	} `json:"screens"`
}

// readCoverage reads a runtime coverage document bound to the head revision and the head map
// (CCE-V0-045). Any other document refuses `e2e-coverage-invalid`; it returns the document's
// SHA-256 and, per head-map screen, the repository paths observed executing on it.
func readCoverage(file, head string, m *appmap.Map) (string, map[string][]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", nil, refuse("e2e-coverage-invalid", "%v", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxCoverageBytes+1))
	if err != nil || len(data) > MaxCoverageBytes {
		return "", nil, refuse("e2e-coverage-invalid", "--coverage must name a readable file of at most %d bytes", MaxCoverageBytes)
	}
	var c runtimeCoverage
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil || dec.More() {
		return "", nil, refuse("e2e-coverage-invalid", "%v", err)
	}
	if c.Schema != CoverageSchema {
		return "", nil, refuse("e2e-coverage-invalid", "schema %q", c.Schema)
	}
	if c.Revision != head {
		return "", nil, refuse("e2e-coverage-invalid", "coverage was recorded at %q, not the head revision %s", c.Revision, head)
	}
	screens := map[string]bool{}
	for _, s := range m.Screens {
		screens[s.ID] = true
	}
	out := map[string][]string{}
	for _, s := range c.Screens {
		if !screens[s.Screen] || out[s.Screen] != nil {
			return "", nil, refuse("e2e-coverage-invalid", "screen %q is not a head-map screen or repeats", s.Screen)
		}
		for _, p := range s.Paths {
			if p == "" || path.Clean(p) != p || path.IsAbs(p) || p == ".." || strings.HasPrefix(p, "../") {
				return "", nil, refuse("e2e-coverage-invalid", "screen %q: path %q is not a clean repository-relative path", s.Screen, p)
			}
		}
		out[s.Screen] = append([]string{}, s.Paths...)
	}
	return sha256Hex(data), out, nil
}

// coverageHits returns the screens runtime coverage places changed paths on, beyond the screens
// impact placed them on, each with basis CoverageBasis (CCE-V0-046). They only add reach: they
// never place a path for the unknown-span rules, so no unknown span is removed.
func coverageHits(covered map[string][]string, changed map[string]bool, impact []appmap.ImpactScreen) []appmap.ImpactScreen {
	already := map[string]bool{}
	for _, h := range impact {
		for _, p := range h.Paths {
			already[h.ID+"\x00"+p] = true
		}
	}
	var hits []appmap.ImpactScreen
	for id, paths := range covered {
		var hit []string
		for _, p := range paths {
			if changed[p] && !already[id+"\x00"+p] {
				hit = append(hit, p)
			}
		}
		if len(hit) == 0 {
			continue
		}
		sort.Strings(hit)
		hits = append(hits, appmap.ImpactScreen{ID: id, Basis: CoverageBasis, Paths: uniqueStrings(hit)})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].ID < hits[j].ID })
	return hits
}

func uniqueStrings(sorted []string) []string {
	out := sorted[:0]
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			out = append(out, s)
		}
	}
	return out
}
