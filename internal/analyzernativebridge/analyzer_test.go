package analyzernativebridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// These receipts pin the active Beamfall source corpus. The candidate never opens these paths:
// callers supply immutable bytes, and this test table is the only local provenance record.
type corpusReceipt struct {
	repository, revision, path, blob, sha256, family, profile, inputFamily, status, marker string
	bytes                                                                                  int
}

var beamfallCorpus = []corpusReceipt{
	{"beamfall-android-ui", "6e379d7feec88128439d1753325bcfb22194fdfc", "kit/src/main/cpp/beamfall_mpv.c", "c92c60b1f2cb1bf098e0899f220a7f282995a9e1", "6a6d71a89204ec3856d2f38db5efe217c28bd2fef94b5dde61ceec12b7c0a7c0", "c", CProfile, "c.source", "CANDIDATE", "c.jni.symbol", 10922},
	{"beamfall-android-ui", "6e379d7feec88128439d1753325bcfb22194fdfc", "kit/src/main/cpp/CMakeLists.txt", "20276872459adc947f16f6214a67983b44559bb8", "4de6a87f04d0bbf32c2a984c9d0437f422f6b9c9af452fd94379e03aa5e86edc", "c", CProfile, "c.cmake", "REJECTED", "UNSUPPORTED_SCHEMA", 1719},
	{"beamfall-apple-ui", "830a154d1d8f2f0b1b801a9a9a3b626bc74f6aa6", "Tests/BeamfallTestWatchdog/BeamfallTestWatchdog.c", "7a870b1ad818989a8255c35f8aa9c0d3afcd6cc2", "e3a67a5684e8acf32c9c7111d4fbf1f870ad80990037deed7055276773c9bbda", "c", CProfile, "c.source", "CANDIDATE", "c.include", 2956},
	{"beamfall-apple-ui", "830a154d1d8f2f0b1b801a9a9a3b626bc74f6aa6", "Tests/BeamfallTestWatchdog/BeamfallTestCaseWatchdog.m", "5c83304d23af5220cb0b4fbaada2f9dbba630d15", "c71025303b0bbf5832c363f29465e46e90880b2a3ce6255d79dc2353e8a0121f", "objective-c", ObjectiveCProfile, "objc.source", "CANDIDATE", "objc.interface", 4817},
	{"beamfall-apple-ui", "830a154d1d8f2f0b1b801a9a9a3b626bc74f6aa6", "Tests/BeamfallTestWatchdog/include/BeamfallTestWatchdog.h", "232f92089832582d9c3a08419e6a68f2f2919af5", "59e69b89a05acbf296a53ac2904a3ce3658fd82020d76db96fec5c2143d3211c", "c", CProfile, "c.header", "REJECTED", "UNSUPPORTED_SCHEMA", 1167},
	{"beamfall-apple-ui", "830a154d1d8f2f0b1b801a9a9a3b626bc74f6aa6", "Sources/BeamfallAppleLab/Shaders/VisualShared.h", "7f474f22e5a5461a35e7d4a989afcecde75eac6c", "625e8e091d565a652ffc61ecf86b726a733b150f0bc1888e062c2001b00ae3d6", "c", CProfile, "c.header", "CANDIDATE", "c.macro", 34543},
	{"beamfall-apple-ui", "830a154d1d8f2f0b1b801a9a9a3b626bc74f6aa6", "Sources/BeamfallVisualKit/Shaders/VisualShared.h", "1fead4291acfbef1062261d736d4aaf494a2c068", "cc172efd1de4c7a3a4e0b72af89cecc728d7e24afb9af55ab75da39131692dce", "c", CProfile, "c.header", "CANDIDATE", "c.macro", 34041},
	{"beamfall-android-ui", "6e379d7feec88128439d1753325bcfb22194fdfc", "kit-test-entitlement-stub/src/main/java/com/beamfall/kit/test/entitlement/TestModeEntitlementMint.java", "4eaffcfe5c1cff0b619603850b9958c85ba36c96", "9b03d4bb0ce070c9f2d96a054371c90881a20d2ff868255ce3d6de403a655ecc", "java", JavaProfile, "java.source", "CANDIDATE", "java.class", 3355},
	{"beamfall-android", "52799c501cc291ff003d05f9eda391c2008cd51e", "decoder-ffmpeg/src/androidTest/java/androidx/media3/decoder/ffmpeg/FfmpegAudioDecoder.java", "efd01c98f58f3933a510db4d417111ab1b5e679f", "0813cb7b59798b705762d96560865d469243a384a03f3766cc8da26abab0d035", "java", JavaProfile, "java.source", "CANDIDATE", "java.class", 5683},
	{"beamfall-android", "52799c501cc291ff003d05f9eda391c2008cd51e", "decoder-ffmpeg/src/androidTest/java/androidx/media3/decoder/ffmpeg/FfmpegAudioRenderer.java", "cbf8a5a96a34488a3b94f995103d5a273e3a0900", "aaef8a857adfcbb57973f1798f56de025f48822f8f750c16c6443f4f6c0b8b30", "java", JavaProfile, "java.source", "CANDIDATE", "java.class", 4995},
	{"beamfall-android", "52799c501cc291ff003d05f9eda391c2008cd51e", "decoder-ffmpeg/src/androidTest/java/androidx/media3/decoder/ffmpeg/FfmpegDecoderException.java", "bdc0c1b3b461e2ffae7266f7167bc07e46fa71a3", "323be6dbd6917c3d1637f3d887b01cef77010f852c2a655b6828656f3ba6fe4e", "java", JavaProfile, "java.source", "CANDIDATE", "java.class", 597},
	{"beamfall-android", "52799c501cc291ff003d05f9eda391c2008cd51e", "decoder-ffmpeg/src/androidTest/java/androidx/media3/decoder/ffmpeg/FfmpegLibrary.java", "1e887d1c1f05f755b2806619c3893f41f48657f0", "cf0611b1bbfb6a14570229c88b5e08e4d458c2de1f4ccf7184e02f217815d7c2", "java", JavaProfile, "java.source", "CANDIDATE", "java.native.method", 1841},
}

const beamfallC = `#include <jni.h>
#include <mpv/client.h>
#define LOG_TAG "BeamfallMpv"
JNIEXPORT jlong JNICALL
Java_com_beamfall_kit_LibmpvNativeBridge_nativeCreate(JNIEnv *env, jclass clazz) {
    return 0;
}
`
const beamfallObjectiveC = `#import "BeamfallTestWatchdog.h"
#import <XCTest/XCTest.h>
@interface BeamfallTestCaseWatchdog : NSObject <XCTestObservation>
- (instancetype)initWithBudgetSeconds:(long)budgetSeconds;
@end
@implementation BeamfallTestCaseWatchdog
- (void)testCaseDidFinish:(XCTestCase *)testCase { }
@end
`
const beamfallJava = `package androidx.media3.decoder.ffmpeg;
import androidx.annotation.Nullable;
public final class FfmpegLibrary {
    static { System.loadLibrary("ffmpegJNI"); }
    public static native String ffmpegGetVersion();
}
`

func testAnalyze(profile string, reader io.Reader) []byte {
	data, err := io.ReadAll(reader)
	if err != nil {
		panic(err)
	}
	var req request
	if err := json.Unmarshal(data, &req); err != nil {
		panic(err)
	}
	switch req.Family {
	case "c":
		return AnalyzeC(bytes.NewReader(data))
	case "objective-c":
		return AnalyzeObjectiveC(bytes.NewReader(data))
	case "java":
		return AnalyzeJava(bytes.NewReader(data))
	default:
		panic("unknown test family for " + profile)
	}
}

func TestPinnedBeamfallCorpusFixtures(t *testing.T) {
	if len(beamfallCorpus) != 12 {
		t.Fatalf("corpus entries=%d", len(beamfallCorpus))
	}
	binaries := map[string]string{}
	for _, candidate := range candidateExecutables() {
		binaries[candidate.family] = buildCandidate(t, candidate)
	}
	for _, receipt := range beamfallCorpus {
		if len(receipt.revision) != 40 || !strings.Contains(receipt.path, "/") || len(receipt.blob) != 40 || len(receipt.sha256) != 64 || receipt.bytes == 0 {
			t.Fatalf("invalid receipt=%+v", receipt)
		}
		body := pinnedBlob(t, receipt)
		encoded := runCLI(t, binaries[receipt.family], frameCorpus(receipt, body))
		if receipt.status == "CANDIDATE" {
			var got output
			if err := json.Unmarshal(encoded, &got); err != nil || got.Status != receipt.status {
				t.Errorf("%s status=%q err=%v output=%s", receipt.path, got.Status, err, encoded)
				continue
			}
			found := false
			for _, item := range got.Facts {
				if item.Kind == receipt.marker {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s missing marker=%q output=%s", receipt.path, receipt.marker, encoded)
			}
			continue
		}
		var got rejection
		if err := json.Unmarshal(encoded, &got); err != nil || got.Status != receipt.status || got.Reason != receipt.marker {
			t.Errorf("%s status=%q reason=%q err=%v output=%s", receipt.path, got.Status, got.Reason, err, encoded)
		}
	}
}

func frameCorpus(receipt corpusReceipt, body []byte) []byte {
	req := request{
		Profile: receipt.profile, Family: receipt.family, RequestID: "beamfall-corpus",
		ScopeID: "beamfall", CompilationUnitID: "bridge", Target: candidateTarget(receipt.family),
		Inputs: []input{{Handle: "source", Family: receipt.inputFamily, Path: receipt.path,
			SHA256: "sha256:" + receipt.sha256, ContentBase64: base64.StdEncoding.EncodeToString(body)}},
	}
	data, _ := json.Marshal(req)
	return append(data, '\n')
}

func pinnedBlob(t testing.TB, receipt corpusReceipt) []byte {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source location unavailable")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(source), "testdata", "beamfall-corpus", fixtureName(receipt)))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	if len(body) != receipt.bytes || hex.EncodeToString(sum[:]) != receipt.sha256 {
		t.Fatalf("bytes=%d sha256=%x", len(body), sum)
	}
	return body
}
func fixtureName(receipt corpusReceipt) string {
	return map[string]string{
		"kit/src/main/cpp/beamfall_mpv.c":                                                                        "android-jni.c",
		"kit/src/main/cpp/CMakeLists.txt":                                                                        "android-jni.CMakeLists.txt",
		"Tests/BeamfallTestWatchdog/BeamfallTestWatchdog.c":                                                      "apple-watchdog.c",
		"Tests/BeamfallTestWatchdog/BeamfallTestCaseWatchdog.m":                                                  "apple-watchdog.m",
		"Tests/BeamfallTestWatchdog/include/BeamfallTestWatchdog.h":                                              "apple-watchdog.h",
		"Sources/BeamfallAppleLab/Shaders/VisualShared.h":                                                        "apple-lab-VisualShared.h",
		"Sources/BeamfallVisualKit/Shaders/VisualShared.h":                                                       "apple-kit-VisualShared.h",
		"kit-test-entitlement-stub/src/main/java/com/beamfall/kit/test/entitlement/TestModeEntitlementMint.java": "android-ui-TestModeEntitlementMint.java",
		"decoder-ffmpeg/src/androidTest/java/androidx/media3/decoder/ffmpeg/FfmpegAudioDecoder.java":             "android-FfmpegAudioDecoder.java",
		"decoder-ffmpeg/src/androidTest/java/androidx/media3/decoder/ffmpeg/FfmpegAudioRenderer.java":            "android-FfmpegAudioRenderer.java",
		"decoder-ffmpeg/src/androidTest/java/androidx/media3/decoder/ffmpeg/FfmpegDecoderException.java":         "android-FfmpegDecoderException.java",
		"decoder-ffmpeg/src/androidTest/java/androidx/media3/decoder/ffmpeg/FfmpegLibrary.java":                  "android-FfmpegLibrary.java",
	}[receipt.path]
}

