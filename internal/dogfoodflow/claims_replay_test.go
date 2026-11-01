package dogfoodflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

// TestClaimsReplay is the CKC-V0-011 replay harness. It runs only when
// CKC_REPLAY_REPO names a clone of this repository and CKC_REPLAY_SEQUENCES
// a file of real fix sequences, one per line:
//
//	NAME MAPCOMMIT BASE0 COMMIT1:BASE1 COMMIT2:BASE2 ...
//
// MAPCOMMIT is the lane's first commit of a sealed .corvint/change.cem.json;
// its rows are the real reviewed plan. Each later COMMIT is a fix step at its
// BASE. Both flows replay the same steps: the current flow refreshes the
// author's last plan (--refresh PLAN), the claims flow refreshes claims
// recorded from that plan (--refresh CLAIMS) and records the next ones.
//
// An independent oracle gives each step's target plan: a hunk whose path
// and body equal exactly one earlier hunk's, both ways, keeps that hunk's
// reviewed rows, with each cite moved to the single line-aligned occurrence
// of its cited bytes when BASE moved; any other hunk takes the plain draft's
// row, with a cite the author deleted from the step-0 draft for that file
// replaced by an unknown row. Manual edits are the rows each flow's output
// differs from the target by, a replaced row counting once. An incorrect
// carry is an output row outside the target that equals an earlier reviewed
// row. For the claims flow it is instead any such row on a hunk the claims
// compile carried, since the refresh drafts every other hunk afresh, and a
// fresh draft can differ from the plain draft (a span already carried is
// not drafted again) and coincide with an earlier row. The current flow's
// count has no such filter, so it is an upper bound.
func TestClaimsReplay(t *testing.T) {
	source, sequences := os.Getenv("CKC_REPLAY_REPO"), os.Getenv("CKC_REPLAY_SEQUENCES")
	if source == "" || sequences == "" {
		t.Skip("set CKC_REPLAY_REPO and CKC_REPLAY_SEQUENCES to replay real fix sequences (CKC-V0-011)")
	}
	limit := 20
	if value, err := strconv.Atoi(os.Getenv("CKC_REPLAY_LIMIT")); err == nil && value > 0 {
		limit = value
	}
	data, err := os.ReadFile(sequences)
	if err != nil {
		t.Fatal(err)
	}
	var total replayTotals
	included := 0
	for _, line := range textLines(data) {
		if included == limit {
			break
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		result, excluded := replaySequence(t, source, fields)
		if excluded != "" {
			fmt.Printf("REPLAY excluded %s map=%s: %s\n", fields[0], fields[1][:12], excluded)
			continue
		}
		included++
		fmt.Printf("REPLAY sequence %s map=%s steps=%d rebased=%d hunks=%d current-edits=%d claims-edits=%d current-incorrect=%d claims-incorrect=%d current-carried=%d claims-carried=%d reassess=%d gone=%d suppressed=%d\n",
			fields[0], fields[1][:12], result.steps, result.rebased, result.hunks, result.currentEdits, result.claimsEdits,
			result.currentIncorrect, result.claimsIncorrect, result.currentCarried, result.claimsCarried, result.reassess, result.gone, result.suppressed)
		total.add(result)
	}
	fmt.Printf("REPLAY total sequences=%d steps=%d rebased=%d hunks=%d step0-edits=%d current-edits=%d claims-edits=%d current-incorrect=%d claims-incorrect=%d current-carried=%d claims-carried=%d reassess=%d gone=%d suppressed=%d\n",
		included, total.steps, total.rebased, total.hunks, total.step0Edits, total.currentEdits, total.claimsEdits,
		total.currentIncorrect, total.claimsIncorrect, total.currentCarried, total.claimsCarried, total.reassess, total.gone, total.suppressed)
	fmt.Printf("REPLAY time-ms current p50=%d p95=%d max=%d claims p50=%d p95=%d max=%d (refresh; claims adds its record) claims-refresh-only p50=%d p95=%d max=%d\n",
		percentile(total.currentTimes, 50), percentile(total.currentTimes, 95), percentile(total.currentTimes, 100),
		percentile(total.claimsTimes, 50), percentile(total.claimsTimes, 95), percentile(total.claimsTimes, 100),
		percentile(total.claimsRefreshTimes, 50), percentile(total.claimsRefreshTimes, 95), percentile(total.claimsRefreshTimes, 100))
	for _, detail := range total.details {
		fmt.Printf("REPLAY incorrect %s\n", detail)
	}
	if included < limit {
		t.Errorf("replayed %d sequences, want %d", included, limit)
	}
	if total.claimsIncorrect != 0 {
		t.Errorf("claims flow made %d incorrect carries", total.claimsIncorrect)
	}
}

type replayTotals struct {
	steps, rebased, hunks, step0Edits int
	currentEdits, claimsEdits         int
	currentIncorrect, claimsIncorrect int
	currentCarried, claimsCarried     int
	reassess, gone, suppressed        int
	currentTimes, claimsTimes         []time.Duration
	claimsRefreshTimes                []time.Duration
	details                           []string
}

func (r *replayTotals) add(other replayTotals) {
	r.steps += other.steps
	r.rebased += other.rebased
	r.hunks += other.hunks
	r.step0Edits += other.step0Edits
	r.currentEdits += other.currentEdits
	r.claimsEdits += other.claimsEdits
	r.currentIncorrect += other.currentIncorrect
	r.claimsIncorrect += other.claimsIncorrect
	r.currentCarried += other.currentCarried
	r.claimsCarried += other.claimsCarried
	r.reassess += other.reassess
	r.gone += other.gone
	r.suppressed += other.suppressed
	r.currentTimes = append(r.currentTimes, other.currentTimes...)
	r.claimsTimes = append(r.claimsTimes, other.claimsTimes...)
	r.claimsRefreshTimes = append(r.claimsRefreshTimes, other.claimsRefreshTimes...)
	r.details = append(r.details, other.details...)
}

// percentile is the nearest-rank percentile in milliseconds.
func percentile(values []time.Duration, rank int) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	index := (rank*len(sorted)+99)/100 - 1
	if index < 0 {
		index = 0
	}
	return sorted[index].Milliseconds()
}

