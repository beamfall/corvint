package companionrelease

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

const maxRetainedSidecarBytes = 8 << 20

// VerifiedRetainedBundle is an immutable, fully decoded retained companion
// bundle. Its archive members stay private so callers cannot alter the bytes
// between verification and extraction.
type VerifiedRetainedBundle struct {
	Directory      string
	ArchivePath    string
	ChecksumPath   string
	SmokePath      string
	ArchiveSHA256  string
	ChecksumSHA256 string
	SmokeSHA256    string
	Manifest       BundleManifest
	SmokeSteps     []SmokeStep
	entries        []ArchiveEntry
}

// VerifyRetainedBundle admits the closed three-file retained bundle, verifies
// both sidecars and every manifest/checksum reference, and decodes the archive
// without writing any output.
func VerifyRetainedBundle(directory string) (*VerifiedRetainedBundle, error) {
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve retained bundle: %w", err)
	}
	info, err := os.Lstat(resolved)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("retained bundle is not a real directory")
	}
	dirEntries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, err
	}
	if len(dirEntries) != 3 {
		return nil, fmt.Errorf("retained bundle has %d files, expected 3", len(dirEntries))
	}
	var archiveName, checksumName, smokeName string
	for _, entry := range dirEntries {
		entryInfo, statErr := entry.Info()
		if statErr != nil || !entryInfo.Mode().IsRegular() {
			return nil, fmt.Errorf("retained member %s is not a regular file", entry.Name())
		}
		switch {
		case strings.HasSuffix(entry.Name(), ".tar.gz.sha256"):
			checksumName = entry.Name()
		case strings.HasSuffix(entry.Name(), ".tar.gz"):
			archiveName = entry.Name()
		case strings.HasSuffix(entry.Name(), ".smoke.json"):
			smokeName = entry.Name()
		default:
			return nil, fmt.Errorf("unexpected retained member %s", entry.Name())
		}
	}
	base := strings.TrimSuffix(archiveName, ".tar.gz")
	if base == "" || checksumName != archiveName+".sha256" || smokeName != base+".smoke.json" {
		return nil, fmt.Errorf("retained sidecar names do not match archive %q", archiveName)
	}
	archivePath := filepath.Join(resolved, archiveName)
	archive, err := readBoundedRegular(archivePath, maxRetainedArchiveBytes)
	if err != nil {
		return nil, fmt.Errorf("read retained archive: %w", err)
	}
	digest := sha256Hex(archive)
	checksum, err := readBoundedRegular(filepath.Join(resolved, checksumName), maxRetainedSidecarBytes)
	if err != nil {
		return nil, fmt.Errorf("read archive checksum: %w", err)
	}
	if string(checksum) != digest+"  "+archiveName+"\n" {
		return nil, fmt.Errorf("archive checksum sidecar mismatch")
	}
	entries, err := decodeTarGz(archive, maxRetainedEntries, maxRetainedArchiveBytes)
	if err != nil {
		return nil, fmt.Errorf("decode retained archive: %w", err)
	}
	byPath := make(map[string]ArchiveEntry, len(entries))
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}
	manifestEntry, ok := byPath["MANIFEST.json"]
	if !ok {
		return nil, fmt.Errorf("archive lacks MANIFEST.json")
	}
	if _, err := wire.Parse(manifestEntry.Data); err != nil {
		return nil, fmt.Errorf("manifest JSON: %w", err)
	}
	var manifest BundleManifest
	decoder := json.NewDecoder(bytes.NewReader(manifestEntry.Data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("manifest shape: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("manifest shape: %w", err)
	}
	inventory, err := inventoryForManifest(manifest)
	if err != nil {
		return nil, err
	}
	if inventory.core {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(manifestEntry.Data, &fields); err != nil {
			return nil, err
		}
		for _, name := range []string{"profile", "target", "goVersion", "gitVersion", "notRunTargets", "components", "artifacts"} {
			if _, ok := fields[name]; !ok {
				return nil, fmt.Errorf("core manifest lacks %s", name)
			}
		}
		if len(fields) != 7 {
			return nil, fmt.Errorf("core manifest requires exact seven fields")
		}
	}
	if err := validateClosedManifest(entries, manifest); err != nil {
		return nil, err
	}
	if err := validateClosedArchiveInventory(entries, manifest); err != nil {
		return nil, err
	}
	sums, ok := byPath["SHA256SUMS"]
	if !ok {
		return nil, fmt.Errorf("archive lacks SHA256SUMS")
	}
	withoutSums := make([]ArchiveEntry, 0, len(entries)-1)
	for _, entry := range entries {
		if entry.Path != "SHA256SUMS" {
			withoutSums = append(withoutSums, entry)
		}
	}
	if !bytes.Equal(sums.Data, renderSHA256SUMS(withoutSums)) {
		return nil, fmt.Errorf("SHA256SUMS does not cover the exact archive inventory")
	}
	smokePath := filepath.Join(resolved, smokeName)
	smokeData, err := readBoundedRegular(smokePath, maxRetainedSidecarBytes)
	if err != nil {
		return nil, fmt.Errorf("read smoke report: %w", err)
	}
	if _, err := wire.Parse(smokeData); err != nil {
		return nil, fmt.Errorf("smoke JSON: %w", err)
	}
	var smoke struct {
		BundleSHA256 string      `json:"bundleSha256"`
		Steps        []SmokeStep `json:"steps"`
	}
	decoder = json.NewDecoder(bytes.NewReader(smokeData))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&smoke); err != nil {
		return nil, fmt.Errorf("smoke shape: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("smoke shape: %w", err)
	}
	if smoke.BundleSHA256 != digest || len(smoke.Steps) == 0 {
		return nil, fmt.Errorf("smoke report does not bind archive")
	}
	for _, step := range smoke.Steps {
		if !step.OK || step.BundleSHA256 != digest {
			return nil, fmt.Errorf("smoke step %q is failed or unbound", step.Name)
		}
		if err := validateSmokeComponent(step, manifest); err != nil {
			return nil, err
		}
	}
	if err := validateSmokeInventory(smoke.Steps, inventory.core); err != nil {
		return nil, err
	}
	return &VerifiedRetainedBundle{Directory: resolved, ArchivePath: archivePath, ChecksumPath: filepath.Join(resolved, checksumName), SmokePath: smokePath, ArchiveSHA256: digest, ChecksumSHA256: sha256Hex(checksum), SmokeSHA256: sha256Hex(smokeData), Manifest: manifest, SmokeSteps: append([]SmokeStep(nil), smoke.Steps...), entries: entries}, nil
}

func validateSmokeInventory(steps []SmokeStep, core bool) error {
	want := map[string]bool{}
	for _, name := range []string{"atm-version", "atm-help", "corvint-mcp-version", "corvint-docs-mcp-version", "corvint-test-validity-mcp-version", "corvint-mcp-discover-list-status", "corvint-docs-mcp-draft-consume", "corvint-test-validity-mcp-list", "corvint-js-test-provider-help", "corvint-go-test-provider-help", "atm-init", "atm-ticket-create", "atm-ticket-refine", "corvint-dashboard-snapshot", "console-listen", "console-board", "console-detail", "console-create-form", "console-refuse-cross-origin", "console-refuse-missing-token", "console-refusal-no-store-effect", "console-detail-controls", "console-edit", "console-evidence", "console-requirement-links", "console-code-links", "console-link-gaps", "console-stop-no-descendants"} {
		want[name] = true
	}
	if core {
		for _, name := range []string{"corvint-version-identity", "corvint-affected-selection", "corvint-playwright-external-discovery", "corvint-documentation-corpus-discovery", "corvint-work-queue-observation"} {
			want[name] = true
		}
	}
	if len(steps) != len(want) {
		return fmt.Errorf("smoke step count %d, expected %d", len(steps), len(want))
	}
	seen := map[string]bool{}
	for _, step := range steps {
		if !want[step.Name] || seen[step.Name] {
			return fmt.Errorf("unexpected or duplicate smoke step %q", step.Name)
		}
		seen[step.Name] = true
		if step.ComponentSHA256 == "" || step.SourceCommit == "" || step.SourceTree == "" || step.InvokedPath == "" {
			return fmt.Errorf("smoke step %q lacks producer binding", step.Name)
		}
	}
	return nil
}

func validateClosedArchiveInventory(entries []ArchiveEntry, manifest BundleManifest) error {
	inventory, err := inventoryForManifest(manifest)
	if err != nil {
		return err
	}
	byPath := map[string]ArchiveEntry{}
	allowed := map[string]bool{"MANIFEST.json": true, "README.md": true, "SHA256SUMS": true}
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}
	for _, c := range manifest.Components {
		allowed[c.BinaryPath] = true
		allowed[c.SourceArchivePath] = true
	}
	for _, a := range manifest.Artifacts {
		if a.Kind == "vsix" {
			allowed[a.Path] = true
		} else {
			for _, entry := range entries {
				if strings.HasPrefix(entry.Path, a.Path) {
					allowed[entry.Path] = true
				}
			}
		}
	}
	for _, module := range []string{inventory.name("corvint"), inventory.name("corvint-taskman")} {
		sourcePath := "source/" + module + "-src.tar.gz"
		source, ok := byPath[sourcePath]
		if !ok {
			return fmt.Errorf("missing %s", sourcePath)
		}
		sourceEntries, err := decodeTarGz(source.Data, maxExportEntries, maxExportTotalBytes)
		if err != nil {
			return err
		}
		export := Export{}
		for _, entry := range sourceEntries {
			mode := "100644"
			if entry.Mode == 0o755 {
				mode = "100755"
			}
			export.Files = append(export.Files, SourceFile{Path: entry.Path, Mode: mode, Data: entry.Data})
		}
		notices, err := componentNotices(export, "notices/"+module, module)
		if err != nil {
			return err
		}
		for _, notice := range notices {
			allowed[notice.Path] = true
			if got, ok := byPath[notice.Path]; !ok || !bytes.Equal(got.Data, notice.Data) {
				return fmt.Errorf("notice %s mismatch", notice.Path)
			}
		}
		if module == inventory.name("corvint") && !inventory.core {
			npm, err := buildOnlyNPMNotice(export)
			if err != nil {
				return err
			}
			npm.Path = "notices/" + inventory.name("corvint") + "/VSIX-BUILD-ONLY-NPM.txt"
			allowed[npm.Path] = true
			if got, ok := byPath[npm.Path]; !ok || !bytes.Equal(got.Data, npm.Data) {
				return fmt.Errorf("VSIX npm notice mismatch")
			}
		}
	}
	for _, entry := range entries {
		if !allowed[entry.Path] {
			return fmt.Errorf("unreferenced archive member %s", entry.Path)
		}
	}
	return nil
}

