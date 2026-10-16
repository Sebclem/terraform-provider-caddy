package provider

import (
	"context"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBootstrapDiagnostic(t *testing.T) {
	writes := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes++
		}
		w.Header().Set("ETag", `"null"`)
		w.Write([]byte(`null`))
	}))
	defer s.Close()
	c, _ := caddy.NewClient(caddy.Options{Endpoint: s.URL})
	err := createObject(context.Background(), c, managed{kind: "server", id: "s", name: "s", fields: map[string]any{"listen": []any{":8080"}}})
	if err == nil || !strings.Contains(err.Error(), "bootstrap") || writes != 0 {
		t.Fatalf("not actionable or unsafe: writes=%d err=%v", writes, err)
	}
}
