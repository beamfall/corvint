package doccompiler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var mkdocsVersionPattern = regexp.MustCompile(`(?i)mkdocs(?:,)?\s+version\s+([^\s]+)`)

const inventoryScript = `import importlib.metadata as m,json,platform,sys
p=sorted([[d.metadata.get("Name","") or "",d.version] for d in m.distributions()])
print(json.dumps({"implementation":platform.python_implementation(),"packages":p,"python":sys.version},sort_keys=True,separators=(",",":")))`

const configAuthorityScript = `import hashlib,json,os,sys
from mkdocs.config import load_config
c=load_config(config_file=sys.argv[1])
t=c.get("theme")
def values(v):
 return [] if v is None else [str(x) for x in v]
def plain(v):
 if v is None or isinstance(v,(bool,int,float,str)): return v
 if isinstance(v,dict): return {str(k):plain(v[k]) for k in sorted(v,key=lambda x:str(x))}
 if isinstance(v,(list,tuple)): return [plain(x) for x in v]
 return str(v)
def digest(v):
 return hashlib.sha256(json.dumps(plain(v),sort_keys=True,separators=(",",":")).encode()).hexdigest()
plugins=[]
for name,plugin in (c.get("plugins") or {}).items():
 plugins.append({"name":str(name),"options_sha256":digest(getattr(plugin,"config",{}))})
extensions=[]
extension_options=c.get("mdx_configs") or {}
for name in (c.get("markdown_extensions") or []):
 extensions.append({"name":str(name),"options_sha256":digest(extension_options.get(str(name),{}))})
nav=plain(c.get("nav"))
validation=plain(c.get("validation") or {})
o={"config_file_path":str(getattr(c,"config_file_path","") or ""),"docs_dir":str(c.get("docs_dir") or ""),"extra_css":values(c.get("extra_css")),"extra_javascript":values(c.get("extra_javascript")),"hooks":values(c.get("hooks")),"markdown_extension_options":extensions,"markdown_extensions":[x["name"] for x in extensions],"nav_configured":c.get("nav") is not None,"nav_sha256":digest(nav),"plugin_options":plugins,"plugins":[x["name"] for x in plugins],"site_dir":str(c.get("site_dir") or ""),"site_url":str(c.get("site_url") or ""),"theme_custom_dir":str(t.get("custom_dir") or "") if t else "","theme_features":values(t.get("features")) if t else [],"theme_name":str(t.get("name") or "") if t else "","theme_options_sha256":digest(t or {}),"use_directory_urls":bool(c.get("use_directory_urls")),"validation":validation,"validation_configured":c.get("validation") is not None,"validation_sha256":digest(validation)}
print(json.dumps(o,sort_keys=True,separators=(",",":")))`

type pythonInventory struct {
	Implementation string     `json:"implementation"`
	Packages       [][]string `json:"packages"`
	Python         string     `json:"python"`
}

type authoritativeConfig struct {
	ConfigFilePath           string         `json:"config_file_path"`
	DocsDir                  string         `json:"docs_dir"`
	SiteDir                  string         `json:"site_dir"`
	ExtraCSS                 []string       `json:"extra_css"`
	ExtraJavaScript          []string       `json:"extra_javascript"`
	Hooks                    []string       `json:"hooks"`
	MarkdownExtensions       []string       `json:"markdown_extensions"`
	MarkdownExtensionOptions []OptionDigest `json:"markdown_extension_options"`
	NavConfigured            bool           `json:"nav_configured"`
	NavSHA256                string         `json:"nav_sha256"`
	Plugins                  []string       `json:"plugins"`
	PluginOptions            []OptionDigest `json:"plugin_options"`
	SiteURL                  string         `json:"site_url"`
	UseDirectoryURLs         bool           `json:"use_directory_urls"`
	ThemeCustomDir           string         `json:"theme_custom_dir"`
	ThemeFeatures            []string       `json:"theme_features"`
	ThemeName                string         `json:"theme_name"`
	ThemeOptionsSHA256       string         `json:"theme_options_sha256"`
	ValidationConfigured     bool           `json:"validation_configured"`
	ValidationSHA256         string         `json:"validation_sha256"`
	Validation               map[string]any `json:"validation"`
}

