package contextindex

import (
	"context"
	"encoding/json"
	"fmt"
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
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":               "module example.test/prompt\n\ngo 1.27.0\n",
		"AGENTS.md":            "# Instructions\nPreserve project authority.\n",
		"docs/specs/packet.md": "# Packet\n\n- PKT-001: Validate packet bytes.\n",
		"pkg/packet.go":        "package packet\n\n// can you fix all that so its fully dogfooding now\nfunc ParsePacket() {}\nfunc All() {}\nfunc Now() {}\nfunc DuplicateName() {}\nfunc x() {}\nfunc id() {}\nfunc all() {}\nfunc now() {}\n",
		"other/other.go":       "package other\n\nfunc DuplicateName() {}\n",
		"one/duplicate.go":     "package one\n",
		"two/duplicate.go":     "package two\n",
	})
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
		task := ""
		for i := 0; i < localPromptMaxAnchors+1; i++ {
			task += fmt.Sprintf("`Symbol%d` ", i)
		}
		if _, err := DogfoodPromptContext(context.Background(), index, task, nil, 20, 8000); err == nil || !strings.Contains(err.Error(), "bounded candidate profile") {
			t.Fatalf("anchor ceiling=%v", err)
		}
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
