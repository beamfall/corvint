// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"context"
	"fmt"
	"sort"

	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// InventoryMatrix requires one complete capture for every configuration in the
// core's declared CI matrix. It never returns a successful partial matrix, and
// does not infer that an inventory on this host covers a different target.
func InventoryMatrix(ctx context.Context, expected []testrunner.InventoryKey, plugins []*Plugin) ([]testrunner.RunnerInventory, error) {
	if len(expected) == 0 || len(expected) > testrunner.MaxTests || len(plugins) != len(expected) {
		return nil, fmt.Errorf("go-inventory-matrix-incomplete")
	}
	instances := map[testrunner.InventoryKey]*Plugin{}
	for _, p := range plugins {
		if p == nil || instances[p.key] != nil {
			return nil, fmt.Errorf("go-inventory-matrix-duplicate-or-absent")
		}
		instances[p.key] = p
	}
	out := make([]testrunner.RunnerInventory, 0, len(expected))
	for _, key := range expected {
		p := instances[key]
		if p == nil {
			return nil, fmt.Errorf("go-inventory-matrix-configuration-missing")
		}
		inv, err := p.Inventory(ctx, registry.InventoryRequest{Key: key, Output: p.capture.List})
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
		delete(instances, key)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Key, out[j].Key
		if a.Runner != b.Runner {
			return a.Runner < b.Runner
		}
		if a.OS != b.OS {
			return a.OS < b.OS
		}
		if a.Arch != b.Arch {
			return a.Arch < b.Arch
		}
		return a.ConfigDigest < b.ConfigDigest
	})
	return out, nil
}
