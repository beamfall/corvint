package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// Service and hook bounds (UPD-V0-011 to UPD-V0-014, UPD-V0-015). The run
// budget grows by at most the drain bound, two start and two stop waits (each
// a start bound) plus six service steps, so a run stays under the 30-minute
// incomplete-transaction age (UPD-V0-007). A restart of the old build after a
// failure runs under its own cleanup context of one start wait and two steps.
const (
	defaultHookTimeout  = 2 * time.Minute
	maxHookTimeout      = 10 * time.Minute
	defaultHookAttempts = 3
	maxHookAttempts     = 5
	maxHooks            = 8
	maxHookCommandBytes = 4096
	defaultDrainTimeout = 10 * time.Minute
	maxDrainTimeout     = 15 * time.Minute
	defaultStartTimeout = 30 * time.Second
	maxStartTimeout     = 2 * time.Minute
	serviceStepTimeout  = 30 * time.Second
	hookOutputTail      = 2048
)

// ServiceOutcome is the restart of the operator's service around a switch.
// Mode is ADOPT (equal Tasks formats: stop, switch, start; the restarted
// service adopts live attempts), DRAIN (formats differ or are unknown: stop and
// wait for zero live attempts first) or RESTART (Core: stop, switch, start).
// State is STOPPED only while the switch is in progress; it ends as RESUMED,
// NOT_RUNNING_BEFORE, RESUMED_OLD (the switch did not happen
// and the service runs the old build again), ROLLED_BACK or FAILED.
type ServiceOutcome struct {
	Program      string `json:"program"`
	Mode         string `json:"mode"`
	WasRunning   bool   `json:"wasRunning"`
	LiveAttempts *int   `json:"liveAttempts,omitempty"`
	State        string `json:"state"`
	Reason       string `json:"reason,omitempty"`
}

