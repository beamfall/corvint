package opencodequalification

import "encoding/json"

func jsonBytes(v any) ([]byte, error) { return json.Marshal(v) }
