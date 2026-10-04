package store

import (
	"context"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
	"testing"
)

func TestCALV0062_StageEffortSelection(t *testing.T) {
	c := ProgramConfig{Model: "pinned-model", Effort: "low", StageEfforts: map[string]string{"implement": "high", "review": "medium"}}
	for stage, want := range map[string]string{"implement": "high", "review": "medium", "integrate": "low"} {
		if got := c.StageEffort(stage); got != want {
			t.Fatalf("%s effort %q, want %q", stage, got, want)
		}
	}
	argv := strings.Join(codexStageArgv(c, "implement", ""), " ")
	if argv != `exec --json --sandbox workspace-write --model pinned-model -c model_reasoning_effort="high" -c mcp_servers={} -` {
		t.Fatalf("implement argv %s", argv)
	}
	argv = strings.Join(codexStageArgv(c, "review", "session-7"), " ")
	if argv != `exec resume session-7 --json --model pinned-model -c model_reasoning_effort="medium" -c sandbox_mode="read-only" -c mcp_servers={} -` {
		t.Fatalf("resumed review argv %s", argv)
	}
	argv = strings.Join(codexStageArgv(c, "integrate", ""), " ")
	if !strings.Contains(argv, `model_reasoning_effort="low"`) || !strings.Contains(argv, "--sandbox read-only") {
		t.Fatalf("integrate argv %s", argv)
	}
}

