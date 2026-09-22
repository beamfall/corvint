package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const capturePacket = " {\"state\":\"READY\",\"results\":[],\"coverage\":{\"answerability\":{\"verdict\":\"unsupported-conjunction\",\"nearest_claims\":[{\"id\":\"tests/help_test.rs\",\"supports\":[\"quote\"],\"lacks\":[\"default\"]}]}}}\n"

// This regression uses only the pre-existing CLI seam so its RED proves the
// missing diagnostic behavior, rather than a missing implementation symbol.
func TestContextPacketsCLIExactUnsupported(t *testing.T) {
	configuration := fixtureOptions(t, fixtureSnapshot(t))
	script := "#!/bin/sh\nif [ \"$1\" = --version ]; then echo test; exit; fi\ncat <<'PACKET'\n" + capturePacket + "PACKET\n"
	if err := os.WriteFile(configuration.corvintGo, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(parent, "packets.jsonl")
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--samples", configuration.samples, "--snapshot", "clap-rs/clap@68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6=" + configuration.snapshots["clap-rs/clap@68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6"], "--corvint", configuration.corvintGo, "--arms", "context", "--max-samples", "1", "--registration", filepath.Join(t.TempDir(), "registration.json"), "--context-packets", destination}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("context diagnostics unavailable: exit=%d stderr=%s", code, &stderr)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
	if len(lines) != 3 {
		t.Fatalf("got %d lines", len(lines))
	}
	var record struct {
		Stdout struct {
			Base64 string `json:"base64"`
		} `json:"stdout"`
	}
	if err := json.Unmarshal(lines[1], &record); err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(record.Stdout.Base64)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, []byte(capturePacket)) {
		t.Fatalf("packet bytes lost: %q", raw)
	}
}

func captureFixture(t *testing.T, body string) options {
	t.Helper()
	configuration := fixtureOptions(t, fixtureSnapshot(t))
	configuration.arms = map[string]bool{"context": true}
	configuration.maxSamples = 1
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	configuration.registrationPath = filepath.Join(parent, "registration.json")
	configuration.contextPackets = filepath.Join(parent, "packets.jsonl")
	script := "#!/bin/sh\nif [ \"$1\" = --version ]; then echo test; exit; fi\n" + body
	if err := os.WriteFile(configuration.corvintGo, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return configuration
}

func packetScript(packet string) string { return "cat <<'PACKET'\n" + packet + "PACKET\n" }

func readCapture(t *testing.T, path string) ([]map[string]any, []captureRecord) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatal("incomplete JSONL")
	}
	lines := bytes.Split(data[:len(data)-1], []byte("\n"))
	objects := make([]map[string]any, len(lines))
	records := []captureRecord{}
	for i, line := range lines {
		if err := json.Unmarshal(line, &objects[i]); err != nil {
			t.Fatal(err)
		}
		if objects[i]["type"] == "invocation" {
			var record captureRecord
			if err := json.Unmarshal(line, &record); err != nil {
				t.Fatal(err)
			}
			records = append(records, record)
		}
	}
	footer := objects[len(objects)-1]
	if footer["type"] == "footer" {
		preceding := data[:len(data)-len(lines[len(lines)-1])-1]
		if footer["prior_sha256"] != captureHash(preceding) || footer["total_bytes"] != float64(len(data)) || footer["invocations"] != float64(len(records)) {
			t.Fatalf("invalid footer: %v", footer)
		}
	}
	return objects, records
}

func normalizeCaptureReport(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if key == "wall_ms" || key == "cold_wall_ms" || key == "registered_at" || key == "latency" {
				delete(typed, key)
				continue
			}
			typed[key] = normalizeCaptureReport(item)
		}
	case []any:
		for i, item := range typed {
			typed[i] = normalizeCaptureReport(item)
		}
	}
	return value
}

func TestRBDV0001CaptureReportParity(t *testing.T) {
	t.Run("RBD-V0-001", testRBDV0001CaptureReportParity)
}

