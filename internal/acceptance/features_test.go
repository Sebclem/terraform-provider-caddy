package acceptance

import (
	"context"
	"encoding/json"
	"fmt"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAccTypedFeatures(t *testing.T) {
	r := newRunner(t)
	type incoming struct{ method, body string }
	received := make(chan incoming, 10)
	probes := make(chan struct{}, 10)
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		if q.URL.Path == "/health" {
			select {
			case probes <- struct{}{}:
			default:
			}
		}
		if q.URL.Path != "/health" {
			b, _ := io.ReadAll(q.Body)
			received <- incoming{q.Method, string(b)}
		}
		fmt.Fprint(w, q.Host+"|"+q.Header.Get("Remote-User"))
	}))
	defer b.Close()
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		if q.Method != "GET" || q.URL.Path != "/auth" {
			t.Errorf("auth request %s %s", q.Method, q.URL)
		}
		w.Header().Set("Remote-User", "fixture")
		w.WriteHeader(204)
	}))
	defer auth.Close()
	config := fmt.Sprintf(`resource "caddy_http_server" "main" {
 name = "main"
 listen = ["127.0.0.1:%d"]
 trusted_proxies = ["127.0.0.0/8", "::1/128"]
}
resource "caddy_http_route" "site" { server_id = caddy_http_server.main.id }
resource "caddy_handler_subroute" "site" { route_id = caddy_http_route.site.id }
resource "caddy_http_route" "app" { subroute_id = caddy_handler_subroute.site.id }
resource "caddy_handler_forward_auth" "auth" {
 route_id = caddy_http_route.app.id
 upstream = %q
 uri = "/auth"
 copy_headers = ["Remote-User"]
}
resource "caddy_handler_reverse_proxy" "app" {
 route_id = caddy_http_route.app.id
 after_id = caddy_handler_forward_auth.auth.id
 upstream { dial = %q }
 request_headers = {set = { Host = ["override.test"] }}
 request_buffers = 36700160
 response_buffers = 36700160
 flush_interval = "-1ns"
 health = {uri = "/health", follow_redirects = true, fails = 3, timeout = "20s"}
}
`, r.port, strings.TrimPrefix(auth.URL, "http://"), strings.TrimPrefix(b.URL, "http://"))
	r.write(config)
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d/", r.port), strings.NewReader("body"))
	req.Header.Set("Remote-User", "forged")
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(data) != "override.test|fixture" {
		t.Fatalf("response %d %s", resp.StatusCode, data)
	}
	request := <-received
	if request.method != "POST" || request.body != "body" {
		t.Fatal("forward auth changed original request", request)
	}
	select {
	case <-probes:
	case <-time.After(5 * time.Second):
		t.Fatal("no active health probe observed")
	}
	ids := r.values()
	r.f.ResetRequests()
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.noWrites()
	r.write(strings.Replace(config, "20s", "21s", 1))
	r.f.ResetRequests()
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	for _, q := range r.f.Requests() {
		if q.Method != "GET" && q.Path != "/id/"+ids["caddy_handler_reverse_proxy.app"]["id"].(string)+"/health_checks/active/timeout" {
			t.Fatalf("broad write %+v", q)
		}
	}
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}