// replayRow is one plan row of a hunk. A wildcard unknown row matches any
// unknown row of its reason, whatever its note.
type replayRow struct {
	path, lines, relation string // cite
	reason, note          string // unknown
	wildcard              bool
}

func (r replayRow) text() string {
	if r.reason != "" {
		return "unknown\t" + r.reason + "\t" + r.note
	}
	return r.path + "\t" + r.lines + "\t" + r.relation
}

func (r replayRow) matches(other replayRow) bool {
	if r.reason != "" || other.reason != "" {
		return r.reason == other.reason && (r.wildcard || other.wildcard || r.note == other.note)
	}
	return r.path == other.path && r.lines == other.lines && r.relation == other.relation
}

func parseReplayRow(fields []string) (replayRow, bool) {
	if len(fields) != 3 {
		return replayRow{}, false
	}
	if fields[0] == "unknown" {
		return replayRow{reason: fields[1], note: fields[2]}, true
	}
	return replayRow{path: fields[0], lines: fields[1], relation: fields[2]}, true
}

// replayRows parses a plan's hunk-ID-keyed rows.
func replayRows(plan string) map[string][]replayRow {
	rows := map[string][]replayRow{}
	for _, line := range textLines([]byte(plan)) {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 || !strings.HasPrefix(fields[0], "hunk:") {
			continue
		}
		if row, ok := parseReplayRow(fields[1:]); ok {
			rows[fields[0]] = append(rows[fields[0]], row)
		}
	}
	return rows
}

func renderReplayPlan(hunks []*patch.Hunk, rows map[string][]replayRow) string {
	var plan strings.Builder
	for _, hunk := range hunks {
		for _, row := range rows[hunk.ID] {
			plan.WriteString(hunk.ID + "\t" + row.text() + "\n")
		}
	}
	return plan.String()
}

type replayRepo struct {
	t     *testing.T
	root  string
	blobs map[string][]byte
}

func (r *replayRepo) checkout(commit string) {
	r.t.Helper()
	testGit(r.t, r.root, "checkout", "--quiet", "--detach", "--force", commit)
	testGit(r.t, r.root, "clean", "-q", "-f", "-d", "--", ".corvint")
	// A committed map is an earlier bind's, not this step's prepared map.
	_ = os.Remove(filepath.Join(r.root, ".corvint", "change.cem.json"))
}

