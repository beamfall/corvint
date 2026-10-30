package analyzerstructured

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func testInput(handle string, profile FormatProfile, logicalPath, body string) Input {
	sum := sha256.Sum256([]byte(body))
	return Input{Handle: handle, Family: profile, Path: logicalPath, SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString([]byte(body))}
}
func testFrame(t *testing.T, input Input) []byte {
	t.Helper()
	encoded, err := json.Marshal(Request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []Input{input}})
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func TestExactFormatTupleMatrix(t *testing.T) {
	cases := []struct {
		profile    FormatProfile
		path, body string
	}{
		{JSONProfile, "contracts/fixture.json", `{"schema":"v1","active":true}`},
		{JSONLProfile, "contracts/events.jsonl", "{\"event\":\"one\"}\n{\"event\":\"two\"}\n"},
		{YAMLProfile, "config/fixture.yaml", "service: beamfall\nenabled: true\n"},
		{TOMLProfile, "config/fixture.toml", "version = \"1.0.0\"\nretry = 3\n"},
		{XMLProfile, "res/fixture.xml", "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<manifest><application/></manifest>"},
		{PlistProfile, "Apps/Fixture.plist", "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<plist version=\"1.0\"><dict/></plist>"},
		{PropertiesProfile, "gradle.properties", "android.useAndroidX=true\n"},
		{HCLProfile, "fixture.hcl", "enabled = true\n"},
		{WebManifestProfile, "web/manifest.webmanifest", `{"name":"Beamfall","display":"standalone","icons":[{"src":"icon.png","sizes":"512x512"}]}`},
		{SVGProfile, "assets/logo.svg", `<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0"/></svg>`},
	}
	for _, tc := range cases {
		t.Run(string(tc.profile), func(t *testing.T) {
			output := Analyze(testFrame(t, testInput("input-1", tc.profile, tc.path, tc.body)))
			var result success
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatal(err)
			}
			if result.Status != "CANDIDATE" || len(result.Facts) != 1 || result.Facts[0].Format != tc.profile || result.Facts[0].Path != "$" || result.Facts[0].ByteEnd != len(tc.body) {
				t.Fatalf("unexpected output: %s", output)
			}
		})
	}
}

func TestClosedReasonsAndFullRejectionBinding(t *testing.T) {
	valid := testInput("input-1", JSONProfile, "contract.json", `{"a":1}`)
	for _, tc := range []struct {
		name       string
		profile    FormatProfile
		body, want string
	}{
		{"json-duplicate", JSONProfile, `{"a":1,"a":2}`, "MALFORMED_INPUT"},
		{"yaml-alias", YAMLProfile, "a: &x value\nb: *x\n", "UNSUPPORTED_SCHEMA"},
		{"toml-date", TOMLProfile, "released = 2026-08-25\n", "MALFORMED_INPUT"},
		{"xml-dtd", XMLProfile, "<!DOCTYPE x><x/>", "UNSUPPORTED_SCHEMA"},
		{"properties-continuation", PropertiesProfile, "a=one\\\ntwo\n", "MALFORMED_INPUT"},
		{"hcl-expression", HCLProfile, "tag = \"${CHANNEL}\"\n", "UNSUPPORTED_SCHEMA"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			valid.Family = tc.profile
			valid.ContentBase64 = base64.StdEncoding.EncodeToString([]byte(tc.body))
			sum := sha256.Sum256([]byte(tc.body))
			valid.SHA256 = "sha256:" + hex.EncodeToString(sum[:])
			output := string(Analyze(testFrame(t, valid)))
			if !strings.Contains(output, `"reason":"`+tc.want+`"`) || !strings.Contains(output, `"scope_id":"root"`) || !strings.Contains(output, valid.SHA256) {
				t.Fatalf("unbound rejection: %s", output)
			}
		})
	}
	if output := string(Analyze([]byte(`{"profile":"x"}` + "\n"))); output != `{"profile":"corvint-structured-data/experimental-v1","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}`+"\n" {
		t.Fatalf("sentinel=%s", output)
	}
}

