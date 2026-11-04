package contextindex

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// The opt-in orderings of the step that follows slot admission
// (docs/specs/context-ranking-v0.md, CRK-V0-004..008). Unset, `off` or any
// other value leaves decision 0027's corroboration order and the packet bytes
// unchanged. Neither ordering admits, withholds or caps a row: membership is
// the slots' (TCP-V0-049), and only the order the `--limit` truncation follows
// changes.

// contextRRFConstant is TCP-V0-048's k, fixed before any reading.
const contextRRFConstant = 60

// contextAdditiveMaxScore keeps an additive row's reported score below a
// governing row's 1000 (TCP-V0-008).
const contextAdditiveMaxScore = 999

// contextSlotBonus is CRK-V0-005's additive slot evidence, in the units of
// the lexical band (0..contextLexicalCeiling-contextLexicalBase). It was fixed
// before any reading: a path the task names outright outweighs any term
// match, a structural relation is worth half the band, a weaker relation a
// third, and a convention-only relation a sixth. A kind not listed adds 0.
var contextSlotBonus = map[string]int{
	"mentioned":      300,
	"pair":           150,
	"definition":     150,
	"calls-local":    150,
	"calls-imported": 150,
	"ruby-constant":  150,
	"reverse-import": 100,
	"reference":      100,
	"cochange":       50,
	"sibling":        50,
	"test":           50,
}

// configureContextOrder reads the two opt-in orderings (CRK-V0-004, -005) and
// the opt-in acronym rule (CRK-V0-006).
func configureContextOrder(compiler *taskContextCompiler) *taskContextCompiler {
	compiler.rrfOrder = runtimeenv.Value("CONTEXT_RRF") == "on"
	compiler.additiveOrder = runtimeenv.Value("CONTEXT_ADDITIVE") == "on"
	compiler.acronymProse = runtimeenv.Value("CONTEXT_ACRONYMS") == "prose"
	if compiler.rrfOrder {
		compiler.relationRank = map[string]map[string]int{}
		compiler.relationSeen = map[string]int{}
		compiler.relationOrder = map[string]int{}
	}
	return compiler
}

// recordRelationRanks keeps, for RRF, each path's 1-based position in a
// relation's materialised candidate list with the subject removed
// (TCP-V0-048's rank_r). A relation's list spans every slot call that yields
// its kind, so the position counts the earlier calls' candidates too.
func (compiler *taskContextCompiler) recordRelationRanks(candidates []contextRow) {
	if !compiler.rrfOrder {
		return
	}
	for _, candidate := range candidates {
		if candidate.path == compiler.subject {
			continue
		}
		if compiler.relationOrder == nil {
			compiler.relationOrder = map[string]int{}
		}
		if _, seen := compiler.relationOrder[candidate.kind]; !seen {
			compiler.relationOrder[candidate.kind] = len(compiler.relationOrder)
		}
		compiler.relationSeen[candidate.kind]++
		ranks := compiler.relationRank[candidate.path]
		if ranks == nil {
			ranks = map[string]int{}
			compiler.relationRank[candidate.path] = ranks
		}
		if _, ok := ranks[candidate.kind]; !ok {
			ranks[candidate.kind] = compiler.relationSeen[candidate.kind]
		}
	}
}

// orderRows is the ordering step after admission: RRF when it is on
// (CRK-V0-007: it governs when both are on), else the additive order, else
// decision 0027's corroboration count. The score rise and the "; also" list
// are corroborate's in every arm (TCP-V0-049).
func (compiler *taskContextCompiler) orderRows(rows []contextRow) []contextRow {
	switch {
	case compiler.rrfOrder:
		return compiler.fuseRanks(compiler.annotateSupport(rows))
	case compiler.additiveOrder:
		return compiler.addSlotBonuses(compiler.annotateSupport(rows))
	default:
		return compiler.corroborate(rows)
	}
}

// fuseRanks sorts rows by F(p) = sum over r of 1/(k + rank_r(p)), over the
// admitting relation and every corroborating one, descending and stable, and
// names the terms in each row's reason (TCP-V0-048/049, CRK-V0-004).
func (compiler *taskContextCompiler) fuseRanks(rows []contextRow) []contextRow {
	fused := make(map[string]float64, len(rows))
	for index := range rows {
		row := &rows[index]
		relations := append([]string{row.kind}, compiler.support[row.path]...)
		terms := make([]string, 0, len(relations))
		score := 0.0
		for _, relation := range relations {
			rank, ok := compiler.relationRank[row.path][relation]
			if !ok {
				continue
			}
			score += 1 / float64(contextRRFConstant+rank)
			terms = append(terms, relation+"@"+strconv.Itoa(rank))
		}
		fused[row.path] = score
		row.reason += "; rrf " + strconv.FormatFloat(score, 'f', 4, 64) + " (" + strings.Join(terms, ", ") + ")"
	}
	sort.SliceStable(rows, func(left, right int) bool {
		a, b := rows[left], rows[right]
		if fused[a.path] != fused[b.path] {
			return fused[a.path] > fused[b.path]
		}
		if compiler.relationOrder[a.kind] != compiler.relationOrder[b.kind] {
			return compiler.relationOrder[a.kind] < compiler.relationOrder[b.kind]
		}
		return a.path < b.path
	})
	return rows
}

