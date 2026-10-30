package contextindex

import (
	"encoding/binary"
	"errors"
	"sort"
	"unicode/utf8"
)

// identGraph is the identifier definition/reference graph (TCP-V0-030): one
// node per term-table source id, and an edge between two sources when one
// names, as a whole identifier word, a name the other defines. It is derived
// from the index's own Symbols and Words postings, so it is a pure function
// of the Git content at the indexed revision, and it is carried in the
// snapshot beside the term table.
//
// The adjacency is symmetric and laid out as CSR: node u's edges are
// Targets/Weights/Labels[Offsets[u]:Offsets[u+1]], ascending by target.
// Weight is the number of distinct eligible names joining the pair; Label is
// the rarest of them (fewest referencing sources, then name order) shifted
// left one bit, with the low bit set when u references the name the target
// defines and clear when u defines the name the target references. Every
// value is an integer, so the encoding is byte-deterministic.
type identGraph struct {
	Nodes   uint32
	Bounded bool
	Offsets []uint32
	Targets []uint32
	Weights []uint32
	Labels  []uint32
	// NameBytes and NameOffsets hold the label names in ascending order.
	NameBytes   string
	NameOffsets []uint32
}

// Graph bounds (TCP-V0-031). A name with more definers or more referencing
// sources is a common word and adds no edge, as the reference slot rules
// (decision 0035); a tree past the node or arc bound gets a Bounded graph
// with no edges, and the slot abstains rather than rank a partial graph.
const (
	identGraphMinName       = 4
	identGraphMaxName       = 128
	identGraphMaxDefiners   = 5
	identGraphMaxReferences = 50
	identGraphMaxNodes      = 1 << 20
	identGraphMaxArcs       = 1 << 21
	identGraphReferenceBit  = 1
)

// identGraphArc is one directed arc before the merge: from names or defines
// name, the other end defines or names it.
type identGraphArc struct {
	from, to, name, references uint32
	refers                     bool
}

// buildIdentGraph derives the graph from Symbols and the Words postings of
// table. Names are visited in sorted order and arcs are sorted before the
// merge, so the result does not depend on the symbol or map order.
func (index *Index) buildIdentGraph(table *TermTable) *identGraph {
	nodes := len(table.Paths)
	if nodes > identGraphMaxNodes {
		return boundedIdentGraph(uint32(nodes), 1)
	}
	definers := identGraphDefiners(index.Symbols, table)
	candidates := make([]string, 0, len(definers))
	for name := range definers {
		candidates = append(candidates, name)
	}
	sort.Strings(candidates)
	names := make([]string, 0, len(candidates))
	arcs := make([]identGraphArc, 0)
	for _, name := range candidates {
		defined := definers[name]
		low, high, found := table.Words.find(name)
		if !found || len(defined) > identGraphMaxDefiners || high-low > identGraphMaxReferences {
			continue
		}
		before := len(arcs)
		id, references := uint32(len(names)), uint32(high-low)
		for _, referrer := range table.Words.Sources[low:high] {
			for _, definer := range defined {
				if referrer == definer {
					continue
				}
				arcs = append(arcs,
					identGraphArc{from: referrer, to: definer, name: id, references: references, refers: true},
					identGraphArc{from: definer, to: referrer, name: id, references: references})
			}
		}
		if len(arcs) > identGraphMaxArcs {
			return boundedIdentGraph(uint32(nodes), nodes+1)
		}
		if len(arcs) > before {
			names = append(names, name)
		}
	}
	return mergeIdentGraphArcs(uint32(nodes), arcs, names)
}

// boundedIdentGraph is the edgeless graph a tree past a bound stores. It
// carries one name offset so it passes check and a saved index still loads.
func boundedIdentGraph(nodes uint32, offsets int) *identGraph {
	return &identGraph{Nodes: nodes, Bounded: true, Offsets: make([]uint32, offsets), NameOffsets: []uint32{0}}
}

