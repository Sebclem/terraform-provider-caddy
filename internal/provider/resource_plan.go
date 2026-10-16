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
	if !explicit.IsUnknown() {
		if !explicit.IsNull() {
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("id"), explicit)...)
		} else if !req.State.Raw.IsNull() {
			var old types.String
			resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &old)...)
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("id"), old)...)
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("object_id"), old)...)
		}
	}
	if r.kind != "server" {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("order_satisfied"), types.BoolValue(true))...)
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
