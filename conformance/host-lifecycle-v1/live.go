//go:build unix

package main

// live.go is the live-session mode of HLQ-V1-012 to HLQ-V1-015: the real host runs a
// non-interactive session against a model-free loopback provider and a recording wrapper stands in
// for the corvint executable, so the report observes what the host itself dispatched, timed out,
// cancelled and cleaned up. It never calls a paid model and never reads the operator's host state.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// liveCaseOrder is the HLQ-V1-013 per-case order of a live report.
var liveCaseOrder = []string{"dispatch", "timeout", "cancellation", "cleanup"}

// liveHosts names the hosts with a model-free non-interactive live path; every other host reports
// each live case NOT_RUN with the reason (HLQ-V1-012).
var liveHosts = map[string]string{
	"claude-code": "no model-free non-interactive session path is qualified for this host; live dispatch NOT_RUN",
	"codex":       "no model-free non-interactive session path is qualified for this host; live dispatch NOT_RUN",
}

const livePrompt = "Explain what this project does"

// liveRecord is one wrapper log line: a start, a held start, an end, or a self-release of a held
// invocation that the host never terminated.
type liveRecord struct {
	Phase    string  `json:"phase"`
	PID      int     `json:"pid"`
	PGID     int     `json:"pgid"`
	Event    string  `json:"event"`
	Ancestry []int   `json:"ancestry"`
	Time     float64 `json:"t"`
	Exit     *int    `json:"exit,omitempty"`
}

// liveSession is what one host session left behind.
type liveSession struct {
	hostPID     int
	exit        int
	err         error
	elapsed     time.Duration
	interrupted bool
	records     []liveRecord
	heldAlive   bool // the held invocation's process still existed after the host's exit and grace
}

type liveRunner struct {
	host, hostExecutable string
	corvint, source      string
	work, home, repo     string
	environment          []string
	hostVersion          string
	adapterVersion       string
	corvintVersion       string
	sourceRevision       string
	hold                 time.Duration // how long a held invocation sleeps before releasing itself
	grace                time.Duration // how long a terminated invocation may take to disappear
	bound                time.Duration // one host session's wall-clock bound
	results              []result
	pids                 map[int]int // every wrapper pid observed -> its process group
}

func runLive(host, hostExecutable, current, source, report string, stdout, stderr io.Writer) int {
	l := &liveRunner{host: host, hold: 20 * time.Second, grace: 4 * time.Second, bound: 90 * time.Second, pids: map[int]int{}}
	var err error
	if l.corvint, err = filepath.Abs(current); err == nil {
		l.source, err = filepath.Abs(source)
	}
	if err == nil {
		l.work, err = os.MkdirTemp("", "corvint-host-lifecycle-live-")
	}
	if err != nil {
		fmt.Fprintln(stderr, "host-lifecycle-v1:", err)
		return 2
	}
	defer os.RemoveAll(l.work)
	if reason, unsupported := liveHosts[host]; unsupported {
		l.hostVersion, l.adapterVersion = "unknown", "unknown"
		for _, name := range liveCaseOrder {
			l.results = append(l.results, result{name, "NOT_RUN", reason})
		}
	} else if err = l.prepare(hostExecutable); err != nil {
		fmt.Fprintln(stderr, "host-lifecycle-v1:", err)
		return 2
	} else {
		l.runOpenCode()
	}
	text := l.render()
	fmt.Fprint(stdout, text)
	exit := 1
	if liveDispatchVerdict(l.results) == "PASS" {
		exit = 0
	}
	if report != "" {
		if err := os.WriteFile(report, []byte(text), 0o644); err != nil {
			fmt.Fprintln(stderr, "host-lifecycle-v1:", err)
			return 2
		}
	}
	return exit
}

