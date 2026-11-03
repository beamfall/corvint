package contextindex

import "github.com/Beamfall/corvint/internal/langplugin/dialect"

// rubySymbols extracts one Ruby source's declarations with the symbol
// extractor the source's region dialect registered (LGP-V0-025); the lexical
// scanner and declaration rules live in the Ruby plugin.
var rubySymbols = regionSymbols

// regionSymbols extracts a source's declarations with its region dialect's
// registered extractor. Every path that stops short of the end of the file, or
// that reaches the end with a lexical state it could not resolve, returns an
// ExtractionNote. A caller that sees no note may treat the returned symbols as
// the file's whole declaration set; that guarantee is what the notes buy. A
// dialect with no registered extractor fails closed with a note.
func regionSymbols(source Source, text string) ([]Symbol, []ExtractionNote) {
	symbols := make([]Symbol, 0)
	notes := make([]ExtractionNote, 0)
	region, ok := dialect.LookupRegion(SymbolRegionLanguage(source.Path))
	if !ok || region.Symbols == nil {
		return symbols, append(notes, ExtractionNote{source.Path, noteExtractorUnavailable})
	}
	extraction := region.Symbols(text, maxSourceLines, maxSymbolsPerSource)
	for _, declaration := range extraction.Declarations {
		symbols = append(symbols, Symbol{Kind: declaration.Kind, Name: declaration.Name, Path: source.Path, BlobHash: source.BlobHash, Line: declaration.Line})
	}
	switch extraction.Stop {
	case dialect.LineCap:
		return symbols, append(notes, ExtractionNote{source.Path, noteLineCap})
	case dialect.SymbolCap:
		return symbols, append(notes, ExtractionNote{source.Path, noteSymbolCap})
	}
	if extraction.UnterminatedComment {
		notes = append(notes, ExtractionNote{source.Path, noteUnterminatedComment})
	}
	if extraction.UnterminatedString {
		notes = append(notes, ExtractionNote{source.Path, noteUnterminatedString})
	}
	return symbols, notes
}
