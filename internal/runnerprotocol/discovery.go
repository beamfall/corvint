// SPDX-License-Identifier: Apache-2.0
package runnerprotocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

const MaxDiscoveryBytes = 1 << 20
const MaxExecutableBytes = 256 << 20

var pluginName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Pin struct {
	Name       string   `json:"name"`
	Protocol   string   `json:"protocol"`
	Executable string   `json:"executable"`
	SHA256     string   `json:"sha256"`
	Runners    []string `json:"runners"`
}
type Discovery struct {
	Schema  string `json:"schema"`
	Plugins []Pin  `json:"plugins"`
}

func sortStrings(s []string) { sort.Strings(s) }
func discoveryShape() *Schema {
	return obj(map[string]*Schema{"schema": str(), "plugins": arr(obj(map[string]*Schema{"name": str(), "protocol": str(), "executable": str(), "sha256": str(), "runners": arr(str())}))})
}
func DecodeDiscovery(data []byte) (Discovery, error) {
	var d Discovery
	if len(data) > MaxDiscoveryBytes {
		return d, fmt.Errorf("discovery-cap-exceeded")
	}
	if err := decode(data, discoveryShape(), &d); err != nil {
		return Discovery{}, err
	}
	names, runners := map[string]bool{}, map[string]bool{}
	for _, p := range d.Plugins {
		if !pluginName.MatchString(p.Name) || names[p.Name] || p.Protocol == "" || !digestPattern.MatchString(p.SHA256) || !executableLabel(p.Executable) || len(p.Runners) == 0 {
			return Discovery{}, fmt.Errorf("discovery-invalid-plugin")
		}
		names[p.Name] = true
		for _, r := range p.Runners {
			if r == "" || runners[r] {
				return Discovery{}, fmt.Errorf("discovery-duplicate-or-empty-runner")
			}
			if _, compiled := registry.Lookup(r); compiled || r == "script" || r == "make" {
				return Discovery{}, fmt.Errorf("compiled-in-runner-claim:%s", r)
			}
			runners[r] = true
		}
	}
	// Keep unknown versions and their runner claims: resolution produces FULL.
	if d.Schema == "" {
		return Discovery{}, fmt.Errorf("discovery-schema-missing")
	}
	sort.Slice(d.Plugins, func(i, j int) bool { return d.Plugins[i].Name < d.Plugins[j].Name })
	return d, nil
}
func (d Discovery) Canonical() ([]byte, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	d, err = DecodeDiscovery(b)
	if err != nil {
		return nil, err
	}
	return json.Marshal(d)
}
func ReadDiscovery(path string) (Discovery, error) {
	f, err := openRegular(path)
	if err != nil {
		return Discovery{}, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, MaxDiscoveryBytes+1))
	if err != nil {
		return Discovery{}, err
	}
	return DecodeDiscovery(b)
}

// DiscoveryPlan retains the union of claims on candidate pin changes. A malformed
// file produces FullAll, never a partly decoded inventory that could narrow.
type DiscoveryPlan struct {
	Pins    []Pin
	Full    map[string]string
	FullAll string
}

func PlanDiscovery(trusted, candidate []byte, ci bool) DiscoveryPlan {
	d, err := DecodeDiscovery(trusted)
	p := DiscoveryPlan{Full: map[string]string{}}
	if err != nil {
		p.FullAll = "discovery-invalid:" + err.Error()
		return p
	}
	p.Pins = d.Plugins
	if d.Schema != DiscoverySchema {
		for _, pin := range d.Plugins {
			for _, r := range pin.Runners {
				p.Full[r] = "unsupported-schema"
			}
		}
	}
	if ci && !bytes.Equal(trusted, candidate) {
		other, e := DecodeDiscovery(candidate)
		if e != nil {
			p.FullAll = "candidate-pins-invalid"
			return p
		}
		for _, pins := range [][]Pin{d.Plugins, other.Plugins} {
			for _, pin := range pins {
				for _, r := range pin.Runners {
					// An earlier cause, such as unsupported-schema, keeps
					// precedence over the candidate edit (RSP-V0-063).
					if p.Full[r] == "" {
						p.Full[r] = "candidate-pins-changed"
					}
				}
			}
		}
	}
	return p
}