// fuseSelectedRows changes only ordinary positions in the final selected packet.
// Reservations and graph rows keep their positions, fields and membership.
func (compiler *taskContextCompiler) fuseSelectedRows(rows []contextRow) []contextRow {
	ordinary := make([]contextRow, 0, len(rows))
	positions := make([]int, 0, len(rows))
	for position, row := range rows {
		if reservedRelation(row.kind) || row.kind == "graph" {
			continue
		}
		ordinary = append(ordinary, row)
		positions = append(positions, position)
	}
	ordinary = compiler.fuseRanks(ordinary)
	for index, position := range positions {
		rows[position] = ordinary[index]
	}
	return rows
}

// addSlotBonuses sorts rows by the additive key L(p) + B(kind) + 50 per
// corroborating relation, where L is the path's lexical band from the same
// BM25 walk the fill ranks by (0 for a path the walk did not reach), and
// reports that key as the row's score (CRK-V0-005).
func (compiler *taskContextCompiler) addSlotBonuses(rows []contextRow) []contextRow {
	band := compiler.lexicalBand()
	keys := make(map[string]int, len(rows))
	for index := range rows {
		row := &rows[index]
		lexical, bonus, also := band[row.path], contextSlotBonus[row.kind], 50*len(compiler.support[row.path])
		key := lexical + bonus + also
		keys[row.path] = key
		row.score = min(contextLexicalBase+key, contextAdditiveMaxScore)
		row.reason += "; additive " + strconv.Itoa(key) + " (lexical " + strconv.Itoa(lexical) + ", " + row.kind + " +" +
			strconv.Itoa(bonus) + ", also +" + strconv.Itoa(also) + ")"
	}
	sort.SliceStable(rows, func(left, right int) bool { return keys[rows[left].path] > keys[rows[right].path] })
	return rows
}

// lexicalBand maps each path the lexical walk reached to its share of the
// lexical band, the lexicalScore a lexical row for it would carry less the
// band's base.
func (compiler *taskContextCompiler) lexicalBand() map[string]int {
	hits := compiler.lexicalHits()
	band := make(map[string]int, len(hits))
	if len(hits) == 0 {
		return band
	}
	strongest := hits[0].score
	for _, hit := range hits {
		if _, seen := band[hit.path]; !seen {
			band[hit.path] = lexicalScore(hit.score, strongest) - contextLexicalBase
		}
	}
	return band
}

// annotateSupport is corroborate's score rise and summary without its sort.
func (compiler *taskContextCompiler) annotateSupport(rows []contextRow) []contextRow {
	for index := range rows {
		others := compiler.support[rows[index].path]
		if len(others) == 0 {
			continue
		}
		rows[index].score += 50 * len(others)
		rows[index].summary += "; also " + strings.Join(others, ", ")
		rows[index].reason = discloseFuzzy(rows[index].reason, compiler.fuzzySupport[rows[index].path])
	}
	return rows
}

// slotDefinitionEligible is definitionSlotEligible (TCP-V0-961) for the
// task-context definition slot. With `CORVINT_CONTEXT_ACRONYMS=prose` an unquoted
// all-capital word such as `CEM` is prose, not a code name the task gives (CRK-V0-006): the word
// names a concept many sources discuss, and its one declaring type is rarely
// the file the task means.
func (compiler *taskContextCompiler) slotDefinitionEligible(identifier taskIdentifier) bool {
	if compiler.acronymProse && identifier.weight < 3 && allCapitalWord(identifier.name) {
		return false
	}
	return compiler.definitionSlotEligible(identifier)
}

func allCapitalWord(name string) bool {
	if len(name) < 2 {
		return false
	}
	for index := 0; index < len(name); index++ {
		if !isASCIIUpper(name[index]) {
			return false
		}
	}
	return true
}

// rankSelectedAdditively keeps reservations and graph positions and every
// selected path. Scores reflect lexical strength plus the existing slot bonus.
func (compiler *taskContextCompiler) rankSelectedAdditively(rows []contextRow) []contextRow {
	positions := []int{}
	ordinary := []contextRow{}
	for i, row := range rows {
		if reservedRelation(row.kind) || row.kind == "graph" {
			continue
		}
		positions = append(positions, i)
		ordinary = append(ordinary, row)
	}
	// Keep decision 0027's corroboration order. Additive scores describe
	// strength; promoting a global additive order still needs CRK-V0-008.
	band := compiler.lexicalBand()
	for i := range ordinary {
		row := &ordinary[i]
		if row.kind != "definition" && row.kind != "test" {
			continue
		}
		lexical, bonus, also := band[row.path], contextSlotBonus[row.kind], 50*len(compiler.support[row.path])
		key := lexical + bonus + also
		row.score = min(contextLexicalBase+key, contextAdditiveMaxScore)
		row.reason += "; additive " + strconv.Itoa(key) + " (lexical " + strconv.Itoa(lexical) + ", " + row.kind + " +" + strconv.Itoa(bonus) + ", also +" + strconv.Itoa(also) + ")"
	}
	// A lookup asks for implementation. Keep a convention-paired test after
	// its source when both survived selection; never invent an absent source.
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(compiler.task)), "where ") {
		for i := 0; i < len(ordinary); i++ {
			anchor := ordinary[i].testAnchor
			if anchor == "" || contextIsTest(anchor) {
				continue
			}
			for j := i + 1; j < len(ordinary); j++ {
				if ordinary[j].path == anchor {
					test := ordinary[i]
					copy(ordinary[i:j], ordinary[i+1:j+1])
					ordinary[j] = test
					i--
					break
				}
			}
		}
	}
	for i, position := range positions {
		rows[position] = ordinary[i]
	}
	return rows
}
