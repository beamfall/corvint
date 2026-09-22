package gotest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestObservePassDigestTimeElapsed(t *testing.T) {
	input := lines(
		`{"Time":"2026-08-23T15:00:00.123456789-04:00","Action":"start","Package":"example/p"}`,
		`{"Action":"run","Package":"example/p","Test":"TestPass"}`,
		`{"Action":"output","Package":"example/p","Test":"TestPass","Output":"secret output\n","OutputType":"frame"}`,
		`{"Action":"pass","Package":"example/p","Test":"TestPass","Elapsed":1.000000001}`,
		`{"Action":"pass","Package":"example/p","Elapsed":0.001}`,
	)
	got, err := Observe(strings.NewReader(input), testConfig(t, 0, "example/p"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Events) != 5 || got.Events[0].Sequence != 0 || got.Events[4].Sequence != 4 {
		t.Fatalf("unexpected event sequence: %#v", got.Events)
	}
	var canonicalBytes uint64
	for _, event := range got.Events {
		body, bodyErr := event.CanonicalFactBody()
		if bodyErr != nil {
			t.Fatal(bodyErr)
		}
		canonicalBytes += uint64(len(body))
	}
	if got.EventBytes != canonicalBytes {
		t.Fatalf("event byte accounting is not canonical: got %d, want %d", got.EventBytes, canonicalBytes)
	}
	if got.Events[0].RawTime != "2026-08-23T15:00:00.123456789-04:00" || got.Events[0].TimeUTC.Format("2006-01-02T15:04:05.999999999Z07:00") != "2026-08-23T19:00:00.123456789Z" {
		t.Fatalf("time identity was not preserved and normalized: %#v", got.Events[0])
	}
	if got.Events[3].ElapsedNS != 1_000_000_001 || got.Events[4].ElapsedNS != 1_000_000 {
		t.Fatalf("elapsed was rounded: %d %d", got.Events[3].ElapsedNS, got.Events[4].ElapsedNS)
	}
	digest := sha256.Sum256([]byte("secret output\n"))
	if got.Events[2].OutputDigest != hex.EncodeToString(digest[:]) || got.Events[2].OutputBytes != 14 {
		t.Fatalf("wrong output evidence: %#v", got.Events[2])
	}
}

func TestObserveSubtestParallelMetadataSkipAndBench(t *testing.T) {
	config := testConfig(t, 0, "example/p")
	artifactDir := filepath.Join(config.ArtifactRoot, "artifacts")
	if err := os.Mkdir(artifactDir, 0o700); err != nil {
		t.Fatal(err)
	}
	input := lines(
		`{"Action":"start","Package":"example/p"}`,
		`{"Action":"run","Package":"example/p","Test":"TestParent"}`,
		`{"Action":"attr","Package":"example/p","Test":"TestParent","Key":"owner","Value":"corvint"}`,
		`{"Action":"artifacts","Package":"example/p","Test":"TestParent","Path":"`+artifactDir+`"}`,
		`{"Action":"run","Package":"example/p","Test":"TestParent/sub"}`,
		`{"Action":"pause","Package":"example/p","Test":"TestParent/sub"}`,
		`{"Action":"cont","Package":"example/p","Test":"TestParent/sub"}`,
		`{"Action":"skip","Package":"example/p","Test":"TestParent/sub","Elapsed":0}`,
		`{"Action":"pass","Package":"example/p","Test":"TestParent","Elapsed":0.01}`,
		`{"Action":"run","Package":"example/p","Test":"BenchmarkWork"}`,
		`{"Action":"bench","Package":"example/p","Test":"BenchmarkWork"}`,
		`{"Action":"pass","Package":"example/p","Elapsed":0.02}`,
	)
	got, err := Observe(strings.NewReader(input), config)
	if err != nil {
		t.Fatal(err)
	}
	if got.Packages[0].Status != "pass" || len(got.Packages[0].Tests) != 3 {
		t.Fatalf("wrong state: %#v", got.Packages)
	}
	if got.Events[2].Action != "attr" || got.Events[3].Action != "artifacts" {
		t.Fatalf("raw actions not preserved: %#v", got.Events)
	}
	keyDigest := sha256.Sum256([]byte("owner"))
	valueDigest := sha256.Sum256([]byte("corvint"))
	pathDigest := "4724378663f1d38977136b83e6d40d5684a6319a7be165381fe6d55c0d044595"
	if got.Events[2].Attribute == nil ||
		got.Events[2].Attribute.KeySHA256 != hex.EncodeToString(keyDigest[:]) ||
		got.Events[2].Attribute.ValueSHA256 != hex.EncodeToString(valueDigest[:]) {
		t.Fatalf("attribute was not reduced to digests: %#v", got.Events[2])
	}
	if got.Events[3].ArtifactPathSHA256 != pathDigest {
		t.Fatalf("artifact path was not reduced to a digest: %#v", got.Events[3])
	}
}

func TestObserveDoesNotRetainRawMetadata(t *testing.T) {
	eventType := reflect.TypeOf(Event{})
	for _, forbidden := range []string{"Key", "Value", "Path", "IdentityDigest"} {
		if _, exists := eventType.FieldByName(forbidden); exists {
			t.Fatalf("Event retains raw metadata field %q", forbidden)
		}
	}

	input := lines(
		`{"Action":"start","Package":"example/p"}`,
		`{"Action":"run","Package":"example/p","Test":"TestEmptyAttr"}`,
		`{"Action":"attr","Package":"example/p","Test":"TestEmptyAttr"}`,
		`{"Action":"pass","Package":"example/p","Test":"TestEmptyAttr","Elapsed":0}`,
		`{"Action":"pass","Package":"example/p","Elapsed":0}`,
	)
	got, err := Observe(strings.NewReader(input), testConfig(t, 0, "example/p"))
	if err != nil {
		t.Fatal(err)
	}
	emptyDigest := sha256.Sum256(nil)
	want := hex.EncodeToString(emptyDigest[:])
	if got.Events[2].Attribute == nil || got.Events[2].Attribute.KeySHA256 != want || got.Events[2].Attribute.ValueSHA256 != want {
		t.Fatalf("omitted attr values did not normalize as empty digests: %#v", got.Events[2])
	}
}

func TestErrorsNeverRenderRunnerControlledText(t *testing.T) {
	input := lines(`{"Action":"\u001b[2Jsecret-action","Package":"secret-package"}`)
	_, err := Observe(strings.NewReader(input), testConfig(t, 0, "secret-package"))
	var parseErr *Error
	if !errors.As(err, &parseErr) || parseErr.Failure != FailureUnknownAction {
		t.Fatalf("got %v", err)
	}
	if got, want := err.Error(), "go-test-json:unknown_action:line-1"; got != want {
		t.Fatalf("unstable or hostile error: got %q, want %q", got, want)
	}
	for _, forbidden := range []string{"secret-action", "secret-package", "\x1b"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("error leaked runner-controlled text %q: %q", forbidden, err)
		}
	}
	errorType := reflect.TypeOf(Error{})
	if _, exists := errorType.FieldByName("Detail"); exists {
		t.Fatal("Error retains a free-form detail field")
	}
}

