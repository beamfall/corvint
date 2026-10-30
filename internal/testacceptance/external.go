package testacceptance

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// ExternalHealthSchema names the one JSON document the health hook reads on stdin (NEA-V0-015).
const ExternalHealthSchema = "corvint-new-e2e-external-health/0"

// externalHookTimeout bounds one health hook invocation; externalHookBudget is what the parent
// adds to a worker's timeout for the two hooks of a run.
const externalHookTimeout = 10 * time.Second
const externalHookBudget = 20

var environmentKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

// reservedEnvironmentKeys and reservedEnvironmentPrefixes are the keys the worker fixes and the
// keys a shell, runtime, loader or network stack reads as configuration: startup files, option
// strings, certificate stores and proxies would change executed code or reach a non-loopback host
// without changing any pinned byte (NEA-V0-013).
var reservedEnvironmentKeys = []string{"PATH", "LANG", "TMPDIR", "HOME", "SHELL", "ENV", "BASH_ENV", "SHELLOPTS", "BASHOPTS", "IFS", "PS4", "PROMPT_COMMAND", "CDPATH", "GLOBIGNORE", "JAVA_TOOL_OPTIONS", "GCONV_PATH", "LOCPATH", "NLSPATH", "HOSTALIASES", "RES_OPTIONS", "LOCALDOMAIN", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "CURL_CA_BUNDLE"}
var reservedEnvironmentPrefixes = []string{"CORVINT_", "NODE_", "NPM_", "YARN_", "PNPM_", "COREPACK_", "DENO_", "BUN_", "LD_", "DYLD_", "LC_", "PLAYWRIGHT_", "PW_", "GIT_", "SSL_", "OPENSSL_", "ELECTRON_", "XDG_", "PYTHON", "PERL", "RUBY"}

// commands is the request's companion-run commands: the runner, and the server unless the
// caller owns it (NEA-V0-012).
func commands(r Request) []Command {
	if r.ExternalApp != nil {
		return []Command{r.Runner}
	}
	return []Command{r.Runner, r.Server}
}

// validateExternal admits the authenticated-disposable profile (NEA-V0-012..014).
func validateExternal(r Request) error {
	x := r.ExternalApp
	if x == nil {
		return nil
	}
	if x.Profile != behaviorfalsify.AuthenticatedDisposable {
		return errors.New("external-app-profile-invalid")
	}
	if len(r.Server.Argv) != 0 || r.Server.ExecutableSHA256 != "" || r.Server.EntrypointSHA256 != "" {
		return errors.New("external-server-command-present")
	}
	h := x.Health
	if len(h.Argv) != 1 || h.EntrypointSHA256 != "" || !hashPattern.MatchString(h.ExecutableSHA256) || regular(h.Argv[0]) != nil || len(h.Argv[0]) > 2048 || strings.ContainsAny(h.Argv[0], "\x00\n\r") {
		return errors.New("external-health-invalid")
	}
	if b, e := os.ReadFile(h.Argv[0]); e != nil || Hash(b) != h.ExecutableSHA256 {
		return errors.New("external-health-invalid")
	}
	if secretscreen.MatchArgv(r.Runner.Argv) || secretscreen.MatchArgv(h.Argv) {
		return errors.New("argv-secret-shaped")
	}
	if e := validateEnvironment(x.Environment); e != nil {
		return e
	}
	for _, f := range r.Inputs {
		b, e := os.ReadFile(f.Path)
		if e != nil || secretscreen.MatchString(string(b)) {
			return errors.New("input-secret-shaped")
		}
	}
	_, e := authFileBytes(r)
	return e
}

func validateEnvironment(values []EnvironmentValue) error {
	if len(values) > 16 {
		return errors.New("external-environment-invalid")
	}
	seen := map[string]bool{}
	for _, v := range values {
		if !environmentKeyPattern.MatchString(v.Key) || seen[v.Key] {
			return errors.New("external-environment-invalid")
		}
		seen[v.Key] = true
		if slices.Contains(reservedEnvironmentKeys, v.Key) || slices.ContainsFunc(reservedEnvironmentPrefixes, func(p string) bool { return strings.HasPrefix(v.Key, p) }) {
			return errors.New("external-environment-key-reserved")
		}
		if behaviorfalsify.CredentialKey(v.Key) {
			return errors.New("external-environment-key-credential")
		}
		if len(v.Value) > 1024 || strings.ContainsAny(v.Value, "\x00\r\n") || secretscreen.MatchString(v.Key+"="+v.Value) || secretscreen.MatchString(v.Value) {
			return errors.New("external-environment-value-secret")
		}
	}
	return nil
}

