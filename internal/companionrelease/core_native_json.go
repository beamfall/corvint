package companionrelease

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Native receipts permit finite fractional durations; CEM's number contract does
// not. Validate a structural shadow while retaining every original evidence byte.
func validateCoreNativeJSON(raw []byte) error {
	if len(raw) == 0 || len(raw) > installedStageOutputLimit || !utf8.Valid(raw) || !json.Valid(raw) {
		return fmt.Errorf("native core JSON violates syntax or UTF-8/size bounds")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	shadow := append([]byte(nil), raw...)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		number, ok := token.(json.Number)
		if !ok {
			continue
		}
		value, err := number.Float64()
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("native core JSON number is not finite binary64")
		}
		end := int(decoder.InputOffset())
		start := end - len(number.String())
		if start < 0 || !bytes.Equal(raw[start:end], []byte(number.String())) {
			return fmt.Errorf("native core JSON number offset differs")
		}
		for i := start; i < end; i++ {
			shadow[i] = ' '
		}
		shadow[start] = '0'
	}
	// This retains CEM's depth64, decoded duplicate-key, surrogate and trailing
	// data refusals. Native typed decoders retain their own numeric constraints.
	_, err := wire.Parse(shadow)
	return err
}
