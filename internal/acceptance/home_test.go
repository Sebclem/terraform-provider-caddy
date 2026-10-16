package acceptance

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/testutil"
)

func TestAccHomeRebuild(t *testing.T) {
	r := newRunner(t)
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	os.MkdirAll(logDir, 0755)
	for _, status := range []int{403, 404, 500} {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.html", status)), []byte(fmt.Sprintf("error-%d", status)), 0644)
	}
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		if q.URL.Path == "/health" {
			w.WriteHeader(200)
			return
		}
		fmt.Fprint(w, "app:"+q.Method+":"+q.URL.Path)
	}))
	defer app.Close()
	tlsApp := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) { fmt.Fprint(w, "app:"+q.Method+":"+q.URL.Path) }))
	defer tlsApp.Close()
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		if q.Header.Get("X-Fixture-Deny") == "yes" {
			w.WriteHeader(401)
			fmt.Fprint(w, "auth-denied")
			return
		}
		w.Header().Set("Remote-User", "fixture")
		w.WriteHeader(204)
	}))
	defer auth.Close()
	raw, e := os.ReadFile("../../testdata/legacy/active.json")
	if e != nil {
		t.Fatal(e)
	}
	var legacy map[string]any
	json.Unmarshal(raw, &legacy)
	httpApp := legacy["apps"].(map[string]any)["http"].(map[string]any)
	server := httpApp["servers"].(map[string]any)["srv0"].(map[string]any)
	var hosts, sebHosts, clapHosts []string
	for _, route := range server["routes"].([]any) {
		h := route.(map[string]any)["match"].([]any)[0].(map[string]any)["host"].([]any)[0].(string)
		hosts = append(hosts, h)
		if strings.Contains(h, "claptrapbot.com") {
			clapHosts = append(clapHosts, h)
		} else {
			sebHosts = append(sebHosts, h)
		}
	}
	cp, kp := certificateHosts(t, dir, "sebclem", sebHosts)
	cp2, kp2 := certificateHosts(t, dir, "claptrapbot", clapHosts)
	var hash string
	var rewrite func(any)
	rewrite = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if p, ok := x["password"].(string); ok {
				hash = p
			}
			if _, ok := x["certificate"]; ok {
				if strings.Contains(x["certificate"].(string), "claptrapbot") {
					x["certificate"] = cp2
					x["key"] = kp2
				} else {
					x["certificate"] = cp
					x["key"] = kp
				}
			}
			if p, ok := x["filename"].(string); ok {
				x["filename"] = filepath.Join(logDir, filepath.Base(p))
			}
			if _, ok := x["root"]; ok {
				x["root"] = dir
			}
			if x["handler"] == "reverse_proxy" {
				dial := strings.TrimPrefix(app.URL, "http://")
				if _, ok := x["transport"]; ok {
					dial = strings.TrimPrefix(tlsApp.URL, "https://")
				}
				if _, ok := x["rewrite"]; ok {
					dial = strings.TrimPrefix(auth.URL, "http://")
				}
				for _, u := range x["upstreams"].([]any) {
					u.(map[string]any)["dial"] = dial
				}
			}
			for _, v := range x {
				rewrite(v)
			}
		case []any:
			for _, v := range x {
				rewrite(v)
			}
		}
	}
	rewrite(legacy)
	admin := strings.TrimPrefix(r.f.Direct, "http://")
	legacy["admin"] = map[string]any{"listen": admin}
	httpPort := testutil.FreePort(t)
	httpApp["http_port"] = httpPort
	httpApp["https_port"] = r.port
	server["listen"] = []any{fmt.Sprintf("127.0.0.1:%d", r.port)}
	server["automatic_https"] = map[string]any{"disable_certificates": true}
	server["trusted_proxies"] = map[string]any{"source": "static", "ranges": []any{"10.0.0.0/8", "127.0.0.0/8"}}
	load := func(value any) {
		t.Helper()
		data, _ := json.Marshal(value)
		resp, e := http.Post(r.f.Direct+"/load", "application/json", strings.NewReader(string(data)))
		if e != nil {
			t.Fatal(e)
		}
		body := readBody(t, resp)
		if resp.StatusCode != 200 {
			t.Fatalf("operator fixture load %d %s", resp.StatusCode, body)
		}
	}
	load(legacy)
	type observation struct {
		Status int
		Body   string
	}
	type probe struct {
		host, method, path, ip string
		deny                   bool
	}
	probes := []probe{}
	for _, h := range hosts {
		probes = append(probes, probe{h, "GET", "/", "10.1.2.3", false})
	}
	probes = append(probes, probe{"jellyfin.sebclem.fr", "GET", "/metrics", "10.1.2.3", false}, probe{"jellyfin.sebclem.fr", "GET", "/metrics", "192.0.2.1", false}, probe{"grafana-webdav.sebclem.fr", "POST", "/file", "192.0.2.1", false}, probe{"grafana-webdav.sebclem.fr", "GET", "/file", "192.0.2.1", false}, probe{"git.sebclem.fr", "GET", "/", "192.0.2.1", false}, probe{"git.sebclem.fr", "GET", "/", "10.1.2.3", true})
	observe := func() []observation {
		t.Helper()
		out := []observation{}
		for _, p := range probes {
			client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: p.host}}}
			q, _ := http.NewRequest(p.method, fmt.Sprintf("https://127.0.0.1:%d%s", r.port, p.path), nil)
			q.Host = p.host
			q.Header.Set("X-Forwarded-For", p.ip)
			q.SetBasicAuth("vm", "fixture-password")
			if p.deny {
				q.Header.Set("X-Fixture-Deny", "yes")
			}
			resp, e := client.Do(q)
			if e != nil {
				t.Fatal(p, e)
			}
			if err := resp.TLS.PeerCertificates[0].VerifyHostname(p.host); err != nil {
				t.Fatal("SNI certificate", p.host, err)
			}
			expectedGroup := "sebclem"
			if strings.Contains(p.host, "claptrapbot.com") {
				expectedGroup = "claptrapbot"
			}
			if resp.TLS.PeerCertificates[0].Subject.CommonName != expectedGroup {
				t.Fatal("wrong SNI certificate group", p.host)
			}
			out = append(out, observation{resp.StatusCode, readBody(t, resp)})
			client.CloseIdleConnections()
		}
		return out
	}
	before := observe()
	// Operator export and replacement are separate from provider CRUD.
	export, e := http.Get(r.f.Direct + "/config/")
	if e != nil {
		t.Fatal(e)
	}
	backup, _ := io.ReadAll(export.Body)
	export.Body.Close()
	os.WriteFile(filepath.Join(dir, "backup.json"), backup, 0600)
	bootstrap := map[string]any{"admin": map[string]any{"listen": admin}, "apps": map[string]any{"http": map[string]any{"http_port": httpPort, "https_port": r.port}}}
	load(bootstrap)
	for _, name := range []string{"main.tf", "sites.tf"} {
		data, e := os.ReadFile("../../examples/home/" + name)
		if e != nil {
			t.Fatal(e)
		}
		text := string(data)
		if name == "sites.tf" {
			rx := regexp.MustCompile(`"upstream"\s*=\s*"[^"]+"`)
			lines := strings.Split(text, "\n")
			for i, line := range lines {
				dial := strings.TrimPrefix(app.URL, "http://")
				if strings.Contains(line, `"tls" = true`) {
					dial = strings.TrimPrefix(tlsApp.URL, "https://")
				}
				lines[i] = rx.ReplaceAllString(line, `"upstream" = "`+dial+`"`)
			}
			text = strings.Join(lines, "\n")
		}
		text = strings.ReplaceAll(text, `"auth.home:9091"`, fmt.Sprintf("%q", strings.TrimPrefix(auth.URL, "http://")))
		text = strings.ReplaceAll(text, `["10.0.0.0/8"]`, `["10.0.0.0/8", "127.0.0.0/8"]`)
		os.WriteFile(filepath.Join(r.dir, name), []byte(text), 0600)
	}
	vars := map[string]any{"caddy_endpoint": r.f.Endpoint, "listen": []string{fmt.Sprintf("127.0.0.1:%d", r.port)}, "error_root": dir, "log_directory": logDir, "certificate_files": map[string]any{"sebclem": map[string]any{"certificate": cp, "key": kp}, "claptrapbot": map[string]any{"certificate": cp2, "key": kp2}}, "basic_auth_accounts": []any{map[string]any{"username": "vm", "password_hash": hash}, map[string]any{"username": "grafana", "password_hash": hash}, map[string]any{"username": "opnsense", "password_hash": hash}, map[string]any{"username": "admin", "password_hash": hash}}}
	varBytes, _ := json.Marshal(vars)
	os.WriteFile(filepath.Join(r.dir, "fixture.auto.tfvars.json"), varBytes, 0600)
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "validate", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	after := observe()
	for i, want := range before {
		if after[i] != want {
			t.Errorf("parity %+v: before %+v after %+v", probes[i], want, after[i])
		}
	}
	for _, h := range hosts {
		data, e := os.ReadFile(filepath.Join(logDir, h+".log"))
		expected := 0
		for _, p := range probes {
			if p.host == h {
				expected += 2
			}
		}
		if count := strings.Count(string(data), `"host":"`+h+`"`); count != expected {
			t.Errorf("duplicate/missing access records %s: got %d want %d", h, count, expected)
		}
		if e != nil || !strings.Contains(string(data), h) {
			t.Errorf("missing site log %s: %v", h, e)
		}
	}
	r.f.ResetRequests()
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.noWrites()
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
	// Rehearse operator recovery, then leave only the isolated bootstrap.
	var recovered any
	json.Unmarshal(backup, &recovered)
	load(recovered)
	result := observe()
	for i, want := range before {
		if result[i] != want {
			t.Errorf("recovery parity %s", probes[i].host)
		}
	}
	load(bootstrap)
	if !t.Failed() {
		t.Logf("Rebuilt %d active sites; %d behavior probes matched legacy and recovery; second plan/apply made no writes", len(hosts), len(probes))
	}
}