// These complete byte goldens are intentionally data, not modeled with production wire structs.
// Each equality pins the bridge-v0 envelope, all fact order/evidence digests, and the absence of
// parser-local spans and witnesses, so added or fabricated output cannot be accepted.
func TestLiteralBeamfallNativeBridgeGoldens(t *testing.T) {
	binaries := map[string]string{}
	for _, candidate := range candidateExecutables() {
		binaries[candidate.name] = buildCandidate(t, candidate)
	}
	for _, golden := range []struct {
		name, command, requestSHA256, responseSHA256 string
	}{
		{"c-jni", "corvint-analyzer-c-jni", "34eedd5e52b81caba7b40e26718e8a90be57f22a259745cc3938a9dfb7dab694", "2f6a3887cef66e9d11653a95b611168f38f8eaf42d03370b3dab72b1547648f8"},
		{"objective-c", "corvint-analyzer-objective-c", "73f95a0e5e594ea06d322696d63557661b540d5d8638735128a110907d945ef3", "891d26acba53d539d5f844812a1b19e72d19caa33dc68ed2b508ecb8ef3385ab"},
		{"java", "corvint-analyzer-java", "63ec11fbd43cb74e9b99e7105894711a5cc5ba0c5c1d4b1de0392759fa51985c", "56906180db4df291ebf7705ae20157076f926be29b3bbe3cac029b3894e6566c"},
		{"cmake", "corvint-analyzer-c-jni", "a2161b98973d8e316ff45b2929da10f4d6fadd4435f52e4dcb51a7bda7e79e6b", "53018e02c4a7abacec8b599975ebd534870147397c72ec809f513e3dee9b8db6"},
		{"reject-malformed-c", "corvint-analyzer-c-jni", "e07597e27f5cb0c0929c2f99a40028758e0d7d0c863dbf66662e2c21f587aac1", "3ea81d26d99d8f2b2ddde791bf695eef8ea7a8e4903e5e9433d29916aa54dfb5"},
		{"reject-malformed-objective-c", "corvint-analyzer-objective-c", "0364aa60b623df538de2159841407a0fb001c722a6ab58ffc0d4a9a8d80725b4", "39ebe588b1e3d63ce3e27d8ea69ba27184f937b44e0b3bf529d3d3ea53315a3d"},
		{"reject-malformed-java", "corvint-analyzer-java", "0c404b038782d9837fc314d845eecce63286ee7029c28ba5760418ecaf1c5fcf", "39152970a9184b401d712bdb1a72d806e68916f2b3f517dadbd42024306e51ac"},
		{"reject-malformed-cmake", "corvint-analyzer-c-jni", "087fde62ee5d4a2928c82d765d07898f40e00eae73a3ac791f83f2ff31404ae8", "a657b08f93759468db91a59ed7f935b0e108ffe2ceda6412a1d4be82926bcc6b"},
		{"reject-wrong-family", "corvint-analyzer-c-jni", "39666823fc414d95757fdc931b078e045674162f02bee08f02c7973260f1a743", "54fec270b3482acdcfec952e36bfd05adca815c998933c417e0faa8f3944c1c9"},
	} {
		t.Run(golden.name, func(t *testing.T) {
			request := nativeBridgeGolden(t, golden.name+".request.json", golden.requestSHA256)
			want := nativeBridgeGolden(t, golden.name+".response.json", golden.responseSHA256)
			if got := runCLI(t, binaries[golden.command], request); !bytes.Equal(got, want) {
				t.Fatalf("response mismatch\nwant=%s\ngot=%s", want, got)
			}
		})
	}
}

func nativeBridgeGolden(t testing.TB, name, wantSHA256 string) []byte {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source location unavailable")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(source), "testdata", "bridge-goldens", name))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != wantSHA256 {
		t.Fatalf("golden %s sha256=%s", name, got)
	}
	return data
}

func TestOutcomeReasonBoundaryMatrix(t *testing.T) {
	for _, test := range []struct{ name, profile, family, path, source, reason string }{
		{"unterminated-c-string", CProfile, "c", "bridge.c", "char *x = \"unterminated\n", "UNSUPPORTED_SCHEMA"},
		{"java-dynamic-loader", JavaProfile, "java", "bridge.java", "class Bridge { static { System.loadLibrary(name); } }\n", "DYNAMIC_INPUT"},
		{"objc-unclosed-comment", ObjectiveCProfile, "objective-c", "bridge.m", "/* unclosed\n", "UNSUPPORTED_SCHEMA"},
	} {
		t.Run(test.name, func(t *testing.T) {
			output := testAnalyze(test.profile, bytes.NewReader(frame(test.profile, test.family, "request-1", test.path, test.source)))
			if !bytes.Contains(output, []byte(`"reason":"`+test.reason+`"`)) {
				t.Fatalf("output=%s", output)
			}
		})
	}
}

func TestProspectiveDecodedAndOutputBounds(t *testing.T) {
	tooLarge := strings.Repeat("x", maxInput+1)
	if output := AnalyzeC(bytes.NewReader(frame(CProfile, "c", "decoded-bound", "bridge.c", tooLarge))); !bytes.Contains(output, []byte(`"reason":"LIMIT_EXCEEDED"`)) {
		t.Fatalf("decoded output=%s", output)
	}
	var source strings.Builder
	for index := 0; index < maxFacts; index++ {
		source.WriteString("#define M")
		source.WriteString(strconv.Itoa(index))
		source.WriteString(strings.Repeat("x", 120-len(strconv.Itoa(index))))
		source.WriteByte('\n')
	}
	if output := AnalyzeC(bytes.NewReader(frame(CProfile, "c", "output-bound", "bridge.h", source.String()))); !bytes.Contains(output, []byte(`"reason":"OUTPUT_LIMIT"`)) || bytes.Contains(output, []byte(`"facts"`)) {
		t.Fatalf("output bound=%s", output)
	}
}

func TestPreflightLimitAndMalformedSentinels(t *testing.T) {
	inputs := strings.TrimSuffix(strings.Repeat(`{"handle":"x"},`, 129), ",")
	tooManyInputs := []byte(`{"profile":"corvint-analyzer-native-bridge/v0","family":"c","request_id":"x","scope_id":"x","compilation_unit_id":"x","target":{"os":"android","architecture":"arm64-v8a","abi":"android-24","features":[]},"inputs":[` + inputs + `]}` + "\n")
	tooDeep := []byte(strings.Repeat("[", maxJSONDepth+1) + "0" + strings.Repeat("]", maxJSONDepth+1) + "\n")
	// ACP-012: an oversize request is LIMIT_EXCEEDED whether or not it ends in LF.
	oversizeNoLF := []byte(strings.Repeat("x", maxWire+1))
	for _, wire := range [][]byte{tooManyInputs, tooDeep, oversizeNoLF} {
		if output := AnalyzeC(bytes.NewReader(wire)); !bytes.Equal(output, limitSentinel) {
			t.Fatalf("limit output=%s", output)
		}
	}
	for _, wire := range [][]byte{[]byte("not-json\n"), []byte("{}\n")} {
		if output := AnalyzeC(bytes.NewReader(wire)); !bytes.Equal(output, sentinel) {
			t.Fatalf("noncanonical output=%s", output)
		}
	}
}

func TestInvalidEnvelopeRejectionStaysUnbound(t *testing.T) {
	for name, edit := range map[string]func(*request){
		"family": func(req *request) { req.Family = "rust" },
		"target": func(req *request) { req.Target.OS = "bad os" },
		"handle": func(req *request) { req.Inputs[0].Handle = "bad handle" },
		"digest": func(req *request) { req.Inputs[0].SHA256 = "sha256:BAD" },
	} {
		valid := frame(CProfile, "c", "unbound", "one.c", "#define ONE\n")
		var req request
		if err := json.Unmarshal(valid[:len(valid)-1], &req); err != nil {
			t.Fatal(err)
		}
		edit(&req)
		data, _ := json.Marshal(req)
		if output := AnalyzeC(bytes.NewReader(append(data, '\n'))); !bytes.Equal(output, sentinel) {
			t.Fatalf("%s: output=%s", name, output)
		}
	}
}

