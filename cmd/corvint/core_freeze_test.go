package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// frozenCoreVerbs is the CCF-V1-001 Core boundary from decision 0332, in the order root help lists it.
var frozenCoreVerbs = []string{"init", "adopt", "index", "query", "context", "impact", "affected", "prove", "cem", "ocm", "frontier", "dogfood"}

// hookPlumbingVerbs are the CCF-V1-003 adapter plumbing verbs runContext dispatches before the
// topLevelCommands check; they are undocumented, absent from root help and outside the freeze.
var hookPlumbingVerbs = []string{"native-hook", "authority-event", "qualified-event"}

// absentMember marks a wanted member that the frozen document must not carry.
type absentMember struct{}

// coreFreezeParent returns the full id of HEAD's first parent, the committed range base the
// --base modes need.
func coreFreezeParent(t *testing.T, root string) string {
	t.Helper()
	return strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD~1"))
}

// jsonMember reads one dotted member path from a decoded JSON object.
func jsonMember(document map[string]any, path string) (any, bool) {
	var value any = document
	for _, name := range strings.Split(path, ".") {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		if value, ok = object[name]; !ok {
			return nil, false
		}
	}
	return value, true
}

// coreEvidenceArguments builds a committed cem/0.2 map and bound OCM, and returns the read
// arguments of one CEM, OCM or frontier mode against them.
func coreEvidenceArguments(t *testing.T, verb string, extra ...string) []string {
	t.Helper()
	fixture := newOCMReadFixture(t)
	maps := map[string][]string{
		"cem":      {"--map", fixture.mapPath},
		"ocm":      {"--map", "change.ocm.json", "--cem", fixture.mapPath},
		"frontier": {"--cem", fixture.mapPath, "--ocm", "change.ocm.json", "--json"},
	}
	arguments := append([]string{"--root", fixture.root, verb}, extra...)
	arguments = append(arguments, maps[verb]...)
	return append(arguments, "--expected-base", fixture.base, "--target", fixture.target)
}

