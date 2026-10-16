package acceptance

import (
	"context"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAccRecovery(t *testing.T) {
	r := newRunner(t)
	a := backend(t, "A")
	var dropped atomic.Bool
	r.f.SetFault(func(w http.ResponseWriter, req *http.Request) bool {
		if req.Method != "POST" || !strings.HasSuffix(req.URL.Path, "/routes") || !dropped.CompareAndSwap(false, true) {
			return false
		}
		clone := req.Clone(req.Context())
		clone.RequestURI = ""
		clone.URL.Scheme = "http"
		clone.URL.Host = strings.TrimPrefix(r.f.Direct, "http://")
		clone.Host = clone.URL.Host
		resp, e := http.DefaultClient.Do(clone)
		if e != nil {
			t.Error(e)
			return false
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("fault setup Caddy status %d", resp.StatusCode)
		}
		conn, _, e := w.(http.Hijacker).Hijack()
		if e != nil {
			t.Error(e)
		} else {
			conn.Close()
		}
		return true
	})
	r.write(r.config(a.URL, "", ""))
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	if !dropped.Load() {
		t.Fatal("no response dropped")
	}
	r.checkResponse("A")
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	values := r.values()
	server := values["caddy_http_server.main"]["id"].(string)
	c, _ := caddy.NewClient(caddy.Options{Endpoint: r.f.Endpoint})
	s, _ := c.Get(context.Background(), caddy.Path{"config"})
	o, _ := caddy.Resolve(s, server)
	if len(o.Value["routes"].([]any)) != 1 {
		t.Fatal("duplicate route after lost response")
	}
	// Parent deletion intentionally removes descendants. Their subsequent destruction is idempotent.
	if e := c.Write(context.Background(), "DELETE", caddy.Path{"id", server}, nil, s.ETag); e != nil {
		t.Fatal(e)
	}
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
func TestAccUncertainCreateRetainsID(t *testing.T) {
	r := newRunner(t)
	a := backend(t, "A")
	var dropped atomic.Bool
	r.f.SetFault(func(w http.ResponseWriter, req *http.Request) bool {
		if req.Method == "GET" && dropped.Load() {
			w.WriteHeader(503)
			return true
		}
		if req.Method != "POST" || !strings.HasSuffix(req.URL.Path, "/routes") || !dropped.CompareAndSwap(false, true) {
			return false
		}
		clone := req.Clone(req.Context())
		clone.RequestURI = ""
		clone.URL.Scheme = "http"
		clone.URL.Host = strings.TrimPrefix(r.f.Direct, "http://")
		clone.Host = clone.URL.Host
		resp, e := http.DefaultClient.Do(clone)
		if e != nil {
			t.Error(e)
			return false
		}
		resp.Body.Close()
		conn, _, _ := w.(http.Hijacker).Hijack()
		conn.Close()
		return true
	})
	r.write(r.config(a.URL, "", ""))
	r.run(0, "init", "-input=false", "-no-color")
	r.run(1, "apply", "-auto-approve", "-input=false", "-no-color")
	values := r.values()
	if values["caddy_http_route.app"]["id"] == nil {
		t.Fatal("uncertain created route identity was discarded")
	}
	r.f.SetFault(nil)
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