func (r *replayRepo) blob(commit, name string) []byte {
	key := commit + ":" + name
	if data, ok := r.blobs[key]; ok {
		return data
	}
	var stdout bytes.Buffer
	command := exec.Command("git", "-C", r.root, "cat-file", "blob", key)
	command.Stdout = &stdout
	if command.Run() != nil {
		r.blobs[key] = nil
		return nil
	}
	r.blobs[key] = stdout.Bytes()
	return r.blobs[key]
}

// lineBytes returns lines START:END of data, or nil.
func lineBytes(data []byte, lines string) []byte {
	startText, endText, ok := strings.Cut(lines, ":")
	start, startErr := strconv.Atoi(startText)
	end, endErr := strconv.Atoi(endText)
	if !ok || startErr != nil || endErr != nil || start < 1 || end < start {
		return nil
	}
	offsets := lineOffsets(data)
	if end > len(offsets) {
		return nil
	}
	stop := len(data)
	if end < len(offsets) {
		stop = offsets[end]
	}
	return data[offsets[start-1]:stop]
}

func lineOffsets(data []byte) []int {
	offsets := []int{}
	for index := 0; index < len(data); {
		offsets = append(offsets, index)
		next := bytes.IndexByte(data[index:], '\n')
		if next < 0 {
			break
		}
		index += next + 1
	}
	return offsets
}

// relocate returns the single line-aligned occurrence of text in data.
func relocate(data, text []byte) (string, bool) {
	count := bytes.Count(text, []byte("\n"))
	if len(text) > 0 && text[len(text)-1] != '\n' {
		count++
	}
	found := ""
	for index, offset := range lineOffsets(data) {
		if bytes.HasPrefix(data[offset:], text) && (offset+len(text) == len(data) || text[len(text)-1] == '\n') {
			if found != "" {
				return "", false
			}
			found = fmt.Sprintf("%d:%d", index+1, index+count)
		}
	}
	return found, found != ""
}

func (r *replayRepo) hunks(base string) []*patch.Hunk {
	return claimsHunks(r.t, r.root, base)
}

func (r *replayRepo) propose(base, refresh string) (string, string, time.Duration, bool) {
	var stdout, stderr bytes.Buffer
	started := time.Now()
	code, err := ProposeCitations(context.Background(), ProposeOptions{Root: r.root, Base: base, Refresh: refresh}, &stdout, &stderr)
	elapsed := time.Since(started)
	if err != nil || code != 0 {
		return "", fmt.Sprintf("propose-citations refresh=%q exit %d %v: %s", refresh, code, err, firstLines(stderr.String(), 6)), elapsed, false
	}
	return stdout.String(), stderr.String(), elapsed, true
}

func (r *replayRepo) record(base, plan, claims, draft string) (time.Duration, string) {
	var stdout, stderr bytes.Buffer
	options := RecordClaimsOptions{Root: r.root, Base: base, Plan: writeInput(r.t, plan), Claims: claims}
	if draft != "" {
		options.Draft = writeInput(r.t, draft)
	}
	started := time.Now()
	code, err := RecordClaims(context.Background(), options, &stdout, &stderr)
	elapsed := time.Since(started)
	if err != nil || code != 0 {
		return elapsed, fmt.Sprintf("record-claims exit %d %v: %s", code, err, firstLines(stderr.String(), 6))
	}
	return elapsed, ""
}

func firstLines(text string, count int) string {
	lines := strings.SplitN(text, "\n", count+1)
	if len(lines) > count {
		lines = lines[:count]
	}
	return strings.Join(lines, " | ")
}

