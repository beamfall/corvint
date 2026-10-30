package doccompiler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLexicalConfigNeverQualifiesAndRejectsExecutableBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		config string
		want   string
	}{
		{"minimal-material-is-only-preflight", "site_name: Corvint\ntheme:\n  name: material\n", "PRECHECK_PASS"},
		{"inherit", "INHERIT: base.yml\ntheme:\n  name: material\n", "PRECHECK_FAILED"},
		{"include", "theme:\n  name: material\nextra: !include extras.yml\n", "PRECHECK_FAILED"},
		{"hook", "theme:\n  name: material\nhooks:\n  - hook.py\n", "PRECHECK_FAILED"},
		{"unknown-plugin", "theme:\n  name: material\nplugins:\n  - search\n  - privacy\n", "PRECHECK_FAILED"},
		{"search-not-readded", "theme:\n  name: material\nplugins: []\n", "PRECHECK_FAILED"},
		{"unknown-extension", "theme:\n  name: material\nmarkdown_extensions:\n  - hostile.extension\n", "PRECHECK_FAILED"},
		{"snippets-path-reader", "theme:\n  name: material\nmarkdown_extensions:\n  - pymdownx.snippets\n", "PRECHECK_FAILED"},
		{"remote-asset", "theme:\n  name: material\nextra_css:\n  - https://cdn.example/x.css\n", "PRECHECK_FAILED"},
		{"privacy-fetch", "theme:\n  name: material\nplugins:\n  - privacy:\n      assets_fetch: true\n", "PRECHECK_FAILED"},
		{"offline-plugin", "theme:\n  name: material\nplugins:\n  - offline\n", "PRECHECK_FAILED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observation := observeConfig([]byte(test.config))
			status, _ := lexicalMaterialPreflight(observation)
			if status != test.want {
				t.Fatalf("status = %q, want %q; observation=%+v", status, test.want, observation)
			}
			if status == "QUALIFIED" {
				t.Fatal("lexical observation qualified a config")
			}
		})
	}
}

