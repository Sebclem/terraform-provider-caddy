package provider

import (
	"context"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	ps "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"time"
)

type Provider struct{ version string }
type providerModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	CA       types.String `tfsdk:"ca_file"`
	Cert     types.String `tfsdk:"client_certificate_file"`
	Key      types.String `tfsdk:"client_key_file"`
	Timeout  types.String `tfsdk:"request_timeout"`
	Retries  types.Int64  `tfsdk:"conflict_retries"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &Provider{version: version} }
}
func (p *Provider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "caddy"
	resp.Version = p.version
}
func (p *Provider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = ps.Schema{Description: "Manages live Caddy JSON configuration through typed resources.", Attributes: map[string]ps.Attribute{
		"endpoint": ps.StringAttribute{Required: true, Description: "HTTP or HTTPS origin of the externally managed Caddy admin API."},
		"ca_file":  ps.StringAttribute{Optional: true, Description: "Custom CA PEM file for HTTPS verification."}, "client_certificate_file": ps.StringAttribute{Optional: true, Description: "Mutual TLS client certificate PEM file."}, "client_key_file": ps.StringAttribute{Optional: true, Description: "Mutual TLS private key PEM file."}, "request_timeout": ps.StringAttribute{Optional: true, Description: "Request timeout duration; defaults to 30s."}, "conflict_retries": ps.Int64Attribute{Optional: true, Description: "Conditional-write retries; defaults to 5."}}}
}
func (p *Provider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var m providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if m.Endpoint.IsUnknown() {
		resp.Diagnostics.AddError("Unknown endpoint", "The Caddy API endpoint must be known before configuration.")
		return
	}
	timeout := 30 * time.Second
	if !m.Timeout.IsNull() {
		v, e := time.ParseDuration(m.Timeout.ValueString())
		if e != nil || v <= 0 {
			resp.Diagnostics.AddError("Invalid request timeout", "Provide a positive duration, for example 30s.")
			return
		}
		timeout = v
	}
	retries := 5
	if !m.Retries.IsNull() {
		if m.Retries.ValueInt64() < 0 || m.Retries.ValueInt64() > 100 {
			resp.Diagnostics.AddError("Invalid conflict retries", "Use a retry limit between 0 and 100.")
			return
		}
		retries = int(m.Retries.ValueInt64())
	}
	c, e := caddy.NewClient(caddy.Options{Endpoint: m.Endpoint.ValueString(), CAFile: m.CA.ValueString(), ClientCertificateFile: m.Cert.ValueString(), ClientKeyFile: m.Key.ValueString(), RequestTimeout: timeout, ConflictRetries: retries})
	if e != nil {
		resp.Diagnostics.AddError("Invalid Caddy connection", e.Error())
		return
	}
	resp.ResourceData = c
}
func (p *Provider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{NewHTTPServerResource, NewHTTPRouteResource, NewReverseProxyResource}
}
func (p *Provider) DataSources(context.Context) []func() datasource.DataSource { return nil }
