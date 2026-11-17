// SPDX-License-Identifier: Apache-2.0
package dialect

import "io/fs"

// SwiftPMTarget is one SwiftPM target the affected Swift model reads from the
// indexed Package.swift manifests: its unit ID, module name and member paths
// (sources, then tests).
type SwiftPMTarget struct {
	ID      string
	Module  string
	Members []string
}

// SwiftImport is one module a Swift import declaration names, with the 1-based
// line of the declaration.
type SwiftImport struct {
	Module string
	Line   int
}

// SwiftPMReader supplies the affected Swift model's SwiftPM reading to path
// impact (NGI-V0-980, NGI-V0-988). This package does not import the model: it
// is part of the corvint-tasks source archive closure (PUB-V0-012, PUB-V0-013)
// and the test-selection planner is not, so a binary links
// internal/swiftimpact and passes it explicitly. Without a reader, Swift paths have no
// target membership and the receipt says so.
type SwiftPMReader interface {
	Targets(fsys fs.FS) ([]SwiftPMTarget, error)
	ImportedModules(text string) []SwiftImport
}