func TestAccAccessAndAuth(t *testing.T) {
	r := newRunner(t)
	app := backend(t, "application")
	// Fixed synthetic bcrypt hash for fixture-password, never a production credential.
	data, e := os.ReadFile("../../testdata/legacy/active.json")
	if e != nil {
		t.Fatal(e)
	}
	var legacy any
	json.Unmarshal(data, &legacy)
	var hash string
	var find func(any)
	find = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if s, ok := x["password"].(string); ok {
				hash = s
			}
			for _, v := range x {
				find(v)
			}
		case []any:
			for _, v := range x {
				find(v)
			}
		}
	}
	find(legacy)
	cfg := fmt.Sprintf(`resource "caddy_http_server" "main" {
 name = "main"
 listen = ["127.0.0.1:%d"]
 trusted_proxies = ["127.0.0.0/8"]
}
resource "caddy_http_route" "deny" {
 server_id = caddy_http_server.main.id
 match = [{ methods = ["POST", "PUT", "DELETE"], not = [{client_ip_ranges = ["10.0.0.0/8", "2001:db8::/32"]}] }]
}
resource "caddy_handler_error" "deny" {
 route_id = caddy_http_route.deny.id
 status_code = 403
 message = "Unauthorized"
}
resource "caddy_http_route" "app" {
 server_id = caddy_http_server.main.id
 after_id = caddy_http_route.deny.id
 match = [{ paths = ["/metrics"] }]
}
resource "caddy_handler_basic_auth" "auth" {
 route_id = caddy_http_route.app.id
 accounts = [{username = "fixture", password_hash = %q}]
}
resource "caddy_handler_reverse_proxy" "app" {
 route_id = caddy_http_route.app.id
 after_id = caddy_handler_basic_auth.auth.id
 upstream {dial = %q}
 transport = { tls = false }
}
`, r.port, hash, strings.TrimPrefix(app.URL, "http://"))
	r.write(cfg)
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	for _, tc := range []struct {
		method, ip string
		auth       bool
		status     int
	}{{"POST", "192.0.2.1", true, 403}, {"POST", "10.1.2.3", true, 200}, {"GET", "192.0.2.1", false, 401}, {"GET", "192.0.2.1", true, 200}, {"PUT", "2001:db8::1", true, 200}} {
		q, _ := http.NewRequest(tc.method, fmt.Sprintf("http://127.0.0.1:%d/metrics", r.port), nil)
		q.Header.Set("X-Forwarded-For", tc.ip)
		if tc.auth {
			q.SetBasicAuth("fixture", "fixture-password")
		}
		resp, e := http.DefaultClient.Do(q)
		if e != nil {
			t.Fatal(e)
		}
		resp.Body.Close()
		if resp.StatusCode != tc.status {
			t.Fatalf("%+v got %d", tc, resp.StatusCode)
		}
	}
	r.f.ResetRequests()
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.noWrites()
	for _, name := range []string{"caddy_handler_basic_auth.auth", "caddy_handler_error.deny"} {
		id := r.values()[name]["id"].(string)
		r.run(0, "state", "rm", name)
		r.run(0, "import", "-input=false", "-no-color", name, id)
	}
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.write(strings.Replace(cfg, `trusted_proxies = ["127.0.0.0/8"]`, "", 1))
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	q, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d/metrics", r.port), nil)
	q.Header.Set("X-Forwarded-For", "10.1.2.3")
	q.SetBasicAuth("fixture", "fixture-password")
	resp, e := http.DefaultClient.Do(q)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatal("untrusted peer spoofed client IP", resp.StatusCode)
	}
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
func TestAccForwardAuthOmittedHeaders(t *testing.T) {
	r := newRunner(t)
	b := backend(t, "pass")
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) { w.WriteHeader(204) }))
	defer auth.Close()
	cfg := r.config(b.URL, "", "")
	cfg = strings.Replace(cfg, `resource "caddy_handler_reverse_proxy" "app" {`, `resource "caddy_handler_forward_auth" "auth" {
 route_id = caddy_http_route.app.id
 upstream = "`+strings.TrimPrefix(auth.URL, "http://")+`"
 uri = "/auth"
}
resource "caddy_handler_reverse_proxy" "app" {
 after_id = caddy_handler_forward_auth.auth.id`, 1)
	r.write(cfg)
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.checkResponse("pass")
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
func TestAccProxyTLSSelectiveUpdate(t *testing.T) {
	r := newRunner(t)
	b := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) { fmt.Fprint(w, "tls-backend") }))
	defer b.Close()
	cfg := r.config(strings.Replace(b.URL, "https://", "http://", 1), "", "")
	cfg = strings.Replace(cfg, " upstream {", " transport = {tls = true, insecure_skip_verify = true}\n upstream {", 1)
	r.write(cfg)
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.checkResponse("tls-backend")
	id := r.values()["caddy_handler_reverse_proxy.app"]["id"].(string)
	c, e := caddy.NewClient(caddy.Options{Endpoint: r.f.Endpoint, RequestTimeout: time.Second})
	if e != nil {
		t.Fatal(e)
	}
	s, e := c.Get(context.Background(), caddy.Path{"config"})
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Write(context.Background(), "POST", caddy.Path{"id", id, "transport", "dial_timeout"}, "10s", s.ETag); e != nil {
		t.Fatal(e)
	}
	r.write(strings.Replace(cfg, "insecure_skip_verify = true", "insecure_skip_verify = false", 1))
	r.f.ResetRequests()
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	for _, q := range r.f.Requests() {
		if q.Method != "GET" && q.Path != "/id/"+id+"/transport/tls/insecure_skip_verify" {
			t.Fatalf("broad TLS write %+v", q)
		}
	}
	s, e = c.Get(context.Background(), caddy.Path{"id", id})
	if e != nil {
		t.Fatal(e)
	}
	if v, e := caddy.At(s.Value, []string{"transport", "dial_timeout"}); e != nil || v != "10s" {
		t.Fatal("unrelated transport setting lost", v, e)
	}
	resp, e := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", r.port))
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != 502 {
		t.Fatal("TLS verification bypass remained enabled", resp.StatusCode)
	}
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
func TestAccUnknownNestedSetting(t *testing.T) {
	r := newRunner(t)
	b := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) { fmt.Fprint(w, "unknown") }))
	defer b.Close()
	cfg := r.config(strings.Replace(b.URL, "https://", "http://", 1), "", "")
	cfg = strings.Replace(cfg, " upstream {", " transport = {tls = true, insecure_skip_verify = true, server_name = caddy_http_server.main.id}\n upstream {", 1)
	r.write(cfg)
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.checkResponse("unknown")
	r.f.ResetRequests()
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.noWrites()
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
