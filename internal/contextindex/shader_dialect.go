// SPDX-License-Identifier: Apache-2.0

package contextindex

import (
	"github.com/Beamfall/corvint/internal/langplugin/dialect"
	"github.com/Beamfall/corvint/internal/shaderdialect"
)

func shaderSymbols(source Source, text string) ([]Symbol, []ExtractionNote) {
	extraction := shaderdialect.Symbols(text, maxSourceLines, maxSymbolsPerSource)
	reason := ""
	switch extraction.Stop {
	case "":
	case dialect.LineCap:
		reason = noteLineCap
	case dialect.SymbolCap:
		reason = noteSymbolCap
	default:
		reason = string(extraction.Stop)
	}
	if reason != "" {
		return nil, []ExtractionNote{{source.Path, reason}}
	}
	symbols := make([]Symbol, 0, len(extraction.Declarations))
	for _, d := range extraction.Declarations {
		symbols = append(symbols, Symbol{Kind: d.Kind, Name: d.Name, Path: source.Path, BlobHash: source.BlobHash, Line: d.Line})
	}
	return symbols, nil
}
