package provider

import (
	"errors"
	"reflect"

	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func NewTLSCertificateResource() resource.Resource { return &caddyResource{kind: "certificate"} }
func NewLogResource() resource.Resource            { return &caddyResource{kind: "log"} }
func unordered(kind string) bool {
	return kind == "server" || kind == "certificate" || kind == "log" || kind == "error_pages"
}
func rootLocation(m managed) caddy.Path {
	switch m.kind {
	case "server":
		return caddy.Path{"apps", "http", "servers", m.name}
	case "certificate":
		return caddy.Path{"apps", "tls", "certificates", "load_files"}
	case "log":
		return caddy.Path{"logging", "logs", m.name}
	}
	return nil
}
func createRootDecision(s caddy.Snapshot, m managed, want map[string]any) (caddy.Decision, error) {
	dest := rootLocation(m)
	v := s.Value
	for i, key := range dest {
		obj, ok := v.(map[string]any)
		if !ok {
			return caddy.Decision{}, errors.New("configuration ancestor is not an object")
		}
		next, exists := obj[key]
		if !exists {
			var payload any = want
			if m.kind == "certificate" {
				payload = []any{want}
			}
			for j := len(dest) - 1; j > i; j-- {
				payload = map[string]any{dest[j]: payload}
			}
			return caddy.Decision{Method: "POST", Path: append(caddy.Path{"config"}, dest[:i+1]...), Body: payload}, nil
		}
		v = next
	}
	if m.kind == "certificate" {
		if _, ok := v.([]any); !ok {
			return caddy.Decision{}, errors.New("certificate loader is not an array")
		}
		return caddy.Decision{Method: "POST", Path: append(caddy.Path{"config"}, dest...), Body: want}, nil
	}
	return caddy.Decision{}, errors.New("object name already exists; import by @id explicitly")
}
func lookupRoot(s caddy.Snapshot, m managed, o caddy.Object) (caddy.Object, error) {
	if m.kind == "certificate" {
		if len(o.Path) != 6 || !reflect.DeepEqual(o.Path[:5], caddy.Path{"config", "apps", "tls", "certificates", "load_files"}) {
			return o, errors.New("ID is not a certificate-file loader")
		}
	} else if m.kind == "log" {
		if len(o.Path) != 4 || !reflect.DeepEqual(o.Path[:3], caddy.Path{"config", "logging", "logs"}) {
			return o, errors.New("ID is not a named log")
		}
		if m.name != "" && o.Path[3] != m.name {
			return o, errors.New("log ID has a different name")
		}
	}
	return o, nil
}

func lookupErrorPages(s caddy.Snapshot, m managed, o caddy.Object) (caddy.Object, error) {
	n := len(o.Path)
	if n < 4 || o.Path[n-2] != "routes" || o.Path[n-3] != "errors" {
		return o, errors.New("ID is not an error-page route")
	}
	raw, e := caddy.At(s.Value, o.Path[len(s.Path):n-3])
	if e != nil {
		return o, e
	}
	parent, ok := raw.(map[string]any)
	if !ok {
		return o, errors.New("error-page parent must be an HTTP server or subroute")
	}
	id, _ := parent["@id"].(string)
	if id == "" || (m.parent != "" && id != m.parent) {
		return o, errors.New("invalid error-page parent ID")
	}

	if _, e := errorPagesParent(s, id, m.parentKind); e != nil {
		return o, e
	}
	return o, nil
}

// Infer the parent on import, while checking the configured parent type on create.
func errorPagesParent(s caddy.Snapshot, id, expected string) (caddy.Object, error) {
	parent, err := caddy.Resolve(s, id)
	if err != nil {
		return parent, err
	}
	kind := "server"
	if parent.Value["handler"] == "subroute" {
		kind = "subroute"
	}
	if expected != "" && expected != kind {
		return parent, errors.New("error-page parent type does not match server_id or subroute_id")
	}
	return lookup(s, managed{kind: kind, id: id})
}

func createErrorPages(s caddy.Snapshot, m managed, want map[string]any) (caddy.Decision, error) {
	parent, e := errorPagesParent(s, m.parent, m.parentKind)
	if e != nil {
		return caddy.Decision{}, e
	}
	if a, e := caddy.At(parent.Value, []string{"errors", "routes"}); e == nil {
		if arr, ok := a.([]any); !ok || len(arr) != 0 {
			return caddy.Decision{}, errors.New("parent already has error routes; refusing duplicate ownership")
		}
	}
	return settingDecision(s, append(append(caddy.Path(nil), parent.Path...), "errors", "routes"), []any{want})
}