func TestDiscoverUsesPinnedMkDocsAuthorityForMaterialProfile(t *testing.T) {
	root, err := projectRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	config := `site_name: Corvint
site_url: https://example.test/corvint
theme:
  name: material
  features:
    - navigation.tabs
  custom_dir: overrides
plugins:
  - search
markdown_extensions:
  - admonition
  - pymdownx.details
extra_css:
  - assets/local.css
extra_javascript:
  - assets/local.js
validation:
  nav:
    omitted_files: warn
`
	writeTestFile(t, root, "mkdocs.yml", config)
	writeTestFile(t, root, "docs/index.md", "# Corvint\n")
	writeTestFile(t, root, "docs/assets/local.css", "body{}\n")
	writeTestFile(t, root, "docs/assets/local.js", "void 0;\n")
	writeTestFile(t, root, ".venv/requirements.lock", "mkdocs==1.6.1\nmkdocs-material==9.6.0\nMarkdown==3.8\npymdown-extensions==10.16\n")
	if err := os.MkdirAll(filepath.Join(root, "overrides"), 0700); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, root, ".venv/bin/mkdocs", "#!/bin/sh\nprintf 'mkdocs, version 1.6.1'\n")
	inventory := `{"implementation":"CPython","packages":[["Markdown","3.8"],["mkdocs","1.6.1"],["mkdocs-material","9.6.0"],["pymdown-extensions","10.16"]],"python":"3.13.7"}`
	emptyOptions := textDigest("{}")
	navDigest := textDigest("null")
	validationJSON := `{"nav":{"omitted_files":"warn"}}`
	authority := `{"config_file_path":"` + filepath.Join(root, "mkdocs.yml") + `","docs_dir":"` + filepath.Join(root, "docs") + `","extra_css":["assets/local.css"],"extra_javascript":["assets/local.js"],"hooks":[],"markdown_extension_options":[{"name":"admonition","options_sha256":"` + emptyOptions + `"},{"name":"pymdownx.details","options_sha256":"` + emptyOptions + `"}],"markdown_extensions":["admonition","pymdownx.details"],"nav_configured":false,"nav_sha256":"` + navDigest + `","plugin_options":[{"name":"search","options_sha256":"` + emptyOptions + `"}],"plugins":["search"],"site_dir":"` + filepath.Join(root, "site") + `","site_url":"https://example.test/corvint","theme_custom_dir":"` + filepath.Join(root, "overrides") + `","theme_features":["navigation.tabs"],"theme_name":"material","theme_options_sha256":"` + textDigest(`{"name":"material"}`) + `","use_directory_urls":true,"validation":` + validationJSON + `,"validation_configured":true,"validation_sha256":"` + textDigest(validationJSON) + `"}`
	pythonScript := "#!/bin/sh\ncase \"$3\" in\n  *importlib.metadata*) printf '%s' '" + inventory + "' ;;\n  *) printf '%s' '" + authority + "' ;;\nesac\n"
	writeExecutable(t, root, ".venv/bin/python", pythonScript)
	_, lockPin, err := readPinned(root, ".venv/requirements.lock", defaultSourceBytes)
	if err != nil {
		t.Fatal(err)
	}
	discoverOptions := DiscoverOptions{
		ProjectRoot: root, MkDocsPath: ".venv/bin/mkdocs", PythonPath: ".venv/bin/python",
		ProjectLockPath: ".venv/requirements.lock", ExpectedProjectLockSHA256: lockPin.SHA256,
		ExpectedMkDocsVersion: "1.6.1", ExpectedMaterialVersion: "9.6.0",
		ExpectedMarkdownVersion: "3.8", ExpectedPyMdownVersion: "10.16",
	}
	inventoryDigest := testEnvironmentDigest(t, root, inventory, lockPin, discoverOptions)
	trust := EnvironmentTrustAttestation{
		Profile: "corvint-doccompiler-environment-trust/0", EnvironmentSHA256: hex.EncodeToString(inventoryDigest[:]),
		Authority: "repository owner", Revision: strings.Repeat("a", 40),
	}
	discoverOptions.TrustAttestation = trust
	environment, err := discover(t.Context(), discoverOptions)
	if err != nil {
		t.Fatal(err)
	}
	if environment.Qualification != "QUALIFIED" || !strings.Contains(environment.Observations.ParserStatus, "MkDocs-authoritative") {
		t.Fatalf("environment=%+v", environment)
	}
	if environment.Observations.SiteURL != "https://example.test/corvint" || environment.Observations.SiteDir != "site" || environment.Observations.NavOwner != "mkdocs.yml" || environment.Observations.SearchStatus != "explicit-effective" || !contains(environment.Observations.ThemeFeatures, "navigation.tabs") {
		t.Fatalf("observations=%+v", environment.Observations)
	}
	if len(environment.Toolchain.Distributions) != 4 || environment.Toolchain.PyMdownVersion != "10.16" {
		t.Fatalf("toolchain=%+v", environment.Toolchain)
	}
	if environment.Toolchain.ProjectLock.SHA256 != lockPin.SHA256 || environment.Observations.NavSHA256 != navDigest || environment.Observations.ValidationSHA256 != textDigest(validationJSON) {
		t.Fatalf("missing authority pins: toolchain=%+v observations=%+v", environment.Toolchain, environment.Observations)
	}
	if len(environment.Observations.PluginOptions) != 1 || len(environment.Observations.MarkdownExtensionOptions) != 2 || environment.Observations.MarkdownExtensionOptions[1].Name != "pymdownx.details" {
		t.Fatalf("option ownership/order=%+v %+v", environment.Observations.PluginOptions, environment.Observations.MarkdownExtensionOptions)
	}
}