// identGraphDefiners maps each eligible defined name to its ascending,
// distinct definer source ids.
func identGraphDefiners(symbols []Symbol, table *TermTable) map[string][]uint32 {
	definers := map[string][]uint32{}
	for _, symbol := range symbols {
		if !identGraphName(symbol.Name) {
			continue
		}
		source, ok := table.sourceID(symbol.Path)
		if !ok {
			continue
		}
		definers[symbol.Name] = append(definers[symbol.Name], uint32(source))
	}
	for name, sources := range definers {
		sortedSources := append([]uint32(nil), sources...)
		sort.Slice(sortedSources, func(left, right int) bool { return sortedSources[left] < sortedSources[right] })
		definers[name] = compactUint32(sortedSources)
	}
	return definers
}

// identGraphName admits an ASCII identifier the Words table can carry, using
// the declaration scanners' identifier classes (langsymbols.go), of at least
// identGraphMinName bytes so a receiver or short accessor joins nothing. A
// name of lowercase letters only is prose-shaped (`unexamined`, `seed`): a
// document or comment using the English word would read as a reference.
func identGraphName(name string) bool {
	if len(name) < identGraphMinName || len(name) > identGraphMaxName {
		return false
	}
	prose := true
	for position, character := range name {
		if character >= utf8.RuneSelf || !isIdentifierPart(character) || position == 0 && !isIdentifierStart(character) {
			return false
		}
		prose = prose && character >= 'a' && character <= 'z'
	}
	return !prose
}

func compactUint32(values []uint32) []uint32 {
	kept := values[:0]
	for position, value := range values {
		if position > 0 && value == values[position-1] {
			continue
		}
		kept = append(kept, value)
	}
	return kept
}

// mergeIdentGraphArcs sorts the arcs by (from, to, rarity, name) and folds
// each (from, to) run into one edge whose label is the run's first arc.
func mergeIdentGraphArcs(nodes uint32, arcs []identGraphArc, names []string) *identGraph {
	sort.Slice(arcs, func(left, right int) bool {
		a, b := arcs[left], arcs[right]
		if a.from != b.from {
			return a.from < b.from
		}
		if a.to != b.to {
			return a.to < b.to
		}
		if a.references != b.references {
			return a.references < b.references
		}
		return a.name < b.name
	})
	graph := &identGraph{Nodes: nodes, Offsets: make([]uint32, nodes+1), Targets: []uint32{}, Weights: []uint32{}, Labels: []uint32{}}
	for position, arc := range arcs {
		last := len(graph.Targets) - 1
		if position > 0 && arcs[position-1].from == arc.from && arcs[position-1].to == arc.to {
			graph.Weights[last]++
			continue
		}
		label := arc.name << 1
		if arc.refers {
			label |= identGraphReferenceBit
		}
		graph.Targets = append(graph.Targets, arc.to)
		graph.Weights = append(graph.Weights, 1)
		graph.Labels = append(graph.Labels, label)
		graph.Offsets[arc.from+1]++
	}
	for node := uint32(1); node <= nodes; node++ {
		graph.Offsets[node] += graph.Offsets[node-1]
	}
	graph.NameOffsets = make([]uint32, 0, len(names)+1)
	nameBytes := make([]byte, 0)
	for _, name := range names {
		graph.NameOffsets = append(graph.NameOffsets, uint32(len(nameBytes)))
		nameBytes = append(nameBytes, name...)
	}
	graph.NameOffsets = append(graph.NameOffsets, uint32(len(nameBytes)))
	graph.NameBytes = string(nameBytes)
	return graph
}

// edges returns node's CSR range.
func (graph *identGraph) edges(node uint32) (int, int) {
	return int(graph.Offsets[node]), int(graph.Offsets[node+1])
}

// name returns an edge label's identifier and whether the edge's source
// node references it (true) or defines it (false).
func (graph *identGraph) name(label uint32) (string, bool) {
	id := label >> 1
	return graph.NameBytes[graph.NameOffsets[id]:graph.NameOffsets[id+1]], label&identGraphReferenceBit != 0
}

