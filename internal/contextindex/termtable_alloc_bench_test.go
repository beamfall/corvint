package contextindex

// The one-shot tokenisers' allocation readings (CRK-V0-001..003, V1-3273):
// a role or evidence line is a few dozen bytes, so the interner a long-lived
// worker sizes for a source window must not be allocated for it.
const benchmarkLine = "func (compiler *taskContextCompiler) lexicalHits() []lexicalHit { // BM25 posting walk"

var benchmarkWanted = map[string]struct{}{"compiler": {}, "lexical": {}, "hits": {}, "posting": {}, "absent": {}}
