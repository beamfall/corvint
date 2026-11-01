package p

// SetPublishFaultForTest is a setter declared only in export_test.go.
func SetPublishFaultForTest(f func() error) func() {
	prev := publishFault
	publishFault = f
	return func() { publishFault = prev }
}

// SetKeyedFaultForTest uses a keyed seam: a method call, not a write.
func SetKeyedFaultForTest(key string, f func() error) func() {
	keyed.Store(key, f)
	return func() { keyed.Delete(key) }
}

func bump() { count++ }
