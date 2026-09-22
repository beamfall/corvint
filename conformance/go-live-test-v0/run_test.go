// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

const zeroDigest = "0000000000000000000000000000000000000000000000000000000000000000"
const oneDigest = "1111111111111111111111111111111111111111111111111111111111111111"

var emptyDigest = func() string {
	digest := sha256.Sum256(nil)
	return hex.EncodeToString(digest[:])
}()

func TestCapabilityAndPlanPreimageBindingsRejectTampering(t *testing.T) {
	providerBuildBody, _ := canonical(map[string]any{"executableRawSha256": zeroDigest, "goVersion": "go1.27.1"})
	providerBuild := bareDomainDigest("go-provider-build", "go-provider-build/0", providerBuildBody)
	capability := map[string]any{
		"goProtocol": "go1.27/test2json", "operations": []any{"execute"}, "profile": "go-live-capability/0",
		"providerBuildSha256": providerBuild, "providerVersion": "0.1.0-experimental",
		"supportedContainment": []any{"PROCESS_GROUP_BEST_EFFORT"}, "supportedCoverage": []any{"NONE"}, "supportedNetwork": []any{"UNKNOWN"},
	}
	capabilityRaw, _ := canonical(capability)
	capabilityID := domainID("go-live-capability", "go-live-capability/0", capabilityRaw)
	decodedCapability, ok := decodeIdentityPreimage(base64.RawStdEncoding.EncodeToString(capabilityRaw), capabilityID, "go-live-capability", "go-live-capability/0")
	if !ok || !verifyCapabilityBody(decodedCapability, zeroDigest) {
		t.Fatal("valid closed capability rejected")
	}
	forgedCapability := cloneMap(capability)
	forgedCapability["providerVersion"] = "forged"
	forgedCapabilityRaw, _ := canonical(forgedCapability)
	forgedCapabilityID := domainID("go-live-capability", "go-live-capability/0", forgedCapabilityRaw)
	decodedCapability, ok = decodeIdentityPreimage(base64.RawStdEncoding.EncodeToString(forgedCapabilityRaw), forgedCapabilityID, "go-live-capability", "go-live-capability/0")
	if !ok || verifyCapabilityBody(decodedCapability, zeroDigest) {
		t.Fatal("forged capability body with recomputed id accepted")
	}
	forgedBuildCapability := cloneMap(capability)
	forgedBuildCapability["providerBuildSha256"] = oneDigest
	forgedBuildRaw, _ := canonical(forgedBuildCapability)
	forgedBuildID := domainID("go-live-capability", "go-live-capability/0", forgedBuildRaw)
	decodedCapability, ok = decodeIdentityPreimage(base64.RawStdEncoding.EncodeToString(forgedBuildRaw), forgedBuildID, "go-live-capability", "go-live-capability/0")
	if !ok || verifyCapabilityBody(decodedCapability, zeroDigest) {
		t.Fatal("forged provider build with recomputed capability id accepted")
	}

	environment := []any{map[string]any{"name": "GOENV", "valueSha256": zeroDigest}}
	environmentRaw, _ := canonical(environment)
	environmentID := bareDomainDigest("go-environment", "go-environment/0", environmentRaw)
	toolchain := map[string]any{
		"cgoEnabled": "0", "goarch": "amd64", "goenvSha256": zeroDigest, "goexeSha256": zeroDigest, "goos": "linux",
		"gorootSha256": zeroDigest, "goversion": "go1.27.1", "invokedToolsSha256": zeroDigest,
		"pathSha256": zeroDigest, "toolDirSha256": zeroDigest,
	}
	toolchainRaw, _ := canonical(toolchain)
	toolchainID := domainID("go-toolchain", "go-toolchain/0", toolchainRaw)
	toolchain["id"] = toolchainID
	discovery := &discoveryDocument{
		DependencyMaterializationSHA256: zeroDigest, EnvironmentSHA256: environmentID,
		ID: "go-live-discovery:sha256:" + zeroDigest, ModuleMode: "MODULE",
		SourceWSI: "workspace-source:sha256:" + zeroDigest, ToolchainID: toolchainID,
	}
	context := transcriptContext{
		ActualEnvironmentSHA256: environmentID, CapabilityID: capabilityID, DiscoveryID: discovery.ID,
		GOARCH: "amd64", GOOS: "linux", PlanID: "", RequestedPackagePatterns: []string{"example.test/probe"},
		VerifierExecutableSHA256: zeroDigest,
	}
	plan := map[string]any{
		"capabilityId": capabilityID, "coverage": map[string]any{"mode": "NONE", "packagePatterns": []any{}},
		"discoveryId": discovery.ID, "environment": environment,
		"invocation": map[string]any{"argv": []any{"@PINNED_GO@", "test", "-json", "-count=1", "-vet=off"}, "cwdPathSha256": zeroDigest, "packagePatterns": []any{"example.test/probe"}},
		"limits": map[string]any{
			"coverageBytes": "268435456", "coverageFiles": "4096", "cpuMilliseconds": "1800000", "eventBytes": "16777216",
			"events": "100000", "lineBytes": "1048576", "memoryBytes": "4294967296", "openFiles": "4096", "outputBytes": "8388608",
			"packages": "4096", "processes": "1024", "runMilliseconds": "1800000", "tests": "100000",
		},
		"limitsEnforced": []any{"EVENT_BYTES", "EVENTS", "LINE_BYTES", "OUTPUT_BYTES", "PACKAGES", "RUN_TIME", "TESTS"},
		"network":        map[string]any{"mechanism": nil, "mode": "UNKNOWN"}, "profile": "go-live-plan/0",
		"scope":     map[string]any{"conclusion": "UNKNOWN", "excluded": []any{}, "requestedPackagePatterns": []any{"example.test/probe"}, "unknownReasons": stringAny(mandatoryUnknownReasons)},
		"source":    map[string]any{"dependencyMaterializationSha256": zeroDigest, "materializationSha256": zeroDigest, "moduleMode": "MODULE", "rootPathSha256": zeroDigest, "wsi": discovery.SourceWSI},
		"toolchain": toolchain,
	}
	planRaw, _ := canonical(plan)
	context.PlanID = domainID("go-live-plan", "go-live-plan/0", planRaw)
	decodedPlan, ok := decodeIdentityPreimage(base64.RawStdEncoding.EncodeToString(planRaw), context.PlanID, "go-live-plan", "go-live-plan/0")
	if !ok || !verifyPlanBody(decodedPlan, context, discovery) {
		t.Fatal("valid closed plan rejected")
	}
	forgedPlan := cloneMap(plan)
	forgedLimits := cloneMap(plan["limits"].(map[string]any))
	forgedLimits["outputBytes"] = "1"
	forgedPlan["limits"] = forgedLimits
	forgedPlanRaw, _ := canonical(forgedPlan)
	forgedPlanID := domainID("go-live-plan", "go-live-plan/0", forgedPlanRaw)
	decodedPlan, ok = decodeIdentityPreimage(base64.RawStdEncoding.EncodeToString(forgedPlanRaw), forgedPlanID, "go-live-plan", "go-live-plan/0")
	if !ok || verifyPlanBody(decodedPlan, context, discovery) {
		t.Fatal("forged plan body with recomputed id accepted")
	}
	forgedToolchainPlan := cloneMap(plan)
	forgedToolchain := cloneMap(toolchain)
	forgedToolchain["goexeSha256"] = oneDigest
	forgedToolchainPlan["toolchain"] = forgedToolchain
	forgedToolchainRaw, _ := canonical(forgedToolchainPlan)
	forgedToolchainPlanID := domainID("go-live-plan", "go-live-plan/0", forgedToolchainRaw)
	decodedPlan, ok = decodeIdentityPreimage(base64.RawStdEncoding.EncodeToString(forgedToolchainRaw), forgedToolchainPlanID, "go-live-plan", "go-live-plan/0")
	forgedContext := context
	forgedContext.PlanID = forgedToolchainPlanID
	if !ok || verifyPlanBody(decodedPlan, forgedContext, discovery) {
		t.Fatal("forged toolchain body retaining old toolchain id accepted")
	}
	forgedRootPlan := cloneMap(plan)
	forgedSource := cloneMap(plan["source"].(map[string]any))
	forgedSource["rootPathSha256"] = oneDigest
	forgedRootPlan["source"] = forgedSource
	forgedRootRaw, _ := canonical(forgedRootPlan)
	forgedRootPlanID := domainID("go-live-plan", "go-live-plan/0", forgedRootRaw)
	decodedPlan, ok = decodeIdentityPreimage(base64.RawStdEncoding.EncodeToString(forgedRootRaw), forgedRootPlanID, "go-live-plan", "go-live-plan/0")
	forgedContext.PlanID = forgedRootPlanID
	if !ok || verifyPlanBody(decodedPlan, forgedContext, discovery) {
		t.Fatal("mismatched source root and invocation cwd accepted")
	}
}

