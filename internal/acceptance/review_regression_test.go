package acceptance

import (
	"context"
	"fmt"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/testutil"
	"strings"
	"testing"
)

func TestAccParentReplacement(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		name := "generated"
		if explicit {
			name = "explicit"
		}
		t.Run(name, func(t *testing.T) {
			r := newRunner(t)
			a := backend(t, "A")
			body := r.config(a.URL, "", "")
			if explicit {
				body = strings.Replace(body, `name = "main"`, `name = "main"`+"\n object_id = \"fixed-server\"", 1)
			}
			r.write(body)
			r.run(0, "init", "-input=false", "-no-color")
			r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
			r.write(strings.Replace(body, `name = "main"`, `name = "renamed"`, 1))
			r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
			r.checkResponse("A")
			r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
			r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
		})
	}
}
func TestAccMissingAnchorRecovery(t *testing.T) {
	r := newRunner(t)
	a := backend(t, "A")
	extra := `resource "caddy_http_route" "anchor" {
 server_id = caddy_http_server.main.id
 object_id = "anchor"
}
`
	r.write(r.config(a.URL, `before_id = caddy_http_route.anchor.id`, extra))
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	c, _ := caddy.NewClient(caddy.Options{Endpoint: r.f.Endpoint})
	s, _ := c.Get(context.Background(), caddy.Path{"config"})
	if e := c.Write(context.Background(), "DELETE", caddy.Path{"id", "anchor"}, nil, s.ETag); e != nil {
		t.Fatal(e)
	}
	r.write(r.config(a.URL, "", ""))
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
func TestAccRouteParentReplacement(t *testing.T) {
	r := newRunner(t)
	a := backend(t, "A")
	otherPort := testutil.FreePort(t)
	extra := fmt.Sprintf(`resource "caddy_http_server" "other" {
 name = "other"
 listen = ["127.0.0.1:%d"]
}
`, otherPort)
	body := r.config(a.URL, `object_id = "fixed-route"`, extra)
	r.write(body)
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.write(strings.Replace(body, "server_id = caddy_http_server.main.id", "server_id = caddy_http_server.other.id", 1))
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.port = otherPort
	r.checkResponse("A")
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
