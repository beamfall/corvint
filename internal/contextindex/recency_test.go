package contextindex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

const (
	recencyOldDate = "2025-01-01T00:00:00Z"
	recencyNewDate = "2026-06-01T00:00:00Z"
)

// recencyCommit commits everything staged at one author and committer date.
func recencyCommit(t *testing.T, root, date, message string) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_DATE", date)
	t.Setenv("GIT_COMMITTER_DATE", date)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", message)
}

func recencyRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, "go.mod", "module example.test/fixture\n\ngo 1.27.0\n")
	return root
}

func recencyPacket(t *testing.T, root, task, subject string) map[string]any {
	t.Helper()
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := TaskContext(context.Background(), index, task, subject, 20)
	if err != nil {
		t.Fatal(err)
	}
	return packet
}

func rowReason(t *testing.T, packet map[string]any, id string) string {
	t.Helper()
	for _, item := range mapsFromAny(packet["results"]) {
		if item["id"] == id {
			return mapsFromAny(item["evidence"])[0]["reason"].(string)
		}
	}
	t.Fatalf("no row %s in %v", id, contextRowsByKind(t, packet))
	return ""
}

func TestContextRecencyDefaultBytes(t *testing.T) {
	t.Run("TCP-V0-035", func(t *testing.T) {
		index := recipeFixtureIndex(t)
		golden, err := os.ReadFile("testdata/context-recipe-default-golden.json")
		if err != nil {
			t.Fatal(err)
		}
		for _, flag := range []string{"", "off", "unknown"} {
			t.Setenv("CORVINT_CONTEXT_RECENCY", flag)
			if recency := startContextRecency(context.Background(), index); recency != nil {
				t.Fatalf("flag %q started a history reading", flag)
			}
			encoded, err := json.MarshalIndent(recipePacket(t, index, "", recipeFixtureTask), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(append(encoded, '\n'), golden) {
				t.Fatalf("flag %q changed golden packet bytes", flag)
			}
		}
		t.Setenv("CORVINT_CONTEXT_RECENCY", "on")
		coverage := recipePacket(t, index, "", recipeFixtureTask)["coverage"].(map[string]any)
		if _, ok := coverage["recency"]; !ok {
			t.Fatal("flag on must add coverage.recency")
		}
	})
}

// TestContextRecencyRanksRecentLexicalRowsAndNamesFeatures: two sources with
// equal BM25 keep path order by default; with the flag the recently written
// one leads, and every lexical row names the recency and blame features.
func TestContextRecencyRanksRecentLexicalRowsAndNamesFeatures(t *testing.T) {
	t.Run("TCP-V0-035", func(t *testing.T) {
		root := recencyRepository(t)
		body := "package fixture\n\n// Widget renders the gizmo.\nfunc Widget() string { return \"gizmo\" }\n"
		writeTestFile(t, root, "alpha/old.go", body)
		recencyCommit(t, root, recencyOldDate, "old widget")
		writeTestFile(t, root, "beta/new.go", body)
		recencyCommit(t, root, recencyNewDate, "new widget")
		task := "Where does the gizmo widget render"
		t.Setenv("CORVINT_CONTEXT_RECENCY", "")
		if lexical := contextRowsByKind(t, recencyPacket(t, root, task, ""))["lexical"]; len(lexical) < 2 || lexical[0] != "alpha/old.go" {
			t.Fatalf("default lexical order = %v, want the equal-score path order", lexical)
		}
		t.Setenv("CORVINT_CONTEXT_RECENCY", "on")
		packet := recencyPacket(t, root, task, "")
		if lexical := contextRowsByKind(t, packet)["lexical"]; len(lexical) < 2 || lexical[0] != "beta/new.go" {
			t.Fatalf("recency lexical order = %v, want the recent source first", lexical)
		}
		recent := rowReason(t, packet, "beta/new.go")
		for _, want := range []string{"recency 1.00 (last commit 0 days", "blame 1.00 (4 of 4 lines", "rank bm25 x 1.50"} {
			if !strings.Contains(recent, want) {
				t.Fatalf("recent reason %q lacks %q", recent, want)
			}
		}
		if old := rowReason(t, packet, "alpha/old.go"); !strings.Contains(old, "recency 0.02 (last commit 516 days") || !strings.Contains(old, "blame 0.00 (0 of 4 lines") {
			t.Fatalf("old reason %q must name both features, the root commit's lines outside the window", old)
		}
	})
}

func TestContextRecencyBoundsBlameAndAbstains(t *testing.T) {
	t.Run("TCP-V0-036", func(t *testing.T) {
		root := recencyRepository(t)
		for index := 0; index < contextBlameCap+2; index++ {
			writeTestFile(t, root, fmt.Sprintf("pkg/file%02d.go", index), fmt.Sprintf("package pkg\n\n// sprocket %s\n", strings.Repeat("sprocket ", index+1)))
		}
		recencyCommit(t, root, recencyNewDate, "sprockets")
		t.Setenv("CORVINT_CONTEXT_RECENCY", "on")
		packet := recencyPacket(t, root, "sprocket", "")
		summary := packet["coverage"].(map[string]any)["recency"].(map[string]any)
		if summary["blamed"] != contextBlameCap || summary["state"] != "examined" || summary["window_commits"] != 1 {
			t.Fatalf("coverage.recency = %v, want %d blamed over a one-commit window", summary, contextBlameCap)
		}
		bounded := 0
		for _, item := range mapsFromAny(packet["results"]) {
			if strings.Contains(mapsFromAny(item["evidence"])[0]["reason"].(string), "blame abstained (beyond the 10-file blame bound)") {
				bounded++
			}
		}
		if bounded != 2 {
			t.Fatalf("%d rows name the blame bound, want 2", bounded)
		}
		recency := &contextRecency{state: "examined", lastTouch: map[string]int64{}, commits: maxHistoryCommits}
		if reason := recency.reason("elsewhere.go"); reason != "recency abstained (no commit in the 200-commit window)" {
			t.Fatalf("out-of-window reason = %q", reason)
		}
		if reason := (&contextRecency{state: "history-unreadable"}).reason("x.go"); reason != "recency abstained (history-unreadable)" {
			t.Fatalf("unreadable reason = %q", reason)
		}
	})
}

// TestContextRecencyWindowIsTheCochangeWindow: a full window records its
// oldest commit, which bounds the blame walk.
func TestContextRecencyWindowIsTheCochangeWindow(t *testing.T) {
	t.Run("TCP-V0-036", func(t *testing.T) {
		var raw bytes.Buffer
		for index := 0; index < maxHistoryCommits; index++ {
			fmt.Fprintf(&raw, "\x1e%040x\x1f%d\x00\npath%03d.go\x00", index+1, 1_700_000_000-index, index)
		}
		recency := &contextRecency{index: &Index{ObjectFormat: "sha1"}}
		if state := recency.parse(raw.Bytes()); state != "examined" || !recency.windowFull || recency.oldest != fmt.Sprintf("%040x", maxHistoryCommits) {
			t.Fatalf("state %q full %v oldest %q", state, recency.windowFull, recency.oldest)
		}
		if recency.lastTouch["path000.go"] != 1_700_000_000 || len(recency.lastTouch) != maxHistoryCommits {
			t.Fatalf("lastTouch = %d entries", len(recency.lastTouch))
		}
	})
}

// TestContextRecencyWeightsCochangeByAge: two old co-changes lose to one
// recent co-change under the half-life, and the default keeps raw counts.
func TestContextRecencyWeightsCochangeByAge(t *testing.T) {
	t.Run("TCP-V0-035", func(t *testing.T) {
		root := recencyRepository(t)
		writeTestFile(t, root, "core/subject.go", "package core\n\nfunc Subject() {}\n")
		recencyCommit(t, root, recencyOldDate, "start")
		for revision := 1; revision <= 2; revision++ {
			writeTestFile(t, root, "core/subject.go", fmt.Sprintf("package core\n\nfunc Subject() int { return %d }\n", revision))
			writeTestFile(t, root, "notes/stale.txt", fmt.Sprintf("stale %d\n", revision))
			recencyCommit(t, root, recencyOldDate, "old pair")
		}
		writeTestFile(t, root, "core/subject.go", "package core\n\nfunc Subject() int { return 3 }\n")
		writeTestFile(t, root, "notes/fresh.txt", "fresh\n")
		recencyCommit(t, root, recencyNewDate, "new pair")
		t.Setenv("CORVINT_CONTEXT_RECENCY", "")
		if rows := contextRowsByKind(t, recencyPacket(t, root, "adjust the subject", "core/subject.go"))["cochange"]; len(rows) < 2 || rows[0] != "notes/stale.txt" {
			t.Fatalf("default cochange = %v, want the two-commit path first", rows)
		}
		t.Setenv("CORVINT_CONTEXT_RECENCY", "on")
		packet := recencyPacket(t, root, "adjust the subject", "core/subject.go")
		if rows := contextRowsByKind(t, packet)["cochange"]; len(rows) < 2 || rows[0] != "notes/fresh.txt" {
			t.Fatalf("recency cochange = %v, want the recent co-change first", rows)
		}
		if reason := rowReason(t, packet, "notes/fresh.txt"); !strings.HasSuffix(reason, "; recency-weighted 1.00 (90-day half-life)") {
			t.Fatalf("cochange reason = %q", reason)
		}
	})
}

// TestContextRecencyCoverageMember: the flag adds one `coverage.recency`
// member naming what the features read, and a history the packet cannot read
// abstains in that member rather than ranking by a guess.
func TestContextRecencyCoverageMember(t *testing.T) {
	t.Run("TCP-V0-038", func(t *testing.T) {
		root := recencyRepository(t)
		writeTestFile(t, root, "pkg/gear.go", "package pkg\n\n// Gear turns.\nfunc Gear() {}\n")
		recencyCommit(t, root, recencyNewDate, "gear")
		t.Setenv("CORVINT_CONTEXT_RECENCY", "on")
		summary := recencyPacket(t, root, "gear turns", "")["coverage"].(map[string]any)["recency"].(map[string]any)
		want := map[string]any{"state": "examined", "half_life_days": contextRecencyHalfLifeDays, "window_commits": 1, "window_full": false, "blame_cap": contextBlameCap, "blamed": 1, "codeowners": nil}
		for key, value := range want {
			if summary[key] != value {
				t.Fatalf("coverage.recency[%s] = %v, want %v (%v)", key, summary[key], value, summary)
			}
		}
		if ownership := mapsFromAny(summary["ownership"]); len(ownership) != 0 || len(summary) != len(want)+1 {
			t.Fatalf("coverage.recency = %v, want exactly the named members and an empty ownership list", summary)
		}
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		index.CommitRevision = ""
		packet, err := TaskContext(context.Background(), index, "gear turns", "", 20)
		if err != nil {
			t.Fatal(err)
		}
		if state := packet["coverage"].(map[string]any)["recency"].(map[string]any)["state"]; state != "no-indexed-commit" {
			t.Fatalf("state = %v, want no-indexed-commit", state)
		}
		if reason := rowReason(t, packet, "pkg/gear.go"); !strings.Contains(reason, "recency abstained (no-indexed-commit); blame abstained (no-indexed-commit)") {
			t.Fatalf("reason %q must name both abstentions", reason)
		}
	})
}

// TestContextRecencyCanChangeLexicalMembership: the fill is reordered before
// the limit, so a recent candidate below the cut can displace an older one.
func TestContextRecencyCanChangeLexicalMembership(t *testing.T) {
	t.Run("TCP-V0-035", func(t *testing.T) {
		root := recencyRepository(t)
		body := "package fixture\n\n// Widget renders the gizmo.\nfunc Widget() string { return \"gizmo\" }\n"
		for index := range 5 {
			writeTestFile(t, root, fmt.Sprintf("a%d.go", index), body)
		}
		recencyCommit(t, root, recencyOldDate, "old widgets")
		writeTestFile(t, root, "z/new.go", body)
		recencyCommit(t, root, recencyNewDate, "new widget")
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		lexical := func() []string {
			packet, err := TaskContext(context.Background(), index, "Where does the gizmo widget render", "", 3)
			if err != nil {
				t.Fatal(err)
			}
			return contextRowsByKind(t, packet)["lexical"]
		}
		t.Setenv("CORVINT_CONTEXT_RECENCY", "")
		if off := fmt.Sprint(lexical()); off != "[a0.go a1.go a2.go]" {
			t.Fatalf("default lexical rows = %s", off)
		}
		t.Setenv("CORVINT_CONTEXT_RECENCY", "on")
		if on := fmt.Sprint(lexical()); on != "[z/new.go a0.go a1.go]" {
			t.Fatalf("recency lexical rows = %s, want the recent file admitted and a2.go cut", on)
		}
	})
}
