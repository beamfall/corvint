package analyzerjs

import (
	"encoding/base64"
	"maps"
	"slices"
	"strings"
)

type lockedPackage struct {
	name             string
	version          string
	path             string
	dependencyGroups [4]map[string]string
	locked           bool
}

func parseNPMLock(evidence *evidenceContext, lock, manifest sourceFile, declared manifestInfo, witnesses map[string]workspaceManifest, facts *[]Fact, prospective, dependencyBudget *int) error {
	value, err := decodeObject(lock.bytes, dependencyBudget, true)
	if err != nil {
		if _, limited := err.(cError); limited {
			return err
		}
		return reject("MALFORMED_INPUT")
	}
	if err := requireOnlyKeys(value, "name", "version", "lockfileVersion", "requires", "packages"); err != nil {
		return err
	}
	name, ok := text(value["name"])
	if !ok || name != declared.name {
		return reject("CONFLICTING_VALUE")
	}
	version, ok := text(value["version"])
	if !ok || version != declared.version {
		return reject("CONFLICTING_VALUE")
	}
	if !jsonNumber(value["lockfileVersion"], "3") {
		return reject("UNSUPPORTED_SCHEMA")
	}
	if _, ok := value["requires"].(bool); !ok {
		return reject("MALFORMED_INPUT")
	}
	packages, ok := object(value["packages"])
	if !ok || len(packages) == 0 || len(packages) > MaxFacts {
		return reject("LIMIT_EXCEEDED")
	}
	entries := make(map[string]lockedPackage, len(packages))
	for _, instancePath := range sortedKeys(packages) {
		if !npmPathFor(instancePath, declared.workspaces) {
			return reject("INVALID_PATH")
		}
		entry, ok := object(packages[instancePath])
		if !ok {
			return reject("MALFORMED_INPUT")
		}
		if err := requireOnlyKeys(entry, "name", "version", "resolved", "integrity", "link", "dev", "optional", "peer", "inBundle", "dependencies", "devDependencies", "peerDependencies", "optionalDependencies"); err != nil {
			return err
		}
		if instancePath == "" {
			if err := validateManifestLockEntry(entry, declared); err != nil {
				return err
			}
			continue
		}
		_, workspace := declared.workspaces[instancePath]
		if workspace {
			witness, exists := witnesses[instancePath]
			if !exists {
				return reject("EXACT_BINDING_UNAVAILABLE")
			}
			if err := validateManifestLockEntry(entry, witness.info); err != nil {
				return err
			}
		}
		pkg, err := parseLockedPackage(instancePath, entry, workspace)
		if err != nil {
			return err
		}
		entries[instancePath] = pkg
	}
	if _, ok := packages[""]; !ok {
		return reject("MALFORMED_INPUT")
	}
	for workspace := range declared.workspaces {
		if _, ok := entries[workspace]; !ok {
			return reject("EXACT_BINDING_UNAVAILABLE")
		}
	}
	for _, instancePath := range sortedKeys(entries) {
		pkg := entries[instancePath]
		if !pkg.locked {
			continue
		}
		if !npmPathFor(pkg.path, declared.workspaces) {
			return reject("INVALID_PATH")
		}
		fact, err := makeFact(evidence, lock, &manifest, "js.package.locked", pkg.name, "locked-at", pkg.version, pkg.path)
		if err != nil {
			return err
		}
		if err := addFact(facts, fact, prospective); err != nil {
			return err
		}
	}
	if err := addDependencyGroupFacts(evidence, &lock, &manifest, "root", declared.dependencyGroups, declared.workspaces, entries, facts, prospective); err != nil {
		return err
	}
	for _, parentPath := range sortedKeys(entries) {
		parent := entries[parentPath]
		witness, ok := witnesses[parent.path]
		if !ok {
			continue
		}
		if err := addDependencyGroupFacts(evidence, &lock, witness.file, parent.path, parent.dependencyGroups, declared.workspaces, entries, facts, prospective); err != nil {
			return err
		}
	}
	// Validating every locked parent covers bindings with no manifest witness.
	return validateLockedDependencies(entries)
}