// sealedPlan converts the sealed map at commit into hunk-ID-keyed rows.
func (r *replayRepo) sealedPlan(commit string, hunks []*patch.Hunk) (map[string][]replayRow, string) {
	data := r.blob(commit, ".corvint/change.cem.json")
	var sealed citationMap
	if err := json.Unmarshal(data, &sealed); err != nil {
		return nil, "sealed map unreadable: " + err.Error()
	}
	known := map[string]bool{}
	for _, hunk := range hunks {
		known[hunk.ID] = true
	}
	evidence := map[string]replayRow{}
	for _, item := range sealed.Evidence {
		blob := r.blob(sealed.BaseRevision, item.Path)
		start, end := int(item.Span.Start), int(item.Span.End)
		if blob == nil || start < 0 || end > len(blob) || end <= start || (start > 0 && blob[start-1] != '\n') || (end < len(blob) && blob[end-1] != '\n') {
			return nil, "sealed evidence " + item.Path + " is not a line-aligned span of its BASE blob"
		}
		first := bytes.Count(blob[:start], []byte("\n")) + 1
		last := bytes.Count(blob[:end], []byte("\n"))
		if blob[end-1] != '\n' {
			last++
		}
		evidence[item.ID] = replayRow{path: item.Path, lines: fmt.Sprintf("%d:%d", first, last)}
	}
	rows := map[string][]replayRow{}
	for _, hunk := range sealed.Hunks {
		if !known[hunk.ID] {
			return nil, "sealed hunk " + hunk.ID[:20] + " is not in the canonical patch"
		}
		switch hunk.Disposition {
		case "supported":
			for _, basis := range hunk.Basis {
				row, ok := evidence[basis.EvidenceID]
				if !ok {
					return nil, "sealed basis names no evidence"
				}
				row.relation = basis.Relation
				rows[hunk.ID] = append(rows[hunk.ID], row)
			}
		case "unknown":
			if hunk.Reason == "insufficient-evidence" || hunk.Reason == "conflicting-evidence" {
				rows[hunk.ID] = append(rows[hunk.ID], replayRow{reason: hunk.Reason, note: "replayed from the sealed map"})
			}
		}
	}
	if len(rows) == 0 {
		return nil, "sealed map has no plan rows"
	}
	return rows, ""
}

var claimsHeader = regexp.MustCompile(`carried=(\d+) reassess=(\d+) gone=(\d+) suppressed=(\d+)`)

