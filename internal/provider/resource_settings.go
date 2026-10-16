package provider

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"time"

	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type settingsResource struct {
	kind   string
	client *caddy.Client
}
type settingsModel struct {
	ID       types.String `tfsdk:"id"`
	Server   types.String `tfsdk:"server_id"`
	Hostname types.String `tfsdk:"hostname"`
	Log      types.String `tfsdk:"log_id"`
}
type metricsModel struct {
	ID      types.String `tfsdk:"id"`
	PerHost types.Bool   `tfsdk:"per_host"`
}

func NewLogBindingResource() resource.Resource  { return &settingsResource{kind: "binding"} }
func NewHTTPMetricsResource() resource.Resource { return &settingsResource{kind: "metrics"} }
func (r *settingsResource) Metadata(_ context.Context, q resource.MetadataRequest, p *resource.MetadataResponse) {
	s := "log_binding"
	if r.kind == "metrics" {
		s = "http_metrics"
	}
	p.TypeName = q.ProviderTypeName + "_" + s
}
func (r *settingsResource) Schema(_ context.Context, _ resource.SchemaRequest, p *resource.SchemaResponse) {
	a := map[string]schema.Attribute{"id": schema.StringAttribute{Computed: true, Description: "Provider state identity; not a Caddy @id. Import is unsupported."}}
	if r.kind == "metrics" {
		a["per_host"] = schema.BoolAttribute{Required: true}
	} else {
		replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
		a["server_id"] = schema.StringAttribute{Required: true, PlanModifiers: replace}
		a["hostname"] = schema.StringAttribute{Required: true, PlanModifiers: replace}
		a["log_id"] = schema.StringAttribute{Required: true}
	}
	p.Schema = schema.Schema{Attributes: a, Description: "Manages a typed ID-less Caddy setting. Exactly one state resource may own this location; import is unsupported."}
}
func (r *settingsResource) Configure(_ context.Context, q resource.ConfigureRequest, p *resource.ConfigureResponse) {
	if q.ProviderData == nil {
		return
	}
	c, ok := q.ProviderData.(*caddy.Client)
	if !ok {
		p.Diagnostics.AddError("Invalid provider data", "Expected Caddy client.")
		return
	}
	r.client = c
}
func (r *settingsResource) ModifyPlan(ctx context.Context, q resource.ModifyPlanRequest, p *resource.ModifyPlanResponse) {
	if q.Plan.Raw.IsNull() || q.State.Raw.IsNull() {
		return
	}
	if r.kind == "metrics" {
		p.Diagnostics.Append(p.Plan.SetAttribute(ctx, path.Root("id"), types.StringValue("http-metrics"))...)
		return
	}
	var old, next settingsModel
	p.Diagnostics.Append(q.State.Get(ctx, &old)...)
	p.Diagnostics.Append(q.Plan.Get(ctx, &next)...)
	if old.Server.Equal(next.Server) && old.Hostname.Equal(next.Hostname) {
		p.Diagnostics.Append(p.Plan.SetAttribute(ctx, path.Root("id"), old.ID)...)
	}
}
func (r *settingsResource) desired(ctx context.Context, g getter) (settingsModel, caddy.Path, any, error) {
	if r.kind == "metrics" {
		var b types.Bool
		d := g.GetAttribute(ctx, path.Root("per_host"), &b)
		if d.HasError() {
			return settingsModel{}, nil, nil, fmt.Errorf("invalid metrics config")
		}
		return settingsModel{ID: types.StringValue("http-metrics")}, caddy.Path{"config", "apps", "http", "metrics"}, map[string]any{"per_host": b.ValueBool()}, nil
	}
	var m settingsModel
	for k, v := range map[string]any{"id": &m.ID, "server_id": &m.Server, "hostname": &m.Hostname, "log_id": &m.Log} {
		if d := g.GetAttribute(ctx, path.Root(k), v); d.HasError() {
			return m, nil, nil, fmt.Errorf("invalid binding config")
		}
	}
	if m.Hostname.ValueString() == "" {
		return m, nil, nil, errors.New("hostname must not be empty")
	}
	m.ID = types.StringValue(fmt.Sprintf("binding-%x", sha256.Sum256([]byte(m.Server.ValueString()+"\x00"+m.Hostname.ValueString()))))
	return m, nil, nil, nil
}
func (r *settingsResource) location(s caddy.Snapshot, m settingsModel) (caddy.Path, any, error) {
	if r.kind == "metrics" {
		return caddy.Path{"config", "apps", "http", "metrics"}, nil, nil
	}
	server, e := lookup(s, managed{kind: "server", id: m.Server.ValueString()})
	if e != nil {
		return nil, nil, e
	}
	log, e := lookup(s, managed{kind: "log", id: m.Log.ValueString()})
	if e != nil {
		return nil, nil, e
	}
	name := log.Path[len(log.Path)-1]
	return append(append(caddy.Path(nil), server.Path...), "logs", "logger_names", m.Hostname.ValueString()), []any{name}, nil
}
func settingDecision(s caddy.Snapshot, dest caddy.Path, value any) (caddy.Decision, error) {
	for pos := len(s.Path); pos < len(dest); pos++ {
		v, e := caddy.At(s.Value, dest[len(s.Path):pos+1])
		if errors.Is(e, caddy.ErrNotFound) {
			payload := value
			for j := len(dest) - 1; j > pos; j-- {
				payload = map[string]any{dest[j]: payload}
			}
			return caddy.Decision{Method: "POST", Path: dest[:pos+1], Body: payload}, nil
		}
		if e != nil {
			return caddy.Decision{}, e
		}
		if pos == len(dest)-1 {
			if reflect.DeepEqual(v, value) {
				return caddy.Decision{Done: true}, nil
			}
			return caddy.Decision{Method: "PATCH", Path: dest, Body: value}, nil
		}
	}
	return caddy.Decision{}, errors.New("invalid setting path")
}

