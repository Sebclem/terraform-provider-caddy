package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/testutil"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccDisableProxyTLS(t *testing.T) {
	r := newRunner(t)
	b := backend(t, "plain")
	cfg := strings.Replace(r.config(b.URL, "", ""), " upstream {", " transport = {tls = true}\n upstream {", 1)
	r.write(cfg)
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.write(strings.Replace(cfg, "tls = true", "tls = false", 1))
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.checkResponse("plain")
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
func TestAccNestedOwnershipAdoption(t *testing.T) {
	r := newRunner(t)
	b := backend(t, "plain")
	cfg := strings.Replace(r.config(b.URL, "", ""), " upstream {", " health = {uri = \"/health\", timeout = \"10s\"}\n upstream {", 1)
	r.write(cfg)
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	id := r.values()["caddy_handler_reverse_proxy.app"]["id"].(string)
	omit := strings.Replace(cfg, ", timeout = \"10s\"", "", 1)
	r.write(omit)
	r.run(0, "state", "rm", "caddy_handler_reverse_proxy.app")
	r.run(0, "import", "-input=false", "-no-color", "caddy_handler_reverse_proxy.app", id)
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.write(cfg)
	r.f.ResetRequests()
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.noWrites()
	r.write(omit)
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	c, _ := caddy.NewClient(caddy.Options{Endpoint: r.f.Endpoint})
	s, _ := c.Get(context.Background(), caddy.Path{"id", id})
	if v, e := caddy.At(s.Value, []string{"health_checks", "active", "timeout"}); e == nil {
		t.Fatalf("managed then removed timeout survives: %v", v)
	}
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}

func TestAccHTTPSOwnershipRemoval(t *testing.T) {
	r := newRunner(t)
	httpPort := testutil.FreePort(t)
	bootstrap, e := http.Post(r.f.Direct+"/config/apps/http", "application/json", strings.NewReader(fmt.Sprintf(`{"http_port":%d,"https_port":%d}`, httpPort, r.port)))
	if e != nil {
		t.Fatal(e)
	}
	bootstrap.Body.Close()
	cp, kp := certificate(t, t.TempDir(), "app.test")
	b := backend(t, "safe")
	cfg := r.config(b.URL, `match = [{hosts = ["app.test"]}]`, fmt.Sprintf("resource \"caddy_tls_certificate\" \"app\" {\ncertificate = %q\nkey = %q\n}\n", cp, kp))
	cfg = strings.Replace(cfg, ` name = "main"`, " name = \"main\"\n https = true\n depends_on = [caddy_tls_certificate.app]", 1)
	r.write(cfg)
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	id := r.values()["caddy_http_server.main"]["id"].(string)
	// Reject unsafe mutations before Caddy can enable certificate automation.
	r.f.SetFault(func(w http.ResponseWriter, q *http.Request) bool {
		unsafe := q.Method == "DELETE" && q.URL.Path == "/id/"+id+"/automatic_https"
		if q.Method == "PATCH" && q.URL.Path == "/id/"+id {
			data, _ := io.ReadAll(q.Body)
			q.Body = io.NopCloser(bytes.NewReader(data))
			var obj map[string]any
			json.Unmarshal(data, &obj)
			auto, _ := obj["automatic_https"].(map[string]any)
			unsafe = auto["disable"] != true && auto["disable_certificates"] != true
		}
		if unsafe {
			w.WriteHeader(400)
			return true
		}
		return false
	})
	r.write(strings.Replace(cfg, " https = true\n", "", 1))
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.checkResponse("safe")
	c, _ := caddy.NewClient(caddy.Options{Endpoint: r.f.Endpoint})
	s, e := c.Get(context.Background(), caddy.Path{"id", id})
	if e != nil {
		t.Fatal(e)
	}
	v, e := caddy.At(s.Value, []string{"automatic_https", "disable"})
	if e != nil || v != true {
		t.Fatal("omitting HTTPS must keep automation disabled", v, e)
	}
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
func TestAccMissingBindingLogRecovery(t *testing.T) {
	for _, mode := range []string{"generated", "explicit"} {
		t.Run(mode, func(t *testing.T) {
			r := newRunner(t)
			b := backend(t, "plain")
			identity := ""
			if mode == "explicit" {
				identity = `object_id = "fixed-log"`
			}
			cfg := r.config(b.URL, "", `resource "caddy_log" "app" {
 name = "site"
 `+identity+`
}
resource "caddy_log_binding" "app" {
 server_id = caddy_http_server.main.id
 hostname = "app.test"
 log_id = caddy_log.app.id
}`)
			r.write(cfg)
			r.run(0, "init", "-input=false", "-no-color")
			r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
			id := r.values()["caddy_log.app"]["id"].(string)
			c, _ := caddy.NewClient(caddy.Options{Endpoint: r.f.Endpoint})
			snapshot, _ := c.Get(context.Background(), caddy.Path{"config"})
			if e := c.Write(context.Background(), "DELETE", caddy.Path{"id", id}, nil, snapshot.ETag); e != nil {
				t.Fatal(e)
			}
			r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
			r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
			r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
		})
	}
}
func TestAccNestedHeaderContainerRemoval(t *testing.T) {
	r := newRunner(t)
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		fmt.Fprint(w, q.Header.Get("X-Test")+"|"+q.Header.Get("X-Add"))
	}))
	defer b.Close()
	cfg := strings.Replace(r.config(b.URL, "", ""), " upstream {", ` request_headers = { set = { "X-Test" = ["A"] }, add = { "X-Add" = ["B"] } }
 upstream {`, 1)
	r.write(cfg)
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.checkResponse("A|B")
	r.write(strings.Replace(cfg, `set = { "X-Test" = ["A"] }, `, "", 1))
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.checkResponse("|B")
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
