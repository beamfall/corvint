// SPDX-License-Identifier: Apache-2.0
package lspstdio

import (
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Named refusals (LWB-V0-006..008). Each is a fixed reason token, never a
// backend path, argument or message.
var (
	ErrUnsupportedSuffix   = errors.New("UNSUPPORTED_SUFFIX")
	ErrServerBinaryMissing = errors.New("SERVER_BINARY_MISSING")
	ErrBackendConfig       = errors.New("backend configuration invalid")
)

const (
	reasonUnsupportedSuffix  = "UNSUPPORTED_SUFFIX"
	reasonBinaryMissing      = "SERVER_BINARY_MISSING"
	reasonServerCrashed      = "SERVER_CRASHED"
	reasonBackendUnavailable = "BACKEND_UNAVAILABLE"
	wrappedObservationClass  = "wrapped-server-observation"
)

// BackendConfig is one operator-named wrapped language server (LWB-V0-001).
// Family is "typescript" or "ruby"; Argv[0] is absolute; Suffixes is a
// nonempty subset of that family's closed suffix table.
type BackendConfig struct {
	Family   string
	Argv     []string
	Suffixes []string
}

// wrappedLanguages is the closed suffix table (LWB-V0-002): each admitted
// suffix names its family and the one LSP languageId its documents carry.
var wrappedLanguages = map[string]struct{ family, language string }{
	".ts": {"typescript", "typescript"}, ".mts": {"typescript", "typescript"}, ".cts": {"typescript", "typescript"},
	".tsx": {"typescript", "typescriptreact"},
	".js":  {"typescript", "javascript"}, ".mjs": {"typescript", "javascript"}, ".cjs": {"typescript", "javascript"},
	".jsx": {"typescript", "javascriptreact"},
	".rb":  {"ruby", "ruby"}, ".rake": {"ruby", "ruby"},
}

func familySuffixes(family string) []string {
	var out []string
	for suffix, v := range wrappedLanguages {
		if v.family == family {
			out = append(out, suffix)
		}
	}
	slices.Sort(out)
	return out
}

// TypeScriptBackend is the --tsserver alias: every TS/JS suffix, served by
// `BINARY --stdio` (typescript-language-server's stdio form).
func TypeScriptBackend(binary string) BackendConfig {
	return BackendConfig{Family: "typescript", Argv: []string{binary, "--stdio"}, Suffixes: familySuffixes("typescript")}
}

// RubyBackend is the --ruby-lsp alias: every Ruby suffix, served by `BINARY`.
func RubyBackend(binary string) BackendConfig {
	return BackendConfig{Family: "ruby", Argv: []string{binary}, Suffixes: familySuffixes("ruby")}
}

// ParseServer parses the generic `SUFFIXES=ABSOLUTE_ARGV` form: comma-separated
// suffixes of one family, then a whitespace-separated argv whose first word is
// absolute. Arguments therefore cannot contain whitespace.
func ParseServer(spec string) (BackendConfig, error) {
	list, command, ok := strings.Cut(spec, "=")
	argv := strings.Fields(command)
	if !ok || list == "" || len(argv) == 0 || len(argv) > 32 || len(spec) > 4096 {
		return BackendConfig{}, ErrBackendConfig
	}
	b := BackendConfig{Argv: argv}
	for _, suffix := range strings.Split(list, ",") {
		v, known := wrappedLanguages[suffix]
		if !known {
			return BackendConfig{}, ErrUnsupportedSuffix
		}
		if b.Family != "" && b.Family != v.family {
			return BackendConfig{}, ErrBackendConfig
		}
		if slices.Contains(b.Suffixes, suffix) {
			return BackendConfig{}, ErrBackendConfig
		}
		b.Family = v.family
		b.Suffixes = append(b.Suffixes, suffix)
	}
	return b, nil
}

// CheckBackends validates a whole configuration before serving: absolute
// existing argv[0], at most one process per family (and one gopls) and no
// suffix claimed twice. A missing executable is the named SERVER_BINARY_MISSING.
func CheckBackends(gopls string, backends []BackendConfig) error {
	families := map[string]bool{}
	claimed := map[string]bool{}
	executables := []string{}
	if gopls != "" {
		executables = append(executables, gopls)
	}
	for _, b := range backends {
		if b.Family != "typescript" && b.Family != "ruby" || families[b.Family] || len(b.Argv) == 0 || len(b.Suffixes) == 0 {
			return ErrBackendConfig
		}
		families[b.Family] = true
		for _, suffix := range b.Suffixes {
			if v, ok := wrappedLanguages[suffix]; !ok || v.family != b.Family {
				return ErrUnsupportedSuffix
			}
			if claimed[suffix] {
				return ErrBackendConfig
			}
			claimed[suffix] = true
		}
		executables = append(executables, b.Argv[0])
	}
	for _, executable := range executables {
		if !filepath.IsAbs(executable) {
			return ErrBackendConfig
		}
		if _, err := os.Stat(executable); errors.Is(err, fs.ErrNotExist) {
			return ErrServerBinaryMissing
		}
	}
	return nil
}

// uriSuffix is the routing key: the exact, case-sensitive extension of a
// file URI's path. Unparseable URIs have no suffix.
func uriSuffix(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return ""
	}
	return path.Ext(u.Path)
}