func (r *settingsResource) write(ctx context.Context, g getter, creating bool) (settingsModel, error) {
	m, p, want, e := r.desired(ctx, g)
	if e != nil {
		return m, e
	}
	e = r.client.Mutate(ctx, caddy.Path{"config"}, func(s caddy.Snapshot) (caddy.Decision, error) {
		if r.kind == "binding" {
			var e error
			p, want, e = r.location(s, m)
			if e != nil {
				return caddy.Decision{}, e
			}
		}
		live, err := caddy.At(s.Value, p[len(s.Path):])
		if err != nil && !errors.Is(err, caddy.ErrNotFound) {
			return caddy.Decision{}, err
		}
		if creating && err == nil && !s.AfterWrite {
			return caddy.Decision{}, errors.New("setting already exists; refusing duplicate ownership")
		}
		if r.kind == "metrics" && err == nil {
			obj, ok := live.(map[string]any)
			if !ok {
				return caddy.Decision{}, errors.New("metrics is not an object")
			}
			copy := caddy.Clone(obj).(map[string]any)
			copy["per_host"] = want.(map[string]any)["per_host"]
			want = copy
		}
		if r.kind == "metrics" && err == nil {
			return settingDecision(s, append(append(caddy.Path(nil), p...), "per_host"), want.(map[string]any)["per_host"])
		}
		return settingDecision(s, p, want)
	})
	return m, e
}
func (r *settingsResource) Create(ctx context.Context, q resource.CreateRequest, p *resource.CreateResponse) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	m, e := r.write(ctx, q.Plan, true)
	if e != nil && !errors.Is(e, caddy.ErrUncertainWrite) {
		p.Diagnostics.AddError("Caddy setting create failed", e.Error())
		return
	}
	if r.kind == "metrics" {
		var b types.Bool
		q.Plan.GetAttribute(ctx, path.Root("per_host"), &b)
		p.Diagnostics.Append(p.State.Set(ctx, metricsModel{m.ID, b})...)
	} else {
		p.Diagnostics.Append(p.State.Set(ctx, m)...)
	}
	if e != nil {
		p.Diagnostics.AddError("Uncertain setting create", e.Error())
	}
}
func (r *settingsResource) Update(ctx context.Context, q resource.UpdateRequest, p *resource.UpdateResponse) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	m, e := r.write(ctx, q.Plan, false)
	if e != nil {
		p.Diagnostics.AddError("Caddy setting update failed", e.Error())
		return
	}
	if r.kind == "metrics" {
		var b types.Bool
		q.Plan.GetAttribute(ctx, path.Root("per_host"), &b)
		p.Diagnostics.Append(p.State.Set(ctx, metricsModel{m.ID, b})...)
	} else {
		p.Diagnostics.Append(p.State.Set(ctx, m)...)
	}
}
func (r *settingsResource) Read(ctx context.Context, q resource.ReadRequest, p *resource.ReadResponse) {
	m, dest, _, e := r.desired(ctx, q.State)
	if e != nil {
		p.Diagnostics.AddError("Invalid setting state", e.Error())
		return
	}
	s, e := r.client.Get(ctx, caddy.Path{"config"})
	if e != nil {
		p.Diagnostics.AddError("Caddy setting read failed", e.Error())
		return
	}
	if r.kind == "binding" {
		server, err := lookup(s, managed{kind: "server", id: m.Server.ValueString()})
		e = err
		if e == nil {
			dest = append(append(caddy.Path(nil), server.Path...), "logs", "logger_names", m.Hostname.ValueString())
		}
	}
	if errors.Is(e, caddy.ErrNotFound) {
		p.State.RemoveResource(ctx)
		return
	}
	if e != nil {
		p.Diagnostics.AddError("Caddy binding conflict", e.Error())
		return
	}
	v, e := caddy.At(s.Value, dest[len(s.Path):])
	if errors.Is(e, caddy.ErrNotFound) {
		p.State.RemoveResource(ctx)
		return
	}
	if e != nil {
		p.Diagnostics.AddError("Caddy setting read failed", e.Error())
		return
	}
	if r.kind == "metrics" {
		obj, ok := v.(map[string]any)
		if !ok {
			p.Diagnostics.AddError("Invalid metrics", "Expected object.")
			return
		}
		b, _ := obj["per_host"].(bool)
		p.Diagnostics.Append(p.State.Set(ctx, metricsModel{m.ID, types.BoolValue(b)})...)
		return
	}
	names, ok := v.([]any)
	name := ""
	if ok && len(names) == 1 {
		name, ok = names[0].(string)
	} else {
		ok = false
	}
	if !ok {
		p.Diagnostics.AddError("Invalid binding", "Expected one log name.")
		return
	}
	raw, e := caddy.At(s.Value, []string{"logging", "logs", name})
	if e != nil {
		p.Diagnostics.AddError("Invalid log reference", e.Error())
		return
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		p.Diagnostics.AddError("Invalid log reference", "Expected named log object.")
		return
	}
	id, _ := obj["@id"].(string)
	if id == "" {
		p.Diagnostics.AddError("Missing log ID", "Binding target must have @id.")
		return
	}
	m.Log = types.StringValue(id)
	p.Diagnostics.Append(p.State.Set(ctx, m)...)
}
func (r *settingsResource) Delete(ctx context.Context, q resource.DeleteRequest, p *resource.DeleteResponse) {
	m, dest, _, e := r.desired(ctx, q.State)
	if e != nil {
		p.Diagnostics.AddError("Invalid setting state", e.Error())
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	e = r.client.Mutate(ctx, caddy.Path{"config"}, func(s caddy.Snapshot) (caddy.Decision, error) {
		if r.kind == "binding" {
			server, err := lookup(s, managed{kind: "server", id: m.Server.ValueString()})
			if errors.Is(err, caddy.ErrNotFound) {
				return caddy.Decision{Done: true}, nil
			}
			if err != nil {
				return caddy.Decision{}, err
			}
			dest = append(append(caddy.Path(nil), server.Path...), "logs", "logger_names", m.Hostname.ValueString())
		}
		_, e := caddy.At(s.Value, dest[len(s.Path):])
		if errors.Is(e, caddy.ErrNotFound) {
			return caddy.Decision{Done: true}, nil
		}
		if e != nil {
			return caddy.Decision{}, e
		}
		return caddy.Decision{Method: "DELETE", Path: dest}, nil
	})
	if e != nil {
		p.Diagnostics.AddError("Caddy setting delete failed", e.Error())
	}
}
