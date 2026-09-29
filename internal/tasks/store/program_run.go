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
	Prompt                 string `json:"prompt"`
	WorkRoot               string `json:"workRoot"`
	WallSeconds            int    `json:"wallSeconds"`
}

func RunProgram(ctx context.Context, repo *intent.Repository, actor mutation.Binding, id, self string, c ProgramConfig) (snapshot.Program, supervisor.Outcome, error) {
	p := snapshot.Program{}
	out := supervisor.Outcome{}
	if c.Profile != snapshot.SupervisedProfile || c.WallSeconds < 1 || c.WallSeconds > 3600 || c.Model == "" || c.Effort != "low" || !filepath.IsAbs(c.WorkRoot) {
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
	raw, e := supervisor.ReadBounded(c.Executable, 256<<20)
	if e != nil {
		return p, out, e
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
	capsule := supervisor.Capsule{Profile: c.Profile, Effect: p.Effect, Executable: c.Executable, ExecutableSHA256: c.ExecutableSHA256, Directory: work, Prompt: c.Prompt, Env: env, Argv: []string{"exec", "--json", "--sandbox", "read-only", "--model", c.Model, "-c", "model_reasoning_effort=" + strconv.Quote(c.Effort), "-c", "mcp_servers={}", "-"}}
	deadline, cancel := context.WithTimeout(ctx, time.Duration(c.WallSeconds)*time.Second)
	defer cancel()
	out, e = supervisor.Run(deadline, self, protocol, capsule, record)
	return p, out, e
}
