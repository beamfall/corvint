# Python selection corpus

A tracked, layered Python package (`src/corpus`, 10 modules) with 21 pytest files, written for
`TestPythonSelectionNarrowsARealCorpus` (AFP-V0-186, V1-2298). The original 18-module corpus that
test named (`src/context_corvint_index.py`) is not tracked on `origin/main`, so the test skipped
and the requirement had no evidence. Import chain: `config` <- `paths` <- `store` (also `hashing`)
<- `index` <- `cache` and `query` <- `cli`; `render` and `report` (a relative import) hang off
`config`. Editing `src/corpus/index.py` must select exactly the index, cache, query and cli tests.
