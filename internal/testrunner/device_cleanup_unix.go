//go:build darwin || linux

package testrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

// deviceCleanupStepTimeout bounds each device-cleanup step, and
// deviceCleanupWaitDelay bounds a step's pipe drain after its group is
// killed, so a whole cleanup is bounded by steps * (timeout + delay).
var (
	deviceCleanupStepTimeout = 10 * time.Second
	deviceCleanupWaitDelay   = 2 * time.Second
)

const maxCleanupSteps = 4

var webDriverSessionID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// checkDeviceCleanup admits a declared device cleanup before anything runs
// (TRE-V0-087): each step is either a declared-tool argv or a WebDriver
// session deletion on a canonical loopback endpoint.
func checkDeviceCleanup(r Request, steps []CleanupStep) error {
	if len(steps) > maxCleanupSteps {
		return fmt.Errorf("device cleanup step bound")
	}
	names := map[string]bool{}
	for _, s := range steps {
		if s.Name == "" || len(s.Name) > 64 || names[s.Name] {
			return fmt.Errorf("invalid or duplicate device cleanup step name")
		}
		names[s.Name] = true
		switch {
		case len(s.Argv) > 0 && s.WebDriver == "" && s.SessionFile == "":
			if _, ok := tool(r, s.Tool); !ok {
				return fmt.Errorf("undeclared device cleanup tool")
			}
			if len(s.Argv) > 32 || len(s.Expect) > 128 || strings.ContainsAny(s.Expect, "\x00\r\n") {
				return fmt.Errorf("device cleanup argv bound")
			}
			for _, a := range s.Argv {
				if len(a) > 4096 || strings.ContainsRune(a, 0) {
					return fmt.Errorf("invalid device cleanup argv")
				}
			}
		case len(s.Argv) == 0 && s.Tool == "" && s.Expect == "" && s.WebDriver != "":
			if !canonicalLoopbackEndpoint(s.WebDriver) {
				return fmt.Errorf("device cleanup WebDriver endpoint must be canonical http://127.0.0.1:PORT/")
			}
			if err := relative(s.SessionFile); err != nil {
				return fmt.Errorf("device cleanup session file: %w", err)
			}
		default:
			return fmt.Errorf("ambiguous device cleanup step")
		}
	}
	return nil
}

func canonicalLoopbackEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	port, err := strconv.Atoi(u.Port())
	return err == nil && port >= 1 && port <= 65535 && endpoint == "http://127.0.0.1:"+strconv.Itoa(port)+"/"
}

// cleanDevice runs every declared step once, each under its own bound, even
// after an earlier step failed, and is clean only when all succeeded. It runs
// after the caller's cancellation by design, so it detaches from it.
func cleanDevice(ctx context.Context, r Request, steps []CleanupStep, reportRoot *os.Root) *DeviceCleanup {
	base := context.WithoutCancel(ctx)
	d := &DeviceCleanup{Outcome: CleanupClean, Steps: make([]CleanupStepResult, 0, len(steps))}
	for _, s := range steps {
		stepCtx, cancel := context.WithTimeout(base, deviceCleanupStepTimeout)
		var res CleanupStepResult
		if s.WebDriver != "" {
			res = deleteWebDriverSession(stepCtx, s, reportRoot)
		} else {
			res = runCleanupTool(stepCtx, r, s)
		}
		cancel()
		res.Name = s.Name
		if res.Status != "ok" {
			d.Outcome = CleanupIncomplete
		}
		d.Steps = append(d.Steps, res)
	}
	return d
}

