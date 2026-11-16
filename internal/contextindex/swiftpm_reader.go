// SPDX-License-Identifier: Apache-2.0
package contextindex

import "github.com/Beamfall/corvint/internal/langplugin/dialect"

type SwiftPMTarget = dialect.SwiftPMTarget
type SwiftImport = dialect.SwiftImport
type SwiftPMReader = dialect.SwiftPMReader

// WithSwiftPMReader returns a request-local index view with an explicit Swift reader.
// The reader is runtime wiring, never serialized index content.
func (index *Index) WithSwiftPMReader(reader SwiftPMReader) *Index {
	if index == nil {
		return nil
	}
	copy := *index
	copy.swiftPMReader = reader
	return &copy
}