func TestRequestPreflightAndDuplicateBindingRejections(t *testing.T) {
	valid := frame(CProfile, "c", "duplicate", "one.c", "#define ONE\n")
	var req request
	if err := json.Unmarshal(valid[:len(valid)-1], &req); err != nil {
		t.Fatal(err)
	}
	second := req.Inputs[0]
	second.Path = "two.c"
	second.ContentBase64 = base64.StdEncoding.EncodeToString([]byte("#define TWO\n"))
	sum := sha256.Sum256([]byte("#define TWO\n"))
	second.SHA256 = "sha256:" + hex.EncodeToString(sum[:])
	req.Inputs = append(req.Inputs, second)
	data, _ := json.Marshal(req)
	if output := AnalyzeC(bytes.NewReader(append(data, '\n'))); !bytes.Equal(output, sentinel) {
		t.Fatalf("duplicate handle rejection=%s", output)
	}
	if !preflightRequest([]byte(`{"profile":"x","family":"x","request_id":"x","scope_id":"x","compilation_unit_id":"x","target":{"os":"x","architecture":"x","abi":"x","features":["x"]},"inputs":[]}`)) {
		t.Fatal("bounded feature grammar was rejected during allocation preflight")
	}
	if output := rejected(request{Profile: strings.Repeat("x", maxOutput), Family: "c"}, nil, "LIMIT_EXCEEDED"); !bytes.Equal(output, sentinel) || len(output) > maxOutput {
		t.Fatalf("bounded rejection=%d", len(output))
	}
}

func TestFeatureGrammarAndOutOfTupleIdentity(t *testing.T) {
	t.Run("NJB-001 feature grammar and target identity", func(t *testing.T) {
		base := frame(JavaProfile, "java", "feature-binding", "Bridge.java", "class Bridge {}\n")
		var req request
		if err := json.Unmarshal(base[:len(base)-1], &req); err != nil {
			t.Fatal(err)
		}
		for _, test := range []struct {
			name, reason string
			features     []string
		}{
			{"valid-nonempty-out-of-tuple", "EXACT_BINDING_UNAVAILABLE", []string{"jdk-17"}},
			{"duplicate", "", []string{"jdk-17", "jdk-17"}},
			{"descending", "", []string{"z", "a"}},
			{"invalid-identifier", "", []string{"jdk/17"}},
		} {
			t.Run(test.name, func(t *testing.T) {
				candidate := req
				candidate.Target.Features = test.features
				wire, _ := json.Marshal(candidate)
				encoded := AnalyzeJava(bytes.NewReader(append(wire, '\n')))
				if test.reason == "" && bytes.Equal(encoded, sentinel) {
					return
				}
				var got rejection
				if err := json.Unmarshal(encoded, &got); err != nil || got.Reason != test.reason || got.RequestID != candidate.RequestID || got.Target.OS != candidate.Target.OS || !slicesEqual(got.Target.Features, test.features) || len(got.InputEchoes) != 1 {
					t.Fatalf("bound rejection=%s err=%v", encoded, err)
				}
			})
		}
		req.Target.OS = "linux"
		wire, _ := json.Marshal(req)
		var got rejection
		if encoded := AnalyzeJava(bytes.NewReader(append(wire, '\n'))); json.Unmarshal(encoded, &got) != nil || got.Reason != "EXACT_BINDING_UNAVAILABLE" || got.Target.OS != "linux" {
			t.Fatalf("target rejection=%s", AnalyzeJava(bytes.NewReader(append(wire, '\n'))))
		}
		features := make([]string, 65)
		for index := range features {
			features[index] = fmt.Sprintf("f%02d", index)
		}
		req.Target = candidateTarget("java")
		req.Target.Features = features
		wire, _ = json.Marshal(req)
		if encoded := AnalyzeJava(bytes.NewReader(append(wire, '\n'))); !bytes.Equal(encoded, limitSentinel) {
			t.Fatalf("feature cap=%s", encoded)
		}
	})
}

func TestNativeBridgeEnvelopeDoesNotBroadenFrozenCandidateProfile(t *testing.T) {
	if Profile != "corvint-analyzer-native-bridge/v0" || Profile == "corvint-analyzer-candidate/experimental" {
		t.Fatalf("native bridge profile=%q", Profile)
	}
	wire := frame(Profile, "c", "bridge-profile", "bridge.c", "#define BRIDGE\n")
	legacy := bytes.Replace(wire, []byte(Profile), []byte("corvint-analyzer-candidate/experimental"), 1)
	var got rejection
	if encoded := AnalyzeC(bytes.NewReader(legacy)); json.Unmarshal(encoded, &got) != nil || got.Reason != "EXACT_BINDING_UNAVAILABLE" {
		t.Fatalf("legacy profile=%s", AnalyzeC(bytes.NewReader(legacy)))
	}
}

