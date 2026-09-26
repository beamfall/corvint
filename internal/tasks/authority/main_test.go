//go:build darwin || linux

package authority

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
)

func TestMain(m *testing.M) { fixture.Main(m) }
