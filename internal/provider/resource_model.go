package provider

import (
	"context"
	"errors"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type getter interface {
	GetAttribute(context.Context, path.Path, any) diag.Diagnostics
}
type setter interface {
	SetAttribute(context.Context, path.Path, any) diag.Diagnostics
}
type model struct {
	ID, ObjectID, Name, Parent, Group, Before, After, Revision types.String
	Listen, Match, Upstream                                    types.List
	Terminal, Order                                            types.Bool
}
type matcher struct {
	Hosts types.List `tfsdk:"hosts"`
}
type upstream struct {
	Dial types.String `tfsdk:"dial"`
}

var matchTypes = map[string]attr.Type{"hosts": types.ListType{ElemType: types.StringType}}
var upstreamTypes = map[string]attr.Type{"dial": types.StringType}

func (r *caddyResource) readModel(ctx context.Context, g getter) (model, diag.Diagnostics) {
	m := model{}
	var d diag.Diagnostics
	fields := map[string]any{"id": &m.ID, "object_id": &m.ObjectID, "configuration_revision": &m.Revision}
	if r.kind == "server" {
		fields["name"] = &m.Name
		fields["listen"] = &m.Listen
	} else {
		p := "server_id"
		if r.kind == "proxy" {
			p = "route_id"
		}
		fields[p] = &m.Parent
		fields["before_id"] = &m.Before
		fields["after_id"] = &m.After
		fields["order_satisfied"] = &m.Order
		if r.kind == "route" {
			fields["match"] = &m.Match
			fields["terminal"] = &m.Terminal
			fields["group"] = &m.Group
		} else {
			fields["upstream"] = &m.Upstream
		}
	}
	for k, v := range fields {
		d.Append(g.GetAttribute(ctx, path.Root(k), v)...)
	}
	return m, d
}

func (r *caddyResource) setModel(ctx context.Context, s setter, m model) diag.Diagnostics {
	var d diag.Diagnostics
	fields := map[string]any{"id": m.ID, "object_id": m.ObjectID, "configuration_revision": m.Revision}
	if r.kind == "server" {
		fields["name"] = m.Name
		fields["listen"] = m.Listen
	} else {
		p := "server_id"
		if r.kind == "proxy" {
			p = "route_id"
		}
		fields[p] = m.Parent
		fields["before_id"] = m.Before
		fields["after_id"] = m.After
		fields["order_satisfied"] = m.Order
		if r.kind == "route" {
			fields["match"] = m.Match
			fields["terminal"] = m.Terminal
			fields["group"] = m.Group
		} else {
			fields["upstream"] = m.Upstream
		}
	}
	for k, v := range fields {
		d.Append(s.SetAttribute(ctx, path.Root(k), v)...)
	}
	return d
}

func (r *caddyResource) configOwnership(ctx context.Context, g getter) Ownership {
	out := Ownership{}
	if r.kind == "server" {
		out["listen"] = true
	} else if r.kind == "proxy" {
		out["upstreams"] = true
	} else {
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
	} else {
		if owned["terminal"] {
			v.fields["terminal"] = m.Terminal.ValueBool()
		}
		if owned["group"] {
			v.fields["group"] = m.Group.ValueString()
		}
		if owned["match"] {
			var a []matcher
			d.Append(m.Match.ElementsAs(ctx, &a, false)...)
			out := make([]any, len(a))
			for i, x := range a {
				var hosts []string
				d.Append(x.Hosts.ElementsAs(ctx, &hosts, false)...)
				h := make([]any, len(hosts))
				for j, s := range hosts {
					h[j] = s
				}
				out[i] = map[string]any{"host": h}
			}
			v.fields["match"] = out
		}
	}
	return v, d
}

func (r *caddyResource) live(ctx context.Context, s caddy.Snapshot, m model) (model, error) {
	o, e := lookup(s, managed{kind: r.kind, id: m.ID.ValueString(), name: m.Name.ValueString(), parent: m.Parent.ValueString()})
	if e != nil {
		return m, e
	}
	m.ObjectID = m.ID
	if r.kind == "server" {
		m.Name = types.StringValue(o.Path[len(o.Path)-1])
		m.Listen, e = stringsList(o.Value["listen"])
		return m, e
	}
	p, _ := caddy.At(s.Value, o.Path[len(s.Path):len(o.Path)-2])
	parent := p.(map[string]any)
	m.Parent = types.StringValue(parent["@id"].(string))
	a, _ := caddy.At(s.Value, o.ParentPath[len(s.Path):])
	array, _ := a.([]any)
	ok, e := caddy.OrderSatisfied(array, m.ID.ValueString(), caddy.Position{BeforeID: m.Before.ValueString(), AfterID: m.After.ValueString()})
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
	} else {
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
		vals := []attr.Value{}
		if raw, exists := o.Value["match"]; exists {
			sets, ok := raw.([]any)
			if !ok {
				return m, errors.New("unsupported matcher shape")
			}
			for _, set := range sets {
				obj, ok := set.(map[string]any)
				if !ok || len(obj) != 1 {
					return m, errors.New("first release can only represent host-only matcher sets")
				}
				hosts, e := stringsList(obj["host"])
				if e != nil {
					return m, errors.New("first release can only represent host-only matcher sets")
				}
				vals = append(vals, types.ObjectValueMust(matchTypes, map[string]attr.Value{"hosts": hosts}))
			}
		}
		m.Match = types.ListValueMust(types.ObjectType{AttrTypes: matchTypes}, vals)
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
