package analyzernativebridge

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type preReviewCase struct {
	name, family, path, source, reason string
	candidate                          bool
}

type preReviewFact struct{ kind, value string }

// nativeBridgeCumulativeCorpus is the immutable regression set for every manual repair witness.
// Generated operator, loader, and JNI matrices extend this literal corpus in analyzer_test.go.
var nativeBridgeCumulativeCorpus = [...]preReviewCase{
	{"c-expression-positive", "c", "bridge.c", "int bridge(void) { int value = 2; return value + 1; }\n", "", true},
	{"objc-expression-positive", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { int value = 2; value += 1; }\n@end\n", "", true},
	{"java-library-positive", "java", "Bridge.java", "class Bridge { static { System.loadLibrary(\"bridge\"); } }\n", "", true},
	{"c-word-adjacency", "c", "bridge.c", "int bridge(void) { not valid + syntax; }\n", "UNSUPPORTED_SCHEMA", false},
	{"objc-word-adjacency", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { not valid + syntax; }\n@end\n", "UNSUPPORTED_SCHEMA", false},
	{"java-word-adjacency", "java", "Bridge.java", "class Bridge { void run() { not valid + syntax; } }\n", "UNSUPPORTED_SCHEMA", false},
	{"c-leading-xor", "c", "bridge.c", "int bridge(void) { ^ value; }\n", "UNSUPPORTED_SCHEMA", false},
	{"objc-leading-xor", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { ^ value; }\n@end\n", "UNSUPPORTED_SCHEMA", false},
	{"java-leading-xor", "java", "Bridge.java", "class Bridge { void run() { ^ value; } }\n", "UNSUPPORTED_SCHEMA", false},
	{"java-leading-dereference", "java", "Bridge.java", "class Bridge { int run() { int x = * value; return x; } }\n", "UNSUPPORTED_SCHEMA", false},
	{"java-leading-address", "java", "Bridge.java", "class Bridge { int run() { int x = & value; return x; } }\n", "UNSUPPORTED_SCHEMA", false},
	{"java-empty-parentheses", "java", "Bridge.java", "class Bridge { int run() { int x = (); return x; } }\n", "UNSUPPORTED_SCHEMA", false},
	{"java-runtime-alias", "java", "Bridge.java", "class Bridge { static { Runtime runtime = Runtime.getRuntime(); runtime.loadLibrary(\"bridge\"); } }\n", "DYNAMIC_INPUT", false},
	{"java-system-class-reflection", "java", "Bridge.java", "class Bridge { static { System.class.getMethod(\"loadLibrary\", String.class).invoke(null, \"bridge\"); } }\n", "DYNAMIC_INPUT", false},
	{"java-method-alias", "java", "Bridge.java", "class Bridge { static { Class system = Class.forName(\"java.lang.System\"); Method method = system.getDeclaredMethod(\"loadLibrary\", String.class); method.invoke(null, \"bridge\"); } }\n", "DYNAMIC_INPUT", false},
	{"jni-line-break", "c", "bridge.c", "JNIEXPORT jlong JNICALL\nJava_com_example_Bridge_open(\nJNIEnv *env,\njobject object) { return 0; }\n", "", true},
	{"jni-nested", "c", "bridge.c", "void outer(void) { JNIEXPORT jlong JNICALL Java_com_example_Bridge_open(JNIEnv *env, jclass clazz) { return 0; } }\n", "UNSUPPORTED_SCHEMA", false},
	{"c-adjacent-operands", "c", "bridge.c", "int bridge(void) { int value = left right; }\n", "UNSUPPORTED_SCHEMA", false},
	{"c-bare-colon", "c", "bridge.c", "int bridge(void) { left : right; }\n", "UNSUPPORTED_SCHEMA", false},
	{"objc-adjacent-operands", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { int value = left right; }\n@end\n", "UNSUPPORTED_SCHEMA", false},
	{"objc-bare-interface", "objective-c", "Bridge.m", "@interface\n@end\n", "UNSUPPORTED_SCHEMA", false},
	{"objc-unterminated-generic", "objective-c", "Bridge.m", "@interface Bridge\n@property NSArray<NSString *items;\n@end\n", "UNSUPPORTED_SCHEMA", false},
	{"java-record-declaration", "java", "Bridge.java", "record Bridge() {}\n", "", true},
	{"java-enum-declaration", "java", "Bridge.java", "enum Bridge {}\n", "", true},
	{"java-adjacent-operands", "java", "Bridge.java", "class Bridge { void run() { int value = left right; } }\n", "UNSUPPORTED_SCHEMA", false},
	{"c-quoted-include", "c", "bridge.c", "#include \"beamfall_mpv.h\"\n", "", true},
	{"objc-quoted-import", "objective-c", "Bridge.m", "#import \"BeamfallPlayer.h\"\n", "", true},
	{"c-unterminated-include-operand", "c", "bridge.c", "#include \"beamfall\n", "UNSUPPORTED_SCHEMA", false},
	{"c-block-extern-pointer", "c", "bridge.c", "JNIEXPORT jlong JNICALL Java_com_example_Bridge_open(JNIEnv *env, jclass clazz) {\n    extern void *eglGetProcAddress(const char *procname);\n    return (jlong)eglGetProcAddress(\"glCreateShader\");\n}\n", "", true},
	{"c-pointer-initializer", "c", "bridge.c", "JNIEXPORT jlong JNICALL Java_com_example_Bridge_open(JNIEnv *env, jclass clazz) {\n    const char *name = \"bridge\";\n    return (jlong)name;\n}\n", "", true},
	{"java-switch-labels", "java", "Bridge.java", "class Bridge { static String pick(String value) {\nswitch (value) {\ncase \"audio\":\nreturn \"a\";\ndefault:\nreturn \"b\";\n}\n}\n}\n", "", true},
	{"objc-message-send-assignment", "objective-c", "Bridge.m", "@implementation Bridge\n- (instancetype)init {\n    self = [super init];\n    return self;\n}\n@end\n", "", true},
	{"objc-implementation-ivar-block", "objective-c", "Bridge.m", "@implementation Bridge {\n    long _budget;\n}\n- (void)run { _budget += 1; }\n@end\n", "", true},
	{"java-malformed-case-label", "java", "Bridge.java", "class Bridge { static int f(int m) {\nswitch (m) {\ncase a b:\nreturn 1;\ndefault:\nreturn 0;\n}\n}\n}\n", "UNSUPPORTED_SCHEMA", false},
	{"java-case-outside-switch", "java", "Bridge.java", "class Bridge { static int f(int m) {\ncase 1:\nreturn m;\n}\n}\n", "UNSUPPORTED_SCHEMA", false},
	{"c-case-outside-switch", "c", "bridge.c", "int bridge(int m) {\ncase 1:\nreturn m;\n}\n", "UNSUPPORTED_SCHEMA", false},
	{"objc-message-extra-selector", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { self = [super init extra]; }\n@end\n", "UNSUPPORTED_SCHEMA", false},
	{"objc-subscript-adjacent-operands", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { buffer[not valid] = 0; }\n@end\n", "UNSUPPORTED_SCHEMA", false},
	{"java-switch-missing-selector-parens", "java", "Bridge.java", "class Bridge { static int f(int m) {\nswitch m {\ncase 1:\nreturn 1;\n}\n}\n}\n", "UNSUPPORTED_SCHEMA", false},
	{"c-switch-empty-selector", "c", "bridge.c", "int bridge(int m) {\nswitch () {\ncase 1:\nreturn m;\n}\n}\n", "UNSUPPORTED_SCHEMA", false},
	{"java-switch-adjacent-selector", "java", "Bridge.java", "class Bridge { static int f(int m, int n) {\nswitch (m n) {\ncase 1:\nreturn 1;\n}\n}\n}\n", "UNSUPPORTED_SCHEMA", false},
	{"c-switch-adjacent-selector", "c", "bridge.c", "int bridge(int m, int n) {\nswitch (m n) {\ncase 1:\nreturn 1;\n}\n}\n", "UNSUPPORTED_SCHEMA", false},
	{"java-literal-qualified-case-label", "java", "Bridge.java", "class Bridge { static int f(String m) {\nswitch (m) {\ncase \"x\".foo:\nreturn 1;\n}\n}\n}\n", "UNSUPPORTED_SCHEMA", false},
	{"java-case-numeric-member", "java", "Bridge.java", "class Bridge { static int f(int m) {\nswitch (m) {\ncase 1.foo:\nreturn 1;\n}\n}\n}\n", "UNSUPPORTED_SCHEMA", false},
	{"java-case-numeric-word", "java", "Bridge.java", "class Bridge { static int f(int m) {\nswitch (m) {\ncase 1abc:\nreturn 1;\n}\n}\n}\n", "UNSUPPORTED_SCHEMA", false},
	{"java-case-empty-hex", "java", "Bridge.java", "class Bridge { static int f(int m) {\nswitch (m) {\ncase 0x:\nreturn 1;\n}\n}\n}\n", "UNSUPPORTED_SCHEMA", false},
	{"java-switch-label-literals", "java", "Bridge.java", "class Bridge { static int f(int m) {\nswitch (m) {\ncase 1:\nreturn 1;\ncase 'a':\nreturn 2;\ncase \"x\":\nreturn 3;\ncase Names.VALUE:\nreturn 4;\ndefault:\nreturn 0;\n}\n}\n}\n", "", true},
	{"c-switch-suffix-labels", "c", "bridge.c", "int bridge(int m) {\nswitch (m) {\ncase 2L:\nreturn m;\ncase 3lu:\nreturn m;\ncase 4ull:\nreturn m;\n}\n}\n", "", true},
	{"objc-selector-word-after-argument", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { self = [super init:value extra]; }\n@end\n", "UNSUPPORTED_SCHEMA", false},
	{"objc-selector-after-literal-argument", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { self = [super init:1 extra]; }\n@end\n", "UNSUPPORTED_SCHEMA", false},
	{"objc-selector-after-paren-argument", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { self = [super init:(value) extra]; }\n@end\n", "UNSUPPORTED_SCHEMA", false},
	{"java-selector-nested-literal", "java", "Bridge.java", "class Bridge { static int f(int m) {\nswitch ((1.foo)) {\ncase 1:\nreturn 1;\n}\n}\n}\n", "UNSUPPORTED_SCHEMA", false},
	{"java-selector-call-literal", "java", "Bridge.java", "class Bridge { static int f(int m) {\nswitch (g(1.foo, m)) {\ncase 1:\nreturn 1;\n}\n}\n}\n", "UNSUPPORTED_SCHEMA", false},
	{"java-case-float-long-suffix", "java", "Bridge.java", "class Bridge { static int f(int m) {\nswitch (m) {\ncase 1e1L:\nreturn 1;\n}\n}\n}\n", "UNSUPPORTED_SCHEMA", false},
	{"c-case-mixed-long-suffix", "c", "bridge.c", "int bridge(int m) {\nswitch (m) {\ncase 1lUl:\nreturn m;\n}\n}\n", "UNSUPPORTED_SCHEMA", false},
	{"objc-argument-after-argument", "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { self = [super init:1 @selector(foo)]; }\n@end\n", "UNSUPPORTED_SCHEMA", false},
	{"java-missing-final-lf", "java", "Bridge.java", "class Bridge {}", "MALFORMED_INPUT", false},
}

var nativeBridgeFactExpectations = map[string]preReviewFact{
	"java-record-declaration": {"java.record", "Bridge"},
	"java-enum-declaration":   {"java.enum", "Bridge"},
	"named-interface-control": {"objc.interface", "Bridge"},
	"c-quoted-include":        {"c.include", "beamfall_mpv.h"},
	"objc-quoted-import":      {"c.include", "BeamfallPlayer.h"},
}

func TestNativeBridgeCumulativePreReviewCorpus(t *testing.T) {
	for _, test := range nativeBridgeCumulativeCorpus {
		t.Run(test.name, func(t *testing.T) {
			assertPreReviewCase(t, test)
		})
	}
}

func TestNativeBridgePositiveBodyGrammar(t *testing.T) {
	for _, test := range []struct {
		name, source string
		language     bodyLanguage
	}{
		{"c", "int bridge(void) { int value = 2; return value + 1; }\n", c},
		{"c-integer-suffixes", "int bridge(void) { unsigned long long value = 1ull; long other = 2L; return value + other + 3lu; }\n", c},
		{"java-float-suffix", "class Bridge { float run() { float value = 1e1f; return value; } }\n", java},
		{"objective-c", "@implementation Bridge\n- (void)run { int value = 2; value += 1; }\n@end\n", objc},
		{"java", "class Bridge { int run() { int value = 2; return value + 1; } }\n", java},
		{"java-record", "record Bridge()\t{}\n", java},
		{"java-enum", "enum Bridge {}\n", java},
	} {
		t.Run(test.name, func(t *testing.T) {
			lines, reason := lexicalLines([]byte(test.source))
			if reason != "" {
				t.Fatal(reason)
			}
			if reason := closedBodyReason(lines, test.language); reason != "" {
				t.Fatalf("closed body grammar rejected positive source: %s", reason)
			}
		})
	}
}

func assertPreReviewCase(t testing.TB, test preReviewCase) {
	t.Helper()
	profile := CProfile
	if test.family == "objective-c" {
		profile = ObjectiveCProfile
	}
	if test.family == "java" {
		profile = JavaProfile
	}
	encoded := testAnalyze(profile, bytes.NewReader(frame(profile, test.family, test.name, test.path, test.source)))
	if !test.candidate {
		var got rejection
		if err := json.Unmarshal(encoded, &got); err != nil || got.Reason != test.reason {
			t.Fatalf("output=%s err=%v", encoded, err)
		}
		return
	}
	var got output
	if err := json.Unmarshal(encoded, &got); err != nil || got.Status != "CANDIDATE" {
		t.Fatalf("output=%s err=%v", encoded, err)
	}
	expectation, asserted := nativeBridgeFactExpectations[test.name]
	if !asserted {
		return
	}
	for _, fact := range got.Facts {
		if fact.Kind == expectation.kind && fact.Value == expectation.value {
			return
		}
	}
	t.Fatalf("missing fact kind=%q value=%q output=%s", expectation.kind, expectation.value, encoded)
}

const (
	nativeBridgeHeldOutSeed  uint64 = 0x4e4a42484f4c444f
	nativeBridgeHeldOutCases        = 1206
)

// nativeBridgeHeldOutWitnesses is a distinct deterministic property corpus. It
// retains the nine revealed witness classes while varying only inert whitespace,
// so every generated input has one pinned outcome and no repair can hide behind
// a hand-picked example.
var nativeBridgeHeldOutWitnesses = [...]string{
	"c-adjacent-operands",
	"c-bare-colon",
	"objc-adjacent-operands",
	"objc-bare-interface",
	"objc-unterminated-generic",
	"java-record-declaration",
	"java-enum-declaration",
	"java-adjacent-operands",
	"java-missing-final-lf",
}

func TestNativeBridgeHeldOutStyleRegressionCorpus(t *testing.T) {
	corpus := nativeBridgeHeldOutStyleCorpus()
	if len(corpus) != nativeBridgeHeldOutCases || len(nativeBridgeHeldOutWitnesses) != 9 {
		t.Fatalf("cases=%d witnesses=%d", len(corpus), len(nativeBridgeHeldOutWitnesses))
	}
	counts := make(map[string]int, len(nativeBridgeHeldOutWitnesses))
	for _, test := range corpus {
		counts[test.name]++
		assertPreReviewCase(t, test)
	}
	for _, witness := range nativeBridgeHeldOutWitnesses {
		if counts[witness] != nativeBridgeHeldOutCases/len(nativeBridgeHeldOutWitnesses) {
			t.Fatalf("witness=%q cases=%d", witness, counts[witness])
		}
	}
}

func nativeBridgeHeldOutStyleCorpus() []preReviewCase {
	state := nativeBridgeHeldOutSeed
	corpus := make([]preReviewCase, 0, nativeBridgeHeldOutCases)
	for index := 0; index < nativeBridgeHeldOutCases; index++ {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		gap := " "
		if state&1 != 0 {
			gap = "\t"
		}
		corpus = append(corpus, nativeBridgeHeldOutCase(nativeBridgeHeldOutWitnesses[index%len(nativeBridgeHeldOutWitnesses)], gap))
	}
	return corpus
}

func nativeBridgeHeldOutCase(witness, gap string) preReviewCase {
	switch witness {
	case "c-adjacent-operands":
		return preReviewCase{witness, "c", "bridge.c", "int bridge(void) { int value = left" + gap + "right; }\n", "UNSUPPORTED_SCHEMA", false}
	case "c-bare-colon":
		return preReviewCase{witness, "c", "bridge.c", "int bridge(void) { left" + gap + ":" + gap + "right; }\n", "UNSUPPORTED_SCHEMA", false}
	case "objc-adjacent-operands":
		return preReviewCase{witness, "objective-c", "Bridge.m", "@implementation Bridge\n- (void)run { int value = left" + gap + "right; }\n@end\n", "UNSUPPORTED_SCHEMA", false}
	case "objc-bare-interface":
		return preReviewCase{witness, "objective-c", "Bridge.m", "@interface\n@end\n", "UNSUPPORTED_SCHEMA", false}
	case "objc-unterminated-generic":
		return preReviewCase{witness, "objective-c", "Bridge.m", "@interface Bridge\n@property NSArray<NSString" + gap + "*items;\n@end\n", "UNSUPPORTED_SCHEMA", false}
	case "java-record-declaration":
		return preReviewCase{witness, "java", "Bridge.java", "record Bridge()" + gap + "{}\n", "", true}
	case "java-enum-declaration":
		return preReviewCase{witness, "java", "Bridge.java", "enum Bridge" + gap + "{}\n", "", true}
	case "java-adjacent-operands":
		return preReviewCase{witness, "java", "Bridge.java", "class Bridge { void run() { int value = left" + gap + "right; } }\n", "UNSUPPORTED_SCHEMA", false}
	case "java-missing-final-lf":
		return preReviewCase{witness, "java", "Bridge.java", "class Bridge {}", "MALFORMED_INPUT", false}
	}
	panic("unknown held-out witness: " + witness)
}

func TestNativeBridgeHeldOutRedMutations(t *testing.T) {
	assertPreReviewCase(t, preReviewCase{"named-interface-control", "objective-c", "Bridge.m", "@interface Bridge\n@end\n", "", true})
	assertPreReviewCase(t, preReviewCase{"c-adjacency-control", "c", "bridge.c", "int bridge(void) { int value = left + right; }\n", "", true})
	assertPreReviewCase(t, nativeBridgeHeldOutCase("java-missing-final-lf", " "))
}

func TestNativeBridgeDirectiveFormsClose(t *testing.T) {
	assertPreReviewCase(t, preReviewCase{"directive-control", "c", "bridge.c", "#if FEATURE\n#define VALUE 1\n#elif OTHER\n#define VALUE 2\n#else\n#define VALUE 3\n#endif\n", "", true})
	for _, test := range []preReviewCase{
		{"directive-unmatched-else", "c", "bridge.c", "#else\n", "UNSUPPORTED_SCHEMA", false},
		{"directive-elif-after-else", "c", "bridge.c", "#if FEATURE\n#else\n#elif OTHER\n#endif\n", "UNSUPPORTED_SCHEMA", false},
		{"directive-unterminated-if", "c", "bridge.c", "#if FEATURE\n", "UNSUPPORTED_SCHEMA", false},
		{"directive-endif-tail", "c", "bridge.c", "#if FEATURE\n#endif trailing\n", "UNSUPPORTED_SCHEMA", false},
	} {
		assertPreReviewCase(t, test)
	}
}

func TestNativeBridgeGeneratedLoaderReflectionMatrix(t *testing.T) {
	for _, shape := range []string{
		"System.loadLibrary(name)",
		"Runtime.getRuntime().loadLibrary(\"bridge\")",
		"Runtime runtime = Runtime.getRuntime(); runtime.loadLibrary(\"bridge\")",
		"System.class.getMethod(\"loadLibrary\", String.class).invoke(null, \"bridge\")",
		"System.class.getDeclaredMethod(\"loadLibrary\", String.class).invoke(null, \"bridge\")",
		"Class.forName(\"java.lang.System\").getMethod(\"loadLibrary\", String.class).invoke(null, \"bridge\")",
		"Class system = Class.forName(\"java.lang.System\"); Method method = system.getDeclaredMethod(\"loadLibrary\", String.class); method.invoke(null, \"bridge\")",
	} {
		encoded := AnalyzeJava(bytes.NewReader(frame(JavaProfile, "java", "loader-matrix", "Bridge.java", "class Bridge { static { "+shape+"; } }\n")))
		var got rejection
		if err := json.Unmarshal(encoded, &got); err != nil || got.Reason != "DYNAMIC_INPUT" {
			t.Fatalf("shape=%q output=%s err=%v", shape, encoded, err)
		}
	}
	decoy := "// Runtime.getRuntime().loadLibrary(\"bridge\")\nclass Bridge { String text = \"System.class.getMethod(\\\"loadLibrary\\\", String.class)\"; }\n"
	var candidate output
	if encoded := AnalyzeJava(bytes.NewReader(frame(JavaProfile, "java", "loader-decoy", "Bridge.java", decoy))); json.Unmarshal(encoded, &candidate) != nil || candidate.Status != "CANDIDATE" {
		t.Fatalf("decoy=%s", encoded)
	}
}

func TestNativeBridgeGeneratedJNIDepthAndLineBreakMatrix(t *testing.T) {
	valid := []string{
		"JNIEXPORT jlong JNICALL Java_com_example_Bridge_open(JNIEnv *env, jclass clazz) { return 0; }\n",
		"JNIEXPORT jlong JNICALL\nJava_com_example_Bridge_open(JNIEnv *env,\njobject object) { return 0; }\n",
		"JNIEXPORT jlong JNICALL\nJava_com_example_Bridge_open(\nJNIEnv *env,\njclass clazz) { return 0; }\n",
	}
	for _, source := range valid {
		var candidate output
		if encoded := AnalyzeC(bytes.NewReader(frame(CProfile, "c", "jni-valid", "bridge.c", source))); json.Unmarshal(encoded, &candidate) != nil || candidate.Status != "CANDIDATE" {
			t.Fatalf("source=%q output=%s", source, encoded)
		}
	}
	for _, source := range []string{
		"JNIEXPORT jlong JNICALL Java_com_example_Bridge_open(JNIEnv env, jclass clazz) { return 0; }\n",
		"JNIEXPORT jlong JNICALL Java_com_example_Bridge_open(JNIEnv *env, jlong handle) { return 0; }\n",
		"void outer(void) { JNIEXPORT jlong JNICALL Java_com_example_Bridge_open(JNIEnv *env, jclass clazz) { return 0; } }\n",
	} {
		var rejected rejection
		if encoded := AnalyzeC(bytes.NewReader(frame(CProfile, "c", "jni-invalid", "bridge.c", source))); json.Unmarshal(encoded, &rejected) != nil || rejected.Reason != "UNSUPPORTED_SCHEMA" {
			t.Fatalf("source=%q output=%s", source, encoded)
		}
	}
}

func TestNativeBridgePreReviewGateManifest(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source location unavailable")
	}
	root := filepath.Join(filepath.Dir(source), "..", "..")
	script, err := os.ReadFile(filepath.Join(root, "tools", "native-bridge-pre-review.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"git status --porcelain --untracked-files=all",
		"git rev-parse HEAD",
		"git diff --check",
		"go test -count=1 -timeout=180s ./internal/analyzernativebridge ./cmd/corvint-analyzer-c-jni ./cmd/corvint-analyzer-objective-c ./cmd/corvint-analyzer-java",
		"go test -race -count=1 -timeout=180s ./internal/analyzernativebridge",
		"go vet ./internal/analyzernativebridge ./cmd/corvint-analyzer-c-jni ./cmd/corvint-analyzer-objective-c ./cmd/corvint-analyzer-java",
		"darwin/arm64 darwin/amd64 linux/arm64 linux/amd64 windows/amd64",
		"go test -exec=/usr/bin/true -run ^$",
		"\"cem_ocm\":\"NOT_RUN\"",
		"GOFLAGS=-p=1 GOMAXPROCS=2",
		"corvint-native-bridge-pre-review/v1",
		"failure_artifact",
		"CORVINT_NATIVE_BRIDGE_PRE_REVIEW_FAULT_STAGE",
	} {
		if !strings.Contains(string(script), required) {
			t.Fatalf("gate missing %q", required)
		}
	}
	archives, err := filepath.Glob(filepath.Join(root, "docs", "build-log", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range append([]string{filepath.Join(root, "docs", "BUILD-LOG.md")}, archives...) {
		buildLog, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(buildLog), "Native/JVM bridge pre-review gate PASS") {
			t.Fatalf("%s may not claim a pre-review PASS; the clean-HEAD receipt is local under .git/corvint", path)
		}
	}
}

func TestLiteralSuffixMatrix(t *testing.T) {
	accepted := []struct {
		text string
		lang bodyLanguage
	}{
		{"1ull", c}, {"1ULL", c}, {"3lu", c}, {"2L", c}, {"7u", c}, {"1llu", c},
		{"1.5f", c}, {"1e1L", c},
		{"1L", java}, {"1e1f", java}, {"1.0d", java}, {"1_000", java},
	}
	for _, tc := range accepted {
		if !literalToken(bodyToken{text: tc.text}, tc.lang) {
			t.Fatalf("valid literal %q rejected for language %v", tc.text, tc.lang)
		}
	}
	rejected := []struct {
		text string
		lang bodyLanguage
	}{
		{"1lUl", c}, {"1lL", c}, {"1uu", c}, {"1ulu", c}, {"1abc", c},
		{"1e1L", java}, {"1.foo", java}, {"1abc", java}, {"1e1uf", c},
	}
	for _, tc := range rejected {
		if literalToken(bodyToken{text: tc.text}, tc.lang) {
			t.Fatalf("invalid literal %q accepted for language %v", tc.text, tc.lang)
		}
	}
}
