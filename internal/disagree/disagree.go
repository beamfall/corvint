// Package disagree reports whether two independent retrieval channels agree on
// the files a task needs (retriever-disagreement-v0). It proposes an
// answerability state for the abstention layer; it changes no threshold, no
// packet, and no stored state.
package disagree

import (
	"context"
	"math"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// Disclosure is the fixed statement every report carries (RDS-V0-007).
const Disclosure = "proposal for the abstention layer: this command changes no threshold and no packet, " +
	"and is read-only"

// agreement thresholds over the top-K Jaccard overlap (RDS-V0-004).
const (
	highAgreementJaccard = 0.34
	topKCeiling          = 10
)

// Signal builds both channels over one already-loaded index and reports their
// agreement. It performs no I/O beyond what the packet itself performs.
func Signal(ctx context.Context, index *contextindex.Index, task, subject string, limit int) (map[string]any, error) {
	packet, err := contextindex.TaskContext(ctx, index, task, subject, limit)
	if err != nil {
		return nil, err
	}
	lexical := lexicalChannel(packet)
	structural := structuralChannel(index, task, subject, limit)
	top := topK(limit)
	signal := compare(lexical, structural, top)
	return map[string]any{
		"tool": "answerability", "ok": true, "mutates": false, "schema_version": 1,
		"revision": index.Revision, "disclosure": Disclosure,
		"request":   map[string]any{"limit": limit, "top_k": top, "task_chars": len(strings.TrimSpace(task)), "subject": subject},
		"channel_l": channelRows(lexical),
		"channel_s": channelRows(structural),
		"signal":    signal,
		"coverage":  coverage(index, packet),
	}, nil
}

// candidate is one ranked path with the reason its own channel admitted it.
type candidate struct {
	path   string
	reason string
}

func topK(limit int) int {
	if limit < topKCeiling {
		return limit
	}
	return topKCeiling
}

// lexicalChannel reads the ordinary task-context packet's result order
// (RDS-V0-001): contextindex.TaskContext is the only exported entry point that
// yields a ranked path list, so the packet is channel L verbatim.
func lexicalChannel(packet map[string]any) []candidate {
	results, _ := packet["results"].([]any)
	rows := make([]candidate, 0, len(results))
	for _, raw := range results {
		result, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		path, _ := result["id"].(string)
		if path == "" {
			continue
		}
		rows = append(rows, candidate{path: path, reason: packetReason(result)})
	}
	return rows
}

func packetReason(result map[string]any) string {
	rows, _ := result["evidence"].([]any)
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if reason, _ := row["reason"].(string); reason != "" {
			return "packet relation: " + reason
		}
	}
	return "packet relation: unstated"
}

// structuralChannel ranks paths reached only through index structure
// (RDS-V0-002): identifier-shaped tokens of the task text that are declared
// symbol names (RDS-V0-962), the files declaring them, the files referencing a
// declared name structurally (RDS-V0-960), and one hop of `Index.Imports` in
// each direction, a Go package counting once (RDS-V0-961). No co-change
// expansion is performed: internal/contextindex/pack_cochange.go exports no
// accessor, and this command may not modify that package.
func structuralChannel(index *contextindex.Index, task, subject string, limit int) []candidate {
	declared := declaredIdentifiers(index, task)
	if len(declared) == 0 {
		return nil
	}
	links := map[string]map[string]struct{}{}
	link := func(path, reason string) {
		if path == "" || path == subject {
			return
		}
		if _, ok := index.Sources[path]; !ok {
			return
		}
		if links[path] == nil {
			links[path] = map[string]struct{}{}
		}
		links[path][reason] = struct{}{}
	}
	targets, packages := importTargets(index)
	definers, seedPackages := map[string]map[string]struct{}{}, map[string]struct{}{}
	for _, symbol := range index.Symbols {
		if _, ok := declared[symbol.Name]; !ok {
			continue
		}
		if definers[symbol.Name] == nil {
			definers[symbol.Name] = map[string]struct{}{}
		}
		definers[symbol.Name][symbol.Path] = struct{}{}
		if path.Ext(symbol.Path) == ".go" {
			seedPackages[path.Dir(symbol.Path)] = struct{}{}
		}
		link(symbol.Path, "declares "+symbol.Name)
	}
	seeds := map[string]struct{}{}
	for _, paths := range definers {
		for definer := range paths {
			seeds[definer] = struct{}{}
		}
	}
	occurrences := map[string]int{}
	for name, paths := range definers {
		for reference, count := range structuralReferences(index, targets, packages, name, paths) {
			link(reference, "references "+name)
			occurrences[reference] += count
		}
	}
	for seed := range seeds {
		for specifier := range index.Imports[seed] {
			if directory, ok := packages.directories[specifier]; ok {
				link(packages.representative[directory], "imported by "+seed)
				continue
			}
			for imported := range targets[specifier] {
				link(imported, "imported by "+seed)
			}
		}
	}
	for importer, specifiers := range index.Imports {
		for specifier := range specifiers {
			if directory, ok := packages.directories[specifier]; ok {
				if _, seeded := seedPackages[directory]; seeded && path.Dir(importer) != directory {
					link(importer, "imports "+specifier)
				}
				continue
			}
			for target := range targets[specifier] {
				if _, ok := seeds[target]; ok && importer != target {
					link(importer, "imports "+target)
				}
			}
		}
	}
	return rankLinks(links, occurrences, limit)
}