func discover(ctx context.Context, options DiscoverOptions) (Environment, error) {
	root, err := projectRoot(options.ProjectRoot)
	if err != nil {
		return Environment{}, err
	}
	if options.MkDocsPath == "" {
		return Environment{}, failure("mkdocs-path-required", "an exact project-owned mkdocs path is required")
	}
	if options.ProjectLockPath == "" || !digestPattern.MatchString(options.ExpectedProjectLockSHA256) {
		return Environment{}, failure("project-lock-pin-required", "an exact project-owned lock path and SHA-256 are required")
	}
	if options.ExpectedMkDocsVersion == "" || options.ExpectedMaterialVersion == "" || options.ExpectedMarkdownVersion == "" || options.ExpectedPyMdownVersion == "" {
		return Environment{}, failure("version-pins-required", "exact MkDocs, Material, Python-Markdown, and PyMdown Extensions versions are required")
	}
	lockRelative, err := relativeFromRoot(root, options.ProjectLockPath)
	if err != nil {
		return Environment{}, err
	}
	_, lockSource, err := readPinned(root, lockRelative, defaultSourceBytes)
	if err != nil {
		return Environment{}, err
	}
	if lockSource.SHA256 != options.ExpectedProjectLockSHA256 {
		return Environment{}, failure("project-lock-pin-mismatch", "project-owned lock does not match its expected SHA-256")
	}
	mkdocsPin, mkdocsAbsolute, err := pinExecutable(root, options.MkDocsPath, false)
	if err != nil {
		return Environment{}, err
	}
	pythonPath := options.PythonPath
	if pythonPath == "" {
		pythonPath = filepath.Join(filepath.Dir(mkdocsAbsolute), "python")
	}
	pythonPin, pythonAbsolute, err := pinExecutable(root, pythonPath, true)
	if err != nil {
		return Environment{}, err
	}
	configRelative, err := discoverConfig(root, options.ConfigPath)
	if err != nil {
		return Environment{}, err
	}
	configLimit := options.MaxConfigBytes
	if configLimit <= 0 {
		configLimit = defaultConfigBytes
	}
	configRaw, configSource, err := readPinned(root, configRelative, configLimit)
	if err != nil {
		return Environment{}, err
	}
	commandLimit := options.MaxCommandBytes
	if commandLimit <= 0 {
		commandLimit = defaultCommandBytes
	}
	stateRoot, err := os.MkdirTemp("", "corvint-doccompiler-discovery-")
	if err != nil {
		return Environment{}, failure("isolation-failed", "cannot create discovery state root")
	}
	defer os.RemoveAll(stateRoot)
	if err := createStateDirectories(stateRoot); err != nil {
		return Environment{}, err
	}
	mkdocsResult, err := runCommand(ctx, mkdocsAbsolute, []string{"--version"}, root, isolatedEnvironment(mkdocsAbsolute, stateRoot), commandLimit, commandLimit)
	if err != nil {
		return Environment{}, err
	}
	match := mkdocsVersionPattern.FindStringSubmatch(mkdocsResult.stdout + "\n" + mkdocsResult.stderr)
	if len(match) != 2 {
		return Environment{}, failure("mkdocs-version-unknown", "pinned mkdocs did not report an exact version")
	}
	mkdocsVersion := strings.TrimSpace(match[1])
	inventoryResult, err := runCommand(ctx, pythonAbsolute, []string{"-I", "-c", inventoryScript}, root, isolatedEnvironment(pythonAbsolute, stateRoot), commandLimit, commandLimit)
	if err != nil {
		return Environment{}, err
	}
	var inventory pythonInventory
	decoder := json.NewDecoder(strings.NewReader(inventoryResult.stdout))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&inventory); err != nil || inventory.Python == "" || inventory.Implementation == "" {
		return Environment{}, failure("environment-inventory-invalid", "pinned Python returned an invalid environment inventory")
	}
	materialVersion := inventoryVersion(inventory.Packages, "mkdocs-material")
	inventoryMkDocsVersion := inventoryVersion(inventory.Packages, "mkdocs")
	markdownVersion := inventoryVersion(inventory.Packages, "markdown")
	pymdownVersion := inventoryVersion(inventory.Packages, "pymdown-extensions")
	if materialVersion == "" {
		return Environment{}, failure("material-version-unknown", "mkdocs-material is absent from the pinned environment inventory")
	}
	if inventoryMkDocsVersion == "" || inventoryMkDocsVersion != mkdocsVersion {
		return Environment{}, failure("mkdocs-version-mismatch", "mkdocs executable and environment inventory disagree")
	}
	if options.ExpectedMkDocsVersion != mkdocsVersion {
		return Environment{}, failure("mkdocs-pin-mismatch", "expected MkDocs %s, found %s", options.ExpectedMkDocsVersion, mkdocsVersion)
	}
	if options.ExpectedMaterialVersion != materialVersion {
		return Environment{}, failure("material-pin-mismatch", "expected Material %s, found %s", options.ExpectedMaterialVersion, materialVersion)
	}
	if options.ExpectedMarkdownVersion != markdownVersion {
		return Environment{}, failure("markdown-pin-mismatch", "expected Python-Markdown %s, found %s", options.ExpectedMarkdownVersion, markdownVersion)
	}
	if options.ExpectedPyMdownVersion != pymdownVersion {
		return Environment{}, failure("pymdown-pin-mismatch", "expected PyMdown Extensions %s, found %s", options.ExpectedPyMdownVersion, pymdownVersion)
	}
	environmentDigest := environmentSHA256(inventoryResult.stdout, lockSource, mkdocsPin, pythonPin, options)
	observation := observeConfig(configRaw)
	preflight, reasons := lexicalMaterialPreflight(observation)
	observation.Uncertainty = reasons
	qualification := "UNQUALIFIED"
	if preflight == "PRECHECK_PASS" {
		provisional := Environment{ProjectRoot: root, Toolchain: Toolchain{EnvironmentSHA256: hex.EncodeToString(environmentDigest[:])}}
		if err := validateTrustAttestation(provisional, options.TrustAttestation); err == nil {
			authoritative, authorityErr := loadAuthoritativeConfig(ctx, pythonAbsolute, root, stateRoot, filepath.Join(root, filepath.FromSlash(configRelative)), commandLimit)
			if authorityErr != nil {
				return Environment{}, authorityErr
			}
			observation, qualification, err = applyAuthoritativeConfig(root, configRelative, observation, authoritative)
			if err != nil {
				return Environment{}, err
			}
		} else {
			observation.Uncertainty = uniqueSorted(append(observation.Uncertainty, "MkDocs-authority config discovery was not run without a matching environment trust attestation"))
		}
	}
	return Environment{
		ProjectRoot: root,
		Toolchain: Toolchain{
			MkDocs:            mkdocsPin,
			Python:            pythonPin,
			ProjectLock:       FilePin{Path: lockSource.Path, SHA256: lockSource.SHA256, Size: lockSource.Size},
			MkDocsVersion:     mkdocsVersion,
			MaterialVersion:   materialVersion,
			MarkdownVersion:   markdownVersion,
			PyMdownVersion:    pymdownVersion,
			PythonVersion:     inventory.Implementation + " " + strings.ReplaceAll(inventory.Python, "\n", " "),
			EnvironmentSHA256: hex.EncodeToString(environmentDigest[:]),
			DistributionCount: len(inventory.Packages),
			Distributions:     inventoryDistributions(inventory.Packages),
		},
		Config:        FilePin{Path: configSource.Path, SHA256: configSource.SHA256, Size: configSource.Size},
		Observations:  observation,
		Qualification: qualification,
	}, nil
}