func validateSmokeComponent(step SmokeStep, manifest BundleManifest) error {
	inventory, err := inventoryForManifest(manifest)
	if err != nil {
		return err
	}
	wantName := "corvint-console"
	switch {
	case strings.HasPrefix(step.Name, "atm-"):
		wantName = "atm"
	case step.Name == "corvint-dashboard-snapshot":
		wantName = "corvint-dashboard-snapshot"
	case step.Name == "corvint-mcp-discover-list-status" || step.Name == "corvint-mcp-version":
		wantName = "corvint-mcp"
	case step.Name == "corvint-docs-mcp-draft-consume" || step.Name == "corvint-docs-mcp-version":
		wantName = "corvint-docs-mcp"
	case step.Name == "corvint-test-validity-mcp-list" || step.Name == "corvint-test-validity-mcp-version":
		wantName = "corvint-test-validity-mcp"
	case step.Name == "corvint-js-test-provider-help":
		wantName = "corvint-js-test-provider"
	case step.Name == "corvint-go-test-provider-help":
		wantName = "corvint-go-test-provider"
	case step.Name == "corvint-version-identity" || step.Name == "corvint-affected-selection" ||
		step.Name == "corvint-playwright-external-discovery" || step.Name == "corvint-documentation-corpus-discovery" ||
		step.Name == "corvint-work-queue-observation":
		wantName = "corvint"
	}
	wantName = inventory.name(wantName)
	for _, component := range manifest.Components {
		if component.Name != wantName {
			continue
		}
		if !filepath.IsAbs(step.InvokedPath) || filepath.Clean(step.InvokedPath) != step.InvokedPath || !strings.HasSuffix(filepath.ToSlash(step.InvokedPath), "/"+component.BinaryPath) || step.ComponentSHA256 != component.BinarySHA256 || step.SourceCommit != component.Commit || step.SourceTree != component.Tree {
			return fmt.Errorf("smoke step %q component binding disagrees", step.Name)
		}
		return nil
	}
	return fmt.Errorf("smoke step %q names no manifest component", step.Name)
}

