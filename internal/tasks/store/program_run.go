package store

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// ProgramConfig is the bounded qualification capsule for the named Codex
// profile. Stage dispatch will construct prompts from bound ticket context.
type ProgramConfig struct {
	Pool                   string `json:"pool,omitempty"`
	CoreExecutable         string `json:"coreExecutable,omitempty"`
	CoreSHA256             string `json:"coreSha256,omitempty"`
	OwnIntegrationCheckout bool   `json:"ownIntegrationCheckout,omitempty"`
	Profile                string `json:"profile"`
	Executable             string `json:"executable"`
	ExecutableSHA256       string `json:"executableSha256"`
	Model                  string `json:"model"`
	Effort                 string `json:"effort"`
	// StageEfforts optionally overrides Effort per supervised stage
	// (implement, review, integrate); every value needs a policy allowance.
	StageEfforts map[string]string `json:"stageEfforts,omitempty"`
	Prompt       string            `json:"prompt"`
	WorkRoot     string            `json:"workRoot"`
	WallSeconds  int               `json:"wallSeconds"`
	// Repositories optionally names policy-declared extra repositories the
	// program spans (CAL-V0-071), sorted by name; absent keeps single-repo
	// config bytes and digests unchanged.
	Repositories []ProgramRepository `json:"repositories,omitempty"`
	// Host optionally selects the supervised host vocabulary (CAL-V0-074):
	// "claude-code", or absent for Codex, which keeps config bytes unchanged.
	Host string `json:"host,omitempty"`
}