// Resolved is intentionally opaque; only declared store or release entries can
// produce one. Repository executable fields are source labels, never launch paths.
type Resolved struct {
	pin  Pin
	path string
}

func (r Resolved) Pin() Pin { p := r.pin; p.Runners = append([]string{}, p.Runners...); return p }
func Resolve(pin Pin, store, release Discovery) (Resolved, error) {
	b, _ := json.Marshal(Discovery{Schema: DiscoverySchema, Plugins: []Pin{pin}})
	if _, err := DecodeDiscovery(b); err != nil {
		return Resolved{}, err
	}
	if pin.Protocol != Protocol {
		return Resolved{}, fmt.Errorf("unsupported-protocol")
	}
	for _, source := range []Discovery{store, release} {
		if source.Schema == "" && len(source.Plugins) == 0 {
			continue
		}
		canonical, err := source.Canonical()
		if err != nil {
			return Resolved{}, fmt.Errorf("store-invalid:%w", err)
		}
		source, err = DecodeDiscovery(canonical)
		if err != nil {
			return Resolved{}, err
		}
		if source.Schema != DiscoverySchema {
			return Resolved{}, fmt.Errorf("unsupported-schema")
		}
		for _, installed := range source.Plugins {
			if installed.Name != pin.Name || installed.SHA256 != pin.SHA256 {
				continue
			}
			if installed.Protocol != pin.Protocol || !sameStrings(installed.Runners, pin.Runners) {
				return Resolved{}, fmt.Errorf("store-pin-mismatch")
			}
			if !filepath.IsAbs(installed.Executable) || outsideRepository(installed.Executable) != nil {
				return Resolved{}, fmt.Errorf("repository-executable-refused")
			}
			f, err := openRegular(installed.Executable)
			if err != nil {
				return Resolved{}, fmt.Errorf("missing-executable:%w", err)
			}
			err = checkDigest(f, pin.SHA256)
			f.Close()
			if err != nil {
				return Resolved{}, err
			}
			return Resolved{pin: pin, path: installed.Executable}, nil
		}
	}
	return Resolved{}, fmt.Errorf("missing-executable")
}
func sameStrings(a, b []string) bool {
	a = append([]string{}, a...)
	b = append([]string{}, b...)
	sortStrings(a)
	sortStrings(b)
	return reflectStrings(a, b)
}
func reflectStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func checkDigest(f *os.File, want string) error {
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return fmt.Errorf("launch-failure:non-executable")
	}
	if info.Size() > MaxExecutableBytes {
		return fmt.Errorf("executable-cap-exceeded")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, MaxExecutableBytes+1))
	if err != nil || n > MaxExecutableBytes {
		return fmt.Errorf("executable-cap-exceeded")
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		return fmt.Errorf("digest-mismatch")
	}
	_, err = f.Seek(0, 0)
	return err
}
func outsideRepository(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for p := path; ; p = filepath.Dir(p) {
		// Bare repositories have no .git marker but are still repositories.
		head, headErr := os.Lstat(filepath.Join(p, "HEAD"))
		objects, objectsErr := os.Lstat(filepath.Join(p, "objects"))
		refs, refsErr := os.Lstat(filepath.Join(p, "refs"))
		if headErr == nil && head.Mode().IsRegular() && objectsErr == nil && objects.IsDir() && refsErr == nil && refs.IsDir() {
			return fmt.Errorf("plugin-store-inside-repository")
		}
		if _, err := os.Lstat(filepath.Join(p, ".git")); err == nil {
			return fmt.Errorf("plugin-store-inside-repository")
		}
		info, err := os.Lstat(p)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink-refused")
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return nil
}
func DefaultStore() (string, error) {
	root, err := os.UserConfigDir()
	return filepath.Join(root, "corvint", "runner-plugins"), err
}

// Install is operator-only. It copies verified bytes to a digest-keyed private
// store, then atomically publishes the same discovery shape. It never edits pins.
func Install(store string, pin Pin, source string) (Pin, error) {
	if err := outsideRepository(store); err != nil {
		return Pin{}, err
	}
	store, err := filepath.Abs(store)
	if err != nil {
		return Pin{}, err
	}
	if err := os.MkdirAll(store, 0700); err != nil {
		return Pin{}, err
	}
	unlock, err := lockStore(store)
	if err != nil {
		return Pin{}, err
	}
	defer unlock()
	configPath := filepath.Join(store, "runner-plugins.json")
	config, err := ReadDiscovery(configPath)
	if os.IsNotExist(err) {
		config = Discovery{Schema: DiscoverySchema, Plugins: []Pin{}}
	} else if err != nil {
		return Pin{}, err
	}
	if config.Schema != DiscoverySchema {
		return Pin{}, fmt.Errorf("unsupported-schema")
	}
	if pin.Protocol != Protocol {
		return Pin{}, fmt.Errorf("unsupported-protocol")
	}
	// Validate before constructing any path from the pin.
	b, _ := json.Marshal(Discovery{Schema: DiscoverySchema, Plugins: []Pin{pin}})
	if _, err := DecodeDiscovery(b); err != nil {
		return Pin{}, err
	}
	f, err := openRegular(source)
	if err != nil {
		return Pin{}, err
	}
	defer f.Close()
	if err := checkDigest(f, pin.SHA256); err != nil {
		return Pin{}, err
	}
	directory := filepath.Join(store, pin.SHA256)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return Pin{}, err
	}
	if err := outsideRepository(directory); err != nil {
		return Pin{}, err
	}
	dest := filepath.Join(directory, "corvint-runner-"+pin.Name)
	pin.Executable = dest
	next := Discovery{Schema: DiscoverySchema, Plugins: []Pin{}}
	for _, old := range config.Plugins {
		if old.Name != pin.Name {
			next.Plugins = append(next.Plugins, old)
		}
	}
	next.Plugins = append(next.Plugins, pin)
	data, err := next.Canonical()
	if err != nil {
		return Pin{}, err
	}
	if existing, e := openRegular(dest); e == nil {
		e = checkDigest(existing, pin.SHA256)
		existing.Close()
		if e != nil {
			return Pin{}, e
		}
	} else if os.IsNotExist(e) {
		temp, e := os.CreateTemp(directory, ".install-")
		if e != nil {
			return Pin{}, e
		}
		defer os.Remove(temp.Name())
		_, e = io.Copy(temp, io.LimitReader(f, MaxExecutableBytes+1))
		if e == nil {
			e = temp.Chmod(0500)
		}
		if e == nil {
			_, e = temp.Seek(0, 0)
		}
		if e == nil {
			e = checkDigest(temp, pin.SHA256)
		}
		if e == nil {
			e = temp.Sync()
		}
		temp.Close()
		if e != nil {
			return Pin{}, e
		}
		if e = os.Rename(temp.Name(), dest); e != nil {
			return Pin{}, e
		}
	} else {
		return Pin{}, e
	}
	temp, err := os.CreateTemp(store, ".pins-")
	if err != nil {
		return Pin{}, err
	}
	defer os.Remove(temp.Name())
	if _, err = temp.Write(append(data, '\n')); err == nil {
		err = temp.Sync()
	}
	temp.Close()
	if err != nil {
		return Pin{}, err
	}
	if err = os.Rename(temp.Name(), configPath); err != nil {
		return Pin{}, err
	}
	return pin, nil
}

func executableLabel(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, "\\:\x00") && (filepath.IsAbs(s) || !hasParent(filepath.Clean(s)))
}