// Extract writes verified bytes to a fresh target. Existing targets are
// refused, including empty directories, so qualification never merges trees.
func (v *VerifiedRetainedBundle) Extract(target string) (err error) {
	if _, statErr := os.Lstat(target); statErr == nil {
		return fmt.Errorf("refusing to overwrite extraction target %s", target)
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	if err := os.Mkdir(target, 0o700); err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			err = errorsJoin(err, os.RemoveAll(target))
		}
	}()
	for _, entry := range v.entries {
		full := filepath.Join(target, filepath.FromSlash(entry.Path))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(full, entry.Data, os.FileMode(entry.Mode)); err != nil {
			return err
		}
	}
	complete = true
	return nil
}

// ExtractSourceArchive materializes one verified source archive member into a
// fresh directory using the same closed decoder and bounds as the outer bundle.
func (v *VerifiedRetainedBundle) ExtractSourceArchive(member, target string) error {
	data, ok := v.Entry(member)
	if !ok {
		return fmt.Errorf("verified bundle lacks source archive %s", member)
	}
	entries, err := decodeTarGz(data, maxExportEntries, maxExportTotalBytes)
	if err != nil {
		return fmt.Errorf("decode source archive %s: %w", member, err)
	}
	nested := &VerifiedRetainedBundle{entries: entries}
	return nested.Extract(target)
}

