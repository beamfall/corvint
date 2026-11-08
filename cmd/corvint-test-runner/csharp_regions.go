package main

// The dotnet runner reads C# body regions through contextindex, which the
// test-runner registry cannot import (LGP-V0-020).
import _ "github.com/Beamfall/corvint/internal/csharpregions"
