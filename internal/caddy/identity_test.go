package caddy

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestResolveID(t *testing.T) {
	root := map[string]any{"apps": map[string]any{"http": map[string]any{"servers": map[string]any{"s": map[string]any{"@id": "server", "listen": []any{":8080"}, "routes": []any{map[string]any{"@id": "route", "handle": []any{map[string]any{"@id": "proxy", "handler": "reverse_proxy", "n": json.Number("9007199254740993")}}}}}}}}}
	snap := Snapshot{Value: root, Path: Path{"config"}}
	obj, e := Resolve(snap, "proxy")
	if e != nil {
		t.Fatal(e)
	}
	if len(obj.Path) != 9 || obj.Path[8] != "0" {
		t.Fatalf("wrong path: %v", obj.Path)
	}
	parent, e := Resolve(snap, "route")
	if e != nil {
		t.Fatal(e)
	}
	if e = RequireParent(obj, parent, "handle"); e != nil {
		t.Fatal(e)
	}
	server, _ := Resolve(snap, "server")
	if e = RequireParent(obj, server, "handle"); e == nil {
		t.Fatal("wrong parent accepted")
	}
	if _, e = Resolve(snap, "absent"); !errors.Is(e, ErrNotFound) {
		t.Fatalf("absence: %v", e)
	}
	root["duplicate"] = map[string]any{"@id": "proxy"}
	if _, e = Resolve(snap, "proxy"); e == nil {
		t.Fatal("duplicate accepted")
	}
}
