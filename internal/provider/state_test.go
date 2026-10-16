package provider

import (
	"reflect"
	"testing"
)

func TestOwnershipTransitions(t *testing.T) {
	intent, owned, e := PlanFields(map[string]any{"listen": []any{":8081"}}, map[string]any{"listen": []any{":8080"}, "group": "old", "foreign": true}, Ownership{"listen": true, "group": true})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(intent.Remove, []string{"group"}) || !owned["listen"] || owned["group"] || intent.Set["listen"] == nil {
		t.Fatalf("bad ownership: %+v %+v", intent, owned)
	}
	intent, _, _ = PlanFields(map[string]any{}, map[string]any{"group": "external"}, Ownership{})
	if len(intent.Remove) != 0 || len(intent.Set) != 0 {
		t.Fatal("never-owned setting changed")
	}
}
