package cli

import (
	"regexp"
	"slices"
	"strings"
)

const (
	// pluginVersionVariable and pluginToolsVariable are the launching plugin's
	// declaration (MCPV0-951): HOST/VERSION and the comma list of tools it expects.
	pluginVersionVariable = "CORVINT_PLUGIN_VERSION"
	pluginToolsVariable   = "CORVINT_PLUGIN_TOOLS"
	// pluginCompatibilityMeta is the corvint.status result _meta member.
	pluginCompatibilityMeta    = "io.corvint/pluginCompatibility"
	pluginCompatibilityProfile = "corvint-plugin-compatibility/0"
	maxPluginTools             = 64
)

var (
	pluginVersionPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}/[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$`)
	pluginToolPattern    = regexp.MustCompile(`^corvint\.[a-z0-9_.]{1,64}$`)
)

// pluginDeclaration is what the launching plugin says it is and expects; a
// declaration that breaks the bounds is kept as malformed, never guessed at.
type pluginDeclaration struct {
	version   string
	tools     []string
	malformed bool
}

// declaredPlugin reads the plugin declaration once at startup; nil when the
// launcher declares no plugin version, so a bare server's bytes are unchanged.
func declaredPlugin(lookup func(string) (string, bool)) *pluginDeclaration {
	version, declared := lookup(pluginVersionVariable)
	if !declared {
		return nil
	}
	declaration := &pluginDeclaration{version: version}
	toolList, _ := lookup(pluginToolsVariable)
	if toolList != "" {
		declaration.tools = strings.Split(toolList, ",")
	}
	declaration.malformed = !pluginVersionPattern.MatchString(version) || len(declaration.tools) == 0 || len(declaration.tools) > maxPluginTools ||
		slices.ContainsFunc(declaration.tools, func(tool string) bool { return !pluginToolPattern.MatchString(tool) })
	if declaration.malformed {
		declaration.version, declaration.tools = "", nil
	}
	return declaration
}

// notePluginCompatibility adds the plugin/binary compatibility of the declared
// plugin to a corvint.status result (MCPV0-951): a _meta object, and one text
// block that names every expected tool this server does not list. Compatible
// means the server lists every tool the plugin expects; the status receipt
// itself is unchanged.
func (handler *toolHandler) notePluginCompatibility(response map[string]any) {
	if handler.plugin == nil {
		return
	}
	server := serverName + " " + handler.version
	value := map[string]any{"profile": pluginCompatibilityProfile, "server": server}
	var text string
	if handler.plugin.malformed {
		value["compatible"], value["reason"] = false, "malformed-declaration"
		text = "corvint plugin compatibility: UNKNOWN: the plugin's " + pluginVersionVariable + "/" + pluginToolsVariable + " declaration is malformed; " + server
	} else {
		listed := map[string]bool{}
		for _, tool := range handler.listTools() {
			listed[tool.Name] = true
		}
		missing := []string{}
		for _, tool := range handler.plugin.tools {
			if !listed[tool] {
				missing = append(missing, tool)
			}
		}
		slices.Sort(missing)
		missing = slices.Compact(missing)
		value["plugin"], value["expectedTools"], value["missingTools"], value["compatible"] = handler.plugin.version, len(handler.plugin.tools), missing, len(missing) == 0
		text = "corvint plugin compatibility: plugin " + handler.plugin.version + " with " + server + ": compatible"
		if len(missing) != 0 {
			text = "corvint plugin compatibility: INCOMPATIBLE: plugin " + handler.plugin.version + " expects " + strings.Join(missing, ", ") +
				", which " + server + " does not serve; update the corvint binary or the plugin so they match"
		}
	}
	meta, _ := response["_meta"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
	}
	meta[pluginCompatibilityMeta] = value
	response["_meta"] = meta
	content, _ := response["content"].([]any)
	response["content"] = append(content, map[string]any{"type": "text", "text": text})
}
