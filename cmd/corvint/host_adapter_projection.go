package main

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

// hookContextProfile names the model-visible projection the Claude Code and Codex adapters inject
// in place of the full corvint-dogfood-event/0 receipt (AHI-045, V1-0942). The engine receipt, its
// digests and its wire profile are unchanged; only the host-facing text is projected.
const hookContextProfile = "corvint-hook-context/0"

// hookBaselineDegradations are the per-installation receipt codes every event of these adapters
// carries. A main-thread SessionStart names them once; a later event repeats only other codes
// (AHI-046).
var hookBaselineDegradations = map[string]bool{"frontier-authority-unavailable": true, "host-version-unknown": true}

// hookContextProjection is the actionable part of a dogfood-event receipt: task evidence with its
// blob pins, governance, non-current declared scope, unresolved anchors, omitted or unavailable
// selectors, new degradations, a non-inactive policy, a dirty-worktree freshness note and the
// compaction block. It drops the adapter, repository identity, request and result digests,
// coverage counters, the constant Frontier result and the completion decision.
//
// With full (a main-thread SessionStart) the projection is always returned. Otherwise it is nil
// unless the event carries evidence or named uncertainty the model can act on (AHI-046): task
// evidence, critical omissions, unavailable selectors, stale or unavailable declared scope, a
// non-baseline degradation, or a compaction block.
func hookContextProjection(event string, result map[string]any, full bool) map[string]any {
	packet, _ := result["context"].(map[string]any)
	coverage, _ := packet["coverage"].(map[string]any)
	projection := map[string]any{"profile": hookContextProfile, "event": event}
	actionable := false
	add := func(key string, value any, trigger bool) {
		projection[key] = value
		actionable = actionable || trigger
	}
	if rows := hookList(packet["task_evidence"]); len(rows) > 0 {
		add("task_evidence", rows, true)
	}
	scope := hookList(packet["declared_scope"])
	if !full {
		scope = hookNonCurrentScope(scope)
	}
	if len(scope) > 0 {
		add("declared_scope", scope, true)
	}
	if missing := hookList(coverage["critical_missing"]); len(missing) > 0 {
		add("omitted", missing, true)
	}
	if unavailable := hookList(coverage["unavailable_selectors"]); len(unavailable) > 0 {
		add("unavailable", unavailable, true)
	}
	if codes := hookDegradations(result["degradations"], full); len(codes) > 0 {
		add("degradations", codes, true)
	}
	if compaction, ok := packet["compaction"].(map[string]any); ok {
		add("compaction", hookCompaction(compaction), true)
	}
	if !full && !actionable {
		return nil
	}
	// Unresolved anchor counts qualify an emitted packet but never trigger one: with no evidence,
	// unavailable selector or omission to name, silence is the abstention (invariant 2).
	if resolution, _ := packet["resolution"].(map[string]any); hookUnresolved(resolution) {
		projection["resolution"] = resolution
	}
	if governance := hookList(packet["governance"]); len(governance) > 0 {
		projection["governance"] = governance
	}
	if freshness, _ := packet["freshness"].(string); freshness != "" && freshness != "clean" {
		projection["freshness"] = freshness
	}
	if taskState, _ := packet["task_state"].(map[string]any); taskState != nil {
		projection["task_state"] = map[string]any{"class": taskState["class"], "path_count": taskState["path_count"]}
	}
	if policy, _ := result["policy"].(map[string]any); policy != nil && policy["lifecycle"] != "inactive" {
		projection["policy"] = map[string]any{"lifecycle": policy["lifecycle"], "satisfied": policy["satisfied"], "unmet": policy["unmet"]}
	}
	return projection
}

// hookCompaction keeps what the model needs to resume after compaction: the snapshot revision,
// the rehydrated paths and their evidence, the rehydration counts, named omissions and the
// verification commands. Budgets, counters, digests and the schema version stay in the receipt.
func hookCompaction(block map[string]any) map[string]any {
	kept := map[string]any{}
	for _, key := range []string{"revision", "state", "mode", "results", "verification"} {
		if value, ok := block[key]; ok {
			kept[key] = value
		}
	}
	if request, _ := block["request"].(map[string]any); request != nil {
		kept["paths"] = request["paths"]
	}
	if rehydration, _ := block["rehydration"].(map[string]any); rehydration != nil {
		kept["rehydration"] = map[string]any{"state": rehydration["state"], "trackedDirtyPathCount": rehydration["trackedDirtyPathCount"], "untrackedDirtyPathCount": rehydration["untrackedDirtyPathCount"]}
	}
	coverage, _ := block["coverage"].(map[string]any)
	for _, key := range []string{"critical_missing", "uncertainty"} {
		if rows := hookList(coverage[key]); len(rows) > 0 {
			kept[key] = rows
		}
	}
	if freshness, _ := block["freshness"].(map[string]any); freshness != nil && freshness["state"] != "clean" {
		kept["freshness"] = freshness["state"]
	}
	return kept
}

func hookList(value any) []any {
	list, _ := value.([]any)
	return list
}

