package cli

import (
	"context"
	"os"
	"os/user"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// initActor builds the local-operator binding of decision 0003. The binding
// is a recorded claim about who ran the process, never an authentication: the
// receipt carries it with ActorAuthentication NOT_OBSERVED, and `corvint-tasks`
// verifies nothing about it. A real multi-agent queue still needs a
// qualified issuer, which this is not.
func initActor(role string) (mutation.Binding, error) {
	primary := os.Getenv("CORVINT_TASKS_ACTOR")
	legacy := os.Getenv("ATM_ACTOR")
	if primary != "" && legacy != "" && primary != legacy {
		return mutation.Binding{}, wire.Errorf(wire.CodeMalformed, "actor",
			"CORVINT_TASKS_ACTOR and ATM_ACTOR disagree")
	}
	id := primary
	if id == "" {
		id = legacy
	}
	if id == "" {
		u, err := user.Current()
		if err != nil {
			return mutation.Binding{}, wire.Errorf(wire.CodeMalformed, "actor",
				"no local operator identity: set CORVINT_TASKS_ACTOR (%v)", err)
		}
		id = u.Username
	}
	if _, err := wire.ParseLabel("actor", id); err != nil {
		return mutation.Binding{}, err
	}
	return mutation.Binding{ID: id, Role: role}, nil
}

// initCommand runs `corvint-tasks init`: it qualifies the filesystem, takes the
// exclusive lock, creates the state dir and commits the genesis INIT
// transaction. It is the first verb of this binary that writes anything.
func initCommand(env Env, args []string) *wire.Result {
	cmd := []string{"init"}
	role := "OWNER"
	requestID := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--role":
			if i+1 >= len(args) {
				return usage(cmd, "--role needs a value")
			}
			i++
			role = args[i]
		case "--request-id":
			if i+1 >= len(args) {
				return usage(cmd, "--request-id needs a value")
			}
			i++
			requestID = args[i]
		default:
			return usage(cmd, "unknown init flag "+args[i])
		}
	}
	actor, err := initActor(role)
	if err != nil {
		return errorResult(cmd, err)
	}
	if requestID == "" {
		requestID = "init-" + string(actor.ID)
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	now, err := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return errorResult(cmd, err)
	}
	report, err := store.Init(context.Background(), repo, actor, requestID, now)
	if err != nil {
		return errorResult(cmd, err)
	}
	o := wire.NewObject()
	o.Set("stateDir", wire.String(repo.StateDir))
	o.Set("primaryWorktree", wire.String(repo.PrimaryWorktree))
	o.Set("receipt", wire.String(report.Receipt))
	o.Set("directories", wire.Strings(report.Directories))
	o.Set("actorAuthentication", wire.String(report.Coverage.ActorAuthentication))
	o.Set("administrativeAuthorization", wire.String(report.Coverage.AdministrativeAuthorization))
	o.Set("durability", wire.String(report.Coverage.Durability))
	res := &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
	if report.Receipt == "" {
		res.Outcome = wire.OutcomeRefused
		res.Codes = report.Outcome.Codes
		reason := "init committed nothing"
		if report.Outcome.Outcome == "BLOCKED" {
			reason = "already initialized: the state dir exists, so init wrote nothing"
		}
		if report.Detail != "" {
			reason = prose(report.Detail)
		}
		res.Warnings = append(res.Warnings, reason)
	}
	res.Warnings = append(res.Warnings,
		"the actor binding is a recorded local-operator claim, not an authentication (decision 0003); a real queue still needs the §7.4 cutover record")
	return res
}

// errorResult reports a refusal or failure with its own code, so a caller
// sees which step did not happen rather than a bare failure.
func errorResult(cmd []string, err error) *wire.Result {
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeError, Codes: []string{wire.CodeOf(err)}, Warnings: []string{prose(err.Error())}}
}
