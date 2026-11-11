// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const catalogSchema = "corvint.release-binaries/1"

//go:embed binaries.json
var embeddedCatalog []byte

// Catalog is the committed per-binary release catalog (ARTIFACT-BIN-V0-001). It is the
// authority for what may be released; the tool never infers a binary from cmd/.
type Catalog struct {
	Schema      string            `json:"schema"`
	LegalFiles  []string          `json:"legalFiles"`
	Targets     []Target          `json:"targets"`
	VersionSets map[string]string `json:"versionSets"`
	Binaries    []Binary          `json:"binaries"`
}

type Target struct {
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
	Suffix string `json:"binarySuffix"`
}

func (t Target) id() string { return t.GOOS + "/" + t.GOARCH }

// Binary is one cmd/ executable. Role product means it is released as its own archive; role
// support means release tooling that is built explicitly and never archived; role withheld means
// an executable the accepted plan removes or leaves without a 1.0 verdict: it stays classified
// but is never selected, built or archived (ARTIFACT-BIN-V0-520).
// Stamp is how the binary carries its release identity: "" (the default) means its main
// package declares the `-X` stamp variables the release builder sets and `build` checks that
// they are declared; "none" means the binary carries no stamp (an analyzer's `--version` is a
// protocol receipt owned by its analyzer contract) and the release manifest alone records its
// version and commit (ARTIFACT-BIN-V0-002, -508).
type Binary struct {
	Name       string `json:"name"`
	Package    string `json:"package"`
	Role       string `json:"role"`
	VersionSet string `json:"versionSet,omitempty"`
	Stamp      string `json:"stamp,omitempty"`
	Decision   string `json:"decision"`
}

// stampVariables is what the release builder stamps into the main package: the build number and
// source commit for every stamped binary, and the version too for an independent binary, whose
// version is not the repository's compiled VERSION (ARTIFACT-BIN-V0-002).
func (b Binary) stampVariables() []string {
	if b.Stamp == "none" {
		return nil
	}
	if b.VersionSet == "" {
		return []string{"build", "sourceCommit", "version"}
	}
	return []string{"build", "sourceCommit"}
}

var (
	binaryNamePattern = regexp.MustCompile(`^corvint(-[a-z0-9]+)*$`)
	decisionPattern   = regexp.MustCompile(`^[0-9]{4}$`)
)

func decodeStrict(raw []byte, into any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("trailing bytes after the JSON document")
	}
	return nil
}

// loadCatalog reads path, or the embedded catalog when path is empty, and validates it.
func loadCatalog(path string) (Catalog, error) {
	raw := embeddedCatalog
	if path != "" {
		var err error
		raw, err = os.ReadFile(path)
		if err != nil {
			return Catalog{}, err
		}
	}
	var catalog Catalog
	if err := decodeStrict(raw, &catalog); err != nil {
		return Catalog{}, fmt.Errorf("catalog: %w", err)
	}
	if err := catalog.validate(); err != nil {
		return Catalog{}, fmt.Errorf("catalog: %w", err)
	}
	return catalog, nil
}

