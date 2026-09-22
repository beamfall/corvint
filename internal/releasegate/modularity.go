package releasegate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"path"
	"strconv"
	"strings"
)

func packagePath(module, directory string) string {
	if directory == "." {
		return module
	}
	return strings.TrimSuffix(module, "/") + "/" + strings.Trim(directory, "/")
}

func modularityFindings(ctx context.Context, root string, entries []treeEntry, manifest Manifest, run gitRun) ([]Finding, error) {
	module, err := modulePath(ctx, root, entries, run)
	if err != nil {
		return nil, err
	}
	inventory := map[string]struct{}{}
	for _, name := range manifest.ArtifactInventory {
		inventory[name] = struct{}{}
	}
	packages, err := sourcePackages(ctx, root, entries, inventory, module, manifest.BuildTargets, run)
	if err != nil {
		return nil, err
	}
	roles, err := packageRoles(manifest)
	if err != nil {
		return nil, err
	}
	// The union is useful for a single deterministic finding set, but policy
	// validity is established independently for every pinned target.  A file
	// selected on one target must never fill a role or dependency hole on
	// another target.
	findings := []Finding{}
	for _, target := range manifest.BuildTargets {
		selected, err := sourcePackages(ctx, root, entries, inventory, module, []BuildTarget{target}, run)
		if err != nil {
			return nil, err
		}
		for packageName := range roles {
			if _, ok := selected[packageName]; !ok {
				return nil, fmt.Errorf("target %s/%s omits role package %q", target.GOOS, target.GOARCH, packageName)
			}
		}
		for packageName := range selected {
			if _, ok := roles[packageName]; !ok {
				return nil, fmt.Errorf("target %s/%s selects unclassified production package %q", target.GOOS, target.GOARCH, packageName)
			}
		}
		closure, unknown := dependencyClosure(module, manifest.BuildPackages, selected)
		if len(unknown) != 0 {
			return nil, fmt.Errorf("target %s/%s has an external production dependency", target.GOOS, target.GOARCH)
		}
		for packageName := range closure {
			if roles[packageName] != "core" {
				return nil, fmt.Errorf("target %s/%s core closure reaches %s package %q", target.GOOS, target.GOARCH, roles[packageName], packageName)
			}
		}
		for plugin, ceiling := range manifest.PluginSizeCeilings {
			pluginClosure, pluginUnknown := dependencyClosure(module, []string{plugin}, selected)
			if len(pluginUnknown) != 0 {
				return nil, fmt.Errorf("target %s/%s plugin %q has an external dependency", target.GOOS, target.GOARCH, plugin)
			}
			if err := validatePluginArtifactBinding(pluginClosure, selected, entries, inventory, manifest.PluginArtifacts); err != nil {
				return nil, err
			}
			if size := closureSize(pluginClosure, selected, entries, inventory, manifest.PluginArtifacts); size > ceiling {
				item := selected[plugin]
				findings = append(findings, wholeFinding("plugin-size-ceiling", "plugin dependency and embed closure exceeds configured size ceiling", item.evidence, item.content))
			}
		}
		for _, analyzer := range manifest.AnalyzerPackages {
			analyzerClosure, analyzerUnknown := dependencyClosure(module, []string{analyzer}, selected)
			if len(analyzerUnknown) != 0 {
				return nil, fmt.Errorf("target %s/%s analyzer %q has an external dependency", target.GOOS, target.GOARCH, analyzer)
			}
			size := closureSize(analyzerClosure, selected, entries, inventory, nil)
			if size <= 0 {
				return nil, fmt.Errorf("target %s/%s analyzer %q closure is empty", target.GOOS, target.GOARCH, analyzer)
			}
			if size > manifest.AnalyzerSizeCeilings[analyzer] {
				item := selected[analyzer]
				findings = append(findings, wholeFinding("analyzer-size-ceiling", "analyzer dependency and embed closure exceeds configured size ceiling", item.evidence, item.content))
			}
		}
	}
	for _, name := range append(append(append([]string{}, manifest.BuildPackages...), manifest.CorePackages...), append(manifest.AnalyzerPackages, manifest.PluginPackages...)...) {
		if _, ok := packages[name]; !ok {
			return nil, fmt.Errorf("manifest package %q is not a complete production source package", name)
		}
	}
	for name, item := range packages {
		if _, declared := roles[name]; !declared {
			return nil, fmt.Errorf("selected production package %q has no externally pinned role", name)
		}
		if item.path == "" {
			return nil, fmt.Errorf("selected production package %q is incomplete", name)
		}
	}
	closure, unknown := dependencyClosure(module, manifest.BuildPackages, packages)
	allowances := allowanceMap(manifest.Allowances)
	for packageName := range closure {
		item := packages[packageName]
		for _, pattern := range item.embeds {
			if pattern == "!invalid!" {
				return nil, fmt.Errorf("production embed syntax is unsupported in %q", item.path)
			}
			target := path.Join(item.directory, pattern)
			if _, shipped := inventory[target]; !shipped {
				return nil, fmt.Errorf("production embed %q is absent from artifact inventory", target)
			}
			if _, allowed := allowances[target]; allowed {
				return nil, fmt.Errorf("allowance fixture %q is reachable through production embed", target)
			}
		}
	}
	findings = append(findings, packageRuntimeFindings(packages)...)
	for packageName := range unknown {
		item := packages[packageName]
		findings = append(findings, wholeFinding("production-dependency-unknown", "production source imports package outside the manifest-selected source closure", item.evidence, item.content))
	}
	for _, core := range manifest.CorePackages {
		if !closure[core] {
			item := packages[core]
			findings = append(findings, wholeFinding("core-package-unselected", "declared core package is not in production build closure", item.evidence, item.content))
		}
	}
	for _, analyzer := range manifest.AnalyzerPackages {
		if closure[analyzer] {
			item := packages[analyzer]
			findings = append(findings, wholeFinding("core-analyzer-dependency", "production dependency closure includes analyzer implementation package", item.evidence, item.content))
		}
		if embedded := analyzerEmbed(closure, packages, entries, module, analyzer); embedded != "" {
			item := packages[analyzer]
			findings = append(findings, wholeFinding("core-analyzer-embed", "production embed closure includes analyzer implementation package", item.evidence, item.content))
		}
	}
	coreSize := int64(0)
	for core := range closure {
		coreSize += packages[core].size + embeddedSize(packages[core], entries, inventory)
	}
	if coreSize > manifest.CoreSizeCeiling {
		item := packages[manifest.BuildPackages[0]]
		findings = append(findings, wholeFinding("core-size-ceiling", "production dependency closure exceeds configured size ceiling", item.evidence, item.content))
	}
	for plugin, ceiling := range manifest.PluginSizeCeilings {
		item, ok := packages[plugin]
		if !ok {
			return nil, fmt.Errorf("plugin ceiling package %q is incomplete", plugin)
		}
		if item.size+embeddedSize(item, entries, inventory) > ceiling {
			findings = append(findings, wholeFinding("plugin-size-ceiling", "plugin package exceeds configured size ceiling", item.evidence, item.content))
		}
	}
	return findings, nil
}

