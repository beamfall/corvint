// SPDX-License-Identifier: Apache-2.0

// Package csharpregions registers the contextindex C# body reader into the
// test-runner registry, which cannot import contextindex (LGP-V0-020).
package csharpregions

import (
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

func init() { registry.RegisterCSharpBodies(Read) }

// Read adapts contextindex.CSharpBodies to the registry's body type.
func Read(data []byte) ([]registry.CSharpBody, bool) {
	bodies, ok := contextindex.CSharpBodies(data)
	if !ok {
		return nil, false
	}
	out := make([]registry.CSharpBody, len(bodies))
	for i, b := range bodies {
		out[i] = registry.CSharpBody{Name: b.Name, Start: b.Start, End: b.End}
	}
	return out, true
}
