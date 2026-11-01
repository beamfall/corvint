package contextindex

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func localPromptFixture(t *testing.T) *Index {
	t.Helper()
	return localPromptFixtureWith(t, nil)
}

func localPromptFixtureWith(t *testing.T, extra map[string]string) *Index {
	t.Helper()
	files := map[string]string{
		"go.mod":               "module example.test/prompt\n\ngo 1.27.0\n",
		"AGENTS.md":            "# Instructions\nPreserve project authority.\n",
		"docs/specs/packet.md": "# Packet\n\n- PKT-001: Validate packet bytes.\n",
		"pkg/packet.go":        "package packet\n\n// can you fix all that so its fully dogfooding now\nfunc ParsePacket() {}\nfunc All() {}\nfunc Now() {}\nfunc DuplicateName() {}\nfunc x() {}\nfunc id() {}\nfunc all() {}\nfunc now() {}\n",
		"other/other.go":       "package other\n\nfunc DuplicateName() {}\n",
		"one/duplicate.go":     "package one\n",
		"two/duplicate.go":     "package two\n",
	}
	maps.Copy(files, extra)
	root := impactRepositoryWithFiles(t, files)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func localPrompt(t *testing.T, index *Index, task string, scope []PinnedIntentPointer, limit, budget int) map[string]any {
	t.Helper()
	packet, err := DogfoodPromptContext(context.Background(), index, task, scope, limit, budget)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := CanonicalJSON(packet)
	if err != nil {
		t.Fatal(err)
	}
	coverage := packet["coverage"].(map[string]any)
	if coverage["packet_bytes"] != len(encoded)+1 || len(encoded)+1 > budget {
		t.Fatalf("incorrect packet accounting: coverage=%v bytes=%d", coverage, len(encoded)+1)
	}
	return packet
}

// LCP-V0-010: these cases were frozen before the implementation, including
// framing-word collisions and explicit ordinary-word declaration names.
func TestDogfoodPromptFrozenAnchors(t *testing.T) {
	t.Run("LCP-V0-010 anchor", func(t *testing.T) {
		index := localPromptFixture(t)
		data, err := os.ReadFile("testdata/local-completion/prompt-cases.json")
		if err != nil {
			t.Fatal(err)
		}
		var cases []struct {
			Task, Reason string
			Paths        []string
		}
		if err := json.Unmarshal(data, &cases); err != nil {
			t.Fatal(err)
		}
		for _, test := range cases {
			t.Run(test.Task, func(t *testing.T) {
				packet := localPrompt(t, index, test.Task, nil, 20, 8000)
				resolution := packet["resolution"].(map[string]any)
				if resolution["reason"] != test.Reason {
					t.Fatalf("resolution=%v; want %s", resolution, test.Reason)
				}
				found := []string{}
				for _, row := range mapsFromAny(packet["task_evidence"]) {
					found = append(found, row["path"].(string))
					if !localObjectID(row["blob_hash"].(string)) || row["line"].(int) < 1 {
						t.Fatalf("unpinned evidence: %v", row)
					}
				}
				slices.Sort(found)
				if !reflect.DeepEqual(found, test.Paths) {
					t.Fatalf("paths=%v want=%v", found, test.Paths)
				}
				governance := mapsFromAny(packet["governance"])
				if len(governance) != 1 || governance[0]["path"] != "AGENTS.md" {
					t.Fatalf("governance=%v", governance)
				}
			})
		}
	})
}

func TestDogfoodPromptScopeDoesNotResolveAndPreservesStaleness(t *testing.T) {
	t.Run("LCP-V0-010 anchor", func(t *testing.T) {
		index := localPromptFixture(t)
		pointer := PinnedIntentPointer{Path: "docs/specs/packet.md", Revision: index.Revision, BlobHash: index.Sources["docs/specs/packet.md"].BlobHash}
		packet := localPrompt(t, index, "can you fix all that", []PinnedIntentPointer{pointer}, 20, 8000)
		if packet["resolution"].(map[string]any)["reason"] != "explicit-task-anchor-required" || len(mapsFromAny(packet["task_evidence"])) != 0 {
			t.Fatalf("scope rescued an unresolved prompt: %v", packet)
		}
		if mapsFromAny(packet["declared_scope"])[0]["state"] != "current" {
			t.Fatalf("current scope=%v", packet["declared_scope"])
		}
		pointer.BlobHash = strings.Repeat("b", len(pointer.BlobHash))
		packet = localPrompt(t, index, "can you fix all that", []PinnedIntentPointer{pointer}, 20, 8000)
		if mapsFromAny(packet["declared_scope"])[0]["state"] != "stale" {
			t.Fatal("changed scope was silently repinned")
		}
		pointer.BlobHash = ""
		packet = localPrompt(t, index, "can you fix all that", []PinnedIntentPointer{pointer}, 20, 8000)
		if mapsFromAny(packet["declared_scope"])[0]["state"] != "unavailable" {
			t.Fatal("unavailable enrolled scope was silently repinned")
		}
	})
}

func TestDogfoodPromptPrivacyNoHistoryAndNonmutation(t *testing.T) {
	t.Run("LCP-V0-011 privacy", func(t *testing.T) {
		index := localPromptFixture(t)
		// A fully acquired index is sufficient. The compiler cannot reopen history,
		// traces or repository files even when its Root points at a nonexistent path.
		index.Root = filepath.Join(t.TempDir(), "does-not-exist")
		before, _ := json.Marshal(index.Sources)
		packet := localPrompt(t, index, "fix `PrivateCanary9876` and `OtherPrivateCanary8765` in pkg/packet.go", nil, 20, 8000)
		encoded, _ := CanonicalJSON(packet)
		for _, forbidden := range []string{"PrivateCanary", "OtherPrivateCanary", "specific_terms", "nearest_claims", "fix `"} {
			if strings.Contains(string(encoded), forbidden) {
				t.Fatalf("private prompt fragment leaked: %s", forbidden)
			}
		}
		after, _ := json.Marshal(index.Sources)
		if string(before) != string(after) {
			t.Fatal("compiler mutated index")
		}
		if _, err := os.Stat(index.Root); !os.IsNotExist(err) {
			t.Fatalf("compiler touched nonexistent repository: %v", err)
		}
	})
}

func TestDogfoodPromptCriticalBudgetAndImpossibleEnvelope(t *testing.T) {
	t.Run("LCP-V0-011 privacy", func(t *testing.T) {
		index := localPromptFixture(t)
		pointer := PinnedIntentPointer{Path: "docs/specs/packet.md", Revision: index.Revision, BlobHash: index.Sources["docs/specs/packet.md"].BlobHash}
		limited := localPrompt(t, index, "pkg/packet.go", []PinnedIntentPointer{pointer}, 1, 8000)
		coverage := limited["coverage"].(map[string]any)
		if coverage["included_results"] != 1 || coverage["omitted_results"] != 2 || len(coverage["critical_missing"].([]any)) != 2 {
			t.Fatalf("lost critical omissions: %v", coverage)
		}
		full := localPrompt(t, index, "pkg/packet.go", []PinnedIntentPointer{pointer}, 20, 8000)
		fullSize := full["coverage"].(map[string]any)["packet_bytes"].(int)
		budgeted := localPrompt(t, index, "pkg/packet.go", []PinnedIntentPointer{pointer}, 20, fullSize-10)
		if budgeted["coverage"].(map[string]any)["omitted_results"].(int) == 0 {
			t.Fatal("budget failed to omit a critical result")
		}
		if _, err := DogfoodPromptContext(context.Background(), index, "pkg/packet.go", nil, 20, 1); err == nil {
			t.Fatal("impossible mandatory envelope admitted")
		}
	})
}

func TestDogfoodPromptUnavailableGovernanceAnchorAndDirtyEvidence(t *testing.T) {
	t.Parallel()
	t.Run("LCP-V0-010 anchor", func(t *testing.T) {
		index := localPromptFixture(t)
		delete(index.Sources, "AGENTS.md")
		packet := localPrompt(t, index, "pkg/packet.go", nil, 20, 8000)
		if len(mapsFromAny(packet["governance"])) != 0 || len(packet["coverage"].(map[string]any)["unavailable_selectors"].([]any)) != 1 {
			t.Fatal("unread governance was hidden or substituted")
		}
		delete(index.Sources, "pkg/packet.go")
		packet = localPrompt(t, index, "pkg/packet.go", nil, 20, 8000)
		if packet["resolution"].(map[string]any)["reason"] != "anchor-evidence-unavailable" {
			t.Fatal("tracked unread anchor reported absent or resolved")
		}
		index = localPromptFixture(t)
		index.DirtyPaths = []string{"pkg/packet.go"}
		packet = localPrompt(t, index, "pkg/packet.go", nil, 20, 8000)
		if packet["freshness"] != "mixed-worktree" || packet["resolution"].(map[string]any)["reason"] != "anchor-worktree-changed" {
			t.Fatal("dirty evidence presented as current")
		}
	})
}

func TestDogfoodPromptBoundsAndCancellation(t *testing.T) {
	t.Run("LCP-V0-012 bound", func(t *testing.T) {
		index := localPromptFixture(t)
		for _, pointer := range []PinnedIntentPointer{
			{Path: "../private", Revision: index.Revision},
			{Path: "docs/specs/packet.md", Revision: "main"},
			{Path: "docs/specs/packet.md", Revision: index.Revision, BlobHash: "bad"},
		} {
			if _, err := DogfoodPromptContext(context.Background(), index, "pkg/packet.go", []PinnedIntentPointer{pointer}, 20, 8000); err == nil {
				t.Fatalf("invalid scope admitted: %v", pointer)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := DogfoodPromptContext(ctx, index, "pkg/packet.go", nil, 20, 8000); err != context.Canceled {
			t.Fatalf("cancellation=%v", err)
		}
		for _, task := range []string{strings.Repeat("x", 2001), string([]byte{0xff})} {
			if _, err := DogfoodPromptContext(context.Background(), index, task, nil, 20, 8000); err == nil {
				t.Fatal("invalid task admitted")
			}
		}
	})
}

// LCP-V0-010: startup has no prompt; this is never a successful task resolution.
func TestDogfoodPromptStartupAndExplicitIdentifierShapes(t *testing.T) {
	t.Run("LCP-V0-010 anchor", func(t *testing.T) {
		index := localPromptFixture(t)
		packet := localPrompt(t, index, "", nil, 20, 8000)
		if packet["resolution"].(map[string]any)["reason"] != "explicit-task-anchor-required" || len(mapsFromAny(packet["governance"])) != 1 {
			t.Fatal("empty startup falsely resolved or lost governance")
		}
		for _, task := range []string{"x", "id", "all", "now", "fix `x`", "fix `id`", "fix `all`", "fix `now`"} {
			packet := localPrompt(t, index, task, nil, 20, 8000)
			if packet["resolution"].(map[string]any)["reason"] != "none" || len(mapsFromAny(packet["task_evidence"])) != 1 {
				t.Fatalf("explicit short/lowercase identifier failed: %s => %v", task, packet)
			}
		}
		for _, task := range []string{"fix `packet.ParsePacket`", "inspect packet.ParsePacket"} {
			packet := localPrompt(t, index, task, nil, 20, 8000)
			resolution := packet["resolution"].(map[string]any)
			if resolution["reason"] != "qualification-unverified" || resolution["anchors"] != 1 || resolution["missing_anchors"] != 0 || len(mapsFromAny(packet["task_evidence"])) != 1 {
				t.Fatalf("qualification was guessed or counted twice: %v", packet)
			}
		}
	})
}

func TestDogfoodPromptExtensionlessPathsAndPunctuation(t *testing.T) {
	t.Run("LCP-V0-010 anchor", func(t *testing.T) {
		root := impactRepositoryWithFiles(t, map[string]string{"AGENTS.md": "# Instructions\n", "Makefile": "all:\n", "scripts/check": "#!/bin/sh\nexit 0\n", "docs/guide.md": "# Guide\n"})
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		for _, task := range []string{"inspect Makefile", "inspect scripts/check", "inspect docs/guide.md?"} {
			packet := localPrompt(t, index, task, nil, 20, 8000)
			if packet["resolution"].(map[string]any)["anchors"] != 1 {
				t.Fatalf("path not recognized: %s => %v", task, packet)
			}
			// An unsupported extension may be tracked but unread; it is never absent.
			if packet["resolution"].(map[string]any)["reason"] == "anchor-not-found" {
				t.Fatalf("tracked path treated as absent: %s", task)
			}
		}
	})
	// LCP-V0-039: an absolute or rebased mention of a tracked top-level file
	// without a word extension still names it after the rebase.
	t.Run("LCP-V0-039 rebased extensionless", func(t *testing.T) {
		root := impactRepositoryWithFiles(t, map[string]string{"AGENTS.md": "# Instructions\n", "Makefile": "all:\n\ttrue\n", ".gitignore": "bin/\n"})
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		slashed := filepath.ToSlash(index.Root)
		for _, name := range []string{"Makefile", ".gitignore"} {
			for _, task := range []string{
				"inspect " + slashed + "/" + name,
				"inspect " + strings.TrimPrefix(slashed, "/") + "/" + name,
				"see " + slashed + "/" + name + ":1",
			} {
				got := localPrompt(t, index, task, nil, 20, 8000)["resolution"].(map[string]any)
				if got["anchors"] != 1 || got["reason"] == "anchor-not-found" {
					t.Fatalf("rebased tracked file dropped: %q => %v", task, got)
				}
			}
		}
	})
}

func TestDogfoodPromptOmittedTaskNeverResolves(t *testing.T) {
	t.Run("LCP-V0-011 privacy", func(t *testing.T) {
		index := localPromptFixture(t)
		packet := localPrompt(t, index, "pkg/packet.go", nil, 1, 8000)
		if packet["resolution"].(map[string]any)["reason"] != "task-evidence-omitted" || len(mapsFromAny(packet["task_evidence"])) != 0 {
			t.Fatal("governance-only packet resolved task")
		}
		packet = localPrompt(t, index, "pkg/packet.go pkg/missing.go", nil, 1, 8000)
		if packet["resolution"].(map[string]any)["reason"] != "anchor-not-found" {
			t.Fatal("omission overwrote missing-anchor diagnostic")
		}
	})
}

func TestDogfoodPromptEarlyResourceBounds(t *testing.T) {
	t.Run("LCP-V0-012 bound", func(t *testing.T) {
		index := localPromptFixture(t)
		for i := 0; i < localPromptMaxCandidates+1; i++ {
			index.Symbols = append(index.Symbols, Symbol{Name: "Excess", Path: fmt.Sprintf("pkg/%d.go", i), Line: 1})
		}
		if _, err := DogfoodPromptContext(context.Background(), index, "`Excess`", nil, 20, 8000); err == nil || !strings.Contains(err.Error(), "bounded candidate profile") {
			t.Fatalf("candidate ceiling=%v", err)
		}
	})
}

// Cancel deterministically during the index scan, not merely before invocation.
type localPromptCancelContext struct{ calls, cancelAt int }

func (ctx *localPromptCancelContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (ctx *localPromptCancelContext) Done() <-chan struct{}       { return nil }
func (ctx *localPromptCancelContext) Value(any) any               { return nil }
func (ctx *localPromptCancelContext) Err() error {
	ctx.calls++
	if ctx.calls >= ctx.cancelAt {
		return context.Canceled
	}
	return nil
}

func TestDogfoodPromptCancellationDuringCompilation(t *testing.T) {
	t.Run("LCP-V0-012 bound", func(t *testing.T) {
		index := localPromptFixture(t)
		ctx := &localPromptCancelContext{cancelAt: 8}
		_, err := DogfoodPromptContext(ctx, index, "fix `ParsePacket`", nil, 20, 8000)
		if err != context.Canceled {
			t.Fatalf("in-flight cancellation=%v (polls=%d)", err, ctx.calls)
		}
	})
}

func localMentionFixture(t *testing.T) *Index {
	t.Helper()
	return localPromptFixtureWith(t, map[string]string{
		"pkg/methods.go":  "package packet\n\ntype Reader struct{}\ntype Writer struct{}\n\nfunc (Reader) Close() {}\nfunc (Writer) Close() {}\n",
		"one/sized.go":    "package one\n\nfunc Sized() {}\n",
		"two/sized.go":    "package two\n",
		"one/abc1234x.go": "package one\n",
		".env":            "TOKEN=canary\n",
	})
}

func localPromptRows(packet map[string]any) []string {
	rows := []string{}
	for _, row := range mapsFromAny(packet["task_evidence"]) {
		rows = append(rows, fmt.Sprintf("%s:%d", row["path"], row["line"]))
	}
	slices.Sort(rows)
	return rows
}

// LCP-V0-013: every mention form and refusal, frozen before the implementation.
func TestDogfoodPromptMentionAnchors(t *testing.T) {
	t.Run("LCP-V0-013 mention", func(t *testing.T) {
		index := localMentionFixture(t)
		data, err := os.ReadFile("testdata/local-completion/mention-cases.json")
		if err != nil {
			t.Fatal(err)
		}
		var cases []struct {
			Task, Reason string
			Rows         []string
		}
		if err := json.Unmarshal(data, &cases); err != nil {
			t.Fatal(err)
		}
		for _, test := range cases {
			t.Run(test.Task, func(t *testing.T) {
				packet := localPrompt(t, index, test.Task, nil, 20, 8000)
				resolution := packet["resolution"].(map[string]any)
				if resolution["reason"] != test.Reason {
					t.Fatalf("resolution=%v; want %s", resolution, test.Reason)
				}
				for _, row := range mapsFromAny(packet["task_evidence"]) {
					if !localObjectID(row["blob_hash"].(string)) {
						t.Fatalf("unpinned evidence: %v", row)
					}
				}
				if rows := localPromptRows(packet); !reflect.DeepEqual(rows, test.Rows) {
					t.Fatalf("rows=%v want=%v", rows, test.Rows)
				}
			})
		}
	})
}

func TestDogfoodPromptMentionIdentityAndRefusals(t *testing.T) {
	t.Parallel()
	t.Run("LCP-V0-013 mention", func(t *testing.T) {
		index := localMentionFixture(t)
		packet := localPrompt(t, index, "inspect pkg/packet.go:4-6", nil, 20, 8000)
		rows := mapsFromAny(packet["task_evidence"])
		if resolution := packet["resolution"].(map[string]any); resolution["anchors"] != 1 || len(rows) != 1 || rows[0]["relation"] != "explicit-line" || rows[0]["authority"] != "task-text" {
			t.Fatalf("line mention counted twice or mislabelled: %v", packet)
		}
		packet = localPrompt(t, index, "fix pkg/packet.go#ParsePacket", nil, 20, 8000)
		if rows := mapsFromAny(packet["task_evidence"]); len(rows) != 1 || rows[0]["relation"] != "explicit-identifier" || rows[0]["authority"] != "syntax" {
			t.Fatalf("symbol mention mislabelled: %v", packet)
		}
		// The shortest accepted prefix must hold a digit and a letter.
		short := 7
		for short < len(index.CommitRevision) && !(strings.ContainsAny(index.CommitRevision[:short], "0123456789") && strings.ContainsAny(index.CommitRevision[:short], "abcdef")) {
			short++
		}
		for _, commit := range []string{index.CommitRevision, index.CommitRevision[:short], index.CommitRevision[:max(short, 12)]} {
			packet := localPrompt(t, index, "revert "+commit, nil, 20, 8000)
			resolution := packet["resolution"].(map[string]any)
			if resolution["reason"] != "none" || resolution["anchors"] != 1 || len(mapsFromAny(packet["task_evidence"])) != 0 {
				t.Fatalf("bound commit %s not resolved as a revision anchor: %v", commit, packet)
			}
		}
		packet = localPrompt(t, index, "inspect ./pkg/packet.go:4 and pkg/packet.go:4", nil, 20, 8000)
		if packet["resolution"].(map[string]any)["anchors"] != 1 {
			t.Fatalf("one mention in two spellings counted twice: %v", packet["resolution"])
		}
		index.DirtyPaths = []string{"pkg/packet.go"}
		packet = localPrompt(t, index, "inspect pkg/packet.go:4", nil, 20, 8000)
		if packet["resolution"].(map[string]any)["reason"] != "anchor-worktree-changed" {
			t.Fatal("dirty line mention presented as current")
		}
		// Lines or declarations added in the worktree are absent from the bound
		// blob; a dirty path reports that change, never anchor-not-found.
		for _, task := range []string{"inspect pkg/packet.go:50", "fix pkg/packet.go#NewFunc"} {
			packet = localPrompt(t, index, task, nil, 20, 8000)
			if packet["resolution"].(map[string]any)["reason"] != "anchor-worktree-changed" || len(mapsFromAny(packet["task_evidence"])) != 0 {
				t.Fatalf("%s on a dirty path: %v", task, packet)
			}
		}
		// No content satisfies an inverted or zero range, dirty or not.
		for _, task := range []string{"inspect pkg/packet.go:0", "inspect pkg/packet.go:9-4"} {
			packet = localPrompt(t, index, task, nil, 20, 8000)
			if packet["resolution"].(map[string]any)["reason"] != "anchor-not-found" {
				t.Fatalf("%s on a dirty path: %v", task, packet["resolution"])
			}
		}
		// A dirty base-name candidate may hold the line in the worktree, so a
		// clean in-range match beside it is ambiguous, not unique.
		index.DirtyPaths = []string{"two/sized.go"}
		packet = localPrompt(t, index, "inspect sized.go:3", nil, 20, 8000)
		if packet["resolution"].(map[string]any)["reason"] != "ambiguous-anchor" || !reflect.DeepEqual(localPromptRows(packet), []string{"one/sized.go:3"}) {
			t.Fatalf("dirty base-name candidate: %v", packet)
		}
		index = localMentionFixture(t)
		delete(index.Sources, "pkg/packet.go")
		packet = localPrompt(t, index, "inspect pkg/packet.go:99", nil, 20, 8000)
		if packet["resolution"].(map[string]any)["reason"] != "anchor-evidence-unavailable" {
			t.Fatal("unread line mention reported absent or resolved")
		}
		packet = localPrompt(t, index, "inspect pkg/packet.go:4", nil, 1, 8000)
		if packet["resolution"].(map[string]any)["reason"] != "anchor-evidence-unavailable" {
			t.Fatal("unread line mention resolved")
		}
		index = localMentionFixture(t)
		packet = localPrompt(t, index, "inspect pkg/packet.go:4", nil, 1, 8000)
		if packet["resolution"].(map[string]any)["reason"] != "task-evidence-omitted" {
			t.Fatal("omitted mention evidence resolved")
		}
	})
}

// LCP-V0-035: more than localPromptMaxAnchors anchors degrade to a partial
// packet that keeps the first anchors in the fixed kind order and counts the
// dropped ones by kind, never naming the raw task terms.
func TestDogfoodPromptLCPV0035KeepsFirstAnchorsInOrder(t *testing.T) {
	index := localPromptFixture(t)
	task := "pkg/packet.go pkg/missing.go "
	for i := 0; i < localPromptMaxAnchors+3; i++ {
		task += fmt.Sprintf("`Dropped%02d` ", i)
	}
	first := localPrompt(t, index, task, nil, 20, 8000)
	resolution := first["resolution"].(map[string]any)
	// Two paths come before identifiers, so 30 identifiers are kept and 5 dropped.
	if resolution["anchors"] != localPromptMaxAnchors || resolution["dropped_anchors"] != 5 {
		t.Fatalf("resolution=%v", resolution)
	}
	if kinds := resolution["dropped_anchor_kinds"].(map[string]any); len(kinds) != 1 || kinds["identifiers"] != 5 {
		t.Fatalf("dropped kinds=%v", kinds)
	}
	encoded, err := CanonicalJSON(first)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "Dropped") {
		t.Fatalf("packet names a raw task term: %s", encoded)
	}
	// The order is fixed: a permuted prompt yields the same packet.
	fields := strings.Fields(task)
	slices.Reverse(fields)
	again, err := CanonicalJSON(localPrompt(t, index, strings.Join(fields, " "), nil, 20, 8000))
	if err != nil {
		t.Fatal(err)
	}
	firstNoTask, againNoTask := dropTaskDigest(t, encoded), dropTaskDigest(t, again)
	if firstNoTask != againNoTask {
		t.Fatalf("order not deterministic:\n%s\n%s", firstNoTask, againNoTask)
	}
	// Within the bound the packet carries no dropped-anchor field.
	if within := localPrompt(t, index, "pkg/packet.go", nil, 20, 8000)["resolution"].(map[string]any); within["dropped_anchors"] != nil || within["dropped_anchor_kinds"] != nil {
		t.Fatalf("within-bound resolution=%v", within)
	}
	// Paths alone past the bound drop the last path in sorted order.
	resolved := "pkg/packet.go"
	for i := 0; i < localPromptMaxAnchors; i++ {
		resolved += fmt.Sprintf(" pkg/extra%02d.go", i)
	}
	packet := localPrompt(t, index, resolved, nil, 20, 8000)
	if got := packet["resolution"].(map[string]any); got["dropped_anchors"] != 1 || got["anchors"] != localPromptMaxAnchors {
		t.Fatalf("path bound resolution=%v", got)
	}
}

func dropTaskDigest(t *testing.T, encoded []byte) string {
	t.Helper()
	var packet map[string]any
	if err := json.Unmarshal(encoded, &packet); err != nil {
		t.Fatal(err)
	}
	delete(packet, "task_sha256")
	again, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	return string(again)
}

// LCP-V0-039: prose that cannot name anything in this tree is not an anchor, so
// it is neither resolved nor missing; an explicit selector keeps its outcome.
func TestDogfoodPromptLCPV0039ProseIsNotAnAnchor(t *testing.T) {
	t.Run("LCP-V0-039 prose", func(t *testing.T) {
		index := localPromptFixtureWith(t, map[string]string{
			"web/config.json": "{}\n",
			"pkg/encode.go":   "package packet\n\nfunc json() {}\n",
		})
		root := filepath.ToSlash(index.Root)
		resolution := func(task string) map[string]any {
			t.Helper()
			return localPrompt(t, index, task, nil, 20, 8000)["resolution"].(map[string]any)
		}
		// A real selector among prose noise resolves alone.
		for _, task := range []string{
			"fix pkg/packet.go after origin/main merged; see https://example.com/a:1, pass/fail 28/28, 1.64 rc.2 7.7% go1.27.3 e.g. NOT_RUN GitHub </agent-message> **REPORT** MET.** in pkg and docs/specs/",
			"inspect " + root + "/pkg/packet.go",
			"inspect " + strings.TrimPrefix(root, "/") + "/pkg/packet.go and /private/elsewhere/pkg/packet.go",
		} {
			packet := localPrompt(t, index, task, nil, 20, 8000)
			got := packet["resolution"].(map[string]any)
			if got["reason"] != "none" || got["anchors"] != 1 || !slices.Contains(localPromptRows(packet), "pkg/packet.go:1") {
				t.Fatalf("selector among prose: %q => %v rows=%v", task, got, localPromptRows(packet))
			}
		}
		packet := localPrompt(t, index, "see "+root+"/pkg/packet.go:4", nil, 20, 8000)
		if rows := localPromptRows(packet); !slices.Equal(rows, []string{"pkg/packet.go:4"}) {
			t.Fatalf("absolute mention under the root: %v", rows)
		}
		// Prose alone names no anchor and abstains.
		for _, task := range []string{
			"look at pkg and docs/specs/ then origin/main",
			"see https://example.com/pkg/missing.go",
			"compare 1.64 to rc.2 and 7.7% on linux/amd64 e.g. today",
			"read /private/elsewhere/pkg/missing.go and ../pkg/missing.go",
			"GitHub NOT_RUN WSL2 qa.Probe",
			"update pkg/*.go and *_test.sh",
		} {
			if got := resolution(task); got["reason"] != "explicit-task-anchor-required" || got["anchors"] != 0 {
				t.Fatalf("prose became an anchor: %q => %v", task, got)
			}
		}
		// Explicit selectors keep their outcome.
		for task, reason := range map[string]string{
			"inspect pkg/missing.go":                 "anchor-not-found",
			"inspect " + root + "/pkg/missing.go":    "anchor-not-found",
			"repair `SecretCanaryUnknown`":           "anchor-not-found",
			"inspect missing.go":                     "anchor-not-found",
			"Read ParsePacket in packet.go":          "none",
			"inspect packet.ParsePacket":             "qualification-unverified",
			"repair `qa.Probe`":                      "anchor-not-found",
			"inspect config.json, not the json func": "none",
		} {
			if got := resolution(task); got["reason"] != reason {
				t.Fatalf("explicit selector: %q => %v, want %s", task, got, reason)
			}
		}
	})
	t.Run("LCP-V0-039 base-name overflow", func(t *testing.T) {
		extra := map[string]string{}
		for i := 0; i <= localPromptMaxCandidates; i++ {
			extra[fmt.Sprintf("m%03d/main.go", i)] = "package main\n"
		}
		index := localPromptFixtureWith(t, extra)
		for _, task := range []string{"inspect main.go and pkg/packet.go", "see main.go:1 and pkg/packet.go", "see main.go#main and pkg/packet.go"} {
			packet := localPrompt(t, index, task, nil, 20, 8000)
			got := packet["resolution"].(map[string]any)
			if got["reason"] != "ambiguous-anchor" || got["anchors"] != 2 || got["ambiguous_anchors"] != 1 || got["missing_anchors"] != 0 || !slices.Equal(localPromptRows(packet), []string{"pkg/packet.go:1"}) {
				t.Fatalf("over-bound base name: %q => %v rows=%v", task, got, localPromptRows(packet))
			}
		}
		// Two ambiguous names that fit alone but not together: the second
		// counts ambiguous without rows instead of refusing the prompt.
		extra = map[string]string{}
		for i := 0; i < 70; i++ {
			extra[fmt.Sprintf("a%02d/x.go", i)], extra[fmt.Sprintf("b%02d/y.go", i)] = "package a\n", "package b\n"
		}
		index = localPromptFixtureWith(t, extra)
		packet := localPrompt(t, index, "inspect x.go and y.go", nil, 20, 8000)
		got := packet["resolution"].(map[string]any)
		if got["reason"] != "ambiguous-anchor" || got["anchors"] != 2 || got["ambiguous_anchors"] != 2 {
			t.Fatalf("crowded anchors: %v", got)
		}
	})
}

// LCP-V0-955 (V1-3542): a governing file the working tree modifies is downgraded
// with its TCP-V0-057 reason, and an unread one says so, in the selector itself.
func TestLocalPromptDowngradedGovernanceNamesItsReason(t *testing.T) {
	t.Parallel()
	index := localPromptFixture(t)
	index.DirtyPaths = []string{"AGENTS.md"}
	packet := localPrompt(t, index, "pkg/packet.go", nil, 20, 8000)
	selectors := mapsFromAny(packet["coverage"].(map[string]any)["unavailable_selectors"])
	if len(selectors) != 1 || selectors[0]["section"] != "governance" || selectors[0]["path"] != "AGENTS.md" ||
		selectors[0]["reason"] != DowngradedAuthority || !reflect.DeepEqual(selectors[0]["warnings"], []any{SelfModifiedAuthorityWarning}) {
		t.Fatalf("downgraded governance selector = %v", selectors)
	}
	index = localPromptFixture(t)
	delete(index.Sources, "AGENTS.md")
	packet = localPrompt(t, index, "pkg/packet.go", nil, 20, 8000)
	selectors = mapsFromAny(packet["coverage"].(map[string]any)["unavailable_selectors"])
	if len(selectors) != 1 || selectors[0]["reason"] != "unread" {
		t.Fatalf("unread governance selector = %v", selectors)
	}
}
