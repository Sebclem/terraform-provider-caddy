package main

import (
	"testing"
)

func TestProviderDefaultAddress(t *testing.T) {
	expected := "terralist.sebclem.fr/sebclem/caddy"
	if address != expected {
		t.Errorf("expected address %q, got %q", expected, address)
	}
}
