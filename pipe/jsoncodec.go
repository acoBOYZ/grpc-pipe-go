package pipe

import "encoding/json"

func JSONEncode(v any) ([]byte, error)   { return json.Marshal(v) }
func JSONDecode(b []byte, out any) error { return json.Unmarshal(b, out) }
