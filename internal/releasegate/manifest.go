package releasegate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// loadPinnedPolicy deliberately reads policy outside the scanned tree.  A
// release cannot author the policy that decides whether it is exhaustive.
func loadPinnedPolicy(root, policyPath, pinnedSHA256 string) (Manifest, error) {
	if !filepath.IsAbs(policyPath) || !validSHA256(pinnedSHA256) {
		return Manifest{}, errors.New("release gate requires an externally pinned absolute policy and SHA-256")
	}
	rootResolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return Manifest{}, errors.New("release root cannot be resolved")
	}
	policyPath = filepath.Clean(policyPath)
	resolvedPolicy, err := filepath.EvalSymlinks(policyPath)
	if err != nil {
		return Manifest{}, errors.New("external policy path cannot be resolved")
	}
	if relative, err := filepath.Rel(rootResolved, resolvedPolicy); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return Manifest{}, errors.New("external policy must be outside the scanned release root")
	}
	data, err := readPinnedPolicy(policyPath, maxManifestBytes)
	if err != nil || sha256Hex(data) != pinnedSHA256 {
		return Manifest{}, errors.New("external policy does not match its pinned digest")
	}
	return decodeManifest(data)
}

func samePinnedPolicy(release, external Manifest) bool { return reflect.DeepEqual(release, external) }

