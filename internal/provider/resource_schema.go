package provider

import (
	"context"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"strings"
)

type caddyResource struct {
	kind   string
	client *caddy.Client
}

func (r *caddyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	suffix := map[string]string{"error_pages": "error_pages", "certificate": "tls_certificate", "log": "log", "server": "http_server", "route": "http_route", "proxy": "handler_reverse_proxy", "subroute": "handler_subroute", "forward_auth": "handler_forward_auth", "basic_auth": "handler_basic_auth", "error": "handler_error"}[r.kind]
	resp.TypeName = req.ProviderTypeName + "_" + suffix
}

func (r *caddyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	a := map[string]schema.Attribute{"configuration_revision": schema.StringAttribute{Computed: true, Description: "Fingerprint of the last applied typed field ownership; records state-only adoption after import."}, "id": schema.StringAttribute{Computed: true, Description: "Effective stable Caddy @id."}, "object_id": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}, Description: "Optional explicit Caddy @id; generated UUID v4 when omitted."}}
	blocks := map[string]schema.Block{}
	if r.kind == "certificate" {
	} else if r.kind == "error_pages" {
		a["subroute_id"] = schema.StringAttribute{Required: true, PlanModifiers: replace}
	} else if r.kind == "log" {
		a["name"] = schema.StringAttribute{Required: true, PlanModifiers: replace}
	} else if r.kind == "server" {
		a["name"] = schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "Server map name; changing it replaces the server."}
		a["listen"] = schema.ListAttribute{Required: true, ElementType: types.StringType, Description: "Nonempty ordered listening addresses."}
	} else {
		parent := "server_id"
		if r.kind != "route" {
			parent = "route_id"
		}
		a[parent] = schema.StringAttribute{Required: r.kind != "route", Optional: r.kind == "route", PlanModifiers: replace, Description: "Parent Caddy object ID."}
		a["before_id"] = schema.StringAttribute{Optional: true, Description: "Sibling ID this object must precede; mutually exclusive with after_id."}
		a["after_id"] = schema.StringAttribute{Optional: true, Description: "Sibling ID this object must follow; mutually exclusive with before_id."}
		a["order_satisfied"] = schema.BoolAttribute{Computed: true, Description: "Whether live order satisfies the configured relative constraint."}
		if r.kind == "route" {
			a["subroute_id"] = schema.StringAttribute{Optional: true, PlanModifiers: replace, Description: "Parent subroute ID; exactly one of server_id or subroute_id."}
			a["terminal"] = schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether to end routing after this route."}
			a["group"] = schema.StringAttribute{Optional: true, Computed: true, Description: "Mutually exclusive route group."}
			a["match"] = schema.ListNestedAttribute{Optional: true, Computed: true, Description: "OR matcher sets with AND conditions and one negation level.", NestedObject: schema.NestedAttributeObject{Attributes: matcherAttributes(true)}}
		} else if r.kind == "proxy" {
			blocks["upstream"] = schema.ListNestedBlock{Description: "Static upstreams; at least one is required.", NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{"dial": schema.StringAttribute{Required: true, Description: "Static backend network address, including port."}}}}
		}
	}
	for k, f := range fields(r.kind) {
		a[k] = f.attribute
	}
	resp.Schema = schema.Schema{Description: "Independently manages a live Caddy object. Deletion removes its whole subtree.", Attributes: a, Blocks: blocks}
}

func (r *caddyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*caddy.Client)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider data", "Expected a Caddy API client.")
		return
	}
	r.client = c
}

func (r *caddyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var id types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("object_id"), &id)...)
	if !id.IsNull() && !id.IsUnknown() && (id.ValueString() == "" || strings.ContainsAny(id.ValueString(), "/\\") || id.ValueString() == "." || id.ValueString() == "..") {
		resp.Diagnostics.AddError("Invalid object ID", "Use a nonempty ID without slashes or dot path segments.")
	}
	if r.kind == "error_pages" || r.kind == "certificate" || r.kind == "log" {
		validateExtra(ctx, r, req.Config, &resp.Diagnostics)
		return
	}
	if r.kind == "server" {
		var name types.String
		var listen types.List
		req.Config.GetAttribute(ctx, path.Root("name"), &name)
		req.Config.GetAttribute(ctx, path.Root("listen"), &listen)
		if !name.IsUnknown() && (name.ValueString() == "" || strings.ContainsAny(name.ValueString(), "/\\")) {
			resp.Diagnostics.AddError("Invalid server name", "Use a nonempty server map key without slashes.")
		}
		if !listen.IsUnknown() && len(listen.Elements()) == 0 {
			resp.Diagnostics.AddError("Empty listeners", "At least one listening address is required.")
		}
		validateExtra(ctx, r, req.Config, &resp.Diagnostics)
		return
	}
	var before, after types.String
	req.Config.GetAttribute(ctx, path.Root("before_id"), &before)
	req.Config.GetAttribute(ctx, path.Root("after_id"), &after)
	if !before.IsNull() && !after.IsNull() {
		resp.Diagnostics.AddError("Conflicting ordering", "Configure only one of before_id or after_id.")
	}
	if !id.IsUnknown() && !id.IsNull() && ((!before.IsUnknown() && id.Equal(before)) || (!after.IsUnknown() && id.Equal(after))) {
		resp.Diagnostics.AddError("Invalid ordering", "An object cannot be ordered relative to itself.")
	}
	if r.kind == "proxy" {
		var u types.List
		req.Config.GetAttribute(ctx, path.Root("upstream"), &u)
		if !u.IsUnknown() && len(u.Elements()) == 0 {
			resp.Diagnostics.AddError("Empty upstreams", "At least one static upstream is required.")
		}
		var values []upstream
		if !u.IsUnknown() {
			resp.Diagnostics.Append(u.ElementsAs(ctx, &values, false)...)
			for _, v := range values {
				if !v.Dial.IsUnknown() && strings.TrimSpace(v.Dial.ValueString()) == "" {
					resp.Diagnostics.AddError("Invalid upstream", "Upstream dial must not be empty.")
				}
			}
		}
	}
	if r.kind == "route" {
		var parent, sub types.String
		req.Config.GetAttribute(ctx, path.Root("server_id"), &parent)
		req.Config.GetAttribute(ctx, path.Root("subroute_id"), &sub)
		if parent.IsNull() == sub.IsNull() {
			resp.Diagnostics.AddError("Invalid route parent", "Configure exactly one of server_id and subroute_id.")
		}
	}
	validateExtra(ctx, r, req.Config, &resp.Diagnostics)
}
