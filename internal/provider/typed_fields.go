package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"reflect"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type fieldSpec struct {
	attribute schema.Attribute
	native    string
}

func str(required bool) schema.StringAttribute {
	return schema.StringAttribute{Required: required, Optional: !required, Computed: !required}
}
func boolean() schema.BoolAttribute  { return schema.BoolAttribute{Optional: true, Computed: true} }
func integer() schema.Int64Attribute { return schema.Int64Attribute{Optional: true, Computed: true} }
func list() schema.ListAttribute {
	return schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType}
}
func object(a map[string]schema.Attribute) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Optional: true, Computed: true, Attributes: a}
}
func fields(kind string) map[string]fieldSpec {
	switch kind {
	case "certificate":
		return map[string]fieldSpec{"certificate": {str(true), "certificate"}, "key": {str(true), "key"}, "tags": {list(), "tags"}}
	case "error_pages":
		return map[string]fieldSpec{"root": {str(true), "root"}, "page_uri": {str(true), "page_uri"}}
	case "log":
		return map[string]fieldSpec{"writer": {object(map[string]schema.Attribute{"filename": str(true), "mode": str(false), "roll_size_mb": integer(), "roll_local_time": boolean()}), "writer"}, "include": {list(), "include"}, "exclude": {list(), "exclude"}}
	case "server":
		return map[string]fieldSpec{
			"trusted_proxies": {list(), "trusted_proxies"}, "https": {boolean(), "https"},
		}
	case "proxy":
		return map[string]fieldSpec{
			"transport":       {object(map[string]schema.Attribute{"tls": schema.BoolAttribute{Required: true}, "insecure_skip_verify": boolean(), "server_name": str(false)}), "transport"},
			"request_headers": {object(map[string]schema.Attribute{"set": schema.MapAttribute{Optional: true, Computed: true, ElementType: types.ListType{ElemType: types.StringType}}, "add": schema.MapAttribute{Optional: true, Computed: true, ElementType: types.ListType{ElemType: types.StringType}}, "delete": list()}), "headers"},
			"request_buffers": {integer(), "request_buffers"}, "response_buffers": {integer(), "response_buffers"}, "flush_interval": {str(false), "flush_interval"},
			"health": {object(map[string]schema.Attribute{"uri": str(false), "follow_redirects": boolean(), "fails": integer(), "timeout": str(false)}), "health_checks"},
		}
	case "forward_auth":
		return map[string]fieldSpec{"upstream": {str(true), "upstreams"}, "uri": {str(true), "rewrite"}, "copy_headers": {list(), "handle_response"}}
	case "basic_auth":
		return map[string]fieldSpec{"accounts": {schema.ListNestedAttribute{Required: true, Sensitive: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{"username": schema.StringAttribute{Required: true, Sensitive: true}, "password_hash": schema.StringAttribute{Required: true, Sensitive: true}}}}, "providers"}}
	case "error":
		return map[string]fieldSpec{"status_code": {schema.Int64Attribute{Required: true}, "status_code"}, "message": {str(true), "error"}}
	}
	return nil
}
func nativeValue(v attr.Value) any {
	if v == nil || v.IsNull() || v.IsUnknown() {
		return nil
	}
	switch x := v.(type) {
	case types.String:
		return x.ValueString()
	case types.Bool:
		return x.ValueBool()
	case types.Int64:
		return json.Number(strconv.FormatInt(x.ValueInt64(), 10))
	case types.List:
		out := make([]any, len(x.Elements()))
		for i, v := range x.Elements() {
			out[i] = nativeValue(v)
		}
		return out
	case types.Map:
		out := map[string]any{}
		for k, v := range x.Elements() {
			if a := nativeValue(v); a != nil {
				out[k] = a
			}
		}
		return out
	case types.Object:
		out := map[string]any{}
		for k, v := range x.Attributes() {
			if a := nativeValue(v); a != nil {
				out[k] = a
			}
		}
		return out
	}
	return nil
}
func typedValue(ctx context.Context, t attr.Type, v any) (attr.Value, error) {
	if v == nil {
		return t.ValueFromTerraform(ctx, tftypes.NewValue(t.TerraformType(ctx), nil))
	}
	switch x := t.(type) {
	case basetypes.StringType: // string coercion only for native duration values is handled before this function
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("expected string")
		}
		return types.StringValue(s), nil
	case basetypes.BoolType:
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("expected boolean")
		}
		return types.BoolValue(b), nil
	case basetypes.Int64Type:
		n, e := strconv.ParseInt(fmt.Sprint(v), 10, 64)
		if e != nil {
			return nil, e
		}
		return types.Int64Value(n), nil
	case types.ListType:
		a, ok := v.([]any)
		if !ok {
			return nil, fmt.Errorf("expected array")
		}
		out := make([]attr.Value, len(a))
		for i, v := range a {
			a, e := typedValue(ctx, x.ElemType, v)
			if e != nil {
				return nil, e
			}
			out[i] = a
		}
		return types.ListValueMust(x.ElemType, out), nil
	case types.MapType:
		a, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected map")
		}
		out := map[string]attr.Value{}
		for k, v := range a {
			a, e := typedValue(ctx, x.ElemType, v)
			if e != nil {
				return nil, e
			}
			out[k] = a
		}
		return types.MapValueMust(x.ElemType, out), nil
	case types.ObjectType:
		a, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected object")
		}
		out := map[string]attr.Value{}
		for k, t := range x.AttrTypes {
			a, e := typedValue(ctx, t, a[k])
			if e != nil {
				return nil, fmt.Errorf("%s: %w", k, e)
			}
			out[k] = a
		}
		return types.ObjectValueMust(x.AttrTypes, out), nil
	}
	return nil, fmt.Errorf("unsupported typed value %T", t)
}
func durationNative(v any) any {
	if s, ok := v.(string); ok {
		if d, e := time.ParseDuration(s); e == nil {
			return json.Number(strconv.FormatInt(int64(d), 10))
		}
	}
	return v
}
func durationString(v any) any {
	if v == nil {
		return nil
	}
	if _, ok := v.(string); ok {
		return v
	}
	i, e := strconv.ParseInt(fmt.Sprint(v), 10, 64)
	if e != nil {
		return v
	}
	return time.Duration(i).String()
}
func encodeField(kind, key string, v any) any {
	if v == nil {
		return nil
	}
	switch key {
	case "writer":
		m := v.(map[string]any)
		m["output"] = "file"
		return m
	case "trusted_proxies":
		return map[string]any{"source": "static", "ranges": v}
	case "https":
		if v == true {
			return map[string]any{"disable_certificates": true}
		}
		return map[string]any{"disable": true}
	case "transport":
		m := v.(map[string]any)
		out := map[string]any{"protocol": "http"}
		if m["tls"] == true {
			tls := map[string]any{}
			for _, k := range []string{"insecure_skip_verify", "server_name"} {
				if x, ok := m[k]; ok {
					tls[k] = x
				}
			}
			out["tls"] = tls
		}
		return out
	case "request_headers":
		return map[string]any{"request": v}
	case "flush_interval":
		return durationNative(v)
	case "health":
		m := v.(map[string]any)
		if a, ok := m["timeout"]; ok {
			m["timeout"] = durationNative(a)
		}
		return map[string]any{"active": m}
	case "accounts":
		a := v.([]any)
		users := make([]any, len(a))
		for i, u := range a {
			m := u.(map[string]any)
			users[i] = map[string]any{"username": m["username"], "password": m["password_hash"]}
		}
		return map[string]any{"http_basic": map[string]any{"hash": map[string]any{"algorithm": "bcrypt"}, "accounts": users}}
	}
	return v
}
func decodeField(kind, key string, v any) any {
	if v == nil {
		return nil
	}
	switch key {
	case "trusted_proxies":
		return v.(map[string]any)["ranges"]
	case "https":
		m := v.(map[string]any)
		return m["disable"] != true
	case "transport":
		m := v.(map[string]any)
		out := map[string]any{"tls": false}
		if tls, ok := m["tls"].(map[string]any); ok {
			out["tls"] = true
			for _, k := range []string{"insecure_skip_verify", "server_name"} {
				if a, ok := tls[k]; ok {
					out[k] = a
				}
			}
		}
		return out
	case "request_headers":
		return v.(map[string]any)["request"]
	case "flush_interval":
		return durationString(v)
	case "health":
		m := v.(map[string]any)["active"]
		if a, ok := m.(map[string]any); ok {
			if d, ok := a["timeout"]; ok {
				a["timeout"] = durationString(d)
			}
		}
		return m
	case "accounts":
		m := v.(map[string]any)["http_basic"].(map[string]any)
		users := m["accounts"].([]any)
		out := make([]any, len(users))
		for i, u := range users {
			a := u.(map[string]any)
			out[i] = map[string]any{"username": a["username"], "password_hash": a["password"]}
		}
		return out
	}
	return v
}
func nativeKey(k string, f fieldSpec) string {
	if k == "https" {
		return "automatic_https"
	}
	return f.native
}
func markLeaves(o Ownership, prefix string, v any) {
	o[prefix] = true
	if m, ok := v.(map[string]any); ok {
		for k, v := range m {
			markLeaves(o, prefix+"."+k, v)
		}
	} else {
		o[prefix] = true
	}
}