// hookNonCurrentScope keeps the declared-scope rows whose state is not current.
func hookNonCurrentScope(rows []any) []any {
	kept := []any{}
	for _, row := range rows {
		if entry, _ := row.(map[string]any); entry["state"] != "current" {
			kept = append(kept, row)
		}
	}
	return kept
}

// hookUnresolved reports a task that named anchors the packet did not fully resolve. A prompt with
// no explicit anchor has nothing to resolve and stays silent.
func hookUnresolved(resolution map[string]any) bool {
	anchors, _ := resolution["anchors"].(float64)
	return anchors > 0 && resolution["state"] != "resolved"
}

func hookDegradations(value any, full bool) []any {
	codes := []any{}
	for _, item := range hookList(value) {
		if code, _ := item.(string); full || !hookBaselineDegradations[code] {
			codes = append(codes, item)
		}
	}
	return codes
}

// claudeSubagent reports a Claude Code hook fired inside a subagent. Claude Code 2.1.267 documents
// agent_id as present only then and absent on the main thread, even in --agent sessions.
func claudeSubagent(payload map[string]any) bool {
	id, _ := payload["agent_id"].(string)
	return strings.TrimSpace(id) != ""
}

// claudeSessionGuidance is the workflow argv for a main-thread SessionStart, including the compact
// restart, and "" for every other event (AHI-047, V1-0939) and for a repository that is neither
// Corvint nor enrolled in dogfood (AHI-230, V1-2754).
func claudeSessionGuidance(root, key, event string, subagent bool) string {
	if event != "session-start" || subagent || !dogfoodWorkflowApplies(root) {
		return ""
	}
	return claudeGuidance(root, key)
}

// withHookContextSuffix appends the experimental kernel block, and gives it its own context when
// the adapter output is silent so the operator's opt-in still reaches the model.
func withHookContextSuffix(output map[string]any, eventName, suffix string) map[string]any {
	if len(output) == 0 && suffix != "" {
		return claudeContextOutput(eventName, strings.TrimPrefix(suffix, "\n"))
	}
	return withAdapterContextSuffix(output, suffix)
}

// hostLoadedGovernance names the repository-relative instruction files the host itself loads into
// the model context at the root (AHI-216, V1-2762): Claude Code loads CLAUDE.md and .claude/CLAUDE.md
// and follows their @path imports up to five hops. A file the host cannot be shown to load stays in
// the projection; Codex rows are unchanged.
func hostLoadedGovernance(host, root string) map[string]bool {
	if host != "claude-code" {
		return nil
	}
	loaded := map[string]bool{}
	frontier := []string{}
	for _, entry := range []string{"CLAUDE.md", ".claude/CLAUDE.md"} {
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(entry))); err == nil && info.Mode().IsRegular() {
			loaded[entry] = true
			frontier = append(frontier, entry)
		}
	}
	for hop := 0; hop < 5 && len(frontier) > 0 && len(loaded) < 64; hop++ {
		next := []string{}
		for _, file := range frontier {
			for _, imported := range claudeMemoryImports(root, file) {
				if !loaded[imported] && len(loaded) < 64 {
					loaded[imported] = true
					next = append(next, imported)
				}
			}
		}
		frontier = next
	}
	return loaded
}

// claudeMemoryImports reads the @path imports of one root-relative memory file: tokens outside
// code fences and code spans, resolved against the file's directory. Home, absolute and
// out-of-root paths, and files that do not exist, are not imports the projection can rely on.
func claudeMemoryImports(root, file string) []string {
	name := filepath.Join(root, filepath.FromSlash(file))
	if info, err := os.Stat(name); err != nil || !info.Mode().IsRegular() || info.Size() > 256<<10 {
		return nil
	}
	raw, err := os.ReadFile(name)
	if err != nil || len(raw) > 256<<10 {
		return nil
	}
	imports := []string{}
	fenced := false
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		for i, part := range strings.Split(line, "`") {
			if i%2 == 1 {
				continue
			}
			for _, word := range strings.Fields(part) {
				target, ok := strings.CutPrefix(word, "@")
				if !ok || target == "" || strings.HasPrefix(target, "~") || strings.HasPrefix(target, "/") {
					continue
				}
				clean := path.Clean(path.Join(path.Dir(file), target))
				if clean == ".." || strings.HasPrefix(clean, "../") {
					continue
				}
				if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(clean))); err == nil && info.Mode().IsRegular() {
					imports = append(imports, clean)
				}
			}
		}
	}
	return imports
}

// withoutHostLoadedGovernance drops the governance rows whose file the host already loads. The
// receipt keeps every row with its blob pin; only the model-facing repeat is removed. It edits and
// returns projection.
func withoutHostLoadedGovernance(projection map[string]any, loaded map[string]bool) map[string]any {
	rows := hookList(projection["governance"])
	if len(rows) == 0 || len(loaded) == 0 {
		return projection
	}
	kept := []any{}
	for _, row := range rows {
		entry, _ := row.(map[string]any)
		if file, _ := entry["path"].(string); !loaded[file] {
			kept = append(kept, row)
		}
	}
	if len(kept) == 0 {
		delete(projection, "governance")
		return projection
	}
	projection["governance"] = kept
	return projection
}