// NJB-002: an incomplete required array cannot be echoed as a full request.
func TestRequiredFeaturesAndLogicalPathGrammar(t *testing.T) {
	t.Run("NJB-002 incomplete required feature array", func(t *testing.T) {
		const minimal = `{"profile":"corvint-analyzer-native-bridge/v0","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n"
		for _, candidate := range candidateExecutables() {
			t.Run(candidate.family, func(t *testing.T) {
				valid := candidateFrame(candidate, "wire-grammar", 0)
				for name, replacement := range map[string]string{
					"null":    `,"features":null`,
					"missing": "",
					"string":  `,"features":""`,
					"object":  `,"features":{}`,
				} {
					t.Run(name, func(t *testing.T) {
						wire := bytes.Replace(valid, []byte(`,"features":[]`), []byte(replacement), 1)
						if encoded := map[string]func(io.Reader) []byte{"c": AnalyzeC, "objective-c": AnalyzeObjectiveC, "java": AnalyzeJava}[candidate.family](bytes.NewReader(wire)); string(encoded) != minimal {
							t.Fatalf("incomplete required features=%s", encoded)
						}
					})
				}
			})
		}
	})
	valid := frame(CProfile, "c", "wire-grammar", "bridge.c", "#define BRIDGE\n")
	var got rejection
	var req request
	if err := json.Unmarshal(valid[:len(valid)-1], &req); err != nil {
		t.Fatal(err)
	}
	req.Inputs[0].Path = "drive:bridge.c"
	wire, _ := json.Marshal(req)
	if encoded := AnalyzeC(bytes.NewReader(append(wire, '\n'))); json.Unmarshal(encoded, &got) != nil || got.Reason != "MALFORMED_INPUT" {
		t.Fatalf("colon path=%s", AnalyzeC(bytes.NewReader(append(wire, '\n'))))
	}
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func TestFactCollectorPrechargesBeforeRetention(t *testing.T) {
	req := request{Profile: Profile, Family: "c", RequestID: "charge", ScopeID: "beamfall", CompilationUnitID: "bridge", Target: candidateTarget("c")}
	collector := newFactCollector(req, nil)
	item := fact{Kind: strings.Repeat("x", maxOutput), InputHandle: "source"}
	if reason := collector.add(item); reason != "LIMIT_EXCEEDED" || len(collector.facts) != 0 {
		t.Fatalf("reason=%q retained=%d bytes=%d", reason, len(collector.facts), collector.outputBytes)
	}
	for _, item := range []fact{
		{Kind: "kind", InputHandle: "source", RelatedHandle: "-", Subject: strings.Repeat("x", 4097), Predicate: "predicate", Value: "value", InstanceID: "bridge", EvidenceSHA256: strings.Repeat("e", 71)},
		{Kind: "kind", InputHandle: "source", RelatedHandle: "-", Subject: "subject", Predicate: "predicate", Value: strings.Repeat("x", 4097), InstanceID: "bridge", EvidenceSHA256: strings.Repeat("e", 71)},
	} {
		collector := newFactCollector(req, nil)
		if reason := collector.add(item); reason != "LIMIT_EXCEEDED" || len(collector.facts) != 0 {
			t.Fatalf("oversized fact reason=%q retained=%d", reason, len(collector.facts))
		}
	}
}

func TestClosedEnvelopeAndWrongCommandBinding(t *testing.T) {
	java := frame(JavaProfile, "java", "wrong-command", "Bridge.java", "class Bridge {}\n")
	var rejectedByC rejection
	if err := json.Unmarshal(AnalyzeC(bytes.NewReader(java)), &rejectedByC); err != nil {
		t.Fatal(err)
	}
	if rejectedByC.Status != "REJECTED" || rejectedByC.Reason != "EXACT_BINDING_UNAVAILABLE" || rejectedByC.Profile != Profile || rejectedByC.Family != "java" || rejectedByC.Target.OS != "android" || rejectedByC.Target.Architecture != "arm64-v8a" || len(rejectedByC.InputEchoes) != 1 {
		t.Fatalf("wrong command=%+v", rejectedByC)
	}
	for _, raw := range []string{
		strings.Repeat("[", 9) + "{}" + strings.Repeat("]", 9),
		`{"profile":"corvint-analyzer-native-bridge/v0","family":"c","request_id":"x","scope_id":"x","compilation_unit_id":"x","target":{"os":"android","architecture":"arm64-v8a","abi":"android-24","features":[]},"inputs":[` + strings.Repeat(`{"handle":"x",`, 129) + "]}",
	} {
		if preflightRequest([]byte(raw)) {
			t.Fatalf("preflight accepted bounded-invalid input")
		}
	}
	for _, source := range []struct{ family, path, source, reason string }{
		{"java", "Bridge.java", "class Bridge { Object x = System.out; static { System.loadLibrary(name); } }\n", "DYNAMIC_INPUT"},
		{"c", "bridge.c", "void bridge(void) { Java_com_example_call(); }\n", "UNSUPPORTED_SCHEMA"},
		{"c", "CMakeLists.txt", "add_library(bridge $<TARGET_OBJECTS:x>)\n", "UNSUPPORTED_SCHEMA"},
	} {
		encoded := testAnalyze(Profile, bytes.NewReader(frame(Profile, source.family, "closed-grammar", source.path, source.source)))
		var got rejection
		if err := json.Unmarshal(encoded, &got); err != nil || got.Reason != source.reason || len(got.InputEchoes) != 1 {
			t.Fatalf("%s %q err=%v", source.path, encoded, err)
		}
	}
}

func TestExactLexicalFactsAndHostileSyntax(t *testing.T) {
	t.Run("NJB-003 lexical fact and hostile syntax", func(t *testing.T) {
		for _, test := range []struct {
			name, profile, family, path, source, kind, value, reason string
		}{
			{"c", CProfile, "c", "bridge.c", "#define FLAG\n", "c.macro", "FLAG", ""},
			{"objc", ObjectiveCProfile, "objective-c", "bridge.m", "@interface Bridge\n@end\n", "objc.interface", "Bridge", ""},
			{"java", JavaProfile, "java", "Bridge.java", "final class Bridge {}\n", "java.class", "Bridge", ""},
			{"c-unknown", CProfile, "c", "bridge.c", "@@@\n", "", "", "UNSUPPORTED_SCHEMA"},
			{"c-words", CProfile, "c", "bridge.c", "not valid syntax;\n", "", "", "UNSUPPORTED_SCHEMA"},
			{"c-body-words", CProfile, "c", "bridge.c", "int bridge(void) {\nnot valid syntax;\n}\n", "", "", "UNSUPPORTED_SCHEMA"},
			{"c-continuation", CProfile, "c", "bridge.c", "#define FLAG \\\n+next\n", "", "", "UNSUPPORTED_SCHEMA"},
			{"objc-malformed", ObjectiveCProfile, "objective-c", "bridge.m", "- (void) :\n", "", "", "UNSUPPORTED_SCHEMA"},
			{"objc-words", ObjectiveCProfile, "objective-c", "bridge.m", "not valid syntax;\n", "", "", "UNSUPPORTED_SCHEMA"},
			{"objc-body-words", ObjectiveCProfile, "objective-c", "bridge.m", "- (void)bridge {\nnot valid syntax;\n}\n", "", "", "UNSUPPORTED_SCHEMA"},
			{"java-malformed", JavaProfile, "java", "Bridge.java", "native void bridge;\n", "", "", "UNSUPPORTED_SCHEMA"},
			{"java-words", JavaProfile, "java", "Bridge.java", "not valid syntax;\n", "", "", "UNSUPPORTED_SCHEMA"},
			{"java-body-words", JavaProfile, "java", "Bridge.java", "class Bridge {\nnot valid syntax;\n}\n", "", "", "UNSUPPORTED_SCHEMA"},
		} {
			t.Run(test.name, func(t *testing.T) {
				requestID := "literal-" + test.name
				encoded := testAnalyze(test.profile, bytes.NewReader(frame(test.profile, test.family, requestID, test.path, test.source)))
				if test.reason != "" {
					var got rejection
					want := independentRejection(test.profile, test.family, requestID, test.source, test.reason)
					if err := json.Unmarshal(encoded, &got); err != nil || got.Status != "REJECTED" || got.Reason != test.reason || bytes.Count(encoded, []byte{'\n'}) != 1 || !bytes.Equal(encoded, want) {
						t.Fatalf("exact rejection=%q err=%v", encoded, err)
					}
					return
				}
				var got output
				if err := json.Unmarshal(encoded, &got); err != nil || got.Status != "CANDIDATE" || len(got.Facts) != 1 {
					t.Fatalf("candidate=%q err=%v", encoded, err)
				}
				fact := got.Facts[0]
				text := literalFactText(test.kind, test.value)
				if fact.Kind != test.kind || fact.Value != test.value || fact.Witness != "" || fact.Span != (span{}) || text == "" {
					t.Fatalf("literal fact=%+v", fact)
				}
				if fact.EvidenceSHA256 != manualFactEvidence(test.profile, test.family, requestID, test.path, test.value, test.kind, span{}) {
					t.Fatalf("literal digest=%s", fact.EvidenceSHA256)
				}
			})
		}
	})
}

type independentEcho struct {
	Handle string `json:"handle"`
	SHA256 string `json:"sha256"`
}

type independentRejectionEnvelope struct {
	Profile           string            `json:"profile"`
	Family            string            `json:"family"`
	RequestID         string            `json:"request_id"`
	Status            string            `json:"status"`
	ScopeID           string            `json:"scope_id"`
	CompilationUnitID string            `json:"compilation_unit_id"`
	Target            target            `json:"target"`
	InputEchoes       []independentEcho `json:"input_echoes"`
	Reason            string            `json:"reason"`
}

func independentRejection(profile, family, requestID, source, reason string) []byte {
	sum := sha256.Sum256([]byte(source))
	encoded, err := json.Marshal(independentRejectionEnvelope{Profile: profile, Family: family, RequestID: requestID, Status: "REJECTED", ScopeID: "beamfall", CompilationUnitID: "bridge", Target: candidateTarget(family), InputEchoes: []independentEcho{{Handle: "source", SHA256: "sha256:" + hex.EncodeToString(sum[:])}}, Reason: reason})
	if err != nil {
		panic(err)
	}
	return append(encoded, '\n')
}

func TestLexicalDecoysAndSpacedCallsHaveExactOmissions(t *testing.T) {
	cDecoy := "// #define FORGED\n#define REAL\n"
	encoded := AnalyzeC(bytes.NewReader(frame(CProfile, "c", "c-decoy", "bridge.c", cDecoy)))
	var got output
	if err := json.Unmarshal(encoded, &got); err != nil || len(got.Facts) != 1 || got.Facts[0].Kind != "c.macro" || got.Facts[0].Value != "REAL" {
		t.Fatalf("c decoy=%q err=%v", encoded, err)
	}
	objcDecoy := "/* @interface Forged */\n@interface Real\n@end\n"
	encoded = AnalyzeObjectiveC(bytes.NewReader(frame(ObjectiveCProfile, "objective-c", "objc-decoy", "Bridge.m", objcDecoy)))
	if err := json.Unmarshal(encoded, &got); err != nil || len(got.Facts) != 1 || got.Facts[0].Kind != "objc.interface" || got.Facts[0].Value != "Real" {
		t.Fatalf("objc decoy=%q err=%v", encoded, err)
	}
	decoys := "// System.loadLibrary(name) Class.forName(\"java.lang.System\")\nclass Bridge { String text = \"System.loadLibrary(name) Class.forName(\\\"java.lang.System\\\")\"; }\n"
	encoded = AnalyzeJava(bytes.NewReader(frame(JavaProfile, "java", "decoy", "Bridge.java", decoys)))
	if err := json.Unmarshal(encoded, &got); err != nil || got.Status != "CANDIDATE" {
		t.Fatalf("decoys=%q err=%v", encoded, err)
	}
	for _, fact := range got.Facts {
		if fact.Kind == "java.jni.library" {
			t.Fatalf("forged library fact=%+v", fact)
		}
	}
	spaced := "class Bridge { static { System . loadLibrary ( \"ffmpegJNI\" ); } }\n"
	encoded = AnalyzeJava(bytes.NewReader(frame(JavaProfile, "java", "spaced", "Bridge.java", spaced)))
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Facts) != 2 || got.Facts[1].Kind != "java.jni.library" || got.Facts[1].Value != "ffmpegJNI" {
		t.Fatalf("spaced loader=%s", encoded)
	}
	cmake := "add_library ( bridge SHARED )\n"
	encoded = AnalyzeC(bytes.NewReader(frame(CProfile, "c", "cmake", "CMakeLists.txt", cmake)))
	if err := json.Unmarshal(encoded, &got); err != nil || len(got.Facts) != 1 || got.Facts[0].Kind != "c.cmake.declaration" || got.Facts[0].Value != "bridge" {
		t.Fatalf("spaced cmake=%q err=%v", encoded, err)
	}
}

func TestCMakeInputFamilyRequiresExactFilename(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"CMakeLists.txt", true},
		{"kit/src/main/cpp/CMakeLists.txt", true},
		{"module.cmake", true},
		{"fooCMakeLists.txt", false},
		{"kit/src/mainCMakeLists.txt", false},
	} {
		if got := validInputFamily("c", "c.cmake", tc.path); got != tc.want {
			t.Fatalf("validInputFamily(c.cmake, %q)=%t want %t", tc.path, got, tc.want)
		}
	}
}

func TestJavaDynamicLoaderLexicalVariantsFailClosed(t *testing.T) {
	for _, source := range []string{
		"class Bridge { static { System . loadLibrary ( name ); } }\n",
		"class Bridge { static { System /* gap */ . loadLibrary(name); } }\n",
		"class Bridge { static { System\n.\nloadLibrary(name); } }\n",
		"class Bridge { static { java.lang.System.load(\"/tmp/lib.so\"); } }\n",
		"class Bridge { static { Runtime . getRuntime ( ) . loadLibrary ( \"bridge\" ); } }\n",
		"class Bridge { static { Runtime runtime = Runtime.getRuntime(); runtime.loadLibrary(\"bridge\"); } }\n",
		"class Bridge { java.util.function.Consumer<String> loader = System::loadLibrary; }\n",
		"class Bridge { static { System.class.getMethod(\"loadLibrary\", String.class).invoke(null, \"bridge\"); } }\n",
		"class Bridge { static { System.class.getDeclaredMethod(\"loadLibrary\", String.class).invoke(null, \"bridge\"); } }\n",
		"class Bridge { static { Class.forName(\"java.lang.System\").getMethod(\"loadLibrary\", String.class).invoke(null, \"bridge\"); } }\n",
		"class Bridge { static { Class.forName(\"java.lang.System\").getDeclaredMethod(\"loadLibrary\", String.class).invoke(null, \"bridge\"); } }\n",
		"class Bridge { static { Class system = Class.forName(\"java.lang.System\"); Method method = system.getMethod(\"loadLibrary\", String.class); method.invoke(null, \"bridge\"); } }\n",
		"class Bridge { static { Class system = Class.forName(\"java.lang.System\"); Method method = system.getDeclaredMethod(\"loadLibrary\", String.class); method.invoke(null, \"bridge\"); } }\n",
	} {
		encoded := AnalyzeJava(bytes.NewReader(frame(JavaProfile, "java", "dynamic-variant", "Bridge.java", source)))
		var got rejection
		if err := json.Unmarshal(encoded, &got); err != nil || got.Reason != "DYNAMIC_INPUT" || len(got.InputEchoes) != 1 {
			t.Errorf("source=%q output=%s err=%v", source, encoded, err)
		}
	}
	static := "class Bridge { static { System\n.\nloadLibrary\n(\n\"bridge\"\n); } }\n"
	encoded := AnalyzeJava(bytes.NewReader(frame(JavaProfile, "java", "static-multiline", "Bridge.java", static)))
	var got output
	if err := json.Unmarshal(encoded, &got); err != nil || got.Status != "CANDIDATE" {
		t.Fatalf("static output=%s err=%v", encoded, err)
	}
	found := false
	for _, item := range got.Facts {
		found = found || item.Kind == "java.jni.library" && item.Value == "bridge"
	}
	if !found {
		t.Fatalf("static loader fact missing: %s", encoded)
	}
}

func TestJavaUnicodeEscapesPrecedeLoaderClassification(t *testing.T) {
	t.Run("NJB-004 unicode escape loader classification", func(t *testing.T) {
		for _, source := range []string{
			`class Bridge { static { System.\u006coadLibrary(name); } }` + "\n",
			`class Bridge { static { System.load\u004cibrary(name); } }` + "\n",
			`class Bridge { static { Runtime.getRuntime().\u006coadLibrary("bridge"); } }` + "\n",
		} {
			encoded := AnalyzeJava(bytes.NewReader(frame(JavaProfile, "java", "unicode-dynamic", "Bridge.java", source)))
			var got rejection
			if err := json.Unmarshal(encoded, &got); err != nil || got.Reason != "DYNAMIC_INPUT" {
				t.Errorf("source=%q output=%s err=%v", source, encoded, err)
			}
		}
		static := `class Bridge { static { System.\u006coadLibrary("bridge"); } }` + "\n"
		var got output
		if encoded := AnalyzeJava(bytes.NewReader(frame(JavaProfile, "java", "unicode-static", "Bridge.java", static))); json.Unmarshal(encoded, &got) != nil || len(got.Facts) != 2 || got.Facts[1].Kind != "java.jni.library" || got.Facts[1].Value != "bridge" {
			t.Fatalf("static Unicode loader=%s", encoded)
		}
		for _, malformed := range []string{
			`class Bridge { int value = \u00ZZ; }` + "\n",
			`class Bridge { String value = "\\\u006c"; }` + "\n",
			`class Bridge { int value = \u005cu006c; }` + "\n",
		} {
			encoded := AnalyzeJava(bytes.NewReader(frame(JavaProfile, "java", "unicode-ambiguous", "Bridge.java", malformed)))
			var rejected rejection
			if err := json.Unmarshal(encoded, &rejected); err != nil || rejected.Reason != "UNSUPPORTED_SCHEMA" {
				t.Errorf("malformed=%q output=%s err=%v", malformed, encoded, err)
			}
		}
	})
}

func TestJNIDeclarationsAreNotCalls(t *testing.T) {
	declaration := "JNIEXPORT jlong JNICALL\nJava_com_example_Bridge_open(JNIEnv *env, jclass clazz) { return 0; }\n"
	var candidate output
	if encoded := AnalyzeC(bytes.NewReader(frame(CProfile, "c", "jni-declaration", "bridge.c", declaration))); json.Unmarshal(encoded, &candidate) != nil || candidate.Status != "CANDIDATE" {
		t.Fatalf("declaration=%s", encoded)
	}
	found := false
	for _, item := range candidate.Facts {
		found = found || item.Kind == "c.jni.symbol" && item.Value == "Java_com_example_Bridge_open"
	}
	if !found {
		t.Fatalf("declaration fact missing=%+v", candidate.Facts)
	}
	for _, call := range []string{
		"void bridge(void) { JNIEnv *env = NULL; Java_com_example_Bridge_open(env, 0); }\n",
		"JNIEXPORT void JNICALL wrapper(JNIEnv *env) { Java_com_example_Bridge_open(env, 0); }\n",
	} {
		encoded := AnalyzeC(bytes.NewReader(frame(CProfile, "c", "jni-call", "bridge.c", call)))
		var got rejection
		if err := json.Unmarshal(encoded, &got); err != nil || got.Reason != "UNSUPPORTED_SCHEMA" {
			t.Errorf("call=%q output=%s err=%v", call, encoded, err)
		}
	}
}

func TestMalformedBodiesFailClosed(t *testing.T) {
	for _, test := range []struct{ family, path, source string }{
		{"c", "bridge.c", "int bridge(void) { int value = ; }\n"},
		{"c", "bridge.c", "int bridge(void) { unknown malformed body; }\n"},
		{"objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { long value = ; }\n@end\n"},
		{"objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { unknown malformed body; }\n@end\n"},
		{"java", "Bridge.java", "class Bridge { int value = ; }\n"},
		{"java", "Bridge.java", "class Bridge { unknown malformed body; }\n"},
		{"c", "bridge.c", "int bridge(void) { nonsense }\n"},
		{"objective-c", "Bridge.m", "- (void)run { nonsense }\n"},
		{"java", "Bridge.java", "class Bridge { nonsense }\n"},
		{"c", "bridge.c", "int bridge(void) { nonsense() }\n"},
		{"objective-c", "Bridge.m", "- (void)run { nonsense() }\n"},
		{"java", "Bridge.java", "class Bridge { nonsense() }\n"},
		{"c", "bridge.c", "JNIEXPORT jlong JNICALL Java_x(JNIEnv *env) nonsense\n"},
		{"c", "bridge.c", "JNIEXPORT jlong JNICALL Java_x(JNIEnv *env) { return 0; }\n"},
		{"c", "bridge.c", "JNIEXPORT jlong JNICALL Java_x(JNIEnv env) { return 0; }\n"},
		{"c", "bridge.c", "JNIEXPORT unknown JNICALL Java_x(JNIEnv *env) { return 0; }\n"},
	} {
		encoded := testAnalyze(Profile, bytes.NewReader(frame(Profile, test.family, "malformed-body", test.path, test.source)))
		var got rejection
		if err := json.Unmarshal(encoded, &got); err != nil || got.Reason != "UNSUPPORTED_SCHEMA" {
			t.Errorf("family=%s source=%q output=%s err=%v", test.family, test.source, encoded, err)
		}
	}
}

func TestClosedNativeBodyGrammarAndJNISignatures(t *testing.T) {
	tests := []struct {
		name, family, path, source, reason string
		candidate                          bool
	}{
		{"c-punctuation", "c", "bridge.c", "int bridge(void) { unknown + ; }\n", "UNSUPPORTED_SCHEMA", false},
		{"objc-punctuation", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { unknown + ; }\n@end\n", "UNSUPPORTED_SCHEMA", false},
		{"java-punctuation", "java", "Bridge.java", "class Bridge { unknown + ; }\n", "UNSUPPORTED_SCHEMA", false},
		{"c-operator-adjacency", "c", "bridge.c", "int bridge(void) { int x * / y; }\n", "UNSUPPORTED_SCHEMA", false},
		{"objc-operator-adjacency", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { int x * / y; }\n@end\n", "UNSUPPORTED_SCHEMA", false},
		{"java-operator-adjacency", "java", "Bridge.java", "class Bridge { int x * / y; }\n", "UNSUPPORTED_SCHEMA", false},
		{"c-malformed-call", "c", "bridge.c", "int bridge(void) { unknown(,); }\n", "UNSUPPORTED_SCHEMA", false},
		{"objc-malformed-call", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { unknown(,); }\n@end\n", "UNSUPPORTED_SCHEMA", false},
		{"java-malformed-call", "java", "Bridge.java", "class Bridge { unknown(,); }\n", "UNSUPPORTED_SCHEMA", false},
		{"java-reflective-loader", "java", "Bridge.java", "class Bridge { static { Class.forName(\"java.lang.System\").getMethod(\"loadLibrary\", String.class).invoke(null, \"bridge\"); } }\n", "DYNAMIC_INPUT", false},
		{"java-runtime-alias-loader", "java", "Bridge.java", "class Bridge { static { Runtime runtime = Runtime.getRuntime(); runtime.loadLibrary(\"bridge\"); } }\n", "DYNAMIC_INPUT", false},
		{"java-system-class-reflective-loader", "java", "Bridge.java", "class Bridge { static { System.class.getMethod(\"loadLibrary\", String.class).invoke(null, \"bridge\"); } }\n", "DYNAMIC_INPUT", false},
		{"jni-missing-env", "c", "bridge.c", "JNIEXPORT jlong JNICALL Java_com_example_Bridge_open(jclass clazz) { return 0; }\n", "UNSUPPORTED_SCHEMA", false},
		{"jni-wrong-receiver", "c", "bridge.c", "JNIEXPORT jlong JNICALL Java_com_example_Bridge_open(JNIEnv *env, jlong handle) { return 0; }\n", "UNSUPPORTED_SCHEMA", false},
		{"jni-nested-declaration", "c", "bridge.c", "void outer(void)\n{\nJNIEXPORT jlong JNICALL Java_com_example_Bridge_open(JNIEnv *env, jclass clazz) { return 0; }\n}\n", "UNSUPPORTED_SCHEMA", false},
		{"c-statement", "c", "bridge.c", "int bridge(void) { return 0; }\n", "", true},
		{"c-pointer-and-division", "c", "bridge.c", "int bridge(void) { int *value = NULL; int half = 8 / 2; return *value + half; }\n", "", true},
		{"objc-statement", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { return; }\n@end\n", "", true},
		{"objc-pointer-and-division", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run:(NSObject *)object { int *value = NULL; int half = 8 / 2; }\n@end\n", "", true},
		{"java-statement", "java", "Bridge.java", "class Bridge { static { System.loadLibrary(\"bridge\"); } }\n", "", true},
		{"java-division", "java", "Bridge.java", "class Bridge { int half = 8 / 2; }\n", "", true},
		{"jni-jclass-receiver", "c", "bridge.c", "JNIEXPORT jlong JNICALL Java_com_example_Bridge_open(JNIEnv *env, jclass clazz) { return 0; }\n", "", true},
		{"jni-jobject-receiver", "c", "bridge.c", "JNIEXPORT jlong JNICALL\nJava_com_example_Bridge_open(JNIEnv *env,\njobject object) { return 0; }\n", "", true},
		{"jni-empty-post-open-continuation", "c", "bridge.c", "JNIEXPORT jlong JNICALL\nJava_com_example_Bridge_open(\nJNIEnv *env,\njobject object) { return 0; }\n", "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile := CProfile
			if test.family == "objective-c" {
				profile = ObjectiveCProfile
			} else if test.family == "java" {
				profile = JavaProfile
			}
			encoded := testAnalyze(profile, bytes.NewReader(frame(profile, test.family, test.name, test.path, test.source)))
			if test.candidate {
				var got output
				if err := json.Unmarshal(encoded, &got); err != nil || got.Status != "CANDIDATE" {
					t.Fatalf("output=%s err=%v", encoded, err)
				}
				return
			}
			var got rejection
			if err := json.Unmarshal(encoded, &got); err != nil || got.Reason != test.reason {
				t.Fatalf("output=%s err=%v", encoded, err)
			}
		})
	}
}

func TestClosedExpressionProductionsConsumeOperands(t *testing.T) {
	for _, test := range []struct {
		name, family, path, source string
	}{
		{"c-word-adjacency", "c", "bridge.c", "int bridge(void) { not valid + syntax; }\n"},
		{"objc-word-adjacency", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { not valid + syntax; }\n@end\n"},
		{"java-word-adjacency", "java", "Bridge.java", "class Bridge { void run() { not valid + syntax; } }\n"},
		{"c-leading-xor", "c", "bridge.c", "int bridge(void) { ^ y; }\n"},
		{"objc-leading-xor", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { ^ y; }\n@end\n"},
		{"java-leading-xor", "java", "Bridge.java", "class Bridge { void run() { ^ y; } }\n"},
		{"java-leading-dereference", "java", "Bridge.java", "class Bridge { int run() { int x = * y; return x; } }\n"},
		{"java-leading-address", "java", "Bridge.java", "class Bridge { int run() { int x = & y; return x; } }\n"},
		{"java-empty-parentheses", "java", "Bridge.java", "class Bridge { int run() { int x = (); return x; } }\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile := CProfile
			if test.family == "objective-c" {
				profile = ObjectiveCProfile
			}
			if test.family == "java" {
				profile = JavaProfile
			}
			encoded := testAnalyze(profile, bytes.NewReader(frame(profile, test.family, test.name, test.path, test.source)))
			var got rejection
			if err := json.Unmarshal(encoded, &got); err != nil || got.Reason != "UNSUPPORTED_SCHEMA" {
				t.Fatalf("output=%s err=%v", encoded, err)
			}
		})
	}
}

func TestClosedExpressionOperatorOperandMatrix(t *testing.T) {
	operators := []string{"=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<=", ">>=", "+", "-", "*", "/", "%", "&", "|", "^", "&&", "||", "==", "!=", "<", "<=", ">", ">=", "<<", ">>"}
	for _, family := range []string{"c", "objective-c", "java"} {
		profile := CProfile
		path := "bridge.c"
		if family == "objective-c" {
			profile, path = ObjectiveCProfile, "Bridge.m"
		}
		if family == "java" {
			profile, path = JavaProfile, "Bridge.java"
		}
		for _, operator := range operators {
			t.Run(family+"-missing-right-"+operator, func(t *testing.T) {
				source := expressionFixture(family, "value "+operator+";")
				encoded := testAnalyze(profile, bytes.NewReader(frame(profile, family, "missing-right", path, source)))
				var got rejection
				if err := json.Unmarshal(encoded, &got); err != nil || got.Reason != "UNSUPPORTED_SCHEMA" {
					t.Fatalf("operator=%q output=%s err=%v", operator, encoded, err)
				}
			})
		}
	}
	for _, test := range []struct{ family, statement string }{
		{"c", "int x = * value; int *pointer = &x; int y = x + value;"},
		{"objective-c", "int x = * value; int *pointer = &x; int y = x + value;"},
		{"java", "int x = -value; int y = x + value; boolean equal = x == value;"},
	} {
		profile := CProfile
		path := "bridge.c"
		if test.family == "objective-c" {
			profile, path = ObjectiveCProfile, "Bridge.m"
		}
		if test.family == "java" {
			profile, path = JavaProfile, "Bridge.java"
		}
		encoded := testAnalyze(profile, bytes.NewReader(frame(profile, test.family, "expression-positive", path, expressionFixture(test.family, test.statement))))
		var got output
		if err := json.Unmarshal(encoded, &got); err != nil || got.Status != "CANDIDATE" {
			t.Fatalf("family=%s output=%s err=%v", test.family, encoded, err)
		}
	}
}

func TestClosedExpressionProductionLinearWork(t *testing.T) {
	for _, count := range []int{1 << 10, 1 << 12, 1 << 14} {
		tokens := make([]bodyToken, 0, count*2+1)
		for index := 0; index < count; index++ {
			tokens = append(tokens, bodyToken{text: "value", word: true}, bodyToken{text: "+"})
		}
		tokens = append(tokens, bodyToken{text: "value", word: true})
		work := 0
		if reason := expr(tokens, java, &work); reason != "" || work != len(tokens) {
			t.Fatalf("count=%d reason=%q work=%d tokens=%d", count, reason, work, len(tokens))
		}
	}
}

func expressionFixture(family, statement string) string {
	switch family {
	case "objective-c":
		return "@implementation Bridge\n- (void)run { " + statement + " }\n@end\n"
	case "java":
		return "class Bridge { void run() { " + statement + " } }\n"
	default:
		return "int bridge(void) { " + statement + " }\n"
	}
}

func testClosedGrammarCLIRejectsAdversarialBodies(t *testing.T, candidate candidateExecutable, binary string) {
	t.Helper()
	for _, test := range []struct{ family, path, source, reason string }{
		{"c", "bridge.c", "int bridge(void) { int x * / y; }\n", "UNSUPPORTED_SCHEMA"},
		{"objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { int x * / y; }\n@end\n", "UNSUPPORTED_SCHEMA"},
		{"java", "Bridge.java", "class Bridge { int x * / y; }\n", "UNSUPPORTED_SCHEMA"},
		{"java", "Bridge.java", "class Bridge { void run() { int x = * y; } }\n", "UNSUPPORTED_SCHEMA"},
		{"java", "Bridge.java", "class Bridge { static { Runtime runtime = Runtime.getRuntime(); runtime.loadLibrary(\"bridge\"); } }\n", "DYNAMIC_INPUT"},
		{"java", "Bridge.java", "class Bridge { static { Class system = Class.forName(\"java.lang.System\"); Method method = system.getDeclaredMethod(\"loadLibrary\", String.class); method.invoke(null, \"bridge\"); } }\n", "DYNAMIC_INPUT"},
		{"c", "bridge.c", "void outer(void)\n{\nJNIEXPORT jlong JNICALL Java_com_example_Bridge_open(JNIEnv *env, jclass clazz) { return 0; }\n}\n", "UNSUPPORTED_SCHEMA"},
	} {
		if test.family != candidate.family {
			continue
		}
		encoded := runCLI(t, binary, frame(candidate.profile, test.family, "cli-adversarial", test.path, test.source))
		var got rejection
		if err := json.Unmarshal(encoded, &got); err != nil || got.Reason != test.reason {
			t.Errorf("family=%s output=%s err=%v", test.family, encoded, err)
		}
	}
}

func TestRunWriteRejectsShortAndBrokenPipes(t *testing.T) {
	if err := Run([]string{"--version"}, "abcdef", nil, &shortWriter{limit: 2}, nil); err != nil {
		t.Fatalf("short write recovery=%v", err)
	}
	if err := Run(nil, "", nil, errorWriter{}, func(io.Reader) []byte { return []byte("x") }); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("broken pipe=%v", err)
	}
	if err := Run([]string{"x"}, "", nil, zeroWriter{}, nil); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("zero write=%v", err)
	}
	for _, count := range []int{-1, 8} {
		if err := Run([]string{"--version"}, "abcdef", nil, countWriter{count}, nil); !errors.Is(err, io.ErrShortWrite) {
			t.Fatalf("invalid write count %d=%v", count, err)
		}
	}
}

// countWriter reports a fixed byte count outside [0, len(data)].
type countWriter struct{ count int }

func (writer countWriter) Write([]byte) (int, error) { return writer.count, nil }

type unreadReader struct{ t *testing.T }

func (reader unreadReader) Read([]byte) (int, error) { reader.t.Fatal("stdin read"); return 0, io.EOF }

// NJB-008: a sole --version writes the receipt and every other argv writes the minimal
// NONCANONICAL_REQUEST sentinel; neither reads stdin. No argv analyzes stdin.
func TestRunGuardsArgvWithoutReadingStdin(t *testing.T) {
	const noncanonical = `{"profile":"corvint-analyzer-native-bridge/v0","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n"
	refuse := func(io.Reader) []byte { t.Fatal("analyzed"); return nil }
	for _, tc := range []struct {
		args []string
		want string
	}{{[]string{"--version"}, "corvint-analyzer-java/v0\n"}, {[]string{"--version", "extra"}, noncanonical}, {[]string{"--input-file", "request.json"}, noncanonical}, {[]string{""}, noncanonical}} {
		var out bytes.Buffer
		if err := Run(tc.args, "corvint-analyzer-java/v0", unreadReader{t}, &out, refuse); err != nil || out.String() != tc.want {
			t.Fatalf("args=%q err=%v output=%q", tc.args, err, out.String())
		}
	}
	var out bytes.Buffer
	if err := Run(nil, "corvint-analyzer-java/v0", strings.NewReader("request"), &out, func(reader io.Reader) []byte {
		read, _ := io.ReadAll(reader)
		return append(read, '\n')
	}); err != nil || out.String() != "request\n" {
		t.Fatalf("no argv err=%v output=%q", err, out.String())
	}
}

type shortWriter struct{ limit int }

func (writer *shortWriter) Write(data []byte) (int, error) {
	if len(data) < writer.limit {
		return len(data), nil
	}
	return writer.limit, nil
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

func manualFactEvidence(profile, family, requestID, path, value, kind string, at span) string {
	target, _ := json.Marshal(candidateTarget(family))
	sum := sha256Bytes(sourceForLiteral(kind, value))
	fields := []string{family, requestID, "beamfall", "bridge", string(target), "source", "sha256:" + hex.EncodeToString(sum[:]), "-", "-", kind, path, predicateForKind(kind), value, "bridge"}
	hash := sha256.New()
	_, _ = hash.Write([]byte("corvint-analyzer-native-bridge-evidence/v0"))
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(fields)))
	_, _ = hash.Write(length[:])
	for _, field := range fields {
		binary.BigEndian.PutUint32(length[:], uint32(len(field)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(field))
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func sha256Bytes(value string) [32]byte { return sha256.Sum256([]byte(value)) }
func sourceForLiteral(kind, value string) string {
	return map[string]string{"c.macro": "#define " + value + "\n", "objc.interface": "@interface " + value + "\n@end\n", "java.class": "final class " + value + " {}\n"}[kind]
}
func literalFactText(kind, value string) string {
	return map[string]string{"c.macro": "#define " + value, "objc.interface": "@interface " + value, "java.class": "final class " + value + " {}"}[kind]
}
func predicateForKind(kind string) string {
	return map[string]string{"c.macro": "declares", "objc.interface": "declares", "java.class": "declares"}[kind]
}

// Every regular run launches 1,000 genuinely different complete requests and 1,000 byte-identical
// replays through each separately built candidate executable. Race runs retain one equivalent case.
// Each candidate parent owns its cold build through adversarial checks and all parallel replays;
// t.TempDir cleanup runs only after those subtests finish. No executable survives this test.
func TestFreshProcessPermutationMatrix(t *testing.T) {
	t.Run("candidates", func(t *testing.T) {
		for _, candidate := range candidateExecutables() {
			t.Run(candidate.name, func(t *testing.T) {
				t.Parallel()
				binary := buildCandidate(t, candidate)
				t.Run("adversarial", func(t *testing.T) {
					testClosedGrammarCLIRejectsAdversarialBodies(t, candidate, binary)
				})
				versionContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				versionCommand := exec.Command(binary, "--version")
				version, err := boundedCombinedOutput(versionContext, versionCommand)
				cancel()
				if versionContext.Err() == context.DeadlineExceeded || err != nil || string(version) != candidate.name+"/v0\n" {
					t.Fatalf("%s version=%q err=%v", candidate.name, version, err)
				}
				t.Run("unique", func(t *testing.T) {
					t.Parallel()
					for index := 0; index < freshProcessPermutationCount(); index++ {
						input := candidateFrame(candidate, "fresh-"+strconv.Itoa(index), index)
						want := candidateExpected(candidate, "fresh-"+strconv.Itoa(index), index)
						if got := runCLI(t, binary, input); !bytes.Equal(got, want) {
							t.Fatalf("%s unique=%d\nwant=%s\ngot=%s", candidate.name, index, want, got)
						}
					}
				})
				t.Run("replay", func(t *testing.T) {
					t.Parallel()
					replay := candidateFrame(candidate, "identical-replay", 0)
					want := candidateExpected(candidate, "identical-replay", 0)
					for index := 0; index < freshProcessPermutationCount(); index++ {
						if got := runCLI(t, binary, replay); !bytes.Equal(got, want) {
							t.Fatalf("%s replay=%d\nwant=%s\ngot=%s", candidate.name, index, want, got)
						}
					}
				})
			})
		}
	})
}
func TestFreshHelperInterruptionReapsChild(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "descendant.pid")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	command := exec.Command(os.Args[0], "-test.run=^TestBridgeInterruptionHelper$")
	command.Env = append(os.Environ(), "CORVINT_NATIVE_BRIDGE_HELPER=block", "CORVINT_NATIVE_BRIDGE_HELPER_MARKER="+marker)
	output, err := boundedCombinedOutput(ctx, command)
	if ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("context=%v err=%v output=%s", ctx.Err(), err, output)
	}
	if err == nil || command.ProcessState == nil {
		t.Fatalf("helper state=%v", command.ProcessState)
	}
	pidBytes, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("descendant did not start: %v", err)
	}
	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil || !waitDescendantReaped(pid, time.Second) {
		t.Fatalf("descendant pid=%q err=%v was not reaped", pidBytes, err)
	}
	if tracked := trackedProcessTrees(); tracked != 0 {
		t.Fatalf("tracked process trees after cancellation=%d", tracked)
	}
}

func TestNormalHelperCompletionReleasesContainment(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	command := exec.Command(os.Args[0], "-test.run=^TestBridgeInterruptionHelper$")
	command.Env = append(os.Environ(), "CORVINT_NATIVE_BRIDGE_HELPER=exit")
	if output, err := boundedCombinedOutput(ctx, command); err != nil || ctx.Err() != nil {
		t.Fatalf("normal helper err=%v context=%v output=%s", err, ctx.Err(), output)
	}
	if tracked := trackedProcessTrees(); tracked != 0 {
		t.Fatalf("tracked process trees after normal completion=%d", tracked)
	}
}
func TestBridgeInterruptionHelper(t *testing.T) {
	if os.Getenv("CORVINT_NATIVE_BRIDGE_HELPER") == "descendant" {
		if marker := os.Getenv("CORVINT_NATIVE_BRIDGE_HELPER_MARKER"); marker != "" {
			if err := os.WriteFile(marker, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		time.Sleep(time.Hour)
		return
	}
	if os.Getenv("CORVINT_NATIVE_BRIDGE_HELPER") == "block" {
		child := exec.Command(os.Args[0], "-test.run=^TestBridgeInterruptionHelper$")
		child.Env = append(os.Environ(), "CORVINT_NATIVE_BRIDGE_HELPER=descendant")
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
	}
}
func freshProcessPermutationCount() int {
	if raceEnabled {
		return 1
	}
	return 1000
}
func TestFreshProcessRaceContainmentPolicy(t *testing.T) {
	if raceEnabled && freshProcessPermutationCount() != 1 {
		t.Fatal("race containment count drift")
	}
	if !raceEnabled && freshProcessPermutationCount() != 1000 {
		t.Fatal("normal fresh-process count drift")
	}
}

type candidateExecutable struct {
	name, command, profile, family, path string
}

func candidateExecutables() []candidateExecutable {
	return []candidateExecutable{
		{"corvint-analyzer-c-jni", "../../cmd/corvint-analyzer-c-jni", CProfile, "c", "bridge.c"},
		{"corvint-analyzer-objective-c", "../../cmd/corvint-analyzer-objective-c", ObjectiveCProfile, "objective-c", "Bridge.m"},
		{"corvint-analyzer-java", "../../cmd/corvint-analyzer-java", JavaProfile, "java", "Bridge.java"},
	}
}

func buildCandidate(t testing.TB, candidate candidateExecutable) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), candidate.name)
	cache := filepath.Join(t.TempDir(), "gocache")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	build := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", binary, candidate.command)
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOCACHE="+cache, "GOPROXY=off", "GOSUMDB=off")
	if output, err := boundedCombinedOutput(ctx, build); err != nil || ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("%s build: %v\n%s", candidate.name, err, output)
	}
	return binary
}

func boundedCombinedOutput(ctx context.Context, command *exec.Cmd) ([]byte, error) {
	configureProcessGroup(command)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		return output.Bytes(), err
	}
	if err := attachProcessTree(command); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return output.Bytes(), err
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		releaseProcessTree(command)
		return output.Bytes(), err
	case <-ctx.Done():
		killProcessTree(command)
		<-done
		releaseProcessTree(command)
		return output.Bytes(), ctx.Err()
	}
}

func candidateFrame(candidate candidateExecutable, requestID string, index int) []byte {
	source, _, _, _, _ := candidateFixture(candidate, index)
	return frame(candidate.profile, candidate.family, requestID, candidate.path, source)
}

// candidateExpected is deliberately independent of the production parser. The matrix checks every
// emitted byte, including the literal fact, span, witness, and evidence digest for every fresh CLI.
func candidateExpected(candidate candidateExecutable, requestID string, index int) []byte {
	source, kind, predicate, value, text := candidateFixture(candidate, index)
	sourceDigest := sha256.Sum256([]byte(source))
	at := span{0, len(text), 1, 1, 1, len(text) + 1}
	target := candidateTarget(candidate.family)
	targetJSON, _ := json.Marshal(target)
	fields := []string{candidate.family, requestID, "beamfall", "bridge", string(targetJSON), "source", "sha256:" + hex.EncodeToString(sourceDigest[:]), "-", "-", kind, candidate.path, predicate, value, "bridge"}
	item := fact{Kind: kind, InputHandle: "source", RelatedHandle: "-", Subject: candidate.path, Predicate: predicate, Value: value, InstanceID: "bridge", Span: at, Witness: value, EvidenceSHA256: independentEvidence(fields)}
	encoded, err := json.Marshal(output{Profile: candidate.profile, Family: candidate.family, RequestID: requestID, Status: "CANDIDATE", ScopeID: "beamfall", CompilationUnitID: "bridge", Target: target, InputEchoes: []echo{{Handle: "source", SHA256: "sha256:" + hex.EncodeToString(sourceDigest[:])}}, Facts: []fact{item}})
	if err != nil {
		panic(err)
	}
	return append(encoded, '\n')
}

func candidateFixture(candidate candidateExecutable, index int) (source, kind, predicate, value, text string) {
	suffix := strconv.Itoa(index)
	switch candidate.family {
	case "c":
		return "#define VECTOR_" + suffix + "\n", "c.macro", "declares", "VECTOR_" + suffix, "#define VECTOR_" + suffix
	case "objective-c":
		return "@interface Candidate" + suffix + "\n@end\n", "objc.interface", "declares", "Candidate" + suffix, "@interface Candidate" + suffix
	case "java":
		return "final class Candidate" + suffix + " {}\n", "java.class", "declares", "Candidate" + suffix, "final class Candidate" + suffix + " {}"
	}
	panic("unknown candidate family")
}

func independentEvidence(fields []string) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte("corvint-analyzer-native-bridge-evidence/v0"))
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(fields)))
	_, _ = hash.Write(length[:])
	for _, field := range fields {
		binary.BigEndian.PutUint32(length[:], uint32(len(field)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(field))
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}
func runCLI(t testing.TB, binary string, input []byte) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary)
	command.Stdin = bytes.NewReader(input)
	output, err := command.Output()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("cli deadline")
	}
	if err != nil {
		t.Fatal(err)
	}
	return output
}

