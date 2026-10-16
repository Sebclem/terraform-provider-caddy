package acceptance

import (
	"context"
	"encoding/json"
	"fmt"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/testutil"
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type runner struct {
	t        *testing.T
	bin, dir string
	f        *testutil.Fixture
	port     int
}

func newRunner(t *testing.T) *runner {
	t.Helper()
	bin := os.Getenv("IAC_BIN")
	if bin == "" {
		t.Skip("set IAC_BIN for CLI acceptance")
	}
	f := testutil.StartCaddy(t)
	dir := t.TempDir()
	provider := os.Getenv("PROVIDER_BIN")
	if provider == "" {
		provider, _ = filepath.Abs("../../bin/terraform-provider-caddy")
	}
	data, e := os.ReadFile(provider)
	if e != nil {
		t.Fatal(e)
	}
	mirror := filepath.Join(dir, "mirror")
	dest := filepath.Join(mirror, "local.sebclem.fr/sebclem/caddy/0.1.0/linux_amd64")
	os.MkdirAll(dest, 0755)
	if e = os.WriteFile(filepath.Join(dest, "terraform-provider-caddy_v0.1.0"), data, 0755); e != nil {
		t.Fatal(e)
	}
	conf := fmt.Sprintf("provider_installation {\n filesystem_mirror {\n path = %q\n include = [\"local.sebclem.fr/sebclem/caddy\"]\n }\n}\n", mirror)
	cfg := filepath.Join(dir, "cli.tfrc")
	os.WriteFile(cfg, []byte(conf), 0600)
	t.Setenv("TF_CLI_CONFIG_FILE", cfg)
	return &runner{t, bin, dir, f, testutil.FreePort(t)}
}
func (r *runner) run(want int, args ...string) testutil.CLIResult {
	r.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	v, e := testutil.RunCLI(ctx, r.bin, r.dir, args...)
	if e != nil || v.ExitCode != want {
		r.t.Fatalf("%s %v: exit=%d err=%v\n%s\n%s", r.bin, args, v.ExitCode, e, v.Stdout, v.Stderr)
	}
	if len(args) > 0 && (args[0] == "apply" || args[0] == "destroy") {
		http.DefaultClient.CloseIdleConnections()
	}
	return v
}
func (r *runner) write(body string) {
	r.t.Helper()
	header := fmt.Sprintf(`terraform {
 required_providers {
  caddy = {
   source = "local.sebclem.fr/sebclem/caddy"
   version = "0.1.0"
  }
 }
}
provider "caddy" {
 endpoint = %q
}
`, r.f.Endpoint)
	if e := os.WriteFile(filepath.Join(r.dir, "main.tf"), []byte(header+body), 0600); e != nil {
		r.t.Fatal(e)
	}
}
func (r *runner) config(backend, routeOptions, extra string) string {
	return fmt.Sprintf(`resource "caddy_http_server" "main" {
 name = "main"
 listen = ["127.0.0.1:%d"]
}
resource "caddy_http_route" "app" {
 server_id = caddy_http_server.main.id
 %s
}
resource "caddy_handler_reverse_proxy" "app" {
 route_id = caddy_http_route.app.id
 upstream {
  dial = %q
 }
}
%s
`, r.port, routeOptions, strings.TrimPrefix(backend, "http://"), extra)
}
func (r *runner) values() map[string]map[string]any {
	result := r.run(0, "show", "-json")
	var state struct {
		Values struct {
			Root struct {
				Resources []struct {
					Address string
					Values  map[string]any
				} `json:"resources"`
			} `json:"root_module"`
		} `json:"values"`
	}
	if e := json.Unmarshal([]byte(result.Stdout), &state); e != nil {
		r.t.Fatal(e)
	}
	out := map[string]map[string]any{}
	for _, v := range state.Values.Root.Resources {
		out[v.Address] = v.Values
	}
	return out
}
func (r *runner) noWrites() {
	r.t.Helper()
	for _, v := range r.f.Requests() {
		if v.Method != "GET" {
			r.t.Fatalf("unexpected mutation: %+v", v)
		}
	}
}
func (r *runner) checkResponse(want string) {
	r.t.Helper()
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/", r.port), nil)
	req.Host = "app.test"
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		r.t.Fatal(e)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if string(b) != want {
		r.t.Fatalf("response %s expected %s", b, want)
	}
}
func backend(t *testing.T, name string) *httptest.Server {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(name)) }))
	t.Cleanup(s.Close)
	return s
}
func TestAccLifecycle(t *testing.T) {
	r := newRunner(t)
	a := backend(t, "A")
	b := backend(t, "B")
	opts := `terminal = true
 group = "app"
 match = [{ hosts = ["app.test"] }]`
	r.write(r.config(a.URL, opts, ""))
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "validate", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.checkResponse("A")
	values := r.values()
	for _, v := range values {
		id, ok := v["id"].(string)
		if !ok {
			t.Fatal("missing id")
		}
		u, e := uuid.Parse(id)
		if e != nil || u.Version() != 4 {
			t.Fatalf("not UUID v4: %s", id)
		}
		if v["object_id"] != id {
			t.Fatal("mismatched ID")
		}
	}
	r.f.ResetRequests()
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.noWrites()
	r.write(r.config(b.URL, opts, ""))
	r.run(2, "plan", "-detailed-exitcode", "-input=false", "-no-color", "-out=update.plan")
	plan := r.run(0, "show", "-json", "update.plan")
	var p struct {
		Changes []struct {
			Address string
			Change  struct {
				Actions []string `json:"actions"`
			} `json:"change"`
		} `json:"resource_changes"`
	}
	json.Unmarshal([]byte(plan.Stdout), &p)
	updates := 0
	for _, c := range p.Changes {
		if len(c.Change.Actions) == 1 && c.Change.Actions[0] == "update" {
			updates++
			if c.Address != "caddy_handler_reverse_proxy.app" {
				t.Fatal("unrelated update", c.Address)
			}
		} else if len(c.Change.Actions) != 1 || c.Change.Actions[0] != "no-op" {
			t.Fatal("unexpected action", c.Change.Actions)
		}
	}
	if updates != 1 {
		t.Fatalf("updates=%d", updates)
	}
	r.f.ResetRequests()
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color", "update.plan")
	r.checkResponse("B")
	for _, v := range r.f.Requests() {
		if v.Method != "GET" && v.Path != "/id/"+values["caddy_handler_reverse_proxy.app"]["id"].(string)+"/upstreams/0/dial" {
			t.Fatalf("broad write: %+v", v)
		}
	}
	after := r.values()
	for k, v := range values {
		if v["id"] != after[k]["id"] {
			t.Fatal("ID changed", k)
		}
	}
	id := values["caddy_handler_reverse_proxy.app"]["id"].(string)
	r.run(0, "state", "rm", "caddy_handler_reverse_proxy.app")
	r.f.ResetRequests()
	r.run(0, "import", "-input=false", "-no-color", "caddy_handler_reverse_proxy.app", id)
	r.noWrites()
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	serverID := values["caddy_http_server.main"]["id"].(string)
	r.run(0, "state", "rm", "caddy_http_server.main")
	r.f.ResetRequests()
	r.run(0, "import", "-input=false", "-no-color", "caddy_http_server.main", serverID)
	r.noWrites()
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.write(r.config(b.URL, "", ""))
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.checkResponse("B")
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
