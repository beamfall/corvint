package doccompiler

import (
	"bufio"
	"path/filepath"
	"sort"
	"strings"
)

func observeConfig(raw []byte) ConfigObservations {
	observation := ConfigObservations{
		Authority:    "mkdocs build --strict",
		ParserStatus: "lexical-observation-only; not a YAML or MkDocs parser",
		DocsDir:      "docs",
	}
	section := ""
	subsection := ""
	topKeys := map[string]int{}
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		line := stripComment(scanner.Text())
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Contains(line, "\t") {
			observation.Uncertainty = append(observation.Uncertainty, "tab indentation prevents reliable lexical observation")
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "!include") || strings.Contains(trimmed, "!!python") {
			observation.InheritsConfig = true
		}
		observeOfflinePrivacySignals(&observation, trimmed)
		key, value, hasKey := splitKeyValue(trimmed)
		if indent == 0 && hasKey {
			section = key
			subsection = ""
			topKeys[key]++
			observeTopLevel(&observation, key, value)
			continue
		}
		if indent > 0 && hasKey && !strings.HasPrefix(trimmed, "-") {
			subsection = key
			observeNested(&observation, section, key, value)
			continue
		}
		if strings.HasPrefix(trimmed, "-") {
			item := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			observeListItem(&observation, section, subsection, item)
		}
	}
	for key, count := range topKeys {
		if count > 1 {
			observation.Uncertainty = append(observation.Uncertainty, "duplicate top-level key observed: "+key)
		}
	}
	if !observation.PluginsConfigured {
		observation.SearchStatus = "implicit-default"
	} else if contains(observation.Plugins, "search") {
		observation.SearchStatus = "explicit"
	} else {
		observation.SearchStatus = "missing-after-plugins-configured"
		observation.Uncertainty = append(observation.Uncertainty, "built-in search is not lexically re-added after plugins is configured")
	}
	observation.ThemeFeatures = uniqueSorted(observation.ThemeFeatures)
	observation.Plugins = uniqueSorted(observation.Plugins)
	observation.MarkdownExtensions = uniqueSorted(observation.MarkdownExtensions)
	observation.ExtraCSS = uniqueSorted(observation.ExtraCSS)
	observation.ExtraJavaScript = uniqueSorted(observation.ExtraJavaScript)
	observation.Hooks = uniqueSorted(observation.Hooks)
	observation.PrivacySettings = uniqueSorted(observation.PrivacySettings)
	observation.OfflineRelated = uniqueSorted(observation.OfflineRelated)
	observation.RemoteAssets = uniqueSorted(observation.RemoteAssets)
	observation.Uncertainty = uniqueSorted(observation.Uncertainty)
	return observation
}

func observeTopLevel(observation *ConfigObservations, key, value string) {
	switch key {
	case "docs_dir":
		if scalar := cleanScalar(value); scalar != "" {
			observation.DocsDir = scalar
		}
	case "nav":
		observation.NavConfigured = true
	case "site_url":
		observation.SiteURL = cleanScalar(value)
	case "theme":
		if scalar := cleanScalar(value); scalar != "" {
			observation.ThemeName = scalar
		}
	case "plugins":
		observation.PluginsConfigured = true
		for _, item := range inlineList(value) {
			observation.Plugins = append(observation.Plugins, itemName(item))
		}
	case "markdown_extensions":
		for _, item := range inlineList(value) {
			observation.MarkdownExtensions = append(observation.MarkdownExtensions, itemName(item))
		}
	case "extra_css":
		observation.ExtraCSS = append(observation.ExtraCSS, inlineList(value)...)
	case "extra_javascript":
		observation.ExtraJavaScript = append(observation.ExtraJavaScript, inlineList(value)...)
	case "hooks":
		observation.Hooks = append(observation.Hooks, inlineList(value)...)
	case "INHERIT":
		observation.InheritsConfig = true
	case "validation":
		observation.ValidationConfigured = true
	}
}

func observeNested(observation *ConfigObservations, section, key, value string) {
	if section == "theme" {
		switch key {
		case "name":
			observation.ThemeName = cleanScalar(value)
		case "custom_dir":
			observation.ThemeCustomDir = cleanScalar(value)
		}
	}
	if section == "plugins" && key != "enabled" {
		observation.Plugins = append(observation.Plugins, itemName(key))
	}
	if section == "markdown_extensions" {
		observation.MarkdownExtensions = append(observation.MarkdownExtensions, itemName(key))
	}
}

func observeListItem(observation *ConfigObservations, section, subsection, item string) {
	name := itemName(item)
	if name == "" {
		return
	}
	switch section {
	case "plugins":
		observation.Plugins = append(observation.Plugins, name)
	case "markdown_extensions":
		observation.MarkdownExtensions = append(observation.MarkdownExtensions, name)
	case "extra_css":
		observation.ExtraCSS = append(observation.ExtraCSS, cleanScalar(item))
	case "extra_javascript":
		observation.ExtraJavaScript = append(observation.ExtraJavaScript, cleanScalar(item))
	case "hooks":
		observation.Hooks = append(observation.Hooks, cleanScalar(item))
	case "theme":
		if subsection == "features" {
			observation.ThemeFeatures = append(observation.ThemeFeatures, cleanScalar(item))
		}
	}
}

