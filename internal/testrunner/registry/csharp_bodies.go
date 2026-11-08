// SPDX-License-Identifier: Apache-2.0

package registry

import "sync/atomic"

// CSharpBody is a whole-line interior of a C# method body, as the
// contextindex C# dialect reports it. Start is inclusive and End exclusive.
type CSharpBody struct {
	Name       string
	Start, End int
}

// CSharpBodiesReader returns a source's method bodies, or false when it
// cannot bound them conservatively.
type CSharpBodiesReader func(data []byte) ([]CSharpBody, bool)

var csharpBodiesReader atomic.Pointer[CSharpBodiesReader]

// RegisterCSharpBodies installs the C# body reader. The registry cannot import
// internal/contextindex (LGP-V0-020), so internal/csharpregions' init calls it
// from each binary that links the dotnet runner.
func RegisterCSharpBodies(reader CSharpBodiesReader) {
	csharpBodiesReader.Store(&reader)
}

// csharpBodies fails closed: with no registered reader no region is claimed,
// so evidence stays incomplete and mutation refuses.
func csharpBodies(data []byte) ([]CSharpBody, bool) {
	reader := csharpBodiesReader.Load()
	if reader == nil || *reader == nil {
		return nil, false
	}
	return (*reader)(data)
}