// structuralReferences counts, per file, the whole-word occurrences of a
// declared name in the files where that name resolves structurally
// (RDS-V0-960): a Go file in a definer's own package, where the name needs no
// import, and a file whose import resolves to a definer or its Go package. A
// file naming the word anywhere else is prose, and a file in scope that never
// names it is only an import hop. Definers themselves are not references, and
// a source whose text is not loaded counts nothing.
func structuralReferences(index *contextindex.Index, targets map[string]map[string]struct{}, packages goPackages, name string, definers map[string]struct{}) map[string]int {
	directories := map[string]struct{}{}
	for definer := range definers {
		if path.Ext(definer) == ".go" {
			directories[path.Dir(definer)] = struct{}{}
		}
	}
	scope := map[string]struct{}{}
	for source := range index.Sources {
		if _, ok := directories[path.Dir(source)]; ok && path.Ext(source) == ".go" {
			scope[source] = struct{}{}
		}
	}
	for importer, specifiers := range index.Imports {
		for specifier := range specifiers {
			if directory, ok := packages.directories[specifier]; ok {
				if _, seeded := directories[directory]; seeded {
					scope[importer] = struct{}{}
				}
				continue
			}
			for target := range targets[specifier] {
				if _, ok := definers[target]; ok {
					scope[importer] = struct{}{}
				}
			}
		}
	}
	counts := map[string]int{}
	for source := range scope {
		if _, defining := definers[source]; defining {
			continue
		}
		text, valid, _ := index.Sources[source].Text()
		if !valid {
			continue
		}
		if count := wholeWordCount(text, name); count > 0 {
			counts[source] = count
		}
	}
	return counts
}

// wholeWordCount counts the occurrences of word in text bounded on both sides
// by a byte that cannot continue an identifier.
func wholeWordCount(text, word string) int {
	count := 0
	for from := 0; ; {
		at := strings.Index(text[from:], word)
		if at < 0 {
			return count
		}
		start, end := from+at, from+at+len(word)
		if (start == 0 || !identifierByte(text[start-1])) && (end == len(text) || !identifierByte(text[end])) {
			count++
		}
		from = end
	}
}

