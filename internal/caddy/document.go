package caddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// Clone preserves JSON numbers and all fields not owned by a typed resource.
func Clone(v Value) Value {
	b, _ := json.Marshal(v)
	var out Value
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	_ = d.Decode(&out)
	return out
}
func At(v Value, p Path) (Value, error) {
	for _, s := range p {
		switch x := v.(type) {
		case map[string]any:
			var ok bool
			v, ok = x[s]
			if !ok {
				return nil, ErrNotFound
			}
		case []any:
			i, e := strconv.Atoi(s)
			if e != nil || i < 0 || i >= len(x) {
				return nil, ErrNotFound
			}
			v = x[i]
		default:
			return nil, fmt.Errorf("invalid configuration traversal")
		}
	}
	return v, nil
}
