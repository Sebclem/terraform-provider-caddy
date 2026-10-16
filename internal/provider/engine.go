package provider

import (
	"context"
	"errors"
	"fmt"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"reflect"
	"strconv"
	"time"
)

type managed struct {
	kind, id, name, parent string
	fields                 map[string]any
	position               caddy.Position
}

func collection(kind string) string {
	if kind == "proxy" {
		return "handle"
	}
	return "routes"
}
func lookup(s caddy.Snapshot, m managed) (caddy.Object, error) {
	o, e := caddy.Resolve(s, m.id)
	if e != nil {
		return o, e
	}
	if m.kind == "server" {
		if len(o.Path) != 5 || !reflect.DeepEqual(o.Path[:4], caddy.Path{"config", "apps", "http", "servers"}) {
			return o, errors.New("ID is not an HTTP server")
		}
		if m.name != "" && o.Path[4] != m.name {
			return o, errors.New("server ID has a different name")
		}
		return o, nil
	}
	if len(o.Path) < 3 || o.Path[len(o.Path)-2] != collection(m.kind) {
		return o, errors.New("ID has the wrong object type or collection")
	}
	if m.kind == "proxy" && o.Value["handler"] != "reverse_proxy" {
		return o, errors.New("ID is not a reverse_proxy handler")
	}
	parentPath := o.Path[:len(o.Path)-2]
	v, e := caddy.At(s.Value, parentPath[len(s.Path):])
	if e != nil {
		return o, e
	}
	parent, ok := v.(map[string]any)
	if !ok {
		return o, errors.New("parent is not an object")
	}
	pid, _ := parent["@id"].(string)
	if pid == "" {
		return o, errors.New("parent must already have a Caddy @id")
	}
	if m.parent != "" && pid != m.parent {
		return o, errors.New("ID belongs to a different parent")
	}
	if m.kind == "route" && len(parentPath) != 5 {
		return o, errors.New("first-release route parent must be an HTTP server")
	}
	if m.kind == "proxy" && (len(parentPath) < 2 || parentPath[len(parentPath)-2] != "routes") {
		return o, errors.New("proxy parent must be a route")
	}
	return o, nil
}
func objectBody(m managed) map[string]any {
	body := caddy.Clone(m.fields).(map[string]any)
	body["@id"] = m.id
	if m.kind == "server" {
		body["automatic_https"] = map[string]any{"disable": true}
	}
	if m.kind == "proxy" {
		body["handler"] = "reverse_proxy"
	}
	return body
}
func createObject(ctx context.Context, c *caddy.Client, m managed) error {
	if m.id == "" {
		return errors.New("missing object identity")
	}
	want := objectBody(m)
	return c.Mutate(ctx, caddy.Path{"config"}, func(s caddy.Snapshot) (caddy.Decision, error) {
		found, e := caddy.Resolve(s, m.id)
		if e == nil {
			if s.AfterWrite {
				if _, e = lookup(s, m); e != nil {
					return caddy.Decision{}, e
				}
				for k, v := range want {
					if !reflect.DeepEqual(v, found.Value[k]) {
						return caddy.Decision{}, errors.New("created object readback differs; retain ID for recovery")
					}
				}
				return caddy.Decision{Done: true}, nil
			}
			return caddy.Decision{}, errors.New("Caddy ID already exists; import it explicitly")
		}
		if !errors.Is(e, caddy.ErrNotFound) {
			return caddy.Decision{}, e
		}
		if m.kind == "server" {
			dest := caddy.Path{"apps", "http", "servers", m.name}
			v := s.Value
			for i, key := range dest {
				obj, ok := v.(map[string]any)
				if !ok {
					return caddy.Decision{}, errors.New("server map ancestor is not an object")
				}
				next, exists := obj[key]
				if !exists {
					var payload any = want
					for j := len(dest) - 1; j > i; j-- {
						payload = map[string]any{dest[j]: payload}
					}
					return caddy.Decision{Method: "POST", Path: append(caddy.Path{"config"}, dest[:i+1]...), Body: payload}, nil
				}
				v = next
			}
			return caddy.Decision{}, errors.New("server name already exists; refusing overwrite")
		}
		parent, e := caddy.Resolve(s, m.parent)
		if e != nil {
			return caddy.Decision{}, fmt.Errorf("parent: %w", e)
		}
		pkind := "server"
		if m.kind == "proxy" {
			pkind = "route"
		}
		if _, e = lookup(s, managed{kind: pkind, id: m.parent}); e != nil {
			return caddy.Decision{}, e
		}
		coll := collection(m.kind)
		raw, exists := parent.Value[coll]
		a, ok := raw.([]any)
		if exists && !ok {
			return caddy.Decision{}, errors.New("parent collection is not an array")
		}
		p := caddy.Path{"id", m.parent, coll}
		if m.position.BeforeID != "" || m.position.AfterID != "" {
			a = append(append([]any(nil), a...), want)
			out, _, e := caddy.Reorder(a, m.id, m.position)
			if e != nil {
				return caddy.Decision{}, e
			}
			method := "PATCH"
			if !exists {
				method = "POST"
			}
			return caddy.Decision{Method: method, Path: p, Body: out}, nil
		}
		if exists {
			return caddy.Decision{Method: "POST", Path: p, Body: want}, nil
		}
		return caddy.Decision{Method: "POST", Path: p, Body: []any{want}}, nil
	})
}