func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func pointer(value string) *string { return &value }

func canonicalPackage(kind string, depOnly bool, forTest *string, match []string, importPath, name string) map[string]any {
	inputFiles := []any{map[string]any{"mode": "0644", "pathSha256": zeroDigest, "rawSha256": oneDigest}}
	inputRaw, _ := canonical(inputFiles)
	inputDigest := bareDomainDigest("go-input-set", "go-input-set/0", inputRaw)
	body := map[string]any{
		"depOnly": depOnly, "dirPathSha256": zeroDigest, "forTest": optional(forTest),
		"importPath": importPath, "inputFilesSha256": inputDigest, "kind": kind,
		"match": stringArray(match), "moduleSha256": nil, "name": name,
	}
	raw, _ := canonical(body)
	object := map[string]any{}
	for key, value := range body {
		object[key] = value
	}
	object["id"] = domainID("go-package", "go-package/0", raw)
	object["inputFiles"] = inputFiles
	object["imports"] = []any{}
	object["module"] = nil
	object["testImports"] = []any{}
	object["xTestImports"] = []any{}
	return object
}

func validDiscoveryObject() map[string]any {
	requested := canonicalPackage("REQUESTED", false, nil, []string{"./..."}, "example.test/a", "a")
	dependency := canonicalPackage("DEPENDENCY", true, nil, nil, "example.test/dep", "dep")
	packages := []map[string]any{requested, dependency}
	sortObjects(packages)
	packageValues := []any{packages[0], packages[1]}
	body := map[string]any{
		"argv":                            []any{"@PINNED_GO@", "list", "-deps", "-test", "-json=" + closedListFields, "./..."},
		"dependencyMaterializationSha256": oneDigest, "environmentSha256": zeroDigest,
		"exitCode": "0", "inputsSha256": zeroDigest, "moduleMode": "MODULE",
		"packages": packageValues, "profile": discoveryProfile, "rawStderrSha256": emptyDigest,
		"rawStdoutSha256": oneDigest, "requestedRunnerPackages": []any{"example.test/a"},
		"sourceWsi":   "workspace-source:sha256:" + zeroDigest,
		"toolchainId": "go-toolchain:sha256:" + oneDigest,
	}
	rehashDiscovery(body)
	return body
}