func TestCALV0062_CheckProgramConfigEffort(t *testing.T) {
	allow := &intent.SupervisionPolicy{Efforts: map[string][]string{"implement": {"high", "medium"}, "review": {"high", "low", "medium"}}}
	base := ProgramConfig{Effort: "low", WallSeconds: 3600}
	cases := []struct {
		name   string
		policy *intent.SupervisionPolicy
		c      ProgramConfig
		refuse string
	}{
		{"absent policy keeps low", nil, base, ""},
		{"absent policy refuses medium", nil, ProgramConfig{Effort: "medium", WallSeconds: 60}, `effort "medium"`},
		{"default effort outside implement allowlist", allow, base, `effort "low" for stage implement`},
		{"allowed per-stage efforts", allow, ProgramConfig{Effort: "low", WallSeconds: 60, StageEfforts: map[string]string{"implement": "medium", "review": "high"}}, ""},
		{"integrate falls back outside allowlist", allow, ProgramConfig{Effort: "medium", WallSeconds: 60, StageEfforts: map[string]string{"integrate": "medium"}}, `stage integrate`},
		{"unknown stage", allow, ProgramConfig{Effort: "low", WallSeconds: 60, StageEfforts: map[string]string{"implement": "high", "repair": "high"}}, `unknown stage "repair"`},
		{"unknown effort", allow, ProgramConfig{Effort: "low", WallSeconds: 60, StageEfforts: map[string]string{"implement": "xhigh"}}, `effort "xhigh"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := CheckProgramConfig(tc.c, tc.policy)
			if tc.refuse == "" && e != nil {
				t.Fatalf("refused: %v", e)
			}
			if tc.refuse != "" && (e == nil || !strings.Contains(e.Error(), tc.refuse)) {
				t.Fatalf("want refusal %q, got %v", tc.refuse, e)
			}
		})
	}
}

// TestCALV0062_StageRechecksCurrentPolicy proves an existing program whose
// policy was narrowed is refused at stage launch, before any worktree, record
// or host process (the workflow below has none to touch).
func TestCALV0062_StageRechecksCurrentPolicy(t *testing.T) {
	w := &Workflow{cfg: ProgramConfig{Effort: "medium", WallSeconds: 60}, policy: &intent.Policy{}}
	if _, e := w.stage(context.Background(), "review"); e == nil || !strings.Contains(e.Error(), `effort "medium"`) {
		t.Fatalf("narrowed policy reached launch: %v", e)
	}
	w.cfg = ProgramConfig{Effort: "low", WallSeconds: 7200}
	if _, e := w.stage(context.Background(), "implement"); e == nil || !strings.Contains(e.Error(), "wallSeconds") {
		t.Fatalf("narrowed wall reached launch: %v", e)
	}
}

func TestCALV0063_CheckProgramConfigStageWall(t *testing.T) {
	long := &intent.SupervisionPolicy{StageWallMinutes: wire.CountOf(240)}
	for _, tc := range []struct {
		policy *intent.SupervisionPolicy
		wall   int
		ok     bool
	}{
		{nil, 0, false}, {nil, 1, true}, {nil, 3600, true}, {nil, 3601, false},
		{&intent.SupervisionPolicy{}, 3601, false},
		{long, 3601, true}, {long, 240 * 60, true}, {long, 240*60 + 1, false},
	} {
		e := CheckProgramConfig(ProgramConfig{Effort: "low", WallSeconds: tc.wall}, tc.policy)
		if (e == nil) != tc.ok {
			t.Fatalf("wall %d under %+v: %v", tc.wall, tc.policy, e)
		}
	}
}

// TestCALV0062_BaseEffortRequired proves the default effort must itself be a
// supervised effort even when every stage overrides it.
func TestCALV0062_BaseEffortRequired(t *testing.T) {
	all := map[string]string{"implement": "low", "review": "low", "integrate": "low"}
	for _, effort := range []string{"", "xhigh"} {
		e := CheckProgramConfig(ProgramConfig{Effort: effort, WallSeconds: 60, StageEfforts: all}, nil)
		if e == nil || !strings.Contains(e.Error(), "is not a supervised effort") {
			t.Fatalf("base effort %q accepted: %v", effort, e)
		}
	}
	if e := CheckProgramConfig(ProgramConfig{Effort: "low", WallSeconds: 60, StageEfforts: all}, nil); e != nil {
		t.Fatalf("low base refused: %v", e)
	}
}

func TestCALV0071_CheckProgramConfigRepositories(t *testing.T) {
	docs, site := "/work/docs", "/work/site"
	policy := &intent.SupervisionPolicy{Repositories: map[string]wire.Digest{"docs": wire.Sum([]byte(docs)), "site": wire.Sum([]byte(site))}}
	for _, tc := range []struct {
		policy *intent.SupervisionPolicy
		repos  []ProgramRepository
		refuse string
	}{
		{policy, nil, ""},
		{policy, []ProgramRepository{{"docs", docs}, {"site", site}}, ""},
		{nil, []ProgramRepository{{"docs", docs}}, "not declared by policy"},
		{policy, []ProgramRepository{{"wiki", docs}}, "not declared by policy"},
		{policy, []ProgramRepository{{"site", site}, {"docs", docs}}, "sorted by unique name"},
		{policy, []ProgramRepository{{"docs", docs}, {"docs", docs}}, "sorted by unique name"},
		{policy, []ProgramRepository{{"docs", "work/docs"}}, "absolute clean path"},
		{policy, []ProgramRepository{{"docs", "/work/../work/docs"}}, "absolute clean path"},
		{policy, []ProgramRepository{{"docs", site}}, "differs from the policy path pin"},
	} {
		e := CheckProgramConfig(ProgramConfig{Effort: "low", WallSeconds: 60, Repositories: tc.repos}, tc.policy)
		if tc.refuse == "" && e != nil {
			t.Fatalf("%+v refused: %v", tc.repos, e)
		}
		if tc.refuse != "" && (e == nil || !strings.Contains(e.Error(), tc.refuse)) {
			t.Fatalf("%+v: want %q, got %v", tc.repos, tc.refuse, e)
		}
	}
}

// TestCALV0071_WritableRoots proves only an implement stage gains the extra
// repositories' sibling worktrees as sandbox writable roots, in sorted order
// and before the prompt argument.
func TestCALV0071_WritableRoots(t *testing.T) {
	c := ProgramConfig{Model: "pinned-model", Effort: "low"}
	extras := map[string]string{"site": "/w/implement-1@site", "docs": "/w/implement-1@docs"}
	argv := strings.Join(withWritableRoots(codexStageArgv(c, "implement", ""), "implement", extras), " ")
	if !strings.HasSuffix(argv, ` -c sandbox_workspace_write.writable_roots=["/w/implement-1@docs","/w/implement-1@site"] -`) {
		t.Fatalf("implement argv %s", argv)
	}
	argv = strings.Join(withWritableRoots(codexStageArgv(c, "implement", "session-1"), "implement", extras), " ")
	if !strings.HasPrefix(argv, "exec resume session-1 ") || !strings.Contains(argv, "writable_roots") {
		t.Fatalf("resumed implement argv %s", argv)
	}
	for _, stage := range []string{"review", "integrate"} {
		if argv := strings.Join(withWritableRoots(codexStageArgv(c, stage, ""), stage, extras), " "); strings.Contains(argv, "writable_roots") {
			t.Fatalf("%s argv %s", stage, argv)
		}
	}
	if got, want := withWritableRoots(codexStageArgv(c, "implement", ""), "implement", nil), codexStageArgv(c, "implement", ""); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatal("single-repository argv changed")
	}
}