func TestAuthoritativeDocsDirIsConfigRelativeLikeLexical(t *testing.T) {
	root := t.TempDir()
	digest := textDigest("null")
	authority := authoritativeConfig{
		ConfigFilePath: filepath.Join(root, "site", "mkdocs.yml"), DocsDir: filepath.Join(root, "site", "docs"),
		SiteDir: filepath.Join(root, "site", "site"), ThemeCustomDir: filepath.Join(root, "site", "overrides"),
		NavSHA256: digest, ValidationSHA256: digest, ThemeOptionsSHA256: digest,
	}
	observation, _, err := applyAuthoritativeConfig(root, "site/mkdocs.yml", ConfigObservations{}, authority)
	if err != nil {
		t.Fatal(err)
	}
	docsRoot, docsErr := resolveConfigRelative("site/mkdocs.yml", observation.DocsDir)
	customRoot, customErr := resolveConfigRelative("site/mkdocs.yml", observation.ThemeCustomDir)
	if docsErr != nil || customErr != nil || docsRoot != "site/docs" || customRoot != "site/overrides" {
		t.Fatalf("docs=%q custom=%q errors=%v %v", docsRoot, customRoot, docsErr, customErr)
	}
	authority.DocsDir = filepath.Join(root, "docs")
	var failed *Error
	if _, _, err := applyAuthoritativeConfig(root, "site/mkdocs.yml", ConfigObservations{}, authority); !errors.As(err, &failed) || failed.Code != "mkdocs-docs-dir-escape" {
		t.Fatalf("docs_dir outside the config directory was not refused: %v", err)
	}
}

func TestDiscoverDoesNotRunMkDocsConfigAuthorityForUnknownExtension(t *testing.T) {
	root, err := projectRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "mkdocs.yml", "theme:\n  name: material\nmarkdown_extensions:\n  - hostile.extension\n")
	writeTestFile(t, root, ".venv/requirements.lock", "pinned\n")
	writeExecutable(t, root, ".venv/bin/mkdocs", "#!/bin/sh\nprintf 'mkdocs, version 1.6.1'\n")
	marker := filepath.Join(root, "authority-ran")
	inventory := `{"implementation":"CPython","packages":[["Markdown","3.8"],["mkdocs","1.6.1"],["mkdocs-material","9.6.0"],["pymdown-extensions","10.16"]],"python":"3.13.7"}`
	pythonScript := "#!/bin/sh\ncase \"$3\" in\n  *importlib.metadata*) printf '%s' '" + inventory + "' ;;\n  *) printf x > '" + marker + "'; exit 9 ;;\nesac\n"
	writeExecutable(t, root, ".venv/bin/python", pythonScript)
	_, lockPin, err := readPinned(root, ".venv/requirements.lock", defaultSourceBytes)
	if err != nil {
		t.Fatal(err)
	}
	options := DiscoverOptions{
		ProjectRoot: root, MkDocsPath: ".venv/bin/mkdocs", PythonPath: ".venv/bin/python",
		ProjectLockPath: ".venv/requirements.lock", ExpectedProjectLockSHA256: lockPin.SHA256,
		ExpectedMkDocsVersion: "1.6.1", ExpectedMaterialVersion: "9.6.0", ExpectedMarkdownVersion: "3.8", ExpectedPyMdownVersion: "10.16",
	}
	inventoryDigest := testEnvironmentDigest(t, root, inventory, lockPin, options)
	options.TrustAttestation = EnvironmentTrustAttestation{Profile: "corvint-doccompiler-environment-trust/0", EnvironmentSHA256: hex.EncodeToString(inventoryDigest[:]), Authority: "owner", Revision: strings.Repeat("a", 40)}
	environment, err := discover(t.Context(), options)
	if err != nil {
		t.Fatal(err)
	}
	if environment.Qualification != "UNQUALIFIED" {
		t.Fatalf("qualification=%s", environment.Qualification)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("hostile extension reached MkDocs config authority")
	}
}

