package analyzerjs

import (
	"path"
	"strings"
)

var allowedPackage = []string{"name", "version", "workspaces", "dependencies", "devDependencies", "peerDependencies", "optionalDependencies"}
var dependencyGroups = [...]string{"dependencies", "devDependencies", "peerDependencies", "optionalDependencies"}

type manifestInfo struct {
	name             string
	version          string
	dependencyGroups [4]map[string]string
	workspaces       map[string]struct{}
}
type workspaceManifest struct {
	info manifestInfo
	file *sourceFile
}

func analyze(request Request, files []sourceFile) (Candidate, error) {
	evidence, err := newEvidenceContext(request)
	if err != nil {
		return Candidate{}, err
	}
	var lock *sourceFile
	var firstPackage *sourceFile
	var extraPackages []*sourceFile
	dependencyBudget := 0
	facts := make([]Fact, 0, min(len(files)*8, 64))
	prospective := candidateBaseSize(request, files)
	for index := range files {
		file := &files[index]
		switch file.Family {
		case "js.package":
			if firstPackage == nil {
				firstPackage = file
			} else {
				extraPackages = append(extraPackages, file)
			}
		case "js.npm-lock-v3":
			if lock != nil {
				return Candidate{}, reject("DUPLICATE_VALUE")
			}
			lock = file
		}
	}
	if firstPackage != nil {
		manifest, err := rootManifest(firstPackage, extraPackages, lock)
		if err != nil {
			return Candidate{}, err
		}
		info, err := parsePackage(evidence, *manifest, &facts, &prospective, &dependencyBudget, true)
		if err != nil {
			return Candidate{}, err
		}
		witnesses, err := workspaceWitnesses(firstPackage, extraPackages, manifest, info, evidence, &facts, &prospective, &dependencyBudget)
		if err != nil {
			return Candidate{}, err
		}
		if lock == nil {
			if dependencyCount(info.dependencyGroups) != 0 || len(witnesses) != 0 {
				return Candidate{}, reject("EXACT_BINDING_UNAVAILABLE")
			}
		} else {
			err = parseNPMLock(evidence, *lock, *manifest, info, witnesses, &facts, &prospective, &dependencyBudget)
			if err != nil {
				return Candidate{}, err
			}
		}
	} else if lock != nil {
		return Candidate{}, reject("EXACT_BINDING_UNAVAILABLE")
	}
	for _, file := range files {
		if file.Family != "js.source" {
			continue
		}
		if err := addSourceFacts(evidence, file, &facts, &prospective); err != nil {
			return Candidate{}, err
		}
	}
	if err := sortFacts(facts); err != nil {
		return Candidate{}, err
	}
	candidate := Candidate{Profile: Profile, Family: Family, RequestID: request.RequestID, Status: "CANDIDATE", ScopeID: request.ScopeID, CompilationUnitID: request.CompilationUnitID, Target: request.Target, InputEchoes: inputEchoes(request.Inputs), Facts: facts, prospective: prospective}
	candidate.seal = seal(candidate)
	candidate.sealed = true
	return candidate, nil
}
func rootManifest(first *sourceFile, extras []*sourceFile, lock *sourceFile) (*sourceFile, error) {
	if lock == nil {
		if len(extras) != 0 {
			return nil, reject("EXACT_BINDING_UNAVAILABLE")
		}
		return first, nil
	}
	directory := path.Dir(lock.Path)
	var manifest *sourceFile
	if path.Dir(first.Path) == directory {
		manifest = first
	}
	for _, candidate := range extras {
		if path.Dir(candidate.Path) != directory {
			continue
		}
		if manifest != nil {
			return nil, reject("DUPLICATE_VALUE")
		}
		manifest = candidate
	}
	if manifest == nil {
		return nil, reject("EXACT_BINDING_UNAVAILABLE")
	}
	return manifest, nil
}
func workspaceWitnesses(first *sourceFile, extras []*sourceFile, root *sourceFile, declared manifestInfo, evidence *evidenceContext, facts *[]Fact, prospective, dependencyBudget *int) (map[string]workspaceManifest, error) {
	if first == root && len(extras) == 0 {
		return nil, nil
	}
	witnesses := make(map[string]workspaceManifest, len(declared.workspaces))
	directory := path.Dir(root.Path)
	check := func(candidate *sourceFile) error {
		if candidate == root {
			return nil
		}
		workspace := ""
		for declaredPath := range declared.workspaces {
			if candidate.Path == path.Join(directory, declaredPath, "package.json") {
				workspace = declaredPath
				break
			}
		}
		if workspace == "" {
			return reject("INVALID_PATH")
		}
		if _, exists := witnesses[workspace]; exists {
			return reject("DUPLICATE_VALUE")
		}
		info, err := parsePackage(evidence, *candidate, facts, prospective, dependencyBudget, false)
		if err != nil {
			return err
		}
		witnesses[workspace] = workspaceManifest{info: info, file: candidate}
		return nil
	}
	if err := check(first); err != nil {
		return nil, err
	}
	for _, candidate := range extras {
		if err := check(candidate); err != nil {
			return nil, err
		}
	}
	return witnesses, nil
}
func parsePackage(evidence *evidenceContext, file sourceFile, facts *[]Fact, prospective, dependencyBudget *int, emitWorkspaces bool) (manifestInfo, error) {
	value, err := decodeObject(file.bytes, dependencyBudget, false)
	if err != nil {
		if _, limited := err.(cError); limited {
			return manifestInfo{}, err
		}
		return manifestInfo{}, reject("MALFORMED_INPUT")
	}
	if err := requireOnlyKeys(value, allowedPackage...); err != nil {
		return manifestInfo{}, err
	}
	name, ok := text(value["name"])
	if !ok || !npmName(name) {
		return manifestInfo{}, reject("MALFORMED_INPUT")
	}
	version, ok := text(value["version"])
	if !ok || !exactVersion(version) {
		return manifestInfo{}, reject("MALFORMED_INPUT")
	}
	info := manifestInfo{name: name, version: version}
	if info.dependencyGroups, err = parseDependencyGroups(value); err != nil {
		return manifestInfo{}, err
	}
	if raw, exists := value["workspaces"]; exists {
		workspaces, ok := raw.([]any)
		if !ok || len(workspaces) > MaxInputs {
			return manifestInfo{}, reject("UNSUPPORTED_SCHEMA")
		}
		info.workspaces = make(map[string]struct{}, len(workspaces))
		previous := ""
		for _, rawWorkspace := range workspaces {
			workspace, ok := text(rawWorkspace)
			if !ok || workspace == "root" || !logicalPath(workspace) {
				return manifestInfo{}, reject("INVALID_PATH")
			}
			if previous >= workspace {
				return manifestInfo{}, reject("DUPLICATE_VALUE")
			}
			previous = workspace
			info.workspaces[workspace] = struct{}{}
			if emitWorkspaces {
				fact, err := makeFact(evidence, file, nil, "js.workspace", name, "declares-workspace", workspace, workspace)
				if err != nil {
					return manifestInfo{}, err
				}
				if err := addFact(facts, fact, prospective); err != nil {
					return manifestInfo{}, err
				}
			}
		}
	}
	return info, nil
}
func dependencyCount(groups [4]map[string]string) int {
	count := 0
	for _, group := range groups {
		count += len(group)
	}
	return count
}
func dependencyInGroups(groups [4]map[string]string, dependency string) bool {
	for _, group := range groups {
		if _, exists := group[dependency]; exists {
			return true
		}
	}
	return false
}
func addSourceFacts(evidence *evidenceContext, file sourceFile, facts *[]Fact, prospective *int) error {
	if len(*facts) >= MaxFacts {
		return reject("LIMIT_EXCEEDED")
	}
	imports, err := importsOfLimit(file.bytes, MaxFacts-len(*facts)-1)
	if err != nil {
		if err == errImportBudget {
			return reject("LIMIT_EXCEEDED")
		}
		if err == errModuleSecret {
			return reject("CREDENTIAL_INPUT")
		}
		if err == errModuleDynamic {
			return reject("DYNAMIC_INPUT")
		}
		if err == errUnsupportedLexical {
			return reject("UNSUPPORTED_SCHEMA")
		}
		return reject("MALFORMED_INPUT")
	}
	classification := "javascript"
	if strings.Contains(path.Base(file.Path), ".test.") || strings.Contains(path.Base(file.Path), ".spec.") {
		classification = "test"
	} else {
		switch path.Ext(file.Path) {
		case ".ts", ".tsx", ".mts", ".cts":
			classification = "typescript"
		}
	}
	fact, err := makeFact(evidence, file, nil, "js.source", file.Path, "classifies", classification, file.Path)
	if err != nil {
		return err
	}
	if err := addFact(facts, fact, prospective); err != nil {
		return err
	}
	for _, module := range imports {
		fact, err := makeFact(evidence, file, nil, "js.import.static", file.Path, "imports", module, file.Path)
		if err != nil {
			return err
		}
		if err := addFact(facts, fact, prospective); err != nil {
			return err
		}
	}
	return nil
}
