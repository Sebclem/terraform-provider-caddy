package provider

import (
	"context"
	"errors"
	"fmt"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"reflect"
)

type getter interface {
	GetAttribute(context.Context, path.Path, any) diag.Diagnostics
}
type setter interface {
	SetAttribute(context.Context, path.Path, any) diag.Diagnostics
}
type model struct {
	ID, ObjectID, Name, Parent, Group, Before, After, Revision types.String
	SubParent                                                  types.String
	Extra                                                      map[string]attr.Value
	Listen, Match, Upstream                                    types.List
	Terminal, Order                                            types.Bool
}
type upstream struct {
	Dial types.String `tfsdk:"dial"`
}

var upstreamTypes = map[string]attr.Type{"dial": types.StringType}

func (r *caddyResource) modelFields(m *model) map[string]any {
	fields := map[string]any{"id": &m.ID, "object_id": &m.ObjectID, "configuration_revision": &m.Revision}
	if r.kind == "error_pages" {
		fields["subroute_id"] = &m.Parent
		return fields
	}
	if r.kind == "certificate" {
		return fields
	}
	if r.kind == "log" {
		fields["name"] = &m.Name
		return fields
	}
	if r.kind == "server" {
		fields["name"] = &m.Name
		fields["listen"] = &m.Listen
	} else {
		p := "route_id"
		if r.kind == "route" {
			p = "server_id"
			fields["subroute_id"] = &m.SubParent
		}
		fields[p] = &m.Parent
		fields["before_id"] = &m.Before
		fields["after_id"] = &m.After
		fields["order_satisfied"] = &m.Order
		if r.kind == "route" {
			fields["match"] = &m.Match
			fields["terminal"] = &m.Terminal
			fields["group"] = &m.Group
		}
		if r.kind == "proxy" {
			fields["upstream"] = &m.Upstream
		}
	}
	return fields
}
func (r *caddyResource) readModel(ctx context.Context, g getter) (model, diag.Diagnostics) {
	m := model{Extra: map[string]attr.Value{}}
	var d diag.Diagnostics
	for k, v := range r.modelFields(&m) {
		d.Append(g.GetAttribute(ctx, path.Root(k), v)...)
	}
	for k := range fields(r.kind) {
		var v attr.Value
		d.Append(g.GetAttribute(ctx, path.Root(k), &v)...)
		m.Extra[k] = v
	}
	return m, d
}
func (r *caddyResource) setModel(ctx context.Context, s setter, m model) diag.Diagnostics {
	var d diag.Diagnostics
	for k, v := range r.modelFields(&m) {
		d.Append(s.SetAttribute(ctx, path.Root(k), v)...)
	}
	for k, v := range m.Extra {
		d.Append(s.SetAttribute(ctx, path.Root(k), v)...)
	}
	return d
}

func (r *caddyResource) configOwnership(ctx context.Context, g getter) Ownership {
	out := Ownership{}
	if r.kind == "error_pages" {
		return Ownership{"handle": true}
	}
	for k, f := range fields(r.kind) {
		var v attr.Value
		g.GetAttribute(ctx, path.Root(k), &v)
		if v != nil && !v.IsNull() {
			out[nativeKey(k, f)] = true
			if k == "https" {
				out["tls_connection_policies"] = true
				out["automatic_https.disable"] = true
				out["automatic_https.disable_certificates"] = true
			}
			markLeaves(out, nativeKey(k, f), encodeField(r.kind, k, nativeValue(v)))
		}
	}
	if r.kind == "forward_auth" {
		out["headers"] = true
	}
	if r.kind == "server" {
		out["listen"] = true
	} else if r.kind == "proxy" {
		out["upstreams"] = true
	} else if r.kind == "route" {
		var match types.List
		var terminal types.Bool
		var group types.String
		g.GetAttribute(ctx, path.Root("match"), &match)
		g.GetAttribute(ctx, path.Root("terminal"), &terminal)
		g.GetAttribute(ctx, path.Root("group"), &group)
		if !match.IsNull() {
			out["match"] = true
		}
		if !terminal.IsNull() {
			out["terminal"] = true
		}
		if !group.IsNull() {
			out["group"] = true
		}
	}
	return out
}

func (r *caddyResource) managed(ctx context.Context, m model, owned Ownership) (managed, diag.Diagnostics) {
	v := managed{kind: r.kind, id: m.ID.ValueString(), name: m.Name.ValueString(), parent: m.Parent.ValueString(), fields: map[string]any{}, position: caddy.Position{BeforeID: m.Before.ValueString(), AfterID: m.After.ValueString()}}
	var d diag.Diagnostics
	if r.kind == "route" && !m.SubParent.IsNull() {
		v.parent = m.SubParent.ValueString()
	}
	for k, f := range fields(r.kind) {
		if owned[nativeKey(k, f)] {
			v.fields[nativeKey(k, f)] = encodeField(r.kind, k, nativeValue(m.Extra[k]))
		}
	}
	if r.kind == "server" && owned["automatic_https"] {
		if m.Extra["https"] != nil && nativeValue(m.Extra["https"]) == true {
			v.fields["tls_connection_policies"] = []any{map[string]any{}}
		} else {
			v.fields["automatic_https"] = map[string]any{"disable": true}
		}
	}
	if r.kind == "error_pages" {
		v.fields = errorPagesFields(v.id, nativeValue(m.Extra["root"]), nativeValue(m.Extra["page_uri"]))
	}
	if r.kind == "forward_auth" {
		v.fields = forwardAuthFields(m.Extra)
	}
	if r.kind == "server" {
		var a []string
		d.Append(m.Listen.ElementsAs(ctx, &a, false)...)
		out := make([]any, len(a))
		for i, s := range a {
			out[i] = s
		}
		v.fields["listen"] = out
	} else if r.kind == "proxy" {
		var a []upstream
		d.Append(m.Upstream.ElementsAs(ctx, &a, false)...)
		out := make([]any, len(a))
		for i, u := range a {
			out[i] = map[string]any{"dial": u.Dial.ValueString()}
		}
		v.fields["upstreams"] = out
	} else if r.kind == "route" {
		if owned["terminal"] {
			v.fields["terminal"] = m.Terminal.ValueBool()
		}
		if owned["group"] {
			v.fields["group"] = m.Group.ValueString()
		}
		if owned["match"] {
			out, e := encodeMatchers(nativeValue(m.Match))
			if e != nil {
				d.AddError("Invalid matchers", e.Error())
			}
			v.fields["match"] = out
		}
	}
	return v, d
}

