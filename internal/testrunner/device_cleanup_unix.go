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
	"syscall"
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

// maxRecordedSessions bounds the session identity record: the session the
// run started plus each replacement browser.reloadSession() reported.
const maxRecordedSessions = 8

// deleteWebDriverSession deletes every session the test phase recorded: the
// first, and each replacement a reload recorded. A missing, non-regular or
// malformed record is unobserved, never clean: a created session cannot be
// ruled out. Only a W3C {"value":null} success, read whole before the
// deadline, or a W3C "invalid session id" answer proves a session gone.
func deleteWebDriverSession(ctx context.Context, s CleanupStep, reportRoot *os.Root) CleanupStepResult {
	res := CleanupStepResult{Status: "unobserved", ExitCode: -1}
	ids, detail := recordedWebDriverSessions(reportRoot, s.SessionFile)
	if ids == nil {
		res.Detail = detail
		return res
	}
	client := &http.Client{
		Transport:     &http.Transport{Proxy: nil, DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	res.Status = "ok"
	details := []string{}
	for _, id := range ids {
		status, detail := deleteOneWebDriverSession(ctx, client, s.WebDriver, id)
		if status != "ok" && res.Status != "timed-out" {
			res.Status = status
		}
		details = append(details, "session "+id+" "+detail)
	}
	res.Detail = boundDetail(strings.Join(details, "; "))
	return res
}

// recordedWebDriverSessions reads the session identity record without
// blocking on a FIFO: it must be a regular file, the same one when opened
// nonblocking, of one to maxRecordedSessions newline-terminated session ids.
func recordedWebDriverSessions(reportRoot *os.Root, name string) ([]string, string) {
	const malformed = "session identity record is not newline-terminated WebDriver session ids"
	info, err := reportRoot.Lstat(name)
	if err != nil {
		return nil, "session identity not recorded; a created session cannot be ruled out"
	}
	if !info.Mode().IsRegular() {
		return nil, malformed
	}
	f, err := reportRoot.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, malformed
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, malformed
	}
	b, err := io.ReadAll(io.LimitReader(f, maxRecordedSessions*37+1))
	if err != nil || len(b) == 0 || b[len(b)-1] != '\n' {
		return nil, malformed
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, id := range strings.Split(string(b[:len(b)-1]), "\n") {
		if !webDriverSessionID.MatchString(id) || len(ids) == maxRecordedSessions {
			return nil, malformed
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, ""
}

func deleteOneWebDriverSession(ctx context.Context, client *http.Client, webDriver, id string) (string, string) {
	failed := func(detail string) (string, string) {
		if ctx.Err() != nil {
			return "timed-out", boundDetail(detail)
		}
		return "failed", boundDetail(detail)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, webDriver+"session/"+id, nil)
	if err != nil {
		return "failed", err.Error()
	}
	resp, err := client.Do(req)
	if err != nil {
		return failed(err.Error())
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
	if err != nil || ctx.Err() != nil {
		return failed(fmt.Sprintf("HTTP %d response body not read whole: %v", resp.StatusCode, err))
	}
	var envelope map[string]json.RawMessage
	var answer struct {
		Value struct {
			Error string `json:"error"`
		} `json:"value"`
	}
	switch {
	case resp.StatusCode == http.StatusOK && json.Unmarshal(body, &envelope) == nil && string(envelope["value"]) == "null":
		return "ok", "deleted"
	case resp.StatusCode == http.StatusNotFound && json.Unmarshal(body, &answer) == nil && answer.Value.Error == "invalid session id":
		return "ok", "already absent"
	case resp.StatusCode == http.StatusOK:
		return "failed", "HTTP 200 without a W3C null-value envelope"
	default:
		return "failed", fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
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
