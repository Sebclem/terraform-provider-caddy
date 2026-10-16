package provider

import (
	"context"
	"fmt"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/testutil"
	"testing"
)

func connection(t *testing.T) (context.Context, *caddy.Client, *testutil.Fixture) {
	t.Helper()
	f := testutil.StartCaddy(t)
	c, e := caddy.NewClient(caddy.Options{Endpoint: f.Endpoint, ConflictRetries: 5})
	if e != nil {
		t.Fatal(e)
	}
	return context.Background(), c, f
}
func TestServerLifecycle(t *testing.T) {
	ctx, c, f := connection(t)
	m := managed{kind: "server", id: "srv", name: "main", fields: map[string]any{"listen": []any{"127.0.0.1:0"}}}
	if e := createObject(ctx, c, m); e != nil {
		t.Fatal(e)
	}
	s, e := c.Get(ctx, caddy.Path{"config"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = caddy.Resolve(s, "srv"); e != nil {
		t.Fatalf("server not created: %v", e)
	}
	if e = createObject(ctx, c, m); e == nil {
		t.Fatal("collision adopted")
	}
	s, e = c.Get(ctx, caddy.Path{"config"})
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Write(ctx, "POST", caddy.Path{"id", "srv", "routes"}, []any{map[string]any{"@id": "unmanaged-route"}}, s.ETag); e != nil {
		t.Fatal(e)
	}
	m.fields["listen"] = []any{fmt.Sprintf("127.0.0.1:%d", testutil.FreePort(t))}
	f.ResetRequests()
	if e = updateObject(ctx, c, m, Ownership{"listen": true}); e != nil {
		t.Fatal(e)
	}
	for _, r := range f.Requests() {
		if r.Method != "GET" && r.Path != "/id/srv/listen" {
			t.Fatalf("broad write %+v", r)
		}
	}
	s, e = c.Get(ctx, caddy.Path{"config"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = caddy.Resolve(s, "unmanaged-route"); e != nil {
		t.Fatal("listener update removed child", e)
	}
	if e = deleteObject(ctx, c, m); e != nil {
		t.Fatal(e)
	}
	if e = deleteObject(ctx, c, m); e != nil {
		t.Fatal(e)
	}
}