func validatePluginArtifactBinding(closure map[string]bool, packages map[string]sourcePackage, entries []treeEntry, inventory map[string]struct{}, artifacts map[string][]string) error {
	bound := map[string]bool{}
	for packageName := range closure {
		for _, artifact := range artifacts[packageName] {
			bound[artifact] = true
		}
	}
	for packageName := range closure {
		directory := packages[packageName].directory
		for _, entry := range entries {
			if _, shipped := inventory[entry.path]; !shipped || !strings.HasPrefix(entry.path, directory+"/") || strings.HasSuffix(entry.path, ".go") {
				continue
			}
			if !bound[entry.path] {
				return fmt.Errorf("shipped plugin artifact %q is not externally bound to its dependency closure", entry.path)
			}
		}
	}
	return nil
}

type sourcePackage struct {
	path, directory string
	imports, embeds []string
	files           []goSource
	size            int64
	evidence        Evidence
	content         []byte
}
type goSource struct {
	path     string
	content  []byte
	evidence Evidence
}

func modulePath(ctx context.Context, root string, entries []treeEntry, run gitRun) (string, error) {
	for _, entry := range entries {
		if entry.path != "go.mod" {
			continue
		}
		data, err := readBlob(ctx, root, entry, run, maxBlobBytes)
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[0] == "module" && strings.Contains(fields[1], "/") {
				return fields[1], nil
			}
		}
	}
	return "", errors.New("release tree has no valid Go module path")
}
func sourcePackages(ctx context.Context, root string, entries []treeEntry, inventory map[string]struct{}, module string, targets []BuildTarget, run gitRun) (map[string]sourcePackage, error) {
	packages := map[string]sourcePackage{}
	for _, entry := range entries {
		if _, accepted := inventory[entry.path]; !accepted || !strings.HasSuffix(entry.path, ".go") || strings.HasSuffix(entry.path, "_test.go") {
			continue
		}
		if entry.mode != "100644" {
			return nil, fmt.Errorf("production Go source %q has non-source mode", entry.path)
		}
		data, err := readBlob(ctx, root, entry, run, maxBlobBytes)
		if err != nil {
			return nil, err
		}
		selected, err := selectedForAnyTarget(entry.path, data, targets)
		if err != nil {
			return nil, fmt.Errorf("cannot select build-tagged source %q: %w", entry.path, err)
		}
		if !selected {
			continue
		}
		imports, err := goImports(data)
		if err != nil {
			return nil, fmt.Errorf("cannot parse production source %q: %w", entry.path, err)
		}
		directory := path.Dir(entry.path)
		key := packagePath(module, directory)
		item := packages[key]
		if item.path != "" && item.directory != directory {
			return nil, errors.New("ambiguous package directory")
		}
		item.path = entry.path
		item.directory = directory
		item.evidence = Evidence{Path: entry.path, BlobOID: entry.oid, BlobSHA256: sha256Hex(data)}
		item.content = data
		item.files = append(item.files, goSource{path: entry.path, content: data, evidence: item.evidence})
		item.size += entry.size
		item.imports = append(item.imports, imports...)
		item.embeds = append(item.embeds, goEmbeds(data)...)
		packages[key] = item
	}
	return packages, nil
}
func hasBuildConstraint(data []byte) bool {
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		trimmed := strings.TrimSpace(string(line))
		if strings.HasPrefix(trimmed, "//go:build") || strings.HasPrefix(trimmed, "// +build") {
			return true
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			break
		}
	}
	return false
}