// TestCoreVerbsEmitTheFrozenProfiles pins CCF-V1-002: every frozen Core mode exits with its
// frozen code and emits one JSON document carrying the exact envelope and profile identifiers the
// freeze lists. Unless a case says otherwise the envelope is ok=true, mutates=false and exit 0. A
// change to any value here is a breaking change under CCF-V1-006 and needs a new profile version,
// an N-1 reader and a decision.
func TestCoreVerbsEmitTheFrozenProfiles(t *testing.T) {
	t.Parallel()
	genesis := "genesis-inventory/0.1-experimental"
	genesisSummary := "genesis-inventory-summary/0.1-experimental"
	task := "where is StableValue"
	cases := []struct {
		name   string
		invoke func(*testing.T) []string
		exit   int
		want   map[string]any
	}{
		{"init summary", func(t *testing.T) []string {
			return activationArguments(newActivationFixture(t, "sha1"), "init", false)
		}, 0,
			map[string]any{"tool": "init", "inventory.profile": genesis, "inventory.summaryProfile": genesisSummary}},
		{"init full receipt", func(t *testing.T) []string { return activationArguments(newActivationFixture(t, "sha1"), "init", true) }, 0,
			map[string]any{"tool": "init", "inventory.profile": genesis}},
		{"adopt summary", func(t *testing.T) []string {
			return activationArguments(newActivationFixture(t, "sha1"), "adopt", false)
		}, 0,
			map[string]any{"tool": "adopt", "inventory.profile": genesis, "inventory.summaryProfile": genesisSummary}},
		{"index write", func(t *testing.T) []string { return []string{"--root", impactCLIRepository(t), "index"} }, 0,
			map[string]any{"command": "index", "profile": "corvint-index-snapshot/1", "mutates": true}},
		{"index if stale when fresh", func(t *testing.T) []string {
			root := impactCLIRepository(t)
			if code, _, stderr := runCLI(t, "--root", root, "index"); code != 0 {
				t.Fatalf("index: exit %d, stderr=%s", code, stderr)
			}
			return []string{"--root", root, "index", "--if-stale"}
		}, 0, map[string]any{"state": "fresh", "ok": absentMember{}, "profile": absentMember{}, "command": absentMember{}}},
		{"query repository task", func(t *testing.T) []string {
			return []string{"--root", impactCLIRepository(t), "query", "--task", task}
		}, 0,
			map[string]any{"tool": "query", "context.schema_version": float64(1), "context.mode": "query"}},
		{"query authority start", func(t *testing.T) []string {
			return []string{"--root", queryCLIRepository(t), "query", "--task", authorityStartPrompt, "--limit", "1"}
		}, 0, map[string]any{"tool": "query", "context.schema_version": float64(1), "context.mode": "query", "context.intent.id": "project-operations"}},
		{"context task", func(t *testing.T) []string {
			return []string{"--root", impactCLIRepository(t), "context", "--task", task}
		}, 0,
			map[string]any{"tool": "context", "schema_version": float64(1)}},
		{"impact path", func(t *testing.T) []string {
			return []string{"--root", impactCLIRepository(t), "impact", "pkg/main.go"}
		}, 0,
			map[string]any{"tool": "impact", "context.schema_version": float64(1), "context.mode": "impact"}},
		{"impact committed range", func(t *testing.T) []string {
			root := impactCLIRepository(t)
			return []string{"--root", root, "impact", "--base", coreFreezeParent(t, root)}
		}, 0, map[string]any{"tool": "impact", "context.profile": "corvint-range-impact/0"}},
		{"impact untracked working tree", func(t *testing.T) []string {
			root := impactCLIRepository(t)
			writeFixtureFile(t, root, "pkg/extra.go", "package main\n")
			return []string{"--root", root, "impact", "--working-tree-untracked", "pkg/extra.go"}
		}, 0, map[string]any{"tool": "impact", "context.profile": "corvint-working-tree-impact/0"}},
		{"affected worktree", func(t *testing.T) []string { return []string{"--root", impactCLIRepository(t), "affected"} }, 0,
			map[string]any{"tool": "affected", "profile": "affected-plan/0"}},
		{"affected committed range", func(t *testing.T) []string {
			root := impactCLIRepository(t)
			return []string{"--root", root, "affected", "--base", coreFreezeParent(t, root)}
		}, 0, map[string]any{"tool": "affected", "profile": "affected-plan/0"}},
		{"prove task", func(t *testing.T) []string {
			return []string{"--root", impactCLIRepository(t), "prove", "--task", task}
		}, 0,
			map[string]any{"tool": "prove", "profile": "falsifiable-packet/0"}},
		{"prove path", func(t *testing.T) []string { return []string{"--root", impactCLIRepository(t), "prove", "pkg/main.go"} }, 0,
			map[string]any{"tool": "prove", "profile": "falsifiable-packet/0"}},
		{"prove committed range", func(t *testing.T) []string {
			root := impactCLIRepository(t)
			return []string{"--root", root, "prove", "--base", coreFreezeParent(t, root)}
		}, 0, map[string]any{"tool": "prove", "profile": "falsifiable-packet/0", "packet.profile": "corvint-range-impact/0"}},
		{"cem status", func(t *testing.T) []string { return coreEvidenceArguments(t, "cem", "status") }, 0,
			map[string]any{"tool": "cem-status", "verification.spec": "cem/0.2"}},
		{"cem verify", func(t *testing.T) []string { return coreEvidenceArguments(t, "cem", "verify") }, 0,
			map[string]any{"tool": "cem-verify", "verification.spec": "cem/0.2"}},
		{"ocm status", func(t *testing.T) []string { return coreEvidenceArguments(t, "ocm", "status") }, 0,
			map[string]any{"tool": "ocm-status", "verification.spec": "ocm/0.1-experimental"}},
		{"ocm verify", func(t *testing.T) []string { return coreEvidenceArguments(t, "ocm", "verify") }, 0,
			map[string]any{"tool": "ocm-verify", "verification.spec": "ocm/0.1-experimental"}},
		{"frontier open json", func(t *testing.T) []string { return coreEvidenceArguments(t, "frontier") }, 1,
			map[string]any{"profile": "frontier/0", "frontierState": "OPEN", "ok": absentMember{}, "mutates": absentMember{}}},
		{"dogfood status retained outcome", func(t *testing.T) []string {
			return []string{"--root", impactCLIRepository(t), "dogfood", "status", "--session-key", strings.Repeat("a", 64)}
		}, 0, map[string]any{"tool": "dogfood-status", "profile": "corvint-local-completion/0", "claim": "caller-owned-selected-workflow-only"}},
	}
	register := observeCoreEnumerations(t)
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			document := runCoreMode(t, test.invoke, test.exit)
			register.observe(t, document)
			golden := filepath.Join("testdata", "core-freeze", strings.ReplaceAll(test.name, " ", "-")+".json")
			if os.Getenv("CORVINT_UPDATE_GOLDEN") == "1" {
				time.Sleep(1100 * time.Millisecond) // a later second gives the second fixture new commit ids
				writeCoreGolden(t, golden, mergeCoreRuns(t, "$", document, runCoreMode(t, test.invoke, test.exit)))
			}
			compareCoreGolden(t, "$", readCoreGolden(t, golden), document)
			want := map[string]any{"ok": true, "mutates": false}
			for path, value := range test.want {
				want[path] = value
			}
			for path, value := range want {
				got, present := jsonMember(document, path)
				if _, absent := value.(absentMember); absent == present || !absent && got != value {
					t.Errorf("%s = %#v (present %v), want %#v", path, got, present, value)
				}
			}
		})
	}
}

