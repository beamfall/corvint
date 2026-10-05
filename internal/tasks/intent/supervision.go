package intent

import "github.com/Beamfall/corvint/internal/tasks/wire"

// SupervisedStages are the native supervised stages a policy effort
// allowlist may name (CAL-V0-062).
var SupervisedStages = []string{"implement", "integrate", "review"}

// SupervisedEfforts are the Codex reasoning efforts an owner may allow.
var SupervisedEfforts = []string{"high", "low", "medium"}

// DefaultStageWallMinutes is the per-stage wall bound when the policy does
// not declare stageWallMinutes; it preserves the original one-hour cap.
const DefaultStageWallMinutes = 60

// MaxStageWallMinutes bounds the optional per-stage wall allowance. It equals
// the lane wallClockMinutes ceiling, because the active stage deadline is never
// longer than the lane cap (CAL-V0-063); a larger value would have no effect.
const MaxStageWallMinutes = wire.MaxLaneWallMinutes

// MaxStageContinuations bounds the optional checkpointed continuations of one
// supervised stage run (CAL-V0-089). Each continuation is another dispatched
// turn, so the lane turn cap and the program turn and wall caps still bound
// the total.
const MaxStageContinuations = 16

type SupervisionPolicy struct {
	MaxRepairCycles, Turns, WallClockMinutes wire.Count
	InputTokens, OutputTokens                wire.Size
	// Efforts is the owner's per-stage effort allowlist (CAL-V0-062). A
	// stage without an entry allows only "low".
	Efforts map[string][]string
	// StageWallMinutes bounds one supervised stage's configured wall time
	// (CAL-V0-063); empty means DefaultStageWallMinutes.
	StageWallMinutes wire.Count
	// Repositories are the owner-declared extra repositories a supervised
	// program may span (CAL-V0-071): name -> sha256 of the absolute
	// checkout path. Empty means single-repository programs only.
	Repositories map[string]wire.Digest
	// Host is the owner-selected supervised host the pinned runtime speaks
	// (CAL-V0-074, CAL-V0-076): "claude-code", "opencode", or empty for Codex.
	Host string
	// Continuations is how many times one stage run that reaches its stage
	// wall may continue its preserved session and worktree (CAL-V0-089);
	// empty means none, so a wall interruption waits for an operator.
	Continuations wire.Count
}

// AllowsEffort reports whether the policy admits effort for stage. A nil
// policy, or a stage without an allowlist, admits only "low".
func (p *SupervisionPolicy) AllowsEffort(stage, effort string) bool {
	if p == nil || p.Efforts[stage] == nil {
		return effort == "low"
	}
	for _, e := range p.Efforts[stage] {
		if e == effort {
			return true
		}
	}
	return false
}

// StageWallSeconds is the largest configured wall time one stage may use.
func (p *SupervisionPolicy) StageWallSeconds() int {
	if p == nil || p.StageWallMinutes == "" {
		return DefaultStageWallMinutes * 60
	}
	return int(p.StageWallMinutes.Int()) * 60
}

// StageContinuations is the policy's checkpointed continuation bound for one
// stage run; a nil policy or an absent key allows none.
func (p *SupervisionPolicy) StageContinuations() int {
	if p == nil || p.Continuations == "" {
		return 0
	}
	return int(p.Continuations.Int())
}

