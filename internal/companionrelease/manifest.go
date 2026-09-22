package companionrelease

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func validateManifestReferences(entries []ArchiveEntry, manifest BundleManifest) error {
	byPath := make(map[string]ArchiveEntry, len(entries))
	for _, entry := range entries {
		if _, exists := byPath[entry.Path]; exists {
			return fmt.Errorf("duplicate component path %s", entry.Path)
		}
		byPath[entry.Path] = entry
	}
	for _, component := range manifest.Components {
		binary, ok := byPath[component.BinaryPath]
		if !ok || sha256Hex(binary.Data) != component.BinarySHA256 || int64(len(binary.Data)) != component.BinarySizeBytes {
			return fmt.Errorf("component %s binary reference does not match archive", component.Name)
		}
		source, ok := byPath[component.SourceArchivePath]
		if !ok || sha256Hex(source.Data) != component.SourceArchiveSHA256 {
			return fmt.Errorf("component %s source reference does not match archive", component.Name)
		}
	}
	for _, artifact := range manifest.Artifacts {
		source, ok := byPath[artifact.SourceArchivePath]
		if !ok || sha256Hex(source.Data) != artifact.SourceArchiveSHA256 {
			return fmt.Errorf("artifact %s source reference does not match archive", artifact.Name)
		}
		if artifact.Kind == "vsix" {
			v := manifest.VSIXTools
			if v.NodeVersion != requiredNodeVersion || v.NPMVersion != requiredNPMVersion || v.TypeScriptVersion != requiredTypeScriptVersion || v.VSCEVersion != requiredVSCEVersion || len(v.NodeSHA256) != 64 || len(v.NPMSHA256) != 64 || len(v.TypeScriptSHA256) != 64 || len(v.VSCESHA256) != 64 || len(v.LockfileSHA256) != 64 {
				return fmt.Errorf("artifact %s lacks exact VSIX toolchain identity", artifact.Name)
			}
			entry, ok := byPath[artifact.Path]
			if !ok || sha256Hex(entry.Data) != artifact.SHA256 || int64(len(entry.Data)) != artifact.SizeBytes {
				return fmt.Errorf("artifact %s does not match archive", artifact.Name)
			}
			continue
		}
		if artifact.Kind != "host-package-tree" {
			return fmt.Errorf("artifact %s has unsupported kind %s", artifact.Name, artifact.Kind)
		}
		h := sha256.New()
		count, total := 0, 0
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Path, artifact.Path) {
				continue
			}
			count++
			total += len(entry.Data)
			fmt.Fprintf(h, "%s\x00%06o\x00%d\x00", entry.Path, entry.Mode, len(entry.Data))
			_, _ = h.Write(entry.Data)
		}
		if count == 0 || int64(total) != artifact.SizeBytes || hex.EncodeToString(h.Sum(nil)) != artifact.SHA256 {
			return fmt.Errorf("artifact %s tree reference does not match archive", artifact.Name)
		}
	}
	return nil
}

// ComponentManifest is one bundled binary's exact provenance: the commit and
// tree it was built from, the source archive that reproduces those bytes,
// and the binary's own digest.
type ComponentManifest struct {
	Name                string `json:"name"`
	Module              string `json:"module"`
	BinaryPath          string `json:"binaryPath"`
	BinarySHA256        string `json:"binarySha256"`
	BinarySizeBytes     int64  `json:"binarySizeBytes"`
	Commit              string `json:"commit"`
	Tree                string `json:"tree"`
	SourceArchivePath   string `json:"sourceArchivePath"`
	SourceArchiveSHA256 string `json:"sourceArchiveSha256"`
}

type ArtifactManifest struct {
	Name                string `json:"name"`
	Kind                string `json:"kind"`
	Path                string `json:"path"`
	SHA256              string `json:"sha256,omitempty"`
	SizeBytes           int64  `json:"sizeBytes,omitempty"`
	Commit              string `json:"commit"`
	Tree                string `json:"tree"`
	SourceArchivePath   string `json:"sourceArchivePath"`
	SourceArchiveSHA256 string `json:"sourceArchiveSha256"`
	Support             string `json:"support,omitempty"`
}

