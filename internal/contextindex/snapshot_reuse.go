package contextindex

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// Incremental re-index (IDX-SNAP-V0-037, V1-0328). A re-index of a new tree
// used to pin, extract and tokenise every source again although most blobs are
// unchanged since the last snapshot. snapshotReuse lets the writer's build
// start from the newest verified snapshot this engine wrote for the same
// object format: a source whose path, blob id, size and rehashed bytes match a
// prior source is taken from it with its per-source facts (symbols, markers,
// imports, refusals, notes, approximate-import count) and its term postings,
// and only the remaining sources are pinned, extracted and tokenised. Every
// table that depends on more than one source -- documents, symbol windows, the
// identifier graph, the profile, features and scenarios -- is still compiled
// over the whole tree. The result is the cold build's index exactly
// (IDX-SNAP-V0-022); any refusal below falls back to the cold build.

// snapshotReuse loads its prior once per build, on the first attempt that asks.
type snapshotReuse struct {
	root   string
	once   sync.Once
	prior  *reusePrior
	reason string
}

// reusePrior is a decoded, verified prior snapshot with its rows regrouped per
// source path.
type reusePrior struct {
	index *Index
	facts map[string]*blobFacts
	// taken is the set of paths takeUnchanged reused for the current attempt.
	taken map[string]struct{}
}

// priorVocabulary carries the prior term table and the prior-to-new source id
// map into buildVocabulary. remap[old] is the new id, or -1 when the old
// source was not reused.
type priorVocabulary struct {
	table *TermTable
	remap []int32
	reuse map[string]struct{}
}

func newSnapshotReuse(root string) *snapshotReuse { return &snapshotReuse{root: root} }

// priorFor returns the reusable prior for this build's identity, or nil.
func (reuse *snapshotReuse) priorFor(identity repositoryIdentity) *reusePrior {
	if reuse == nil {
		return nil
	}
	reuse.once.Do(func() {
		prior, err := loadReusePrior(reuse.root, identity.objectFormat)
		if err != nil {
			reuse.reason = err.Error()
			return
		}
		reuse.prior = prior
	})
	if reuse.prior == nil || reuse.prior.index.ObjectFormat != identity.objectFormat {
		return nil
	}
	return reuse.prior
}

// loadReusePrior picks the newest snapshot in the store written by the running
// engine for objectFormat, decodes it through the reader's verification
// (header identity and digest, vocabulary and symbol-window bounds), and
// regroups its rows per path. Any inconsistency refuses the prior.
func loadReusePrior(root, objectFormat string) (*reusePrior, error) {
	engineID := engine()
	if engineID == "" {
		return nil, errors.New("running binary cannot be digested")
	}
	directory, ok := snapshotDirectoryPresent(root)
	if !ok {
		return nil, errors.New("no snapshot store")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	prefix, suffix := objectFormat+"-", "-"+engineID+".gob"
	newest, newestTime := "", int64(0)
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if stamp := info.ModTime().UnixNano(); newest == "" || stamp > newestTime || stamp == newestTime && name > newest {
			newest, newestTime = name, stamp
		}
	}
	if newest == "" {
		return nil, errors.New("no snapshot of this engine")
	}
	target := filepath.Join(directory, newest)
	tree := snapshotTreeOf(target)
	if snapshotPath(directory, objectFormat, tree, engineID) != target {
		return nil, errors.New("snapshot name does not round-trip")
	}
	file, err := os.Open(target)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	index, err := decodeSnapshot(file, repositoryIdentity{objectFormat: objectFormat, treeRevision: tree}, engineID)
	if err != nil {
		return nil, err
	}
	facts, err := priorFacts(index)
	if err != nil {
		return nil, err
	}
	return &reusePrior{index: index, facts: facts}, nil
}

