package playwrightruns

import (
	"encoding/json"
	"sort"
)

// Shard-set vocabulary (PWR-V0-016 to PWR-V0-020).
const (
	ShardSetAdmitted = "ADMITTED"
	ShardSetRefused  = "REFUSED"

	ReasonShardSetIncomplete    = "SHARD_SET_INCOMPLETE"
	ReasonShardDuplicate        = "SHARD_DUPLICATE"
	ReasonShardInputsUndeclared = "SHARD_INPUTS_NOT_DECLARED"
)

// ShardSet is the shard reports of one logical run: the reports that declare one config.shard
// total at one Playwright version, applicationRevision, configDigest and repeatEach declaration.
type ShardSet struct {
	Run       int    `json:"run"`     // the logical run: the 1-based position of its first report
	Total     int    `json:"total"`   // the declared shard count
	Reports   []int  `json:"reports"` // 1-based report positions, ascending
	Shards    []int  `json:"shards"`  // the shard index each of those reports declares
	Status    string `json:"status"`  // ADMITTED | REFUSED
	Reason    string `json:"reason"`  // "" when admitted
	Missing   []int  `json:"missing"`
	Duplicate []int  `json:"duplicate"`
}

// logicalRun is one run as stability and compare count it: an unsharded report, or the merged
// shard reports of one shard set. refusal is the set's refusal reason, "" when admitted.
type logicalRun struct {
	*Run
	index   int
	refusal string
}

// logicalRuns groups reports into logical runs (PWR-V0-016). positions are the reports' 1-based
// argument positions. A report without config.shard is its own logical run; shard reports group by
// shard total and identity inputs, and a group is admitted only when shards 1..total are each
// present once at declared inputs. Logical runs keep the order of their first report.
func logicalRuns(runs []*Run, positions []int) ([]logicalRun, []ShardSet) {
	type group struct {
		set     *ShardSet
		members []*Run
	}
	groups := map[string]*group{}
	order := []*group{}
	slot := make([]*group, len(runs))
	for i, run := range runs {
		if run.Shard == nil {
			continue
		}
		key := shardGroupKey(run)
		g := groups[key]
		if g == nil {
			g = &group{set: &ShardSet{Run: positions[i], Total: run.Shard.Total, Reports: []int{}, Shards: []int{}, Missing: []int{}, Duplicate: []int{}}}
			groups[key] = g
			order = append(order, g)
			slot[i] = g
		}
		g.set.Reports = append(g.set.Reports, positions[i])
		g.set.Shards = append(g.set.Shards, run.Shard.Current)
		g.members = append(g.members, run)
	}
	merged := map[*group]logicalRun{}
	sets := []ShardSet{}
	for _, g := range order {
		set := g.set
		count := map[int]int{}
		for _, n := range set.Shards {
			count[n]++
		}
		for n := 1; n <= set.Total; n++ {
			switch {
			case count[n] == 0:
				set.Missing = append(set.Missing, n)
			case count[n] > 1:
				set.Duplicate = append(set.Duplicate, n)
			}
		}
		set.Status = ShardSetAdmitted
		switch {
		case !inputsComplete(g.members[0]):
			set.Reason = ReasonShardInputsUndeclared
		case len(set.Missing) > 0:
			set.Reason = ReasonShardSetIncomplete
		case len(set.Duplicate) > 0:
			set.Reason = ReasonShardDuplicate
		}
		if set.Reason != "" {
			set.Status = ShardSetRefused
		}
		merged[g] = logicalRun{Run: mergeShards(g.members), index: set.Run, refusal: set.Reason}
		sets = append(sets, *set)
	}
	out := []logicalRun{}
	for i, run := range runs {
		switch {
		case run.Shard == nil:
			out = append(out, logicalRun{Run: run, index: positions[i]})
		case slot[i] != nil:
			out = append(out, merged[slot[i]])
		}
	}
	return out, sets
}

func inputsComplete(run *Run) bool {
	return run.InputsDeclared && run.Inputs.ApplicationRevision != "" && run.Inputs.ConfigDigest != ""
}

// shardGroupKey is what the shards of one logical run must share. Undeclared inputs group as
// empty values, and such a group is refused rather than trusted.
func shardGroupKey(run *Run) string {
	app, config := "", ""
	if run.InputsDeclared {
		app, config = run.Inputs.ApplicationRevision, run.Inputs.ConfigDigest
	}
	raw, _ := json.Marshal([]any{run.Version, app, config, run.Shard.Total, inputsComplete(run), run.RepeatEach})
	return string(raw)
}

// mergeShards is one run from the shard reports of one set, read in shard order: tests are
// concatenated, projects and declared test-file blobs are united (a file two shards declare with
// different blobs keeps both as a conflict, PWR-V0-002), and repeats are resolved over the whole
// set, so a test in two shards is ambiguous unless repeatEach explains it (PWR-V0-018, PWR-V0-021).
func mergeShards(members []*Run) *Run {
	sorted := append([]*Run{}, members...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Shard.Current < sorted[j].Shard.Current })
	first := sorted[0]
	out := &Run{Version: first.Version, Inputs: Inputs{ApplicationRevision: first.Inputs.ApplicationRevision,
		ConfigDigest: first.Inputs.ConfigDigest, TestFileBlobs: map[string]string{}},
		InputsDeclared: first.InputsDeclared, RepeatEach: first.RepeatEach, Shard: first.Shard, Tests: []Test{}}
	projects, configured := map[string]bool{}, map[string]bool{}
	blobs := map[string]map[string]bool{}
	note := func(file, blob string) {
		if blobs[file] == nil {
			blobs[file] = map[string]bool{}
		}
		blobs[file][blob] = true
	}
	for _, m := range sorted {
		out.Workers += m.Workers
		out.ReportErrors += m.ReportErrors
		out.Tests = append(out.Tests, m.Tests...)
		for _, p := range m.Projects {
			projects[p] = true
		}
		for _, p := range m.ConfiguredProjects {
			configured[p] = true
		}
		for file, blob := range m.Inputs.TestFileBlobs {
			note(file, blob)
		}
		for file, list := range m.Inputs.conflicts {
			for _, blob := range list {
				note(file, blob)
			}
		}
	}
	for file, set := range blobs {
		for blob := range set {
			if len(set) == 1 {
				out.Inputs.TestFileBlobs[file] = blob
				continue
			}
			if out.Inputs.conflicts == nil {
				out.Inputs.conflicts = map[string][]string{}
			}
			out.Inputs.conflicts[file] = append(out.Inputs.conflicts[file], blob)
		}
		sort.Strings(out.Inputs.conflicts[file])
	}
	// As Parse leaves them: test projects nil when there are none, configured projects never nil.
	out.Projects, out.ConfiguredProjects = sortedKeys(projects), append([]string{}, sortedKeys(configured)...)
	out.resolveRepeats()
	return out
}

func sortedKeys(set map[string]bool) []string {
	var out []string
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