func runCleanupTool(ctx context.Context, r Request, s CleanupStep) CleanupStepResult {
	res := CleanupStepResult{Status: "failed", ExitCode: -1}
	t, _ := tool(r, s.Tool)
	if err := checkTool(t); err != nil {
		res.Detail = "tool identity: " + err.Error()
		return res
	}
	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()
	cmd := exec.CommandContext(runCtx, t.Executable, s.Argv...)
	cmd.Dir = r.Root
	env := map[string]string{"PATH": declaredPath(r), "HOME": filepath.Join(r.ReportDir, ".home"), "TMPDIR": ExecutionTempDir(r.ReportDir), "LANG": "C.UTF-8", "TZ": "UTC"}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		cmd.Env = append(cmd.Env, k+"="+env[k])
	}
	containPhase(cmd, false, nil)
	cmd.WaitDelay = deviceCleanupWaitDelay
	stdout := &limitedBuffer{cancel: runCancel}
	cmd.Stdout = stdout
	cmd.Stderr = io.Discard
	containment, _ := groupreap.RunContained(cmd)
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	out, _ := stdout.value()
	last := lastLine(out)
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		res.Status, res.Detail = "timed-out", fmt.Sprintf("no result within %s", deviceCleanupStepTimeout)
	case !containment.Complete():
		res.Detail = containmentDetail(0, containment)
	case res.ExitCode != 0:
		res.Detail = fmt.Sprintf("exit %d: %s", res.ExitCode, last)
	case s.Expect != "" && last != s.Expect:
		res.Detail = fmt.Sprintf("exit 0 without the expected completion line: %q", last)
	default:
		res.Status, res.Detail = "ok", last
	}
	return res
}

func lastLine(b []byte) string {
	lines := strings.Split(strings.TrimRight(string(b), "\r\n"), "\n")
	last := strings.TrimSuffix(lines[len(lines)-1], "\r")
	if len(last) > 200 {
		last = last[:200]
	}
	return last
}

// deleteWebDriverSession deletes the session the test phase recorded. A
// missing or malformed record is unobserved, never clean: a created session
// cannot be ruled out. Only a W3C "invalid session id" answer proves absence.
func deleteWebDriverSession(ctx context.Context, s CleanupStep, reportRoot *os.Root) CleanupStepResult {
	res := CleanupStepResult{Status: "unobserved", ExitCode: -1}
	f, err := reportRoot.Open(s.SessionFile)
	if err != nil {
		res.Detail = "session identity not recorded; a created session cannot be ruled out"
		return res
	}
	info, err := f.Stat()
	var b []byte
	if err == nil && info.Mode().IsRegular() {
		b, err = io.ReadAll(io.LimitReader(f, 129))
	}
	f.Close()
	id := string(b)
	if err != nil || !webDriverSessionID.MatchString(id) {
		res.Detail = "session identity record is not one WebDriver session id"
		return res
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.WebDriver+"session/"+id, nil)
	if err != nil {
		res.Status, res.Detail = "failed", err.Error()
		return res
	}
	client := &http.Client{
		Transport:     &http.Transport{Proxy: nil, DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		res.Status, res.Detail = "failed", "session "+id+": "+boundDetail(err.Error())
		if ctx.Err() == context.DeadlineExceeded {
			res.Status = "timed-out"
		}
		return res
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
	var answer struct {
		Value struct {
			Error string `json:"error"`
		} `json:"value"`
	}
	switch {
	case resp.StatusCode == http.StatusOK:
		res.Status, res.Detail = "ok", "session "+id+" deleted"
	case resp.StatusCode == http.StatusNotFound && json.Unmarshal(body, &answer) == nil && answer.Value.Error == "invalid session id":
		res.Status, res.Detail = "ok", "session "+id+" already absent"
	default:
		res.Status, res.Detail = "failed", fmt.Sprintf("session %s: HTTP %d", id, resp.StatusCode)
	}
	return res
}

func boundDetail(s string) string {
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

// cleanupDetail names every step that did not succeed.
func cleanupDetail(d *DeviceCleanup) string {
	parts := []string{}
	for _, s := range d.Steps {
		if s.Status != "ok" {
			parts = append(parts, s.Name+" "+s.Status+": "+s.Detail)
		}
	}
	detail := "device cleanup " + d.Outcome + ": " + strings.Join(parts, "; ")
	if len(detail) > 1024 {
		detail = detail[:1024]
	}
	return detail
}
