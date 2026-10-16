GO ?= go
.PHONY: test-api test check build test-acceptance
build:
	mkdir -p bin
	$(GO) build -o bin/terraform-provider-caddy .
test:
	$(GO) test ./...
test-api:
	test -n "$(CADDY_BIN)"
	$(GO) test ./internal/caddy -run TestAPI -count=1 -v
check:
	$(GO) test ./...
	$(GO) vet ./...
	$(GO) build ./...
test-acceptance: build
	test -n "$(CADDY_BIN)"
	test -n "$(IAC_BIN)"
	$(GO) test ./internal/acceptance -count=1 -v -timeout 10m
