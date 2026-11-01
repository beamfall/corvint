package p_test

import (
	"testing"

	q "example.test/p"
)

func TestExportedSetterParallel(t *testing.T) {
	t.Parallel()
	restore := q.SetPublishFaultForTest(nil)
	defer restore()
}

func TestExportedSetterViaHelper(t *testing.T) {
	install(t)
}

func install(t *testing.T) { t.Cleanup(q.SetPublishFaultForTest(nil)) }

func TestKeyedSeamParallel(t *testing.T) {
	t.Parallel()
	defer q.SetKeyedFaultForTest(t.Name(), nil)()
}