func TestObserveBuildFailure(t *testing.T) {
	input := lines(
		`{"ImportPath":"example/bad","Action":"build-output","Output":"compile failed\n"}`,
		`{"ImportPath":"example/bad","Action":"build-fail"}`,
		`{"Action":"start","Package":"example/bad"}`,
		`{"Action":"output","Package":"example/bad","Output":"FAIL\texample/bad [build failed]\n","OutputType":"frame"}`,
		`{"Action":"fail","Package":"example/bad","Elapsed":0,"FailedBuild":"example/bad"}`,
	)
	got, err := Observe(strings.NewReader(input), testConfig(t, 1, "example/bad"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Builds) != 1 || got.Builds[0].Status != "fail" || got.Events[0].Kind != BuildEvent || got.Packages[0].FailedBuild != "example/bad" {
		t.Fatalf("wrong build state: %#v", got)
	}
}

func TestObservePanicAndNoTests(t *testing.T) {
	panicInput := lines(
		`{"Action":"start","Package":"example/p"}`,
		`{"Action":"run","Package":"example/p","Test":"TestPanic"}`,
		`{"Action":"output","Package":"example/p","Test":"TestPanic","Output":"panic: boom\n"}`,
		`{"Action":"fail","Package":"example/p","Test":"TestPanic","Elapsed":0}`,
		`{"Action":"fail","Package":"example/p","Elapsed":0}`,
	)
	if _, err := Observe(strings.NewReader(panicInput), testConfig(t, 1, "example/p")); err != nil {
		t.Fatal(err)
	}
	noTests := lines(
		`{"Action":"start","Package":"example/empty"}`,
		`{"Action":"output","Package":"example/empty","Output":"?   \texample/empty\t[no test files]\n"}`,
		`{"Action":"skip","Package":"example/empty","Elapsed":0}`,
	)
	if got, err := Observe(strings.NewReader(noTests), testConfig(t, 0, "example/empty")); err != nil || got.Packages[0].Status != "skip" {
		t.Fatalf("no-tests stream rejected: %#v %v", got, err)
	}
}

func TestObserveFailsClosed(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Failure
	}{
		{"unknown action", lines(`{"Action":"future","Package":"p"}`), FailureUnknownAction},
		{"unknown field", lines(`{"Action":"start","Package":"p","Future":true}`), FailureUnknownField},
		{"duplicate", lines(`{"Action":"start","Action":"pass","Package":"p"}`), FailureDuplicateKey},
		{"malformed", "{bad}\n", FailureMalformedJSON},
		{"missing lf", `{"Action":"start","Package":"p"}`, FailureMissingLF},
		{"missing package terminal", lines(`{"Action":"start","Package":"p"}`), FailureMissingTerminal},
		{"missing test terminal", lines(`{"Action":"start","Package":"p"}`, `{"Action":"run","Package":"p","Test":"TestX"}`, `{"Action":"pass","Package":"p","Elapsed":0}`), FailureMissingTerminal},
		{"conflicting package terminals", lines(`{"Action":"start","Package":"p"}`, `{"Action":"pass","Package":"p","Elapsed":0}`, `{"Action":"fail","Package":"p","Elapsed":0}`), FailureConflictingTerminal},
		{"conflicting test terminals", lines(`{"Action":"start","Package":"p"}`, `{"Action":"run","Package":"p","Test":"TestX"}`, `{"Action":"pass","Package":"p","Test":"TestX","Elapsed":0}`, `{"Action":"fail","Package":"p","Test":"TestX","Elapsed":0}`, `{"Action":"fail","Package":"p","Elapsed":0}`), FailureConflictingTerminal},
		{"sub nanosecond", lines(`{"Action":"start","Package":"p"}`, `{"Action":"pass","Package":"p","Elapsed":0.0000000001}`), FailureInvalidField},
		{"null output", lines(`{"Action":"start","Package":"p"}`, `{"Action":"output","Package":"p","Output":null}`, `{"Action":"pass","Package":"p","Elapsed":0}`), FailureInvalidField},
		{"output after test terminal", lines(`{"Action":"start","Package":"p"}`, `{"Action":"run","Package":"p","Test":"TestX"}`, `{"Action":"pass","Package":"p","Test":"TestX","Elapsed":0}`, `{"Action":"output","Package":"p","Test":"TestX","Output":"late"}`, `{"Action":"pass","Package":"p","Elapsed":0}`), FailureSequenceConflict},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exit := uint32(0)
			if strings.Contains(tc.input, `"Action":"fail"`) {
				exit = 1
			}
			_, err := Observe(strings.NewReader(tc.input), testConfig(t, exit, "p"))
			var parseErr *Error
			if !errors.As(err, &parseErr) || parseErr.Failure != tc.want {
				t.Fatalf("got %v, want failure %s", err, tc.want)
			}
		})
	}
}