// check refuses a decoded graph that would index outside its arrays or name a
// node outside the sourceCount paths of the term table it rides in.
func (graph *identGraph) check(sourceCount int) error {
	if graph == nil {
		return nil
	}
	if int(graph.Nodes) != sourceCount && !graph.Bounded {
		return errors.New("identifier graph does not match the vocabulary paths")
	}
	edges := uint32(len(graph.Targets))
	if len(graph.Weights) != int(edges) || len(graph.Labels) != int(edges) {
		return errors.New("identifier graph columns differ in length")
	}
	if !ascendingWithin(graph.Offsets, edges) || len(graph.Offsets) == 0 || graph.Offsets[len(graph.Offsets)-1] != edges {
		return errors.New("identifier graph offsets are not ascending within the edges")
	}
	if !graph.Bounded && len(graph.Offsets) != int(graph.Nodes)+1 {
		return errors.New("identifier graph offsets do not match its nodes")
	}
	if len(graph.NameOffsets) == 0 || !ascendingWithin(graph.NameOffsets, uint32(len(graph.NameBytes))) {
		return errors.New("identifier graph name offsets are not ascending within the names")
	}
	names := uint32(len(graph.NameOffsets) - 1)
	for position, target := range graph.Targets {
		if target >= uint32(len(graph.Offsets)-1) || graph.Labels[position]>>1 >= names {
			return errors.New("identifier graph edge names a node or label outside the graph")
		}
	}
	return nil
}

func ascendingWithin(offsets []uint32, limit uint32) bool {
	for position, offset := range offsets {
		if offset > limit || position > 0 && offset < offsets[position-1] {
			return false
		}
	}
	return true
}

// MarshalBinary lays the graph out fixed-width little-endian so gob and the
// pack move it as one byte string: [u32 nodes][u32 bounded][u32 x 6 counts:
// offsets, targets, weights, labels, name bytes, name offsets][arrays][name
// bytes].
func (graph *identGraph) MarshalBinary() ([]byte, error) {
	arrays := [][]uint32{graph.Offsets, graph.Targets, graph.Weights, graph.Labels}
	bounded := uint32(0)
	if graph.Bounded {
		bounded = 1
	}
	header := []uint32{graph.Nodes, bounded, uint32(len(graph.Offsets)), uint32(len(graph.Targets)), uint32(len(graph.Weights)),
		uint32(len(graph.Labels)), uint32(len(graph.NameBytes)), uint32(len(graph.NameOffsets))}
	data := make([]byte, 0, 4*len(header)+4*(len(graph.Offsets)+3*len(graph.Targets)+len(graph.NameOffsets))+len(graph.NameBytes))
	for _, value := range header {
		data = binary.LittleEndian.AppendUint32(data, value)
	}
	for _, array := range append(arrays, graph.NameOffsets) {
		for _, value := range array {
			data = binary.LittleEndian.AppendUint32(data, value)
		}
	}
	return append(data, graph.NameBytes...), nil
}

func (graph *identGraph) UnmarshalBinary(data []byte) error {
	const headerWords = 8
	if len(data) < 4*headerWords {
		return errors.New("identifier graph header is truncated")
	}
	header := make([]uint64, headerWords)
	for position := range header {
		header[position] = uint64(binary.LittleEndian.Uint32(data[4*position:]))
	}
	if header[1] > 1 {
		return errors.New("identifier graph bounded flag is not 0 or 1")
	}
	offset := uint64(4 * headerWords)
	decoded := identGraph{Nodes: uint32(header[0]), Bounded: header[1] == 1}
	targets := [5]*[]uint32{&decoded.Offsets, &decoded.Targets, &decoded.Weights, &decoded.Labels, &decoded.NameOffsets}
	counts := [5]uint64{header[2], header[3], header[4], header[5], header[7]}
	for position, target := range targets {
		end := offset + 4*counts[position]
		if end > uint64(len(data)) || end < offset {
			return errors.New("identifier graph array lies outside its encoding")
		}
		values := make([]uint32, counts[position])
		for item := range values {
			values[item] = binary.LittleEndian.Uint32(data[offset+4*uint64(item):])
		}
		*target, offset = values, end
	}
	if offset+header[6] != uint64(len(data)) {
		return errors.New("identifier graph names do not end the encoding")
	}
	decoded.NameBytes = string(data[offset:])
	*graph = decoded
	return nil
}
