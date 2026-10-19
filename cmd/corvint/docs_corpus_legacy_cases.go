package main

import "github.com/Beamfall/corvint/internal/doccorpus"

// legacyCasesCorpusOptions holds the test-management export options of
// import-cases and search-cases (DCP-V1-070..079). Corvint reads a
// caller-exported file and never contacts the test-management service.
type legacyCasesCorpusOptions struct {
	format, content, match string
}

func (l *legacyCasesCorpusOptions) addFlags(flags map[string]*string) {
	flags["--format"] = &l.format
	flags["--content"] = &l.content
	flags["--match"] = &l.match
}

func compileLegacyCasesCorpus(o corpusOptions) ([]byte, error) {
	// DCP-V1-169: search-cases takes exactly one of --query and --id, and
	// --match all|any only with --query.
	if match := o.legacyCases.match; o.op == "search-cases" && ((o.query == "") == (o.id == "") || o.id != "" && match != "" || match != "" && match != doccorpus.LegacyMatchAll && match != doccorpus.LegacyMatchAny) {
		return nil, argumentError("search-cases takes exactly one of --query and --id, and --match all|any only with --query")
	}
	raw, err := doccorpus.ReadLegacyCasesFile(o.root, o.input)
	if err != nil {
		return nil, err
	}
	if o.op == "search-cases" {
		return doccorpus.SearchLegacyCasesQuery(o.legacyCases.format, o.input, raw, doccorpus.LegacyCaseQuery{Query: o.query, ID: o.id, Match: o.legacyCases.match, Limit: o.limit})
	}
	return doccorpus.ImportLegacyCases(o.legacyCases.format, o.legacyCases.content, o.input, raw)
}