// prepare fixes the exact tuple and builds the private workspace: HOME and XDG roots under the
// work directory, and a committed fixture repository whose opencode.json is excluded from Git.
func (l *liveRunner) prepare(hostExecutable string) error {
	if hostExecutable == "" {
		hostExecutable = "opencode"
	}
	resolved, err := exec.LookPath(hostExecutable)
	if err != nil {
		return err
	}
	l.hostExecutable, _ = filepath.Abs(resolved)
	l.home, l.repo = filepath.Join(l.work, "home"), filepath.Join(l.work, "repo")
	for _, directory := range []string{l.home, l.repo} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
	}
	l.environment = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + l.home, "XDG_CONFIG_HOME=" + l.home + "/config", "XDG_DATA_HOME=" + l.home + "/data", "XDG_CACHE_HOME=" + l.home + "/cache", "XDG_STATE_HOME=" + l.home + "/state", "TMPDIR=" + l.work, "NO_COLOR=1"}
	output, err := l.capture(l.work, l.hostExecutable, "--version")
	if err != nil {
		return err
	}
	if l.hostVersion = hostVersion(output); l.hostVersion == "unknown" {
		return fmt.Errorf("cannot read the %s version from %q", l.host, firstLine(output))
	}
	version, err := l.capture(l.work, l.corvint, "--version")
	if err != nil || !strings.HasPrefix(version, "Corvint ") {
		return fmt.Errorf("cannot identify %s: %q %v", l.corvint, firstLine(version), err)
	}
	l.corvintVersion = strings.TrimSpace(version)
	if l.adapterVersion, err = readManifestVersion(filepath.Join(l.source, "integrations/opencode/package.json")); err != nil {
		return err
	}
	if l.sourceRevision, err = l.capture(l.source, "git", "rev-parse", "HEAD"); err != nil {
		return err
	}
	l.sourceRevision = strings.TrimSpace(l.sourceRevision)
	if err := os.WriteFile(filepath.Join(l.repo, "fx.go"), []byte("package fx\n\n// Add returns a+b.\nfunc Add(a, b int) int { return a + b }\n"), 0o600); err != nil {
		return err
	}
	for _, argv := range [][]string{{"git", "init", "-q"}, {"git", "add", "."}, {"git", "-c", "user.name=HLQ", "-c", "user.email=hlq@localhost", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture"}} {
		if _, err := l.capture(l.repo, argv...); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(l.repo, ".git/info/exclude"), []byte("opencode.json\n"), 0o600)
}

func (l *liveRunner) capture(directory string, argv ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Dir, command.Env, command.Stdin = directory, l.environment, nil
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s: %v: %s", strings.Join(argv, " "), err, firstLine(string(output)))
	}
	return string(output), nil
}

// runOpenCode runs three sessions: an unheld one (dispatch), one whose user-prompt invocation is
// held past the adapter's automatic bound (timeout), and one whose held invocation is in flight
// when the host's process group receives SIGINT (cancellation). Cleanup is judged over all three.
func (l *liveRunner) runOpenCode() {
	add := func(name, status, detail string) { l.results = append(l.results, result{name, status, detail}) }
	status, detail := dispatchStatus(l.session("plain", "", nil, false))
	add("dispatch", status, detail)
	status, detail = timeoutStatus(l.session("timeout", "user-prompt", nil, false))
	add("timeout", status, detail)
	status, detail = cancellationStatus(l.session("cancel", "user-prompt", []string{"CORVINT_OPENCODE_QUERY_TIMEOUT_MS=10000"}, true))
	add("cancellation", status, detail)
	status, detail = l.cleanupStatus()
	add("cleanup", status, detail)
}

