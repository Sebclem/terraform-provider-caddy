package acceptance

import (
	"context"
	"fmt"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"net/http"
	"strings"
	"testing"
)

func TestAccRejectedUpdate(t *testing.T) {
	r := newRunner(t)
	a := backend(t, "A")
	body := r.config(a.URL, "", "")
	r.write(body)
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	id := r.values()["caddy_http_server.main"]["id"]
	bad := strings.Replace(body, fmt.Sprintf("127.0.0.1:%d", r.port), "127.0.0.1:invalid-port", 1)
	r.write(bad)
	r.run(1, "apply", "-auto-approve", "-input=false", "-no-color")
	r.checkResponse("A")
	if r.values()["caddy_http_server.main"]["id"] != id {
		t.Fatal("rejected update changed identity")
	}
	r.write(body)
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.f.SetFault(func(w http.ResponseWriter, req *http.Request) bool {
		if req.Method == "GET" {
			w.WriteHeader(503)
			return true
		}
		return false
	})
	r.run(1, "plan", "-input=false", "-no-color")
	if len(r.values()) != 3 {
		t.Fatal("endpoint failure removed resources from state")
	}
	r.f.SetFault(nil)
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
func TestAccUnmanagedFields(t *testing.T) {
	r := newRunner(t)
	a := backend(t, "A")
	r.write(r.config(a.URL, "", ""))
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	id := r.values()["caddy_http_route.app"]["id"].(string)
	c, _ := caddy.NewClient(caddy.Options{Endpoint: r.f.Endpoint})
	s, _ := c.Get(context.Background(), caddy.Path{"config"})
	if e := c.Write(context.Background(), "POST", caddy.Path{"id", id, "group"}, "foreign", s.ETag); e != nil {
		t.Fatal(e)
	}
	r.f.ResetRequests()
	r.run(0, "apply", "-refresh-only", "-auto-approve", "-input=false", "-no-color")
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.noWrites()
	r.write(r.config(a.URL, "terminal = true", ""))
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	s, _ = c.Get(context.Background(), caddy.Path{"config"})
	o, _ := caddy.Resolve(s, id)
	if o.Value["group"] != "foreign" {
		t.Fatal("unowned group erased")
	}
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
