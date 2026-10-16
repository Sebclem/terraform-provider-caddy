package provider

import (
	"testing"
)

func TestUnsupportedTypedImport(t *testing.T) {
	for _, tc := range []struct {
		kind, key string
		obj       map[string]any
	}{
		{"proxy", "transport", map[string]any{"transport": map[string]any{"protocol": "fastcgi"}}},
		{"log", "writer", map[string]any{"writer": map[string]any{"output": "net", "address": "localhost:9000"}}},
		{"basic_auth", "accounts", map[string]any{"providers": map[string]any{"http_basic": map[string]any{"hash": map[string]any{"algorithm": "scrypt"}, "accounts": []any{}}}}},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			if _, e := observedField(tc.kind, tc.key, tc.obj); e == nil {
				t.Fatal("unsupported import accepted")
			}
		})
	}
}
func TestNegatedMatcherDepth(t *testing.T) {
	if _, e := decodeMatchers([]any{map[string]any{"not": []any{map[string]any{"not": []any{map[string]any{"host": []any{"example.test"}}}}}}}); e == nil {
		t.Fatal("nested negation cannot be represented by the public schema")
	}
}