func (r *caddyResource) live(ctx context.Context, s caddy.Snapshot, m model) (model, error) {
	parentID := m.Parent.ValueString()
	if r.kind == "route" && !m.SubParent.IsNull() {
		parentID = m.SubParent.ValueString()
	}
	o, e := lookup(s, managed{kind: r.kind, id: m.ID.ValueString(), name: m.Name.ValueString(), parent: parentID})
	if e != nil {
		return m, e
	}
	m.ObjectID = m.ID
	previousExtra := m.Extra
	m.Extra = map[string]attr.Value{}
	for k, f := range fields(r.kind) {
		raw, err := observedField(r.kind, k, o.Value)
		if err != nil {
			return m, err
		}
		v, err := typedValue(ctx, f.attribute.GetType(), raw)
		if err != nil {
			return m, fmt.Errorf("%s: %w", k, err)
		}
		if previous := previousExtra[k]; previous != nil && !previous.IsNull() && fullyKnown(previous) && r.kind != "forward_auth" && r.kind != "error_pages" {
			raw := encodeField(r.kind, k, nativeValue(previous))
			if reflect.DeepEqual(caddy.Clone(raw), o.Value[nativeKey(k, f)]) {
				v = previous
			}
		}
		m.Extra[k] = v
	}
	if r.kind == "error_pages" {
		p, e := caddy.At(s.Value, o.Path[len(s.Path):len(o.Path)-3])
		if e != nil {
			return m, e
		}
		m.Parent = types.StringValue(p.(map[string]any)["@id"].(string))
		return m, nil
	}
	if r.kind == "certificate" {
		return m, nil
	}
	if r.kind == "log" {
		m.Name = types.StringValue(o.Path[len(o.Path)-1])
		return m, nil
	}
	if r.kind == "server" {
		m.Name = types.StringValue(o.Path[len(o.Path)-1])
		m.Listen, e = stringsList(o.Value["listen"])
		return m, e
	}
	p, _ := caddy.At(s.Value, o.Path[len(s.Path):len(o.Path)-2])
	parent := p.(map[string]any)
	m.Parent = types.StringValue(parent["@id"].(string))
	m.SubParent = types.StringNull()
	if r.kind == "route" && parent["handler"] == "subroute" {
		m.SubParent = m.Parent
		m.Parent = types.StringNull()
	}
	a, _ := caddy.At(s.Value, o.ParentPath[len(s.Path):])
	array, _ := a.([]any)
	ok, e := caddy.OrderSatisfied(array, m.ID.ValueString(), caddy.Position{BeforeID: m.Before.ValueString(), AfterID: m.After.ValueString()})
	if errors.Is(e, caddy.ErrMissingAnchor) {
		ok = false
		e = nil
	}
	if e != nil {
		return m, e
	}
	m.Order = types.BoolValue(ok)
	if r.kind == "proxy" {
		a, ok := o.Value["upstreams"].([]any)
		if !ok || len(a) == 0 {
			return m, errors.New("reverse proxy has no representable static upstreams")
		}
		vals := make([]attr.Value, len(a))
		for i, x := range a {
			obj, ok := x.(map[string]any)
			if !ok {
				return m, errors.New("invalid upstream")
			}
			dial, ok := obj["dial"].(string)
			if !ok {
				return m, errors.New("upstream dial is not a string")
			}
			vals[i] = types.ObjectValueMust(upstreamTypes, map[string]attr.Value{"dial": types.StringValue(dial)})
		}
		m.Upstream = types.ListValueMust(types.ObjectType{AttrTypes: upstreamTypes}, vals)
	} else if r.kind == "route" {
		terminal, exists := o.Value["terminal"]
		if exists {
			b, ok := terminal.(bool)
			if !ok {
				return m, errors.New("unsupported terminal value")
			}
			m.Terminal = types.BoolValue(b)
		} else {
			m.Terminal = types.BoolValue(false)
		}
		m.Group = types.StringNull()
		if v, ok := o.Value["group"]; ok {
			str, ok := v.(string)
			if !ok {
				return m, errors.New("unsupported group value")
			}
			m.Group = types.StringValue(str)
		}
		raw := o.Value["match"]
		if raw == nil {
			raw = []any{}
		}
		decoded, err := decodeMatchers(raw)
		if err != nil {
			return m, err
		}
		v, err := typedValue(ctx, types.ListType{ElemType: types.ObjectType{AttrTypes: matchTypes}}, decoded)
		if err != nil {
			return m, err
		}
		m.Match = v.(types.List)
	}
	return m, nil
}

func stringsList(v any) (types.List, error) {
	a, ok := v.([]any)
	if !ok {
		return types.ListNull(types.StringType), errors.New("expected a string array")
	}
	out := make([]attr.Value, len(a))
	for i, v := range a {
		s, ok := v.(string)
		if !ok {
			return types.ListNull(types.StringType), errors.New("expected a string array")
		}
		out[i] = types.StringValue(s)
	}
	return types.ListValueMust(types.StringType, out), nil
}