func TestDiscoverRequiresProjectLockAndAllExactVersions(t *testing.T) {
	root := t.TempDir()
	if _, err := discover(t.Context(), DiscoverOptions{ProjectRoot: root, MkDocsPath: ".venv/bin/mkdocs"}); errorCode(err) != "project-lock-pin-required" {
		t.Fatalf("missing lock error = %v", err)
	}
	writeTestFile(t, root, "requirements.lock", "pinned\n")
	lockDigest := fileDigest(t, filepath.Join(root, "requirements.lock"))
	options := DiscoverOptions{
		ProjectRoot: root, MkDocsPath: ".venv/bin/mkdocs", ProjectLockPath: "requirements.lock",
		ExpectedProjectLockSHA256: lockDigest,
	}
	if _, err := discover(t.Context(), options); errorCode(err) != "version-pins-required" {
		t.Fatalf("missing versions error = %v", err)
	}
}

func TestPlanRejectsHallucinatedSupportedClaim(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "mkdocs.yml", "theme:\n  name: material\n")
	environment := testPlanningEnvironment(t, root)
	withoutEvidence := PlanOptions{Documents: []DocumentProposal{{
		Path: "proposal.md", Claims: []Claim{{ID: "C1", Status: "SUPPORTED", Text: "invented"}},
	}}}
	if _, err := plan(environment, withoutEvidence); errorCode(err) != "unsupported-supported-claim" {
		t.Fatalf("error = %v", err)
	}
}

func TestPlanRejectsSymlinkAndPathEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeTestFile(t, root, "mkdocs.yml", "theme:\n  name: material\n")
	writeTestFile(t, outside, "secret.md", "secret\n")
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(root, "linked.md")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readPinned(root, "linked.md", 1024); errorCode(err) != "symlink-rejected" {
		t.Fatalf("symlink error = %v", err)
	}
	if _, _, err := readPinned(root, "../secret.md", 1024); errorCode(err) != "path-escape" {
		t.Fatalf("escape error = %v", err)
	}
}

