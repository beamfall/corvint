package opencodequalification

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

func fileURL(p string) string { return (&url.URL{Scheme: "file", Path: p}).String() }
func quoted(s string) string  { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
func readRows(path string) ([]Object, error) {
	b, e := readBounded(path, maxEvidenceBytes)
	if os.IsNotExist(e) {
		return []Object{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Object{}
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" {
			continue
		}
		x, e := decode([]byte(line))
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, nil
}
func contains(v any, s string) bool { b, _ := jsonBytes(v); return strings.Contains(string(b), s) }
func anyRow(rows []Object, p func(Object) bool) bool {
	for _, r := range rows {
		if p(r) {
			return true
		}
	}
	return false
}
func allRows(rows []Object, p func(Object) bool) bool {
	for _, r := range rows {
		if !p(r) {
			return false
		}
	}
	return true
}
func Native(ctx context.Context, c Config) (Object, error) {
	if e := mkdir(c.Output); e != nil {
		return nil, e
	}
	root, e := os.MkdirTemp(c.Output, "run-")
	if e != nil {
		return nil, e
	}
	baseline, e := identities(ctx, c, false)
	if e != nil {
		return nil, e
	}
	home, repo, probe := filepath.Join(root, "home"), filepath.Join(root, "repo"), filepath.Join(root, "probe")
	for _, p := range []string{home, repo, probe} {
		if e = mkdir(p); e != nil {
			return nil, e
		}
	}
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "XDG_CONFIG_HOME=" + home + "/config", "XDG_DATA_HOME=" + home + "/data", "XDG_CACHE_HOME=" + home + "/cache", "XDG_STATE_HOME=" + home + "/state", "TMPDIR=" + root, "NO_COLOR=1", "SECRET_DO_NOT_LEAK=qualification-sentinel", "GEMINI_API_KEY=qualification-sentinel"}
	source := "package qualification\n// before\nfunc Add(a, b int) int { return a + b }\n"
	for i := 0; i < 180; i++ {
		source += fmt.Sprintf("// unrelated fixture line %d: retained for the manual full-file read baseline.\n", i)
	}
	for name, text := range map[string]string{"go.mod": "module example.com/qualification\n\ngo 1.27.3\n", "add.go": source, "AGENTS.md": "# Project instructions\nFor repository workflow changes, inspect add.go and run git diff --check.\n", "oversized.txt": strings.Repeat("excluded fixture\n", 150000)} {
		if e = os.WriteFile(filepath.Join(repo, name), []byte(text), 0600); e != nil {
			return nil, e
		}
	}
	for _, args := range [][]string{{"git", "init", "-q"}, {"git", "add", "."}, {"git", "-c", "user.name=Qualification", "-c", "user.email=qualification@localhost", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture"}} {
		if _, e = capture(ctx, repo, env, args); e != nil {
			return nil, e
		}
	}
	tree, e := capture(ctx, repo, env, []string{"git", "rev-parse", "HEAD^{tree}"})
	if e != nil {
		return nil, e
	}
	tree = strings.TrimSpace(tree)
	blob, e := capture(ctx, repo, env, []string{"git", "rev-parse", "HEAD:add.go"})
	if e != nil {
		return nil, e
	}
	handle := "cv1:" + tree + ":" + strings.TrimSpace(blob) + ":1-3:add.go"
	wrapper := filepath.Join(root, "corvint-probe")
	configPath := filepath.Join(root, "probe.json")
	if e = writeJSON(configPath, Object{"binary": c.Corvint, "root": root, "interrupt": c.InterruptProbe}); e != nil {
		return nil, e
	}
	if e = os.WriteFile(wrapper, []byte("#!/bin/sh\nexec "+quoted(c.Self)+" --probe-config "+quoted(configPath)+" -- \"$@\"\n"), 0700); e != nil {
		return nil, e
	}
	if e = os.WriteFile(filepath.Join(probe, "index.js"), []byte(template(probeTemplate, map[string]string{"EVENTS": root + "/events.jsonl", "VERIFY": hash([]byte("git diff --check")), "FRAME_PROMPT": framePrompt})), 0600); e != nil {
		return nil, e
	}
	instrumented := filepath.Join(root, "instrumented")
	if e = mkdir(instrumented); e != nil {
		return nil, e
	}
	if e = os.WriteFile(filepath.Join(instrumented, "index.js"), []byte(template(observerTemplate, map[string]string{"CANDIDATE": fileURL(filepath.Join(c.Source, "integrations/opencode/src/index.js")), "TIMINGS": root + "/timings.jsonl", "FAULTS": root + "/faults.jsonl"})), 0600); e != nil {
		return nil, e
	}
	config := Object{"model": "local/probe", "plugins": []Object{{"package": fileURL(instrumented), "options": Object{"corvintBinary": wrapper}}, {"package": fileURL(probe), "options": Object{}}}, "providers": Object{"local": Object{"name": "Qualification loopback provider", "package": "@opencode/ai/providers/openai-compatible", "settings": Object{"apiKey": "local-fixture"}, "models": Object{"probe": Object{"modelID": "probe", "capabilities": Object{"tools": true, "input": []string{"text"}, "output": []string{"text"}}, "limit": Object{"context": 131072, "output": 1024}}}}}}
	if e = os.WriteFile(filepath.Join(repo, ".git/info/exclude"), []byte("opencode.json\n"), 0600); e != nil {
		return nil, e
	}
	campaign := func(name string, benchmark, smoke bool) error {
		base, stop, e := startProvider(ctx, root, benchmark, smoke, c.InterruptProbe)
		if e != nil {
			return e
		}
		defer stop()
		object(object(object(config["providers"])["local"])["settings"])["baseURL"] = base
		if e = writeJSON(filepath.Join(repo, "opencode.json"), config); e != nil {
			return e
		}
		_, e = runCommand(ctx, repo, env, []string{c.Host, "run", "--standalone", "--format", "json", "--model", "local/probe", "--title", "Qualification", framePrompt}, filepath.Join(root, name), 90*time.Second)
		return e
	}
	if e = campaign("native", false, false); e != nil {
		return nil, e
	}
	providerFaults, e := readRows(root + "/provider-faults.jsonl")
	if e != nil {
		return nil, e
	}
	invocations, e := readRows(root + "/invocations.jsonl")
	if e != nil {
		return nil, e
	}
	events, e := readRows(root + "/events.jsonl")
	if e != nil {
		return nil, e
	}
	requests, e := readRows(root + "/requests.jsonl")
	if e != nil {
		return nil, e
	}
	packets := []Object{}
	for _, x := range invocations {
		if number(x["exit"]) == 0 && strings.HasPrefix(str(x["output"]), "{") {
			p, e := decode([]byte(str(x["output"])))
			if e != nil {
				return nil, e
			}
			packets = append(packets, p)
		}
	}
	prompts := []string{}
	var firstPrompt Object
	for _, x := range events {
		if x["kind"] == "prompt" {
			if firstPrompt == nil {
				firstPrompt = x
			}
			prompts = append(prompts, str(x["text"]))
		}
	}
	if len(prompts) == 0 || len(requests) == 0 {
		return nil, errors.New("native prompt evidence missing")
	}
	queryPackets := []Object{}
	for _, p := range packets {
		if p["event"] == "user-prompt" {
			queryPackets = append(queryPackets, p)
		}
	}
	if len(queryPackets) == 0 {
		return nil, errors.New("native query evidence missing")
	}
	timings, e := readRows(root + "/timings.jsonl")
	if e != nil {
		return nil, e
	}
	injected, deliveredPrompt := hiddenPromptDelivery(prompts[0], events, array(requests[0]["messages"]), receiptLink(str(queryPackets[0]["receiptId"])))
	deliveredPrompt = deliveredPrompt && unchangedPromptObservation(firstPrompt, timings)
	checks := Object{
		"native-discovery": anyRow(events, func(x Object) bool { return x["version"] == c.HostVersion }),
		"native-host-image": anyRow(events, func(x Object) bool {
			return x["kind"] == "setup" && x["executableSHA256"] == baseline.HostSHA256 && x["architecture"] == baseline.Tuple["architecture"] && x["os"] == baseline.Tuple["os"]
		}),
		"awaited-current-prompt": deliveredPrompt,
		"native-bundled-skill":   bundledSkillAdvertised(requests),
		"native-query":           anyRow(events, func(x Object) bool { return x["name"] == "corvint_context" && x["status"] == "completed" }),
		"native-exact-expansion": anyRow(packets, func(x Object) bool {
			return x["handle"] == handle && object(x["selection"])["text"] == strings.Join(strings.Split(source, "\n")[:3], "\n")+"\n"
		}),
		"verification-observation": anyRow(invocations, func(x Object) bool {
			return len(array(object(x["input"])["verification"])) > 0 && argumentEvent(array(x["argv"])) == "post-tool"
		}),
		"explicit-outcome": anyRow(invocations, func(x Object) bool {
			i := object(x["input"])
			_, task := i["task"]
			return i["outcome"] == "passed" && !task
		}),
		"advisory-completion": anyRow(packets, func(x Object) bool {
			f := object(x["frontier"])
			return x["event"] == "stop" && f["state"] == "UNAVAILABLE" && f["shouldContinue"] == false
		}),
		"native-compaction": anyRow(events, func(x Object) bool { return x["type"] == "session.compaction.ended" }),
		"dirty-path-recovery": anyRow(invocations, func(x Object) bool { return contains(x["input"], `"startSource":"compact"`) }) && anyRow(events, func(x Object) bool {
			return x["kind"] == "context" && contains(x["system"], "add.go") && contains(x["system"], "rehydration")
		}),
		"environment-filter": allRows(invocations, func(x Object) bool { v, ok := x["leakedKeys"].([]any); return ok && len(v) == 0 }), "native-exit": true,
	}
	edited, e := os.ReadFile(filepath.Join(repo, "add.go"))
	if e != nil {
		return nil, e
	}
	checks["native-edit"] = strings.Contains(string(edited), "// after")
	// AHI-032 now measures the receipt-linked frame supplied through model context.
	checks["bounded-prompt"] = deliveredPrompt && injected <= 8000
	timingStart := len(timings)
	faults, e := readRows(root + "/faults.jsonl")
	if e != nil {
		return nil, e
	}
	faultStart := len(faults)
	config["plugins"] = []Object{{"package": fileURL(instrumented), "options": Object{"corvintBinary": c.Corvint}}, {"package": fileURL(probe), "options": Object{}}}
	windowStart := readContention()
	stopBaseline := startBaseline(ctx)
	e = campaign("timing", true, false)
	firstBaseline := stopBaseline()
	if e != nil {
		return nil, e
	}
	windowEnd := readContention()
	timings, e = readRows(root + "/timings.jsonl")
	if e != nil {
		return nil, e
	}
	faults, e = readRows(root + "/faults.jsonl")
	if e != nil {
		return nil, e
	}
	first, delivered := timingSample(timings[timingStart:])
	first.start, first.end, first.baseline = windowStart, windowEnd, firstBaseline
	checks["timed-delivery"] = delivered && len(faults) == faultStart
	attempts := []latencySample{first}
	// AHI-056: one same-run re-sample when only the p95 limits missed; the re-sample decides.
	if truth(checks["timed-delivery"]) && latencyResample(first) {
		resampleStart := len(timings)
		windowStart = readContention()
		stopBaseline = startBaseline(ctx)
		e = campaign("timing-resample", true, false)
		secondBaseline := stopBaseline()
		if e != nil {
			return nil, e
		}
		windowEnd = readContention()
		if timings, e = readRows(root + "/timings.jsonl"); e != nil {
			return nil, e
		}
		if faults, e = readRows(root + "/faults.jsonl"); e != nil {
			return nil, e
		}
		second, again := timingSample(timings[resampleStart:])
		second.start, second.end, second.baseline = windowStart, windowEnd, secondBaseline
		checks["timed-delivery"] = again && len(faults) == faultStart
		attempts = append(attempts, second)
	}
	decisive := attempts[len(attempts)-1]
	queryTimes, lifeTimes := decisive.query, decisive.lifecycle
	checks["latency"] = len(latencyFailures(decisive)) == 0
	checks["snapshot"] = allRows(packets, func(x Object) bool { r, ok := x["repository"]; return !ok || object(r)["treeRevision"] == tree }) && anyRow(packets, func(x Object) bool { return object(x["repository"])["worktreeState"] == "mixed" })
	checks["governance"] = anyRow(queryPackets, func(p Object) bool {
		return anyValue(array(object(p["context"])["results"]), func(row Object) bool {
			return anyValue(array(row["evidence"]), func(e Object) bool { return e["path"] == "AGENTS.md" })
		})
	})
	checks["oversized-exclusion"] = anyRow(queryPackets, func(p Object) bool { return number(object(object(p["context"])["exclusions"])["count"]) > 0 })
	// AHI-218: the frame now orients and the source query recalls, so recall counts any query packet.
	recall := anyRow(queryPackets, func(p Object) bool {
		return anyValue(array(object(p["context"])["results"]), func(row Object) bool {
			return row["name"] == "Add" && anyValue(array(row["evidence"]), func(e Object) bool { return e["path"] == "add.go" && number(e["line"]) == 3 })
		})
	})
	checks["critical-recall-and-bytes"] = deliveredPrompt && injected < len(source) && recall
	checks["supplied-evidence"] = anyRow(invocations, func(x Object) bool { return len(array(object(x["input"])["observedEvidenceHandles"])) > 0 })
	stops := 0
	for _, p := range packets {
		if p["event"] == "stop" {
			stops++
		}
	}
	checks["bounded-stops"] = stops >= 1 && stops <= 2
	metrics := Object{"query": queryTimes, "lifecycle": lifeTimes, "queryP95Ms": p95Evidence(queryTimes), "lifecycleP95Ms": p95Evidence(lifeTimes), "latencyAttempts": latencyAttemptEvidence(attempts), "latencyMethod": latencyMethod, "latencyOutcome": latencyOutcome(decisive), "contentionThresholdLoadPerCPU": latencyContentionLoadPerCPU, "latencyBaseline": baselineEvidence(decisive), "injectedBytes": injected, "manualBytes": len(source), "criticalRecall": Object{"injected": boolInt(recall), "manual": 1, "total": 1}, "surface": "real stock host callbacks, including normalization, spawn, framing and delivery", "observer": "transparent registration wrapper; candidate source unchanged; direct Corvint executable, no capture subprocess"}
	for _, installed := range []bool{true, false} {
		config["plugins"] = []Object{}
		name := "uninstall"
		if installed {
			name = "direct-install"
			config["plugins"] = []Object{{"package": fileURL(filepath.Join(c.Source, "integrations/opencode/src")), "options": Object{"corvintBinary": wrapper}}}
		}
		beforeCalls, e := readRows(root + "/invocations.jsonl")
		if e != nil {
			return nil, e
		}
		beforeRequests, e := readRows(root + "/requests.jsonl")
		if e != nil {
			return nil, e
		}
		if e = campaign(name, false, true); e != nil {
			return nil, e
		}
		calls, e := readRows(root + "/invocations.jsonl")
		if e != nil {
			return nil, e
		}
		requests, e := readRows(root + "/requests.jsonl")
		if e != nil {
			return nil, e
		}
		advertised := anyRow(requests[len(beforeRequests):], func(x Object) bool {
			b, _ := jsonBytes(x)
			return contains(x, "corvint_context") && frameLinkPattern.Match(b)
		})
		checks[name] = advertised
		if !installed {
			checks[name] = !advertised && len(calls) == len(beforeCalls)
		}
	}
	cleanup, e := interruptNative(ctx, c, root)
	if e != nil {
		return nil, e
	}
	checks["interruption-cleanup"] = number(cleanup["exit"]) == 143 && len(array(cleanup["survivors"])) == 0
	after, e := identities(ctx, c, false)
	if e != nil {
		return nil, e
	}
	checks["frozen-identities"] = reflect.DeepEqual(baseline, after)
	report := identityFields(baseline)
	for k, v := range baseline.Tuple {
		report[k] = v
	}
	report["profile"] = Profile
	report["result"] = "PASS"
	for _, v := range checks {
		if !truth(v) {
			report["result"] = "FAIL"
		}
	}
	retryLater := latencyOnlyRetry(checks, decisive)
	if retryLater {
		report["result"] = resultRetryLater
		report["retryHint"] = retryHint(decisive)
	}
	// AHI-372: the quiet latency check stays false; the distinct result lets a consumer require PASS.
	if latencyOnlyQualifiedContended(checks, decisive) {
		report["result"] = latencyOutcomeQualifiedContended
		report["contendedQualification"] = contendedQualificationEvidence(decisive)
	}
	report["executionAuthority"] = "NONE"
	report["frontier"] = "UNAVAILABLE"
	report["legacyReceiptSupport"] = "FALLBACK"
	report["checks"] = checks
	failures := checkFailures(checks, providerFaults, attempts)
	report["failures"] = failures
	report["root"] = root
	report["metrics"] = metrics
	report["cleanupEvidence"] = cleanup
	report["manualBaseline"] = Object{"method": "read complete named source file", "bytes": len(source), "criticalEvidence": []string{"add.go:Add at line 3"}}
	report["inputTokens"] = "NOT_OBSERVED (provider usage is synthetic compaction stimulus)"
	report["requiresFocusedSuite"] = "TestHostAdapterJavaScriptHosts"
	for _, p := range []string{root + "/report.json", c.Output + "/report.json"} {
		if e = writeJSON(p, report); e != nil {
			return nil, e
		}
	}
	// Normalize the in-memory report to the same JSON types as a retained report.
	b, e := jsonBytes(report)
	if e != nil {
		return nil, e
	}
	report, e = decode(b)
	if e != nil {
		return nil, e
	}
	if retryLater {
		return report, &RetryLaterError{Message: "native campaign retry-later: " + strings.TrimPrefix(campaignFailure(failures, root), "native campaign failed: ") + "; latency missed under host contention, not a regression; " + retryHint(decisive)}
	}
	if report["result"] != "PASS" && report["result"] != latencyOutcomeQualifiedContended {
		return report, errors.New(campaignFailure(failures, root))
	}
	return report, nil
}
func anyValue(values []any, p func(Object) bool) bool {
	for _, x := range values {
		if p(object(x)) {
			return true
		}
	}
	return false
}
func argumentEvent(args []any) string {
	for i, v := range args {
		if v == "--event" && i+1 < len(args) {
			return str(args[i+1])
		}
	}
	return ""
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// hiddenPromptDelivery checks the actual hook frame and outgoing provider payload together.
// User text stays visible; the identical bounded receipt belongs only in model context (AHI-032).
// frameLinkPattern matches a delivered prompt frame's receipt link (AHI-219).
var frameLinkPattern = regexp.MustCompile(`cvr:[0-9a-f]{24}`)

// receiptLink is the adapter's 28-byte frame link for a receipt id, or "" for a malformed one
// (AHI-219): `cvr:` and the first 24 hex digits of the receipt's SHA-256.
func receiptLink(receipt string) string {
	const prefix = "harness-receipt:sha256:"
	digest, ok := strings.CutPrefix(receipt, prefix)
	if !ok || len(digest) != 64 || strings.Trim(digest, "0123456789abcdef") != "" {
		return ""
	}
	return "cvr:" + digest[:24]
}

func hiddenPromptDelivery(prompt string, events []Object, messages []any, receipt string) (int, bool) {
	const marker = "BEGIN CORVINT REPOSITORY DATA"
	if receipt == "" || strings.Contains(prompt, marker) {
		return 0, false
	}
	for _, value := range messages {
		message := object(value)
		if message["role"] == "user" && contains(message, marker) {
			return 0, false
		}
	}
	for _, event := range events {
		if event["kind"] != "context" {
			continue
		}
		for _, item := range array(event["system"]) {
			frame := str(object(item)["text"])
			if !strings.Contains(frame, marker) || !strings.Contains(frame, receipt) || len(frame) > 8000 {
				continue
			}
			for _, value := range messages {
				message := object(value)
				if message["role"] != "system" && message["role"] != "developer" {
					continue
				}
				content := message["content"]
				if text, ok := content.(string); ok && strings.Contains(text, frame) {
					return len(frame), true
				}
				for _, part := range array(content) {
					if strings.Contains(str(object(part)["text"]), frame) {
						return len(frame), true
					}
				}
			}
		}
	}
	return 0, false
}

// skillDeltaHeader opens OpenCode's appended skill-change notice, which lists skills that a
// plugin registered after the session's first request.
const skillDeltaHeader = "New skills are available in addition to those previously listed:\n"

// bundledSkillAdvertised reports whether an actual outgoing provider request listed the
// plugin-registered corvint skill in OpenCode's available-skills guidance (AHI-176), or, since
// the adapter registers it on the first corvint_context call (AHI-212), in OpenCode's appended
// skill-change notice. The host may map that mid-conversation notice to a user-role message.
func bundledSkillAdvertised(requests []Object) bool {
	advertised := func(text string) bool {
		start := strings.Index(text, "<available_skills>")
		end := strings.Index(text, "</available_skills>")
		return start >= 0 && end > start && strings.Contains(text[start:end], "<id>corvint</id>")
	}
	appended := func(text string) bool {
		start := strings.Index(text, skillDeltaHeader)
		if start < 0 {
			return false
		}
		list := text[start+len(skillDeltaHeader):]
		if end := strings.Index(list, "\n\n"); end >= 0 {
			list = list[:end]
		}
		return strings.Contains(list, "    <id>corvint</id>")
	}
	for _, request := range requests {
		for _, value := range array(request["messages"]) {
			message := object(value)
			guidance := message["role"] == "system" || message["role"] == "developer"
			if !guidance && message["role"] != "user" {
				continue
			}
			found := func(text string) bool { return (guidance && advertised(text)) || appended(text) }
			if text, ok := message["content"].(string); ok && found(text) {
				return true
			}
			for _, part := range array(message["content"]) {
				if found(str(object(part)["text"])) {
					return true
				}
			}
		}
	}
	return false
}

// The delivery observation belongs to the exact prompt that produced the first provider request.
// Later idle timing samples must never mask mutation of that draft (AHI-032).
func unchangedPromptObservation(prompt Object, timings []Object) bool {
	session, message := str(prompt["sessionID"]), str(prompt["messageID"])
	if session == "" || message == "" {
		return false
	}
	return anyRow(timings, func(row Object) bool {
		return row["kind"] == "session.prompt" && row["sessionID"] == session && row["messageID"] == message && truth(row["unchanged"]) && truth(row["delivered"])
	})
}

// AHI-012 latency limits, shared by the campaign and the consumer record check.
const (
	queryP95LimitMs     = 500.0
	lifecycleP95LimitMs = 250.0
	latencyMinSamples   = 20
	latencyMethod       = "AHI-056: absolute AHI-012 p95 limits; at most one same-run re-sample when only a p95 limit missed; the re-sample decides and every attempt is retained. AHI-171: each window records host contention; a latency-only miss whose decisive window peaked above the declared load per CPU is RETRY_LATER (LATENCY_HOST_CONTENDED), not a regression. AHI-172: a fixed no-op probe sampled during each window measures the host's slowdown against its quiet reference; a contended miss beyond the limits scaled by that slowdown and tolerance is a LATENCY_REGRESSION. AHI-372: a contended miss within the AHI-012 limits scaled by that slowdown capped at 1.5, with no tolerance, is QUALIFIED_UNDER_CONTENTION, never PASS"
)

// latencySample is one timing campaign's post-warm-up samples in milliseconds.
// start and end are the AHI-171 host-contention readings at the window's boundaries; baseline is the
// AHI-172 same-window no-op probe.
type latencySample struct {
	query, lifecycle []float64
	start, end       contentionSnapshot
	baseline         baselineSample
}

// timingSample extracts one campaign's samples, dropping each series' first (warm-up) sample.
func timingSample(rows []Object) (latencySample, bool) {
	s := latencySample{query: []float64{}, lifecycle: []float64{}}
	delivered := true
	for _, x := range rows {
		kind := str(x["kind"])
		if kind == "session.prompt" {
			s.query = append(s.query, number(x["ms"]))
			delivered = delivered && truth(x["delivered"])
		}
		if kind == "event.session.created" || kind == "event.session.execution.succeeded" || kind == "event.session.execution.failed" || kind == "event.session.deleted" || (strings.HasPrefix(kind, "tool.execute.after:") && !strings.HasSuffix(kind, ":execute")) {
			s.lifecycle = append(s.lifecycle, number(x["ms"]))
		}
	}
	delivered = delivered && len(s.query) > 0
	if len(s.query) > 0 {
		s.query = s.query[1:]
	}
	if len(s.lifecycle) > 0 {
		s.lifecycle = s.lifecycle[1:]
	}
	return s, delivered
}

// latencyFailures names each missed latency limit with its measured value; empty means PASS.
func latencyFailures(s latencySample) []string {
	out := []string{}
	for _, m := range []struct {
		name   string
		values []float64
		limit  float64
	}{{"query", s.query, queryP95LimitMs}, {"lifecycle", s.lifecycle, lifecycleP95LimitMs}} {
		if len(m.values) < latencyMinSamples {
			out = append(out, fmt.Sprintf("%s samples %d < %d", m.name, len(m.values), latencyMinSamples))
		} else if p := p95(m.values); p > m.limit {
			out = append(out, fmt.Sprintf("%s p95 %.0fms > %.0fms", m.name, p, m.limit))
		}
	}
	return out
}

// latencyResample reports whether a sample missed only on a p95 limit, so a re-sample may decide.
// A short sample is a collection failure, not host load, and is never re-sampled.
func latencyResample(s latencySample) bool {
	return len(s.query) >= latencyMinSamples && len(s.lifecycle) >= latencyMinSamples && len(latencyFailures(s)) > 0
}

// p95Evidence is the report form of p95: NOT_OBSERVED with no samples, because the +Inf p95 uses
// for comparison cannot be encoded and would lose the whole report (AHI-218).
func p95Evidence(values []float64) any {
	if len(values) == 0 {
		return notObserved
	}
	return p95(values)
}

func latencyAttemptEvidence(attempts []latencySample) []Object {
	out := []Object{}
	for _, a := range attempts {
		out = append(out, Object{"queryP95Ms": p95Evidence(a.query), "lifecycleP95Ms": p95Evidence(a.lifecycle), "querySamples": len(a.query), "lifecycleSamples": len(a.lifecycle), "failures": latencyFailures(a), "outcome": latencyOutcome(a), "contention": Object{"start": a.start.evidence(), "end": a.end.evidence()}, "baseline": baselineEvidence(a)})
	}
	return out
}

// checkFailureReasons names, per native check, the typed reason and the predicate a false value
// missed (AHI-213); every check in the native report has an entry.
var checkFailureReasons = map[string][2]string{
	"native-discovery":          {"host-version-not-observed", "no event reported the expected OpenCode host version"},
	"native-host-image":         {"host-image-mismatch", "no setup event matched the baseline host executable digest, OS and architecture"},
	"awaited-current-prompt":    {"prompt-context-not-delivered", "the receipt-linked frame was not delivered hidden in the first provider request with the prompt unchanged"},
	"native-bundled-skill":      {"bundled-skill-not-advertised", "no provider request advertised the corvint skill"},
	"native-query":              {"context-tool-not-completed", "no corvint_context tool event completed"},
	"native-exact-expansion":    {"exact-expansion-not-observed", "no corvint_expand packet carried the expected cv1 lines 1-3 handle for add.go with the first three source lines"},
	"verification-observation":  {"verification-not-observed", "no post-tool invocation carried a verification observation"},
	"explicit-outcome":          {"explicit-outcome-not-recorded", "no invocation recorded an explicit passed outcome"},
	"advisory-completion":       {"advisory-stop-not-observed", "no stop packet carried an UNAVAILABLE frontier that does not continue"},
	"native-compaction":         {"compaction-not-observed", "no session.compaction.ended event"},
	"dirty-path-recovery":       {"dirty-path-recovery-not-observed", "no compact-start invocation and rehydration context naming add.go"},
	"environment-filter":        {"environment-leak-or-unobserved", "an invocation leaked environment keys or did not report them"},
	"native-exit":               {"native-exit-failed", "the native campaign did not exit cleanly"},
	"native-edit":               {"edit-not-applied", "add.go does not contain the scripted edit"},
	"bounded-prompt":            {"prompt-frame-unbounded-or-undelivered", "the delivered prompt frame was missing or over 8000 bytes"},
	"timed-delivery":            {"timed-delivery-incomplete-or-faulted", "the timing campaign was not delivered or recorded faults"},
	"snapshot":                  {"snapshot-revision-mismatch", "a packet named another tree revision or none observed the mixed worktree"},
	"governance":                {"governance-evidence-missing", "no query packet carried AGENTS.md evidence"},
	"oversized-exclusion":       {"oversized-exclusion-missing", "no query packet counted an excluded oversized source"},
	"critical-recall-and-bytes": {"critical-recall-or-bytes-missed", "no query packet recalled Add at add.go:3, or the delivered frame was missing or not smaller than the file"},
	"supplied-evidence":         {"supplied-evidence-not-observed", "no invocation carried observed evidence handles"},
	"bounded-stops":             {"stop-count-out-of-bounds", "the stop packet count was not 1 or 2"},
	"direct-install":            {"direct-install-not-advertised", "the directly installed plugin did not advertise corvint_context with a receipt"},
	"uninstall":                 {"uninstall-left-surface", "the uninstalled campaign still advertised Corvint or invoked it"},
	"interruption-cleanup":      {"interruption-cleanup-incomplete", "the interrupted campaign did not exit 143 without survivors"},
	"frozen-identities":         {"collector-identities-changed", "host, adapter or Corvint identities changed during the run"},
}

// checkFailures lists one named reason per false check, sorted by check (AHI-213). A typed provider
// fault for a check replaces its generic reason; latency carries its outcome and each attempt's misses.
func checkFailures(checks Object, faults []Object, attempts []latencySample) []Object {
	names := []string{}
	for k, v := range checks {
		if !truth(v) {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	out := []Object{}
	for _, name := range names {
		if name == "latency" {
			detail := []string{}
			for i, a := range attempts {
				f := latencyFailures(a)
				if len(f) == 0 {
					f = []string{"within limits"}
				}
				detail = append(detail, fmt.Sprintf("attempt %d: %s", i+1, strings.Join(f, ", ")))
			}
			reason := "latency-samples-missing"
			if len(attempts) > 0 {
				reason = latencyOutcome(attempts[len(attempts)-1])
			}
			out = append(out, Object{"check": name, "reason": reason, "detail": strings.Join(detail, "; ")})
			continue
		}
		known, ok := checkFailureReasons[name]
		if !ok {
			known = [2]string{"check-failed", "the check's predicate was not satisfied"}
		}
		failure := Object{"check": name, "reason": known[0], "detail": known[1]}
		for _, f := range faults {
			if f["check"] == name && str(f["reason"]) != "" {
				failure["reason"] = str(f["reason"])
				if f["reason"] == reasonNoExpansionHandle {
					failure["detail"] = "no corvint_context tool result carried a cv1 handle for add.go, so the scripted provider did not call corvint_expand"
				}
				break
			}
		}
		out = append(out, failure)
	}
	return out
}

// campaignFailure names every failed check with its reason; the latency check carries measured
// values and limits per attempt (AHI-056).
func campaignFailure(failures []Object, root string) string {
	parts := []string{}
	for _, f := range failures {
		if f["check"] == "latency" {
			parts = append(parts, "latency ("+str(f["detail"])+")")
			continue
		}
		parts = append(parts, str(f["check"])+" ("+str(f["reason"])+")")
	}
	return "native campaign failed: " + strings.Join(parts, "; ") + "; inspect " + root
}