// wrapperScript records every invocation the host's plugin makes and holds the first invocation
// of held for hold seconds, so the host's own timeout or cancellation is what ends it.
func wrapperScript(log, held, real string, hold time.Duration) string {
	return `#!/bin/sh
log=` + shellQuote(log) + `
event=; prev=
for a in "$@"; do [ "$prev" = "--event" ] && event=$a; prev=$a; done
now() { perl -MTime::HiRes=time -e 'printf "%.3f", time' 2>/dev/null || date +%s; }
chain=; p=$PPID; n=0
while [ -n "$p" ] && [ "$p" -gt 1 ] && [ $n -lt 32 ]; do chain="$chain${chain:+,}$p"; p=$(ps -o ppid= -p "$p" | tr -d ' '); n=$((n+1)); done
pgid=$(ps -o pgid= -p $$ | tr -d ' ')
record() { printf '{"phase":"%s","pid":%s,"pgid":%s,"event":"%s","ancestry":[%s],"t":%s%s}\n' "$1" $$ "${pgid:-0}" "$event" "$chain" "$(now)" "$2" >> "$log"; }
if [ -n ` + shellQuote(held) + ` ] && [ "$event" = ` + shellQuote(held) + ` ] && [ ! -e "$log.held" ]; then
  : > "$log.held"
  record held ""
  sleep ` + strconv.Itoa(int(hold.Seconds())) + `
  record released ""
  exit 0
fi
record start ""
` + shellQuote(real) + ` "$@"
rc=$?
record end ",\"exit\":$rc"
exit $rc
`
}

func shellQuote(text string) string { return "'" + strings.ReplaceAll(text, "'", `'\''`) + "'" }

// session runs one bounded `opencode run` against the loopback provider.
func (l *liveRunner) session(name, held string, extra []string, interrupt bool) liveSession {
	var s liveSession
	log := filepath.Join(l.work, name+".jsonl")
	wrapper := filepath.Join(l.work, name+"-corvint")
	if s.err = os.WriteFile(wrapper, []byte(wrapperScript(log, held, l.corvint, l.hold)), 0o700); s.err != nil {
		return s
	}
	base, stop, err := startLoopbackProvider()
	if s.err = err; err != nil {
		return s
	}
	defer stop()
	config := map[string]any{
		"model":   "local/probe",
		"plugins": []any{map[string]any{"package": (&url.URL{Scheme: "file", Path: filepath.Join(l.source, "integrations/opencode")}).String(), "options": map[string]any{"corvintBinary": wrapper}}},
		"providers": map[string]any{"local": map[string]any{
			"name": "HLQ loopback provider", "package": "@opencode/ai/providers/openai-compatible",
			"settings": map[string]any{"apiKey": "local-fixture", "baseURL": base},
			"models":   map[string]any{"probe": map[string]any{"modelID": "probe", "capabilities": map[string]any{"tools": true, "input": []string{"text"}, "output": []string{"text"}}, "limit": map[string]any{"context": 131072, "output": 1024}}},
		}},
	}
	data, _ := json.MarshalIndent(config, "", "  ")
	if s.err = os.WriteFile(filepath.Join(l.repo, "opencode.json"), data, 0o600); s.err != nil {
		return s
	}
	command := exec.Command(l.hostExecutable, "run", "--standalone", "--format", "json", "--model", "local/probe", "--title", "HLQ live "+name, livePrompt)
	command.Dir, command.Env = l.repo, append(slices.Clone(l.environment), extra...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Stdout, command.Stderr = io.Discard, io.Discard
	started := time.Now()
	if s.err = command.Start(); s.err != nil {
		return s
	}
	s.hostPID = command.Process.Pid
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	deadline := time.After(l.bound)
	if interrupt {
		if waitFor(30*time.Second, func() bool { return hasPhase(readRecords(log), "held") }) {
			s.interrupted = syscall.Kill(-s.hostPID, syscall.SIGINT) == nil
		}
	}
	select {
	case s.err = <-done:
	case <-deadline:
		_ = syscall.Kill(-s.hostPID, syscall.SIGKILL)
		<-done
		s.err = fmt.Errorf("host session exceeded %s and was killed", l.bound)
	}
	s.elapsed = time.Since(started)
	s.exit = command.ProcessState.ExitCode()
	var exitError *exec.ExitError
	if errors.As(s.err, &exitError) {
		s.err = nil
	}
	s.records = readRecords(log)
	for _, record := range s.records {
		l.pids[record.PID] = record.PGID
	}
	for _, record := range s.records {
		if record.Phase == "held" {
			s.heldAlive = !waitFor(l.grace, func() bool { return !alive(record.PID) })
		}
	}
	return s
}

func readRecords(path string) []liveRecord {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	var records []liveRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record liveRecord
		if json.Unmarshal(scanner.Bytes(), &record) == nil {
			records = append(records, record)
		}
	}
	return records
}