func packageInputObject(object map[string]any) map[string]any {
	return map[string]any{
		"depOnly": object["depOnly"], "forTest": object["forTest"], "id": object["id"],
		"imports": object["imports"], "match": object["match"], "moduleSha256": object["moduleSha256"],
		"name": object["name"], "testImports": object["testImports"], "xTestImports": object["xTestImports"],
	}
}

func rehashDiscovery(object map[string]any) {
	packageInputs := make([]any, 0)
	for _, value := range object["packages"].([]any) {
		packageInputs = append(packageInputs, packageInputObject(value.(map[string]any)))
	}
	sortValues(packageInputs)
	inputsBody := map[string]any{
		"dependencyMaterializationSha256": object["dependencyMaterializationSha256"],
		"moduleMode":                      object["moduleMode"], "packages": packageInputs, "sourceWsi": object["sourceWsi"],
	}
	inputsRaw, _ := canonical(inputsBody)
	object["inputsSha256"] = bareDomainDigest("go-discovery-inputs", "go-discovery-inputs/0", inputsRaw)
	raw, _ := canonical(without(object, "id"))
	object["id"] = domainID("go-live-discovery", discoveryProfile, raw)
}

func rehashPackageObject(pkg map[string]any) {
	parsed := &packageDocument{
		DepOnly: pkg["depOnly"].(bool), DirPathSHA256: pkg["dirPathSha256"].(string),
		ForTest: interfaceString(pkg["forTest"]), ImportPath: pkg["importPath"].(string),
		InputFilesSHA256: pkg["inputFilesSha256"].(string), Kind: pkg["kind"].(string),
		Match: interfaceStrings(pkg["match"]), ModuleSHA256: interfaceString(pkg["moduleSha256"]),
		Name: pkg["name"].(string),
	}
	body, _ := canonical(packageIdentityObject(parsed))
	pkg["id"] = domainID("go-package", "go-package/0", body)
}

func sortValues(values []any) {
	sort.Slice(values, func(left, right int) bool {
		leftRaw, _ := canonical(values[left])
		rightRaw, _ := canonical(values[right])
		return bytes.Compare(leftRaw, rightRaw) < 0
	})
}

func sortObjects(values []map[string]any) {
	left, _ := canonical(values[0])
	right, _ := canonical(values[1])
	if bytes.Compare(left, right) > 0 {
		values[0], values[1] = values[1], values[0]
	}
}

func validDiscoveryBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := canonical(validDiscoveryObject())
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func TestDiscoveryPositiveVectorAndHashPreimages(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(sourceDir(t), "testdata", "valid-discovery.json"))
	if err != nil {
		t.Fatal(err)
	}
	if generated := validDiscoveryBytes(t); !bytes.Equal(generated, raw) {
		t.Fatal("fixed producer-independent golden differs from local fixture builder")
	}
	document, err := VerifyDiscovery(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Packages) != 2 || len(document.RequestedRunnerPackages) != 1 {
		t.Fatalf("unexpected discovery: %#v", document)
	}
	if document.ID != "go-live-discovery:sha256:9f1f891aee28764be0b7232fe00500855888f87670db157f44c294962d913d37" || document.InputsSHA256 != "f182565a5e25b1f0eabfd412a67863b72f82dec0af0669adc2c90979a80b2bc3" {
		t.Fatalf("fixed identities changed: %s %s", document.ID, document.InputsSHA256)
	}
	body, _ := canonical(without(document.raw, "id"))
	if got := domainID("go-live-discovery", discoveryProfile, body); got != document.ID {
		t.Fatalf("discovery id = %s", got)
	}
	// The framing must not collapse to a naive concatenation.
	naive := sha256.Sum256(append(append([]byte("go-live-discovery"), []byte(discoveryProfile)...), body...))
	if strings.HasSuffix(document.ID, hex.EncodeToString(naive[:])) {
		t.Fatal("domain framing collapsed to naive concatenation")
	}
}

func TestActualProducerDiscoveryFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(sourceDir(t), "testdata", "producer-discovery.json"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != "97d19a5d98589e81928a74c752d0005b0a3ea2e7398f894d499dd351e8fa27bd" {
		t.Fatal("producer discovery bytes changed")
	}
	document, err := VerifyDiscovery(raw)
	if err != nil {
		t.Fatal(err)
	}
	if document.ID != "go-live-discovery:sha256:ace16b4deb4389283a0e5338fb1220bfc27a8d768923115174036179c17fd4f4" || document.InputsSHA256 != "f9ccab167542740136b7c469d1b56ce6e3e013d8c9d3a806cba2ce9cd3593a00" {
		t.Fatalf("producer identities changed: %s %s", document.ID, document.InputsSHA256)
	}
}