// Merge schema-owned leaves into fresh live objects, retaining unknown siblings.
func mergeFields(live, want map[string]any, owned Ownership, prefix string) map[string]any {
	out := map[string]any{}
	for k, v := range want {
		out[k] = v
	}
	if prefix != "" {
		for k, v := range live {
			if _, ok := out[k]; !ok && !owned[prefix+k] {
				out[k] = v
			}
		}
	}
	for k, v := range out {
		if m, ok := v.(map[string]any); ok {
			old, _ := live[k].(map[string]any)
			out[k] = mergeFields(old, m, owned, prefix+k+".")
		}
	}
	return out
}
func nestedEdits(p []string, old, want map[string]any) []change {
	var out []change
	for k, v := range want {
		q := append(append([]string(nil), p...), k)
		if reflect.DeepEqual(v, old[k]) {
			continue
		}
		if a, ok := old[k].(map[string]any); ok {
			if b, ok := v.(map[string]any); ok {
				out = append(out, nestedEdits(q, a, b)...)
				continue
			}
		}
		out = append(out, change{p: q, value: v})
	}
	for k := range old {
		if _, ok := want[k]; !ok {
			out = append(out, change{p: append(append([]string(nil), p...), k), remove: true})
		}
	}
	return out
}
func validateExtra(ctx context.Context, r *caddyResource, g getter, d *diag.Diagnostics) {
	for k := range fields(r.kind) {
		var v attr.Value
		d.Append(g.GetAttribute(ctx, path.Root(k), &v)...)
		if v == nil || v.IsNull() || v.IsUnknown() {
			continue
		}
		n := nativeValue(v)
		switch k {
		case "accounts":
			if a, ok := n.([]any); ok && len(a) == 0 {
				d.AddError("Empty accounts", "At least one bcrypt account is required.")
			}
		case "transport":
			if m, ok := n.(map[string]any); ok && m["tls"] != true {
				_, skip := m["insecure_skip_verify"]
				_, name := m["server_name"]
				if skip || name {
					d.AddError("Invalid transport TLS", "insecure_skip_verify and server_name require tls = true.")
				}
			}

		case "flush_interval":
			if _, e := time.ParseDuration(fmt.Sprint(n)); e != nil {
				d.AddError("Invalid flush interval", "Use a duration such as -1ns or 100ms.")
			}
		case "request_buffers", "response_buffers":
			if i, e := strconv.ParseInt(fmt.Sprint(n), 10, 64); e != nil || i < 0 {
				d.AddError("Invalid buffer size", "Buffer size must be nonnegative bytes.")
			}
		case "health":
			if m, ok := n.(map[string]any); ok {
				if v, ok := m["timeout"]; ok {
					duration, e := time.ParseDuration(fmt.Sprint(v))
					if e != nil || duration <= 0 {
						d.AddError("Invalid health timeout", "Use a positive duration.")
					}
				}
			}
		case "status_code":
			i, _ := strconv.Atoi(fmt.Sprint(n))
			if i < 400 || i > 599 {
				d.AddError("Invalid error status", "Use an HTTP error status from 400 through 599.")
			}
		}
	}
}
func observedField(kind, key string, obj map[string]any) (v any, err error) {
	defer func() {
		if recover() != nil {
			v = nil
			err = fmt.Errorf("unsupported %s %s configuration", kind, key)
		}
	}()
	if kind == "forward_auth" {
		return decodeForwardAuth(key, obj), nil
	}
	if kind == "error_pages" {
		return decodeErrorPage(key, obj), nil
	}
	f := fields(kind)[key]
	raw := obj[nativeKey(key, f)]
	if raw != nil {
		switch key {
		case "transport":
			if raw.(map[string]any)["protocol"] != "http" {
				return nil, fmt.Errorf("only HTTP transport can be represented")
			}
		case "writer":
			if raw.(map[string]any)["output"] != "file" {
				return nil, fmt.Errorf("only file writers can be represented")
			}
		case "accounts":
			basic := raw.(map[string]any)["http_basic"].(map[string]any)
			if basic["hash"].(map[string]any)["algorithm"] != "bcrypt" {
				return nil, fmt.Errorf("only bcrypt accounts can be represented")
			}
		}
	}
	return decodeField(kind, key, caddy.Clone(obj[nativeKey(key, f)])), nil
}
func fullyKnown(v attr.Value) bool {
	if v == nil || v.IsUnknown() {
		return false
	}
	if v.IsNull() {
		return true
	}
	switch x := v.(type) {
	case types.Object:
		for _, v := range x.Attributes() {
			if !fullyKnown(v) {
				return false
			}
		}
	case types.List:
		for _, v := range x.Elements() {
			if !fullyKnown(v) {
				return false
			}
		}
	case types.Map:
		for _, v := range x.Elements() {
			if !fullyKnown(v) {
				return false
			}
		}
	}
	return true
}
func nestedOwnershipPath(key, nested string) string {
	switch key {
	case "health":
		return "health_checks.active." + nested
	case "transport":
		if nested == "tls" {
			return "transport.tls"
		}
		return "transport.tls." + nested
	case "request_headers":
		return "headers.request." + nested
	}
	return key + "." + nested
}
func planNestedValue(ctx context.Context, key, prefix string, cfg, previous attr.Value, owned Ownership) attr.Value {
	if cfg.IsUnknown() {
		return cfg
	}
	if cfg.IsNull() {
		if !owned[nestedOwnershipPath(key, prefix)] && previous != nil {
			return previous
		}
		v, _ := typedValue(ctx, cfg.Type(ctx), nil)
		return v
	}
	obj, ok := cfg.(types.Object)
	if !ok {
		return cfg
	}
	old := map[string]attr.Value{}
	if o, ok := previous.(types.Object); ok && !o.IsNull() && !o.IsUnknown() {
		old = o.Attributes()
	}
	result := map[string]attr.Value{}
	for k, v := range obj.Attributes() {
		p := k
		if prefix != "" {
			p = prefix + "." + k
		}
		result[k] = planNestedValue(ctx, key, p, v, old[k], owned)
	}
	return types.ObjectValueMust(obj.AttributeTypes(ctx), result)
}
