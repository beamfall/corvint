package main

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

type queryWideningKey struct{}

// withQueryWidening marks one standalone `corvint query` run, or its byte-equal
// `batch` query operation (GPK-V0-090): the receipt gains the additive
// `widening` member. The harness user-prompt and `prove` paths never set it,
// so their packets keep their bytes.
func withQueryWidening(ctx context.Context) context.Context {
	return context.WithValue(ctx, queryWideningKey{}, true)
}

func queryWideningRequested(ctx context.Context) bool {
	requested, _ := ctx.Value(queryWideningKey{}).(bool)
	return requested
}

// queryWideningDisclosure states what the widening rows are and are not.
const queryWideningDisclosure = "widening rows fuse the task-context slot ranking with the lexical identifier channel " +
	"over the same pinned revision (reciprocal rank fusion; each row's `fusion` names its rank in each list); " +
	"they are lower-confidence candidates beneath results[] and this member is never authority evidence, " +
	"whatever a row's own authority or trust labels say about its document (reserved governing rows are excluded); " +
	"a row is carried only when its document names a task identifier by exact spelling, and rows below the " +
	"fused-score cutoff are withheld and counted; widening yields packet budget to results[] first"

// The fusion constants of GPK-V0-092, chosen once on the preregistered
// ContextBench dev split (benchmarks/results/v2716-query-fusion-prereg-2026-10-26.md,
// amendment of 2026-10-26) and then frozen before the held-out and full runs.
const (
	// queryFusionConstant is the reciprocal-rank-fusion constant k: a rank r
	// contributes 1/(k+r).
	queryFusionConstant = 10
	// queryFusionLexicalWeightPercent scales the lexical list's contribution,
	// in percent of the task-context list's (packet numbers are integers).
	// It is not a tuned parameter: both lists weigh the same.
	queryFusionLexicalWeightPercent = 100
	// queryFusionRelativeFloorPercent withholds a row whose fused score is
	// below this percentage of the top fused score.
	queryFusionRelativeFloorPercent = 50
	// queryFusionGapRatioPercent withholds every row after the first fused
	// score that drops below this percentage of the score before it.
	queryFusionGapRatioPercent = 70
	// queryFusionCap is the most widened rows a receipt carries, within the
	// quota the confident rows leave.
	queryFusionCap = 10
)

// fusedRow is one eligible widening candidate with its rank in each list:
// zero marks absence from that list.
type fusedRow struct {
	path        string
	row         map[string]any
	structural  int
	lexical     int
	identifiers []string
	score       float64
}

