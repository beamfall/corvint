package companionrelease

import (
	"encoding/json"
	"fmt"
)

// currentBundleProfile is the writer's profile: profile /2 plus the
// corvint-ruby-test-provider component (CRB-V0-020). Profile /2 stays readable
// with exactly its nine binaries.
const (
	currentBundleProfile = "corvint-companion-bundle/3"
	coreBundleProfile    = "corvint-companion-bundle/2"
)

type bundleInventory struct {
	current bool
	pi      bool
	core    bool
	// ruby is true for profile /3, whose components add corvint-ruby-test-provider.
	ruby bool
}

func inventoryForManifest(m BundleManifest) (bundleInventory, error) {
	if len(m.Profile) == 0 {
		return bundleInventory{}, nil
	}
	var profile string
	if err := json.Unmarshal(m.Profile, &profile); err != nil || (profile != currentBundleProfile && profile != coreBundleProfile && profile != "corvint-companion-bundle/1" && profile != "corvint-companion-bundle/0") {
		return bundleInventory{}, fmt.Errorf("unsupported companion bundle profile %s", m.Profile)
	}
	core := profile == currentBundleProfile || profile == coreBundleProfile
	return bundleInventory{current: true, pi: profile != "corvint-companion-bundle/0", core: core, ruby: profile == currentBundleProfile}, nil
}

func (i bundleInventory) name(legacy string) string {
	if !i.current {
		return legacy
	}
	switch legacy {
	case "corvint":
		return "corvint"
	case "corvint-console":
		return "corvint-console"
	case "corvint-dashboard-snapshot":
		return "corvint-dashboard-snapshot"
	case "corvint-mcp":
		return "corvint-mcp"
	case "corvint-docs-mcp":
		return "corvint-docs-mcp"
	case "corvint-test-validity-mcp":
		return "corvint-test-validity-mcp"
	case "corvint-js-test-provider":
		return "corvint-js-test-provider"
	case "corvint-go-test-provider":
		return "corvint-go-test-provider"
	case "corvint-ruby-test-provider":
		return "corvint-ruby-test-provider"
	case "atm":
		return "corvint-tasks"
	case "corvint-taskman":
		return "corvint-tasks"
	case "corvint-vscode":
		return "corvint-vscode"
	}
	return legacy
}
func (i bundleInventory) source(module string) string {
	return "source/" + i.name(module) + "-src.tar.gz"
}
func (i bundleInventory) binary(name string) string { return "bin/" + i.name(name) }
func (i bundleInventory) vsix() string {
	return "extensions/" + i.name("corvint-vscode") + "-0.1.0.vsix"
}
