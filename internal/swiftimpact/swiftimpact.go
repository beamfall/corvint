// Package swiftimpact links the affected Swift model's SwiftPM reading into
// path impact (NGI-V0-980, NGI-V0-988). It sits outside internal/contextindex
// so that corvint-tasks, which imports contextindex but not the test-selection
// planner, keeps its offline source archive closure (PUB-V0-012, PUB-V0-013).
// A binary that serves path impact imports it for its side effect.
package swiftimpact

import (
	"io/fs"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/swift"
)

func init() { contextindex.RegisterSwiftPMReader(Reader{}) }

// Reader is the affected Swift model's SwiftPM reading.
type Reader struct{}

// Targets lists the SwiftPM targets (`swift:spm:` units) the affected Swift
// model reads from fsys; other units, such as Xcode targets, are skipped.
func (Reader) Targets(fsys fs.FS) ([]contextindex.SwiftPMTarget, error) {
	result, err := swift.New().UnitsSource(affected.FSSource(fsys))
	if err != nil {
		return nil, err
	}
	targets := make([]contextindex.SwiftPMTarget, 0, len(result.Units))
	for _, unit := range result.Units {
		module, ok := swift.TargetModule(unit.ID)
		if !ok {
			continue
		}
		members := append(append([]string(nil), unit.Sources...), unit.Tests...)
		targets = append(targets, contextindex.SwiftPMTarget{ID: unit.ID, Module: module, Members: members})
	}
	return targets, nil
}

// ImportedModules lists the modules a Swift source imports, from code only.
func (Reader) ImportedModules(text string) []contextindex.SwiftImport {
	imported := swift.ImportedModules(text)
	modules := make([]contextindex.SwiftImport, 0, len(imported))
	for _, item := range imported {
		modules = append(modules, contextindex.SwiftImport{Module: item.Module, Line: item.Line})
	}
	return modules
}
