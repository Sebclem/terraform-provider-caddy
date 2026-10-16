package caddy

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

var ErrNotFound = errors.New("Caddy object not found")

type Object struct {
	ID               string
	Path, ParentPath Path
	Value            map[string]any
}

func Resolve(s Snapshot, id string) (Object, error) {
	if id == "" {
		return Object{}, errors.New("object ID must not be empty")
	}
	ids := map[string]Object{}
	var walk func(any, Path) error
	walk = func(v any, p Path) error {
		switch x := v.(type) {
		case map[string]any:
			if raw, ok := x["@id"]; ok {
				key, ok := raw.(string)
				if !ok {
					return errors.New("unsupported non-string Caddy ID")
				}
				if _, ok = ids[key]; ok {
					return fmt.Errorf("duplicate Caddy ID %q", key)
				}
				ids[key] = Object{key, p, append(Path(nil), p[:len(p)-1]...), x}
			}
			for k, v := range x {
				if err := walk(v, append(append(Path(nil), p...), k)); err != nil {
					return err
				}
			}
		case []any:
			for i, v := range x {
				if err := walk(v, append(append(Path(nil), p...), strconv.Itoa(i))); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(s.Value, s.Path); err != nil {
		return Object{}, err
	}
	o, ok := ids[id]
	if !ok {
		return Object{}, ErrNotFound
	}
	return o, nil
}
func RequireParent(o, p Object, collection string) error {
	want := append(append(Path(nil), p.Path...), collection)
	if !reflect.DeepEqual(o.ParentPath, want) {
		return errors.New("Caddy ID belongs to a different parent or collection")
	}
	return nil
}