func selectedForAnyTarget(name string, data []byte, targets []BuildTarget) (bool, error) {
	expression, err := buildConstraint(data)
	if err != nil {
		return false, err
	}
	if expression != nil && !knownConstraintTags(expression, targets) {
		return false, errors.New("build constraint contains an externally unpinned tag")
	}
	for _, target := range targets {
		context := build.Default
		context.GOOS, context.GOARCH, context.Compiler, context.CgoEnabled = target.GOOS, target.GOARCH, target.Compiler, target.CGOEnabled
		context.BuildTags, context.ReleaseTags = append([]string(nil), target.CustomTags...), target.releaseTags()
		context.OpenFile = func(string) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
		selected, matchErr := context.MatchFile(path.Dir(name), path.Base(name))
		if matchErr != nil {
			return false, matchErr
		}
		if selected {
			return true, nil
		}
	}
	return false, nil
}

func buildConstraint(data []byte) (constraint.Expr, error) {
	var modern constraint.Expr
	var legacy constraint.Expr
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		trimmed := strings.TrimSpace(string(line))
		if strings.HasPrefix(trimmed, "//go:build ") {
			if modern != nil {
				return nil, errors.New("multiple //go:build constraints")
			}
			expression, err := constraint.Parse(trimmed)
			if err != nil {
				return nil, err
			}
			modern = expression
		} else if strings.HasPrefix(trimmed, "// +build ") {
			expression, err := constraint.Parse(trimmed)
			if err != nil {
				return nil, err
			}
			if legacy == nil {
				legacy = expression
			} else {
				legacy = &constraint.AndExpr{X: legacy, Y: expression}
			}
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			break
		}
	}
	if modern != nil && legacy != nil && modern.String() != legacy.String() {
		return nil, errors.New("modern and legacy build constraints disagree")
	}
	if modern != nil {
		return modern, nil
	}
	return legacy, nil
}