type VSIXToolchainManifest struct {
	NodeVersion       string `json:"nodeVersion"`
	NodeSHA256        string `json:"nodeSha256"`
	NPMVersion        string `json:"npmVersion"`
	NPMSHA256         string `json:"npmSha256"`
	TypeScriptVersion string `json:"typeScriptVersion"`
	TypeScriptSHA256  string `json:"typeScriptSha256"`
	VSCEVersion       string `json:"vsceVersion"`
	VSCESHA256        string `json:"vsceSha256"`
	LockfileSHA256    string `json:"lockfileSha256"`
}

// BundleManifest is the companion bundle's top-level, machine-readable
// record: the target it was built for and the exact toolchain and component
// identities that produced it.
type BundleManifest struct {
	Profile    json.RawMessage       `json:"profile,omitempty"`
	Target     string                `json:"target"`
	GoVersion  string                `json:"goVersion"`
	GitVersion string                `json:"gitVersion"`
	NotRun     []string              `json:"notRunTargets"`
	Components []ComponentManifest   `json:"components"`
	Artifacts  []ArtifactManifest    `json:"artifacts"`
	VSIXTools  VSIXToolchainManifest `json:"vsixToolchain"`
}

func (m BundleManifest) MarshalJSON() ([]byte, error) {
	type manifestFields BundleManifest
	inventory, err := inventoryForManifest(m)
	if err != nil || !inventory.core {
		return json.Marshal(manifestFields(m))
	}
	if m.VSIXTools != (VSIXToolchainManifest{}) {
		return nil, fmt.Errorf("core bundle forbids VSIX toolchain")
	}
	return json.Marshal(struct {
		Profile    json.RawMessage     `json:"profile"`
		Target     string              `json:"target"`
		GoVersion  string              `json:"goVersion"`
		GitVersion string              `json:"gitVersion"`
		NotRun     []string            `json:"notRunTargets"`
		Components []ComponentManifest `json:"components"`
		Artifacts  []ArtifactManifest  `json:"artifacts"`
	}{m.Profile, m.Target, m.GoVersion, m.GitVersion, m.NotRun, m.Components, m.Artifacts})
}

func renderManifestJSON(m BundleManifest) ([]byte, error) {
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// renderSHA256SUMS produces one sorted "<sha256>  <path>\n" line per entry,
// the format `shasum -a 256 -c` verifies directly.
func renderSHA256SUMS(entries []ArchiveEntry) []byte {
	sorted := append([]ArchiveEntry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	var b strings.Builder
	for _, e := range sorted {
		fmt.Fprintf(&b, "%s  %s\n", sha256Hex(e.Data), e.Path)
	}
	return []byte(b.String())
}

// noticeEntries looks up the named files in export's hash-verified tree
// (the same pinned blobs the source archive ships) and places them under
// prefix in the bundle. Every file must exist; a missing notice is a
// refusal, not an omission. This deliberately never touches the live
// checkout: a notice not present in the exported commit tree cannot ship,
// even if the working tree has one.
func noticeEntries(export Export, prefix string, files []string) ([]ArchiveEntry, error) {
	entries := make([]ArchiveEntry, 0, len(files))
	for _, name := range files {
		f, ok := findExportFile(export, name)
		if !ok {
			return nil, fmt.Errorf("required notice %s: not present in exported commit tree", name)
		}
		entries = append(entries, ArchiveEntry{Path: prefix + "/" + name, Mode: 0o644, Data: f.Data})
	}
	return entries, nil
}

// thirdPartyNoticeEntry documents the module's dependency set. Both Corvint
// and corvint-taskman are dependency-free (no go.sum, standard library only);
// this states that explicitly rather than shipping an empty file with no
// explanation.
func thirdPartyNoticeEntry(prefix, moduleName string) ArchiveEntry {
	text := fmt.Sprintf(
		"Third-party notices for %s\n\nThis component has no third-party Go module dependencies: it links only the\nGo standard library and source owned by this repository. There is no go.sum\nbecause there is nothing to pin.\n",
		moduleName)
	return ArchiveEntry{Path: prefix + "/THIRD-PARTY-NOTICES.txt", Mode: 0o644, Data: []byte(text)}
}