func hasPhase(records []liveRecord, phase string) bool {
	return slices.ContainsFunc(records, func(r liveRecord) bool { return r.Phase == phase })
}

// waitFor polls condition every 50 ms until it holds or bound elapses.
func waitFor(bound time.Duration, condition func() bool) bool {
	for end := time.Now().Add(bound); ; time.Sleep(50 * time.Millisecond) {
		if condition() {
			return true
		}
		if time.Now().After(end) {
			return false
		}
	}
}

func alive(pid int) bool {
	return pid > 0 && syscall.Kill(pid, 0) == nil
}

// dispatched lists the events the host's own process tree dispatched in s, in first-seen order.
func dispatched(s liveSession) []string {
	var events []string
	for _, record := range s.records {
		if (record.Phase == "start" || record.Phase == "held") && slices.Contains(record.Ancestry, s.hostPID) && !slices.Contains(events, record.Event) {
			events = append(events, record.Event)
		}
	}
	return events
}

func sessionFailure(s liveSession) string {
	if s.err != nil {
		return s.err.Error()
	}
	if s.hostPID == 0 {
		return "host session did not start"
	}
	return ""
}

// dispatchStatus passes only when the host exited 0 and its own process tree invoked the
// user-prompt hook; a command the harness ran itself never counts (HLQ-V1-013).
func dispatchStatus(s liveSession) (string, string) {
	if failure := sessionFailure(s); failure != "" {
		return "FAIL", failure
	}
	events := dispatched(s)
	detail := fmt.Sprintf("host exit=%d elapsed=%s dispatched=[%s] invocations=%d", s.exit, s.elapsed.Round(time.Millisecond), strings.Join(events, ","), len(s.records))
	if s.exit != 0 || !slices.Contains(events, "user-prompt") {
		return "FAIL", detail + "; expected exit 0 and a host-dispatched user-prompt invocation"
	}
	return "PASS", detail
}

// timeoutStatus passes when the held user-prompt invocation was dispatched by the host, ended by
// it before releasing itself, gone after the grace, and the host still completed with exit 0.
func timeoutStatus(s liveSession) (string, string) {
	if failure := sessionFailure(s); failure != "" {
		return "FAIL", failure
	}
	detail := fmt.Sprintf("host exit=%d elapsed=%s held=%t released=%t heldAlive=%t", s.exit, s.elapsed.Round(time.Millisecond), hasPhase(s.records, "held"), hasPhase(s.records, "released"), s.heldAlive)
	switch {
	case !slices.Contains(dispatched(s), "user-prompt"):
		return "FAIL", detail + "; the host never dispatched the held user-prompt invocation"
	case hasPhase(s.records, "released") || s.heldAlive:
		return "FAIL", detail + "; the host did not terminate the held invocation at its bound"
	case s.exit != 0:
		return "FAIL", detail + "; the host did not degrade to a completed session"
	}
	return "PASS", detail
}

// cancellationStatus passes when SIGINT reached the host while the held invocation was in flight
// and the invocation was gone within the grace after the host exited.
func cancellationStatus(s liveSession) (string, string) {
	if failure := sessionFailure(s); failure != "" {
		return "FAIL", failure
	}
	detail := fmt.Sprintf("host exit=%d elapsed=%s interrupted=%t released=%t heldAlive=%t", s.exit, s.elapsed.Round(time.Millisecond), s.interrupted, hasPhase(s.records, "released"), s.heldAlive)
	switch {
	case !s.interrupted:
		return "FAIL", detail + "; the held invocation never started, so no cancellation was observed"
	case hasPhase(s.records, "released") || s.heldAlive:
		return "FAIL", detail + "; the in-flight invocation outlived the cancelled host"
	}
	return "PASS", detail
}

