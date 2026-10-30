package behaviorfalsify

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// AuthenticatedDisposable labels a caller-owned app whose disposable account logs in through a
// pinned Playwright storage-state file (BBF-V0-217, NEA-V0-012).
const AuthenticatedDisposable = "authenticated-disposable"

// StorageStateLimit bounds one pinned storage-state file.
const StorageStateLimit = 1 << 20

// authenticationMaterialMin is the shortest storage-state value searched for in published bytes;
// shorter values collide with ordinary report text.
const authenticationMaterialMin = 8

var (
	ErrStorageStateInvalid = errors.New("authentication-input-invalid")
	ErrStorageStateDrift   = errors.New("authentication-input-drift")
)

// StorageStateFile pins one storage-state file by path and digest; its contents are never copied.
type StorageStateFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// DeclaredAuthentication is the opt-in authenticated-disposable record (BBF-V0-217).
type DeclaredAuthentication struct {
	Profile      string             `json:"profile"`
	StorageState []StorageStateFile `json:"storage_state"`
}

var credentialSegments = map[string]bool{
	"TOKEN": true, "SECRET": true, "PASSWORD": true, "PASSWD": true, "KEY": true, "APIKEY": true,
	"AUTH": true, "COOKIE": true, "SESSION": true, "CREDENTIAL": true, "CREDENTIALS": true,
	"PRIVATE": true, "BEARER": true, "JWT": true,
}

// CredentialKey reports an environment key with a credential-named `_` segment (NEA-V0-013).
func CredentialKey(key string) bool {
	for _, segment := range strings.Split(key, "_") {
		if credentialSegments[segment] {
			return true
		}
	}
	return false
}

// ReadStorageState returns a pinned storage-state file's bytes. The path must be clean, absolute,
// free of symlink components and outside every protected root; the content must be a bounded JSON
// object with keys among `cookies` and `origins`. Bytes that differ from the pin are drift.
func ReadStorageState(path, sha string, protected []string) ([]byte, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || !digestPattern.MatchString(sha) {
		return nil, ErrStorageStateInvalid
	}
	if resolved, err := filepath.EvalSymlinks(path); err != nil || resolved != path {
		return nil, ErrStorageStateInvalid
	}
	for _, root := range protected {
		if root != "" && within(path, root) {
			return nil, ErrStorageStateInvalid
		}
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > StorageStateLimit {
		return nil, ErrStorageStateInvalid
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) > StorageStateLimit {
		return nil, ErrStorageStateInvalid
	}
	var shape map[string]json.RawMessage
	if json.Unmarshal(data, &shape) != nil || shape == nil {
		return nil, ErrStorageStateInvalid
	}
	for key := range shape {
		if key != "cookies" && key != "origins" {
			return nil, ErrStorageStateInvalid
		}
	}
	if digestBytes(data) != sha {
		return nil, ErrStorageStateDrift
	}
	return data, nil
}

// AuthenticationMaterial returns the storage-state values that must never be published: every
// string of at least eight bytes except the cookie and origin addressing fields.
func AuthenticationMaterial(files [][]byte) []string {
	var out []string
	var walk func(key string, value any)
	walk = func(key string, value any) {
		switch v := value.(type) {
		case map[string]any:
			for k, child := range v {
				walk(k, child)
			}
		case []any:
			for _, child := range v {
				walk(key, child)
			}
		case string:
			switch key {
			case "name", "domain", "path", "sameSite", "origin":
				return
			}
			if len(v) >= authenticationMaterialMin && !slices.Contains(out, v) {
				out = append(out, v)
			}
		}
	}
	for _, data := range files {
		var doc any
		if json.Unmarshal(data, &doc) == nil {
			walk("", doc)
		}
	}
	return out
}

// ContainsAuthenticationMaterial reports whether serialized bytes carry any material value, raw
// or JSON-escaped.
func ContainsAuthenticationMaterial(serialized []byte, material []string) bool {
	for _, value := range material {
		if bytes.Contains(serialized, []byte(value)) {
			return true
		}
		if escaped, err := json.Marshal(value); err == nil && bytes.Contains(serialized, escaped[1:len(escaped)-1]) {
			return true
		}
	}
	return false
}

func validateAuthentication(request Request) error {
	auth := request.Authentication
	if auth == nil {
		return nil
	}
	if request.Isolation == nil {
		return errors.New("authentication-requires-isolation")
	}
	if auth.Profile != AuthenticatedDisposable || len(auth.StorageState) < 1 || len(auth.StorageState) > 4 {
		return ErrStorageStateInvalid
	}
	if _, err := storageStateBytes(request); err != nil {
		return err
	}
	for _, key := range request.DeclaredEnvKeys {
		if CredentialKey(key) || secretscreen.MatchString(key+"="+os.Getenv(key)) {
			return errors.New("authentication-environment-secret-shaped")
		}
	}
	return nil
}

// storageStateBytes rereads every pinned storage-state file of the request.
func storageStateBytes(request Request) ([][]byte, error) {
	protected := append(repositoryRootList(request.Repositories), request.DisposableRoot)
	seen := map[string]bool{}
	var out [][]byte
	for _, file := range request.Authentication.StorageState {
		if seen[file.Path] {
			return nil, ErrStorageStateInvalid
		}
		seen[file.Path] = true
		data, err := ReadStorageState(file.Path, file.SHA256, protected)
		if err != nil {
			return nil, err
		}
		out = append(out, data)
	}
	return out, nil
}

// refuseAuthenticationMaterial rechecks the pins and refuses a document that carries any
// storage-state value (BBF-V0-218).
func refuseAuthenticationMaterial(plan Plan, document any) error {
	if plan.Request.Authentication == nil {
		return nil
	}
	files, err := storageStateBytes(plan.Request)
	if err != nil {
		return ErrStorageStateDrift
	}
	material := AuthenticationMaterial(files)
	serialized, err := json.Marshal(document)
	if err != nil || ContainsAuthenticationMaterial(serialized, material) {
		return errors.New("authentication-material-in-report")
	}
	// JSON encodes retained byte payloads as base64, so each is searched in its raw form too.
	if receipt, ok := document.(EvidenceReceipt); ok {
		payloads := [][]byte{receipt.PlanPreimage}
		for _, raw := range receipt.RawAttempts {
			payloads = append(payloads, raw.HookBytes, raw.NativeBytes, raw.IsolationResetBytes, raw.IsolationVerifyBytes)
		}
		for _, payload := range payloads {
			if ContainsAuthenticationMaterial(payload, material) {
				return errors.New("authentication-material-in-report")
			}
		}
	}
	return nil
}