func loadAuthoritativeConfig(ctx context.Context, python, root, stateRoot, config string, limit int) (authoritativeConfig, error) {
	result, err := runCommand(ctx, python, []string{"-I", "-c", configAuthorityScript, config}, root, isolatedEnvironment(python, stateRoot), limit, limit)
	if err != nil {
		return authoritativeConfig{}, err
	}
	return decodeAuthoritativeConfig(result.stdout)
}

func decodeAuthoritativeConfig(output string) (authoritativeConfig, error) {
	var configResult authoritativeConfig
	decoder := json.NewDecoder(strings.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&configResult); err != nil {
		return authoritativeConfig{}, failure("mkdocs-config-authority-invalid", "MkDocs returned an invalid bounded config observation")
	}
	return configResult, nil
}

func applyAuthoritativeConfig(root, configRelative string, lexical ConfigObservations, authority authoritativeConfig) (ConfigObservations, string, error) {
	configAbsolute := filepath.Join(root, filepath.FromSlash(configRelative))
	if filepath.Clean(authority.ConfigFilePath) != filepath.Clean(configAbsolute) {
		return lexical, "UNQUALIFIED", failure("mkdocs-config-boundary-mismatch", "MkDocs loaded a different config file")
	}
	configDirectory := filepath.Dir(configAbsolute)
	docsRelative, err := relativeFromRoot(configDirectory, authority.DocsDir)
	if err != nil {
		return lexical, "UNQUALIFIED", failure("mkdocs-docs-dir-escape", "MkDocs resolved docs_dir outside the config directory")
	}
	siteRelative, err := relativeFromRoot(root, authority.SiteDir)
	if err != nil {
		return lexical, "UNQUALIFIED", failure("mkdocs-site-dir-escape", "MkDocs resolved site_dir outside the project")
	}
	if !digestPattern.MatchString(authority.NavSHA256) || !digestPattern.MatchString(authority.ValidationSHA256) || !digestPattern.MatchString(authority.ThemeOptionsSHA256) {
		return lexical, "UNQUALIFIED", failure("mkdocs-config-authority-invalid", "MkDocs omitted an effective config digest")
	}
	if err := validateOptionDigests(authority.PluginOptions); err != nil {
		return lexical, "UNQUALIFIED", err
	}
	if err := validateOptionDigests(authority.MarkdownExtensionOptions); err != nil {
		return lexical, "UNQUALIFIED", err
	}
	customRelative := ""
	if authority.ThemeCustomDir != "" {
		customRelative, err = relativeFromRoot(configDirectory, authority.ThemeCustomDir)
		if err != nil {
			return lexical, "UNQUALIFIED", failure("mkdocs-theme-dir-escape", "MkDocs resolved theme.custom_dir outside the config directory")
		}
	}
	lexical.Authority = "pinned Python mkdocs.config.load_config plus mkdocs build --strict"
	lexical.ParserStatus = "MkDocs-authoritative effective config; lexical scan retained only for hostile preflight and privacy signals"
	lexical.DocsDir = docsRelative
	lexical.SiteDir = siteRelative
	lexical.NavConfigured = authority.NavConfigured
	lexical.NavMode = "mkdocs-discovery"
	if authority.NavConfigured {
		lexical.NavMode = "explicit"
	}
	lexical.NavOwner = configRelative
	lexical.NavSHA256 = authority.NavSHA256
	lexical.SiteURL = authority.SiteURL
	lexical.UseDirectoryURLs = authority.UseDirectoryURLs
	lexical.ThemeName = authority.ThemeName
	lexical.ThemeOptionsSHA256 = authority.ThemeOptionsSHA256
	lexical.ThemeFeatures = uniqueSorted(authority.ThemeFeatures)
	lexical.ThemeCustomDir = customRelative
	lexical.Plugins = append([]string(nil), authority.Plugins...)
	lexical.PluginOptions = append([]OptionDigest(nil), authority.PluginOptions...)
	lexical.MarkdownExtensions = append([]string(nil), authority.MarkdownExtensions...)
	lexical.MarkdownExtensionOptions = append([]OptionDigest(nil), authority.MarkdownExtensionOptions...)
	lexical.ExtraCSS = uniqueSorted(authority.ExtraCSS)
	lexical.ExtraJavaScript = uniqueSorted(authority.ExtraJavaScript)
	lexical.Hooks = uniqueSorted(authority.Hooks)
	lexical.ValidationConfigured = authority.ValidationConfigured
	lexical.ValidationSHA256 = authority.ValidationSHA256
	lexical.Validation = authority.Validation
	if lexical.PluginsConfigured {
		if contains(lexical.Plugins, "search") {
			lexical.SearchStatus = "explicit-effective"
		} else {
			lexical.SearchStatus = "missing-after-plugins-configured"
		}
	} else if contains(lexical.Plugins, "search") {
		lexical.SearchStatus = "implicit-effective-default"
	} else {
		lexical.SearchStatus = "missing-effective"
	}
	reasons := make([]string, 0)
	if lexical.ThemeName != "material" {
		reasons = append(reasons, "MkDocs effective theme.name is not material")
	}
	if len(lexical.Hooks) > 0 {
		reasons = append(reasons, "MkDocs effective hooks are outside P0")
	}
	for _, plugin := range lexical.Plugins {
		if plugin != "search" {
			reasons = append(reasons, "MkDocs effective plugin is outside P0: "+plugin)
		}
	}
	if !contains(lexical.Plugins, "search") {
		reasons = append(reasons, "MkDocs effective config does not include search")
	}
	lexical.Uncertainty = uniqueSorted(append(lexical.Uncertainty, reasons...))
	if len(reasons) > 0 {
		return lexical, "UNQUALIFIED", nil
	}
	return lexical, "QUALIFIED", nil
}

