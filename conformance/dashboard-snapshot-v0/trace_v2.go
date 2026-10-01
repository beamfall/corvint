package main

import (
	"bytes"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

func isTraceAdapter(value any) bool { return value == "local-trace-v1" || value == "local-trace-v2" }

func expectedTraceRegistryDigest(sources []any) string {
	for _, value := range sources {
		if source, ok := value.(map[string]any); ok && source["adapterId"] == "local-trace-v2" {
			return "sha256:477cf73e98c9ef9c70dcaea45ea4520e8957cac99ce9fea9886207a88df74738"
		}
	}
	return domainDigest("corvint-dashboard-adapter-registry/0", []byte(canonicalRegistry))
}

var typedCredentialFlag = regexp.MustCompile(`(?i)^--?(?:[a-z0-9]+[_-])*(?:api[_-]?key|access[_-]?key|account[_-]?key|authorization|credential|credentials|passphrase|passwd|password|private[_-]?key|secret|token|pass)$`)

// This decoder is separate from the production trace codec. The v2 JSON check
// rejects malformed Unicode before the legacy token walker could replace it.
func validateTypedTraceVerification(raw []byte, row map[string]any) error {
	var strict any
	if err := jsonv2.Unmarshal(raw, &strict); err != nil {
		return reject(rejectJSON)
	}
	task, _ := row["task"].(string)
	if typedSecretMatch(task) {
		return reject(rejectPrivacyText)
	}
	for _, name := range []string{"opened_paths", "changed_paths"} {
		for _, value := range row[name].([]any) {
			if typedSecretMatch(value.(string)) {
				return reject(rejectPrivacyText)
			}
		}
	}
	entries, err := array(row["verification"])
	if err != nil || len(entries) == 0 || len(entries) > 50 {
		return reject(rejectSize)
	}
	var previous []byte
	hasArgv := false
	for _, value := range entries {
		entry, err := object(value)
		if err != nil {
			return err
		}
		switch entry["kind"] {
		case "command":
			if err := exactFields(entry, "kind", "command"); err != nil {
				return err
			}
			command, err := stringValue(entry["command"])
			if err != nil || strings.TrimSpace(command) != command || utf8.RuneCountInString(command) > 512 || !traceSafeCommandRE.MatchString(command) || typedSecretMatch(command) {
				return reject(rejectPrivacyText)
			}
		case "argv":
			if err := exactFields(entry, "kind", "argv"); err != nil {
				return err
			}
			args, err := array(entry["argv"])
			if err != nil || len(args) < 1 || len(args) > 32 {
				return reject(rejectSize)
			}
			argv := make([]string, len(args))
			for i, value := range args {
				arg, err := stringValue(value)
				if err != nil || i == 0 && arg == "" || !utf8.ValidString(arg) || utf8.RuneCountInString(arg) > 512 {
					return reject(rejectJSON)
				}
				for _, r := range arg {
					if unicode.IsControl(r) {
						return reject(rejectPrivacyText)
					}
				}
				if typedSecretMatch(arg) {
					return reject(rejectPrivacyText)
				}
				argv[i] = arg
			}
			encoded, err := canonicalTypedTrace(args)
			if err != nil || len(encoded) > 4096 {
				return reject(rejectSize)
			}
			if typedSecretMatch(string(encoded)) || typedArgvCredential(argv) {
				return reject(rejectPrivacyText)
			}
			hasArgv = true
		default:
			return reject(rejectFieldSet)
		}
		encoded, err := canonicalTypedTrace(entry)
		if err != nil {
			return err
		}
		if previous != nil && bytes.Compare(previous, encoded) >= 0 {
			return reject(rejectOrdering)
		}
		previous = encoded
	}
	if !hasArgv {
		return reject(rejectFieldSet)
	}
	return nil
}

func typedArgvCredential(argv []string) bool {
	login, curl, user := false, strings.EqualFold(path.Base(argv[0]), "curl"), false
	for i, arg := range argv {
		lower := strings.ToLower(arg)
		if lower == "login" {
			login = true
		}
		name, value, inline := strings.Cut(lower, "=")
		if !inline && i+1 < len(argv) {
			value = argv[i+1]
		}
		if typedCredentialFlag.MatchString(name) && value != "" {
			return true
		}
		if name == "-u" || name == "--user" {
			user = true
			if curl && strings.Contains(value, ":") {
				return true
			}
		}
		if name == "-p" && (login || curl && user) && value != "" {
			return true
		}
		if lower == "bearer" && i+1 < len(argv) && typedSecretMatch("bearer "+argv[i+1]) {
			return true
		}
	}
	return false
}

// The trace profile uses ASCII JSON, while dashboard snapshots use their closed
// wire encoder. Keep those domains separate, especially for byte caps and IDs.
func canonicalTypedTrace(value any) ([]byte, error) {
	var utf8JSON bytes.Buffer
	encoder := json.NewEncoder(&utf8JSON)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	var result bytes.Buffer
	for _, r := range strings.TrimSuffix(utf8JSON.String(), "\n") {
		if r < 0x7f {
			result.WriteRune(r)
		} else if r <= 0xffff {
			fmt.Fprintf(&result, `\u%04x`, r)
		} else {
			a, b := utf16.EncodeRune(r)
			fmt.Fprintf(&result, `\u%04x\u%04x`, a, b)
		}
	}
	return result.Bytes(), nil
}

func trimTypedTask(value string) string {
	return strings.TrimFunc(value, func(r rune) bool { return unicode.IsSpace(r) || r >= 0x1c && r <= 0x1f })
}