// TestCoreRefusalsKeepTheFrozenEnvelope pins CCF-V1-004: a Core refusal exits 2 with empty
// stdout and one stderr JSON envelope whose ok is false and whose code keeps its frozen family,
// including the codeless repository envelope.
func TestCoreRefusalsKeepTheFrozenEnvelope(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		arguments func(string) []string
		code      string
	}{
		{"query without a task", func(root string) []string { return []string{"--root", root, "query"} }, "invalid-arguments"},
		{"context without a task", func(root string) []string { return []string{"--root", root, "context"} }, "invalid-arguments"},
		{"affected with an abbreviated base", func(root string) []string { return []string{"--root", root, "affected", "--base", "abc1234"} }, "invalid-arguments"},
		{"prove with an abbreviated base", func(root string) []string { return []string{"--root", root, "prove", "--base", "abc1234"} }, "unsupported-impact-range"},
		{"impact of an untracked path", func(root string) []string { return []string{"--root", root, "impact", "pkg/absent.go"} }, ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := impactCLIRepository(t)
			code, stdout, stderr := runCLI(t, test.arguments(root)...)
			if code != 2 || stdout != "" {
				t.Fatalf("exit %d stdout %q, want 2 and empty", code, stdout)
			}
			var envelope map[string]any
			if err := json.Unmarshal([]byte(stderr), &envelope); err != nil || strings.Count(strings.TrimSpace(stderr), "\n") != 0 {
				t.Fatalf("stderr is not one JSON line: %v\n%s", err, stderr)
			}
			if envelope["ok"] != false || envelope["error"] == nil {
				t.Fatalf("envelope %v lacks ok=false and error", envelope)
			}
			got, present := envelope["code"]
			if test.code == "" && present || test.code != "" && got != test.code {
				t.Fatalf("code = %#v (present %v), want %q", got, present, test.code)
			}
		})
	}
}

var maturityLabel = regexp.MustCompile(`([a-z][a-z-]*) \(([A-Z][A-Z0-9-]*)\)`)