// priorFacts regroups a snapshot's per-source rows by path and proves the
// regrouping complete: every row names a prior source and, where it carries
// one, that source's blob id; every source's
// approximate-import flag is re-derived by the rule recordImports applies, and
// their sum is the snapshot's own count. A vocabulary whose paths are not the
// sorted sources, or whose postings are not canonical, refuses the prior.
func priorFacts(index *Index) (map[string]*blobFacts, error) {
	if index.Vocabulary == nil {
		return nil, errors.New("prior snapshot has no vocabulary")
	}
	paths := blobSourcePaths(index.Sources)
	if len(paths) != len(index.Vocabulary.Paths) {
		return nil, errors.New("prior vocabulary paths are not its sources")
	}
	for position, sourcePath := range paths {
		if index.Vocabulary.Paths[position] != sourcePath {
			return nil, errors.New("prior vocabulary paths are not its sources")
		}
	}
	for _, postings := range []termPostings{index.Vocabulary.Terms, index.Vocabulary.Words, index.Vocabulary.PathTerms} {
		if err := postings.canonical(); err != nil {
			return nil, err
		}
	}
	facts := make(map[string]*blobFacts, len(index.Sources))
	for sourcePath, source := range index.Sources {
		if source.Path != sourcePath || !source.Checked {
			return nil, errors.New("prior source is not a pinned source")
		}
		facts[sourcePath] = &blobFacts{Source: source}
	}
	missing := errors.New("prior row names no prior source")
	// unbound: a row whose blob id is not its source's would carry a citation
	// to other bytes forward under the re-verified source (invariant 1).
	unbound := errors.New("prior row blob id is not its source's blob id")
	for _, symbol := range index.Symbols {
		fact := facts[symbol.Path]
		if fact == nil {
			return nil, missing
		}
		if symbol.BlobHash != fact.Source.BlobHash {
			return nil, unbound
		}
		fact.Symbols = append(fact.Symbols, symbol)
	}
	for _, field := range index.Fields {
		fact := facts[field.Path]
		if fact == nil {
			return nil, missing
		}
		if field.BlobHash != fact.Source.BlobHash {
			return nil, unbound
		}
		fact.Fields = append(fact.Fields, field)
	}
	for key, markers := range index.Markers {
		for _, marker := range markers {
			fact := facts[marker.Path]
			if fact == nil {
				return nil, missing
			}
			if marker.BlobHash != fact.Source.BlobHash {
				return nil, unbound
			}
			if fact.Markers == nil {
				fact.Markers = make(map[string][]Marker)
			}
			fact.Markers[key] = append(fact.Markers[key], marker)
		}
	}
	for _, refusal := range index.Unparsed {
		fact := facts[refusal.Path]
		if fact == nil {
			return nil, missing
		}
		// The extractors bind every refusal to its blob except an import
		// refusal, which carries none; the prior must match that rule exactly.
		bound := fact.Source.BlobHash
		if refusal.Facts == "imports" {
			bound = ""
		}
		if refusal.BlobHash != bound {
			return nil, unbound
		}
		for _, held := range fact.Unparsed {
			if held.Facts == refusal.Facts {
				return nil, errors.New("prior source carries two refusals of one fact class")
			}
		}
		fact.Unparsed = append(fact.Unparsed, refusal)
	}
	for _, note := range index.ExtractionNotes {
		fact := facts[note.Path]
		if fact == nil {
			return nil, missing
		}
		fact.Notes = append(fact.Notes, note)
	}
	approximate := 0
	for sourcePath, imported := range index.Imports {
		fact := facts[sourcePath]
		if fact == nil {
			return nil, missing
		}
		if len(imported) == 0 {
			return nil, errors.New("prior snapshot files an empty import set")
		}
		fact.Imports = imported
		if extension := strings.ToLower(path.Ext(sourcePath)); extension != ".go" && extension != ".py" {
			fact.ApproximateImports = 1
			approximate++
		}
	}
	if approximate != index.ApproximateImports {
		return nil, errors.New("prior approximate-import count does not regroup")
	}
	return facts, nil
}

// takeUnchanged moves every candidate the prior can supply into sources and
// returns the rest, still in tree order, for the pin pass. A candidate is
// reused only when the prior source has its path, blob id and size and the
// prior bytes rehash to that blob id under the build's object format, so the
// reused evidence is the committed blob exactly as a pin would read it.
func (prior *reusePrior) takeUnchanged(candidates []treeEntry, objectFormat string, sources map[string]Source) []treeEntry {
	if prior == nil {
		return candidates
	}
	matched := make([]bool, len(candidates))
	workers := min(runtime.NumCPU(), max(len(candidates)/sourcesPerWorker, 1))
	var pending sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		pending.Add(1)
		low, high := worker*len(candidates)/workers, (worker+1)*len(candidates)/workers
		go func(low, high int) {
			defer pending.Done()
			for position := low; position < high; position++ {
				entry := candidates[position]
				source, ok := prior.index.Sources[entry.path]
				matched[position] = ok && source.BlobHash == entry.oid && len(source.Data) == entry.size &&
					gitBlobHash(source.Data, objectFormat) == entry.oid
			}
		}(low, high)
	}
	pending.Wait()
	prior.taken = make(map[string]struct{})
	rest := make([]treeEntry, 0)
	for position, entry := range candidates {
		if !matched[position] {
			rest = append(rest, entry)
			continue
		}
		source := prior.index.Sources[entry.path]
		source.Mode = entry.mode
		sources[entry.path] = source
		prior.taken[entry.path] = struct{}{}
	}
	return rest
}

// attach hands the reused sources' facts and postings to compile.
func (prior *reusePrior) attach(index *Index) {
	if prior == nil || len(prior.taken) == 0 {
		return
	}
	index.blobFacts = make(map[string]*blobFacts, len(prior.taken))
	for sourcePath := range prior.taken {
		fact := *prior.facts[sourcePath]
		fact.Source = index.Sources[sourcePath]
		index.blobFacts[sourcePath] = &fact
	}
	newPaths := blobSourcePaths(index.Sources)
	remap := make([]int32, len(prior.index.Vocabulary.Paths))
	for old, sourcePath := range prior.index.Vocabulary.Paths {
		remap[old] = -1
		if _, ok := prior.taken[sourcePath]; ok {
			remap[old] = int32(sort.SearchStrings(newPaths, sourcePath))
		}
	}
	index.priorVocabulary = &priorVocabulary{table: prior.index.Vocabulary, remap: remap, reuse: prior.taken}
	index.reusedSources = len(prior.taken)
}

