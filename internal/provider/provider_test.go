package provider

import (
	"context"
	fp "github.com/hashicorp/terraform-plugin-framework/provider"
	"testing"
)

func TestProviderSchema(t *testing.T) {
	p := New("test")()
	var resp fp.SchemaResponse
	p.Schema(context.Background(), fp.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if !resp.Schema.Attributes["endpoint"].IsRequired() {
		t.Fatal("endpoint not required")
	}
	for _, name := range []string{"json", "config", "config_json", "unix_socket"} {
		if _, ok := resp.Schema.Attributes[name]; ok {
			t.Fatalf("unexpected field %s", name)
		}
	}
}
func TestResourceRegistration(t *testing.T) {
	if got := len(New("test")().Resources(context.Background())); got != 12 {
		t.Fatalf("registered %d resources, want 12", got)
	}
}
