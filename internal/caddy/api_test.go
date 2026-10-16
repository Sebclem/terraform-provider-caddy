package caddy_test

import (
	"context"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/testutil"
	"testing"
)

func TestAPIConditionalWrite(t *testing.T) {
	f := testutil.StartCaddy(t)
	c, e := caddy.NewClient(caddy.Options{Endpoint: f.Endpoint})
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	s, e := c.Get(ctx, caddy.Path{"config"})
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Write(ctx, "POST", caddy.Path{"config", "apps", "http"}, map[string]any{"servers": map[string]any{}}, s.ETag); e != nil {
		t.Fatal(e)
	}
	if e = c.Write(ctx, "POST", caddy.Path{"config", "apps", "http"}, map[string]any{}, s.ETag); !caddy.IsStatus(e, 412) {
		t.Fatalf("expected stale guard rejection, got %v", e)
	}
}