func TestFixedLiteralNegativeVectors(t *testing.T) {
	cases := []struct {
		raw  string
		code rejectCode
	}{
		{"{\"argv\":[],\"argv\":[]}\n", canonicalDuplicateKey},
		{"{\"secretPath\":\"/private/source\"}\n", canonicalUnknownField},
		{"{ \"argv\":[]}\n", canonicalNoncanonical},
	}
	for _, test := range cases {
		_, err := VerifyDiscovery([]byte(test.raw))
		if codeOf(err) != test.code {
			t.Fatalf("error = %v, want %s", err, test.code)
		}
	}
}

func TestCanonicalBytes(t *testing.T) {
	raw := validDiscoveryBytes(t)
	if !bytes.HasSuffix(raw, []byte{'\n'}) || bytes.Contains(raw[:len(raw)-1], []byte(" ")) {
		t.Fatalf("not compact canonical JSON: %q", raw)
	}
	edges := map[string]any{"control": "\b\t\n\f\r\x00", "literal": "<&>\u2028"}
	got, err := canonical(edges)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"control\":\"\\b\\t\\n\\f\\r\\u0000\",\"literal\":\"<&>\u2028\"}"
	if string(got) != want {
		t.Fatalf("canonical = %q", got)
	}
}

func TestFixedDomainFramingVector(t *testing.T) {
	const want = "go-package:sha256:7cf5cfd18f7c14e006e98da823537c85030335ab5b0a633ec71febf49f82e17f"
	if got := domainID("go-package", "go-package/0", []byte("{}")); got != want {
		t.Fatalf("framed digest = %s", got)
	}
}

func TestDiscoveryNegativeVectors(t *testing.T) {
	valid := validDiscoveryBytes(t)
	if !duplicateKeys([]byte(`{"argv":[],"argv":[]}`)) {
		t.Fatal("duplicate-key scanner missed a repeated object key")
	}
	cases := []struct {
		name string
		raw  []byte
		code rejectCode
	}{
		{"missing-lf", bytes.TrimSuffix(valid, []byte{'\n'}), canonicalNoncanonical},
		{"trailing-whitespace", append(append([]byte(nil), bytes.TrimSuffix(valid, []byte{'\n'})...), ' ', '\n'), canonicalNoncanonical},
		{"invalid-utf8", []byte{'{', 0xff, '}', '\n'}, canonicalInvalidUTF8},
		{"duplicate-field", bytes.Replace(valid, []byte(`{"argv":`), []byte(`{"argv":[],"argv":`), 1), canonicalDuplicateKey},
		{"unknown-field", mutateObject(t, func(object map[string]any) { object["sourcePath"] = "/secret/repo" }, false), canonicalUnknownField},
		{"missing-field", mutateObject(t, func(object map[string]any) { delete(object, "sourceWsi") }, false), discoveryDecode},
		{"number", bytes.Replace(valid, []byte(`"exitCode":"0"`), []byte(`"exitCode":0`), 1), canonicalNoncanonical},
		{"wrong-discovery-id", mutateObject(t, func(object map[string]any) { object["id"] = "go-live-discovery:sha256:" + zeroDigest }, false), identityMismatch},
		{"wrong-package-id", mutateObject(t, func(object map[string]any) {
			object["packages"].([]any)[0].(map[string]any)["id"] = "go-package:sha256:" + zeroDigest
		}, false), identityMismatch},
		{"classification-mismatch", mutateObject(t, func(object map[string]any) { object["packages"].([]any)[0].(map[string]any)["kind"] = "TEST_MAIN" }, true), discoveryDecode},
		{"requested-commitment-mismatch", mutateObject(t, func(object map[string]any) { object["requestedRunnerPackages"] = []any{"example.test/wrong"} }, true), discoveryInput},
		{"empty-requested-runner-set", mutateObject(t, func(object map[string]any) { object["requestedRunnerPackages"] = []any{} }, true), discoveryInput},
		{"duplicate-import-path", mutateObject(t, func(object map[string]any) {
			packages := object["packages"].([]any)
			packages[1].(map[string]any)["importPath"] = packages[0].(map[string]any)["importPath"]
		}, true), discoveryDecode},
		{"requested-import-path-forgery", mutateObject(t, func(object map[string]any) {
			pkg := object["packages"].([]any)[0].(map[string]any)
			pkg["importPath"] = "/private/source"
			object["requestedRunnerPackages"] = []any{"/private/source"}
		}, true), discoveryInput},
		{"match-not-in-argv", mutateObject(t, func(object map[string]any) {
			object["packages"].([]any)[0].(map[string]any)["match"] = []any{"example.test/forged"}
		}, true), discoveryInput},
		{"input-file-commitment-mismatch", mutateObject(t, func(object map[string]any) {
			object["packages"].([]any)[0].(map[string]any)["inputFiles"].([]any)[0].(map[string]any)["rawSha256"] = zeroDigest
		}, true), identityMismatch},
		{"invalid-mode", mutateObject(t, func(object map[string]any) {
			object["packages"].([]any)[0].(map[string]any)["inputFiles"].([]any)[0].(map[string]any)["mode"] = "1644"
		}, true), discoveryInput},
		{"unordered-requested", mutateObject(t, func(object map[string]any) { object["requestedRunnerPackages"] = []any{"z", "a"} }, true), discoveryInput},
		{"noncharacter", bytes.Replace(valid, []byte("example.test/a"), []byte("example.test/\xef\xb7\x90"), 1), discoveryInput},
		{"raw-cache-path", mutateObject(t, func(object map[string]any) {
			object["packages"].([]any)[0].(map[string]any)["dir"] = "/host/cache/go-build/x"
		}, false), canonicalUnknownField},
		{"ambiguous-generated-input", mutateObject(t, func(object map[string]any) {
			object["packages"].([]any)[0].(map[string]any)["generatedInput"] = "testmain.go"
		}, false), canonicalUnknownField},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := VerifyDiscovery(test.raw)
			if err == nil || codeOf(err) != test.code {
				t.Fatalf("error = %v (%s), want %s; raw-prefix=%q", err, codeOf(err), test.code, test.raw[:min(len(test.raw), 96)])
			}
		})
	}
}

