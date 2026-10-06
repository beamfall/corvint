package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"io"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

func programRun(env Env, args []string) *wire.Result { return programCommand(env, "run", args) }
func programCommand(env Env, verb string, args []string) *wire.Result {
	cmd := []string{verb}
	values := map[string]string{}
	seen := map[string]bool{}
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) || seen[args[i]] {
			return usage(cmd, "missing or repeated run flag")
		}
		seen[args[i]] = true
		switch args[i] {
		case "--program", "--config", "--role", "--host", "--count", "--ticket", "--grant", "--question", "--answer", "--revision":
			values[args[i]] = args[i+1]
		default:
			return usage(cmd, "minimum qualified run accepts --program and --config")
		}
	}
	if values["--program"] == "" || values["--config"] == "" {
		return usage(cmd, "run requires --program and --config")
	}
	raw, e := intent.ReadFile(values["--config"], 65536)
	if e != nil {
		return errorResult(cmd, e)
	}
	if e = wire.RawProfileVersion("/profile", raw, snapshot.SupervisedProfile); e != nil {
		return errorResult(cmd, e)
	}
	var c store.ProgramConfig
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e = d.Decode(&c); e != nil {
		return errorResult(cmd, e)
	}
	if d.Decode(new(any)) != io.EOF {
		return usage(cmd, "trailing config input")
	}
	repo, e := intent.Resolve(env.Cwd)
	if e != nil {
		return errorResult(cmd, e)
	}
	actor, e := initActor("OPERATOR")
	if e != nil {
		return errorResult(cmd, e)
	}
	self, e := os.Executable()
	if e != nil {
		return errorResult(cmd, e)
	}
	ctx, cancel := signal.NotifyContext(writerContext(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if host := values["--host"]; host != "" && !configHost(host, c.Host) {
		return errorResult(cmd, wire.Errorf(wire.CodeUnsupported, "host", "--host %s differs from the config host", host))
	}
	count := 1
	if values["--count"] != "" {
		count, e = strconv.Atoi(values["--count"])
		if e != nil || count < 1 || count > 16 {
			return usage(cmd, "count outside1..16")
		}
	}
	if count > 1 && values["--ticket"] != "" {
		return usage(cmd, "count and explicit ticket are exclusive")
	}
	role := values["--role"]
	if role == "" {
		role = "implementer"
	}
	if role != "implementer" && role != "reviewer" && role != "integrator" {
		return usage(cmd, "unknown role")
	}
	allItems := []wire.Value{}
	requestedCount := count
	for round := 0; round < 64; round++ {
		count = requestedCount
		entries, e := store.ProgramRecords(ctx, repo)
		if e != nil {
			return errorResult(cmd, e)
		}
		attempts, e := store.ProgramAttempts(ctx, repo)
		if e != nil {
			return errorResult(cmd, e)
		}
		candidates := []string{}
		used := map[string]bool{}
		for _, p := range entries {
			used[p.ID] = true
			if p.Group != values["--program"] && p.ID != values["--program"] {
				continue
			}
			a := attempts[p.ID]
			if a == nil {
				if p.Phase == "ADMITTED" {
					candidates = append(candidates, p.ID)
				}
				continue
			}
			eligible := verb != "run" && verb != "admit" && a.Live()
			if verb == "run" || verb == "admit" {
				switch role {
				case "implementer":
					eligible = eligible || !a.Live() || a.Phase == "ADMITTED" || a.Phase == "RETURNED" || (a.Phase == "WAITING" && a.Supervision.Answer != "")
				case "reviewer":
					eligible = eligible || a.Phase == "BUILT"
				case "integrator":
					eligible = eligible || a.Phase == "READY_FOR_INTEGRATION"
				}
			}
			if verb == "answer" {
				eligible = a.Phase == "WAITING" && a.Supervision.QuestionID == values["--question"]
			}
			if eligible {
				candidates = append(candidates, p.ID)
			}
		}
		sort.Strings(candidates)
		if (role != "implementer" || verb != "run" && verb != "admit") && len(candidates) == 0 {
			return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Codes: []string{}, Items: allItems}
		}
		if (verb == "drain" || verb == "cancel") && values["--count"] == "" {
			count = len(candidates)
		}
		if (role != "implementer" || verb != "run" && verb != "admit") && count > len(candidates) {
			count = len(candidates)
		}
		for len(candidates) < count {
			id := values["--program"]
			if used[id] {
				for n := 1; ; n++ {
					id = fmt.Sprintf("%s-%03d", values["--program"], n)
					if !used[id] {
						break
					}
				}
			}
			used[id] = true
			candidates = append(candidates, id)
		}
		items := make([]wire.Value, count)
		failures := make([]error, count)
		var group sync.WaitGroup
		for index := 0; index < count; index++ {
			group.Add(1)
			go func(i int) {
				defer group.Done()
				id := candidates[i]
				ticketID := values["--ticket"]
				if ticketID != "" && !strings.HasPrefix(ticketID, "ticket:") {
					observed, e := snapshot.Probe(repo.StateDir)
					if e != nil {
						failures[i] = e
						return
					}
					ticketID = qualifyTicket(observed.Head.QueueID.Raw, ticketID)
				}
				if verb == "cancel" || verb == "drain" {
					if e := store.RequestProgramControl(ctx, repo, actor, id, strings.ToUpper(verb)); e != nil {
						failures[i] = e
						return
					}
				}
				workflow, e := store.OpenWorkflow(ctx, repo, actor, id, self, c, ticketID, values["--program"])
				if e != nil {
					failures[i] = e
					return
				}
				var a *snapshot.Attempt
				switch verb {
				case "admit":
					a = workflow.Attempt()
				case "drain":
					e = workflow.Drain()
					a = workflow.Attempt()
				case "cancel":
					e = workflow.Cancel()
					a = workflow.Attempt()
				case "answer":
					e = workflow.Answer(values["--question"], values["--answer"], wire.Count(values["--revision"]))
					a = workflow.Attempt()
				default:
					if verb == "resume" || verb == "retry" {
						if e = workflow.Resume(verb == "retry"); e != nil {
							failures[i] = e
							return
						}
					}
					a, e = workflow.RunRole(ctx, role, values["--grant"])
				}
				if e != nil {
					failures[i] = e
					return
				}
				b, e := a.Encode()
				if e != nil {
					failures[i] = e
					return
				}
				items[i], e = wire.Parse(b)
				failures[i] = e
			}(index)
		}
		group.Wait()

		progress := false
		for i, e := range failures {
			if e != nil {
				if errors.Is(e, store.ErrProgramIdle) {
					continue
				}
				r := laneFailureResult(cmd, e, failures)
				r.Items = append(allItems, items[:i]...)
				return r
			}
			if items[i].Kind != wire.KindNull {
				allItems = append(allItems, items[i])
				progress = true
			}
		}
		if !progress || verb != "run" || values["--ticket"] != "" {
			return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Codes: []string{}, Items: allItems}
		}
	}
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Codes: []string{}, Items: allItems, Warnings: []string{"foreground batch bound reached; pending work retained"}}
}

// configHost reports whether a --host value names the config's supervised
// host (CAL-V0-074); a config without host is Codex.
func configHost(flag, host string) bool {
	if host == "" {
		host = supervisor.HostCodex
	}
	return (flag == supervisor.HostCodex || flag == supervisor.HostClaudeCode || flag == supervisor.HostOpenCode) && flag == host
}

// laneFailureResult reports the first failed lane of a batch. A repeat runs
// every lane again, so the command is not retryable when any other failed lane
// is not, by its mark or its code, even if the reported one is (CAL-V0-078).
// Successful and idle lanes do not count.
func laneFailureResult(cmd []string, first error, failures []error) *wire.Result {
	r := errorResult(cmd, first)
	for _, e := range failures {
		if e == nil || errors.Is(e, store.ErrProgramIdle) {
			continue
		}
		if wire.RetryForbidden(e) || !wire.RetryOf(wire.CodeOf(e)).Retryable {
			r.NotRetryable = true
		}
	}
	return r
}
