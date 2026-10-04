// Package cishards partitions complete Go package universes without selecting tests.
package cishards

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"sort"
	"strings"
)

const MaxPackages = 4091
const MaxShards = 64
const MaxInputBytes = 8 << 20

// Protected source bytes bind both the driver and its isolated fallback helper.
//
//go:embed partition.go cmd/main.go package-costs.json
var profileFiles embed.FS

//go:embed package-costs.json
var defaultCosts []byte

func ProfileDigest() string {
	h := sha256.New()
	h.Write([]byte("corvint-ci-partition/1\n"))
	for _, name := range []string{"partition.go", "cmd/main.go", "package-costs.json"} {
		b, err := profileFiles.ReadFile(name)
		if err != nil {
			panic(err)
		}
		h.Write([]byte(name + "\x00"))
		h.Write(b)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

var packageName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~/-]*$`)

func Packages(input []byte) ([]string, error) {
	if len(input) > MaxInputBytes {
		return nil, errors.New("package universe exceeds byte bound")
	}
	return validate(strings.Fields(string(input)))
}

func validate(packages []string) ([]string, error) {
	if len(packages) == 0 || len(packages) > MaxPackages {
		return nil, errors.New("invalid package universe size")
	}
	seen := map[string]bool{}
	out := append([]string(nil), packages...)
	for _, p := range out {
		if !packageName.MatchString(p) || strings.Contains(p, "..") || strings.HasSuffix(p, "/") || strings.Contains(p, "//") || seen[p] {
			return nil, errors.New("invalid or duplicate package argument")
		}
		seen[p] = true
	}
	sort.Strings(out)
	return out, nil
}

type estimates struct {
	Profile string `json:"profile"`
	Source  struct {
		Revision  string `json:"revision"`
		RunURL    string `json:"runURL"`
		GoVersion string `json:"goVersion"`
	} `json:"source"`
	Milliseconds map[string]int64 `json:"milliseconds"`
}

func weights(raw []byte) (map[string]int64, int64, bool) {
	if len(raw) > 1<<20 {
		return nil, 0, false
	}
	var e estimates
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&e) != nil || d.Decode(new(any)) != io.EOF || e.Profile != "corvint-ci-package-costs/0" || e.Source.GoVersion != "go1.27.1" || len(e.Source.Revision) != 40 || len(e.Milliseconds) == 0 || len(e.Milliseconds) > MaxPackages {
		return nil, 0, false
	}
	if _, err := hex.DecodeString(e.Source.Revision); err != nil {
		return nil, 0, false
	}
	if !strings.HasPrefix(e.Source.RunURL, "https://github.com/beamfall/corvint/actions/runs/") {
		return nil, 0, false
	}
	values := make([]int64, 0, len(e.Milliseconds))
	for p, n := range e.Milliseconds {
		if _, err := validate([]string{p}); err != nil || n <= 0 || n > 3000000 {
			return nil, 0, false
		}
		values = append(values, n)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return e.Milliseconds, values[len(values)/2], true
}

// Partition always assigns every supplied package exactly once. Estimates only
// affect placement; missing or invalid estimates cannot remove a package.
func Partition(packages []string, total int) ([][]string, error) {
	return partition(packages, total, defaultCosts)
}

func partition(packages []string, total int, raw []byte) ([][]string, error) {
	if total < 1 || total > MaxShards {
		return nil, errors.New("invalid shard count")
	}
	universe, err := validate(packages)
	if err != nil {
		return nil, err
	}
	shards := make([][]string, total)
	w, unknown, ok := weights(raw)
	if !ok {
		for i, p := range universe {
			shards[i%total] = append(shards[i%total], p)
		}
		return shards, nil
	}
	cost := func(p string) int64 {
		if n, ok := w[p]; ok {
			return n
		}
		return unknown
	}
	sort.Slice(universe, func(i, j int) bool {
		a, b := cost(universe[i]), cost(universe[j])
		if a == b {
			return universe[i] < universe[j]
		}
		return a > b
	})
	loads := make([]int64, total)
	for _, p := range universe {
		bin := 0
		for i := 1; i < total; i++ {
			if loads[i] < loads[bin] {
				bin = i
			}
		}
		shards[bin] = append(shards[bin], p)
		loads[bin] += cost(p)
	}
	for _, s := range shards {
		sort.Strings(s)
	}
	return shards, nil
}

// Intersect uses the complete partition for both selected and fallback jobs,
// so independent admission failures cannot move a selected package to a gap.
func Intersect(universe, selected []string, shard, total int) ([]string, error) {
	if shard < 0 || shard >= total {
		return nil, errors.New("invalid shard index")
	}
	parts, err := Partition(universe, total)
	if err != nil {
		return nil, err
	}
	if len(selected) == 1 && selected[0] == "./..." {
		return parts[shard], nil
	}
	wanted, err := validate(selected)
	if err != nil {
		return nil, err
	}
	all := map[string]bool{}
	for _, p := range universe {
		all[p] = true
	}
	set := map[string]bool{}
	for _, p := range wanted {
		if !all[p] {
			return nil, errors.New("selected package absent from runtime universe")
		}
		set[p] = true
	}
	out := []string{}
	for _, p := range parts[shard] {
		if set[p] {
			out = append(out, p)
		}
	}
	return out, nil
}

// Costs returns the admitted package estimates in raw, or false when partition
// would discard raw and fall back to lexical round-robin.
func Costs(raw []byte) (map[string]int64, bool) {
	w, _, ok := weights(raw)
	return w, ok
}