func TestCandidateHasNoAmbientCapabilityImports(t *testing.T) {
	t.Run("NJB-005 ambient capability import audit", func(t *testing.T) {
		for _, file := range candidateSources(t) {
			parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			allowOutputOS := strings.HasPrefix(filepath.ToSlash(file), filepath.ToSlash(candidateRoot(t)+"/cmd/"))
			if forbidden := forbiddenImport(parsed, allowOutputOS); forbidden != "" {
				t.Fatalf("forbidden ambient capability import %q in %s", forbidden, file)
			}
			body, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if forbidden := forbiddenCapability(string(body)); forbidden != "" {
				t.Fatalf("forbidden capability %q in %s", forbidden, file)
			}
		}
	})
}

func TestAmbientCapabilitySpyPositiveControl(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "positive.go", `package probe; import "os/exec"`, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	if got := forbiddenImport(parsed, false); got != "os/exec" {
		t.Fatalf("spy=%q", got)
	}
	for _, control := range []struct{ source, want string }{
		{`package probe; import alias "os"; func f(){ alias.ReadFile("x") }`, "os.ReadFile"},
		{`package probe; import alias "os/exec"; func f(){ alias.Command("x") }`, "os/exec.Command"},
		{`package probe; import . "net"; func f(){ Dial("tcp","x") }`, "dot import net"},
	} {
		if got := forbiddenCapability(control.source); got != control.want {
			t.Fatalf("capability control=%q want=%q", got, control.want)
		}
	}
}
func forbiddenImport(parsed *ast.File, allowOutputOS bool) string {
	for _, item := range parsed.Imports {
		path := strings.Trim(item.Path.Value, "\"")
		for _, forbidden := range []string{"os/exec", "net", "net/http", "plugin", "syscall", "runtime/cgo"} {
			if path == forbidden {
				return path
			}
		}
		if path == "os" && !allowOutputOS {
			return path
		}
	}
	return ""
}
func forbiddenCapability(body string) string {
	parsed, err := parser.ParseFile(token.NewFileSet(), "candidate.go", body, 0)
	if err != nil {
		return "unparseable source"
	}
	imports := map[string]string{}
	for _, item := range parsed.Imports {
		path := strings.Trim(item.Path.Value, "\"")
		name := filepath.Base(path)
		if item.Name != nil {
			name = item.Name.Name
		}
		imports[name] = path
		if name == "." && (path == "os" || path == "os/exec" || strings.HasPrefix(path, "net") || path == "plugin" || path == "syscall") {
			return "dot import " + path
		}
	}
	var forbidden string
	ast.Inspect(parsed, func(node ast.Node) bool {
		if forbidden != "" {
			return false
		}
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		path := imports[ident.Name]
		if path == "" {
			return true
		}
		if path == "os" && (selector.Sel.Name == "Args" || selector.Sel.Name == "Stdin" || selector.Sel.Name == "Stdout" || selector.Sel.Name == "Exit") {
			return true
		}
		if path == "os" || path == "os/exec" || strings.HasPrefix(path, "net") || path == "plugin" || path == "syscall" || path == "runtime/cgo" {
			forbidden = path + "." + selector.Sel.Name
		}
		return true
	})
	return forbidden
}