// cleanupStatus checks every recorded invocation process, then kills any survivor's group so the
// harness itself never leaves one behind; a survivor is still a FAIL.
func (l *liveRunner) cleanupStatus() (string, string) {
	var survivors []string
	for pid, pgid := range l.pids {
		if !waitFor(l.grace, func() bool { return !alive(pid) }) {
			survivors = append(survivors, strconv.Itoa(pid))
			if pgid > 1 {
				_ = syscall.Kill(-pgid, syscall.SIGKILL)
			}
		}
	}
	slices.Sort(survivors)
	if len(l.pids) == 0 {
		return "FAIL", "no invocation was recorded, so no cleanup was observed"
	}
	if len(survivors) != 0 {
		return "FAIL", fmt.Sprintf("%d of %d recorded invocation processes survived their sessions: %s (killed by the harness)", len(survivors), len(l.pids), strings.Join(survivors, ","))
	}
	return "PASS", fmt.Sprintf("all %d recorded invocation processes exited", len(l.pids))
}

// liveDispatchVerdict is PASS only when every live case passed, FAIL when any failed, otherwise
// NOT_RUN (HLQ-V1-014).
func liveDispatchVerdict(results []result) string {
	statuses := map[string]string{}
	for _, item := range results {
		statuses[item.name] = item.status
	}
	verdict := "PASS"
	for _, name := range liveCaseOrder {
		switch statuses[name] {
		case "FAIL":
			return "FAIL"
		case "PASS":
		default:
			verdict = "NOT_RUN"
		}
	}
	return verdict
}

func (l *liveRunner) render() string {
	var out strings.Builder
	fmt.Fprintf(&out, "host-lifecycle-v1-live\thost=%s\tsurface=plugin\thostVersion=%s\tadapterVersion=%s\tos=%s/%s\tcorvint=%s\tsource=%s\tprovider=loopback\n",
		l.host, l.hostVersion, l.adapterVersion, runtime.GOOS, runtime.GOARCH, l.corvintVersion, l.sourceRevision)
	for _, name := range liveCaseOrder {
		item := result{name, "NOT_RUN", "case did not execute"}
		for _, candidate := range l.results {
			if candidate.name == name {
				item = candidate
			}
		}
		fmt.Fprintf(&out, "case\t%s\t%s\t%s\n", item.name, item.status, item.detail)
	}
	fmt.Fprintf(&out, "verdict\tinstallation\tNOT_RUN\tthe live session loads the plugin from source by file URL; installation is qualified only by the nine-case run\n")
	fmt.Fprintf(&out, "verdict\tdispatch\t%s\tlive cases %s\n", liveDispatchVerdict(l.results), strings.Join(liveCaseOrder, ","))
	authority, why := authorityVerdict(l.source, l.host)
	fmt.Fprintf(&out, "verdict\tauthority\t%s\t%s\n", authority, why)
	return out.String()
}

// startLoopbackProvider serves a model-free OpenAI-compatible chat stream on 127.0.0.1 that
// answers every request with one text delta and finish_reason stop. It calls no model.
func startLoopbackProvider() (string, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 8<<20))
		w.Header().Set("Content-Type", "text/event-stream")
		for _, choice := range []string{`{"index":0,"delta":{"role":"assistant","content":"HLQ_LIVE_DONE"},"finish_reason":null}`, `{"index":0,"delta":{},"finish_reason":"stop"}`} {
			fmt.Fprintf(w, "data: {\"id\":\"hlq\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"probe\",\"choices\":[%s]}\n\n", choice)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	done := make(chan struct{})
	go func() { defer close(done); _ = server.Serve(listener) }()
	return "http://" + listener.Addr().String() + "/v1", func() { _ = server.Close(); <-done }, nil
}
