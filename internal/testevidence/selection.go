// SPDX-License-Identifier: Apache-2.0
package testevidence

import "strings"

// SelectionCapability is derived from admitted record components, not a new wire
// field. It describes prerequisites, never qualification or permission to exclude.
type SelectionCapability uint8

const (
	NativeSelection SelectionCapability = iota
	FileSelection
)

// SelectionCapability recognizes only the complete RSP-V0-050 script marker
// on the declared script runner and a script test identity.
// Partial, unknown or populated absent components retain native prerequisites.
func (r Record) SelectionCapability() SelectionCapability {
	if r.Runner.Plugin == "script" && strings.HasPrefix(r.Test, "script:") &&
		r.Executed.State == Absent && r.Executed.Reason == "script-kind" && len(r.Executed.Items) == 0 &&
		r.Regions.State == Absent && r.Regions.Reason == "script-kind" && len(r.Regions.Items) == 0 &&
		r.Loaded.State == Absent && r.Loaded.Reason == "script-kind" && len(r.Loaded.Items) == 0 {
		return FileSelection
	}
	return NativeSelection
}

func (r Record) SelectionComponents() []string {
	if r.SelectionCapability() == FileSelection {
		return []string{"reads", "listings", "lookups", "execs", "external"}
	}
	return []string{"executed", "regions", "loaded", "reads", "listings", "lookups", "execs", "external"}
}

// MissingSelectionComponents is shared by selection and collection readiness
// (RSP-V0-013/050/055), so neither can waive an applicable prerequisite.
func (r Record) MissingSelectionComponents() []string {
	states := map[string]State{"executed": r.Executed.State, "regions": r.Regions.State, "loaded": r.Loaded.State,
		"reads": r.Reads.State, "listings": r.Listings.State, "lookups": r.Lookups.State, "execs": r.Execs.State, "external": r.External.State}
	var missing []string
	for _, name := range r.SelectionComponents() {
		if states[name] != Present {
			missing = append(missing, name)
		}
	}
	return missing
}