func BenchmarkPinnedCJNI(b *testing.B) {
	vector := frame(CProfile, "c", "benchmark", "kit/src/main/cpp/beamfall_mpv.c", beamfallC)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_ = AnalyzeC(bytes.NewReader(vector))
	}
}

func TestAllocationRatchet(t *testing.T) {
	vector := frame(CProfile, "c", "ratchet", "bridge.c", beamfallC)
	production := testing.AllocsPerRun(100, func() { _ = AnalyzeC(bytes.NewReader(vector)) })
	restored := testing.AllocsPerRun(100, func() { _ = retainedWireAlternative(vector) })
	ceiling := 200.0
	if raceEnabled {
		ceiling = 240
	}
	if production > ceiling {
		t.Fatalf("production allocs=%0.f exceeds %0.f", production, ceiling)
	}
	if restored <= production+100 {
		t.Fatalf("restored alternative allocs=%0.f production=%0.f", restored, production)
	}
}

func TestLineScannerLinearWork(t *testing.T) {
	for _, size := range []int{8 << 10, 16 << 10, 32 << 10, 64 << 10, 1 << 20} {
		source := lineScannerFixture(size)
		lines := scanLines(source)
		linear := linearScanWork(source)
		restored, crossed := restoredByteZeroScanWork(source, len(source)*2)
		if len(lines) == 0 || linear != len(source) {
			t.Fatalf("size=%d lines=%d linear=%d", size, len(lines), linear)
		}
		if !crossed || restored <= len(source)*2 {
			t.Fatalf("restored quadratic scanner escaped size=%d work=%d", size, restored)
		}
	}
}