// HookOutcome is one post-switch hook: State is SUCCEEDED or FAILED, Attempts
// the runs made, ExitStatus the last run's status (-1 when it did not exit,
// for example on timeout), Output the last bytes of its combined output.
type HookOutcome struct {
	Command    string `json:"command"`
	Attempts   int    `json:"attempts"`
	ExitStatus int    `json:"exitStatus"`
	State      string `json:"state"`
	Output     string `json:"output,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// validateService checks the service and hook options before any network or
// state work; they are accepted only by apply.
func validateService(command string, c *Config) error {
	if c.ServiceProgram == "" && len(c.PostSwitch) == 0 {
		return nil
	}
	if command != "apply" {
		return errors.New("--restart-service and --post-switch apply only to apply")
	}
	if c.ServiceProgram != "" {
		if !filepath.IsAbs(c.ServiceProgram) || filepath.Clean(c.ServiceProgram) != c.ServiceProgram {
			return errors.New("--restart-service must be a clean absolute path")
		}
		info, err := os.Stat(c.ServiceProgram)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			return errors.New("--restart-service must name an executable regular file")
		}
	}
	if len(c.PostSwitch) > maxHooks {
		return fmt.Errorf("at most %d --post-switch hooks", maxHooks)
	}
	for _, h := range c.PostSwitch {
		if strings.TrimSpace(h) == "" || len(h) > maxHookCommandBytes || strings.ContainsRune(h, 0) {
			return errors.New("invalid --post-switch command")
		}
	}
	if c.HookTimeout == 0 {
		c.HookTimeout = defaultHookTimeout
	}
	if c.HookAttempts == 0 {
		c.HookAttempts = defaultHookAttempts
	}
	if c.DrainTimeout == 0 {
		c.DrainTimeout = defaultDrainTimeout
	}
	if c.StartTimeout == 0 {
		c.StartTimeout = defaultStartTimeout
	}
	if c.HookTimeout < 0 || c.HookTimeout > maxHookTimeout || c.HookAttempts < 1 || c.HookAttempts > maxHookAttempts ||
		c.DrainTimeout < 0 || c.DrainTimeout > maxDrainTimeout || c.StartTimeout < 0 || c.StartTimeout > maxStartTimeout {
		return errors.New("hook or service bound out of range")
	}
	return nil
}

// serviceBudget is the extra run time a configured service may use.
func serviceBudget(c Config) time.Duration {
	if c.ServiceProgram == "" {
		return 0
	}
	return c.DrainTimeout + 4*c.StartTimeout + 6*serviceStepTimeout
}

// service drives the operator's service-control program, invoked as
// `PROGRAM status|stop|start|live-attempts` with the component and the
// managed binary in its environment.
type service struct {
	program, component, binary string
	poll                       time.Duration
}

func (s service) step(ctx context.Context, verb string) procgroup.Observation {
	env := append(os.Environ(), "CORVINT_UPDATE_COMPONENT="+s.component, "CORVINT_UPDATE_BINARY="+s.binary)
	return procgroup.Run(ctx, procgroup.Spec{Argv: []string{s.program, verb}, Dir: filepath.Dir(s.binary), Env: env, Timeout: serviceStepTimeout, ShutdownTimeout: 2 * time.Second, OutputLimit: 64 << 10, ObserveDescendants: true})
}

func (s service) ok(ctx context.Context, verb string) error {
	o := s.step(ctx, verb)
	if o.Err != nil || o.ExitStatus != 0 || !o.OwnedProcessGroupCleanup {
		return fmt.Errorf("service %s failed: exit %d", verb, o.ExitStatus)
	}
	return nil
}

// Service status values: `status` exits 0 for a running service and 3 (the
// LSB "not running" status) for a stopped one; anything else is unknown.
const (
	statusRunning = "RUNNING"
	statusStopped = "STOPPED"
	statusUnknown = "UNKNOWN"
)

func (s service) status(ctx context.Context) string {
	o := s.step(ctx, "status")
	switch {
	case o.Err != nil || !o.ExitObserved || !o.OwnedProcessGroupCleanup:
		return statusUnknown
	case o.ExitStatus == 0:
		return statusRunning
	case o.ExitStatus == 3:
		return statusStopped
	}
	return statusUnknown
}

func (s service) running(ctx context.Context) bool {
	return s.status(ctx) == statusRunning
}

func (s service) liveAttempts(ctx context.Context) (int, error) {
	o := s.step(ctx, "live-attempts")
	if o.Err != nil || o.ExitStatus != 0 || !o.OwnedProcessGroupCleanup {
		return 0, fmt.Errorf("service live-attempts failed: exit %d", o.ExitStatus)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(o.Stdout)))
	if err != nil || n < 0 {
		return 0, errors.New("service live-attempts printed no count")
	}
	return n, nil
}

// start starts the service and waits until status reports it running.
func (s service) start(ctx context.Context, bound time.Duration) error {
	if err := s.ok(ctx, "start"); err != nil {
		return err
	}
	deadline := time.Now().Add(bound)
	for {
		if s.running(ctx) {
			return nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return errors.New("service did not report running")
		}
		time.Sleep(s.poll)
	}
}

// stopped polls status after a stop verb exited 0 until it reports the
// service stopped, within bound (UPD-V0-015). A status still running at the
// bound, or one that is unknown, is an error: the stop is not observed.
func (s service) stopped(ctx context.Context, bound time.Duration) error {
	deadline := time.Now().Add(bound)
	for {
		switch s.status(ctx) {
		case statusStopped:
			return nil
		case statusUnknown:
			return errors.New("service status after stop is neither running nor stopped")
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return errors.New("service still reports running after stop")
		}
		time.Sleep(s.poll)
	}
}

// stop runs the stop verb and waits until status observes the service
// stopped (UPD-V0-015).
func (s service) stop(ctx context.Context, bound time.Duration) error {
	if err := s.ok(ctx, "stop"); err != nil {
		return err
	}
	return s.stopped(ctx, bound)
}

// restartOld starts the old build again after a failure before the switch
// stands. It runs under a fresh context bounded by one start wait and two
// steps, detached from the operation's cancellation, because a signal or the
// run deadline must not leave a previously running service stopped
// (UPD-V0-015). The outcome becomes RESUMED_OLD, or FAILED with the restart
// error appended to its reason.
func (s service) restartOld(ctx context.Context, bound time.Duration, out *ServiceOutcome) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), bound+2*serviceStepTimeout)
	defer cancel()
	if err := s.start(cleanup, bound); err != nil {
		out.State = "FAILED"
		if out.Reason != "" {
			out.Reason += "; "
		}
		out.Reason += "old build restart: " + err.Error()
		return
	}
	out.State = "RESUMED_OLD"
}

// serviceMode applies the Tasks compatibility rule (CAL-V0-130, CAL-V0-131):
// equal, non-empty `formats` sets adopt live attempts; anything else drains.
func serviceMode(component string, installed, candidate []string) string {
	if component == "core" {
		return "RESTART"
	}
	if len(installed) > 0 && slices.Equal(installed, candidate) {
		return "ADOPT"
	}
	return "DRAIN"
}

// tasksFormats reads the sorted `formats` a Tasks build reports in its
// version command result; any failure yields none, which drains.
func tasksFormats(ctx context.Context, binary, home string) []string {
	o := runProcess(ctx, binary, home, "version")
	if o.Err != nil || o.ExitStatus != 0 || !o.OwnedProcessGroupCleanup {
		return nil
	}
	var env struct {
		Profile string
		Items   []struct{ Formats []string }
	}
	if json.Unmarshal(o.Stdout, &env) != nil || env.Profile != "taskman-command-result/0" || len(env.Items) != 1 {
		return nil
	}
	formats := slices.Clone(env.Items[0].Formats)
	slices.Sort(formats)
	return formats
}

// quiesce records whether the service runs and stops it for the switch. A
// status that is neither running nor stopped refuses before any stop. In DRAIN
// mode it then waits for zero live attempts whether or not the service ran,
// because attempts can outlive a stopped dispatcher; when they do not reach
// zero in the bound it starts the old build again if it ran and refuses,
// leaving the destination unchanged.
func (e engine) quiesce(ctx context.Context, s service, mode string) (*ServiceOutcome, error) {
	out := &ServiceOutcome{Program: s.program, Mode: mode, State: "NOT_RUNNING_BEFORE"}
	switch s.status(ctx) {
	case statusUnknown:
		out.State, out.Reason = "FAILED", "service status is neither running (exit 0) nor stopped (exit 3); destination unchanged"
		return out, errors.New("service status unknown")
	case statusRunning:
		out.WasRunning = true
	}
	if out.WasRunning {
		if n, err := s.liveAttempts(ctx); err == nil {
			out.LiveAttempts = &n
		} else if mode == "DRAIN" {
			out.State, out.Reason = "RESUMED_OLD", err.Error()
			return out, err
		}
		if err := s.stop(ctx, e.config.StartTimeout); err != nil {
			out.Reason = err.Error() + "; destination unchanged"
			s.restartOld(ctx, e.config.StartTimeout, out)
			return out, err
		}
		out.State = "STOPPED"
	}
	if mode != "DRAIN" {
		return out, nil
	}
	deadline := time.Now().Add(e.config.DrainTimeout)
	for {
		n, err := s.liveAttempts(ctx)
		if err == nil {
			out.LiveAttempts = &n
		}
		if err == nil && n == 0 {
			return out, nil
		}
		if err != nil || time.Now().After(deadline) || ctx.Err() != nil {
			reason := "live attempts did not drain before the bound; destination unchanged"
			if err != nil {
				reason = err.Error()
			}
			out.Reason = reason
			if out.WasRunning {
				s.restartOld(ctx, e.config.StartTimeout, out)
			}
			return out, errors.New("service drain: " + reason)
		}
		time.Sleep(s.poll)
	}
}

// rollbackPermitted stops a service whose new build did not report running
// and reports whether the previous build may be restored under it. Rollback
// needs a service that status observes stopped within bound (UPD-V0-015), and a DRAIN-mode Tasks switch is never rolled back
// automatically, because the candidate may already have written formats the
// previous build cannot read (CAL-V0-130). On refusal out is FAILED and the new
// build stays installed.
func rollbackPermitted(ctx context.Context, s service, out *ServiceOutcome, startErr error, bound time.Duration) error {
	out.Reason = "new build: " + startErr.Error()
	if err := s.stop(ctx, bound); err != nil {
		out.State, out.Reason = "FAILED", out.Reason+"; stop: "+err.Error()+"; rollback not attempted"
		return errors.New("service did not start on the new build and could not be stopped; the new build stays installed")
	}
	if out.Mode == "DRAIN" {
		out.State, out.Reason = "FAILED", out.Reason+"; automatic rollback refused: Tasks formats differ or are unknown"
		return errors.New("service did not start on the new build; automatic rollback refused across differing or unknown Tasks formats; the new build stays installed and the service is stopped")
	}
	return nil
}

// runHooks runs each post-switch hook through /bin/sh with bounded attempts
// and per-attempt timeouts, after a switch that stands. A failed hook never
// reverses the switch (UPD-V0-012).
func (e engine) runHooks(ctx context.Context, binary string, build int) ([]HookOutcome, bool) {
	c := e.config
	dir := c.WorkDir
	if dir == "" {
		dir = c.BinDir
	}
	env := append(os.Environ(), "CORVINT_UPDATE_COMPONENT="+c.Component, "CORVINT_UPDATE_BINARY="+binary, "CORVINT_UPDATE_BUILD="+strconv.Itoa(build))
	failed := false
	var outcomes []HookOutcome
	for _, command := range c.PostSwitch {
		h := HookOutcome{Command: command, State: "FAILED"}
		for h.Attempts < c.HookAttempts && ctx.Err() == nil {
			if h.Attempts > 0 {
				time.Sleep(e.poll())
			}
			h.Attempts++
			o := procgroup.Run(ctx, procgroup.Spec{Argv: []string{"/bin/sh", "-c", command}, Dir: dir, Env: env, Timeout: c.HookTimeout, ShutdownTimeout: 2 * time.Second, OutputLimit: 64 << 10, ObserveDescendants: true})
			combined := append(append([]byte{}, o.Stdout...), o.Stderr...)
			if len(combined) > hookOutputTail {
				combined = combined[len(combined)-hookOutputTail:]
			}
			h.Output, h.ExitStatus, h.Reason = string(combined), o.ExitStatus, ""
			if !o.ExitObserved {
				h.ExitStatus = -1
			}
			switch {
			case o.TimedOut:
				h.Reason = "timed out after " + c.HookTimeout.String()
			case o.Err != nil:
				h.Reason = o.Err.Error()
			case !o.OwnedProcessGroupCleanup:
				h.Reason = "descendant cleanup not observed"
			case o.ExitStatus == 0:
				h.State = "SUCCEEDED"
			}
			if h.State == "SUCCEEDED" {
				break
			}
		}
		if h.State != "SUCCEEDED" {
			failed = true
		}
		outcomes = append(outcomes, h)
	}
	return outcomes, failed
}

func (e engine) poll() time.Duration {
	if e.servicePoll > 0 {
		return e.servicePoll
	}
	return time.Second
}