// validate is fail-closed: a catalog row that cites no decision, names an undeclared version
// set, or points anywhere but ./cmd/<name> is refused (ARTIFACT-BIN-V0-001).
func (c Catalog) validate() error {
	if c.Schema != catalogSchema {
		return fmt.Errorf("schema must be %q", catalogSchema)
	}
	if len(c.LegalFiles) == 0 {
		return errors.New("legalFiles must name at least one file")
	}
	seenLegal := map[string]bool{}
	for _, name := range c.LegalFiles {
		if name == "" || name != filepath.Base(name) || seenLegal[name] {
			return fmt.Errorf("legal file %q must be a unique root-level basename", name)
		}
		seenLegal[name] = true
	}
	if len(c.Targets) == 0 {
		return errors.New("targets must name at least one platform")
	}
	seenTarget := map[string]bool{}
	for _, target := range c.Targets {
		if target.GOOS == "" || target.GOARCH == "" || seenTarget[target.id()] {
			return fmt.Errorf("target %q must be a unique goos/goarch pair", target.id())
		}
		if (target.GOOS == "windows") != (target.Suffix == ".exe") {
			return fmt.Errorf("target %s: binarySuffix must be .exe exactly on windows", target.id())
		}
		seenTarget[target.id()] = true
	}
	for name, reason := range c.VersionSets {
		if name == "" || strings.TrimSpace(reason) == "" {
			return fmt.Errorf("version set %q needs a non-empty name and reason", name)
		}
		// A tag SET/vX.Y.Z and NAME/vX.Y.Z share one grammar (ARTIFACT-BIN-V0-005), so a set
		// named like a binary would make the tag ambiguous.
		for _, binary := range c.Binaries {
			if binary.Name == name {
				return fmt.Errorf("version set %q shares a binary's name; a tag %s/vX.Y.Z would be ambiguous", name, name)
			}
		}
	}
	if len(c.Binaries) == 0 {
		return errors.New("binaries must name at least one executable")
	}
	seen := map[string]bool{}
	for _, binary := range c.Binaries {
		if !binaryNamePattern.MatchString(binary.Name) {
			return fmt.Errorf("binary %q: name must match %s", binary.Name, binaryNamePattern)
		}
		if seen[binary.Name] {
			return fmt.Errorf("binary %q is listed twice", binary.Name)
		}
		seen[binary.Name] = true
		if binary.Package != "./cmd/"+binary.Name {
			return fmt.Errorf("binary %q: package must be ./cmd/%s, found %q", binary.Name, binary.Name, binary.Package)
		}
		if binary.Role != "product" && binary.Role != "support" && binary.Role != "withheld" {
			return fmt.Errorf("binary %q: role must be product, support or withheld, found %q", binary.Name, binary.Role)
		}
		if !decisionPattern.MatchString(binary.Decision) {
			return fmt.Errorf("binary %q cites no accepted decision (want four digits, found %q)", binary.Name, binary.Decision)
		}
		if binary.Stamp != "" && binary.Stamp != "none" {
			return fmt.Errorf("binary %q: stamp must be omitted or none, found %q", binary.Name, binary.Stamp)
		}
		if binary.Stamp == "none" && binary.Role != "product" {
			return fmt.Errorf("binary %q: a %s binary has no stamp rule", binary.Name, binary.Role)
		}
		if binary.VersionSet != "" {
			if binary.Role != "product" {
				return fmt.Errorf("binary %q: a %s binary has no version set", binary.Name, binary.Role)
			}
			if _, declared := c.VersionSets[binary.VersionSet]; !declared {
				return fmt.Errorf("binary %q names undeclared version set %q", binary.Name, binary.VersionSet)
			}
		}
	}
	return nil
}

// checkAgainstTree requires root/cmd and the catalog to name the same executables: a new cmd/
// directory must be classified before it can exist in a release tree, and a catalog row must
// not outlive its source.
func (c Catalog) checkAgainstTree(root string) error {
	entries, err := os.ReadDir(filepath.Join(root, "cmd"))
	if err != nil {
		return err
	}
	inTree := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() {
			inTree[entry.Name()] = true
		}
	}
	var problems []string
	for _, binary := range c.Binaries {
		if !inTree[binary.Name] {
			problems = append(problems, "catalog names "+binary.Name+" but cmd/"+binary.Name+" does not exist")
		}
	}
	listed := c.names(func(Binary) bool { return true })
	for name := range inTree {
		if !listed[name] {
			problems = append(problems, "cmd/"+name+" is not classified in the catalog as product, support or withheld")
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func (c Catalog) names(keep func(Binary) bool) map[string]bool {
	names := map[string]bool{}
	for _, binary := range c.Binaries {
		if keep(binary) {
			names[binary.Name] = true
		}
	}
	return names
}

func (c Catalog) find(name string) (Binary, bool) {
	for _, binary := range c.Binaries {
		if binary.Name == name {
			return binary, true
		}
	}
	return Binary{}, false
}

// refuseUnreleased names why a known binary that is not a product is never released.
func refuseUnreleased(binary Binary) error {
	if binary.Role == "withheld" {
		return fmt.Errorf("withheld-binary: %q is withheld from release by decision %s (the accepted plan does not ship it)", binary.Name, binary.Decision)
	}
	return fmt.Errorf("support-binary: %q is release tooling and is never released", binary.Name)
}

// products returns the product binaries in catalog order; set "" means every product.
func (c Catalog) products(set string) []string {
	var names []string
	for _, binary := range c.Binaries {
		if binary.Role == "product" && (set == "" || binary.VersionSet == set) {
			names = append(names, binary.Name)
		}
	}
	return names
}