func lineScannerFixture(size int) []byte {
	line := []byte("token\n")
	result := make([]byte, 0, size)
	for len(result)+len(line) <= size {
		result = append(result, line...)
	}
	return result
}

func linearScanWork(source []byte) int {
	work := 0
	for range source {
		work++
	}
	return work
}

// restoredByteZeroScanWork is the removed locate strategy with an observation ceiling. It proves
// the old rescanning implementation breaches the linear work budget without spending O(n²) time.
func restoredByteZeroScanWork(source []byte, ceiling int) (int, bool) {
	work := 0
	for lineEnd, value := range source {
		if value != '\n' {
			continue
		}
		for offset := 0; offset < lineEnd; offset++ {
			work++
			if work > ceiling {
				return work, true
			}
		}
	}
	return work, false
}

func TestCandidateSourceSizeCeiling(t *testing.T) {
	// ACP-009 measures reachable production source per candidate CLI: every
	// executable reaches the whole shared package plus its own command, so
	// the ceiling binds shared+command per CLI (amended per-candidate rows).
	amended := map[string]int{"corvint-analyzer-c-jni": 76_380, "corvint-analyzer-java": 76_382, "corvint-analyzer-objective-c": 76_395}
	shared, commands := 0, map[string]int{}
	for _, file := range candidateSources(t) {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if dir := filepath.Base(filepath.Dir(file)); dir == "analyzernativebridge" {
			shared += len(body)
		} else {
			commands[dir] += len(body)
		}
	}
	if len(commands) != len(amended) {
		t.Fatalf("candidate command enumeration=%d want %d", len(commands), len(amended))
	}
	for name, bytes := range commands {
		ceiling, ok := amended[name]
		if !ok {
			t.Fatalf("candidate %q has no per-candidate ceiling row", name)
		}
		if shared+bytes > ceiling {
			t.Fatalf("candidate %s reachable source=%d exceeds %d", name, shared+bytes, ceiling)
		}
	}
}

