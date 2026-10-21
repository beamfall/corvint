package contextindex

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// DIRTY-CACHE-017: a worktree dirty only in Tasks store projection files is
// source-clean and carries a separate task-state member; any source dirt
// keeps the mixed-worktree contract unchanged.
func TestTaskStoreProjectionDirtIsSourceClean(t *testing.T) {
	root := impactRepository(t, "")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	target := "internal/token/token.go"
	projection := []string{".taskman/queue.json", ".taskman/tickets/V1-0001.json"}
	for _, tc := range []struct {
		name      string
		dirty     []string
		state     string
		mixed     int
		taskState bool
		warn      bool
	}{
		{"projection only", projection, "fresh", 0, true, false},
		{"projection and source", append([]string{"notes.txt"}, projection...), "mixed-worktree", 3, false, true},
		{"unrecognised taskman path", []string{".taskman/journal.json"}, "mixed-worktree", 1, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			view := *index
			view.DirtyPaths = tc.dirty
			if view.SourceDirty() != (tc.state != "fresh") {
				t.Fatalf("SourceDirty=%v", view.SourceDirty())
			}
			got, err := Impact(&view, []string{target}, 10)
			if err != nil {
				t.Fatal(err)
			}
			freshness := got["freshness"].(map[string]any)
			taskState, present := freshness["task_state"].(map[string]any)
			if freshness["state"] != tc.state || freshness["mixed_path_count"] != tc.mixed || present != tc.taskState {
				t.Fatalf("freshness=%v", freshness)
			}
			if _, warned := got["warning"]; warned != tc.warn {
				t.Fatalf("warning present=%v want %v", warned, tc.warn)
			}
			if present && (taskState["class"] != "TASK_STORE_PROJECTION" || taskState["path_count"] != 2 || !reflect.DeepEqual(taskState["paths"], projection) || len(freshness["mixed_paths"].([]string)) != 0) {
				t.Fatalf("task_state=%v", freshness)
			}
		})
	}
	view := *index
	view.DirtyPaths = projection
	budget := MinPacketBytes
	got, err := EvalImpact(&view, []string{target}, 10, &budget)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := CanonicalJSON(got)
	if len(raw) > budget || !strings.Contains(string(raw), `"state":"fresh"`) || !strings.Contains(string(raw), `"task_state":{"class":"TASK_STORE_PROJECTION","path_count":2`) {
		t.Fatalf("budgeted packet: %s", raw)
	}
}

// DIRTY-CACHE-017: the dogfood prompt (hook) packet reports clean plus task state.
func TestDogfoodPromptTaskStoreProjectionDirtIsClean(t *testing.T) {
	index := localPromptFixture(t)
	index.DirtyPaths = []string{".taskman/queue.json"}
	packet := localPrompt(t, index, "pkg/packet.go", nil, 20, 8000)
	taskState, _ := packet["task_state"].(map[string]any)
	if packet["freshness"] != "clean" || taskState["class"] != "TASK_STORE_PROJECTION" || taskState["path_count"] != 1 {
		t.Fatalf("freshness=%v task_state=%v", packet["freshness"], packet["task_state"])
	}
	index.DirtyPaths = []string{".taskman/queue.json", "pkg/packet.go"}
	packet = localPrompt(t, index, "pkg/packet.go", nil, 20, 8000)
	if packet["freshness"] != "mixed-worktree" || packet["task_state"] != nil {
		t.Fatalf("source dirt hidden: freshness=%v task_state=%v", packet["freshness"], packet["task_state"])
	}
}