func replaySequence(t *testing.T, source string, fields []string) (replayTotals, string) {
	t.Helper()
	var result replayTotals
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, filepath.Dir(root), "clone", "--quiet", "--shared", "--no-checkout", source, root)
	repo := &replayRepo{t: t, root: root, blobs: map[string][]byte{}}
	mapCommit, base := fields[1], fields[2]

	// Step 0: the sealed map's rows on its own target, the map commit's
	// parent or the map commit itself.
	var hunks []*patch.Hunk
	var target map[string][]replayRow
	reason := ""
	for _, commit := range []string{mapCommit + "^", mapCommit} {
		repo.checkout(commit)
		hunks = repo.hunks(base)
		if target, reason = repo.sealedPlan(mapCommit, hunks); reason == "" {
			break
		}
	}
	if reason != "" {
		return result, reason
	}
	draft, why, _, ok := repo.propose(base, "")
	if !ok {
		return result, why
	}
	plan := renderReplayPlan(hunks, target)
	claims := filepath.Join(t.TempDir(), "claims.json")
	recordTime, why := repo.record(base, plan, claims, draft)
	if why != "" {
		return result, "step 0 " + why
	}
	// The author's step-0 deletions: drafted cites the reviewed plan lacks,
	// by file, as path, relation and cited bytes.
	deleted := map[string]map[string]bool{}
	drafted := replayRows(draft)
	for _, hunk := range hunks {
		for _, row := range drafted[hunk.ID] {
			if row.reason != "" || containsRow(target[hunk.ID], row) {
				continue
			}
			if deleted[hunk.DisplayPath] == nil {
				deleted[hunk.DisplayPath] = map[string]bool{}
			}
			deleted[hunk.DisplayPath][repo.citeKey(base, row)] = true
		}
		result.step0Edits += rowDifference(drafted[hunk.ID], target[hunk.ID])
	}

	debug := os.Getenv("CKC_REPLAY_DEBUG") != ""
	previousHunks, previousBase, previousTarget := hunks, base, target
	for _, step := range fields[3:] {
		commit, stepBase, _ := strings.Cut(step, ":")
		repo.checkout(commit)
		hunks = repo.hunks(stepBase)
		plain, why, _, ok := repo.propose(stepBase, "")
		if !ok {
			return result, "step " + commit[:12] + " " + why
		}
		current, why, currentTime, ok := repo.propose(stepBase, writeInput(t, plan))
		if !ok {
			return result, "step " + commit[:12] + " current " + why
		}
		carriedByClaims := repo.carriedHunks(claims, stepBase, hunks)
		compiled, header, claimsTime, ok := repo.propose(stepBase, claims)
		if !ok {
			return result, "step " + commit[:12] + " claims " + header
		}
		if match := claimsHeader.FindStringSubmatch(header); match != nil {
			values := [4]int{}
			for index := range values {
				values[index], _ = strconv.Atoi(match[index+1])
			}
			result.reassess += values[1]
			result.gone += values[2]
			result.suppressed += values[3]
		}
		target = repo.oracle(previousHunks, previousBase, previousTarget, hunks, stepBase, replayRows(plain), deleted)
		earlier := map[string]bool{}
		for _, hunk := range previousHunks {
			for _, row := range previousTarget[hunk.ID] {
				earlier[repo.carryKey(previousBase, row)] = true
			}
		}
		currentRows, compiledRows := replayRows(current), replayRows(compiled)
		for _, hunk := range hunks {
			currentEdits := rowDifference(currentRows[hunk.ID], target[hunk.ID])
			claimsEdits := rowDifference(compiledRows[hunk.ID], target[hunk.ID])
			result.currentEdits += currentEdits
			result.claimsEdits += claimsEdits
			if debug && currentEdits+claimsEdits > 0 {
				fmt.Printf("REPLAY edits %s step=%s %s %s BASE %d,%d current=%d claims=%d\n  target=%q\n  plain=%q\n  current=%q\n  claims=%q\n", fields[0], commit[:12], hunk.ID[:20], hunk.DisplayPath, hunk.OldRange.Start, hunk.OldRange.Count, currentEdits, claimsEdits, rowTexts(target[hunk.ID]), rowTexts(replayRows(plain)[hunk.ID]), rowTexts(currentRows[hunk.ID]), rowTexts(compiledRows[hunk.ID]))
				for _, line := range strings.Split(header, "\n") {
					if strings.Contains(line, hunk.ID) {
						fmt.Printf("  stderr=%q\n", line)
					}
				}
			}
			for flow, rows := range map[string][]replayRow{"current": currentRows[hunk.ID], "claims": compiledRows[hunk.ID]} {
				for _, row := range rows {
					switch {
					case containsRow(target[hunk.ID], row):
						if earlier[repo.carryKey(stepBase, row)] && !containsRow(replayRows(plain)[hunk.ID], row) {
							if flow == "current" {
								result.currentCarried++
							} else {
								result.claimsCarried++
							}
						}
					case flow == "claims" && !carriedByClaims[hunk.ID]:
						// A refused or unclaimed hunk is drafted afresh; a
						// drafted row outside the target is an edit, not a carry.
					case flow == "claims" || earlier[repo.carryKey(stepBase, row)]:
						if flow == "current" {
							result.currentIncorrect++
						} else {
							result.claimsIncorrect++
						}
						result.details = append(result.details, fmt.Sprintf("%s %s step=%s hunk=%s %s BASE %d,%d row=%q", flow, fields[0], commit[:12], hunk.ID[:20], hunk.DisplayPath, hunk.OldRange.Start, hunk.OldRange.Count, row.text()))
					}
				}
			}
		}
		result.steps++
		result.hunks += len(hunks)
		if stepBase != previousBase {
			result.rebased++
		}
		result.currentTimes = append(result.currentTimes, currentTime)
		result.claimsTimes = append(result.claimsTimes, claimsTime+recordTime)
		result.claimsRefreshTimes = append(result.claimsRefreshTimes, claimsTime)

		// The author fixes each flow's output to the target; the claims
		// flow records it, with the output it saw as the draft.
		plan = renderReplayPlan(hunks, concrete(target, hunks, replayRows(plain), compiledRows))
		if recordTime, why = repo.record(stepBase, plan, claims, compiled); why != "" {
			return result, "step " + commit[:12] + " " + why
		}
		previousHunks, previousBase, previousTarget = hunks, stepBase, replayRows(plan)
	}
	return result, ""
}

