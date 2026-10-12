package main

import "github.com/Beamfall/corvint/internal/doccorpus"

// legacyCasesCorpusOptions holds the test-management export options of
// import-cases and search-cases (DCP-V1-070..079). Corvint reads a
// caller-exported file and never contacts the test-management service.
type legacyCasesCorpusOptions struct {
	format, content string
}

func (l *legacyCasesCorpusOptions) addFlags(flags map[string]*string) {
	flags["--format"] = &l.format
	flags["--content"] = &l.content
}

func compileLegacyCasesCorpus(o corpusOptions) ([]byte, error) {
	raw, err := doccorpus.ReadLegacyCasesFile(o.root, o.input)
	if err != nil {
		return nil, err
	}
	if o.op == "search-cases" {
		return doccorpus.SearchLegacyCases(o.legacyCases.format, o.input, raw, o.query, o.limit)
	}
	return doccorpus.ImportLegacyCases(o.legacyCases.format, o.legacyCases.content, o.input, raw)
}
