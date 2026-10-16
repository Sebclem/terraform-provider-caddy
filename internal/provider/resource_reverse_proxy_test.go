package provider

import (
	"fmt"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/testutil"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxySelectiveUpdate(t *testing.T) {
	ctx, c, f := connection(t)
	port := testutil.FreePort(t)
	s := managed{kind: "server", id: "srv", name: "main", fields: map[string]any{"listen": []any{fmt.Sprintf("127.0.0.1:%d", port)}}}
	if e := createObject(ctx, c, s); e != nil {
		t.Fatal(e)
	}
	r := managed{kind: "route", id: "route", parent: "srv", fields: map[string]any{}}
	if e := createObject(ctx, c, r); e != nil {
		t.Fatal(e)
	}
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("A")) }))
	defer a.Close()
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("B")) }))
	defer b.Close()
	p := managed{kind: "proxy", id: "proxy", parent: "route", fields: map[string]any{"upstreams": []any{map[string]any{"dial": strings.TrimPrefix(a.URL, "http://")}}}}
	if e := createObject(ctx, c, p); e != nil {
		t.Fatal(e)
	}
	check := func(want string) {
		t.Helper()
		resp, e := http.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if string(body) != want {
			t.Fatalf("got %s want %s", body, want)
		}
	}
	check("A")
	snapshot, _ := c.Get(ctx, caddy.Path{"config"})
	if e := c.Write(ctx, "POST", caddy.Path{"id", "proxy", "upstreams", "0", "@id"}, "nested-upstream", snapshot.ETag); e != nil {
		t.Fatal(e)
	}
	snapshot, _ = c.Get(ctx, caddy.Path{"config"})
	if e := c.Write(ctx, "POST", caddy.Path{"id", "proxy", "upstreams", "0", "max_requests"}, 16, snapshot.ETag); e != nil {
		t.Fatal(e)
	}
	p.fields["upstreams"] = []any{map[string]any{"dial": strings.TrimPrefix(b.URL, "http://")}}
	f.ResetRequests()
	if e := updateObject(ctx, c, p, Ownership{"upstreams": true}); e != nil {
		t.Fatal(e)
	}
	check("B")
	snapshot, _ = c.Get(ctx, caddy.Path{"config"})
	nested, e := caddy.Resolve(snapshot, "nested-upstream")
	if e != nil || nested.Value["max_requests"] == nil {
		t.Fatal("nested content lost", e)
	}

	writes := 0
	for _, r := range f.Requests() {
		if r.Method != "GET" {
			writes++
			if r.Path != "/id/proxy/upstreams/0/dial" {
				t.Fatalf("not selective: %+v", r)
			}
		}
	}
	if writes != 1 {
		t.Fatalf("writes=%d", writes)
	}
}
