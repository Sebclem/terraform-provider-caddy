package provider

import (
	"context"
	"errors"
	"fmt"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"time"
)

func (r *caddyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	m, d := r.readModel(ctx, req.Plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := m.ObjectID.ValueString()
	if id == "" {
		id = uuid.NewString()
	}
	m.ID = types.StringValue(id)
	m.ObjectID = m.ID
	owned := r.configOwnership(ctx, req.Config)
	v, d := r.managed(ctx, m, owned)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if e := createObject(ctx, r.client, v); e != nil {
		if errors.Is(e, caddy.ErrUncertainWrite) {
			resp.Diagnostics.Append(r.setModel(ctx, &resp.State, m)...)
			resp.Diagnostics.Append(saveOwned(ctx, resp.Private, owned)...)
		}
		resp.Diagnostics.AddError("Caddy create failed", fmt.Sprintf("Object ID %s: %v", id, e))
		return
	}
	s, e := r.client.Get(ctx, caddy.Path{"config"})
	if e == nil {
		m, e = r.live(ctx, s, m)
	}
	if e != nil {
		resp.Diagnostics.Append(r.setModel(ctx, &resp.State, m)...)
		resp.Diagnostics.AddError("Caddy create readback failed", fmt.Sprintf("Retain object ID %s for recovery: %v", id, e))
		return
	}
	resp.Diagnostics.Append(r.setModel(ctx, &resp.State, m)...)
	resp.Diagnostics.Append(saveOwned(ctx, resp.Private, owned)...)
}

func (r *caddyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	m, d := r.readModel(ctx, req.State)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, e := r.client.Get(ctx, caddy.Path{"config"})
	if e != nil {
		resp.Diagnostics.AddError("Caddy refresh failed", e.Error())
		return
	}
	m, e = r.live(ctx, s, m)
	if errors.Is(e, caddy.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if e != nil {
		resp.Diagnostics.AddError("Caddy refresh conflict", e.Error())
		return
	}
	resp.Diagnostics.Append(r.setModel(ctx, &resp.State, m)...)
}

func (r *caddyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	m, d := r.readModel(ctx, req.Plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	owned := r.configOwnership(ctx, req.Config)
	v, d := r.managed(ctx, m, owned)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if e := updateObject(ctx, r.client, v, ownedFields(ctx, req.Private)); e != nil {
		resp.Diagnostics.AddError("Caddy update failed", e.Error())
		return
	}
	s, e := r.client.Get(ctx, caddy.Path{"config"})
	if e == nil {
		m, e = r.live(ctx, s, m)
	}
	if e != nil {
		resp.Diagnostics.AddError("Caddy update readback failed", e.Error())
		return
	}
	resp.Diagnostics.Append(r.setModel(ctx, &resp.State, m)...)
	resp.Diagnostics.Append(saveOwned(ctx, resp.Private, owned)...)
}

func (r *caddyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	m, d := r.readModel(ctx, req.State)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.kind == "route" && !m.SubParent.IsNull() {
		m.Parent = m.SubParent
	}
	if e := deleteObject(ctx, r.client, managed{kind: r.kind, id: m.ID.ValueString(), name: m.Name.ValueString(), parent: m.Parent.ValueString()}); e != nil {
		resp.Diagnostics.AddError("Caddy deletion failed", e.Error())
	}
}

func (r *caddyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		resp.Diagnostics.AddError("Missing import ID", "Import requires an existing Caddy @id.")
		return
	}
	m := model{ID: types.StringValue(req.ID), ObjectID: types.StringValue(req.ID), Before: types.StringNull(), After: types.StringNull(), Name: types.StringNull(), Parent: types.StringNull()}
	s, e := r.client.Get(ctx, caddy.Path{"config"})
	if e == nil {
		m, e = r.live(ctx, s, m)
	}
	if e != nil {
		resp.Diagnostics.AddError("Cannot import Caddy ID", e.Error())
		return
	}
	owned := Ownership{}
	if r.kind == "server" {
		owned["listen"] = true
	}
	if r.kind == "proxy" {
		owned["upstreams"] = true
	}
	for k, f := range fields(r.kind) {
		if f.attribute.IsRequired() {
			owned[nativeKey(k, f)] = true
			markLeaves(owned, nativeKey(k, f), encodeField(r.kind, k, nativeValue(m.Extra[k])))
		}
	}
	if r.kind == "forward_auth" {
		owned["headers"] = true
	}
	if r.kind == "error_pages" {
		owned = Ownership{"handle": true}
	}
	m.Revision = types.StringValue(ownershipRevision(owned))
	resp.Diagnostics.Append(r.setModel(ctx, &resp.State, m)...)
	resp.Diagnostics.Append(saveOwned(ctx, resp.Private, owned)...)
}