func discoverConfig(root, requested string) (string, error) {
	if requested != "" {
		return relativeFromRoot(root, requested)
	}
	candidates := []string{"mkdocs.yml", "mkdocs.yaml"}
	found := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		info, err := os.Lstat(filepath.Join(root, candidate))
		if err == nil && info.Mode().IsRegular() {
			found = append(found, candidate)
		}
	}
	if len(found) == 0 {
		return "", failure("mkdocs-config-missing", "neither mkdocs.yml nor mkdocs.yaml exists at the project root")
	}
	if len(found) > 1 {
		return "", failure("mkdocs-config-ambiguous", "both mkdocs.yml and mkdocs.yaml exist; choose one explicitly")
	}
	return found[0], nil
}

func inventoryVersion(packages [][]string, target string) string {
	target = normalizeDistribution(target)
	versions := make([]string, 0, 1)
	for _, item := range packages {
		if len(item) != 2 {
			continue
		}
		if normalizeDistribution(item[0]) == target {
			versions = append(versions, item[1])
		}
	}
	versions = uniqueSorted(versions)
	if len(versions) != 1 {
		return ""
	}
	return versions[0]
}

func normalizeDistribution(value string) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, ".", "-")
	return value
}

func inventoryDistributions(packages [][]string) []Distribution {
	output := make([]Distribution, 0, len(packages))
	for _, item := range packages {
		if len(item) != 2 || strings.TrimSpace(item[0]) == "" || strings.TrimSpace(item[1]) == "" {
			continue
		}
		output = append(output, Distribution{Name: item[0], Version: item[1]})
	}
	sort.Slice(output, func(left, right int) bool {
		if normalizeDistribution(output[left].Name) == normalizeDistribution(output[right].Name) {
			return output[left].Version < output[right].Version
		}
		return normalizeDistribution(output[left].Name) < normalizeDistribution(output[right].Name)
	})
	return output
}