// widenQueryResult appends the `widening` member to a compiled query receipt:
// the fused ranking of the task-context rows and the lexical identifier
// channel for the same task over the same index, excluding the paths
// `results[]` already names and the reserved governing rows, up to the
// requested limit minus the confident rows (GPK-V0-090, GPK-V0-092). Each row
// carries a `fusion` member naming its rank in each list and the task
// identifiers it names, so its inclusion is explained from evidence, never
// from a bare score. A row whose document names no task identifier, and a
// row below the fused-score cutoff, is withheld and counted in `withheld`;
// eligible rows past the quota are counted in
// `omitted_results` (state `QUOTA_FILLED` when none fits). A budgeted packet
// drops widening rows from the tail until the receipt fits, and drops the
// member when no row fits. The confident `results[]`, its state and its
// abstention are never changed.
func widenQueryResult(ctx context.Context, index *contextindex.Index, result map[string]any, task string, limit int, budget *int) (map[string]any, error) {
	confident := mapsFromAny(result["results"])
	widening := map[string]any{
		"profile": "task-context", "disclosure": queryWideningDisclosure,
		"ranking": map[string]any{
			"method": "rrf", "constant": queryFusionConstant, "lexical_weight_percent": queryFusionLexicalWeightPercent,
			"lists":  []any{"task-context", "lexical-identifier"},
			"cutoff": map[string]any{"relative_floor_percent": queryFusionRelativeFloorPercent, "gap_ratio_percent": queryFusionGapRatioPercent, "cap": queryFusionCap},
		},
		"state": "NO_CANDIDATES", "results": []any{}, "omitted_results": 0,
	}
	result["widening"] = widening
	named := map[string]struct{}{}
	for _, row := range confident {
		for _, evidence := range mapsFromAny(row["evidence"]) {
			if path := stringAt(evidence, "path"); path != "" {
				named[path] = struct{}{}
			}
		}
	}
	quota := max(limit-len(confident), 0)
	uncertainty := []any{}
	fused := map[string]*fusedRow{}
	order := []*fusedRow{}
	admit := func(path string, row map[string]any) *fusedRow {
		if entry, ok := fused[path]; ok {
			return entry
		}
		entry := &fusedRow{path: path, row: row}
		fused[path] = entry
		order = append(order, entry)
		return entry
	}
	packet, err := contextindex.TaskContext(ctx, index, task, "", limit)
	if err != nil {
		if ctx.Err() != nil {
			return nil, err
		}
		widening["state"] = "UNAVAILABLE"
		uncertainty = append(uncertainty, "task-context ranking unavailable: "+err.Error())
	}
	structuralRank := 0
	for _, row := range mapsFromAny(packet["results"]) {
		id := stringAt(row, "id")
		// Reserved governing rows are authority, which the widening tier
		// never carries; the authority-start profile and the harness
		// session-start block serve them. Naming the path keeps the lexical
		// channel from readmitting the same file as a documentation row.
		if stringAt(row, "kind") == "governing" && id != "" {
			named[id] = struct{}{}
		}
		if _, taken := named[id]; taken || id == "" {
			continue
		}
		if entry := admit(id, row); entry.structural == 0 {
			structuralRank++
			entry.structural = structuralRank
			entry.score += 1 / float64(queryFusionConstant+structuralRank)
		}
	}
	channel, err := contextindex.TaskLexicalChannel(ctx, index, task, limit)
	if err != nil {
		if ctx.Err() != nil {
			return nil, err
		}
		uncertainty = append(uncertainty, "lexical identifier channel unavailable: "+err.Error())
	} else {
		widening["lexical"] = map[string]any{"identifiers": stringsToAny(channel.Identifiers), "terms": channel.Terms, "candidates": len(channel.Candidates)}
		if len(channel.Identifiers) == 0 {
			uncertainty = append(uncertainty, "task names no identifier; widening is withheld (every candidate rests on prose terms or on the structural ranking alone)")
		}
		lexicalRank := 0
		for _, candidate := range channel.Candidates {
			if _, taken := named[candidate.Path]; taken || candidate.Path == "" {
				continue
			}
			lexicalRank++
			entry := admit(candidate.Path, candidate.Row)
			entry.lexical = lexicalRank
			entry.identifiers = candidate.Identifiers
			entry.score += float64(queryFusionLexicalWeightPercent) / 100 / float64(queryFusionConstant+lexicalRank)
		}
	}
	if widening["state"] == "UNAVAILABLE" {
		// A failed task-context ranking leaves results[] intact and reports
		// the failure (GPK-V0-090); the lexical channel alone does not widen.
		order = nil
	}
	sort.SliceStable(order, func(i, j int) bool {
		left, right := order[i], order[j]
		leftAuthority := rowTrust(left.row) == contextindex.TrustProjectAuthority
		rightAuthority := rowTrust(right.row) == contextindex.TrustProjectAuthority
		if leftAuthority != rightAuthority {
			return leftAuthority
		}
		if left.score != right.score {
			return left.score > right.score
		}
		if rankOrder(left.structural) != rankOrder(right.structural) {
			return rankOrder(left.structural) < rankOrder(right.structural)
		}
		if rankOrder(left.lexical) != rankOrder(right.lexical) {
			return rankOrder(left.lexical) < rankOrder(right.lexical)
		}
		return left.path < right.path
	})
	// The identifier gate: a fused row is carried only when the lexical
	// channel ranked it among its candidates and its document names a task
	// identifier by exact spelling (the channel attaches the identifiers to
	// the candidates it ranks). A row that rests on prose terms or on the
	// structural ranking alone is withheld, so a task that names no
	// identifier widens to nothing and says so.
	gated := make([]*fusedRow, 0, len(order))
	unnamed := 0
	for _, entry := range order {
		if len(entry.identifiers) == 0 {
			unnamed++
			continue
		}
		gated = append(gated, entry)
	}
	rows := []map[string]any{}
	withheld, withheldReason := 0, ""
	cut := false
	for position, entry := range gated {
		switch {
		case cut:
			withheld++
			continue
		case position >= queryFusionCap:
			cut, withheldReason = true, fmt.Sprintf("ranked past the cap of %d widened rows", queryFusionCap)
			withheld++
			continue
		case entry.score*100 < float64(queryFusionRelativeFloorPercent)*gated[0].score:
			cut, withheldReason = true, fmt.Sprintf("scored below %d%% of the top fused score", queryFusionRelativeFloorPercent)
			withheld++
			continue
		case position > 0 && entry.score*100 < float64(queryFusionGapRatioPercent)*gated[position-1].score:
			cut, withheldReason = true, fmt.Sprintf("scored below %d%% of the fused score before them", queryFusionGapRatioPercent)
			withheld++
			continue
		}
		if len(rows) == quota {
			widening["omitted_results"] = widening["omitted_results"].(int) + 1
			continue
		}
		rows = append(rows, fusedPacketRow(entry))
	}
	if unnamed+withheld > 0 {
		reasons := []string{}
		if unnamed > 0 {
			reasons = append(reasons, fmt.Sprintf("%d %s name no task identifier", unnamed, rowNoun(unnamed)))
		}
		if withheld > 0 {
			reasons = append(reasons, fmt.Sprintf("%d %s %s", withheld, rowNoun(withheld), withheldReason))
		}
		widening["withheld"] = map[string]any{"count": unnamed + withheld, "reason": strings.Join(reasons, "; ")}
	}
	if len(uncertainty) > 0 {
		widening["uncertainty"] = uncertainty
	}
	switch {
	case widening["state"] == "UNAVAILABLE":
	case len(rows) != 0:
		widening["state"] = "READY"
	case widening["omitted_results"].(int) != 0:
		widening["state"] = "QUOTA_FILLED"
	}
	for {
		widening["results"] = rowsToAny(rows)
		if err := contextindex.StabilizePacketBytes(result, budget); err != nil {
			return nil, err
		}
		if budget == nil {
			return result, nil
		}
		encoded, err := contextindex.CanonicalJSON(result)
		if err != nil {
			return nil, &contextindex.Error{Message: "cannot encode context receipt"}
		}
		if len(encoded) <= *budget {
			return result, nil
		}
		if len(rows) != 0 {
			rows = rows[:len(rows)-1]
			widening["omitted_results"] = widening["omitted_results"].(int) + 1
			widening["state"] = "BUDGETED"
		}
		if len(rows) == 0 {
			delete(result, "widening")
			return result, contextindex.StabilizePacketBytes(result, budget)
		}
	}
}

