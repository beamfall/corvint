package cli

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/service"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// serviceVerbs are the `service` verbs and their flags (SERVICE500-001):
// valued flags map to true, bare flags to false.
var serviceVerbs = map[string]map[string]bool{
	"install":   {"--program": true, "--config": true, "--request-id": true, "--replace": false},
	"status":    {"--program": true},
	"uninstall": {"--program": true, "--request-id": true},
	"stop":      {"--program": true, "--request-id": true, "--drain": false},
	"resume":    {"--program": true, "--request-id": true},
	"run":       {"--program": true, "--manifest": true},
}

// serviceHost is the live per-user host: HOME, the real uid and the fixed
// launchctl/systemctl manager. Tests substitute HOME only for pure reads.
func serviceHost() service.Host {
	return service.Host{UID: os.Getuid(), Home: os.Getenv("HOME"), GOOS: runtime.GOOS, Manager: service.ExecManager{}, QueueID: serviceQueueID}
}

func serviceQueueID(workRoot string) (string, error) {
	repo, err := intent.Resolve(workRoot)
	if err != nil {
		return "", err
	}
	st, err := intent.Load(repo.PrimaryWorktree)
	if err != nil {
		return "", err
	}
	if st.Queue == nil {
		return "", wire.Errorf(wire.CodeUninitialized, "/store", "the canonical store has no queue")
	}
	return st.Queue.QueueID.Raw, nil
}

// serviceCommand routes `service install|status|uninstall|stop|resume|run`.
func serviceCommand(env Env, args []string) *wire.Result {
	if len(args) == 0 || serviceVerbs[args[0]] == nil {
		return usage([]string{"service"}, "service needs a verb: install, status, uninstall, stop, resume or run")
	}
	verb := args[0]
	cmd := []string{"service", verb}
	spec := serviceVerbs[verb]
	values := map[string]string{}
	for i := 1; i < len(args); i++ {
		f := args[i]
		valued, known := spec[f]
		if _, dup := values[f]; dup || !known || (valued && i+1 >= len(args)) {
			return usage(cmd, "unknown, repeated or incomplete service flag "+f)
		}
		values[f] = ""
		if valued {
			values[f] = args[i+1]
			i++
		}
	}
	required := []string{}
	for f, valued := range spec {
		if valued {
			required = append(required, f)
		}
	}
	sort.Strings(required)
	for _, f := range required {
		if values[f] == "" {
			return usage(cmd, "service "+verb+" requires "+strings.Join(required, ", "))
		}
	}
	program := values["--program"]
	if !dispatch.ValidName(program) {
		return usage(cmd, "program name must match [a-z][a-z0-9-]{0,23}")
	}
	_, flag := values["--replace"]
	_, drain := values["--drain"]
	h := serviceHost()
	var (
		o   *wire.Object
		err error
	)
	switch verb {
	case "install":
		config := values["--config"]
		if !filepath.IsAbs(config) {
			config = filepath.Join(env.Cwd, config)
		}
		o, err = h.Install(service.InstallRequest{Program: program, Config: filepath.Clean(config), RequestID: values["--request-id"], Replace: flag})
	case "status":
		o, err = h.Status(program)
	case "uninstall":
		o, err = h.Uninstall(program, values["--request-id"])
	case "stop":
		o, err = h.Stop(program, values["--request-id"], drain)
	case "resume":
		o, err = h.Resume(program, values["--request-id"])
	case "run":
		return serviceRun(env, cmd, h, program, values["--manifest"])
	}
	return serviceResult(cmd, o, err)
}

// serviceResult maps a lifecycle answer. Every ROLLED_BACK install answers
// RESTORED: the first attempt and the call completing a held rollback
// return it as the error code, and a replay of the finished record is
// mapped here because it did not take effect.
func serviceResult(cmd []string, o *wire.Object, err error) *wire.Result {
	if err != nil {
		return errorResult(cmd, err)
	}
	res := &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Codes: []string{}, Items: []wire.Value{wire.ObjectValue(o)}}
	if phase, ok := o.Get("phase"); ok && phase.Kind == wire.KindString && phase.Str == "ROLLED_BACK" {
		res.Outcome, res.Codes = wire.OutcomeError, []string{wire.CodeRestored}
	}
	return res
}

// serviceRun is the manager-started foreground main (SERVICE500-003/004/005). It
// runs the existing dispatcher in-process against the installed manifest.
func serviceRun(env Env, cmd []string, h service.Host, program, manifest string) *wire.Result {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return errorResult(cmd, wire.Errorf(wire.CodeCapabilityUnavailable, "/executable", "executable path is not observable: %v", err))
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()
	open := func(p string, c *dispatch.Config, control dispatch.LaunchControl) (service.Controller, error) {
		queueEnv := env
		queueEnv.Cwd = c.WorkRoot
		d, err := dispatch.OpenControlled(p, c, dispatchQueue{env: queueEnv}, env.Stderr, control)
		if err != nil {
			return nil, err
		}
		return d, nil
	}
	if err := service.Run(ctx, service.RunOptions{Host: h, Program: program, Manifest: manifest, Executable: exe, Open: open, Out: env.Stderr}); err != nil {
		return errorResult(cmd, err)
	}
	o := wire.NewObject().Set("profile", wire.String("taskman-user-service-run/0")).Set("program", wire.String(program)).Set("interrupted", wire.Bool(ctx.Err() != nil))
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Codes: []string{}, Items: []wire.Value{wire.ObjectValue(o)}}
}