func (target BuildTarget) matchesTag(tag string) bool {
	for _, allowed := range target.tags() {
		if tag == allowed {
			return true
		}
	}
	return false
}
func (target BuildTarget) tags() []string {
	tags := append([]string{target.GOOS, target.GOARCH, target.Compiler}, target.releaseTags()...)
	tags = append(tags, target.CustomTags...)
	if target.CGOEnabled {
		tags = append(tags, "cgo")
	}
	if target.GOOS != "windows" {
		tags = append(tags, "unix")
	}
	return tags
}
func (target BuildTarget) releaseTags() []string {
	return append([]string(nil), target.ReleaseTags...)
}

func releaseMinor(tag string) (int, bool) {
	if !strings.HasPrefix(tag, "go1.") {
		return 0, false
	}
	minor, err := strconv.Atoi(strings.TrimPrefix(tag, "go1."))
	return minor, err == nil && minor >= 1
}

func knownConstraintTags(expression constraint.Expr, targets []BuildTarget) bool {
	allowed := map[string]bool{}
	for _, target := range targets {
		for _, tag := range target.tags() {
			allowed[tag] = true
		}
	}
	for _, tag := range knownOSArchTags() {
		allowed[tag] = true
	}
	var visit func(constraint.Expr) bool
	visit = func(value constraint.Expr) bool {
		switch item := value.(type) {
		case *constraint.TagExpr:
			return allowed[item.Tag]
		case *constraint.NotExpr:
			return visit(item.X)
		case *constraint.AndExpr:
			return visit(item.X) && visit(item.Y)
		case *constraint.OrExpr:
			return visit(item.X) && visit(item.Y)
		default:
			return false
		}
	}
	return visit(expression)
}

func knownOSArchTags() []string {
	return []string{"aix", "android", "darwin", "dragonfly", "freebsd", "illumos", "ios", "js", "linux", "netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows", "386", "amd64", "arm", "arm64", "loong64", "mips", "mips64", "mips64le", "mipsle", "ppc64", "ppc64le", "riscv64", "s390x", "wasm"}
}

func fileNameMatchesTarget(name string, target BuildTarget) bool {
	base := strings.TrimSuffix(path.Base(name), ".go")
	parts := strings.Split(base, "_")
	if len(parts) < 2 {
		return true
	}
	knownOS := map[string]bool{"aix": true, "android": true, "darwin": true, "dragonfly": true, "freebsd": true, "illumos": true, "ios": true, "js": true, "linux": true, "netbsd": true, "openbsd": true, "plan9": true, "solaris": true, "wasip1": true, "windows": true}
	knownArch := map[string]bool{"386": true, "amd64": true, "arm": true, "arm64": true, "loong64": true, "mips": true, "mips64": true, "mips64le": true, "mipsle": true, "ppc64": true, "ppc64le": true, "riscv64": true, "s390x": true, "wasm": true}
	for _, suffix := range parts[1:] {
		if knownOS[suffix] && suffix != target.GOOS || knownArch[suffix] && suffix != target.GOARCH {
			return false
		}
	}
	return true
}

