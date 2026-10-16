package caddy

import (
	"reflect"
	"testing"
)

func TestReorder(t *testing.T) {
	for _, tc := range []struct {
		name string
		p    Position
		want []string
		err  bool
	}{
		{"before", Position{BeforeID: "a"}, []string{"c", "a", "b"}, false}, {"after", Position{AfterID: "a"}, []string{"a", "b", "c"}, false}, {"satisfied", Position{BeforeID: "c"}, []string{"a", "b", "c"}, true}, {"self", Position{BeforeID: "c"}, nil, true}, {"missing", Position{AfterID: "x"}, nil, true}, {"both", Position{BeforeID: "a", AfterID: "b"}, nil, true}, {"release", Position{}, []string{"a", "b", "c"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := []any{map[string]any{"@id": "a"}, map[string]any{"@id": "b"}, map[string]any{"@id": "c"}}
			out, _, e := Reorder(a, "c", tc.p)
			if tc.err {
				if e == nil {
					t.Fatal("expected error")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			var ids []string
			for _, v := range out {
				ids = append(ids, v.(map[string]any)["@id"].(string))
			}
			if !reflect.DeepEqual(ids, tc.want) {
				t.Fatalf("%v != %v", ids, tc.want)
			}
		})
	}
	a := []any{map[string]any{"@id": "a"}, map[string]any{"@id": "b"}, map[string]any{"@id": "c"}}
	out, moved, e := Reorder(a, "a", Position{BeforeID: "c"})
	if e != nil || moved || !reflect.DeepEqual(out, a) {
		t.Fatal("relative order unnecessarily moved")
	}
}