// canonical refuses postings mergeReusedPostings cannot walk as the canonical
// form mergePostingBuilders writes: keys strictly ascending and each key's
// sources strictly ascending.
func (postings termPostings) canonical() error {
	if len(postings.KeyOffsets) == 0 || len(postings.Offsets) != len(postings.KeyOffsets) {
		return errors.New("prior term table has no key offsets")
	}
	for key := 0; key < postings.keyCount(); key++ {
		if key > 0 && postings.key(key-1) >= postings.key(key) {
			return errors.New("prior term table keys are not strictly ascending")
		}
		for position := postings.Offsets[key] + 1; position < postings.Offsets[key+1]; position++ {
			if postings.Sources[position-1] >= postings.Sources[position] {
				return errors.New("prior term table sources are not strictly ascending")
			}
		}
	}
	return nil
}

// buildReusedVocabulary tokenises only the sources not taken from the prior
// and merges their postings with the prior's, renumbered to the new paths.
func (index *Index) buildReusedVocabulary() *TermTable {
	paths := blobSourcePaths(index.Sources)
	perSource := make([]sourceTerms, len(paths))
	workers := min(runtime.NumCPU(), max(len(paths)/sourcesPerWorker, 1))
	var pending sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		pending.Add(1)
		low, high := worker*len(paths)/workers, (worker+1)*len(paths)/workers
		go func(low, high int) {
			defer pending.Done()
			tokeniser := newTokeniser()
			for position := low; position < high; position++ {
				if _, reused := index.priorVocabulary.reuse[paths[position]]; reused {
					continue
				}
				perSource[position] = tokeniseSource(paths[position], index.Sources[paths[position]], tokeniser)
			}
		}(low, high)
	}
	pending.Wait()
	prior, remap := index.priorVocabulary.table, index.priorVocabulary.remap
	terms, words, pathTerms := invertSourceTerms(perSource)
	return &TermTable{Paths: paths,
		Terms:     mergeReusedPostings(prior.Terms, remap, terms, true),
		Words:     mergeReusedPostings(prior.Words, remap, words, false),
		PathTerms: mergeReusedPostings(prior.PathTerms, remap, pathTerms, false)}
}

// mergeReusedPostings merges the prior's postings, renumbered by remap with
// dropped sources removed, into the fresh sources' postings. The two source
// sets are disjoint and remap is monotone, so each merged list is ascending;
// a key left with no source is dropped, which is the cold build's absence.
func mergeReusedPostings(prior termPostings, remap []int32, fresh termPostings, counted bool) termPostings {
	keyBytes := make([]byte, 0, len(prior.KeyBytes)+len(fresh.KeyBytes))
	keyOffsets := make([]uint32, 1, prior.keyCount()+fresh.keyCount()+1)
	offsets := make([]uint32, 1, cap(keyOffsets))
	sources := make([]uint32, 0, len(prior.Sources)+len(fresh.Sources))
	var counts []uint32
	if counted {
		counts = make([]uint32, 0, cap(sources))
	}
	left, right := 0, 0
	for left < prior.keyCount() || right < fresh.keyCount() {
		var key string
		lowLeft, highLeft, lowRight, highRight := uint32(0), uint32(0), uint32(0), uint32(0)
		switch {
		case right >= fresh.keyCount() || left < prior.keyCount() && prior.key(left) < fresh.key(right):
			key, lowLeft, highLeft = prior.key(left), prior.Offsets[left], prior.Offsets[left+1]
			left++
		case left >= prior.keyCount() || fresh.key(right) < prior.key(left):
			key, lowRight, highRight = fresh.key(right), fresh.Offsets[right], fresh.Offsets[right+1]
			right++
		default:
			key = prior.key(left)
			lowLeft, highLeft, lowRight, highRight = prior.Offsets[left], prior.Offsets[left+1], fresh.Offsets[right], fresh.Offsets[right+1]
			left++
			right++
		}
		start := len(sources)
		for lowLeft < highLeft || lowRight < highRight {
			if lowLeft < highLeft && remap[prior.Sources[lowLeft]] < 0 {
				lowLeft++
				continue
			}
			takeLeft := lowRight >= highRight || lowLeft < highLeft && uint32(remap[prior.Sources[lowLeft]]) < fresh.Sources[lowRight]
			if takeLeft {
				sources = append(sources, uint32(remap[prior.Sources[lowLeft]]))
				if counted {
					counts = append(counts, prior.Counts[lowLeft])
				}
				lowLeft++
				continue
			}
			sources = append(sources, fresh.Sources[lowRight])
			if counted {
				counts = append(counts, fresh.Counts[lowRight])
			}
			lowRight++
		}
		if len(sources) == start {
			continue
		}
		keyBytes = append(keyBytes, key...)
		keyOffsets = append(keyOffsets, uint32(len(keyBytes)))
		offsets = append(offsets, uint32(len(sources)))
	}
	return termPostings{KeyBytes: string(keyBytes), KeyOffsets: keyOffsets, Offsets: offsets, Sources: sources, Counts: counts}
}