func TestObserveRejectsOversizeLine(t *testing.T) {
	input := `{"Action":"output","Package":"p","Output":"` + strings.Repeat("x", MaxLineBytes) + `"}` + "\n"
	_, err := Observe(strings.NewReader(input), testConfig(t, 0, "p"))
	var parseErr *Error
	if !errors.As(err, &parseErr) || parseErr.Failure != FailureOversizeLine {
		t.Fatalf("got %v", err)
	}
}

func TestObservationContextAndTerminalCorrelation(t *testing.T) {
	t.Run("only requested package needs terminal", func(t *testing.T) {
		config := testConfig(t, 0, "requested", "dependency")
		config.RequestedPackages = []string{"requested"}
		got, err := Observe(strings.NewReader(lines(
			`{"Action":"start","Package":"dependency"}`,
			`{"Action":"start","Package":"requested"}`,
			`{"Action":"pass","Package":"requested","Elapsed":0}`,
		)), config)
		if err != nil || len(got.Packages) != 2 {
			t.Fatalf("non-requested package incorrectly required a terminal: %#v %v", got, err)
		}
	})

	tests := []struct {
		name   string
		input  string
		config func(*testing.T) Config
		want   Failure
	}{
		{
			name:   "event package absent from discovery",
			input:  lines(`{"Action":"start","Package":"other"}`),
			config: func(t *testing.T) Config { return testConfig(t, 0, "p") },
			want:   FailurePackageDiscovery,
		},
		{
			name:   "build absent from discovery",
			input:  lines(`{"Action":"build-fail","ImportPath":"other"}`),
			config: func(t *testing.T) Config { return testConfig(t, 1, "p") },
			want:   FailurePackageDiscovery,
		},
		{
			name: "failed build reference lacks build terminal",
			input: lines(
				`{"Action":"start","Package":"p"}`,
				`{"Action":"fail","Package":"p","Elapsed":0,"FailedBuild":"dep"}`,
			),
			config: func(t *testing.T) Config {
				config := testConfig(t, 1, "p", "dep")
				config.RequestedPackages = []string{"p"}
				return config
			},
			want: FailureBuildMismatch,
		},
		{
			name: "build failure lacks failed package reference",
			input: lines(
				`{"Action":"build-fail","ImportPath":"dep"}`,
				`{"Action":"start","Package":"p"}`,
				`{"Action":"fail","Package":"p","Elapsed":0}`,
			),
			config: func(t *testing.T) Config {
				config := testConfig(t, 1, "p", "dep")
				config.RequestedPackages = []string{"p"}
				return config
			},
			want: FailureBuildMismatch,
		},
		{
			name: "failure with zero exit",
			input: lines(
				`{"Action":"start","Package":"p"}`,
				`{"Action":"fail","Package":"p","Elapsed":0}`,
			),
			config: func(t *testing.T) Config { return testConfig(t, 0, "p") },
			want:   FailureProcessMismatch,
		},
		{
			name: "success with nonzero exit",
			input: lines(
				`{"Action":"start","Package":"p"}`,
				`{"Action":"pass","Package":"p","Elapsed":0}`,
			),
			config: func(t *testing.T) Config { return testConfig(t, 9, "p") },
			want:   FailureProcessMismatch,
		},
		{
			name: "unterminated process",
			input: lines(
				`{"Action":"start","Package":"p"}`,
				`{"Action":"pass","Package":"p","Elapsed":0}`,
			),
			config: func(t *testing.T) Config {
				config := testConfig(t, 0, "p")
				config.Process.Exited = false
				return config
			},
			want: FailureProcessMismatch,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Observe(strings.NewReader(tc.input), tc.config(t))
			var got *Error
			if !errors.As(err, &got) || got.Failure != tc.want {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}

func TestArtifactContainmentAndNormalizedDigest(t *testing.T) {
	config := testConfig(t, 0, "p")
	inside := filepath.Join(config.ArtifactRoot, "nested", "artifact")
	if err := os.MkdirAll(inside, 0o700); err != nil {
		t.Fatal(err)
	}
	input := lines(
		`{"Action":"start","Package":"p"}`,
		`{"Action":"run","Package":"p","Test":"TestX"}`,
		`{"Action":"artifacts","Package":"p","Test":"TestX","Path":"`+inside+`"}`,
		`{"Action":"pass","Package":"p","Test":"TestX","Elapsed":0}`,
		`{"Action":"pass","Package":"p","Elapsed":0}`,
	)
	got, err := Observe(strings.NewReader(input), config)
	if err != nil {
		t.Fatal(err)
	}
	if want := "35ba971a2a3f00eb2a20d3b254b65205691a1a6c392a870aaaeacc1076d222c5"; got.Events[2].ArtifactPathSHA256 != want {
		t.Fatalf("got digest %q, want normalized contained digest %q", got.Events[2].ArtifactPathSHA256, want)
	}

	outside := t.TempDir()
	rejectArtifactPath(t, config, outside)
	missing := filepath.Join(config.ArtifactRoot, "missing")
	rejectArtifactPath(t, config, missing)
	symlink := filepath.Join(config.ArtifactRoot, "linked")
	if err := os.Symlink(outside, symlink); err == nil {
		rejectArtifactPath(t, config, symlink)
	}
}

func TestConfigFailsClosed(t *testing.T) {
	valid := testConfig(t, 0, "p")
	tests := []Config{
		{},
		{DiscoveredPackages: []string{"p"}, RequestedPackages: []string{"p"}, ArtifactRoot: "relative", Process: ProcessOutcome{Exited: true}},
		{DiscoveredPackages: []string{"p", "p"}, RequestedPackages: []string{"p"}, ArtifactRoot: valid.ArtifactRoot, Process: ProcessOutcome{Exited: true}},
		{DiscoveredPackages: []string{"p"}, RequestedPackages: []string{"q"}, ArtifactRoot: valid.ArtifactRoot, Process: ProcessOutcome{Exited: true}},
	}
	for index, config := range tests {
		_, err := Observe(strings.NewReader(""), config)
		var got *Error
		if !errors.As(err, &got) || got.Failure != FailureConfig {
			t.Fatalf("case %d: got %v, want config failure", index, err)
		}
	}
}

func rejectArtifactPath(t *testing.T, config Config, path string) {
	t.Helper()
	input := lines(
		`{"Action":"start","Package":"p"}`,
		`{"Action":"run","Package":"p","Test":"TestX"}`,
		`{"Action":"artifacts","Package":"p","Test":"TestX","Path":"`+path+`"}`,
	)
	_, err := Observe(strings.NewReader(input), config)
	var got *Error
	if !errors.As(err, &got) || got.Failure != FailureArtifactContainment {
		t.Fatalf("path %q: got %v, want artifact containment", path, err)
	}
}

func TestGo127LiveOutputConforms(t *testing.T) {
	outputDir := t.TempDir()
	cmd := exec.Command("go", "test", "-count=1", "-json", "-artifacts", "-outputdir", outputDir, "./testdata/livefixture")
	cmd.Env = append(cmd.Environ(), "GOTOOLCHAIN=local")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("live Go fixture failed: %v", err)
	}
	got, err := Observe(bytes.NewReader(out), Config{
		DiscoveredPackages: []string{"github.com/Beamfall/corvint/internal/liveverify/gotest/testdata/livefixture"},
		RequestedPackages:  []string{"github.com/Beamfall/corvint/internal/liveverify/gotest/testdata/livefixture"},
		ArtifactRoot:       outputDir,
		Process:            ProcessOutcome{Exited: true},
	})
	if err != nil {
		t.Fatalf("Go 1.27 emitted an unsupported event stream; capture it before changing the decoder: %v\n%s", err, out)
	}
	if len(got.Packages) != 1 || got.Packages[0].Status != "pass" || len(got.Packages[0].Tests) < 4 {
		t.Fatalf("unexpected live state: %#v\n%s", got.Packages, out)
	}
	var attr, artifacts, pause, cont bool
	for _, event := range got.Events {
		switch event.Action {
		case "attr":
			attr = true
		case "artifacts":
			artifacts = true
		case "pause":
			pause = true
		case "cont":
			cont = true
		}
	}
	if !attr || !artifacts || !pause || !cont {
		t.Fatalf("live fixture missed Go 1.27 event classes: attr=%v artifacts=%v pause=%v cont=%v", attr, artifacts, pause, cont)
	}
}

func lines(lines ...string) string {
	for index, line := range lines {
		if strings.Contains(line, `"Package"`) && !strings.Contains(line, `"Time"`) && strings.HasPrefix(line, "{") {
			lines[index] = `{"Time":"2026-08-23T12:00:00Z",` + line[1:]
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func testConfig(t *testing.T, exitCode uint32, packages ...string) Config {
	t.Helper()
	return Config{
		DiscoveredPackages: append([]string(nil), packages...),
		RequestedPackages:  append([]string(nil), packages...),
		ArtifactRoot:       t.TempDir(),
		Process:            ProcessOutcome{Exited: true, ExitCode: exitCode},
	}
}
