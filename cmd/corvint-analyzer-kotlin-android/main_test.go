package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/analyzerkotlinandroid"
)

func TestCLIRejectsMalformedInput(t *testing.T) {
	var output bytes.Buffer
	if run(nil, bytes.NewBufferString("{}\n"), &output) != 0 || output.String() != "{\"profile\":\"corvint-analyzer-candidate/experimental\",\"family\":\"unknown\",\"request_id\":\"unknown\",\"status\":\"REJECTED\",\"reason\":\"NONCANONICAL_REQUEST\"}\n" {
		t.Fatalf("output=%q", output.String())
	}
}

func TestRunRejectsUnexpectedArgv(t *testing.T) {
	var output bytes.Buffer
	want := "{\"profile\":\"corvint-analyzer-candidate/experimental\",\"family\":\"unknown\",\"request_id\":\"unknown\",\"status\":\"REJECTED\",\"reason\":\"NONCANONICAL_REQUEST\"}\n"
	if run([]string{"--input-file", "request.json"}, bytes.NewBufferString("{}\n"), &output) != 0 || output.String() != want {
		t.Fatalf("output=%q", output.String())
	}
}

// TestRunReportsReadFailureAsNoncanonical pins ACP-012: a stdin read failure
// is the NONCANONICAL_REQUEST sentinel with exit 0.
func TestRunReportsReadFailureAsNoncanonical(t *testing.T) {
	var output bytes.Buffer
	want := "{\"profile\":\"corvint-analyzer-candidate/experimental\",\"family\":\"unknown\",\"request_id\":\"unknown\",\"status\":\"REJECTED\",\"reason\":\"NONCANONICAL_REQUEST\"}\n"
	if code := run(nil, failingReader{}, &output); code != 0 || output.String() != want {
		t.Fatalf("code=%d output=%q", code, output.String())
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, os.ErrClosed }

func TestCLICompletesPositiveProgressShortWrites(t *testing.T) {
	writer := &oneByteWriter{}
	if run(nil, bytes.NewBufferString("{}\n"), writer) != 0 {
		t.Fatal("run failed")
	}
	want := "{\"profile\":\"corvint-analyzer-candidate/experimental\",\"family\":\"unknown\",\"request_id\":\"unknown\",\"status\":\"REJECTED\",\"reason\":\"NONCANONICAL_REQUEST\"}\n"
	if writer.String() != want {
		t.Fatalf("output=%q", writer.String())
	}
}

func TestCLIRejectsZeroProgressShortWrite(t *testing.T) {
	if run(nil, bytes.NewBufferString("{}\n"), zeroWriter{}) != 2 {
		t.Fatal("short write accepted")
	}
}

func TestCLIPinnedSuccessBytes(t *testing.T) {
	raw := cliPinnedRequest(t)
	var first, second bytes.Buffer
	if run(nil, bytes.NewReader(raw), &first) != 0 || run(nil, bytes.NewReader(raw), &second) != 0 || first.String() != second.String() {
		t.Fatal("nondeterministic successful CLI bytes")
	}
	if !bytes.HasSuffix(first.Bytes(), []byte{'\n'}) {
		t.Fatal("missing LF")
	}
	sum := sha256.Sum256(first.Bytes())
	if got := hex.EncodeToString(sum[:]); got != "f731e5d6dbb7a1b63a00f22c1d334daa8e1430563ba103df2868ba1b5194b328" {
		t.Fatalf("success digest=%s", got)
	}
}

type oneByteWriter struct{ bytes.Buffer }

func (writer *oneByteWriter) Write(value []byte) (int, error) { return writer.Buffer.Write(value[:1]) }

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

func cliPinnedRequest(t *testing.T) []byte {
	t.Helper()
	type fixture struct{ handle, family, filePath, name string }
	fixtures := []fixture{
		{"a-project", "android.project.revision", "project.revision", ""},
		{"b-build", "android.gradle.build", "build.gradle.kts", "android-build.gradle.kts"},
		{"c-settings", "android.gradle.settings", "settings.gradle.kts", "android-settings.gradle.kts"},
		{"d-wrapper", "android.gradle.wrapper", "gradle/wrapper/gradle-wrapper.properties", "android-wrapper.properties"},
		{"e-version", "android.project.version", "gradle/version.properties", "android-version.properties"},
		{"f-catalog", "android.version.catalog", "gradle/libs.versions.toml", "android-libs.versions.toml"},
		{"g-module", "android.gradle.module", "app-mobile/build.gradle.kts", "android-app-mobile.gradle.kts"},
		{"h-source", "kotlin.source", "app-mobile/src/main/kotlin/com/beamfall/mobile/BuildIdentity.kt", "android-build-identity.kt"},
		{"i-ui-build", "android-ui.gradle.build", "ui/build.gradle.kts", "ui-build.gradle.kts"},
		{"j-ui-catalog", "android-ui.version.catalog", "ui/gradle/libs.versions.toml", "ui-libs.versions.toml"},
		{"k-ui-source", "android-ui.kotlin.source", "ui/kit/src/main/kotlin/com/beamfall/kit/BeamfallKit.kt", "ui-beamfall-kit.kt"},
		{"l-ui-revision", "android.ui.revision", "android-ui.revision", ""},
	}
	inputs := make([]analyzerkotlinandroid.Input, 0, len(fixtures))
	for _, fixture := range fixtures {
		content := []byte("52799c501cc291ff003d05f9eda391c2008cd51e\n")
		if fixture.family == "android.ui.revision" {
			content = []byte("6e379d7feec88128439d1753325bcfb22194fdfc\n")
		}
		if fixture.name != "" {
			var err error
			content, err = os.ReadFile("../../internal/analyzerkotlinandroid/testdata/" + fixture.name)
			if err != nil {
				t.Fatal(err)
			}
		}
		sum := sha256.Sum256(content)
		inputs = append(inputs, analyzerkotlinandroid.Input{Handle: fixture.handle, Family: fixture.family, Path: fixture.filePath, SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString(content)})
	}
	request := analyzerkotlinandroid.Request{Profile: analyzerkotlinandroid.Profile, Family: analyzerkotlinandroid.Family, RequestID: "request-1", ScopeID: "beamfall-android@52799c501cc291ff003d05f9eda391c2008cd51e", CompilationUnitID: "mobile-arm64", Target: analyzerkotlinandroid.Target{OS: "android", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: inputs}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}
