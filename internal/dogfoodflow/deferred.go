package dogfoodflow

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// deferredRowKind is the first field of a citation-plan row that records an
// intent-scope deferral instead of citing a hunk:
// deferred<TAB>PATH<TAB>REASON (DCW-V0-370). A hunk selector is an ordinal or
// a hunk ID, so the word never names a hunk.
const deferredRowKind = "deferred"

// deferStep is the report row of the plan's deferral rows (DCW-V0-370).
const deferStep = "cem-defer"

// deferral is one accepted deferred plan row.
type deferral struct{ path, reason string }

// splitDeferredRows returns plan without its deferred rows, and those rows in
// plan order. A deferred row that is not exactly three fields, or a set of
// rows the CEM-V1-643 record rules refuse (more than MaxDeferredIntents, a
// repeated or non-canonical path, a reason that is not one line of 1..200
// bytes), makes the whole plan invalid (DCW-V0-370).
func splitDeferredRows(plan []byte) ([]byte, []deferral, bool) {
	var kept []string
	var rows []deferral
	for _, line := range textLines(plan) {
		fields := strings.Split(line, "\t")
		if fields[0] != deferredRowKind {
			kept = append(kept, line)
			continue
		}
		if len(fields) != 3 {
			return nil, nil, false
		}
		rows = append(rows, deferral{fields[1], fields[2]})
	}
	if len(rows) == 0 {
		return plan, nil, true
	}
	entries := make([]workflow.DeferredIntent, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, workflow.DeferredIntent{ObligationCount: 1, Path: row.path, Reason: row.reason})
	}
	slices.SortFunc(entries, func(a, b workflow.DeferredIntent) int { return strings.Compare(a.Path, b.Path) })
	if workflow.ValidateDeferredIntents(entries) != nil {
		return nil, nil, false
	}
	if len(kept) == 0 {
		return []byte{}, rows, true
	}
	return []byte(strings.Join(kept, "\n") + "\n"), rows, true
}

// deferIntents runs `cem defer` for each deferred plan row on the prepared
// map, before anything is cited, so the record the change commits beside the
// sidecar is written before the bind commit (DCW-V0-370). A deferred path the
// intent manifest also keeps is refused before any record is written: one
// scope is either assessed or recorded as deferred, never both.
func (c *change) deferIntents() {
	if c.options.Citations == "" || !isRegular(c.options.Citations) {
		return
	}
	if _, valid := c.validateCitationPlan(); !valid || len(c.deferrals) == 0 {
		return
	}
	if !c.cemPrepared || !isRegular(c.path(".corvint/change.cem.json")) {
		c.addStep(deferStep, "NOT_PRODUCED", "cem-map-not-produced")
		return
	}
	var kept []string
	for _, row := range c.deferrals {
		if slices.Contains(c.intents, row.path) {
			kept = append(kept, row.path)
		}
	}
	if len(kept) > 0 {
		c.addStep(deferStep, "NOT_PRODUCED", "deferred-intent-kept")
		c.say("%s: FAIL %s deferred-intent-kept paths=%s\n  fix: remove each path from DOGFOOD_INTENTS_FILE to defer it, or drop its deferred plan row to keep it assessed (DCW-V0-370)\n",
			c.prefix, deferStep, strings.Join(kept, ","))
		return
	}
	for index, row := range c.deferrals {
		name := fmt.Sprintf("%s-%03d", deferStep, index+1)
		if c.runStep(name, c.evidence+"/"+name+".json", "cem", "defer", "--map", ".corvint/change.cem.json",
			"--intent", row.path, "--reason", row.reason) != 0 {
			return
		}
	}
}

// recordedDeferrals counts the entries of the deferral records target adds
// over base under workflow.DeferredIntentsDir, the bound a check of that range
// passes as --max-deferred-intents (DCW-V0-371). A record that cannot be read
// or parsed counts nothing, so `cem status` refuses it against a zero bound
// instead of the check guessing; paths are the recorded intent paths.
func (f *flow) recordedDeferrals(base, target string) (count int, paths []string) {
	added, status, bounded := f.boundedGit("diff", "--no-renames", "--name-only", "--diff-filter=A", base, target, "--", workflow.DeferredIntentsDir+"/")
	if status != 0 || !bounded {
		return 0, nil
	}
	for _, record := range nonEmpty(strings.Split(added, "\n")) {
		if filepath.Ext(record) != ".json" {
			continue
		}
		data, status, bounded := f.boundedGit("cat-file", "blob", target+":"+record)
		if status != 0 || !bounded || len(data) > workflow.MaxDeferredIntentsRecordBytes {
			continue
		}
		entries, err := workflow.ParseDeferredIntents([]byte(data))
		if err != nil {
			continue
		}
		count += len(entries)
		for _, entry := range entries {
			paths = append(paths, entry.Path)
		}
	}
	return count, paths
}

// statusArguments is the bind loop's strict `cem status` argv. A change that
// records deferrals adds --max-deferred-intents N before the artifact root;
// with none the argv is unchanged, so a verifier that predates the flag reads
// every other change exactly as before (DCW-V0-371).
func statusArguments(root, base, target string, maxUnknown, deferrals int) []string {
	args := []string{"cem", "status", "--map", ".corvint/change.cem.json", "--expected-base", base, "--target", target,
		"--max-unknown", strconv.Itoa(maxUnknown), "--max-mechanical", "0"}
	if deferrals > 0 {
		args = append(args, "--max-deferred-intents", strconv.Itoa(deferrals))
	}
	return stableStatusArguments(root, args...)
}

// deferralRecordHunks is the set of the map's hunk IDs on a deferral record
// path (DCW-V0-372).
func deferralRecordHunks(hunks []map[string]string) map[string]bool {
	records := map[string]bool{}
	for _, hunk := range hunks {
		if strings.HasPrefix(hunk["path"], workflow.DeferredIntentsDir+"/") {
			records[hunk["id"]] = true
		}
	}
	return records
}
