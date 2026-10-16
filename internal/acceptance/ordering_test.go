package acceptance

import (
	"context"
	"fmt"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"strings"
	"testing"
)

func TestAccOrdering(t *testing.T) {
	r := newRunner(t)
	a := backend(t, "A")
	extra := `resource "caddy_http_route" "anchor" {
 server_id = caddy_http_server.main.id
 object_id = "anchor"
}
resource "caddy_http_route" "shared" {
 server_id = caddy_http_server.main.id
 object_id = "shared"
 before_id = caddy_http_route.anchor.id
}
`
	opts := `before_id = caddy_http_route.anchor.id
match = [{ hosts = ["app.test"] }]`
	r.write(r.config(a.URL, opts, extra))
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	v := r.values()
	id := v["caddy_http_route.app"]["id"].(string)
	server := v["caddy_http_server.main"]["id"].(string)
	c, _ := caddy.NewClient(caddy.Options{Endpoint: r.f.Endpoint, ConflictRetries: 5})
	ctx := context.Background()
	s, _ := c.Get(ctx, caddy.Path{"config"})
	srv, _ := caddy.Resolve(s, server)
	routes := srv.Value["routes"].([]any)
	out := []any{}
	for _, x := range routes {
		if x.(map[string]any)["@id"] == "anchor" {
			out = append(out, x)
		}
	}
	for _, x := range routes {
		if x.(map[string]any)["@id"] != "anchor" {
			out = append(out, x)
		}
	}
	if e := c.Write(ctx, "PATCH", caddy.Path{"id", server, "routes"}, out, s.ETag); e != nil {
		t.Fatal(e)
	}
	r.run(2, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	if r.values()["caddy_http_route.app"]["id"] != id {
		t.Fatal("move replaced identity")
	}
	r.write(r.config(a.URL, `after_id = caddy_http_route.anchor.id
match = [{ hosts = ["app.test"] }]`, extra))
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
	_ = fmt.Sprint(id)
}
func TestAccHandlerOrdering(t *testing.T) {
	r := newRunner(t)
	a := backend(t, "A")
	b := backend(t, "B")
	extra := fmt.Sprintf(`resource "caddy_handler_reverse_proxy" "second" {
 route_id = caddy_http_route.app.id
 object_id = "second"
 upstream {
  dial = %q
 }
}
`, strings.TrimPrefix(b.URL, "http://"))
	body := r.config(a.URL, "", extra)
	body = strings.Replace(body, "route_id = caddy_http_route.app.id", "route_id = caddy_http_route.app.id\n before_id = caddy_handler_reverse_proxy.second.id", 1)
	r.write(body)
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.checkResponse("A")
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	id := r.values()["caddy_handler_reverse_proxy.app"]["id"]
	r.write(strings.Replace(body, "before_id =", "after_id =", 1))
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.checkResponse("B")
	if r.values()["caddy_handler_reverse_proxy.app"]["id"] != id {
		t.Fatal("handler move changed identity")
	}
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
