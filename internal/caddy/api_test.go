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
func TestAPIMutateBootstrapAndReorderRace(t *testing.T) {
	f := testutil.StartCaddy(t)
	c, _ := caddy.NewClient(caddy.Options{Endpoint: f.Endpoint, ConflictRetries: 5})
	ctx := context.Background()
	e := c.Mutate(ctx, caddy.Path{"config"}, func(s caddy.Snapshot) (caddy.Decision, error) {
		_, e := caddy.At(s.Value, caddy.Path{"apps", "http"})
		if e == nil {
			return caddy.Decision{Done: true}, nil
		}
		return caddy.Decision{Method: "POST", Path: caddy.Path{"config", "apps", "http"}, Body: map[string]any{"servers": map[string]any{"s": map[string]any{"@id": "server", "listen": []any{":0"}, "automatic_https": map[string]any{"disable": true}, "routes": []any{map[string]any{"@id": "a"}, map[string]any{"@id": "b"}}}}}}, nil
	})
	if e != nil {
		t.Fatal(e)
	}
	once := false
	e = c.Mutate(ctx, caddy.Path{"config"}, func(s caddy.Snapshot) (caddy.Decision, error) {
		a, e := caddy.Resolve(s, "a")
		if e != nil {
			return caddy.Decision{}, e
		}
		if a.Value["terminal"] == true {
			return caddy.Decision{Done: true}, nil
		}
		if !once {
			once = true
			s2, _ := c.Get(ctx, caddy.Path{"config"})
			e = c.Write(ctx, "PATCH", caddy.Path{"id", "server", "routes"}, []any{map[string]any{"@id": "b"}, map[string]any{"@id": "a"}}, s2.ETag)
			if e != nil {
				t.Fatal(e)
			}
		}
		return caddy.Decision{Method: "POST", Path: caddy.Path{"id", "a", "terminal"}, Body: true}, nil
	})
	if e != nil {
		t.Fatal(e)
	}
	s, _ := c.Get(ctx, caddy.Path{"config"})
	a, _ := caddy.Resolve(s, "a")
	b, _ := caddy.Resolve(s, "b")
	if a.Value["terminal"] != true || b.Value["terminal"] != nil {
		t.Fatal("wrong sibling changed")
	}
}
