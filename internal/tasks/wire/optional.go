package wire

// OptionalKeys adds only present extension keys to an otherwise closed record.
// Omission preserves earlier profile bytes; null is validated by the field reader.
func OptionalKeys(v Value, required []string, optional ...string) []string {
	out := append([]string{}, required...)
	if v.Kind == KindObject && v.Obj != nil {
		for _, key := range optional {
			if _, ok := v.Obj.Get(key); ok {
				out = append(out, key)
			}
		}
	}
	return out
}
func Has(v Value, key string) bool {
	if v.Kind != KindObject || v.Obj == nil {
		return false
	}
	_, ok := v.Obj.Get(key)
	return ok
}