// decodeManifest rejects duplicate object keys before decoding. The standard
// decoder silently accepts them, which is not canonical policy parsing.
func decodeManifest(data []byte) (Manifest, error) {
	if len(data) == 0 || len(data) > maxManifestBytes {
		return Manifest{}, errors.New("manifest size is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := rejectDuplicateJSON(decoder); err != nil {
		return Manifest{}, err
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var result Manifest
	if err := decoder.Decode(&result); err != nil {
		return Manifest{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Manifest{}, errors.New("manifest has trailing JSON")
	}
	return result, nil
}

const (
	maxJSONDepth  = 64
	maxJSONTokens = 100_000
)

func rejectDuplicateJSON(decoder *json.Decoder) error {
	tokens := 0
	var value func(depth int) error
	value = func(depth int) error {
		if depth > maxJSONDepth {
			return errors.New("JSON nesting exceeds bound")
		}
		tokens++
		if tokens > maxJSONTokens {
			return errors.New("JSON token count exceeds bound")
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch delimiter := token.(type) {
		case json.Delim:
			switch delimiter {
			case '{':
				seen := map[string]struct{}{}
				for decoder.More() {
					tokens++
					if tokens > maxJSONTokens {
						return errors.New("JSON token count exceeds bound")
					}
					key, err := decoder.Token()
					if err != nil {
						return err
					}
					name, ok := key.(string)
					if !ok {
						return errors.New("object key is invalid")
					}
					if _, duplicate := seen[name]; duplicate {
						return fmt.Errorf("duplicate JSON key %q", name)
					}
					seen[name] = struct{}{}
					if err := value(depth + 1); err != nil {
						return err
					}
				}
				end, err := decoder.Token()
				if err != nil || end != json.Delim('}') {
					return errors.New("object is malformed")
				}
				return nil
			case '[':
				for decoder.More() {
					if err := value(depth + 1); err != nil {
						return err
					}
				}
				end, err := decoder.Token()
				if err != nil || end != json.Delim(']') {
					return errors.New("array is malformed")
				}
				return nil
			default:
				return errors.New("unexpected JSON delimiter")
			}
		}
		return nil
	}
	if err := value(0); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("manifest has trailing JSON")
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("manifest has trailing JSON")
	}
	return nil
}

func validateManifest(m Manifest, entries map[string]treeEntry, manifestPath string) error {
	if len(m.ArtifactInventory) == 0 || len(m.ArtifactModes) != len(m.ArtifactInventory) || !validSHA256(m.ArtifactSHA256) || len(m.BuildPackages) == 0 || len(m.BuildTargets) == 0 || len(m.CorePackages) == 0 || m.CoreSizeCeiling <= 0 {
		return errors.New("canonical manifest is incomplete or has non-positive ceilings")
	}
	targets := map[string]struct{}{}
	supported := supportedReleaseTargets()
	for _, target := range m.BuildTargets {
		key := target.GOOS + "/" + target.GOARCH
		if target.GOOS == "" || target.GOARCH == "" || target.CGOEnabled || target.Compiler != "gc" || !sameStrings(target.ReleaseTags, frozenReleaseTags()) || !sortedUniqueOptional(target.CustomTags) {
			return errors.New("build targets must be explicit CGO_ENABLED=0 targets")
		}
		if _, duplicate := targets[key]; duplicate || !supported[key] {
			return errors.New("build targets must be exact once")
		}
		targets[key] = struct{}{}
	}
	if len(targets) != len(supported) {
		return errors.New("build targets must equal the pinned supported target set")
	}
	if !sortedUniquePaths(m.ArtifactInventory) || !sortedUniqueStrings(m.BuildPackages) || !sortedUniqueStrings(m.CorePackages) || !sortedUniqueOptional(m.AnalyzerPackages) || !sortedUniqueOptional(m.PluginPackages) {
		return errors.New("canonical manifest lists must be sorted and unique")
	}
	for _, name := range m.ArtifactInventory {
		entry, ok := entries[name]
		if !ok || (entry.mode != "100644" && entry.mode != "100755") {
			return fmt.Errorf("artifact inventory entry %q is missing or not regular", name)
		}
		if m.ArtifactModes[name] != entry.mode {
			return fmt.Errorf("artifact inventory mode %q is not externally policy-pinned", name)
		}
	}
	seenAllowance := map[string]struct{}{}
	for _, allowance := range m.Allowances {
		if !inertFixturePath(allowance.Path) || !safePath(allowance.Path) || !validSHA256(allowance.BlobSHA256) {
			return errors.New("allowance must name an inert fixture and exact SHA-256")
		}
		if _, duplicate := seenAllowance[allowance.Path]; duplicate {
			return errors.New("allowance paths must be exact once")
		}
		seenAllowance[allowance.Path] = struct{}{}
		entry, ok := entries[allowance.Path]
		if !ok || entry.mode != "100644" {
			return errors.New("allowance fixture is not a regular tree blob")
		}
		if containsString(m.ArtifactInventory, allowance.Path) {
			return errors.New("allowance fixture is shipped in artifact inventory")
		}
	}
	roles, err := packageRoles(m)
	if err != nil || len(roles) == 0 {
		return errors.New("production package roles must be exact and non-overlapping")
	}
	for packageName, ceiling := range m.PluginSizeCeilings {
		if packageName == "" || ceiling <= 0 {
			return errors.New("plugin size ceilings must be positive")
		}
	}
	if len(m.PluginPackages) != len(m.PluginSizeCeilings) {
		return errors.New("plugin package declarations and ceilings disagree")
	}
	for _, packageName := range m.PluginPackages {
		if _, ok := m.PluginSizeCeilings[packageName]; !ok {
			return errors.New("plugin package declarations and ceilings disagree")
		}
		artifacts, ok := m.PluginArtifacts[packageName]
		if !ok || !sortedUniqueOptional(artifacts) {
			return errors.New("plugin artifacts must be externally bound once per plugin package")
		}
		for _, artifact := range artifacts {
			if _, shipped := entries[artifact]; !shipped || !containsString(m.ArtifactInventory, artifact) {
				return errors.New("plugin artifact is not a shipped exact tree entry")
			}
		}
	}
	if len(m.PluginArtifacts) != len(m.PluginPackages) {
		return errors.New("plugin artifacts and package declarations disagree")
	}
	artifactOwner := map[string]string{}
	for packageName, artifacts := range m.PluginArtifacts {
		for _, artifact := range artifacts {
			if artifactOwner[artifact] != "" {
				return errors.New("shipped plugin artifact has ambiguous package ownership")
			}
			artifactOwner[artifact] = packageName
		}
	}
	if len(m.PluginPackages) > 0 {
		for _, artifact := range m.ArtifactInventory {
			if strings.HasSuffix(artifact, ".go") || artifact == "go.mod" {
				continue
			}
			if artifactOwner[artifact] == "" {
				return errors.New("opaque shipped artifact lacks an exhaustive plugin package binding")
			}
		}
	}
	if len(m.AnalyzerSizeCeilings) != len(m.AnalyzerPackages) {
		return errors.New("analyzer package declarations and ceilings disagree")
	}
	for _, packageName := range m.AnalyzerPackages {
		ceiling, ok := m.AnalyzerSizeCeilings[packageName]
		if !ok || ceiling <= 0 {
			return errors.New("analyzer size ceilings must be positive")
		}
	}
	declared := map[string]struct{}{manifestPath: {}}
	for _, name := range m.ArtifactInventory {
		declared[name] = struct{}{}
	}
	for _, allowance := range m.Allowances {
		declared[allowance.Path] = struct{}{}
	}
	if len(declared) != len(entries) {
		return errors.New("canonical manifest does not account for every tree entry")
	}
	for name := range entries {
		if _, ok := declared[name]; !ok {
			return fmt.Errorf("unexplained release tree entry %q", name)
		}
	}
	return nil
}

func supportedReleaseTargets() map[string]bool {
	return map[string]bool{"darwin/amd64": true, "darwin/arm64": true, "linux/amd64": true, "linux/arm64": true, "windows/amd64": true}
}

func frozenReleaseTags() []string {
	tags := make([]string, 0, 27)
	for minor := 1; minor <= 27; minor++ {
		tags = append(tags, fmt.Sprintf("go1.%d", minor))
	}
	return tags
}
func sameStrings(a, b []string) bool { return len(a) == len(b) && allStringPairsEqual(a, b) }
func allStringPairsEqual(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func artifactInventorySHA256(inventory []string, entries map[string]treeEntry, blobs map[string][]byte) (string, error) {
	var body bytes.Buffer
	for _, name := range inventory {
		entry := entries[name]
		data, ok := blobs[entry.oid]
		if !ok || int64(len(data)) != entry.size {
			return "", errors.New("inventory blob is unavailable")
		}
		fmt.Fprintf(&body, "%d:%s:%d:%s\n", len(name), name, entry.size, sha256Hex(data))
	}
	return sha256Hex(body.Bytes()), nil
}

func sortedUniquePaths(values []string) bool {
	for _, v := range values {
		if !safePath(v) {
			return false
		}
	}
	return sortedUniqueStrings(values)
}
func sortedUniqueStrings(values []string) bool {
	if len(values) == 0 {
		return false
	}
	return sort.SliceIsSorted(values, func(i, j int) bool { return values[i] < values[j] }) && allUnique(values)
}
func sortedUniqueOptional(values []string) bool {
	return len(values) == 0 || sortedUniqueStrings(values)
}
func allUnique(values []string) bool {
	for i := 1; i < len(values); i++ {
		if values[i-1] == values[i] {
			return false
		}
	}
	return true
}
func validSHA256(v string) bool {
	if len(v) != 64 {
		return false
	}
	for _, b := range []byte(v) {
		if !(b >= '0' && b <= '9' || b >= 'a' && b <= 'f') {
			return false
		}
	}
	return true
}
func inertFixturePath(v string) bool {
	return strings.HasPrefix(v, "testdata/") || strings.HasPrefix(v, "fixtures/") || strings.Contains(v, "/testdata/") || strings.Contains(v, "/fixtures/")
}
func containsString(v []string, want string) bool {
	index := sort.SearchStrings(v, want)
	return index < len(v) && v[index] == want
}