type change struct {
	p      caddy.Path
	value  any
	remove bool
}

func updateObject(ctx context.Context, c *caddy.Client, m managed, owned Ownership) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	e := c.Mutate(ctx, caddy.Path{"config"}, func(s caddy.Snapshot) (caddy.Decision, error) {
		o, e := lookup(s, m)
		if e != nil {
			return caddy.Decision{}, e
		}
		desired := caddy.Clone(m.fields).(map[string]any)
		if raw, ok := desired["upstreams"].([]any); ok {
			live, _ := o.Value["upstreams"].([]any)
			merged := make([]any, len(raw))
			for i, v := range raw {
				obj := v.(map[string]any)
				var base map[string]any
				if len(raw) == len(live) {
					base, _ = caddy.Clone(live[i]).(map[string]any)
				} else {
					count := 0
					for _, old := range live {
						oldmap, ok := old.(map[string]any)
						if ok && oldmap["dial"] == obj["dial"] {
							base, _ = caddy.Clone(oldmap).(map[string]any)
							count++
						}
					}
					if count != 1 {
						base = nil
					}
				}
				if base == nil {
					base = map[string]any{}
				}
				for k, x := range obj {
					base[k] = x
				}
				merged[i] = base
			}
			desired["upstreams"] = merged
		}
		intent, _, _ := PlanFields(desired, o.Value, owned)
		var edits []change
		for k, v := range intent.Set {
			old, exists := o.Value[k]
			if k == "upstreams" && exists {
				a, aok := old.([]any)
				b, bok := v.([]any)
				if aok && bok && len(a) == len(b) {
					for i, x := range b {
						oldmap, _ := a[i].(map[string]any)
						newmap, _ := x.(map[string]any)
						for field, value := range newmap {
							if !reflect.DeepEqual(value, oldmap[field]) {
								edits = append(edits, change{p: caddy.Path{k, strconv.Itoa(i), field}, value: value})
							}
						}
					}
					continue
				}
			}
			edits = append(edits, change{p: caddy.Path{k}, value: v})
		}
		for _, k := range intent.Remove {
			edits = append(edits, change{p: caddy.Path{k}, remove: true})
		}
		if len(edits) == 0 {
			return caddy.Decision{Done: true}, nil
		}
		if len(edits) == 1 {
			ch := edits[0]
			method := "PATCH"
			if ch.remove {
				method = "DELETE"
			} else if _, e := caddy.At(o.Value, ch.p); errors.Is(e, caddy.ErrNotFound) {
				method = "POST"
			}
			return caddy.Decision{Method: method, Path: append(caddy.Path{"id", m.id}, ch.p...), Body: ch.value}, nil
		}
		body := caddy.Clone(o.Value).(map[string]any)
		for k, v := range intent.Set {
			body[k] = v
		}
		for _, k := range intent.Remove {
			delete(body, k)
		}
		return caddy.Decision{Method: "PATCH", Path: caddy.Path{"id", m.id}, Body: body}, nil
	})
	if e != nil {
		return e
	}
	if m.kind == "server" {
		return nil
	}
	return c.Mutate(ctx, caddy.Path{"config"}, func(s caddy.Snapshot) (caddy.Decision, error) {
		o, e := lookup(s, m)
		if e != nil {
			return caddy.Decision{}, e
		}
		v, e := caddy.At(s.Value, o.ParentPath[len(s.Path):])
		if e != nil {
			return caddy.Decision{}, e
		}
		a, ok := v.([]any)
		if !ok {
			return caddy.Decision{}, errors.New("parent is not an ordered collection")
		}
		out, moved, e := caddy.Reorder(a, m.id, m.position)
		if e != nil {
			return caddy.Decision{}, e
		}
		if !moved {
			return caddy.Decision{Done: true}, nil
		}
		return caddy.Decision{Method: "PATCH", Path: caddy.Path{"id", m.parent, collection(m.kind)}, Body: out}, nil
	})
}
func deleteObject(ctx context.Context, c *caddy.Client, m managed) error {
	return c.Mutate(ctx, caddy.Path{"config"}, func(s caddy.Snapshot) (caddy.Decision, error) {
		_, e := lookup(s, m)
		if errors.Is(e, caddy.ErrNotFound) {
			return caddy.Decision{Done: true}, nil
		}
		if e != nil {
			return caddy.Decision{}, e
		}
		return caddy.Decision{Method: "DELETE", Path: caddy.Path{"id", m.id}}, nil
	})
}