// authFileBytes rereads every pinned storage-state file. Files must lie outside both
// repositories, so none can also be a tracked input (NEA-V0-014).
func authFileBytes(r Request) ([][]byte, error) {
	files := r.ExternalApp.AuthFiles
	if len(files) < 1 || len(files) > 4 {
		return nil, errors.New("auth-file-invalid")
	}
	protected := []string{r.Product.Root, r.TestRepository.Root}
	seen := map[string]bool{}
	var out [][]byte
	for _, f := range files {
		if seen[f.Path] {
			return nil, errors.New("auth-file-invalid")
		}
		seen[f.Path] = true
		b, e := behaviorfalsify.ReadStorageState(f.Path, f.SHA256, protected)
		if errors.Is(e, behaviorfalsify.ErrStorageStateDrift) {
			return nil, errors.New("auth-file-drift")
		}
		if e != nil {
			return nil, errors.New("auth-file-invalid")
		}
		out = append(out, b)
	}
	return out, nil
}

// externalEnvironment is the declared pairs as KEY=VALUE entries.
func externalEnvironment(r Request) []string {
	if r.ExternalApp == nil {
		return nil
	}
	out := make([]string, 0, len(r.ExternalApp.Environment))
	for _, v := range r.ExternalApp.Environment {
		out = append(out, v.Key+"="+v.Value)
	}
	return out
}

// externalHook runs the pinned health hook once under the worker's declared environment. Its
// stdout and stderr are discarded unread (NEA-V0-015).
func externalHook(ctx context.Context, r Request, phase, kind string, ordinal int) HookResult {
	h := HookResult{Phase: phase, Status: "failed"}
	hook, remove, e := stageHook(r)
	if e != nil {
		return h
	}
	defer remove()
	input, _ := json.Marshal(struct {
		Schema  string `json:"schema"`
		Phase   string `json:"phase"`
		Kind    string `json:"kind"`
		Ordinal int    `json:"ordinal"`
	}{ExternalHealthSchema, phase, kind, ordinal})
	spec := procgroup.Spec{Argv: []string{hook}, Dir: r.TestRepository.Root, Env: os.Environ(), Stdin: input, InputLimit: 4096, OutputLimit: 64 << 10, Timeout: externalHookTimeout, ObserveDescendants: true}
	o := procgroup.Run(ctx, spec)
	// A fork made concurrently elsewhere in the process can briefly hold the staged copy's write
	// descriptor; nothing started, so a bounded retry is safe.
	for _, delay := range []time.Duration{5 * time.Millisecond, 20 * time.Millisecond, 50 * time.Millisecond, 100 * time.Millisecond} {
		if o.Started || !errors.Is(o.Err, syscall.ETXTBSY) || ctx.Err() != nil {
			break
		}
		time.Sleep(delay)
		o = procgroup.Run(ctx, spec)
	}
	h.Cleanup = cleanup(o)
	if o.Err == nil && o.Started && o.WaitCompleted && o.ExitStatus == 0 && !o.TimedOut && !o.Cancelled && !o.OutputOverflow && o.OwnedProcessGroupCleanup {
		h.Status = "passed"
	}
	return h
}

// stageHook reads the health hook once, checks those bytes against the pinned digest and runs
// them from a private copy, so a hook replaced after validation never executes (NEA-V0-015).
func stageHook(r Request) (string, func(), error) {
	argv0 := r.ExternalApp.Health.Argv[0]
	b, e := os.ReadFile(argv0)
	if e != nil || Hash(b) != r.ExternalApp.Health.ExecutableSHA256 {
		return "", nil, errors.New("external-health-invalid")
	}
	dir, e := os.MkdirTemp("", "corvint-new-test-hook-")
	if e != nil {
		return "", nil, e
	}
	path := filepath.Join(dir, filepath.Base(argv0))
	if e := os.WriteFile(path, b, 0o700); e != nil {
		os.RemoveAll(dir)
		return "", nil, e
	}
	return path, func() { os.RemoveAll(dir) }, nil
}

// externalReport binds the report to the profile: paths and digests only (NEA-V0-016).
func externalReport(r Request) *ExternalReport {
	x := r.ExternalApp
	env, _ := json.Marshal(x.Environment)
	return &ExternalReport{Profile: x.Profile, ServerOwnership: "caller", ReadyURL: r.ReadyURL, HealthSHA256: x.Health.ExecutableSHA256, EnvironmentSHA256: Hash(env), AuthFiles: slices.Clone(x.AuthFiles)}
}

// refuseAuthMaterial rereads the pinned storage-state files and refuses a report that carries
// any of their values (NEA-V0-014). A file that no longer matches its pin cannot be searched,
// so publication fails closed.
func refuseAuthMaterial(r Request, report Report) error {
	if r.ExternalApp == nil {
		return nil
	}
	files, e := authFileBytes(r)
	if e != nil {
		return e
	}
	serialized, e := json.Marshal(report)
	if e != nil || behaviorfalsify.ContainsAuthenticationMaterial(serialized, behaviorfalsify.AuthenticationMaterial(files)) {
		return errors.New("auth-material-in-report")
	}
	return nil
}
