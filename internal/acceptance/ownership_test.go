package acceptance

import (
	"context"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"testing"
)

func TestAccOwnership(t *testing.T) {
	r := newRunner(t)
	a := backend(t, "A")
	opts := `terminal = true
 group = "owned"
 match = [{ hosts = ["app.test"] }]`
	r.write(r.config(a.URL, opts, ""))
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	values := r.values()
	id := values["caddy_http_route.app"]["id"].(string)
	// Import observes fields. Matching configuration then establishes ownership without Caddy writes.
	r.run(0, "state", "rm", "caddy_http_route.app")
	r.run(0, "import", "-input=false", "-no-color", "caddy_http_route.app", id)
	r.f.ResetRequests()
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.noWrites()
	r.write(r.config(a.URL, "", ""))
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	c, _ := caddy.NewClient(caddy.Options{Endpoint: r.f.Endpoint})
	s, e := c.Get(context.Background(), caddy.Path{"config"})
	if e != nil {
		t.Fatal(e)
	}
	o, _ := caddy.Resolve(s, id)
	for _, k := range []string{"terminal", "group", "match"} {
		if _, ok := o.Value[k]; ok {
			t.Fatalf("removed configured field %s retained: %+v", k, o.Value)
		}
	}
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
