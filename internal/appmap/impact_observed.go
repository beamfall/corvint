package appmap

import "strings"

// Observed page-object bindings in impact (docs/specs/observed-ui-evidence-v0.md, OUE-V0-070..074).
// An observed-ui-records/0 binding places a test-source file (page object, workflow or spec) on
// the screens its actions ran on in a recorded trace. Impact reads it only to add: a changed file
// with a binding touches the bound map screens and selects the tests that observed it. A binding
// never removes an unknown, never turns full-suite-required into a narrower verdict, and a changed
// file without one keeps the existing import-graph placement or its unknown reason.

// BasisObservedBinding is the screen basis a binding from observed-ui-records/0 adds (OUE-V0-071).
const BasisObservedBinding = "observed-binding"

// ImpactObserved is one changed path that observed bindings placed, with its provenance.
type ImpactObserved struct {
	Path     string   `json:"path"`
	Bindings []string `json:"bindings"`
	// Screens are the bound screens the map declares; UnmappedScreens are bound screens it does
	// not (a route template, a normalized URL or an unresolved screen), which select their tests only.
	Screens         []string `json:"screens"`
	UnmappedScreens []string `json:"unmapped_screens"`
	Tests           []string `json:"tests"`
	Traces          []string `json:"traces"`
}

// checkRecordsBinding refuses records built without a map or for another application (OUE-V0-070).
// A nil records document passes.
func checkRecordsBinding(m *Map, rec *UIRecords) error {
	if rec == nil {
		return nil
	}
	if rec.App == "" || rec.MapDigest == "" {
		return verifyError("appmap-invalid-query", "the observed UI records were built without an application map; rebuild them with --map")
	}
	if rec.App != m.App {
		return verifyError("appmap-invalid-query", "the observed UI records were built for another application")
	}
	return nil
}

// observedBindings collects, per changed path in order, the records' bindings of that path in the
// --root repository (OUE-V0-071, OUE-V0-072).
func observedBindings(m *Map, rec *UIRecords, changed []string) []ImpactObserved {
	if rec == nil {
		return nil
	}
	byFile := map[string][]*UIBinding{}
	for i := range rec.Bindings {
		if b := &rec.Bindings[i]; b.Repo == "" {
			byFile[b.File] = append(byFile[b.File], b)
		}
	}
	var out []ImpactObserved
	for _, p := range changed {
		bs := byFile[p]
		if len(bs) == 0 {
			continue
		}
		ids, screens, unmapped, tests, traces := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
		for _, b := range bs {
			ids[b.ID] = true
			for _, s := range b.Screens {
				if !strings.HasPrefix(s, "unknown:") && m.screen(s) != nil {
					screens[s] = true
				} else {
					unmapped[s] = true
				}
			}
			for _, t := range b.Tests {
				tests[t] = true
			}
			for _, pv := range b.Provenance {
				traces[pv.TraceSHA256] = true
				tests[pv.TestKey] = true
			}
		}
		delete(tests, "")
		delete(traces, "")
		out = append(out, ImpactObserved{Path: p, Bindings: sortedKeys(ids), Screens: sortedKeys(screens),
			UnmappedScreens: sortedKeys(unmapped), Tests: sortedKeys(tests), Traces: sortedKeys(traces)})
	}
	return out
}
