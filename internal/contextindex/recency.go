package contextindex

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// TCP-V0-035..038 (decision 0369): with `CORVINT_CONTEXT_RECENCY=on` the
// packet reads when each path was last committed within the co-change slot's
// 200-commit window at the indexed commit, and weights the lexical fill and
// the `cochange` slot by a 90-day half-life from the indexed commit's own
// committer time. Nothing is read from the worktree, the clock, or any state
// beyond Git history reachable from the indexed commit; an unset or other
// value leaves the packet bytes unchanged.
const (
	contextRecencyHalfLifeDays = 90
	contextRecencyWeight       = 0.25
	contextBlameWeight         = 0.25
	secondsPerDay              = 86400
)

// contextRecency is one request's history reading. The window is read beside
// the slots that do not need it; await joins it.
type contextRecency struct {
	ctx   context.Context
	index *Index
	ready chan struct{}
	// state is `examined`, or the reason every recency feature abstains.
	state      string
	indexedAt  int64
	commits    int
	windowFull bool
	// oldest is the window's oldest commit when the window is full: blame
	// walks only commits the window reaches.
	oldest     string
	commitTime map[string]int64
	lastTouch  map[string]int64
	blame      map[string]blameTouch
	blamed     int
	owners     *codeOwners
}

func startContextRecency(ctx context.Context, index *Index) *contextRecency {
	if runtimeenv.Value("CONTEXT_RECENCY") != "on" {
		return nil
	}
	recency := &contextRecency{ctx: ctx, index: index, ready: make(chan struct{}), blame: map[string]blameTouch{}}
	go func() {
		defer close(recency.ready)
		recency.read()
	}()
	return recency
}

func (recency *contextRecency) await() *contextRecency {
	<-recency.ready
	return recency
}

func (recency *contextRecency) read() {
	commit := recency.index.CommitRevision
	if !validObjectID(commit, recency.index.ObjectFormat) {
		recency.state = "no-indexed-commit"
		return
	}
	stamp, err := git(recency.ctx, recency.index.Root, maxIdentityBytes, nil, "log", "-1", "--format=%ct", commit)
	if err != nil {
		recency.state = "history-unreadable"
		return
	}
	indexedAt, err := strconv.ParseInt(strings.TrimSpace(string(stamp)), 10, 64)
	if err != nil {
		recency.state = "history-unreadable"
		return
	}
	raw, err := git(recency.ctx, recency.index.Root, maxHistoryBytes, nil, "log", "-z", fmt.Sprintf("-%d", maxHistoryCommits), "--no-merges", "--no-renames", "--decorate-refs=refs/nothing", "--format=%x1e%D%x1d%H%x1f%ct", "--name-only", commit)
	if err != nil {
		recency.state = "history-unreadable"
		return
	}
	recency.indexedAt = indexedAt
	recency.state = recency.parse(dropGraftedCommits(raw))
}

// parse records each commit's committer time and each path's newest commit
// in the window, newest first as Git lists them.
func (recency *contextRecency) parse(raw []byte) string {
	recency.commitTime, recency.lastTouch = map[string]int64{}, map[string]int64{}
	order := make([]string, 0, maxHistoryCommits)
	for _, record := range bytes.Split(raw, []byte{0x1e}) {
		fields := bytes.Split(record, []byte{0})
		header := bytes.Trim(fields[0], "\n")
		if len(header) == 0 {
			continue
		}
		commit, stamp, found := bytes.Cut(header, []byte{0x1f})
		when, err := strconv.ParseInt(string(stamp), 10, 64)
		if !found || err != nil || !validObjectID(string(commit), recency.index.ObjectFormat) {
			return "history-unreadable"
		}
		recency.commitTime[string(commit)] = when
		order = append(order, string(commit))
		for _, rawPath := range fields[1:] {
			touched := string(bytes.Trim(rawPath, "\n"))
			if _, seen := recency.lastTouch[touched]; touched != "" && !seen {
				recency.lastTouch[touched] = when
			}
		}
	}
	recency.commits = len(order)
	recency.windowFull = len(order) >= maxHistoryCommits
	if recency.windowFull {
		recency.oldest = order[len(order)-1]
	}
	return "examined"
}

// decay is the 90-day half-life weight of a commit time; a commit dated after
// the indexed commit (a rebase, a skewed clock) weighs 1.
func (recency *contextRecency) decay(when int64) float64 {
	age := max(recency.indexedAt-when, 0)
	return math.Pow(0.5, float64(age)/float64(contextRecencyHalfLifeDays*secondsPerDay))
}

func (recency *contextRecency) ageDays(when int64) int64 {
	return max(recency.indexedAt-when, 0) / secondsPerDay
}

// weight is a path's recency feature, and whether the window observed it.
func (recency *contextRecency) weight(candidate string) (float64, bool) {
	when, ok := recency.lastTouch[candidate]
	if recency.state != "examined" || !ok {
		return 0, false
	}
	return recency.decay(when), true
}