// packageRuntimeFindings type-checks all selected files together.  Object
// identity, rather than spelling, distinguishes real os/exec calls from local
// shadows while allowing dot imports and aliases defined in another file.
func packageRuntimeFindings(packages map[string]sourcePackage) []Finding {
	var findings []Finding
	for packageName, item := range packages {
		files := token.NewFileSet()
		parsed := make([]*ast.File, 0, len(item.files))
		sources := map[*ast.File]goSource{}
		bad := false
		for _, source := range item.files {
			file, err := parser.ParseFile(files, source.path, source.content, parser.AllErrors)
			if err != nil {
				findings = append(findings, wholeFinding("go-analysis-unknown", "Go source parse ambiguity prevents safe execution analysis", source.evidence, source.content))
				bad = true
				continue
			}
			parsed, sources[file] = append(parsed, file), source
		}
		if bad || len(parsed) == 0 {
			continue
		}
		info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}}
		conf := types.Config{Importer: releaseGateImporter{}, Error: func(error) {}}
		_, _ = conf.Check(packageName, files, parsed, info)
		aliases := execFunctionAliases(parsed, info)
		for file, source := range sources {
			if bytes.Contains(source.content, []byte("//go:linkname")) {
				findings = append(findings, wholeFinding("linkname-forbidden", "go:linkname bypasses exact process and capability analysis", source.evidence, source.content))
			}
			for _, imported := range file.Imports {
				if imported.Path.Value == "\"C\"" {
					findings = append(findings, wholeFinding("cgo-forbidden", "selected production source imports C despite CGO_ENABLED=0 policy", source.evidence, source.content))
				}
				if importPath, err := strconv.Unquote(imported.Path.Value); err != nil {
					findings = append(findings, wholeFinding("go-analysis-unknown", "invalid Go import prevents safe release analysis", source.evidence, source.content))
				} else if importPath == "os/exec" {
					findings = append(findings, wholeFinding("process-execution-forbidden", "selected production source imports os/exec process capability", source.evidence, source.content))
				} else if networkCapableImport(importPath) {
					findings = append(findings, wholeFinding("network-capability-forbidden", "selected production source imports a network or DNS-capable API", source.evidence, source.content))
				} else if syscallCapableImport(importPath) {
					findings = append(findings, wholeFinding("syscall-capability-forbidden", "selected production source imports a syscall-capable API", source.evidence, source.content))
				}
			}
			findings = append(findings, execValueEscapeFindings(file, info, aliases, files, source)...)
			findings = append(findings, execEscapedValueFindings(file, info, aliases, files, source)...)
			findings = append(findings, processMethodValueFindings(file, info, files, source)...)
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				if processObjectCall(info, call.Fun) {
					start, end := files.Position(call.Pos()).Offset, files.Position(call.End()).Offset
					findings = append(findings, spanFinding("process-execution-forbidden", "os/exec process object method is forbidden", source.evidence, source.content, start, end))
					return true
				}
				kind, known := execFunctionCall(info, aliases, call.Fun)
				if !known {
					if identifier, ok := call.Fun.(*ast.Ident); ok && isFunctionVariable(info.Uses[identifier]) {
						start, end := files.Position(call.Pos()).Offset, files.Position(call.End()).Offset
						findings = append(findings, spanFinding("go-analysis-unknown", "unresolved function-variable call prevents safe execution analysis", source.evidence, source.content, start, end))
					}
					return true
				}
				start, end := files.Position(call.Pos()).Offset, files.Position(call.End()).Offset
				argument := 0
				if kind == "exec-command-context" {
					argument = 1
				}
				if len(call.Args) <= argument {
					return true
				}
				command, exact := constantString(info, call.Args[argument])
				if !exact {
					findings = append(findings, spanFinding("go-analysis-unknown", "dynamic command executable prevents safe release analysis", source.evidence, source.content, start, end))
					return true
				}
				if pythonInterpreter(command) {
					findings = append(findings, spanFinding("python-execution", "Python interpreter execution command", source.evidence, source.content, start, end))
					return true
				}
				if environmentInterpreter(command) {
					for _, value := range call.Args[argument+1:] {
						text, ok := constantString(info, value)
						if !ok {
							findings = append(findings, spanFinding("go-analysis-unknown", "dynamic environment interpreter arguments prevent safe release analysis", source.evidence, source.content, start, end))
							return true
						}
						if pythonInterpreter(text) {
							findings = append(findings, spanFinding("python-execution", "environment launcher executes Python", source.evidence, source.content, start, end))
							return true
						}
					}
				}
				if shellInterpreter(command) {
					payload, exact := shellPayload(info, call.Args[argument+1:])
					if !exact {
						findings = append(findings, spanFinding("go-analysis-unknown", "dynamic shell payload prevents safe release analysis", source.evidence, source.content, start, end))
					} else if hasPythonCommand(payload) {
						findings = append(findings, spanFinding("python-shell-execution", "shell payload executes Python", source.evidence, source.content, start, end))
					} else {
						findings = append(findings, spanFinding("shell-execution", "shell execution is forbidden in a native release", source.evidence, source.content, start, end))
					}
				}
				return true
			})
		}
	}
	return findings
}

