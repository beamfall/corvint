package opencodequalification

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

func fileURL(p string) string { return (&url.URL{Scheme: "file", Path: p}).String() }
func quoted(s string) string  { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
func readRows(path string) ([]Object, error) {
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return []Object{}, nil
	}
	if e != nil {
		return nil, e
	}
	if len(b) > 16<<20 {
		return nil, errors.New("evidence stream exceeds bound")
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
	for name, text := range map[string]string{"go.mod": "module example.com/qualification\n\ngo 1.27.1\n", "add.go": source, "AGENTS.md": "# Project instructions\nFor repository workflow changes, inspect add.go and run git diff --check.\n", "oversized.txt": strings.Repeat("excluded fixture\n", 150000)} {
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
	if e = os.WriteFile(filepath.Join(probe, "index.js"), []byte(template(probeTemplate, map[string]string{"EVENTS": root + "/events.jsonl", "VERIFY": hash([]byte("git diff --check"))})), 0600); e != nil {
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
		_, e = runCommand(ctx, repo, env, []string{c.Host, "run", "--standalone", "--format", "json", "--model", "local/probe", "--title", "Qualification", "Locate Add in add.go."}, filepath.Join(root, name), 90*time.Second)
		return e
	}
	if e = campaign("native", false, false); e != nil {
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
	for _, x := range events {
		if x["kind"] == "prompt" {
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
	injected, deliveredPrompt := hiddenPromptDelivery(prompts[0], events, array(requests[0]["messages"]), str(queryPackets[0]["receiptId"]))
	deliveredPrompt = deliveredPrompt && anyRow(timings, func(x Object) bool {
		return x["kind"] == "session.prompt" && truth(x["unchanged"]) && truth(x["delivered"])
	})
	checks := Object{
		"native-discovery": anyRow(events, func(x Object) bool { return x["version"] == c.HostVersion }),
		"native-host-image": anyRow(events, func(x Object) bool {
			return x["kind"] == "setup" && x["executableSHA256"] == baseline.HostSHA256 && x["architecture"] == baseline.Tuple["architecture"] && x["os"] == baseline.Tuple["os"]
		}),
		"awaited-current-prompt": deliveredPrompt,
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
	if e = campaign("timing", true, false); e != nil {
		return nil, e
	}
	timings, e = readRows(root + "/timings.jsonl")
	if e != nil {
		return nil, e
	}
	faults, e = readRows(root + "/faults.jsonl")
	if e != nil {
		return nil, e
	}
	queryTimes, lifeTimes := []float64{}, []float64{}
	delivered := true
	for _, x := range timings[timingStart:] {
		kind := str(x["kind"])
		if kind == "session.prompt" {
			queryTimes = append(queryTimes, number(x["ms"]))
			delivered = delivered && truth(x["delivered"])
		}
		if kind == "event.session.created" || kind == "event.session.execution.succeeded" || kind == "event.session.execution.failed" || kind == "event.session.deleted" || (strings.HasPrefix(kind, "tool.execute.after:") && !strings.HasSuffix(kind, ":execute")) {
			lifeTimes = append(lifeTimes, number(x["ms"]))
		}
	}
	checks["timed-delivery"] = len(queryTimes) > 0 && delivered && len(faults) == faultStart
	if len(queryTimes) > 0 {
		queryTimes = queryTimes[1:]
	}
	if len(lifeTimes) > 0 {
		lifeTimes = lifeTimes[1:]
	}
	checks["latency"] = len(queryTimes) >= 20 && len(lifeTimes) >= 20 && p95(queryTimes) <= 500 && p95(lifeTimes) <= 250
	checks["snapshot"] = allRows(packets, func(x Object) bool { r, ok := x["repository"]; return !ok || object(r)["treeRevision"] == tree }) && anyRow(packets, func(x Object) bool { return object(x["repository"])["worktreeState"] == "mixed" })
	checks["governance"] = anyRow(queryPackets, func(p Object) bool {
		return anyValue(array(object(p["context"])["results"]), func(row Object) bool {
			return anyValue(array(row["evidence"]), func(e Object) bool { return e["path"] == "AGENTS.md" })
		})
	})
	checks["oversized-exclusion"] = anyRow(queryPackets, func(p Object) bool { return number(object(object(p["context"])["exclusions"])["count"]) > 0 })
	recall := anyValue(array(object(queryPackets[0]["context"])["results"]), func(row Object) bool {
		return row["name"] == "Add" && anyValue(array(row["evidence"]), func(e Object) bool { return e["path"] == "add.go" && number(e["line"]) == 3 })
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
	metrics := Object{"query": queryTimes, "lifecycle": lifeTimes, "queryP95Ms": p95(queryTimes), "lifecycleP95Ms": p95(lifeTimes), "injectedBytes": injected, "manualBytes": len(source), "criticalRecall": Object{"injected": boolInt(recall), "manual": 1, "total": 1}, "surface": "real stock host callbacks, including normalization, spawn, framing and delivery", "observer": "transparent registration wrapper; candidate source unchanged; direct Corvint executable, no capture subprocess"}
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
		advertised := anyRow(requests[len(beforeRequests):], func(x Object) bool { return contains(x, "corvint_context") && contains(x, "harness-receipt:sha256:") })
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
	report["executionAuthority"] = "NONE"
	report["frontier"] = "UNAVAILABLE"
	report["legacyReceiptSupport"] = "FALLBACK"
	report["checks"] = checks
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
	if report["result"] != "PASS" {
		return report, errors.New("native campaign failed; inspect " + root)
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
