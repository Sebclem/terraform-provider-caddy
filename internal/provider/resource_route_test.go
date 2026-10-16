package provider

import (
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"testing"
)

func TestRouteLifecycle(t *testing.T) {
	ctx, c, _ := connection(t)
	server := managed{kind: "server", id: "srv", name: "main", fields: map[string]any{"listen": []any{"127.0.0.1:0"}}}
	if e := createObject(ctx, c, server); e != nil {
		t.Fatal(e)
	}
	r := managed{kind: "route", id: "route", parent: "srv", fields: map[string]any{"match": []any{map[string]any{"host": []any{"app.test"}}}, "terminal": true}}
	if e := createObject(ctx, c, r); e != nil {
		t.Fatal(e)
	}
	s, _ := c.Get(ctx, caddy.Path{"config"})
	if _, e := caddy.Resolve(s, "route"); e != nil {
		t.Fatal("route not created", e)
	}
}
func TestRouteImport(t *testing.T) {
	ctx, c, _ := connection(t)
	server := managed{kind: "server", id: "srv", name: "main", fields: map[string]any{"listen": []any{"127.0.0.1:0"}}}
	if e := createObject(ctx, c, server); e != nil {
		t.Fatal(e)
	}
	route := managed{kind: "route", id: "r", parent: "srv", fields: map[string]any{}}
	if e := createObject(ctx, c, route); e != nil {
		t.Fatal(e)
	}
	s, _ := c.Get(ctx, caddy.Path{"config"})
	if _, e := lookup(s, managed{kind: "route", id: "r", parent: "wrong-parent"}); e == nil {
		t.Fatal("wrong parent accepted")
	}
	if _, e := lookup(s, managed{kind: "proxy", id: "r"}); e == nil {
		t.Fatal("wrong type accepted")
	}
}
