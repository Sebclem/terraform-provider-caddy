package acceptance

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestAccMetrics(t *testing.T) {
	r := newRunner(t)
	b := backend(t, "metric")
	cfg := r.config(b.URL, `match = [{hosts = ["app.test"]}]`, `resource "caddy_http_metrics" "main" {per_host = true}`)
	r.write(cfg)
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.checkResponse("metric")
	resp, e := http.Get(r.f.Direct + "/metrics")
	if e != nil {
		t.Fatal(e)
	}
	data := readBody(t, resp)
	if !strings.Contains(data, `host="app.test"`) || !strings.Contains(data, "caddy_http_requests_total") {
		t.Fatalf("missing per-host observation: %s", data)
	}
	r.f.ResetRequests()
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.noWrites()
	r.run(1, "import", "-input=false", "-no-color", "caddy_http_metrics.main", "http-metrics")
	r.write(strings.Replace(cfg, "per_host = true", "per_host = false", 1))
	r.f.ResetRequests()
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	for _, q := range r.f.Requests() {
		if q.Method != "GET" && q.Path != "/config/apps/http/metrics/per_host" {
			t.Fatal(fmt.Sprintf("unexpected mutation %+v", q))
		}
	}
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
