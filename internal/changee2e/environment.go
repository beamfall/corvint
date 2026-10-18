package changee2e

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Environment is the optional `environment` member of `e2e-blocks.json` (CCE-V0-021): how prove
// serves each side's application tree. It holds argv and variable names only, never values.
type Environment struct {
	Serve   []string `json:"serve"`
	BaseURL string   `json:"base_url"`
	Ready   string   `json:"ready"`
	Env     []string `json:"env"`
}

// Environment states recorded in the prove document.
const environmentStatic = "static"

const (
	readyTimeout      = 120 * time.Second
	readyPollInterval = 250 * time.Millisecond
	maxServeArgs      = 64
	maxEnvNames       = 32
)

var envName = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,63}$`)

// validate refuses `e2e-blocks-invalid` unless serve is a non-empty argv, base_url is an http URL
// on a loopback host with an explicit port and no path beyond `/`, ready is an absolute path, and
// env holds distinct variable names.
func (e *Environment) validate() error {
	bad := func(format string, args ...any) error {
		return refuse("e2e-blocks-invalid", "%s: environment "+format, append([]any{DeclarationsPath}, args...)...)
	}
	if len(e.Serve) == 0 || len(e.Serve) > maxServeArgs {
		return bad("serve must be an argv of 1..%d elements", maxServeArgs)
	}
	for i, a := range e.Serve {
		if a == "" || len(a) > 4096 {
			return bad("serve[%d] is empty or too long", i)
		}
		if secretscreen.MatchString(a) {
			return bad("serve[%d] is secret-shaped", i)
		}
	}
	u, err := url.Parse(e.BaseURL)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return bad("base_url must be an http origin")
	}
	if !loopbackHost(u.Hostname()) {
		return bad("base_url host must be loopback")
	}
	if p, err := strconv.Atoi(u.Port()); err != nil || p < 1 || p > 65535 {
		return bad("base_url needs an explicit port")
	}
	if r, err := url.Parse(e.Ready); err != nil || len(e.Ready) == 0 || e.Ready[0] != '/' || r.Host != "" || r.Scheme != "" {
		return bad("ready must be an absolute path")
	}
	seen := map[string]bool{}
	if len(e.Env) > maxEnvNames {
		return bad("env holds more than %d names", maxEnvNames)
	}
	for _, n := range e.Env {
		if !envName.MatchString(n) || seen[n] || n == envBaseURL {
			return bad("env name %q", n)
		}
		seen[n] = true
	}
	return nil
}

// readEnvironment returns the environment declared at rev, or nil when the blocks file or its
// environment member is absent.
func readEnvironment(ctx context.Context, root, rev string) (*Environment, error) {
	data, _, ok, err := blobAt(ctx, root, rev, DeclarationsPath)
	if err != nil || !ok {
		return nil, err
	}
	d, err := decodeDeclarations(data)
	return d.Environment, err
}

func loopbackHost(h string) bool {
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// origin is base_url with a trailing slash; readyURL is the origin joined with ready.
func (e *Environment) origin() string {
	u, _ := url.Parse(e.BaseURL)
	return "http://" + u.Host + "/"
}

func (e *Environment) readyURL() string {
	u, _ := url.Parse(e.BaseURL)
	return "http://" + u.Host + e.Ready
}

// digest is the environment's recorded identity in the prove document.
func (e *Environment) digest() string {
	if e == nil {
		return environmentStatic
	}
	return digestCanonical(e)
}

// envError is a failed environment step; prove reads it as INCONCLUSIVE, never a verdict.
type envError struct{ code string }

func (e *envError) Error() string { return "environment-unavailable:" + e.code }

// serveTree runs serve inside tree without a shell, with only PATH, HOME and the declared names
// passed through, and polls ready until it answers or readyTimeout passes. A port already answering
// before serve starts is refused, so a stray server never stands in for the side under test. The
// returned stop kills the serve process group.
func serveTree(ctx context.Context, env *Environment, tree string) (stop func(), err error) {
	u, _ := url.Parse(env.BaseURL)
	if c, err := net.DialTimeout("tcp", u.Host, time.Second); err == nil {
		_ = c.Close()
		return nil, &envError{"port-in-use"}
	}
	bin, err := exec.LookPath(env.Serve[0])
	if err != nil {
		return nil, &envError{"serve-tool-missing"}
	}
	cmd := exec.Command(bin, env.Serve[1:]...)
	cmd.Dir = tree
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	for _, n := range env.Env {
		if v, ok := os.LookupEnv(n); ok {
			cmd.Env = append(cmd.Env, n+"="+v)
		}
	}
	configureServe(cmd)
	if err := cmd.Start(); err != nil {
		return nil, &envError{"serve-start-failed"}
	}
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	stop = func() {
		killServe(cmd)
		select {
		case <-exited:
		case <-time.After(10 * time.Second):
		}
	}
	if err := pollReady(ctx, env.readyURL(), exited); err != nil {
		stop()
		return nil, err
	}
	return stop, nil
}

func pollReady(ctx context.Context, target string, exited <-chan struct{}) error {
	client := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	deadline := time.NewTimer(readyTimeout)
	defer deadline.Stop()
	tick := time.NewTicker(readyPollInterval)
	defer tick.Stop()
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if resp, err := client.Do(req); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return &envError{"ready-cancelled"}
		case <-exited:
			return &envError{"serve-exited"}
		case <-deadline.C:
			return &envError{"ready-timeout"}
		case <-tick.C:
		}
	}
}

func asEnvError(err error) (string, bool) {
	var e *envError
	if errors.As(err, &e) {
		return e.Error(), true
	}
	return "", false
}