func execFunctionAliases(files []*ast.File, info *types.Info) map[types.Object]string {
	aliases := map[types.Object]string{}
	changed := true
	for changed {
		changed = false
		for _, file := range files {
			ast.Inspect(file, func(node ast.Node) bool {
				switch value := node.(type) {
				case *ast.ValueSpec:
					for index, name := range value.Names {
						if index < len(value.Values) {
							if kind, known := execFunctionCall(info, aliases, value.Values[index]); known {
								if object := info.Defs[name]; object != nil && aliases[object] == "" {
									aliases[object], changed = kind, true
								}
							}
						}
					}
				case *ast.AssignStmt:
					if value.Tok.String() == ":=" || value.Tok.String() == "=" {
						for index, left := range value.Lhs {
							if index < len(value.Rhs) {
								if ident, ok := left.(*ast.Ident); ok {
									if kind, known := execFunctionCall(info, aliases, value.Rhs[index]); known {
										object := info.Defs[ident]
										if value.Tok.String() == "=" {
											object = info.Uses[ident]
										}
										if object != nil && aliases[object] == "" {
											aliases[object], changed = kind, true
										}
									}
								}
							}
						}
					}
				}
				return true
			})
		}
	}
	return aliases
}

func isFunctionVariable(object types.Object) bool {
	if object == nil {
		return false
	}
	_, variable := object.(*types.Var)
	_, signature := object.Type().Underlying().(*types.Signature)
	return variable && signature
}

func embeddedSize(item sourcePackage, entries []treeEntry, inventory map[string]struct{}) int64 {
	var total int64
	for _, pattern := range item.embeds {
		if pattern == "!invalid!" {
			continue
		}
		name := path.Join(item.directory, pattern)
		if _, shipped := inventory[name]; !shipped {
			continue
		}
		for _, entry := range entries {
			if entry.path == name {
				total += entry.size
				break
			}
		}
	}
	return total
}

func closureSize(closure map[string]bool, packages map[string]sourcePackage, entries []treeEntry, inventory map[string]struct{}, artifacts map[string][]string) int64 {
	seenAssets, total := map[string]bool{}, int64(0)
	selectedSources := map[string]bool{}
	for packageName := range closure {
		for _, source := range packages[packageName].files {
			selectedSources[source.path] = true
		}
	}
	assetSize := func(name string) int64 {
		if seenAssets[name] {
			return 0
		}
		seenAssets[name] = true
		for _, entry := range entries {
			if entry.path == name {
				if entry.mode != "100644" && entry.mode != "100755" {
					return 1 << 62
				}
				return entry.size
			}
		}
		return 1 << 62
	}
	for packageName := range closure {
		item := packages[packageName]
		total += item.size
		// A shipped Go-shaped file remains executable release surface even when
		// Go excludes it from this target or from ordinary package analysis
		// (notably *_test.go). Count it conservatively in the package closure.
		for _, entry := range entries {
			if _, shipped := inventory[entry.path]; !shipped || selectedSources[entry.path] || !strings.HasPrefix(entry.path, item.directory+"/") || !strings.HasSuffix(entry.path, ".go") {
				continue
			}
			if entry.mode != "100644" {
				return 1 << 62
			}
			total += entry.size
		}
		for _, artifact := range artifacts[packageName] {
			total += assetSize(artifact)
		}
		for _, pattern := range item.embeds {
			name := path.Join(item.directory, pattern)
			if pattern == "!invalid!" {
				continue
			}
			if _, shipped := inventory[name]; !shipped {
				continue
			}
			total += assetSize(name)
		}
	}
	return total
}

func packageRoles(manifest Manifest) (map[string]string, error) {
	roles := map[string]string{}
	for role, names := range map[string][]string{"core": manifest.CorePackages, "plugin": manifest.PluginPackages, "analyzer": manifest.AnalyzerPackages} {
		for _, name := range names {
			if name == "" || roles[name] != "" {
				return nil, fmt.Errorf("production package %q has ambiguous role", name)
			}
			roles[name] = role
		}
	}
	return roles, nil
}

func execFunctionCall(info *types.Info, aliases map[types.Object]string, expression ast.Expr) (string, bool) {
	for {
		parenthesized, ok := expression.(*ast.ParenExpr)
		if !ok {
			break
		}
		expression = parenthesized.X
	}
	if asserted, ok := expression.(*ast.TypeAssertExpr); ok {
		if identifier, ok := asserted.X.(*ast.Ident); ok {
			kind, known := aliases[info.Uses[identifier]]
			return kind, known
		}
	}
	if kind, _, known := commandCall(info, expression); known {
		return kind, true
	}
	if identifier, ok := expression.(*ast.Ident); ok {
		if function, ok := info.Uses[identifier].(*types.Func); ok && function.Pkg() != nil && function.Pkg().Path() == "os/exec" && (function.Name() == "Command" || function.Name() == "CommandContext") {
			if function.Name() == "CommandContext" {
				return "exec-command-context", true
			}
			return "exec-command", true
		}
		if kind, ok := aliases[info.Uses[identifier]]; ok {
			return kind, true
		}
	}
	return "", false
}

