package acceptance

import (
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"testing"
)

func normalizeLegacy(v any) {
	switch x := v.(type) {
	case map[string]any:
		if _, ok := x["hide"]; ok {
			x["hide"] = []any{"<source-caddyfile>"}
		}
		for _, v := range x {
			normalizeLegacy(v)
		}
	case []any:
		for _, v := range x {
			normalizeLegacy(v)
		}
	}
}
func TestLegacyFixture(t *testing.T) {
	bin := os.Getenv("CADDY_BIN")
	if bin == "" {
		t.Skip("set CADDY_BIN")
	}
	b, e := exec.Command(bin, "adapt", "--adapter", "caddyfile", "--config", "../../testdata/legacy/active.caddy").Output()
	if e != nil {
		t.Fatal(e)
	}
	golden, e := os.ReadFile("../../testdata/legacy/active.json")
	if e != nil {
		t.Fatal(e)
	}
	var actual, want any
	if e = json.Unmarshal(b, &actual); e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(golden, &want); e != nil {
		t.Fatal(e)
	}
	normalizeLegacy(actual)
	normalizeLegacy(want)
	if !reflect.DeepEqual(actual, want) {
		t.Fatal("adapted legacy config changed")
	}
}