func validateLockedDependencies(entries map[string]lockedPackage) error {
	for _, parent := range sortedKeys(entries) {
		for _, dependencies := range entries[parent].dependencyGroups {
			for _, name := range sortedKeys(dependencies) {
				if _, err := resolveDependency(parent, name, dependencies[name], entries); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
func addDependencyGroupFacts(evidence *evidenceContext, lock, manifest *sourceFile, instance string, groups [4]map[string]string, workspaces map[string]struct{}, entries map[string]lockedPackage, facts *[]Fact, prospective *int) error {
	for _, dependencies := range groups {
		if err := addDependencyFacts(evidence, lock, manifest, instance, dependencies, workspaces, entries, facts, prospective); err != nil {
			return err
		}
	}
	return nil
}
func addDependencyFacts(evidence *evidenceContext, lock, manifest *sourceFile, instance string, dependencies map[string]string, workspaces map[string]struct{}, entries map[string]lockedPackage, facts *[]Fact, prospective *int) error {
	if instance != "root" && !npmPathFor(instance, workspaces) {
		return reject("INVALID_PATH")
	}
	for _, dependency := range sortedKeys(dependencies) {
		version := dependencies[dependency]
		child, err := resolveDependency(instance, dependency, version, entries)
		if err != nil {
			return err
		}
		if !npmPathFor(child.path, workspaces) {
			return reject("INVALID_PATH")
		}
		fact, err := makeFact(evidence, *lock, manifest, "js.dependency.locked", instance, "depends-on", child.path, instance)
		if err != nil {
			return err
		}
		if err := addFact(facts, fact, prospective); err != nil {
			return err
		}
	}
	return nil
}
func validateManifestLockEntry(entry map[string]any, declared manifestInfo) error {
	name, nameOK := text(entry["name"])
	version, versionOK := text(entry["version"])
	if !nameOK || !versionOK || name != declared.name || version != declared.version {
		return reject("CONFLICTING_VALUE")
	}
	if err := validateLockMetadata(entry, name); err != nil {
		return err
	}
	for groupIndex, group := range dependencyGroups {
		raw, exists := entry[group]
		expected := declared.dependencyGroups[groupIndex]
		if !exists && len(expected) == 0 {
			continue
		}
		if !exists {
			return reject("CONFLICTING_VALUE")
		}
		values, ok := object(raw)
		if !ok || len(values) > MaxFacts || len(values) != len(expected) {
			return reject("LIMIT_EXCEEDED")
		}
		for _, dependency := range sortedKeys(values) {
			locked, ok := values[dependency].(string)
			if !ok || !npmName(dependency) || !exactVersion(locked) {
				return reject("DYNAMIC_INPUT")
			}
			if required, exists := expected[dependency]; !exists || required != locked {
				return reject("CONFLICTING_VALUE")
			}
		}
	}
	return nil
}
func parseLockedPackage(instancePath string, entry map[string]any, workspace bool) (lockedPackage, error) {
	name, nameOK := text(entry["name"])
	if !nameOK && !workspace {
		name, nameOK = packageNameFromPath(instancePath), true
	}
	version, versionOK := text(entry["version"])
	if !nameOK || !npmName(name) || !versionOK || !exactVersion(version) {
		return lockedPackage{}, reject("MALFORMED_INPUT")
	}
	if err := validateLockMetadata(entry, name); err != nil {
		return lockedPackage{}, err
	}
	_, locked := entry["resolved"]
	if !workspace && !locked {
		return lockedPackage{}, reject("MALFORMED_INPUT")
	}
	if !workspace && packageNameFromPath(instancePath) != name {
		return lockedPackage{}, reject("CONFLICTING_VALUE")
	}
	groups, err := parseDependencyGroups(entry)
	return lockedPackage{name: name, version: version, path: instancePath, dependencyGroups: groups, locked: locked}, err
}
func parseDependencyGroups(value map[string]any) ([4]map[string]string, error) {
	var groups [4]map[string]string
	for groupIndex, group := range dependencyGroups {
		raw, exists := value[group]
		if !exists {
			continue
		}
		dependencies, ok := object(raw)
		if !ok || len(dependencies) > MaxFacts {
			return groups, reject("LIMIT_EXCEEDED")
		}
		groups[groupIndex] = make(map[string]string, len(dependencies))
		for _, dependency := range sortedKeys(dependencies) {
			declared, ok := dependencies[dependency].(string)
			if !ok || !npmName(dependency) || !exactVersion(declared) {
				return groups, reject("DYNAMIC_INPUT")
			}
			if dependencyInGroups(groups, dependency) {
				return groups, reject("DUPLICATE_VALUE")
			}
			groups[groupIndex][dependency] = declared
		}
	}
	return groups, nil
}
func validateLockMetadata(entry map[string]any, name string) error {
	resolved, resolvedOK := text(entry["resolved"])
	integrity, integrityOK := text(entry["integrity"])
	_, resolvedPresent := entry["resolved"]
	_, integrityPresent := entry["integrity"]
	version, versionOK := text(entry["version"])
	if resolvedPresent != integrityPresent || resolvedPresent && (!resolvedOK || !integrityOK || !versionOK || !npmResolved(resolved, name, version) || !integrityText(integrity)) {
		return reject("MALFORMED_INPUT")
	}
	for _, field := range []string{"link", "dev", "optional", "peer", "inBundle", "devOptional", "hasInstallScript"} {
		if raw, exists := entry[field]; exists {
			if _, ok := raw.(bool); !ok {
				return reject("MALFORMED_INPUT")
			}
		}
	}
	return nil
}
func resolveDependency(parent, name, version string, entries map[string]lockedPackage) (lockedPackage, error) {
	if parent == "root" {
		parent = ""
	}
	for ancestor := parent; ; ancestor = npmParent(ancestor) {
		candidate := "node_modules/" + name
		if ancestor != "" {
			candidate = ancestor + "/" + candidate
		}
		if pkg, ok := entries[candidate]; ok {
			if pkg.version != version {
				return lockedPackage{}, reject("EXACT_BINDING_UNAVAILABLE")
			}
			return pkg, nil
		}
		if ancestor == "" {
			break
		}
	}
	return lockedPackage{}, reject("EXACT_BINDING_UNAVAILABLE")
}
func npmParent(instancePath string) string {
	index := strings.LastIndex(instancePath, "/node_modules/")
	if index < 0 {
		return ""
	}
	return instancePath[:index]
}
func npmPathFor(value string, workspaces map[string]struct{}) bool {
	if value == "" {
		return true
	}
	workspacePrefix := ""
	for workspace := range workspaces {
		if value == workspace {
			return true
		}
		if strings.HasPrefix(value, workspace+"/") && len(workspace) > len(workspacePrefix) {
			workspacePrefix = workspace
		}
	}
	if workspacePrefix != "" {
		value = strings.TrimPrefix(value, workspacePrefix+"/")
	}
	segments := strings.Split(value, "/")
	for index := 0; index < len(segments); {
		if segments[index] != "node_modules" || index+1 >= len(segments) {
			return false
		}
		index++
		name := segments[index]
		if strings.HasPrefix(name, "@") {
			if index+1 >= len(segments) || !npmName(name+"/"+segments[index+1]) {
				return false
			}
			index += 2
			continue
		}
		if !npmName(name) {
			return false
		}
		index++
	}
	return true
}
func packageNameFromPath(value string) string {
	parts := strings.Split(value, "/")
	if len(parts) >= 3 && strings.HasPrefix(parts[len(parts)-2], "@") {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return parts[len(parts)-1]
}
func npmName(value string) bool {
	if strings.HasPrefix(value, "@") {
		parts := strings.Split(value[1:], "/")
		return len(parts) == 2 && npmSegment(parts[0]) && npmSegment(parts[1])
	}
	return !strings.Contains(value, "/") && npmSegment(value)
}
func npmSegment(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	first := value[0]
	if !(first >= 'a' && first <= 'z' || first >= '0' && first <= '9') {
		return false
	}
	for index := range value {
		character := value[index]
		if !(character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || strings.IndexByte("._~-", character) >= 0) {
			return false
		}
	}
	return true
}
func npmResolved(value, name, version string) bool {
	base := name
	if slash := strings.LastIndexByte(base, '/'); slash >= 0 {
		base = base[slash+1:]
	}
	return value == "https://registry.npmjs.org/"+name+"/-/"+base+"-"+version+".tgz"
}
func exactVersion(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for index := range part {
			if part[index] < '0' || part[index] > '9' {
				return false
			}
		}
		if len(part) > 1 && part[0] == '0' {
			return false
		}
	}
	return true
}
func integrityText(value string) bool {
	if !strings.HasPrefix(value, "sha512-") {
		return false
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(value[len("sha512-"):])
	if err != nil || len(decoded) != 64 {
		return false
	}
	return base64.StdEncoding.EncodeToString(decoded) == value[len("sha512-"):]
}
func sortedKeys[T any](values map[string]T) []string { return slices.Sorted(maps.Keys(values)) }