func processObjectCall(info *types.Info, expression ast.Expr) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	function, ok := info.Uses[selector.Sel].(*types.Func)
	if !ok || function.Pkg() == nil || function.Pkg().Path() != "os/exec" {
		return false
	}
	switch function.Name() {
	case "Run", "Start", "Wait", "Output", "CombinedOutput":
		return true
	}
	return false
}

func processMethodValueFindings(file *ast.File, info *types.Info, files *token.FileSet, source goSource) []Finding {
	var findings []Finding
	var stack []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		var parent ast.Node
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		stack = append(stack, node)
		expression, ok := node.(ast.Expr)
		if !ok || !processObjectCall(info, expression) {
			return true
		}
		if call, direct := parent.(*ast.CallExpr); direct && call.Fun == expression {
			return true
		}
		start, end := files.Position(expression.Pos()).Offset, files.Position(expression.End()).Offset
		findings = append(findings, spanFinding("process-execution-forbidden", "os/exec process method value escapes exact call analysis", source.evidence, source.content, start, end))
		return true
	})
	return findings
}

func networkCapableImport(name string) bool {
	if name == "crypto/tls" || name == "log/syslog" {
		return true
	}
	if name == "net" || strings.HasPrefix(name, "net/") {
		return name != "net/url" && name != "net/mail" && name != "net/netip"
	}
	return false
}
func syscallCapableImport(name string) bool {
	return name == "syscall" || name == "os" || strings.HasPrefix(name, "golang.org/x/sys/")
}

// execValueEscapeFindings rejects any Command function value whose exact
// function type cannot be retained in a local identifier flow.  This keeps
// direct aliases analyzable while failing closed for interfaces, aggregates,
// returns, parameters, and closure captures.
func execValueEscapeFindings(file *ast.File, info *types.Info, aliases map[types.Object]string, files *token.FileSet, source goSource) []Finding {
	var out []Finding
	report := func(node ast.Node, detail string) {
		start, end := files.Position(node.Pos()).Offset, files.Position(node.End()).Offset
		out = append(out, spanFinding("go-analysis-unknown", detail, source.evidence, source.content, start, end))
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.ValueSpec:
			for index, rhs := range value.Values {
				if _, known := execFunctionCall(info, aliases, rhs); known {
					if index >= len(value.Names) || !isFunctionVariable(info.Defs[value.Names[index]]) {
						report(rhs, "os/exec function value escapes through a non-function declaration")
					}
				}
			}
		case *ast.AssignStmt:
			for index, rhs := range value.Rhs {
				if _, known := execFunctionCall(info, aliases, rhs); known {
					if index >= len(value.Lhs) {
						report(rhs, "os/exec function value assignment is ambiguous")
						continue
					}
					ident, ok := value.Lhs[index].(*ast.Ident)
					object := info.Defs[ident]
					if value.Tok.String() == "=" {
						object = info.Uses[ident]
					}
					if !ok || !isFunctionVariable(object) {
						report(rhs, "os/exec function value escapes through a non-function assignment")
					}
				}
			}
		}
		return true
	})
	return out
}

func execEscapedValueFindings(file *ast.File, info *types.Info, aliases map[types.Object]string, files *token.FileSet, source goSource) []Finding {
	var out []Finding
	seen := map[token.Pos]bool{}
	report := func(node ast.Node, detail string) {
		if seen[node.Pos()] {
			return
		}
		seen[node.Pos()] = true
		start, end := files.Position(node.Pos()).Offset, files.Position(node.End()).Offset
		out = append(out, spanFinding("go-analysis-unknown", detail, source.evidence, source.content, start, end))
	}
	var stack []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		var parent ast.Node
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		stack = append(stack, node)
		expression, ok := node.(ast.Expr)
		if !ok {
			return true
		}
		_, known := execFunctionCall(info, aliases, expression)
		if !known {
			return true
		}
		if !withinFunctionLiteral(stack[:len(stack)-1]) && execValueUseAllowed(expression, parent, info) {
			return true
		}
		report(node, "os/exec function value escapes exact local alias analysis")
		return true
	})
	// Any later non-tainted write to a tainted local makes its call flow
	// ambiguous, regardless of source ordering.
	ast.Inspect(file, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for index, left := range assignment.Lhs {
			ident, ok := left.(*ast.Ident)
			if !ok || index >= len(assignment.Rhs) {
				continue
			}
			object := info.Defs[ident]
			if assignment.Tok.String() == "=" {
				object = info.Uses[ident]
			}
			if _, tainted := aliases[object]; tainted {
				if _, stillTainted := execFunctionCall(info, aliases, assignment.Rhs[index]); !stillTainted {
					report(assignment, "os/exec alias is reassigned through an unproven value flow")
				}
			}
		}
		return true
	})
	return out
}