func hasPyMdown(extensions []string) bool {
	for _, extension := range extensions {
		if strings.HasPrefix(extension, "pymdownx.") {
			return true
		}
	}
	return false
}

func createStateDirectories(root string) error {
	for _, relative := range []string{"cache", "config", "data", "tmp"} {
		if err := os.MkdirAll(filepath.Join(root, relative), 0700); err != nil {
			return failure("isolation-failed", "cannot create isolated command state")
		}
	}
	return nil
}

func sortedInventory(packages [][]string) [][]string {
	output := append([][]string(nil), packages...)
	sort.Slice(output, func(left, right int) bool {
		return strings.Join(output[left], "\x00") < strings.Join(output[right], "\x00")
	})
	return output
}

func environmentSHA256(inventory string, lock SourcePin, mkdocs, python FilePin, options DiscoverOptions) [32]byte {
	bound := strings.Join([]string{
		inventory,
		lock.Path,
		lock.SHA256,
		mkdocs.Path,
		mkdocs.ResolvedPath,
		mkdocs.SHA256,
		python.Path,
		python.ResolvedPath,
		python.SHA256,
		options.ExpectedMkDocsVersion,
		options.ExpectedMaterialVersion,
		options.ExpectedMarkdownVersion,
		options.ExpectedPyMdownVersion,
	}, "\x00")
	return sha256.Sum256([]byte(bound))
}

func validateOptionDigests(options []OptionDigest) error {
	seen := map[string]struct{}{}
	for _, option := range options {
		if strings.TrimSpace(option.Name) == "" || !digestPattern.MatchString(option.OptionsSHA256) {
			return failure("mkdocs-config-authority-invalid", "MkDocs returned an invalid option digest")
		}
		if _, exists := seen[option.Name]; exists {
			return failure("mkdocs-config-authority-invalid", "MkDocs returned duplicate option ownership")
		}
		seen[option.Name] = struct{}{}
	}
	return nil
}
