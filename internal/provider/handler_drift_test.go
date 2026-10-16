package provider

import (
	"fmt"
	"testing"

	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestManagedHandlerDrift(t *testing.T) {
	for _, kind := range []string{"error_pages", "forward_auth"} {
		t.Run(kind, func(t *testing.T) {
			ctx, c, _ := connection(t)
			objects := []managed{{kind: "server", id: "srv", name: "main", fields: map[string]any{"listen": []any{"127.0.0.1:0"}}}, {kind: "route", id: "route", parent: "srv", fields: map[string]any{}}, {kind: "subroute", id: "sub", parent: "route", fields: map[string]any{}}, {kind: "route", id: "child", parent: "sub", fields: map[string]any{}}}
			for _, m := range objects {
				if e := createObject(ctx, c, m); e != nil {
					t.Fatal(e)
				}
			}
			m := managed{kind: kind, id: "managed", parent: "sub", fields: errorPagesFields("managed", "/srv", "/{http.error.status_code}.html")}
			owned := Ownership{"handle": true}
			target := caddy.Path{"id", "managed-files", "pass_thru"}
			var bad any = false
			if kind == "forward_auth" {
				m.parent = "child"
				m.fields = forwardAuthFields(map[string]attr.Value{"upstream": types.StringValue("127.0.0.1:9999"), "uri": types.StringValue("/auth"), "copy_headers": types.ListValueMust(types.StringType, []attr.Value{})})
				owned = Ownership{"upstreams": true, "rewrite": true, "headers": true, "handle_response": true}
				target = caddy.Path{"id", "managed", "rewrite", "method"}
				bad = "POST"
			}
			if e := createObject(ctx, c, m); e != nil {
				t.Fatal(e)
			}
			r := &caddyResource{kind: kind}
			s, e := c.Get(ctx, caddy.Path{"config"})
			if e != nil {
				t.Fatal(e)
			}
			state, e := r.live(ctx, s, model{ID: types.StringValue("managed"), Parent: types.StringValue(m.parent), Revision: types.StringValue(ownershipRevision(owned))})
			if e != nil {
				t.Fatal(e)
			}
			drift := func() {
				t.Helper()
				s, e = c.Get(ctx, caddy.Path{"config"})
				if e != nil {
					t.Fatal(e)
				}
				if e = c.Write(ctx, "PATCH", target, bad, s.ETag); e != nil {
					t.Fatal(e)
				}
			}
			drift()
			s, e = c.Get(ctx, caddy.Path{"config"})
			if e != nil {
				t.Fatal(e)
			}
			observed, e := r.live(ctx, s, state)
			if e != nil {
				t.Fatal("managed drift must remain readable", e)
			}
			if observed.Revision.Equal(state.Revision) {
				t.Fatal("malformed internal sequence must schedule repair")
			}
			if e = updateObject(ctx, c, m, owned); e != nil {
				t.Fatal("repair failed", e)
			}
			s, e = c.Get(ctx, caddy.Path{"config"})
			if e != nil {
				t.Fatal(e)
			}
			if _, e = r.live(ctx, s, state); e != nil {
				t.Fatal(e)
			}
			drift()
			if e = deleteObject(ctx, c, m); e != nil {
				t.Fatal("drift must not block deletion", e)
			}
		})
	}
}
func TestErrorPagesImportChildIdentities(t *testing.T) {
	for _, mode := range []string{"arbitrary", "absent"} {
		t.Run(mode, func(t *testing.T) {
			obj := errorPagesFields("root", "/srv", "/{http.error.status_code}.html")
			obj["@id"] = "root"
			for i, v := range obj["handle"].([]any) {
				m := v.(map[string]any)
				if mode == "absent" {
					delete(m, "@id")
				} else {
					m["@id"] = fmt.Sprintf("external-%d", i)
				}
			}
			if e := recognizeTemplate("error_pages", "root", caddy.Clone(obj).(map[string]any)); e != nil {
				t.Fatal(e)
			}
			desired := errorPagesFields("root", "/new-root", "/{http.error.status_code}.html")
			preserveInternalIDs(obj, desired)
			for i, v := range desired["handle"].([]any) {
				if v.(map[string]any)["@id"] != obj["handle"].([]any)[i].(map[string]any)["@id"] {
					t.Fatal("imported child identity changed")
				}
			}
		})
	}
}
