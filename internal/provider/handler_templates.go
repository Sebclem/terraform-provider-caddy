package provider

import (
	"context"
	"fmt"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"reflect"
)

func NewSubrouteResource() resource.Resource    { return &caddyResource{kind: "subroute"} }
func NewForwardAuthResource() resource.Resource { return &caddyResource{kind: "forward_auth"} }
func NewBasicAuthResource() resource.Resource   { return &caddyResource{kind: "basic_auth"} }
func NewErrorResource() resource.Resource       { return &caddyResource{kind: "error"} }

var handlerNames = map[string]string{"proxy": "reverse_proxy", "subroute": "subroute", "forward_auth": "reverse_proxy", "basic_auth": "authentication", "error": "error"}

func forwardAuthFields(a map[string]attr.Value) map[string]any {
	headers, _ := nativeValue(a["copy_headers"]).([]any)
	routes := []any{map[string]any{"handle": []any{map[string]any{"handler": "vars"}}}}
	for _, raw := range headers {
		name := raw.(string)
		placeholder := "{http.reverse_proxy.header." + name + "}"
		routes = append(routes, map[string]any{"handle": []any{map[string]any{"handler": "headers", "request": map[string]any{"delete": []any{name}}}}}, map[string]any{"match": []any{map[string]any{"not": []any{map[string]any{"vars": map[string]any{placeholder: []any{""}}}}}}, "handle": []any{map[string]any{"handler": "headers", "request": map[string]any{"set": map[string]any{name: []any{placeholder}}}}}})
	}
	return map[string]any{"upstreams": []any{map[string]any{"dial": nativeValue(a["upstream"])}}, "rewrite": map[string]any{"method": "GET", "uri": nativeValue(a["uri"])}, "headers": map[string]any{"request": map[string]any{"set": map[string]any{"X-Forwarded-Method": []any{"{http.request.method}"}, "X-Forwarded-Uri": []any{"{http.request.uri}"}}}}, "handle_response": []any{map[string]any{"match": map[string]any{"status_code": []any{2}}, "routes": routes}}}
}
func decodeForwardAuth(k string, m map[string]any) any {
	switch k {
	case "upstream":
		if a, ok := m["upstreams"].([]any); ok && len(a) == 1 {
			return a[0].(map[string]any)["dial"]
		}
	case "uri":
		if a, ok := m["rewrite"].(map[string]any); ok && a["method"] == "GET" {
			return a["uri"]
		}
	case "copy_headers":
		out := []any{}
		if a, ok := m["handle_response"].([]any); ok && len(a) == 1 {
			b, _ := a[0].(map[string]any)
			routes, _ := b["routes"].([]any)
			for _, raw := range routes {
				r, _ := raw.(map[string]any)
				h, _ := r["handle"].([]any)
				if len(h) == 1 {
					obj, _ := h[0].(map[string]any)
					req, _ := obj["request"].(map[string]any)
					if del, ok := req["delete"].([]any); ok {
						out = append(out, del...)
					}
				}
			}
		}
		return out
	}
	return nil
}
func NewErrorPagesResource() resource.Resource { return &caddyResource{kind: "error_pages"} }
func errorPagesFields(id string, root, uri any) map[string]any {
	return map[string]any{"handle": []any{
		map[string]any{"@id": id + "-root", "handler": "vars", "root": root},
		map[string]any{"@id": id + "-rewrite", "handler": "rewrite", "uri": uri},
		map[string]any{"@id": id + "-files", "handler": "file_server", "pass_thru": true},
		map[string]any{"@id": id + "-fallback", "handler": "static_response", "status_code": "{http.error.status_code}"},
	}}
}
func decodeErrorPage(k string, m map[string]any) any {
	h := m["handle"].([]any)
	if len(h) != 4 {
		panic("nonconforming error page sequence")
	}
	if h[0].(map[string]any)["handler"] != "vars" || h[1].(map[string]any)["handler"] != "rewrite" || h[2].(map[string]any)["handler"] != "file_server" || h[3].(map[string]any)["handler"] != "static_response" {
		panic("nonconforming error page sequence")
	}
	if k == "root" {
		return h[0].(map[string]any)["root"]
	}
	return h[1].(map[string]any)["uri"]
}
func recognizeForwardAuth(obj map[string]any) error {
	a := map[string]attr.Value{}
	for k, f := range fields("forward_auth") {
		raw, e := observedField("forward_auth", k, obj)
		if e != nil || raw == nil {
			return fmt.Errorf("ID is not a supported forward-auth handler")
		}
		v, e := typedValue(context.Background(), f.attribute.GetType(), raw)
		if e != nil {
			return e
		}
		a[k] = v
	}
	for k, v := range forwardAuthFields(a) {
		if !reflect.DeepEqual(caddy.Clone(v), obj[k]) {
			return fmt.Errorf("unsupported forward-auth %s sequence", k)
		}
	}
	return nil
}