func mutateObject(t *testing.T, mutate func(map[string]any), rehash bool) []byte {
	t.Helper()
	object := validDiscoveryObject()
	mutate(object)
	if rehash {
		for _, value := range object["packages"].([]any) {
			rehashPackageObject(value.(map[string]any))
		}
		rehashDiscovery(object)
	}
	raw, err := canonical(object)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func interfaceString(value any) *string {
	if value == nil {
		return nil
	}
	text := value.(string)
	return &text
}

func interfaceStrings(value any) []string {
	items := value.([]any)
	result := make([]string, len(items))
	for index, item := range items {
		result[index] = item.(string)
	}
	return result
}

func TestModuleAndInputCommitmentVectors(t *testing.T) {
	module := moduleBody{
		DirPathSHA256: pointer(zeroDigest), GoModPathSHA256: pointer(oneDigest), GoModSum: pointer("h1:mod"),
		GoVersion: pointer("1.27"), Path: "example.test/a", Sum: pointer("h1:sum"),
		Version: pointer("v1.0.0"),
	}
	moduleRaw, _ := canonical(moduleObject(module))
	moduleDigest := bareDomainDigest("go-module", "go-module/0", moduleRaw)
	if err := VerifyModuleIdentity(moduleDigest, module); err != nil {
		t.Fatal(err)
	}
	if err := VerifyModuleIdentity(zeroDigest, module); codeOf(err) != identityMismatch {
		t.Fatalf("module mismatch = %v", err)
	}
	nullableModule := module
	nullableModule.DirPathSHA256 = nil
	nullableModule.GoModPathSHA256 = nil
	nullableRaw, _ := canonical(moduleObject(nullableModule))
	nullableDigest := bareDomainDigest("go-module", "go-module/0", nullableRaw)
	if err := VerifyModuleIdentity(nullableDigest, nullableModule); err != nil {
		t.Fatalf("nullable module paths: %v", err)
	}
	replacement := moduleBody{DirPathSHA256: pointer(oneDigest), GoModPathSHA256: nil, Path: "example.test/替換"}
	withReplacement := module
	withReplacement.Replace = &replacement
	replacementRaw, _ := canonical(moduleObject(withReplacement))
	replacementDigest := bareDomainDigest("go-module", "go-module/0", replacementRaw)
	if err := VerifyModuleIdentity(replacementDigest, withReplacement); err != nil {
		t.Fatalf("module replacement: %v", err)
	}
	tooDeep := module
	tooDeep.Replace = &moduleBody{DirPathSHA256: pointer(zeroDigest), GoModPathSHA256: pointer(oneDigest), Path: "example.test/replacement", Replace: &moduleBody{Path: "forbidden"}}
	if err := VerifyModuleIdentity(zeroDigest, tooDeep); codeOf(err) != discoveryInput {
		t.Fatalf("nested replacement = %v", err)
	}

	entries := []inputEntry{{Mode: "0644", PathSHA256: zeroDigest, RawSHA256: oneDigest}}
	entryObjects := []any{map[string]any{"mode": "0644", "pathSha256": zeroDigest, "rawSha256": oneDigest}}
	inputRaw, _ := canonical(entryObjects)
	inputDigest := bareDomainDigest("go-input-set", "go-input-set/0", inputRaw)
	if err := VerifyInputSetIdentity(inputDigest, entries); err != nil {
		t.Fatal(err)
	}
	if err := VerifyInputSetIdentity(zeroDigest, entries); codeOf(err) != identityMismatch {
		t.Fatalf("input mismatch = %v", err)
	}
	if err := VerifyInputSetIdentity(inputDigest, append(entries, entries[0])); codeOf(err) != discoveryInput {
		t.Fatalf("duplicate input = %v", err)
	}
	emptyRaw, _ := canonical([]any{})
	emptyInputDigest := bareDomainDigest("go-input-set", "go-input-set/0", emptyRaw)
	if err := VerifyInputSetIdentity(emptyInputDigest, nil); err != nil {
		t.Fatalf("empty input set: %v", err)
	}

	pkg := validDiscoveryObject()["packages"].([]any)[0].(map[string]any)
	inputPackage := packageInput{
		DepOnly: pkg["depOnly"].(bool), ForTest: nil, ID: pkg["id"].(string),
		Imports: []string{"fmt"}, Match: []string{"./..."}, Name: pkg["name"].(string),
	}
	inputs := discoveryInputs{DependencyMaterializationSHA256: oneDigest, ModuleMode: "MODULE", Packages: []packageInput{inputPackage}, SourceWSI: "workspace-source:sha256:" + zeroDigest}
	packageObject := map[string]any{
		"depOnly": inputPackage.DepOnly, "forTest": nil, "id": inputPackage.ID,
		"imports": []any{"fmt"}, "match": []any{"./..."}, "moduleSha256": nil,
		"name": inputPackage.Name, "testImports": []any{}, "xTestImports": []any{},
	}
	inputsObject := map[string]any{"dependencyMaterializationSha256": oneDigest, "moduleMode": "MODULE", "packages": []any{packageObject}, "sourceWsi": inputs.SourceWSI}
	inputsRaw, _ := canonical(inputsObject)
	inputsDigest := bareDomainDigest("go-discovery-inputs", "go-discovery-inputs/0", inputsRaw)
	if err := VerifyDiscoveryInputsIdentity(inputsDigest, inputs); err != nil {
		t.Fatal(err)
	}
	inputs.ModuleMode = "UNKNOWN"
	if err := VerifyDiscoveryInputsIdentity(inputsDigest, inputs); codeOf(err) != discoveryInput {
		t.Fatalf("module-mode mismatch = %v", err)
	}
}

func TestPublishedModulePreimageIsWiredIntoDiscovery(t *testing.T) {
	module := moduleBody{
		DirPathSHA256: pointer(zeroDigest), GoModPathSHA256: pointer(oneDigest),
		GoVersion: pointer("1.27"), Main: true, Path: "example.test/a",
	}
	moduleRaw, _ := canonical(moduleObject(module))
	moduleDigest := bareDomainDigest("go-module", "go-module/0", moduleRaw)
	raw := mutateObject(t, func(object map[string]any) {
		pkg := object["packages"].([]any)[0].(map[string]any)
		pkg["module"] = moduleObject(module)
		pkg["moduleSha256"] = moduleDigest
	}, true)
	if _, err := VerifyDiscovery(raw); err != nil {
		t.Fatal(err)
	}
}

func TestClassificationAndPatternEdges(t *testing.T) {
	forTest := "example.test/a"
	variant := &packageDocument{ForTest: &forTest, Match: []string{"./..."}}
	if kind, ok := classify(variant); !ok || kind != "TEST_VARIANT" {
		t.Fatalf("test variant with Match = %s, %v", kind, ok)
	}
	dependency := &packageDocument{DepOnly: true, Match: []string{"./..."}}
	if kind, ok := classify(dependency); !ok || kind != "DEPENDENCY" {
		t.Fatalf("dependency with Match = %s, %v", kind, ok)
	}
	for _, invalid := range [][]string{{"example.test/a\x00b"}, {"example.test/a\x7fb"}, {"example.test/a@v1"}, {"example.test/a,b"}, {"example.test/*"}, {"example.test\\a"}, {"example.test//a"}, {"example.test/./a"}, {"example.test/../a"}, {"example.test/.../a"}, {"-example.test/a"}, {"./...", "example.test/a"}} {
		if validPatterns(invalid) {
			t.Fatalf("accepted invalid patterns: %q", invalid)
		}
	}
	if validPatterns([]string{"example.test/a\u00a0b"}) {
		t.Fatal("accepted Unicode whitespace")
	}
	if !validPatterns([]string{"example.test/a...b"}) {
		t.Fatal("rejected non-segment ellipsis")
	}
	generated := inputEntry{Mode: "0644", RawSHA256: oneDigest}
	generated.PathSHA256 = generatedTestMainPath("example.test/a.test", generated)
	testMain := &packageDocument{ImportPath: "example.test/a.test", Kind: "TEST_MAIN", Name: "main", InputFiles: []inputEntry{generated}}
	if kind, ok := classify(testMain); !ok || kind != "TEST_MAIN" || generated.PathSHA256 != generatedTestMainPath(testMain.ImportPath, generated) {
		t.Fatal("invalid TEST_MAIN generated-file commitment")
	}
}

func TestMatchUnionAndArgvAggregateBounds(t *testing.T) {
	unused := mutateObject(t, func(object map[string]any) {
		object["argv"] = []any{"@PINNED_GO@", "list", "-deps", "-test", "-json=" + closedListFields, "example.test/a", "example.test/b"}
		pkg := object["packages"].([]any)[0].(map[string]any)
		pkg["match"] = []any{"example.test/a"}
	}, true)
	if _, err := VerifyDiscovery(unused); codeOf(err) != discoveryInput {
		t.Fatalf("unused requested pattern = %v", err)
	}

	patterns := make([]any, 4091)
	for index := range patterns {
		patterns[index] = fmt.Sprintf("example.test/%04d/%s", index, strings.Repeat("a", 1010))
	}
	overflow := mutateObject(t, func(object map[string]any) {
		object["argv"] = append([]any{"@PINNED_GO@", "list", "-deps", "-test", "-json=" + closedListFields}, patterns...)
		object["packages"].([]any)[0].(map[string]any)["match"] = append([]any(nil), patterns...)
	}, true)
	if _, err := VerifyDiscovery(overflow); codeOf(err) != limitExceeded {
		t.Fatalf("argv aggregate overflow = %v", err)
	}
}

func TestDiscoveryInputBodiesSortIndependently(t *testing.T) {
	first := packageInput{DepOnly: true, ID: "go-package:sha256:" + zeroDigest, Name: "z"}
	second := packageInput{DepOnly: false, ID: "go-package:sha256:" + oneDigest, Match: []string{"./..."}, Name: "a"}
	inputs := discoveryInputs{
		DependencyMaterializationSHA256: oneDigest, ModuleMode: "MODULE",
		Packages: []packageInput{first, second}, SourceWSI: "workspace-source:sha256:" + zeroDigest,
	}
	objects := []any{
		map[string]any{"depOnly": true, "forTest": nil, "id": first.ID, "imports": []any{}, "match": []any{}, "moduleSha256": nil, "name": "z", "testImports": []any{}, "xTestImports": []any{}},
		map[string]any{"depOnly": false, "forTest": nil, "id": second.ID, "imports": []any{}, "match": []any{"./..."}, "moduleSha256": nil, "name": "a", "testImports": []any{}, "xTestImports": []any{}},
	}
	sortValues(objects)
	body, _ := canonical(map[string]any{"dependencyMaterializationSha256": oneDigest, "moduleMode": "MODULE", "packages": objects, "sourceWsi": inputs.SourceWSI})
	digest := bareDomainDigest("go-discovery-inputs", "go-discovery-inputs/0", body)
	if err := VerifyDiscoveryInputsIdentity(digest, inputs); err != nil {
		t.Fatal(err)
	}
}

func TestProducerLikeRichDiscoveryVector(t *testing.T) {
	requested := canonicalPackage("REQUESTED", false, nil, []string{"./..."}, "example.test/a", "a")
	replacement := moduleBody{DirPathSHA256: pointer(oneDigest), Path: "example.test/替換", Version: pointer("v1.2.3")}
	module := moduleBody{
		DirPathSHA256: pointer(zeroDigest), GoModPathSHA256: pointer(oneDigest), GoVersion: pointer("1.27"),
		Main: true, Path: "example.test/模組", Replace: &replacement,
	}
	moduleRaw, _ := canonical(moduleObject(module))
	moduleDigest := bareDomainDigest("go-module", "go-module/0", moduleRaw)
	requested["module"] = moduleObject(module)
	requested["moduleSha256"] = moduleDigest
	rehashPackageObject(requested)

	forTest := "example.test/a"
	variant := canonicalPackage("TEST_VARIANT", false, &forTest, []string{"./..."}, "example.test/a [example.test/a.test]", "a")
	testMain := canonicalPackage("TEST_MAIN", false, nil, nil, "example.test/a.test", "main")
	testMain["dirPathSha256"] = oneDigest
	entry := inputEntry{Mode: "0644", RawSHA256: oneDigest}
	entry.PathSHA256 = generatedTestMainPath(testMain["importPath"].(string), entry)
	entryObject := map[string]any{"mode": entry.Mode, "pathSha256": entry.PathSHA256, "rawSha256": entry.RawSHA256}
	inputRaw, _ := canonical([]any{entryObject})
	testMain["inputFiles"] = []any{entryObject}
	testMain["inputFilesSha256"] = bareDomainDigest("go-input-set", "go-input-set/0", inputRaw)
	rehashPackageObject(testMain)
	dependency := canonicalPackage("DEPENDENCY", true, nil, nil, "example.test/依存", "依存")

	packages := []any{requested, variant, testMain, dependency}
	sortValues(packages)
	object := map[string]any{
		"argv":                            []any{"@PINNED_GO@", "list", "-deps", "-test", "-json=" + closedListFields, "./..."},
		"dependencyMaterializationSha256": oneDigest, "environmentSha256": zeroDigest,
		"exitCode": "0", "inputsSha256": zeroDigest, "moduleMode": "WORKSPACE",
		"packages": packages, "profile": discoveryProfile, "rawStderrSha256": emptyDigest,
		"rawStdoutSha256": oneDigest, "requestedRunnerPackages": []any{"example.test/a"},
		"sourceWsi": "workspace-source:sha256:" + zeroDigest, "toolchainId": "go-toolchain:sha256:" + oneDigest,
	}
	rehashDiscovery(object)
	raw, _ := canonical(object)
	document, err := VerifyDiscovery(append(raw, '\n'))
	if err != nil {
		t.Fatal(err)
	}
	if document.ID != "go-live-discovery:sha256:068f7bcc75c00011211c83f755e77c36b4efe4baf2e3d232abdb125683f2fee1" || document.InputsSHA256 != "446055bb3ff7eb44ce78b33cee0245f03267e926136a430b5867ae3b83915b52" || moduleDigest != "d588bda16a5e65cc9c517cc14c665c8dcc8c65fc1b9364b759930004aa13f370" || entry.PathSHA256 != "4c5f86e74ebb59d485ba824fc41952e7aca210eec7073e56ae055dafe6518454" {
		t.Fatalf("rich fixed identities: discovery=%s inputs=%s module=%s testmain=%s", document.ID, document.InputsSHA256, moduleDigest, entry.PathSHA256)
	}
	publishedIDs := make([]string, len(packages))
	inputBodies := make([]any, len(packages))
	for index, value := range packages {
		pkg := value.(map[string]any)
		publishedIDs[index] = pkg["id"].(string)
		inputBodies[index] = packageInputObject(pkg)
	}
	sortValues(inputBodies)
	inputIDs := make([]string, len(inputBodies))
	for index, value := range inputBodies {
		inputIDs[index] = value.(map[string]any)["id"].(string)
	}
	if equalStrings(publishedIDs, inputIDs) {
		t.Fatal("fixture does not exercise independent package-input ordering")
	}
}

func TestBoundsAndPrivacy(t *testing.T) {
	oversized := bytes.Repeat([]byte{'x'}, maxDocumentBytes+1)
	if _, err := VerifyDiscovery(oversized); codeOf(err) != limitExceeded {
		t.Fatalf("oversized = %v", err)
	}
	deep := []byte(`{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{"i":{"j":{"k":{"l":{"m":{"n":{"o":{"p":{"q":null}}}}}}}}}}}}}}}}}` + "\n")
	if _, err := VerifyDiscovery(deep); codeOf(err) != limitExceeded {
		t.Fatalf("deep = %v", err)
	}
	depth16 := []byte(`{"a":` + strings.Repeat("[", 15) + `null` + strings.Repeat("]", 15) + `}` + "\n")
	if _, err := parseCanonical(depth16); err != nil {
		t.Fatalf("depth 16 rejected: %v", err)
	}
	depth17 := []byte(`{"a":` + strings.Repeat("[", 16) + `null` + strings.Repeat("]", 16) + `}` + "\n")
	if _, err := parseCanonical(depth17); codeOf(err) != limitExceeded {
		t.Fatalf("depth 17 = %v", err)
	}
	wide := []byte("[" + strings.Repeat("null,", maxArrayEntries) + "null]\n")
	if _, err := parseCanonical(wide); codeOf(err) != limitExceeded {
		t.Fatalf("wide array = %v", err)
	}

	directory := t.TempDir()
	path := filepath.Join(directory, "discovery.json")
	if err := os.WriteFile(path, validDiscoveryBytes(t), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if exit := run([]string{path}, &stdout, &stderr); exit != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.Bytes(), stderr.Bytes())
	}
	if bytes.Contains(stdout.Bytes(), []byte("example.test")) || bytes.Contains(stdout.Bytes(), []byte(directory)) || bytes.Contains(stdout.Bytes(), []byte("discovery.json")) {
		t.Fatalf("CLI leaked source data: %q", stdout.Bytes())
	}
	want := "{\"acquisitionPreimages\":\"NOT_RUN\",\"packages\":\"2\",\"profile\":\"go-live-discovery-conformance/0\",\"publishedCommitments\":\"VERIFIED\",\"runnerPackages\":\"1\",\"status\":\"PASS\"}\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q", stdout.String())
	}
	symlinkPath := filepath.Join(directory, "discovery-link.json")
	if err := os.Symlink(path, symlinkPath); err == nil {
		stdout.Reset()
		if exit := run([]string{symlinkPath}, &stdout, &stderr); exit != 1 || !bytes.Contains(stdout.Bytes(), []byte(`"code":"DISCOVERY_INPUT"`)) {
			t.Fatalf("symlink CLI exit=%d stdout=%q", exit, stdout.Bytes())
		}
	}
	oversizedPath := filepath.Join(directory, "oversized.json")
	file, err := os.Create(oversizedPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxDocumentBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if exit := run([]string{oversizedPath}, &stdout, &stderr); exit != 1 || !bytes.Contains(stdout.Bytes(), []byte(`"code":"LIMIT_EXCEEDED"`)) {
		t.Fatalf("oversized CLI exit=%d stdout=%q", exit, stdout.Bytes())
	}
}

func FuzzVerifyDiscovery(f *testing.F) {
	f.Add([]byte("{}\n"))
	raw, _ := canonical(validDiscoveryObject())
	f.Add(append(raw, '\n'))
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _ = VerifyDiscovery(raw)
	})
}

func TestCrossBuildSupportedTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-build skipped in short mode")
	}
	root := filepath.Clean(filepath.Join(sourceDir(t), "..", ".."))
	for _, target := range [][2]string{{"linux", "amd64"}, {"windows", "amd64"}, {"freebsd", "amd64"}} {
		name := strings.Join(target[:], "-")
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			output := filepath.Join(t.TempDir(), "verifier")
			if target[0] == "windows" {
				output += ".exe"
			}
			command := testCommand(t, root, output)
			command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "CGO_ENABLED=0", "GOOS="+target[0], "GOARCH="+target[1])
			if combined, err := command.CombinedOutput(); err != nil {
				t.Fatalf("cross-build: %v: %s", err, combined)
			}
		})
	}
}

func sourceDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate source")
	}
	return filepath.Dir(file)
}