// TestRootHelpLabelsEveryVerbWithMaturityAndOwner pins CCF-V1-008: root help names exactly the
// frozen Core verbs, and labels every other dispatched verb experimental with an owning spec
// prefix that docs/specs/INDEX.json indexes, so no companion or research verb is frozen by omission.
func TestRootHelpLabelsEveryVerbWithMaturityAndOwner(t *testing.T) {
	t.Parallel()
	section, found := strings.CutPrefix(rootHelp[strings.Index(rootHelp, "\nCommand maturity:\n")+1:], "Command maturity:\n")
	if !found {
		t.Fatal("root help has no Command maturity section")
	}
	section, _, _ = strings.Cut(section, "\n\n")
	core, experimental, found := strings.Cut(section, "  Experimental, no stability promise;")
	if !found {
		t.Fatalf("maturity section has no experimental block:\n%s", section)
	}
	coreVerbs := []string{}
	for _, line := range strings.Split(core, "\n") {
		if strings.HasPrefix(line, "    ") {
			coreVerbs = append(coreVerbs, strings.TrimSpace(line))
		}
	}
	if got := strings.Split(strings.Join(coreVerbs, " "), ", "); !slices.Equal(got, frozenCoreVerbs) {
		t.Fatalf("Core verbs = %q, want %q", got, frozenCoreVerbs)
	}

	raw, err := os.ReadFile(filepath.Join(moduleRoot(t), "docs", "specs", "INDEX.json"))
	if err != nil {
		t.Fatal(err)
	}
	var entries []struct {
		ReqPrefix string `json:"reqPrefix"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	indexed := map[string]bool{}
	for _, entry := range entries {
		indexed[entry.ReqPrefix] = true
	}
	if !indexed["CCF-V1"] {
		t.Error("the Core contract prefix CCF-V1 is not indexed")
	}

	labelled := map[string]string{}
	for _, match := range maturityLabel.FindAllStringSubmatch(experimental, -1) {
		if _, repeated := labelled[match[1]]; repeated {
			t.Errorf("verb %q is labelled twice", match[1])
		}
		labelled[match[1]] = match[2]
		if !indexed[match[2]] {
			t.Errorf("verb %q names owner %q, which docs/specs/INDEX.json does not index", match[1], match[2])
		}
	}
	for _, command := range topLevelCommands {
		_, isExperimental := labelled[command]
		if slices.Contains(frozenCoreVerbs, command) == isExperimental {
			t.Errorf("verb %q must be exactly one of Core or labelled experimental", command)
		}
	}
	if len(labelled)+len(frozenCoreVerbs) != len(topLevelCommands) {
		t.Errorf("%d labelled + %d Core verbs, want the %d dispatched verbs", len(labelled), len(frozenCoreVerbs), len(topLevelCommands))
	}
}

var preDispatchVerb = regexp.MustCompile(`arguments\[0\] == "([a-z][a-z-]*)"`)

// TestOnlyThePinnedHookPlumbingVerbsBypassRootHelp pins CCF-V1-003 and CCF-V1-008: the verbs
// runContext compares literally are either listed in topLevelCommands, and so labelled in root
// help, or exactly the three hook-plumbing verbs, which stay out of root help and outside the freeze.
// A fourth verb dispatched this way fails here until it is listed or deliberately pinned.
func TestOnlyThePinnedHookPlumbingVerbsBypassRootHelp(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	_, body, found := strings.Cut(string(source), "\nfunc runContext(")
	if !found {
		t.Fatal("main.go has no runContext")
	}
	body, _, _ = strings.Cut(body, "\n}\n")
	unlisted := []string{}
	for _, match := range preDispatchVerb.FindAllStringSubmatch(body, -1) {
		if !slices.Contains(topLevelCommands, match[1]) {
			unlisted = append(unlisted, match[1])
		}
	}
	if !slices.Equal(unlisted, hookPlumbingVerbs) {
		t.Fatalf("runContext dispatches unlisted verbs %q, want exactly the pinned plumbing %q", unlisted, hookPlumbingVerbs)
	}
	for _, verb := range hookPlumbingVerbs {
		if strings.Contains(rootHelp, verb) {
			t.Errorf("plumbing verb %q appears in root help", verb)
		}
	}
}

// coreEnumeration is one row of the CCF-V1-007 (d) frozen enumeration register.
type coreEnumeration struct {
	member string
	tools  []string
	values []string
	status string
}

// coreEnumerationObserver checks every string a frozen Core mode emits at a registered member
// against the register row the spec states, and records which rows some mode reached.
type coreEnumerationObserver struct {
	rows    []coreEnumeration
	mu      sync.Mutex
	reached map[int]bool
}

var registerCell = regexp.MustCompile("`([^`]+)`")

// coreEnumerationRegister reads the register table from CCF-V1-007 (d) in the Core contract, the
// single statement of every frozen enumeration, its values and its closed or open status.
func coreEnumerationRegister(t *testing.T) []coreEnumeration {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(moduleRoot(t), "docs", "specs", "core-compatibility-freeze-v1.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, clause, _ := strings.Cut(string(raw), "- **CCF-V1-007:**")
	clause, _, _ = strings.Cut(clause, "- **CCF-V1-008:**")
	rows := []coreEnumeration{}
	for _, line := range strings.Split(clause, "\n") {
		cells := strings.Split(strings.TrimSpace(line), " | ")
		if len(cells) != 5 || !strings.HasPrefix(cells[0], "| `") {
			continue
		}
		row := coreEnumeration{member: strings.Trim(cells[0], "| `"), status: cells[3]}
		for _, match := range registerCell.FindAllStringSubmatch(cells[1], -1) {
			row.tools = append(row.tools, match[1])
		}
		for _, match := range registerCell.FindAllStringSubmatch(cells[2], -1) {
			row.values = append(row.values, match[1])
		}
		if row.status != "closed" && row.status != "open" || len(row.tools) == 0 || len(row.values) == 0 {
			t.Fatalf("malformed CCF-V1-007 register row: %s", line)
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		t.Fatal("CCF-V1-007 has no frozen enumeration register")
	}
	return rows
}

// observeCoreEnumerations pins CCF-V1-007 (d) and the CCF-V1-006 enumeration rule: a value outside
// its register row fails the observing mode, and once every mode has run, a row no mode reached
// fails the parent test, so the register cannot list a member the frozen modes never emit.
func observeCoreEnumerations(t *testing.T) *coreEnumerationObserver {
	t.Helper()
	observer := &coreEnumerationObserver{rows: coreEnumerationRegister(t), reached: map[int]bool{}}
	t.Cleanup(func() {
		for index, row := range observer.rows {
			if !observer.reached[index] {
				t.Errorf("CCF-V1-007 register row %s (%v) is reached by no frozen Core mode", row.member, row.tools)
			}
		}
	})
	return observer
}

func (observer *coreEnumerationObserver) observe(t *testing.T, document map[string]any) {
	t.Helper()
	tool, _ := document["tool"].(string)
	for index, row := range observer.rows {
		if !slices.Contains(row.tools, tool) {
			continue
		}
		emitted := jsonMembers(document, strings.Split(row.member, "."))
		for _, value := range emitted {
			if text, ok := value.(string); !ok || !slices.Contains(row.values, text) {
				t.Errorf("%s %s = %#v, outside the CCF-V1-007 register values %q", tool, row.member, value, row.values)
			}
		}
		if len(emitted) != 0 {
			observer.mu.Lock()
			observer.reached[index] = true
			observer.mu.Unlock()
		}
	}
}

// jsonMembers reads every value at a dotted member path, where a name ending in [] spreads over
// the elements of that array.
func jsonMembers(value any, names []string) []any {
	if len(names) == 0 {
		return []any{value}
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	name, spread := strings.CutSuffix(names[0], "[]")
	member, present := object[name]
	if !present {
		return nil
	}
	if !spread {
		return jsonMembers(member, names[1:])
	}
	elements, _ := member.([]any)
	found := []any{}
	for _, element := range elements {
		found = append(found, jsonMembers(element, names[1:])...)
	}
	return found
}

// coreFreezeVaries prefixes a golden leaf whose value differs between two independent fixture
// builds (commit ids, temporary paths and digests over them); the golden pins its JSON type only.
const coreFreezeVaries = "<varies:"

// runCoreMode runs one frozen Core mode, checks its exit code and decodes its one JSON document.
func runCoreMode(t *testing.T, invoke func(*testing.T) []string, exit int) map[string]any {
	t.Helper()
	code, stdout, stderr := runCLI(t, invoke(t)...)
	if code != exit {
		t.Fatalf("exit %d, want %d, stderr=%s", code, exit, stderr)
	}
	var document map[string]any
	if err := json.Unmarshal([]byte(stdout), &document); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout)
	}
	return document
}

// jsonKind names the JSON type of one decoded value.
func jsonKind(value any) string {
	switch value.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	}
	return "null"
}

// mergeCoreRuns folds two independent runs of one mode into its golden (CCF-V1-002): an equal value
// stays, and a scalar that differs becomes a type placeholder. A difference in members, array length
// or type is output that cannot be frozen, and fails.
func mergeCoreRuns(t *testing.T, path string, first, second any) any {
	t.Helper()
	kind := jsonKind(first)
	if kind != jsonKind(second) {
		t.Fatalf("%s is %s in one run and %s in another: not deterministic, cannot freeze", path, kind, jsonKind(second))
	}
	switch kind {
	case "object":
		return mergeCoreObjects(t, path, first.(map[string]any), second.(map[string]any))
	case "array":
		return mergeCoreArrays(t, path, first.([]any), second.([]any))
	}
	if first == second {
		return first
	}
	return coreFreezeVaries + kind + ">"
}

func mergeCoreObjects(t *testing.T, path string, first, second map[string]any) map[string]any {
	t.Helper()
	names := slices.Sorted(maps.Keys(first))
	if !slices.Equal(names, slices.Sorted(maps.Keys(second))) {
		t.Fatalf("%s has members %q in one run and %q in another: not deterministic, cannot freeze", path, names, slices.Sorted(maps.Keys(second)))
	}
	merged := make(map[string]any, len(first))
	for _, name := range names {
		merged[name] = mergeCoreRuns(t, path+"."+name, first[name], second[name])
	}
	return merged
}

func mergeCoreArrays(t *testing.T, path string, first, second []any) []any {
	t.Helper()
	if len(first) != len(second) {
		t.Fatalf("%s has %d elements in one run and %d in another: not deterministic, cannot freeze", path, len(first), len(second))
	}
	merged := make([]any, len(first))
	for index := range first {
		merged[index] = mergeCoreRuns(t, fmt.Sprintf("%s[%d]", path, index), first[index], second[index])
	}
	return merged
}

// writeCoreGolden writes a merged golden as indented JSON with sorted members.
func writeCoreGolden(t *testing.T, path string, golden any) {
	t.Helper()
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(golden); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readCoreGolden(t *testing.T, path string) any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v: every frozen Core mode needs a golden; generate it with CORVINT_UPDATE_GOLDEN=1", err)
	}
	var golden any
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return golden
}

// compareCoreGolden reports every structural or value difference between a frozen mode's document
// and its golden. A placeholder leaf pins only the JSON type.
func compareCoreGolden(t *testing.T, path string, golden, got any) {
	t.Helper()
	placeholder, _ := golden.(string)
	if strings.HasPrefix(placeholder, coreFreezeVaries) {
		golden = strings.TrimSuffix(strings.TrimPrefix(placeholder, coreFreezeVaries), ">")
		if jsonKind(got) != golden {
			t.Errorf("%s is %s, the golden pins %s: retyping is breaking under CCF-V1-006", path, jsonKind(got), golden)
		}
		return
	}
	if jsonKind(got) != jsonKind(golden) {
		t.Errorf("%s is %s, the golden has %s: retyping is breaking under CCF-V1-006", path, jsonKind(got), jsonKind(golden))
		return
	}
	switch golden := golden.(type) {
	case map[string]any:
		compareCoreObjects(t, path, golden, got.(map[string]any))
	case []any:
		compareCoreArrays(t, path, golden, got.([]any))
	default:
		if got != golden {
			t.Errorf("%s = %#v, the golden has %#v", path, got, golden)
		}
	}
}

func compareCoreObjects(t *testing.T, path string, golden, got map[string]any) {
	t.Helper()
	for name, value := range golden {
		member, present := got[name]
		if !present {
			t.Errorf("%s.%s is missing: removing or renaming a member is breaking under CCF-V1-006", path, name)
			continue
		}
		compareCoreGolden(t, path+"."+name, value, member)
	}
	for name := range got {
		if _, known := golden[name]; !known {
			t.Errorf("%s.%s is not in the golden: an optional member is compatible under CCF-V1-006 but the same change regenerates the golden (CORVINT_UPDATE_GOLDEN=1)", path, name)
		}
	}
}

func compareCoreArrays(t *testing.T, path string, golden, got []any) {
	t.Helper()
	if len(got) != len(golden) {
		t.Errorf("%s has %d elements, the golden has %d", path, len(got), len(golden))
		return
	}
	for index := range golden {
		compareCoreGolden(t, fmt.Sprintf("%s[%d]", path, index), golden[index], got[index])
	}
}