// carriedHunks compiles the claims file as the claims refresh does and
// returns the hunks it carries rows onto, so a row the refresh drafted
// afresh is never counted as a claims carry.
func (r *replayRepo) carriedHunks(claimsPath, base string, hunks []*patch.Hunk) map[string]bool {
	data, err := os.ReadFile(claimsPath)
	if err != nil {
		r.t.Fatal(err)
	}
	file, why := decodeClaims(data)
	if file == nil {
		r.t.Fatalf("claims file %s: %s", claimsPath, why)
	}
	compiler := &claimsCompiler{recordedBase: file.Base, currentBase: base, records: map[string][][]byte{}, blob: func(commit, name string) ([]byte, bool) {
		data := r.blob(commit, name)
		return data, data != nil
	}}
	carried := map[string]bool{}
	for _, result := range compiler.compile(file.Claims, hunks) {
		if result.class == claimCarried {
			carried[result.hunks[0].ID] = true
		}
	}
	return carried
}

// concrete resolves wildcard target rows to the row a flow printed or the
// plain draft's, so the recorded plan is a real one.
func concrete(target map[string][]replayRow, hunks []*patch.Hunk, plain, compiled map[string][]replayRow) map[string][]replayRow {
	out := map[string][]replayRow{}
	for _, hunk := range hunks {
		for _, row := range target[hunk.ID] {
			if row.wildcard {
				row = pickUnknown(row, compiled[hunk.ID], plain[hunk.ID])
			}
			out[hunk.ID] = append(out[hunk.ID], row)
		}
	}
	return out
}

func pickUnknown(want replayRow, sources ...[]replayRow) replayRow {
	for _, rows := range sources {
		for _, row := range rows {
			if row.reason == want.reason {
				return row
			}
		}
	}
	want.wildcard, want.note = false, "replay target"
	return want
}

func rowTexts(rows []replayRow) []string {
	texts := []string{}
	for _, row := range rows {
		texts = append(texts, row.text())
	}
	return texts
}

func containsRow(rows []replayRow, row replayRow) bool {
	for _, other := range rows {
		if other.matches(row) {
			return true
		}
	}
	return false
}

// rowDifference counts the row edits that turn got into want: a replaced
// row is one edit, as is an added or a removed one.
func rowDifference(got, want []replayRow) int {
	extra, missing := 0, 0
	for _, row := range got {
		if !containsRow(want, row) {
			extra++
		}
	}
	for _, row := range want {
		if !containsRow(got, row) {
			missing++
		}
	}
	return max(extra, missing)
}

// citeKey is a cite's path, relation and cited bytes at base.
func (r *replayRepo) citeKey(base string, row replayRow) string {
	sum := sha256.Sum256(lineBytes(r.blob(base, row.path), row.lines))
	return row.path + "\t" + row.relation + "\t" + hex.EncodeToString(sum[:])
}

// carryKey identifies an earlier reviewed row wherever BASE now numbers it.
func (r *replayRepo) carryKey(base string, row replayRow) string {
	if row.reason != "" {
		return "unknown\t" + row.reason + "\t" + row.note
	}
	return r.citeKey(base, row)
}

func (r *replayRepo) oracle(previous []*patch.Hunk, previousBase string, previousTarget map[string][]replayRow, hunks []*patch.Hunk, base string, plain map[string][]replayRow, deleted map[string]map[string]bool) map[string][]replayRow {
	same := func(a, b *patch.Hunk) bool {
		return a.DisplayPath == b.DisplayPath && bytes.Equal(a.RawBody, b.RawBody)
	}
	target := map[string][]replayRow{}
	for _, hunk := range hunks {
		var predecessor *patch.Hunk
		count := 0
		for _, old := range previous {
			if same(old, hunk) {
				predecessor = old
				count++
			}
		}
		twins := 0
		for _, other := range hunks {
			if same(other, hunk) {
				twins++
			}
		}
		if count == 1 && twins == 1 && len(previousTarget[predecessor.ID]) > 0 {
			rows, ok := []replayRow{}, true
			for _, row := range previousTarget[predecessor.ID] {
				if row.reason == "" && base != previousBase {
					lines, found := relocate(r.blob(base, row.path), lineBytes(r.blob(previousBase, row.path), row.lines))
					if !found {
						ok = false
						break
					}
					row.lines = lines
				}
				rows = append(rows, row)
			}
			if ok {
				target[hunk.ID] = rows
				continue
			}
		}
		for _, row := range plain[hunk.ID] {
			if row.reason != "" {
				row.wildcard = true
			} else if deleted[hunk.DisplayPath][r.citeKey(base, row)] {
				row = replayRow{reason: "insufficient-evidence", wildcard: true}
			}
			target[hunk.ID] = append(target[hunk.ID], row)
		}
	}
	return target
}