func TestPlanProducesExactDocumentAndNavBytePatchesForIsolatedCandidate(t *testing.T) {
	root := t.TempDir()
	config := "site_name: Corvint\nnav:\n  - Old: old.md\ntheme:\n  name: material\n"
	writeTestFile(t, root, "mkdocs.yml", config)
	writeTestFile(t, root, "docs/old.md", "# Old\n")
	writeTestFile(t, root, "docs/proposal.md", "# Accepted prose\n")
	environment := testPlanningEnvironment(t, root)
	environment.Observations.NavSHA256 = textDigest(`[{"Old":"old.md"}]`)
	patch, err := plan(environment, PlanOptions{
		Documents: []DocumentProposal{{Path: "proposal.md", Claims: []Claim{{ID: "C1", Status: "UNKNOWN", Text: "candidate", Uncertainty: "requires review"}}}},
		Nav:       []NavEntry{{Title: "Candidate", Path: "proposal.md"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if patch.Documents[0].Operation != "insert_after" || patch.Nav.Operation != "edit_nav" || patch.Documents[0].Patch.StartByte != len("# Accepted prose\n") || patch.Documents[0].Patch.EndByte != len("# Accepted prose\n") {
		t.Fatalf("document patch=%+v", patch.Documents[0])
	}
	stage := t.TempDir()
	if err := stageCorpus(environment, patch, stage, BuildOptions{}); err != nil {
		t.Fatal(err)
	}
	stagedDocument, err := os.ReadFile(filepath.Join(stage, "docs/proposal.md"))
	if err != nil || string(stagedDocument) != patch.Documents[0].RenderedMarkdown || !strings.HasPrefix(string(stagedDocument), "# Accepted prose\n") {
		t.Fatalf("staged document=%q err=%v", stagedDocument, err)
	}
	stagedConfig, err := os.ReadFile(filepath.Join(stage, "mkdocs.yml"))
	if err != nil || strings.Contains(string(stagedConfig), "Old: old.md") || !strings.Contains(string(stagedConfig), `"Candidate": "proposal.md"`) {
		t.Fatalf("staged config=%q err=%v", stagedConfig, err)
	}
}

// The expected bytes and digest were produced by python3 3.9.6 running the authority
// probe's plain() and json.dumps(sort_keys=True, separators=(",",":")) over this nav.
func TestProposedNavDigestMatchesMkDocsProbeSpelling(t *testing.T) {
	probeBytes := `[{"Caf\u00e9 <A> & B\u2028\ud83d\ude00":"caf\u00e9.md"},{"q\"\\\t\u0001\u007f/":"a/b.md"}]`
	const probeDigest = "4b4ad2e9babcd9901f5d228ce48fee24931c37c8674147a33f2c9f58110532a6"
	got := proposedNavDigest([]NavEntry{
		{Title: "Caf\u00e9 <A> & B\u2028\U0001F600", Path: "caf\u00e9.md"},
		{Title: "q\"\\\t\x01\x7f/", Path: "a/b.md"},
	})
	if textDigest(probeBytes) != probeDigest || got != probeDigest {
		t.Fatalf("digest=%s", got)
	}
}

// PyYAML 6.0.3 yaml.safe_load refuses literal U+007F..U+009F (NEL U+0085 folds as a line
// break), U+FFFE, and U+FFFF, and loaded these escaped bytes back to the exact title and path.
func TestRenderNavEscapesCharactersYAMLCannotCarry(t *testing.T) {
	got := renderNav([]NavEntry{{Title: "a\x7f\u0080\u0085\u009f\u00a0\ufeff\ufffe\uffff\u2028<b", Path: "c\u0085.md"}})
	want := "nav:\n  - \"a\\u007f\\u0080\\u0085\\u009f\u00a0\ufeff\\ufffe\\uffff\\u2028\\u003cb\": \"c\\u0085.md\"\n"
	if got != want {
		t.Fatalf("nav=%q", got)
	}
}

// PyYAML 6.0.3 yaml.safe_load refuses an implicit key longer than 1024 stream characters
// (quotes and escapes included) and loaded the explicit "? " form back to the exact title.
func TestRenderNavWritesLongTitlesAsExplicitKeys(t *testing.T) {
	fits, long := strings.Repeat("t", 1022), strings.Repeat("t", 1023)
	got := renderNav([]NavEntry{{Title: fits, Path: "a.md"}, {Title: long, Path: "b.md"}})
	want := "nav:\n  - \"" + fits + "\": \"a.md\"\n  - ? \"" + long + "\"\n    : \"b.md\"\n"
	if got != want {
		t.Fatalf("nav=%q", got)
	}
}

func TestBuildRefusesUncontainedUnixExecution(t *testing.T) {
	root := t.TempDir()
	environment := testBuildEnvironment(t, root, successfulBuildScript())
	patch, err := plan(environment, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := build(t.Context(), environment, patch, testBuildOptions(environment))
	if errorCode(err) != "process-containment-unsupported" || result.BuildStrictStatus != "NOT_RUN" || !strings.Contains(result.ProcessContainment, "setsid") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestValidateSiteRejectsFalseStrictPassAndOutputBounds(t *testing.T) {
	for _, test := range []struct {
		name      string
		content   string
		maxOutput int64
		wantCode  string
	}{
		{"zero-without-site", "", 1024, "false-strict-pass"},
		{"oversized-site", "strict", 4, "build-output-too-large"},
	} {
		t.Run(test.name, func(t *testing.T) {
			output := t.TempDir()
			if test.content != "" {
				writeTestFile(t, output, "index.html", test.content)
			}
			_, _, _, err := validateSite(output, BuildOptions{MaxOutputBytes: test.maxOutput})
			if errorCode(err) != test.wantCode {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestOfflineQualificationCannotBePromotedAndGeneratedAssetsAreScanned(t *testing.T) {
	result := minimalReceipt().Build
	if _, err := QualifyOffline(result, OfflineAttestation{}); errorCode(err) != "offline-observer-required" {
		t.Fatalf("error = %v", err)
	}
	output := t.TempDir()
	writeTestFile(t, output, "index.html", `<script src="https://cdn.example/x.js"></script>`)
	found, err := scanGeneratedAssets(output, BuildOptions{})
	if err != nil || found != "index.html" {
		t.Fatalf("found=%q err=%v", found, err)
	}
}

func TestCanonicalReceiptFixedVectorAndRoundTrip(t *testing.T) {
	receipt := minimalReceipt()
	encoded, digest, err := CanonicalReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(encoded), "\n") || strings.HasSuffix(string(encoded), "\n\n") {
		t.Fatalf("canonical receipt must have exactly one trailing LF: %q", encoded[len(encoded)-2:])
	}
	const expectedDigest = "8abcb9be4b65d39e3805516286b3773b34460107052f3146adc531017b5aa8bd"
	if digest != expectedDigest {
		t.Fatalf("fixed vector digest = %s", digest)
	}
	var decoded Receipt
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	reencoded, secondDigest, err := CanonicalReceipt(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(reencoded) != string(encoded) || secondDigest != digest {
		t.Fatal("receipt round-trip changed canonical bytes")
	}
}

func minimalReceipt() Receipt {
	return Receipt{
		Profile: ReceiptProfile, DeliveryStage: "experimental", Claim: "UNPROVEN", MaterialProfile: MaterialProfile,
		Toolchain: Toolchain{
			MkDocs:        FilePin{Path: ".venv/bin/mkdocs", SHA256: strings.Repeat("1", 64), Size: 1},
			Python:        FilePin{Path: ".venv/bin/python", SHA256: strings.Repeat("2", 64), Size: 1},
			ProjectLock:   FilePin{Path: ".venv/requirements.lock", SHA256: strings.Repeat("8", 64), Size: 1},
			MkDocsVersion: "1.6.1", MaterialVersion: "9.6.0", MarkdownVersion: "3.8",
			PyMdownVersion: "10.16",
			PythonVersion:  "CPython 3.13", EnvironmentSHA256: strings.Repeat("3", 64), DistributionCount: 1,
			Distributions: []Distribution{{Name: "mkdocs", Version: "1.6.1"}},
		},
		Config:              FilePin{Path: "mkdocs.yml", SHA256: strings.Repeat("4", 64), Size: 1},
		ConfigObservations:  ConfigObservations{Authority: "mkdocs", ParserStatus: "authority", DocsDir: "docs", ThemeName: "material"},
		ConfigQualification: "QUALIFIED", PlanSHA256: strings.Repeat("5", 64),
		Build: BuildResult{
			BuildStrictStatus: "PASS", OfflineQualification: "NOT_OBSERVED", OutputSHA256: strings.Repeat("6", 64),
			OutputBytes: 1, OutputFiles: 1, OfflineEnforcement: "environment hardening only; no OS network-denial boundary",
		},
		Evidence:    []ReceiptEvidence{},
		Uncertainty: []string{"offline NOT_OBSERVED"},
	}
}

func testPlanningEnvironment(t *testing.T, root string) Environment {
	t.Helper()
	_, pin, err := readPinned(root, "mkdocs.yml", defaultConfigBytes)
	if err != nil {
		t.Fatal(err)
	}
	return Environment{
		ProjectRoot:  root,
		Config:       FilePin{Path: pin.Path, SHA256: pin.SHA256, Size: pin.Size},
		Observations: ConfigObservations{DocsDir: "docs"},
	}
}

func testBuildEnvironment(t *testing.T, root, script string) Environment {
	t.Helper()
	resolvedRoot, err := projectRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	root = resolvedRoot
	writeExecutable(t, root, ".venv/bin/mkdocs", script)
	emptyDigest := textDigest("{}")
	pythonScript := `#!/bin/sh
config="$4"
stage="${config%/mkdocs.yml}"
printf '%s' '{"config_file_path":"'"$config"'","docs_dir":"'"$stage"'/docs","extra_css":[],"extra_javascript":[],"hooks":[],"markdown_extension_options":[],"markdown_extensions":[],"nav_configured":false,"nav_sha256":"` + textDigest("null") + `","plugin_options":[{"name":"search","options_sha256":"` + emptyDigest + `"}],"plugins":["search"],"site_dir":"'"$stage"'/site","site_url":"","theme_custom_dir":"","theme_features":[],"theme_name":"material","theme_options_sha256":"` + textDigest(`{"name":"material"}`) + `","use_directory_urls":true,"validation":{},"validation_configured":true,"validation_sha256":"` + emptyDigest + `"}'
`
	writeExecutable(t, root, ".venv/bin/python", pythonScript)
	writeTestFile(t, root, ".venv/requirements.lock", "pinned\n")
	writeTestFile(t, root, "mkdocs.yml", "site_name: Test\ntheme:\n  name: material\n")
	writeTestFile(t, root, "docs/index.md", "# Index\n")
	mkdocs, _, err := pinExecutable(root, ".venv/bin/mkdocs", false)
	if err != nil {
		t.Fatal(err)
	}
	python, _, err := pinExecutable(root, ".venv/bin/python", true)
	if err != nil {
		t.Fatal(err)
	}
	_, projectLock, err := readPinned(root, ".venv/requirements.lock", defaultSourceBytes)
	if err != nil {
		t.Fatal(err)
	}
	_, config, err := readPinned(root, "mkdocs.yml", defaultConfigBytes)
	if err != nil {
		t.Fatal(err)
	}
	return Environment{
		ProjectRoot:   root,
		Toolchain:     Toolchain{MkDocs: mkdocs, Python: python, ProjectLock: FilePin{Path: projectLock.Path, SHA256: projectLock.SHA256, Size: projectLock.Size}, EnvironmentSHA256: strings.Repeat("e", 64)},
		Config:        FilePin{Path: config.Path, SHA256: config.SHA256, Size: config.Size},
		Observations:  ConfigObservations{Authority: "test MkDocs authority", DocsDir: "docs", ThemeName: "material", Plugins: []string{"search"}, SearchStatus: "implicit-effective-default"},
		Qualification: "QUALIFIED",
	}
}

func testBuildOptions(environment Environment) BuildOptions {
	return BuildOptions{TrustAttestation: EnvironmentTrustAttestation{
		Profile: "corvint-doccompiler-environment-trust/0", EnvironmentSHA256: environment.Toolchain.EnvironmentSHA256,
		Authority: "test owner", Revision: strings.Repeat("a", 40),
	}}
}

func successfulBuildScript() string { return "#!/bin/sh\n" + siteShellBody() }

func siteShellBody() string {
	return `site=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--site-dir" ]; then shift; site="$1"; fi
  shift
done
/bin/mkdir -p "$site"
printf '<html>strict</html>' > "$site/index.html"
`
}

func writeTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func writeExecutable(t *testing.T, root, relative, content string) {
	t.Helper()
	writeTestFile(t, root, relative, content)
	if err := os.Chmod(filepath.Join(root, filepath.FromSlash(relative)), 0700); err != nil {
		t.Fatal(err)
	}
}

func fileDigest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func textDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func testEnvironmentDigest(t *testing.T, root, inventory string, lock SourcePin, options DiscoverOptions) [32]byte {
	t.Helper()
	mkdocs, _, err := pinExecutable(root, options.MkDocsPath, false)
	if err != nil {
		t.Fatal(err)
	}
	python, _, err := pinExecutable(root, options.PythonPath, true)
	if err != nil {
		t.Fatal(err)
	}
	return environmentSHA256(inventory, lock, mkdocs, python, options)
}

func errorCode(err error) string {
	var problem *Error
	if errors.As(err, &problem) {
		return problem.Code
	}
	return ""
}