func identifierByte(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// goPackages resolves a Go package import path to its directory and to one
// representative source, so an import of a package is one structural link, not
// one per file of the package (RDS-V0-961).
type goPackages struct {
	directories    map[string]string
	representative map[string]string
}

// importTargets resolves the raw specifiers `Index.Imports` records to the
// source paths of the same index. A Go package import path (module prefix
// plus directory, mirroring the index's own reverse-import rule) resolves to
// the package once, through goPackages. Every source is additionally reachable
// by its own path and by that path without its extension, which is how a
// relative specifier is written. A Python source is also reachable by its
// dotted module spellings (`contextindex.PythonImportCandidates`), since
// `index.Imports` records a dotted specifier (`import a.b.c`, `from a.b import
// c`) rather than a path for Python. A specifier no source in the index
// answers to is dropped: an edge out of the repository is not a structural
// link inside it.
func importTargets(index *contextindex.Index) (map[string]map[string]struct{}, goPackages) {
	targets := map[string]map[string]struct{}{}
	register := func(specifier, target string) {
		if specifier == "" {
			return
		}
		if targets[specifier] == nil {
			targets[specifier] = map[string]struct{}{}
		}
		targets[specifier][target] = struct{}{}
	}
	packages := goPackages{directories: map[string]string{}, representative: map[string]string{}}
	for source := range index.Sources {
		register(source, source)
		register(strings.TrimSuffix(source, path.Ext(source)), source)
		switch path.Ext(source) {
		case ".go":
			// A _test.go file belongs to no importable package.
			if strings.HasSuffix(source, "_test.go") {
				continue
			}
			directory := path.Dir(source)
			packages.directories[goPackagePath(index.Module, source)] = directory
			if current, ok := packages.representative[directory]; !ok || representativeRank(source) < representativeRank(current) ||
				representativeRank(source) == representativeRank(current) && source < current {
				packages.representative[directory] = source
			}
		case ".py":
			for candidate := range contextindex.PythonImportCandidates(source) {
				register(candidate, source)
			}
		}
	}
	return targets, packages
}

// representativeRank prefers a package's doc.go, then the file named after
// its directory, then any other file (ties broken by path).
func representativeRank(source string) int {
	switch path.Base(source) {
	case "doc.go":
		return 0
	case path.Base(path.Dir(source)) + ".go":
		return 1
	}
	return 2
}

// goPackagePath is the import path a Go source's package is named by.
func goPackagePath(module, source string) string {
	directory := path.Dir(source)
	switch {
	case module == "":
		return directory
	case directory == ".":
		return module
	}
	return module + "/" + directory
}

// declaredIdentifiers keeps the task's identifier-shaped tokens that name a
// symbol the index declares (RDS-V0-962); every other token is prose and
// cannot vote structurally, even when it spells a declared name.
func declaredIdentifiers(index *contextindex.Index, task string) map[string]struct{} {
	tokens := identifierTokens(task)
	declared := map[string]struct{}{}
	for _, symbol := range index.Symbols {
		if _, ok := tokens[symbol.Name]; ok {
			declared[symbol.Name] = struct{}{}
		}
	}
	return declared
}

// identifierTokens splits task on every non-`[A-Za-z0-9_]` rune and keeps the
// tokens shaped like code rather than prose: any token quoted in backticks,
// and otherwise one holding an underscore anywhere (`rank_links`, `_refresh`,
// `__init__`), or an upper-case letter with a lower-case letter elsewhere
// (`EvalQuery`, `rankLinks`, `Fetch`). An all-lower-case word (`signal`,
// `where`) and an all-upper-case acronym (`CEM`, `PR`) are prose unless quoted.
func identifierTokens(task string) map[string]struct{} {
	tokens := map[string]struct{}{}
	for index, quoted := range strings.Split(task, "`") {
		for _, token := range strings.FieldsFunc(quoted, func(r rune) bool {
			return !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
		}) {
			// Odd segments of a split on backticks lie between a pair.
			if index%2 == 1 || identifierShaped(token) {
				tokens[token] = struct{}{}
			}
		}
	}
	return tokens
}

func identifierShaped(token string) bool {
	if strings.Contains(token, "_") {
		return true
	}
	upper, lower := strings.ContainsAny(token, "ABCDEFGHIJKLMNOPQRSTUVWXYZ"), strings.ContainsAny(token, "abcdefghijklmnopqrstuvwxyz")
	return upper && lower
}

// linkTier orders a declaring file before a referencing file before a file
// reached by an import hop alone (RDS-V0-005, RDS-V0-963), so the file that
// declares a named symbol is never ranked out of the top K by import fan-out.
func linkTier(reasons map[string]struct{}) int {
	tier := 2
	for reason := range reasons {
		switch {
		case strings.HasPrefix(reason, "declares "):
			return 0
		case strings.HasPrefix(reason, "references "):
			tier = 1
		}
	}
	return tier
}

// rankLinks orders by link tier, then by the number of distinct structural
// links, then by the reference occurrences, then by path, so one index yields
// one order (RDS-V0-005, RDS-V0-963).
func rankLinks(links map[string]map[string]struct{}, occurrences map[string]int, limit int) []candidate {
	paths := make([]string, 0, len(links))
	for path := range links {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		if left, right := linkTier(links[paths[i]]), linkTier(links[paths[j]]); left != right {
			return left < right
		}
		left, right := len(links[paths[i]]), len(links[paths[j]])
		if left != right {
			return left > right
		}
		if left, right := occurrences[paths[i]], occurrences[paths[j]]; left != right {
			return left > right
		}
		return paths[i] < paths[j]
	})
	if len(paths) > limit {
		paths = paths[:limit]
	}
	rows := make([]candidate, 0, len(paths))
	for _, path := range paths {
		reasons := make([]string, 0, len(links[path]))
		for reason := range links[path] {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)
		rows = append(rows, candidate{path: path, reason: strings.Join(reasons, "; ")})
	}
	return rows
}

func channelRows(rows []candidate) []any {
	encoded := make([]any, 0, len(rows))
	for position, row := range rows {
		encoded = append(encoded, map[string]any{"rank": position + 1, "path": row.path, "reason": row.reason})
	}
	return encoded
}