func readSupervisionPolicy(r *wire.Reader) *SupervisionPolicy {
	r.Closed(wire.OptionalKeys(r.Value(), []string{"profile", "maxRepairCycles", "contextRequired", "program"}, "efforts", "stageWallMinutes", "repositories", "host", "continuations")...)
	if r.Field("profile").String() != "taskman-codex-supervisor/0" || !r.Field("contextRequired").Bool() {
		r.Fail(wire.CodeUnsupported, "supervision profile/context")
	}
	p := &SupervisionPolicy{MaxRepairCycles: r.Field("maxRepairCycles").Count()}
	b := r.Field("program")
	b.Closed("turns", "wallClockMinutes", "inputTokens", "outputTokens")
	p.Turns = b.Field("turns").Count()
	p.WallClockMinutes = b.Field("wallClockMinutes").Count()
	p.InputTokens = b.Field("inputTokens").Size()
	p.OutputTokens = b.Field("outputTokens").Size()
	if p.MaxRepairCycles.Int() > 2 || p.Turns.Int() < 1 || p.WallClockMinutes.Int() < 1 {
		r.Fail(wire.CodeMalformed, "supervision bounds")
	}
	if wire.Has(r.Value(), "efforts") {
		e := r.Field("efforts")
		e.Closed(wire.OptionalKeys(e.Value(), nil, SupervisedStages...)...)
		p.Efforts = map[string][]string{}
		for _, stage := range SupervisedStages {
			if !wire.Has(e.Value(), stage) {
				continue
			}
			list := []string{}
			for _, v := range e.Field(stage).Array(len(SupervisedEfforts), false) {
				list = append(list, v.Enum(SupervisedEfforts...))
			}
			if len(list) == 0 {
				e.Fail(wire.CodeMalformed, "effort allowlist for %s is empty", stage)
			}
			p.Efforts[stage] = list
		}
		if len(p.Efforts) == 0 {
			e.Fail(wire.CodeMalformed, "efforts names no stage")
		}
	}
	if wire.Has(r.Value(), "stageWallMinutes") {
		p.StageWallMinutes = boundCount(r.Field("stageWallMinutes"), 1, MaxStageWallMinutes)
	}
	if wire.Has(r.Value(), "continuations") {
		p.Continuations = boundCount(r.Field("continuations"), 1, MaxStageContinuations)
	}
	if wire.Has(r.Value(), "repositories") {
		p.Repositories = readSupervisedRepositories(r.Field("repositories"))
	}
	if wire.Has(r.Value(), "host") {
		if p.Host = r.Field("host").String(); p.Host != SupervisedHostClaudeCode && p.Host != SupervisedHostOpenCode {
			r.Fail(wire.CodeUnsupported, "supervision host %q", p.Host)
		}
	}
	return p
}

// MaxSupervisedRepositories bounds the extra repositories one policy may
// declare for supervised multi-repository programs (CAL-V0-071).
const MaxSupervisedRepositories = 8

// ValidRepositoryName reports whether name is a supervised repository name:
// a lowercase ASCII letter followed by at most 31 lowercase letters, digits
// or hyphens. The name becomes a composite tree entry and a worktree suffix.
func ValidRepositoryName(name string) bool {
	if len(name) == 0 || len(name) > 32 || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for i := 1; i < len(name); i++ {
		c := name[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

func readSupervisedRepositories(r *wire.Reader) map[string]wire.Digest {
	if r.Value().Kind != wire.KindObject {
		r.Fail(wire.CodeMalformed, "supervision repositories must be an object")
		return nil
	}
	keys := r.Value().Obj.Keys
	if len(keys) == 0 || len(keys) > MaxSupervisedRepositories {
		r.Fail(wire.CodeMalformed, "supervision repositories must declare 1..%d repositories", MaxSupervisedRepositories)
		return nil
	}
	out := map[string]wire.Digest{}
	for _, name := range keys {
		if !ValidRepositoryName(name) {
			r.Fail(wire.CodeMalformed, "invalid supervised repository name %q", name)
			return nil
		}
		x := r.Field(name)
		x.Closed("pathSha256")
		out[name] = x.Field("pathSha256").Digest()
	}
	return out
}

// SupervisedHostClaudeCode and SupervisedHostOpenCode are the non-default
// supervised hosts a policy may select (CAL-V0-074, CAL-V0-076); an absent
// host is Codex.
const (
	SupervisedHostClaudeCode = "claude-code"
	SupervisedHostOpenCode   = "opencode"
)

// SupervisedHost is the policy's supervised host; a nil policy or an absent
// host is Codex ("").
func (p *SupervisionPolicy) SupervisedHost() string {
	if p == nil {
		return ""
	}
	return p.Host
}
