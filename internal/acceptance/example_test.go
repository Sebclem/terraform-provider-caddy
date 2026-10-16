package acceptance

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccShippedExample(t *testing.T) {
	r := newRunner(t)
	a := backend(t, "example")
	data, e := os.ReadFile("../../examples/basic/main.tf")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(r.dir, "main.tf"), data, 0600); e != nil {
		t.Fatal(e)
	}
	vars := []string{"-var=caddy_endpoint=" + r.f.Endpoint, fmt.Sprintf("-var=listen=127.0.0.1:%d", r.port), "-var=upstream=" + strings.TrimPrefix(a.URL, "http://")}
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "validate", "-no-color")
	schemaResult := r.run(0, "providers", "schema", "-json")
	var schema map[string]any
	if err := json.Unmarshal([]byte(schemaResult.Stdout), &schema); err != nil {
		t.Fatal(err)
	}
	providers := schema["provider_schemas"].(map[string]any)
	for _, raw := range providers {
		resources := raw.(map[string]any)["resource_schemas"].(map[string]any)
		if len(resources) != 3 {
			t.Fatal("unexpected resource surface")
		}
		for _, rs := range resources {
			block := rs.(map[string]any)["block"].(map[string]any)
			attrs := block["attributes"].(map[string]any)
			for name := range attrs {
				if name == "json" || name == "config" || strings.HasSuffix(name, "_json") {
					t.Fatal("raw configuration field", name)
				}
			}
		}
	}

	r.run(0, append([]string{"apply", "-auto-approve", "-input=false", "-no-color"}, vars...)...)
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/", r.port), nil)
	req.Host = "app.example.test"
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "example" {
		t.Fatalf("example response %s", body)
	}
	r.f.ResetRequests()
	r.run(0, append([]string{"plan", "-detailed-exitcode", "-input=false", "-no-color"}, vars...)...)
	r.run(0, append([]string{"apply", "-auto-approve", "-input=false", "-no-color"}, vars...)...)
	r.noWrites()
	r.run(0, append([]string{"destroy", "-auto-approve", "-input=false", "-no-color"}, vars...)...)
}
