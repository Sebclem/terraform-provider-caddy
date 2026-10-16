package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (r *caddyResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("configuration_revision"), types.StringValue(ownershipRevision(r.configOwnership(ctx, req.Config))))...)
	var explicit types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("object_id"), &explicit)...)
	creating := req.State.Raw.IsNull()
	replacing := false
	if !creating {
		key := "name"
		if r.kind == "error_pages" {
			key = "subroute_id"
		} else if r.kind == "route" {
			key = "server_id"
		}
		if !unordered(r.kind) && r.kind != "route" {
			key = "route_id"
		}
		var previous, planned types.String
		if r.kind != "certificate" {
			resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(key), &previous)...)
			resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(key), &planned)...)
			replacing = !previous.Equal(planned)
		}
		if r.kind == "route" {
			var a, b types.String
			req.State.GetAttribute(ctx, path.Root("subroute_id"), &a)
			req.Plan.GetAttribute(ctx, path.Root("subroute_id"), &b)
			replacing = replacing || !a.Equal(b)
		}
		if !explicit.IsNull() && !explicit.IsUnknown() {
			var priorID types.String
			resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &priorID)...)
			replacing = replacing || !explicit.Equal(priorID)
		}
	}
	// Computed IDs must remain unknown for creation/replacement, even with
	// explicit object_id, so references invalidate descendants in the same plan.
	if creating || replacing {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("id"), types.StringUnknown())...)
		if explicit.IsNull() {
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("object_id"), types.StringUnknown())...)
		}
	} else if !explicit.IsUnknown() {
		if !explicit.IsNull() {
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("id"), explicit)...)
		} else {
			var old types.String
			resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &old)...)
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("id"), old)...)
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("object_id"), old)...)
		}
	}
	if !unordered(r.kind) {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("order_satisfied"), types.BoolValue(true))...)
	}
	for k, f := range fields(r.kind) {
		var cfg attr.Value
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(k), &cfg)...)
		if cfg == nil || !cfg.IsNull() {
			continue
		}
		v, _ := typedValue(ctx, f.attribute.GetType(), nil)
		if k == "https" {
			v = types.BoolValue(false)
		}
		if k == "copy_headers" {
			v = types.ListValueMust(types.StringType, []attr.Value{})
		}
		if !req.State.Raw.IsNull() && !ownedFields(ctx, req.Private)[nativeKey(k, f)] {
			resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(k), &v)...)
		}
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(k), v)...)
	}
	if r.kind != "route" {
		return
	}
	owned := ownedFields(ctx, req.Private)
	defaults := map[string]attr.Value{"match": types.ListValueMust(types.ObjectType{AttrTypes: matchTypes}, []attr.Value{}), "terminal": types.BoolValue(false), "group": types.StringNull()}
	for k, fallback := range defaults {
		var cfg attr.Value
		switch k {
		case "match":
			var v types.List
			resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(k), &v)...)
			cfg = v
		case "terminal":
			var v types.Bool
			resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(k), &v)...)
			cfg = v
		case "group":
			var v types.String
			resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(k), &v)...)
			cfg = v
		}
		if !cfg.IsNull() {
			continue
		}
		if !req.State.Raw.IsNull() && !owned[k] {
			switch k {
			case "match":
				var v types.List
				resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(k), &v)...)
				fallback = v
			case "terminal":
				var v types.Bool
				resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(k), &v)...)
				fallback = v
			case "group":
				var v types.String
				resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(k), &v)...)
				fallback = v
			}
		}
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(k), fallback)...)
	}
}