// These are literal complete pinned Beamfall blobs. Their expected rejections
// are evidence of closed-profile safety, not a claim that the source is wrong.
const pinnedBeamfallProperties = "android.useAndroidX=true\nandroid.nonTransitiveRClass=true\nkotlin.code.style=official\norg.gradle.jvmargs=-Xmx3g -Dfile.encoding=UTF-8\n"
const pinnedBeamfallJSONL = "{\"formatVersion\":\"1.0.0\",\"class\":\"library\",\"id\":\"library-1\",\"updatedAt\":\"2026-07-11T00:00:00Z\",\"payload\":{\"title\":\"Household Library\",\"pluginID\":\"org.example.generic\",\"restricted\":false,\"configuration\":{\"scanMode\":\"scheduled\"}}}\n"
const pinnedBeamfallHCL = "group \"default\" {\n  targets = [\"beamfall-core\"]\n}\n\ntarget \"beamfall-core\" {\n  dockerfile = \"Dockerfile\"\n  context    = \".\"\n  platforms  = [\"linux/amd64\", \"linux/arm64\"]\n  tags = [\n    \"ghcr.io/beamfall/core:${CHANNEL:-stable}\",\n    \"ghcr.io/beamfall/core:${VERSION:-development}-${CHANNEL:-stable}\",\n  ]\n\n  # Supply-chain attestations for the server OCI image (PANEL-11, ADR-0013 §A):\n  # BuildKit emits an in-toto SBOM (SPDX) and SLSA provenance (mode=max records\n  # the full build graph) attached to the image index, so a puller can inspect\n  # exactly what the image contains and how it was built. The image is also\n  # cosign-signed at push time in .github/workflows/release.yml; the signature is\n  # what the blessed unattended-upgrade path (Watchtower-style) verifies.\n  attest = [\n    \"type=sbom\",\n    \"type=provenance,mode=max\",\n  ]\n}\n"
const pinnedBeamfallAndroidManifest = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\">\n    <application>\n        <activity\n            android:name=\"com.beamfall.kit.LibmpvTestActivity\"\n            android:exported=\"false\"\n            android:theme=\"@android:style/Theme.Material.NoActionBar.Fullscreen\" />\n    </application>\n</manifest>\n"

func TestPinnedBeamfallDogfoodLiterals(t *testing.T) {
	cases := []struct {
		profile                    FormatProfile
		path, body, digest, reason string
	}{
		{PropertiesProfile, "gradle.properties", pinnedBeamfallProperties, "f37de70e9ddb470b6f286d0c6173e71be72cb9e8", ""},
		{JSONLProfile, "contracts/export/v1/fixtures/full-state/records/library.jsonl", pinnedBeamfallJSONL, "0a6acd1fb9fc8b4e000214a3cc2234a6c3b4e058", ""},
		{HCLProfile, "docker-bake.hcl", pinnedBeamfallHCL, "703bfd80e42eccf0131519dc9c276a051bafcf1a", "UNSUPPORTED_SCHEMA"},
		{XMLProfile, "kit/src/androidTest/AndroidManifest.xml", pinnedBeamfallAndroidManifest, "3e9e4647b0ef36b19e9d7b639de7252b29aa7d54", "UNSUPPORTED_SCHEMA"},
	}
	for _, tc := range cases {
		t.Run(string(tc.profile), func(t *testing.T) {
			if sha := sha1Text(tc.body); sha != tc.digest {
				t.Fatalf("literal drift: %s", sha)
			}
			output := string(Analyze(testFrame(t, testInput("input-1", tc.profile, tc.path, tc.body))))
			if tc.reason == "" && !strings.Contains(output, `"status":"CANDIDATE"`) {
				t.Fatalf("unexpected reject: %s", output)
			}
			if tc.reason != "" && !strings.Contains(output, `"reason":"`+tc.reason+`"`) {
				t.Fatalf("unexpected result: %s", output)
			}
		})
	}
}
func sha1Text(value string) string {
	sum := sha1.Sum(append([]byte("blob "+strconv.Itoa(len(value))+"\x00"), []byte(value)...))
	return hex.EncodeToString(sum[:])
}