// TestCandidateRootMatchesResolvedWalkPaths reproduces the macOS /tmp -> /private/tmp case: a
// checkout reached through a symlinked path must still compare equal to the real (resolved)
// directories `candidateSources` walks, or TestCandidateHasNoAmbientCapabilityImports reports
// false forbidden-import failures for every candidate command.
func TestCandidateRootMatchesResolvedWalkPaths(t *testing.T) {
	root := candidateRoot(t)
	link := filepath.Join(t.TempDir(), "corvint-symlink")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "test", "-run", "^TestCandidateHasNoAmbientCapabilityImports$", "-count=1", "./internal/analyzernativebridge")
	command.Dir = link
	// A real shell's `cd` sets $PWD to the symlinked path it was given, and
	// os.Getwd() honors that PWD when it names the same directory as the
	// resolved cwd — so PWD must be set explicitly here to reproduce what a
	// `git clone`-then-`cd` checkout does; exec.Cmd.Dir alone chdir()s the
	// child without touching the inherited (real-path) PWD.
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "PWD="+link, "GOMODCACHE="+filepath.Join(t.TempDir(), "gomodcache"))
	output, err := boundedCombinedOutput(ctx, command)
	if err != nil || ctx.Err() != nil {
		t.Fatalf("candidate root symlink repro: %v\n%s", err, output)
	}
}

func candidateRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source location unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	// runtime.Caller(0) reflects the (possibly symlinked) path the package was
	// compiled under, but candidateSources reports paths resolved by `go
	// list`/the module loader; resolve once here so both sides compare equal
	// (e.g. a checkout under macOS's /tmp -> /private/tmp).
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("resolve candidate root: %v", err)
	}
	return resolved
}
func candidateSources(t testing.TB) []string {
	t.Helper()
	packages := []string{"./internal/analyzernativebridge"}
	for _, candidate := range candidateExecutables() {
		packages = append(packages, "./cmd/"+candidate.name)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.Command("go", append([]string{"list", "-f", "{{.Dir}}|{{join .GoFiles \" \"}}"}, packages...)...)
	command.Dir = candidateRoot(t)
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOMODCACHE="+filepath.Join(t.TempDir(), "gomodcache"))
	output, err := boundedCombinedOutput(ctx, command)
	if err != nil || ctx.Err() != nil {
		t.Fatalf("candidate source enumeration: %v\n%s", err, output)
	}
	var files []string
	for _, record := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		parts := strings.SplitN(record, "|", 2)
		if len(parts) != 2 || parts[0] == "" {
			t.Fatalf("malformed candidate source receipt=%q", record)
		}
		for _, name := range strings.Fields(parts[1]) {
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				t.Fatalf("unexpected candidate source=%q", name)
			}
			files = append(files, filepath.Join(parts[0], name))
		}
	}
	if len(files) == 0 {
		t.Fatal("candidate source enumeration was empty")
	}
	sort.Strings(files)
	return files
}

// retainedWireAlternative models the rejected pre-bound implementation: it retains a copy of the
// entire request for each candidate fact instead of the production's bounded decoded body/facts.
func retainedWireAlternative(vector []byte) []byte {
	retained := make([][]byte, 512)
	for index := range retained {
		retained[index] = append([]byte(nil), vector...)
	}
	return retained[len(retained)-1]
}

func frame(profile, family, requestID, path, source string) []byte {
	sum := sha256.Sum256([]byte(source))
	req := request{Profile: profile, Family: family, RequestID: requestID, ScopeID: "beamfall", CompilationUnitID: "bridge", Target: candidateTarget(family), Inputs: []input{{Handle: "source", Family: inputFamily(family, path), Path: path, SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString([]byte(source))}}}
	data, _ := json.Marshal(req)
	return append(data, '\n')
}
func candidateTarget(family string) target {
	if family == "objective-c" {
		return target{OS: "darwin", Architecture: "arm64", ABI: "ios-17.0", Features: []string{}}
	}
	return target{OS: "android", Architecture: "arm64-v8a", ABI: "android-24", Features: []string{}}
}
func inputFamily(family, path string) string {
	if family == "java" {
		return "java.source"
	}
	if family == "objective-c" {
		return "objc.source"
	}
	if strings.HasSuffix(path, ".h") {
		return "c.header"
	}
	if path == "CMakeLists.txt" || strings.HasSuffix(path, ".cmake") {
		return "c.cmake"
	}
	return "c.source"
}
