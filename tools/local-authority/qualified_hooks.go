package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const directQualifiedHookCommand = "exec /Library/CorvintAuthority/versions/RELEASE_SHA256/corvint native-hook --qualified-direct-lifecycle"

const qualifiedHookCommand = "/Library/CorvintAuthority/versions/RELEASE_SHA256/corvint native-hook --qualified-lifecycle"

type qualifiedHook struct {
	Type          string `json:"type"`
	Command       string `json:"command"`
	Timeout       int    `json:"timeout"`
	StatusMessage string `json:"statusMessage"`
}
type qualifiedHookGroup struct {
	Hooks []qualifiedHook `json:"hooks"`
}
type qualifiedHooks struct {
	Description string                          `json:"description"`
	Hooks       map[string][]qualifiedHookGroup `json:"hooks"`
}
type qualifiedHooksReceipt struct {
	Profile        string `json:"profile"`
	Authority      string `json:"authority"`
	SourceRevision string `json:"sourceRevision"`
	ReleaseID      string `json:"releaseId"`
	ManifestSHA256 string `json:"manifestSHA256"`
	TemplateSHA256 string `json:"templateSHA256"`
	ConsumerSHA256 string `json:"consumerSHA256"`
	AdapterSHA256  string `json:"adapterSHA256"`
	HooksSHA256    string `json:"hooksSHA256"`
}

// This creates a caller-owned review sidecar only. It never changes the release
// payload allowlist, protected store, native configuration or legacy hook mode.
func prepareQualifiedHooks(bundle, templatePath, output string) error {
	return prepareQualifiedHooksProfile(bundle, templatePath, output, false)
}
func prepareQualifiedHooksProfile(bundle, templatePath, output string, direct bool) error {
	if os.Geteuid() == 0 {
		return errors.New("prepare qualified hooks as unprivileged builder")
	}
	for _, path := range []string{bundle, templatePath, output} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return errors.New("absolute canonical paths required")
		}
	}
	raw, err := readRegular(filepath.Join(bundle, "manifest.json"), 16<<20)
	if err != nil {
		return err
	}
	var manifest releaseManifest
	if strictDecode(raw, &manifest) != nil || !bytes.Equal(raw, mustJSONLine(manifest)) || validateRelease(manifest) != nil {
		return errors.New("invalid canonical release manifest")
	}
	hashes := map[string]string{}
	for _, file := range manifest.Files {
		if file.Path == "corvint" || file.Path == "authority-hook.json" {
			hashes[file.Path] = file.SHA256
		}
	}
	consumer, err := readRegular(filepath.Join(bundle, "corvint"), 64<<20)
	if err != nil || digest(consumer) != hashes["corvint"] {
		return errors.New("consumer bytes differ from manifest")
	}
	adapter, err := readRegular(filepath.Join(bundle, "authority-hook.json"), 128<<10)
	if err != nil || digest(adapter) != hashes["authority-hook.json"] {
		return errors.New("adapter bytes differ from manifest")
	}
	consumerPath := "/Library/CorvintAuthority/versions/" + manifest.ReleaseID + "/corvint"
	var declaration nativeAdapterDeclaration
	if strictDecode(adapter, &declaration) != nil || declaration.Profile != "corvint-native-authority-hook/0" || declaration.Consumer == nil || *declaration.Consumer != consumerPath || declaration.ConsumerSHA256 == nil || *declaration.ConsumerSHA256 != hashes["corvint"] || !bytes.Equal(adapter, mustJSONLine(declaration)) {
		return errors.New("native adapter declaration consumer pins differ")
	}
	declaration.Consumer = nil
	declaration.ConsumerSHA256 = nil
	if digest(mustJSONLine(declaration)) != manifest.AdapterTemplateSHA256 {
		return errors.New("adapter source template differs")
	}
	template, err := readRegular(templatePath, 128<<10)
	if err != nil {
		return err
	}
	command := qualifiedHookCommand
	if direct {
		command = directQualifiedHookCommand
	}
	hooks, err := renderQualifiedHooksCommand(template, manifest.ReleaseID, command)
	if err != nil {
		return err
	}
	receipt := qualifiedHooksReceipt{"corvint-qualified-hooks-preparation/1", "NONE", manifest.SourceRevision, manifest.ReleaseID, digest(raw), digest(template), hashes["corvint"], hashes["authority-hook.json"], digest(hooks)}
	if direct {
		receipt.Profile = "corvint-qualified-direct-hooks-preparation/0"
	}
	return writeQualifiedHooksSidecar(output, hooks, mustJSONLine(receipt), func() error {
		// Recheck actual input bytes before writing the success receipt.
		for path, want := range map[string][]byte{filepath.Join(bundle, "manifest.json"): raw, filepath.Join(bundle, "corvint"): consumer, filepath.Join(bundle, "authority-hook.json"): adapter, templatePath: template} {
			again, e := readRegular(path, len(want)+1)
			if e != nil || !bytes.Equal(again, want) {
				return errors.New("hook preparation input drift")
			}
		}
		return nil
	})
}
func writeQualifiedHooksSidecar(output string, hooks, receipt []byte, recheck func() error) error {
	if err := os.Mkdir(output, 0700); err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.Remove(filepath.Join(output, "preparation.json"))
			_ = os.Remove(filepath.Join(output, "hooks.json"))
			_ = os.Remove(output)
		}
	}()
	if err := exclusiveFile(filepath.Join(output, "hooks.json"), hooks, 0600); err != nil {
		return err
	}
	if err := recheck(); err != nil {
		return err
	}
	if err := exclusiveFile(filepath.Join(output, "preparation.json"), receipt, 0600); err != nil {
		return err
	}
	complete = true
	return nil
}
func renderQualifiedHooks(raw []byte, releaseID string) ([]byte, error) {
	return renderQualifiedHooksCommand(raw, releaseID, qualifiedHookCommand)
}
func renderQualifiedHooksCommand(raw []byte, releaseID, command string) ([]byte, error) {
	if _, err := decodeHex(releaseID, 32); err != nil {
		return nil, err
	}
	var template qualifiedHooks
	if !uniqueHookJSON(raw) || strictDecode(raw, &template) != nil || template.Description == "" || len(template.Hooks) != 4 {
		return nil, errors.New("invalid closed hook template")
	}
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "Stop", "SessionEnd"} {
		groups := template.Hooks[event]
		if len(groups) != 1 || len(groups[0].Hooks) != 1 {
			return nil, errors.New("exactly one command per native event required")
		}
		hook := &groups[0].Hooks[0]
		if hook.Type != "command" || hook.Command != command || hook.Timeout != 2 || hook.StatusMessage == "" {
			return nil, errors.New("invalid qualified hook route")
		}
		hook.Command = strings.Replace(command, "RELEASE_SHA256", releaseID, 1)
	}
	return mustJSONLine(template), nil
}

func uniqueHookJSON(raw []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var value func(int) bool
	value = func(depth int) bool {
		if depth > 16 {
			return false
		}
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return true
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				name, ok := key.(string)
				if err != nil || !ok || seen[name] {
					return false
				}
				seen[name] = true
				if !value(depth + 1) {
					return false
				}
			}
			close, err := decoder.Token()
			return err == nil && close == json.Delim('}')
		case '[':
			for decoder.More() {
				if !value(depth + 1) {
					return false
				}
			}
			close, err := decoder.Token()
			return err == nil && close == json.Delim(']')
		}
		return false
	}
	return value(0)
}
