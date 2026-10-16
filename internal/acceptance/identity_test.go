package acceptance

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAccIDPlan(t *testing.T) {
	r := newRunner(t)
	a := backend(t, "A")
	r.write(r.config(a.URL, `object_id = "route-one"`, ""))
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	v := r.values()
	if v["caddy_http_route.app"]["id"] != "route-one" {
		t.Fatal("explicit identity ignored")
	}
	r.write(r.config(a.URL, "", ""))
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.write(r.config(a.URL, `object_id = "route-two"`, ""))
	r.run(2, "plan", "-input=false", "-no-color", "-detailed-exitcode", "-out=replace.plan")
	out := r.run(0, "show", "-json", "replace.plan")
	var p struct {
		Changes []struct {
			Address string
			Change  struct {
				Actions []string `json:"actions"`
			} `json:"change"`
		} `json:"resource_changes"`
	}
	json.Unmarshal([]byte(out.Stdout), &p)
	found := false
	for _, c := range p.Changes {
		if c.Address == "caddy_http_route.app" {
			found = true
			if strings.Join(c.Change.Actions, ",") != "delete,create" {
				t.Fatalf("identity action %v", c.Change.Actions)
			}
		}
	}
	if !found {
		t.Fatal("replacement missing")
	}
	r.write(r.config(a.URL, `object_id = "route-one"`, ""))
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
func TestAccInvalidImport(t *testing.T) {
	r := newRunner(t)
	a := backend(t, "A")
	r.write(r.config(a.URL, "", ""))
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	v := r.values()
	r.run(0, "state", "rm", "caddy_handler_reverse_proxy.app")
	r.f.ResetRequests()
	r.run(1, "import", "-input=false", "-no-color", "caddy_handler_reverse_proxy.app", v["caddy_http_route.app"]["id"].(string))
	r.noWrites()
	r.run(1, "import", "-input=false", "-no-color", "caddy_handler_reverse_proxy.app", "missing-id")
	r.noWrites()
	r.run(0, "import", "-input=false", "-no-color", "caddy_handler_reverse_proxy.app", v["caddy_handler_reverse_proxy.app"]["id"].(string))
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