func validateClosedManifest(entries []ArchiveEntry, manifest BundleManifest) error {
	inventory, err := inventoryForManifest(manifest)
	if err != nil {
		return err
	}
	if manifest.Target != supportedTarget {
		return fmt.Errorf("manifest target %q is not %s", manifest.Target, supportedTarget)
	}
	wantComponents := map[string]string{"corvint": "corvint", "corvint-console": "corvint", "corvint-dashboard-snapshot": "corvint", "corvint-mcp": "corvint", "corvint-docs-mcp": "corvint", "corvint-test-validity-mcp": "corvint", "corvint-js-test-provider": "corvint", "corvint-go-test-provider": "corvint", "atm": "corvint-taskman"}
	currentComponents := make(map[string]string, len(wantComponents))
	for name, module := range wantComponents {
		currentComponents[inventory.name(name)] = inventory.name(module)
	}
	wantComponents = currentComponents
	if len(manifest.Components) != len(wantComponents) {
		return fmt.Errorf("manifest component count %d, expected %d", len(manifest.Components), len(wantComponents))
	}
	seen := map[string]bool{}
	corvintCommit, corvintTree, corvintSourceDigest := "", "", ""
	for _, c := range manifest.Components {
		module, ok := wantComponents[c.Name]
		if !ok || seen[c.Name] || c.Module != module || c.BinaryPath != "bin/"+c.Name {
			return fmt.Errorf("invalid component manifest row %q", c.Name)
		}
		wantSource := inventory.source("corvint")
		if module == inventory.name("corvint-taskman") {
			wantSource = inventory.source("corvint-taskman")
		}
		if c.SourceArchivePath != wantSource {
			return fmt.Errorf("component %s has wrong source archive", c.Name)
		}
		if module == inventory.name("corvint") {
			if corvintCommit == "" {
				corvintCommit, corvintTree, corvintSourceDigest = c.Commit, c.Tree, c.SourceArchiveSHA256
			}
			if c.Commit != corvintCommit || c.Tree != corvintTree || c.SourceArchiveSHA256 != corvintSourceDigest {
				return fmt.Errorf("Corvint component source identities disagree")
			}
		}
		seen[c.Name] = true
	}
	wantArtifacts := map[string]string{inventory.name("corvint-vscode"): "vsix", "codex": "host-package-tree", "claude-code": "host-package-tree", "gemini-cli": "host-package-tree", "opencode": "host-package-tree"}
	if inventory.core {
		delete(wantArtifacts, inventory.name("corvint-vscode"))
		if manifest.VSIXTools != (VSIXToolchainManifest{}) {
			return fmt.Errorf("core bundle forbids VSIX toolchain")
		}
	}
	if inventory.pi {
		wantArtifacts["pi"] = "host-package-tree"
	}
	if len(manifest.Artifacts) != len(wantArtifacts) {
		return fmt.Errorf("manifest artifact count %d, expected %d", len(manifest.Artifacts), len(wantArtifacts))
	}
	seen = map[string]bool{}
	for _, a := range manifest.Artifacts {
		kind, ok := wantArtifacts[a.Name]
		if !ok || seen[a.Name] || a.Kind != kind || a.Support != "FALLBACK" || a.SourceArchivePath != inventory.source("corvint") {
			return fmt.Errorf("invalid artifact manifest row %q", a.Name)
		}
		wantPath := "plugins/" + a.Name + "/"
		if a.Name == inventory.name("corvint-vscode") {
			wantPath = inventory.vsix()
		}
		if a.Path != wantPath || a.Commit != corvintCommit || a.Tree != corvintTree || a.SourceArchiveSHA256 != corvintSourceDigest {
			return fmt.Errorf("artifact %s source identity or path disagrees", a.Name)
		}
		seen[a.Name] = true
	}
	if err := validateManifestReferences(entries, manifest); err != nil {
		return fmt.Errorf("manifest references: %w", err)
	}
	return nil
}

