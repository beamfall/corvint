package companionrelease

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type npmLock struct {
	Packages map[string]struct{ Version, License, Integrity string } `json:"packages"`
}
type npmNotice struct{ Package, Version, License, Integrity string }

func buildOnlyNPMNotice(export Export) (ArchiveEntry, error) {
	lockFile, ok := findExportFile(export, "extensions/vscode/package-lock.json")
	if !ok {
		return ArchiveEntry{}, fmt.Errorf("VSIX lockfile absent from export")
	}
	var lock npmLock
	if err := json.Unmarshal(lockFile.Data, &lock); err != nil {
		return ArchiveEntry{}, err
	}
	var notices []npmNotice
	for path, item := range lock.Packages {
		if path == "" {
			continue
		}
		name := path
		if at := strings.LastIndex(name, "node_modules/"); at >= 0 {
			name = name[at+len("node_modules/"):]
		}
		if item.Version == "" || item.License == "" {
			return ArchiveEntry{}, fmt.Errorf("build dependency %s lacks version/license", path)
		}
		notices = append(notices, npmNotice{name, item.Version, item.License, item.Integrity})
	}
	sort.Slice(notices, func(i, j int) bool {
		if notices[i].Package == notices[j].Package {
			return notices[i].Version < notices[j].Version
		}
		return notices[i].Package < notices[j].Package
	})
	var b strings.Builder
	b.WriteString("VSIX build-only npm dependency inventory\nThese packages compile/package the VSIX and are not shipped in it. Runtime dependencies: zero.\n\n")
	for _, n := range notices {
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\n", n.Package, n.Version, n.License, n.Integrity)
	}
	return ArchiveEntry{Path: "notices/corvint/VSIX-BUILD-ONLY-NPM.txt", Mode: 0o644, Data: []byte(b.String())}, nil
}