// fusedPacketRow is the widening row for one fused candidate: the list row
// (the task-context row when both lists carry the path, since it names the
// relation; otherwise the lexical row) plus the `fusion` member that explains
// its position from its rank in each list and the task identifiers it names.
// The fused score travels as `rrf_micro`, the score times one million rounded
// to an integer, since canonical packet numbers are integers.
func fusedPacketRow(entry *fusedRow) map[string]any {
	row := make(map[string]any, len(entry.row)+1)
	for key, value := range entry.row {
		row[key] = value
	}
	fusion := map[string]any{"rrf_micro": int(math.Round(entry.score * 1e6))}
	reasons := []string{fmt.Sprintf("rrf %.4f", entry.score)}
	if entry.structural > 0 {
		fusion["structural_rank"] = entry.structural
		reasons = append(reasons, fmt.Sprintf("task-context rank %d", entry.structural))
	}
	if entry.lexical > 0 {
		fusion["lexical_rank"] = entry.lexical
		reason := fmt.Sprintf("lexical rank %d", entry.lexical)
		if len(entry.identifiers) > 0 {
			reason += " (names " + strings.Join(entry.identifiers, ", ") + ")"
		} else {
			reason += " (prose terms only)"
		}
		reasons = append(reasons, reason)
	}
	fusion["identifiers"] = stringsToAny(entry.identifiers)
	fusion["reason"] = strings.Join(reasons, "; ")
	row["fusion"] = fusion
	return row
}

// rowTrust is the trust class of a packet row's first evidence entry.
func rowTrust(row map[string]any) string {
	for _, evidence := range mapsFromAny(row["evidence"]) {
		return stringAt(evidence, "trust")
	}
	return ""
}

// rankOrder orders a list rank with absence (zero) last.
func rankOrder(rank int) int {
	if rank == 0 {
		return math.MaxInt
	}
	return rank
}

func stringsToAny(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func rowsToAny(rows []map[string]any) []any {
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	return out
}

// rowNoun agrees the withheld-reason noun with its count.
func rowNoun(count int) string {
	if count == 1 {
		return "row"
	}
	return "rows"
}
