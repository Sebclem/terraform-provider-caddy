package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/caddy"
)

func TestNullRootBootstrap(t *testing.T) {
	var mu sync.Mutex
	var doc any = nil
	emptyRootWritten := false

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if r.Method == "GET" && r.URL.Path == "/config/" {
			if doc == nil {
				w.Header().Set("ETag", `"null-etag"`)
				w.Write([]byte(`null`))
				return
			}
			data, _ := json.Marshal(doc)
			w.Header().Set("ETag", `"doc-etag"`)
			w.Write(data)
			return
		}

		if r.Method == "POST" && r.URL.Path == "/config/" {
			body, _ := io.ReadAll(r.Body)
			var parsed map[string]any
			json.Unmarshal(body, &parsed)
			if len(parsed) == 0 {
				emptyRootWritten = true
				doc = map[string]any{}
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		if r.Method == "POST" && r.URL.Path == "/config/apps" {
			body, _ := io.ReadAll(r.Body)
			var parsed map[string]any
			json.Unmarshal(body, &parsed)
			if docMap, ok := doc.(map[string]any); ok {
				docMap["apps"] = parsed
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		http.Error(w, "unexpected request", http.StatusBadRequest)
	}))
	defer s.Close()

	c, _ := caddy.NewClient(caddy.Options{Endpoint: s.URL})
	err := createObject(context.Background(), c, managed{kind: "server", id: "s", name: "s", fields: map[string]any{"listen": []any{":8080"}}})
	if err != nil {
		t.Fatalf("expected createObject to succeed with empty root bootstrap, got: %v", err)
	}
	if !emptyRootWritten {
		t.Fatalf("expected empty root POST /config/ to have been executed")
	}
}
