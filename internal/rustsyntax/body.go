// SPDX-License-Identifier: Apache-2.0

package rustsyntax

// Range is a one-based line/column span of an executable body. It mirrors the
// test-evidence region shape field for field, so the runner registry converts
// it without this analyzer importing anything outside its pure closure.
type Range struct {
	Start       int32
	StartColumn int32
	End         int32
	EndColumn   int32
}

// Body is one executable function-body interior with its span.
type Body struct {
	ID     string
	Parent string
	Range  Range
}

// Unsupported is a bounded refusal reason such as
// "rust-regions:signature-unsupported".
type Unsupported string

func (u Unsupported) Error() string { return string(u) }

// decimal formats a non-negative offset without strconv or fmt.
func decimal(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