func testRBDV0001CaptureReportParity(t *testing.T) {
	configuration := captureFixture(t, packetScript(capturePacket))
	enabled, err := bench(context.Background(), configuration, nil, runContext, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	configuration.contextPackets = ""
	configuration.registrationPath = filepath.Join(t.TempDir(), "disabled-registration.json")
	disabled, err := bench(context.Background(), configuration, nil, runContext, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	encode := func(report map[string]any) []byte {
		data, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		var value any
		if err := json.Unmarshal(data, &value); err != nil {
			t.Fatal(err)
		}
		data, err = json.Marshal(normalizeCaptureReport(value))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	if !bytes.Equal(encode(enabled), encode(disabled)) {
		t.Fatalf("report or scorer inputs changed:\n%s\n%s", encode(enabled), encode(disabled))
	}
}

func TestRBDV0004ExactStreamsAndParseStatus(t *testing.T) {
	t.Run("RBD-V0-004", testRBDV0004ExactStreamsAndParseStatus)
}

func testRBDV0004ExactStreamsAndParseStatus(t *testing.T) {
	cases := []struct{ name, body, raw, parse, stdoutStatus, stderrStatus, footer string }{
		{"unsupported", packetScript(capturePacket), capturePacket, "PARSED", "COMPLETE", "COMPLETE", "COMPLETE"},
		{"empty", "exit 0\n", "", "MALFORMED", "COMPLETE", "COMPLETE", "COMPLETE"},
		{"malformed", "printf 'broken'\n", "broken", "MALFORMED", "COMPLETE", "COMPLETE", "COMPLETE"},
		{"failed", "printf 'prefix'; printf 'diagnostic' >&2; exit 3\n", "", "NOT_RUN", "NOT_PRODUCED", "COMPLETE", "PARTIAL"},
		{"stderr-truncated", "head -c 65537 /dev/zero >&2\n" + packetScript(capturePacket), capturePacket, "PARSED", "COMPLETE", "TRUNCATED", "COMPLETE"},
		{"stdout-overflow", "head -c 8388609 /dev/zero\n", "", "NOT_RUN", "NOT_PRODUCED", "COMPLETE", "PARTIAL"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			configuration := captureFixture(t, test.body)
			report, err := bench(context.Background(), configuration, nil, runContext, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			objects, records := readCapture(t, configuration.contextPackets)
			if len(records) != 1 {
				t.Fatalf("records=%d", len(records))
			}
			record := records[0]
			if record.ParseStatus != test.parse || record.Stdout.Status != test.stdoutStatus || record.Stderr.Status != test.stderrStatus || objects[len(objects)-1]["status"] != test.footer {
				t.Fatalf("record=%+v footer=%v", record, objects[len(objects)-1])
			}
			if test.stdoutStatus == "NOT_PRODUCED" {
				if record.Stdout.Base64 != nil || record.Stdout.Bytes != nil || record.Stdout.SHA256 != "" {
					t.Fatal("failure prefix represented as complete stdout")
				}
				return
			}
			raw, err := base64.StdEncoding.DecodeString(*record.Stdout.Base64)
			if err != nil {
				t.Fatal(err)
			}
			if string(raw) != test.raw || *record.Stdout.Bytes != len(raw) || record.Stdout.SHA256 != captureHash(raw) {
				t.Fatal("stream identity changed")
			}
			replay, parseErr := parseContextPacket(raw)
			observed := report["details"].([]sampleReport)[0].Arms["context"]
			if parseErr == nil && (replay.Abstained != observed.Abstained || replay.State != observed.State || replay.PacketBytes != observed.PacketBytes) {
				t.Fatalf("parser replay=%+v report=%+v", replay, observed)
			}
			if parseErr != nil && observed.Error == "" {
				t.Fatal("parser failure lost")
			}
			stderr, err := base64.StdEncoding.DecodeString(*record.Stderr.Base64)
			if err != nil {
				t.Fatal(err)
			}
			if *record.Stderr.Bytes != len(stderr) || record.Stderr.SHA256 != captureHash(stderr) {
				t.Fatal("stderr identity lost")
			}
			if test.stderrStatus == "TRUNCATED" && len(stderr) != maxErrorBytes {
				t.Fatal("stderr bound changed")
			}
		})
	}
}

func TestRBDV0003ColdHitDescriptors(t *testing.T) {
	t.Run("RBD-V0-003", testRBDV0003ColdHitDescriptors)
}

func testRBDV0003ColdHitDescriptors(t *testing.T) {
	body := "if [ \"$3\" = index ]; then mkdir -p \"$2/.corvint/index\"; printf '*\\n' > \"$2/.corvint/.gitignore\"; exit; fi\nif [ -d \"$2/.corvint/index\" ]; then echo 'corvint-bench-snapshot: hit=true' >&2; else echo 'corvint-bench-snapshot: hit=false' >&2; fi\n" + packetScript(capturePacket)
	configuration := captureFixture(t, body)
	configuration.snapshotLatency = true
	configuration.maxSamples = 2
	// Duplicate IDs must retain separate ordinal identities.
	first := strings.Split(fixtureSamples, "\n")[0] + "\n"
	if err := os.WriteFile(configuration.samples, []byte(first+first), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := bench(context.Background(), configuration, nil, runContext, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	objects, records := readCapture(t, configuration.contextPackets)
	header := objects[0]
	registered, err := json.Marshal(report["registration"])
	if err != nil {
		t.Fatal(err)
	}
	bound, err := json.Marshal(header["registration"])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(registered, bound) {
		t.Fatal("registration binding differs")
	}
	samples, _, _, err := readSamples(configuration)
	if err != nil {
		t.Fatal(err)
	}
	for i, record := range records {
		phase := []string{"cold", "hit"}[i%2]
		if record.Invocation != describeCapture(samples[i/2], i/2, i, phase) {
			t.Fatalf("descriptor=%+v", record.Invocation)
		}
	}
	if len(records) != 4 || objects[len(objects)-1]["status"] != "COMPLETE" {
		t.Fatal("missing cold/hit captures")
	}
	info, err := os.Stat(configuration.contextPackets)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
}

func TestRBDV0002RefusesUnsafeDestinationsBeforeRegistration(t *testing.T) {
	t.Run("RBD-V0-002", testRBDV0002RefusesUnsafeDestinationsBeforeRegistration)
}

func testRBDV0002RefusesUnsafeDestinationsBeforeRegistration(t *testing.T) {
	for _, kind := range []string{"existing", "samples", "registration", "report", "snapshot", "corpus", "symlink", "parent-symlink"} {
		t.Run(kind, func(t *testing.T) {
			configuration := captureFixture(t, packetScript(capturePacket))
			original := configuration.contextPackets
			source := configuration.snapshots["clap-rs/clap@68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6"]
			source, err := filepath.EvalSymlinks(source)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "existing":
				if err := os.WriteFile(original, []byte("preserved"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "samples":
				configuration.contextPackets = configuration.samples
			case "registration":
				configuration.contextPackets = configuration.registrationPath
			case "report":
				configuration.output = configuration.contextPackets
			case "snapshot":
				configuration.contextPackets = filepath.Join(source, "packets.jsonl")
			case "corpus":
				configuration.corpus = source
				configuration.contextPackets = filepath.Join(source, "packets.jsonl")
			case "symlink":
				if err := os.Symlink(filepath.Join(filepath.Dir(original), "other"), original); err != nil {
					t.Fatal(err)
				}
			case "parent-symlink":
				link := filepath.Join(filepath.Dir(original), "link")
				if err := os.Symlink(source, link); err != nil {
					t.Fatal(err)
				}
				configuration.contextPackets = filepath.Join(link, "packets.jsonl")
			}
			if _, err := bench(context.Background(), configuration, nil, runContext, nil, nil); err == nil {
				t.Fatal("unsafe capture accepted")
			}
			if _, err := os.Stat(configuration.registrationPath); !os.IsNotExist(err) {
				t.Fatalf("wrote registration before preflight: %v", err)
			}
			if kind == "existing" {
				data, err := os.ReadFile(original)
				if err != nil || string(data) != "preserved" {
					t.Fatal("existing capture changed")
				}
			}
		})
	}
}

func TestRBDV0001RefusesIncompatibleCLI(t *testing.T) {
	for _, args := range [][]string{
		{"--context-packets", "capture", "--summarize", "old"},
		{"--context-packets", "capture", "--samples", "samples", "--corpus", "corpus", "--arms", "grep", "--registration", "registration"},
		{"--context-packets", "capture", "--samples", "samples", "--corpus", "corpus", "--arms", "context"},
	} {
		if _, err := parseOptions(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestRBDV0005CaptureBounds(t *testing.T) {
	t.Run("RBD-V0-005", testRBDV0005CaptureBounds)
}

func testRBDV0005CaptureBounds(t *testing.T) {
	t.Run("count", func(t *testing.T) {
		configuration := captureFixture(t, packetScript(capturePacket))
		if _, err := planContextCapture(configuration, make([]sample, 1025)); err == nil {
			t.Fatal("accepted 1025 invocations")
		}
		configuration.snapshotLatency = true
		if _, err := planContextCapture(configuration, make([]sample, 513)); err == nil {
			t.Fatal("accepted 1026 cold/hit invocations")
		}
	})
	t.Run("header", func(t *testing.T) {
		configuration := captureFixture(t, packetScript(capturePacket))
		expected := []captureDescriptor{{SampleID: strings.Repeat("x", maxCaptureHeader)}}
		if _, err := openContextCapture(configuration, expected, map[string]any{}); err == nil {
			t.Fatal("accepted oversized header")
		}
		if _, err := os.Stat(configuration.contextPackets); !os.IsNotExist(err) {
			t.Fatal("opened oversized capture")
		}
	})
	for _, kind := range []string{"metadata", "encoded-file", "descriptor", "task", "write"} {
		t.Run(kind, func(t *testing.T) {
			configuration := captureFixture(t, packetScript(capturePacket))
			item := sample{ID: "sample"}
			if kind == "metadata" {
				item.ID = strings.Repeat("x", maxCaptureMetadata)
			}
			descriptor := describeCapture(item, 0, 0, "ordinary")
			recorder, err := openContextCapture(configuration, []captureDescriptor{descriptor}, map[string]any{})
			if err != nil {
				t.Fatal(err)
			}
			defer recorder.file.Close()
			observation := &captureObservation{descriptor: descriptor, task: queryText(item), observed: true, stdout: []byte(capturePacket)}
			switch kind {
			case "encoded-file":
				recorder.bytes = maxCaptureBytes - captureFooterReserve - 1
			case "descriptor":
				observation.descriptor.Phase = "hit"
			case "task":
				observation.task = "different task"
			case "write":
				if err := recorder.file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := recorder.append(observation); err == nil {
				t.Fatalf("accepted %s", kind)
			}
			objects, records := readCapture(t, configuration.contextPackets)
			if len(records) != 0 || len(objects) != 1 {
				t.Fatal("wrote invalid complete line")
			}
		})
	}
}

func TestRBDV0007IncompleteOnFailureAndCancellation(t *testing.T) {
	t.Run("RBD-V0-007", testRBDV0007IncompleteOnFailureAndCancellation)
}

func testRBDV0007IncompleteOnFailureAndCancellation(t *testing.T) {
	for _, kind := range []string{"canceled", "write", "identity", "cold-error"} {
		t.Run(kind, func(t *testing.T) {
			configuration := captureFixture(t, packetScript(capturePacket))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			retrieve := func(ctx context.Context, binary, root, task string, limit int) (arm, error) {
				if kind == "canceled" {
					cancel()
				}
				if kind == "write" {
					scope := ctx.Value(captureScopeKey{}).(*captureScope)
					if err := scope.recorder.file.Close(); err != nil {
						t.Fatal(err)
					}
				}
				result, err := runContext(ctx, binary, root, task, limit)
				if kind == "identity" {
					if err := os.WriteFile(binary, []byte("#!/bin/sh\necho changed\n"), 0o755); err != nil {
						t.Fatal(err)
					}
				}
				return result, err
			}
			if kind == "cold-error" {
				configuration.snapshotLatency = true
			}
			report, err := bench(ctx, configuration, nil, retrieve, nil, nil)
			if err == nil || report != nil {
				t.Fatal("failure published report")
			}
			objects, _ := readCapture(t, configuration.contextPackets)
			if objects[len(objects)-1]["type"] == "footer" {
				t.Fatal("failed run emitted completeness footer")
			}
		})
	}
}

func TestRBDV0006WritesAfterTimedRetrieval(t *testing.T) {
	t.Run("RBD-V0-006", testRBDV0006WritesAfterTimedRetrieval)
}

func testRBDV0006WritesAfterTimedRetrieval(t *testing.T) {
	configuration := captureFixture(t, packetScript(capturePacket))
	retrieve := func(ctx context.Context, binary, root, task string, limit int) (arm, error) {
		before, err := os.ReadFile(configuration.contextPackets)
		if err != nil {
			t.Fatal(err)
		}
		result, runErr := runContext(ctx, binary, root, task, limit)
		after, err := os.ReadFile(configuration.contextPackets)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("diagnostic bytes written inside retrieval span")
		}
		return result, runErr
	}
	if _, err := bench(context.Background(), configuration, nil, retrieve, nil, nil); err != nil {
		t.Fatal(err)
	}
	_, records := readCapture(t, configuration.contextPackets)
	if len(records) != 1 {
		t.Fatal("did not flush after retrieval")
	}
}

func TestRBDV0007ReportSinkFailureKeepsCapture(t *testing.T) {
	configuration := captureFixture(t, packetScript(capturePacket))
	snapshot := configuration.snapshots["clap-rs/clap@68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6"]
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--samples", configuration.samples, "--snapshot", "clap-rs/clap@68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6=" + snapshot, "--corvint", configuration.corvintGo, "--arms", "context", "--max-samples", "1", "--registration", configuration.registrationPath, "--context-packets", configuration.contextPackets, "--output", t.TempDir()}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "cannot write the report") {
		t.Fatalf("code=%d err=%s", code, &stderr)
	}
	objects, _ := readCapture(t, configuration.contextPackets)
	if objects[len(objects)-1]["status"] != "COMPLETE" {
		t.Fatal("report sink erased completed capture")
	}
}

func TestRBDV0004DeadlineDoesNotInventStdout(t *testing.T) {
	configuration := captureFixture(t, "printf prefix\n")
	item := sample{}
	descriptor := describeCapture(item, 0, 0, "ordinary")
	recorder, err := openContextCapture(configuration, []captureDescriptor{descriptor}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	defer recorder.file.Close()
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()
	ctx = context.WithValue(ctx, captureScopeKey{}, &captureScope{recorder, 0})
	observedCtx, observation := beginContextCapture(ctx, item, "ordinary")
	if _, err := runContext(observedCtx, configuration.corvintGo, "unused", queryText(item), 5); err == nil {
		t.Fatal("deadline ignored")
	}
	if err := flushContextCapture(ctx, observation); err != nil {
		t.Fatal(err)
	}
	_, records := readCapture(t, configuration.contextPackets)
	if len(records) != 1 || records[0].Stdout.Status != "NOT_PRODUCED" || records[0].Stdout.SHA256 != "" || records[0].Stdout.Base64 != nil {
		t.Fatal("deadline capture invented stdout")
	}
}

func TestRBDV0002CaseInsensitiveFilesystemAliases(t *testing.T) {
	t.Run("RBD-V0-002", func(t *testing.T) {
		parent, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		probe := filepath.Join(parent, "CaseProbe")
		if err := os.WriteFile(probe, []byte("probe"), 0o600); err != nil {
			t.Fatal(err)
		}
		original, err := os.Stat(probe)
		if err != nil {
			t.Fatal(err)
		}
		alias, err := os.Stat(filepath.Join(parent, "caseprobe"))
		if err != nil || !os.SameFile(original, alias) {
			t.Skip("requires an actual case-insensitive filesystem")
		}
		for _, kind := range []string{"report", "snapshot", "corpus"} {
			t.Run(kind, func(t *testing.T) {
				configuration := captureFixture(t, packetScript(capturePacket))
				snapshot := configuration.snapshots["clap-rs/clap@68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6"]
				if kind == "report" {
					configuration.contextPackets = filepath.Join(filepath.Dir(configuration.contextPackets), "Packets.jsonl")
					output := filepath.Join(filepath.Dir(configuration.contextPackets), "packets.jsonl")
					var stdout, stderr bytes.Buffer
					code := run(context.Background(), []string{"--samples", configuration.samples, "--snapshot", "clap-rs/clap@68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6=" + snapshot, "--corvint", configuration.corvintGo, "--arms", "context", "--max-samples", "1", "--registration", configuration.registrationPath, "--context-packets", configuration.contextPackets, "--output", output}, &stdout, &stderr)
					if code == 0 {
						t.Fatal("report alias silently replaced the capture and returned success")
					}
					if stdout.Len() != 0 {
						t.Fatal("published report after alias failure")
					}
					return
				}
				snapshot, err = filepath.EvalSymlinks(snapshot)
				if err != nil {
					t.Fatal(err)
				}
				named := snapshot + "Snapshot"
				if err := os.Rename(snapshot, named); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Rename(named, snapshot) })
				configuration.snapshots["clap-rs/clap@68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6"] = named
				if kind == "corpus" {
					configuration.corpus = named
					configuration.snapshots = map[string]string{}
				}
				configuration.contextPackets = filepath.Join(snapshot+"snapshot", "packets.jsonl")
				if _, err := bench(context.Background(), configuration, nil, runContext, nil, nil); err == nil {
					t.Fatal("accepted capture inside a protected directory alias")
				}
				if _, err := os.Stat(filepath.Join(named, "packets.jsonl")); !os.IsNotExist(err) {
					t.Fatalf("created capture inside source: %v", err)
				}
			})
		}
	})
}

func TestRBDV0002LateReportHardlinkPreservesCapture(t *testing.T) {
	t.Run("RBD-V0-002", func(t *testing.T) {
		configuration := captureFixture(t, packetScript(capturePacket))
		output := filepath.Join(filepath.Dir(configuration.contextPackets), "report.json")
		shellQuote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
		script := "#!/bin/sh\nif [ \"$1\" = --version ]; then echo test; exit; fi\nln " + shellQuote(configuration.contextPackets) + " " + shellQuote(output) + "\n" + packetScript(capturePacket)
		if err := os.WriteFile(configuration.corvintGo, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		snapshot := configuration.snapshots["clap-rs/clap@68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6"]
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"--samples", configuration.samples, "--snapshot", "clap-rs/clap@68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6=" + snapshot, "--corvint", configuration.corvintGo, "--arms", "context", "--max-samples", "1", "--registration", configuration.registrationPath, "--context-packets", configuration.contextPackets, "--output", output}, &stdout, &stderr)
		if code == 0 || !strings.Contains(stderr.String(), "cannot write the report") {
			t.Fatalf("late hardlink accepted: code=%d stderr=%s", code, &stderr)
		}
		objects, records := readCapture(t, configuration.contextPackets)
		if objects[len(objects)-1]["status"] != "COMPLETE" || len(records) != 1 {
			t.Fatal("late report alias destroyed completed capture")
		}
		raw, err := base64.StdEncoding.DecodeString(*records[0].Stdout.Base64)
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != capturePacket {
			t.Fatal("late alias replaced packet bytes")
		}
		captured, err := os.Stat(configuration.contextPackets)
		if err != nil {
			t.Fatal(err)
		}
		reported, err := os.Stat(output)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(captured, reported) {
			t.Fatal("regression did not exercise an actual hardlink")
		}
	})
}

func TestRBDV0002RejectsStaleHeldParent(t *testing.T) {
	t.Run("RBD-V0-002", func(t *testing.T) {
		configuration := captureFixture(t, packetScript(capturePacket))
		if err := validateCapturePath(configuration); err != nil {
			t.Fatal(err)
		}
		directory := filepath.Dir(configuration.contextPackets)
		parked := directory + "-parked"
		snapshot := configuration.snapshots["clap-rs/clap@68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6"]
		if err := os.Rename(directory, parked); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := os.Stat(parked); err == nil {
				_ = os.Remove(directory)
				_ = os.Rename(parked, directory)
			}
		})
		if err := os.Symlink(snapshot, directory); err != nil {
			t.Fatal(err)
		}
		// Acquire the parent in the old validation-to-open window, then restore the
		// innocent pathname. Only the held parent identity reveals the redirection.
		parent, err := os.OpenRoot(directory)
		if err != nil {
			t.Fatal(err)
		}
		defer parent.Close()
		if err := os.Remove(directory); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(parked, directory); err != nil {
			t.Fatal(err)
		}
		file, err := openCaptureFile(parent, configuration)
		if file != nil {
			file.Close()
		}
		if err == nil {
			t.Fatal("stale held parent created capture inside protected snapshot")
		}
		if _, err := os.Stat(filepath.Join(snapshot, filepath.Base(configuration.contextPackets))); !os.IsNotExist(err) {
			t.Fatalf("wrote into protected snapshot: %v", err)
		}
		if _, err := os.Stat(configuration.contextPackets); !os.IsNotExist(err) {
			t.Fatalf("created capture after parent identity mismatch: %v", err)
		}
	})
}