func RunProgram(ctx context.Context, repo *intent.Repository, actor mutation.Binding, id, self string, c ProgramConfig) (snapshot.Program, supervisor.Outcome, error) {
	p := snapshot.Program{}
	out := supervisor.Outcome{}
	if c.Profile != snapshot.SupervisedProfile || c.Model == "" || !filepath.IsAbs(c.WorkRoot) || c.Host != "" {
		return p, out, fmt.Errorf("unsupported program config")
	}
	proof, e := readLeaseProof(ctx, repo)
	if e != nil {
		return p, out, e
	}
	q, e := intent.DecodeQueue(proof.Records["intent/queue.json"].Raw)
	if e != nil {
		return p, out, e
	}
	policy, e := intent.DecodePolicy(proof.Records["intent/policy.json"].Raw)
	if e != nil {
		return p, out, e
	}
	if e = checkNewProgramConfig(c, policy.Supervision); e != nil {
		return p, out, e
	}
	raw, _, e := supervisor.LaunchableExecutable(c.Executable)
	if e != nil {
		return p, out, wire.Errorf(wire.CodeCapabilityUnavailable, "runtime", "pinned executable is not launchable: %v", e)
	}
	pinned := false
	for _, r := range policy.Runtimes {
		if r.RuntimeID == c.Profile && r.Enabled && r.FileSha256 == wire.Sum(raw) && r.PathSha256 == wire.Sum([]byte(c.Executable)) {
			pinned = true
		}
	}
	if !pinned {
		return p, out, fmt.Errorf("enabled policy runtime pin differs")
	}
	if len(policy.RequireEnforcedFields) > 0 {
		return p, out, fmt.Errorf("minimum profile has not qualified requested enforced fields")
	}
	base, _, e := poolSource(repo.PrimaryWorktree)
	if e != nil {
		return p, out, e
	}
	identity, e := supervisor.ProcessIdentity(os.Getpid())
	if e != nil || identity == "" {
		return p, out, fmt.Errorf("owner identity: %v", e)
	}
	config, _ := json.Marshal(c)
	work := filepath.Join(c.WorkRoot, id, "1", "implement-1")
	p = snapshot.Program{ID: id, Profile: c.Profile, OwnerPID: os.Getpid(), OwnerStarted: identity, Epoch: 1, ConfigSHA256: supervisor.Digest(config), Phase: "ADMITTED", Worktree: work, Base: base, Quiescence: "UNKNOWN"}
	n := 0
	record := func(phase string, boot supervisor.Boot, result *supervisor.Outcome) error {
		p.Phase = phase
		if boot.PID > 0 {
			p.LeaderPID = boot.PID
			p.LeaderStarted = boot.Started
		}
		if result != nil {
			p.ResultClass = result.Class
			p.ResultSHA256 = result.OutputSHA256
			p.SessionID = result.SessionID
			if result.Clean {
				p.Quiescence = "PROVED"
			}
		}
		n++
		var output []byte
		if result != nil {
			output = result.Stdout
		}
		report, e := ProgramTransition(context.WithoutCancel(ctx), repo, actor, q.QueueID.Raw, id+"-"+strconv.Itoa(n), p, output)
		if e != nil {
			return e
		}
		if report.Kind != "Transaction" {
			return fmt.Errorf("program transition %s: %s %s", phase, report.Kind, report.Detail)
		}
		return nil
	}
	if e = record("ADMITTED", supervisor.Boot{}, nil); e != nil {
		return p, out, e
	}
	p.Effect = supervisor.Digest([]byte(id + ":1:WORKTREE_ADD:" + base))
	if e = record("WORKTREE_ADD", supervisor.Boot{}, nil); e != nil {
		return p, out, e
	}
	if _, e = os.Lstat(work); !os.IsNotExist(e) {
		return p, out, fmt.Errorf("worktree destination already exists or inaccessible")
	}
	if e = os.MkdirAll(filepath.Dir(work), 0700); e != nil {
		return p, out, e
	}
	if _, e = gitOutput(repo.PrimaryWorktree, "worktree", "add", "--detach", work, base); e != nil {
		return p, out, e
	}
	if e = record("READY", supervisor.Boot{}, nil); e != nil {
		return p, out, e
	}
	protocol, e := os.MkdirTemp(filepath.Dir(work), "effect-")
	if e != nil {
		return p, out, e
	}
	p.Effect = supervisor.Digest([]byte(id + ":1:SPAWN:" + protocol))
	env := []string{}
	for _, k := range []string{"HOME", "PATH", "TMPDIR", "USER", "LOGNAME"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	capsule := supervisor.Capsule{Profile: c.Profile, Effect: p.Effect, Executable: c.Executable, ExecutableSHA256: c.ExecutableSHA256, Directory: work, Prompt: c.Prompt, Env: env, Argv: []string{"exec", "--json", "--sandbox", "read-only", "--model", c.Model, "-c", "model_reasoning_effort=" + strconv.Quote(c.StageEffort("implement")), "-c", "mcp_servers={}", "-"}}
	deadline, cancel := context.WithTimeout(ctx, time.Duration(c.WallSeconds)*time.Second)
	defer cancel()
	out, e = supervisor.Run(deadline, self, protocol, capsule, record)
	return p, out, e
}

// StageEffort is the configured Codex reasoning effort for one stage.
func (c ProgramConfig) StageEffort(stage string) string {
	if e, ok := c.StageEfforts[stage]; ok {
		return e
	}
	return c.Effort
}

// CheckProgramConfig refuses a supervised config whose effort or wall time
// the owner policy does not allow (CAL-V0-062, CAL-V0-063). It runs before
// any program record, worktree or host process is created.
func CheckProgramConfig(c ProgramConfig, policy *intent.SupervisionPolicy) error {
	if e := checkProgramHost(c.Host, policy); e != nil {
		return e
	}
	if e := checkProgramRepositories(c.Repositories, policy); e != nil {
		return e
	}
	if c.WallSeconds < 1 || c.WallSeconds > policy.StageWallSeconds() {
		return fmt.Errorf("wallSeconds outside 1..%d allowed by policy", policy.StageWallSeconds())
	}
	for stage := range c.StageEfforts {
		known := false
		for _, s := range intent.SupervisedStages {
			known = known || s == stage
		}
		if !known {
			return fmt.Errorf("stageEfforts names unknown stage %q", stage)
		}
	}
	for _, stage := range intent.SupervisedStages {
		if e := c.StageEffort(stage); !policy.AllowsEffort(stage, e) {
			return fmt.Errorf("effort %q for stage %s is not allowed by policy", e, stage)
		}
	}
	return nil
}

// ProgramRepository names one extra repository of a multi-repository
// supervised program (CAL-V0-071): a policy-declared name and the absolute
// checkout whose path digest the policy pins.
type ProgramRepository struct {
	Name     string `json:"name"`
	Checkout string `json:"checkout"`
}

// knownEffort keeps the default effort a supervised effort even when every
// stage overrides it, so a config never records an empty or unknown default.
func knownEffort(effort string) bool {
	for _, e := range intent.SupervisedEfforts {
		if e == effort {
			return true
		}
	}
	return false
}

// checkProgramRepositories refuses config repositories the owner policy does
// not declare, out of order, duplicated, or at a checkout whose path digest
// differs from the policy pin (CAL-V0-071).
func checkProgramRepositories(repos []ProgramRepository, policy *intent.SupervisionPolicy) error {
	for i, r := range repos {
		if i > 0 && repos[i-1].Name >= r.Name {
			return fmt.Errorf("repositories must be sorted by unique name")
		}
		pin, ok := wire.Digest(""), false
		if policy != nil {
			pin, ok = policy.Repositories[r.Name]
		}
		if !ok {
			return fmt.Errorf("repository %q is not declared by policy supervision.repositories", r.Name)
		}
		if !filepath.IsAbs(r.Checkout) || filepath.Clean(r.Checkout) != r.Checkout {
			return fmt.Errorf("repository %q checkout must be an absolute clean path", r.Name)
		}
		if wire.Sum([]byte(r.Checkout)) != pin {
			return fmt.Errorf("repository %q checkout differs from the policy path pin", r.Name)
		}
	}
	return nil
}

// checkNewProgramConfig is CheckProgramConfig for a new admission, which also
// requires the default effort to be a supervised effort even when every stage
// overrides it (CAL-V0-062). A recorded program is re-checked without this
// rule, so an existing config is never stranded by it.
func checkNewProgramConfig(c ProgramConfig, policy *intent.SupervisionPolicy) error {
	if !knownEffort(c.Effort) {
		return fmt.Errorf("effort %q is not a supervised effort", c.Effort)
	}
	return CheckProgramConfig(c, policy)
}

// checkProgramHost refuses a config host other than the policy's supervised
// host, or one no vocabulary speaks (CAL-V0-074). Codex is only ever the
// absent host, so Codex config bytes stay canonical.
func checkProgramHost(host string, policy *intent.SupervisionPolicy) error {
	if _, ok := supervisor.HostVocabulary(host); !ok || host == supervisor.HostCodex {
		return wire.Errorf(wire.CodeUnsupported, "host", "supervisor config host %q is unsupported", host)
	}
	if want := policy.SupervisedHost(); host != want {
		return wire.Errorf(wire.CodeUnsupported, "host", "supervisor config host %q differs from policy host %q", host, want)
	}
	return nil
}