func lexicalMaterialPreflight(observation ConfigObservations) (string, []string) {
	reasons := append([]string(nil), observation.Uncertainty...)
	if observation.ThemeName != "material" {
		reasons = append(reasons, "theme.name is not lexically pinned to material")
	}
	if observation.InheritsConfig {
		reasons = append(reasons, "config inheritance or include semantics are outside the P0 boundary")
	}
	if len(observation.Hooks) > 0 {
		reasons = append(reasons, "hooks are project code and outside the qualified P0 execution boundary")
	}
	for _, plugin := range observation.Plugins {
		if plugin != "search" {
			reasons = append(reasons, "plugin is outside the qualified P0 execution boundary: "+plugin)
		}
	}
	if observation.PluginsConfigured && observation.SearchStatus != "explicit" {
		reasons = append(reasons, "configured plugins must explicitly re-add search for the qualified profile")
	}
	for _, extension := range observation.MarkdownExtensions {
		if extension == "pymdownx.snippets" {
			reasons = append(reasons, "pymdownx.snippets may read paths outside docs_dir")
		}
		if !knownMarkdownExtension(extension) {
			reasons = append(reasons, "Markdown extension is outside the qualified P0 allowlist: "+extension)
		}
	}
	if _, err := cleanRelative(observation.DocsDir); err != nil {
		reasons = append(reasons, "docs_dir is not a contained relative path")
	}
	if observation.ThemeCustomDir != "" {
		if _, err := cleanRelative(observation.ThemeCustomDir); err != nil {
			reasons = append(reasons, "theme.custom_dir is not a contained relative path")
		}
	}
	for _, asset := range append(append([]string{}, observation.ExtraCSS...), observation.ExtraJavaScript...) {
		if strings.Contains(asset, "://") || strings.HasPrefix(asset, "//") {
			reasons = append(reasons, "remote asset remains an offline/privacy uncertainty: "+asset)
		}
	}
	reasons = uniqueSorted(reasons)
	if len(reasons) > 0 {
		return "PRECHECK_FAILED", reasons
	}
	return "PRECHECK_PASS", nil
}

func knownMarkdownExtension(extension string) bool {
	builtin := map[string]struct{}{
		"abbr": {}, "admonition": {}, "attr_list": {}, "codehilite": {}, "def_list": {},
		"extra": {}, "fenced_code": {}, "footnotes": {}, "legacy_attrs": {}, "legacy_em": {},
		"md_in_html": {}, "meta": {}, "nl2br": {}, "sane_lists": {}, "smarty": {},
		"tables": {}, "toc": {}, "wikilinks": {},
	}
	if _, exists := builtin[extension]; exists {
		return true
	}
	return strings.HasPrefix(extension, "pymdownx.") && extension != "pymdownx.snippets"
}

func observeOfflinePrivacySignals(observation *ConfigObservations, trimmed string) {
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "assets_fetch") || strings.Contains(lower, "assets: false") || strings.Contains(lower, "assets: true") {
		observation.PrivacySettings = append(observation.PrivacySettings, trimmed)
	}
	if strings.Contains(lower, "offline") || strings.Contains(lower, "privacy") {
		observation.OfflineRelated = append(observation.OfflineRelated, trimmed)
	}
	if strings.Contains(lower, "http://") || strings.Contains(lower, "https://") || strings.Contains(lower, "//cdn.") {
		observation.RemoteAssets = append(observation.RemoteAssets, trimmed)
	}
}

func splitKeyValue(value string) (string, string, bool) {
	index := strings.Index(value, ":")
	if index < 0 {
		return "", "", false
	}
	key := cleanScalar(strings.TrimSpace(strings.TrimPrefix(value[:index], "-")))
	if key == "" {
		return "", "", false
	}
	return key, strings.TrimSpace(value[index+1:]), true
}

func stripComment(value string) string {
	inSingle := false
	inDouble := false
	for index, character := range value {
		switch character {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return value[:index]
			}
		}
	}
	return value
}

func cleanScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"') {
			value = value[1 : len(value)-1]
		}
	}
	return strings.TrimSpace(value)
}

func inlineList(value string) []string {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '[' || value[len(value)-1] != ']' {
		return nil
	}
	items := strings.Split(value[1:len(value)-1], ",")
	output := make([]string, 0, len(items))
	for _, item := range items {
		if scalar := cleanScalar(item); scalar != "" {
			output = append(output, scalar)
		}
	}
	return output
}

func itemName(value string) string {
	name, _, hasKey := splitKeyValue(value)
	if hasKey {
		return name
	}
	return cleanScalar(value)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func uniqueSorted(values []string) []string {
	seen := map[string]struct{}{}
	output := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		output = append(output, value)
	}
	sort.Strings(output)
	return output
}

func resolveConfigRelative(configRelative, configured string) (string, error) {
	clean, err := cleanRelative(configured)
	if err != nil {
		return "", err
	}
	base := filepath.ToSlash(filepath.Dir(filepath.FromSlash(configRelative)))
	if base == "." {
		return clean, nil
	}
	return cleanRelative(filepath.ToSlash(filepath.Join(base, filepath.FromSlash(clean))))
}
