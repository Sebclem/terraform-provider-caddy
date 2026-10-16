package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/testutil"
)

func TestTrustedIPv4IPv6Peers(t *testing.T) {
	ctx, c, _ := connection(t)
	port := testutil.FreePort(t)
	server := managed{kind: "server", id: "srv", name: "dual", fields: map[string]any{"listen": []any{fmt.Sprintf("127.0.0.1:%d", port), fmt.Sprintf("[::1]:%d", port)}, "trusted_proxies": map[string]any{"source": "static", "ranges": []any{"127.0.0.0/8"}}}}
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "allowed") }))
	defer app.Close()
	objects := []managed{server, {kind: "route", id: "deny", parent: "srv", fields: map[string]any{"match": []any{map[string]any{"not": []any{map[string]any{"client_ip": map[string]any{"ranges": []any{"10.0.0.0/8", "2001:db8::/32"}}}}}}}}, {kind: "error", id: "error", parent: "deny", fields: map[string]any{"status_code": 403, "error": "Unauthorized"}}, {kind: "route", id: "fallback", parent: "srv", fields: map[string]any{}}, {kind: "proxy", id: "proxy", parent: "fallback", fields: map[string]any{"upstreams": []any{map[string]any{"dial": strings.TrimPrefix(app.URL, "http://")}}}}}
	for _, m := range objects {
		if e := createObject(ctx, c, m); e != nil {
			t.Fatal(e)
		}
	}
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	check := func(peer, ip string, want int) {
		t.Helper()
		q, _ := http.NewRequest("GET", fmt.Sprintf("http://%s:%d/", peer, port), nil)
		if ip != "" {
			q.Header.Set("X-Forwarded-For", ip)
		}
		resp, e := client.Do(q)
		if e != nil {
			t.Fatal(e)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("peer %s forwarded %s got %d want %d", peer, ip, resp.StatusCode, want)
		}
	}
	check("127.0.0.1", "", 403)
	check("127.0.0.1", "10.1.2.3", 200)
	check("127.0.0.1", "2001:db8::1", 200)
	check("[::1]", "10.1.2.3", 403)
	check("[::1]", "2001:db8::1", 403)
	server.fields["trusted_proxies"] = map[string]any{"source": "static", "ranges": []any{"127.0.0.0/8", "::1/128"}}
	if e := updateObject(ctx, c, server, Ownership{"listen": true, "trusted_proxies": true}); e != nil {
		t.Fatal(e)
	}
	check("[::1]", "2001:db8::1", 200)
	check("[::1]", "10.1.2.3", 200)
}