func readBoundedRegular(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("%s is not a bounded regular file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%s grew beyond the byte bound", path)
	}
	return body, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

func errorsJoin(primary, cleanup error) error {
	if cleanup == nil {
		return primary
	}
	if primary == nil {
		return cleanup
	}
	return fmt.Errorf("%v; cleanup: %w", primary, cleanup)
}

// CorvintSourceIdentity returns the single Corvint commit/tree/source archive
// identity shared by every Corvint-built component and artifact.
func (v *VerifiedRetainedBundle) CorvintSourceIdentity() (commit, tree, archivePath string, err error) {
	inventory, err := inventoryForManifest(v.Manifest)
	if err != nil {
		return "", "", "", err
	}
	for _, c := range v.Manifest.Components {
		if c.Module != inventory.name("corvint") {
			continue
		}
		if commit == "" {
			commit, tree, archivePath = c.Commit, c.Tree, c.SourceArchivePath
			continue
		}
		if c.Commit != commit || c.Tree != tree || c.SourceArchivePath != archivePath {
			return "", "", "", fmt.Errorf("Corvint source identities disagree")
		}
	}
	if commit == "" {
		return "", "", "", fmt.Errorf("manifest has no Corvint component")
	}
	return commit, tree, archivePath, nil
}

// Entry returns a copy of one verified archive member.
func (v *VerifiedRetainedBundle) Entry(path string) ([]byte, bool) {
	i := sort.Search(len(v.entries), func(i int) bool { return v.entries[i].Path >= path })
	if i >= len(v.entries) || v.entries[i].Path != path { // archives are deterministic sorted output
		for _, entry := range v.entries {
			if entry.Path == path {
				return append([]byte(nil), entry.Data...), true
			}
		}
		return nil, false
	}
	return append([]byte(nil), v.entries[i].Data...), true
}