// compare reports the top-K overlap, the overlap-weighted rank correlation, and
// the categorical agreement and proposed state (RDS-V0-003, RDS-V0-004,
// RDS-V0-006).
func compare(lexical, structural []candidate, top int) map[string]any {
	lexicalRanks, structuralRanks := ranksOf(lexical, top), ranksOf(structural, top)
	shared := make([]string, 0, len(lexicalRanks))
	for path := range lexicalRanks {
		if _, ok := structuralRanks[path]; ok {
			shared = append(shared, path)
		}
	}
	sort.Strings(shared)
	union := len(lexicalRanks) + len(structuralRanks) - len(shared)
	jaccard := 0.0
	if union > 0 {
		jaccard = float64(len(shared)) / float64(union)
	}
	agreement := "none"
	switch {
	case jaccard >= highAgreementJaccard:
		agreement = "high"
	case len(shared) > 0:
		agreement = "low"
	}
	vote, state := "present", "unanswerable"
	switch {
	case len(structural) == 0:
		vote, state, agreement = "absent", "uncertain", "none"
	case len(lexicalRanks) == 0:
		// Only channel S voted. RDS-V0-006 reserves `unanswerable` for two
		// channels that voted and shared nothing, so one silent channel is
		// uncertainty, not a negative answer.
		state = "uncertain"
	case agreement == "high":
		state = "answerable"
	case agreement == "low":
		state = "uncertain"
	}
	return map[string]any{
		"jaccard_top_k": round(jaccard), "overlap_paths": anyStrings(shared),
		"overlap_size": len(shared), "union_size": union,
		"rank_correlation": correlation(shared, lexicalRanks, structuralRanks),
		"agreement":        agreement, "structural_vote": vote, "proposed_state": state,
		"cochange_expansion": "skipped: internal/contextindex/pack_cochange.go exports no accessor",
	}
}

func ranksOf(rows []candidate, top int) map[string]int {
	ranks := map[string]int{}
	for position, row := range rows {
		if position >= top {
			break
		}
		if _, seen := ranks[row.path]; !seen {
			ranks[row.path] = position + 1
		}
	}
	return ranks
}

// correlation is the Spearman coefficient over the shared paths alone, which is
// the overlap-weighted form: paths only one channel ranked carry no rank pair
// and are already counted by the Jaccard term. Fewer than two shared paths
// admits no coefficient and reports null; there is no tie case, because a
// shared path is in both rank maps and ranksOf gives each path a distinct
// position, so positionsWithin returns a permutation of 1..n.
func correlation(shared []string, lexical, structural map[string]int) any {
	if len(shared) < 2 {
		return nil
	}
	left, right := positionsWithin(shared, lexical), positionsWithin(shared, structural)
	count := float64(len(shared))
	meanLeft, meanRight := (count+1)/2, (count+1)/2
	numerator, leftVariance, rightVariance := 0.0, 0.0, 0.0
	for index := range shared {
		deltaLeft, deltaRight := float64(left[index])-meanLeft, float64(right[index])-meanRight
		numerator += deltaLeft * deltaRight
		leftVariance += deltaLeft * deltaLeft
		rightVariance += deltaRight * deltaRight
	}
	// Both variances are the same positive constant for a permutation of 1..n,
	// so this is a division backstop, not a reachable uncertainty case.
	if leftVariance == 0 || rightVariance == 0 {
		return nil
	}
	return round(numerator / math.Sqrt(leftVariance*rightVariance))
}

// positionsWithin re-ranks the shared paths 1..n inside one channel's order.
func positionsWithin(shared []string, ranks map[string]int) []int {
	order := append([]string(nil), shared...)
	sort.Slice(order, func(i, j int) bool { return ranks[order[i]] < ranks[order[j]] })
	position := map[string]int{}
	for index, path := range order {
		position[path] = index + 1
	}
	positions := make([]int, len(shared))
	for index, path := range shared {
		positions[index] = position[path]
	}
	return positions
}

func round(value float64) float64 { return math.Round(value*10000) / 10000 }

func anyStrings(values []string) []any {
	encoded := make([]any, 0, len(values))
	for _, value := range values {
		encoded = append(encoded, value)
	}
	return encoded
}

// coverage mirrors the packet's own coverage denominators (RDS-V0-008) so a
// channel built over a partly-understood index is never read as complete.
func coverage(index *contextindex.Index, packet map[string]any) map[string]any {
	unparsed := make([]any, 0, len(index.Unparsed))
	for _, item := range index.Unparsed {
		unparsed = append(unparsed, map[string]any{"path": item.Path, "reason": item.Reason})
	}
	row := map[string]any{
		"sources": len(index.Sources), "symbols": len(index.Symbols),
		"unparsed_sources": unparsed, "approximate_imports": index.ApproximateImports,
		"import_sources": len(index.Imports),
	}
	if packetCoverage, ok := packet["coverage"].(map[string]any); ok {
		row["packet"] = packetCoverage
	}
	return row
}
