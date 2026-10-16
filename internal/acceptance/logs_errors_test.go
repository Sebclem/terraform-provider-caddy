package acceptance

import (
	"encoding/json"
	"fmt"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/testutil"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccLogsAndErrorPages(t *testing.T) {
	r := newRunner(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "403.html"), []byte("denied-page"), 0644)
	logPath := filepath.Join(dir, "access.log")
	cfg := fmt.Sprintf(`resource "caddy_log" "app" {
 name = "site"
 writer = {filename = %q, mode = "0640", roll_size_mb = 48, roll_local_time = true}
 include = ["http.log.access.site"]
}
resource "caddy_log" "default" {
 name = "default"
 exclude = ["http.log.access.site"]
}
resource "caddy_http_server" "main" {
 name = "main"
 listen = ["127.0.0.1:%d"]
}
resource "caddy_log_binding" "app" {
 server_id = caddy_http_server.main.id
 hostname = "app.test"
 log_id = caddy_log.app.id
}
resource "caddy_http_route" "app" {
 server_id = caddy_http_server.main.id
 match = [{hosts = ["app.test"]}]
}
resource "caddy_handler_subroute" "app" {route_id = caddy_http_route.app.id}
resource "caddy_error_pages" "app" {
 subroute_id = caddy_handler_subroute.app.id
 root = %q
 page_uri = "/{http.error.status_code}.html"
}
resource "caddy_http_route" "deny" {subroute_id = caddy_handler_subroute.app.id}
resource "caddy_handler_error" "deny" {
 route_id = caddy_http_route.deny.id
 status_code = 403
 message = "Unauthorized"
}
`, logPath, r.port, dir)
	r.write(cfg)
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	q, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/", r.port), nil)
	q.Host = "app.test"
	resp, e := http.DefaultClient.Do(q)
	if e != nil {
		t.Fatal(e)
	}
	data := readBody(t, resp)
	if resp.StatusCode != 403 || data != "denied-page" {
		t.Fatalf("error response %d %s", resp.StatusCode, data)
	}
	dataBytes, e := os.ReadFile(logPath)
	if e != nil || !strings.Contains(string(dataBytes), "app.test") {
		t.Fatalf("access log: %s %v", dataBytes, e)
	}
	info, _ := os.Stat(logPath)
	if info.Mode().Perm() != 0640 {
		t.Fatalf("log mode %o", info.Mode().Perm())
	}
	r.f.ResetRequests()
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.noWrites()
	id := r.values()["caddy_error_pages.app"]["id"].(string)
	r.run(0, "state", "rm", "caddy_error_pages.app")
	r.run(0, "import", "-input=false", "-no-color", "caddy_error_pages.app", id)
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	for _, status := range []int{404, 500} {
		filename := filepath.Join(dir, fmt.Sprintf("%d.html", status))
		os.WriteFile(filename, []byte(fmt.Sprintf("page-%d", status)), 0644)
		next := strings.Replace(cfg, "status_code = 403", fmt.Sprintf("status_code = %d", status), 1)
		r.write(next)
		r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
		resp, e := http.DefaultClient.Do(q)
		if e != nil {
			t.Fatal(e)
		}
		actual := readBody(t, resp)
		wantStatus, wantBody := legacyErrorResponse(t, dir, "/{http.error.status_code}.html", status)
		if resp.StatusCode != wantStatus || actual != wantBody {
			t.Fatalf("error page %d: got %d %q want %d %q", status, resp.StatusCode, actual, wantStatus, wantBody)
		}
	}
	missing := strings.Replace(cfg, `page_uri = "/{http.error.status_code}.html"`, `page_uri = "/absent.html"`, 1)
	r.write(missing)
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	resp, e = http.DefaultClient.Do(q)
	if e != nil {
		t.Fatal(e)
	}
	actual := readBody(t, resp)
	wantStatus, wantBody := legacyErrorResponse(t, dir, "/absent.html", 403)
	if resp.StatusCode != wantStatus || actual != wantBody {
		t.Fatalf("missing error page got %d %q want %d %q", resp.StatusCode, actual, wantStatus, wantBody)
	}
	if r.values()["caddy_error_pages.app"]["id"] != id {
		t.Fatal("error-page ID changed")
	}
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
func readBody(t *testing.T, r *http.Response) string {
	t.Helper()
	defer r.Body.Close()
	b, e := io.ReadAll(r.Body)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func legacyErrorResponse(t *testing.T, root, uri string, status int) (int, string) {
	t.Helper()
	f := testutil.StartCaddy(t)
	port := testutil.FreePort(t)
	data, e := os.ReadFile("../../testdata/legacy/active.json")
	if e != nil {
		t.Fatal(e)
	}
	var legacy map[string]any
	json.Unmarshal(data, &legacy)
	route := legacy["apps"].(map[string]any)["http"].(map[string]any)["servers"].(map[string]any)["srv0"].(map[string]any)["errors"].(map[string]any)["routes"].([]any)[0].(map[string]any)
	route["match"] = []any{map[string]any{"host": []any{"app.test"}}}
	var visit func(any)
	visit = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if _, ok := x["root"]; ok {
				x["root"] = root
			}
			if x["handler"] == "rewrite" {
				x["uri"] = uri
			}
			for _, v := range x {
				visit(v)
			}
		case []any:
			for _, v := range x {
				visit(v)
			}
		}
	}
	visit(route)
	cfg := map[string]any{"admin": map[string]any{"listen": strings.TrimPrefix(f.Direct, "http://")}, "apps": map[string]any{"http": map[string]any{"servers": map[string]any{"legacy": map[string]any{"listen": []any{fmt.Sprintf("127.0.0.1:%d", port)}, "automatic_https": map[string]any{"disable": true}, "routes": []any{map[string]any{"handle": []any{map[string]any{"handler": "error", "error": "Unauthorized", "status_code": status}}}}, "errors": map[string]any{"routes": []any{route}}}}}}}
	b, _ := json.Marshal(cfg)
	resp, e := http.Post(f.Direct+"/load", "application/json", strings.NewReader(string(b)))
	if e != nil {
		t.Fatal(e)
	}
	if resp.StatusCode != 200 {
		t.Fatal(readBody(t, resp))
	}
	resp.Body.Close()
	q, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/", port), nil)
	q.Host = "app.test"
	resp, e = http.DefaultClient.Do(q)
	if e != nil {
		t.Fatal(e)
	}
	return resp.StatusCode, readBody(t, resp)
}