func withinFunctionLiteral(ancestors []ast.Node) bool {
	for _, ancestor := range ancestors {
		if _, ok := ancestor.(*ast.FuncLit); ok {
			return true
		}
	}
	return false
}

func execValueUseAllowed(expression ast.Expr, parent ast.Node, info *types.Info) bool {
	if call, ok := parent.(*ast.CallExpr); ok && call.Fun == expression {
		_, assertion := expression.(*ast.TypeAssertExpr)
		return !assertion
	}
	if values, ok := parent.(*ast.ValueSpec); ok {
		for index, value := range values.Values {
			if value == expression && index < len(values.Names) {
				name := values.Names[index]
				return !name.IsExported() && isFunctionVariable(info.Defs[name])
			}
		}
	}
	if assignment, ok := parent.(*ast.AssignStmt); ok {
		for index, value := range assignment.Rhs {
			if value == expression && index < len(assignment.Lhs) {
				ident, ok := assignment.Lhs[index].(*ast.Ident)
				if !ok || ident.IsExported() {
					return false
				}
				object := info.Defs[ident]
				if assignment.Tok.String() == "=" {
					object = info.Uses[ident]
				}
				return isFunctionVariable(object)
			}
		}
	}
	return false
}
func goEmbeds(data []byte) []string {
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "//go:embed ") {
			continue
		}
		for _, pattern := range strings.Fields(strings.TrimPrefix(line, "//go:embed ")) {
			pattern = strings.TrimPrefix(pattern, "all:")
			if !safePath(pattern) || strings.ContainsAny(pattern, "*?[") {
				out = append(out, "!invalid!")
			} else {
				out = append(out, pattern)
			}
		}
	}
	return out
}
func goImports(data []byte) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "release-tree.go", data, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(file.Imports))
	for _, imported := range file.Imports {
		value, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}
func dependencyClosure(module string, roots []string, packages map[string]sourcePackage) (map[string]bool, map[string]bool) {
	seen, unknown := map[string]bool{}, map[string]bool{}
	queue := append([]string(nil), roots...)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if seen[current] {
			continue
		}
		seen[current] = true
		item := packages[current]
		for _, imported := range item.imports {
			if _, ok := packages[imported]; ok {
				queue = append(queue, imported)
			} else if imported == module || strings.HasPrefix(imported, module+"/") || externalImport(imported) {
				unknown[current] = true
			}
		}
	}
	return seen, unknown
}

func externalImport(name string) bool {
	if strings.HasPrefix(name, ".") {
		return false
	}
	first, _, _ := strings.Cut(name, "/")
	return strings.Contains(first, ".")
}
func analyzerEmbed(closure map[string]bool, packages map[string]sourcePackage, entries []treeEntry, module, analyzer string) string {
	analyzerDirectory := strings.TrimPrefix(analyzer, strings.TrimSuffix(module, "/")+"/")
	if analyzerDirectory == analyzer {
		return ""
	}
	for packageName := range closure {
		for _, pattern := range packages[packageName].embeds {
			if pattern == "!invalid!" {
				return packages[packageName].path
			}
			full := path.Join(packages[packageName].directory, pattern)
			for _, entry := range entries {
				if entry.path == analyzerDirectory || strings.HasPrefix(entry.path, analyzerDirectory+"/") {
					matched, err := path.Match(full, entry.path)
					if err != nil || matched {
						return entry.path
					}
				}
			}
		}
	}
	return ""
}
