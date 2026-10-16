package caddy

import "errors"

var ErrMissingAnchor = errors.New("ordering anchor is not an existing sibling")

type Position struct{ BeforeID, AfterID string }

func indexes(a []Value, id string, p Position) (int, int, error) {
	if p.BeforeID != "" && p.AfterID != "" {
		return 0, 0, errors.New("before_id and after_id are mutually exclusive")
	}
	anchor := p.BeforeID
	if anchor == "" {
		anchor = p.AfterID
	}
	if id == anchor {
		return 0, 0, errors.New("ordering cannot reference itself")
	}
	i, j := -1, -1
	for k, v := range a {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if m["@id"] == id {
			i = k
		}
		if m["@id"] == anchor {
			j = k
		}
	}
	if i < 0 {
		return 0, 0, ErrNotFound
	}
	if anchor != "" && j < 0 {
		return 0, 0, ErrMissingAnchor
	}
	return i, j, nil
}
func OrderSatisfied(a []Value, id string, p Position) (bool, error) {
	i, j, e := indexes(a, id, p)
	if e != nil {
		return false, e
	}
	return j < 0 || (p.BeforeID != "" && i < j) || (p.AfterID != "" && i > j), nil
}
func Reorder(a []Value, id string, p Position) ([]Value, bool, error) {
	ok, e := OrderSatisfied(a, id, p)
	if e != nil || ok {
		return a, false, e
	}
	i, _, _ := indexes(a, id, p)
	v := a[i]
	out := append([]any(nil), a[:i]...)
	out = append(out, a[i+1:]...)
	anchor := p.BeforeID
	if anchor == "" {
		anchor = p.AfterID
	}
	j := 0
	for k, x := range out {
		if m, ok := x.(map[string]any); ok && m["@id"] == anchor {
			j = k
			break
		}
	}
	if p.AfterID != "" {
		j++
	}
	out = append(out, nil)
	copy(out[j+1:], out[j:])
	out[j] = v
	return out, true, nil
}