func (recency *contextRecency) reason(candidate string) string {
	if recency.state != "examined" {
		return "recency abstained (" + recency.state + ")"
	}
	when, ok := recency.lastTouch[candidate]
	if !ok {
		return fmt.Sprintf("recency abstained (no commit in the %d-commit window)", recency.commits)
	}
	return fmt.Sprintf("recency %.2f (last commit %d days before the indexed commit, %d-day half-life)",
		recency.decay(when), recency.ageDays(when), contextRecencyHalfLifeDays)
}

// recencyLexical reorders the lexical fill by BM25 x (1 + 0.25 recency +
// 0.25 blame), code rows among the code positions and documentation rows among
// the documentation positions, so TCP-V0-013's placement is kept. The whole
// fill is reordered before take applies the limit, so which rows the slot
// admits can change. Blame runs on the first contextBlameCap candidates in
// BM25 order that take could still admit; every other candidate's blame
// feature abstains. Each row's reason names both features.
func (compiler *taskContextCompiler) recencyLexical(rows []contextRow) []contextRow {
	if compiler.recency == nil {
		return rows
	}
	recency := compiler.recency.await()
	bm25 := make(map[string]float64, len(compiler.lexical))
	for _, hit := range compiler.lexical {
		bm25[hit.path] = hit.score
	}
	recency.blameHead(compiler.unchosen(rows))
	factor := make(map[string]float64, len(rows))
	for index := range rows {
		candidate := rows[index].path
		recencyWeight, _ := recency.weight(candidate)
		blameWeight := recency.blame[candidate].freshness
		factor[candidate] = 1 + contextRecencyWeight*recencyWeight + contextBlameWeight*blameWeight
		rows[index].reason += fmt.Sprintf("; %s; %s; rank bm25 x %.2f",
			recency.reason(candidate), recency.blameReason(candidate), factor[candidate])
	}
	for _, kind := range []string{"lexical", "documentation"} {
		reorderKind(rows, kind, func(candidate string) float64 { return bm25[candidate] * factor[candidate] })
	}
	return rows
}

// unchosen is the rows take could still admit: not the subject and not a
// path an earlier slot chose, whose lexical copy take drops as a duplicate.
func (compiler *taskContextCompiler) unchosen(rows []contextRow) []contextRow {
	open := make([]contextRow, 0, len(rows))
	for _, row := range rows {
		if row.path == compiler.subject {
			continue
		}
		if _, seen := compiler.chosen[row.path]; seen {
			continue
		}
		open = append(open, row)
	}
	return open
}

// reorderKind stable-sorts the rows of one kind by key, descending, inside
// the positions that kind already holds.
func reorderKind(rows []contextRow, kind string, key func(string) float64) {
	positions := make([]int, 0, len(rows))
	group := make([]contextRow, 0, len(rows))
	for index, row := range rows {
		if row.kind == kind {
			positions = append(positions, index)
			group = append(group, row)
		}
	}
	sort.SliceStable(group, func(left, right int) bool { return key(group[left].path) > key(group[right].path) })
	for slot, index := range positions {
		rows[index] = group[slot]
	}
}

// recencyCochange orders the `cochange` slot by its recency-weighted count:
// each counted co-change commit weighs its 90-day half-life decay, over the
// same commits and the same size cap cochangeRows counted.
func (compiler *taskContextCompiler) recencyCochange(rows []contextRow) []contextRow {
	if compiler.recency == nil || len(rows) == 0 {
		return rows
	}
	recency := compiler.recency.await()
	if recency.state != "examined" {
		for index := range rows {
			rows[index].reason += "; " + recency.reason(rows[index].path)
		}
		return rows
	}
	decayed := map[string]float64{}
	commitCap := cochangeCommitCap(len(compiler.history))
	for _, entry := range compiler.history {
		when, known := recency.commitTime[entry.commit]
		if !known || len(entry.paths) > commitCap || !slices.Contains(entry.paths, compiler.subject) {
			continue
		}
		for _, candidate := range entry.paths {
			decayed[candidate] += recency.decay(when)
		}
	}
	for index := range rows {
		rows[index].reason += fmt.Sprintf("; recency-weighted %.2f (%d-day half-life)", decayed[rows[index].path], contextRecencyHalfLifeDays)
	}
	sort.SliceStable(rows, func(left, right int) bool { return decayed[rows[left].path] > decayed[rows[right].path] })
	return rows
}

// recencyCoverage is TCP-V0-038's `coverage.recency` member, present only
// when the flag is on: what the features read and every CODEOWNERS/blame
// disagreement among the packet's rows.
func (compiler *taskContextCompiler) recencyCoverage(coverage map[string]any, rows []contextRow) {
	if compiler.recency == nil {
		return
	}
	recency := compiler.recency.await()
	var ownersPath any
	if recency.owners != nil && recency.owners.path != "" {
		ownersPath = recency.owners.path
	}
	coverage["recency"] = map[string]any{
		"state": recency.state, "half_life_days": contextRecencyHalfLifeDays,
		"window_commits": recency.commits, "window_full": recency.windowFull,
		"blame_cap": contextBlameCap, "blamed": recency.blamed,
		"codeowners": ownersPath, "ownership": recency.ownership(rows),
	}
}
