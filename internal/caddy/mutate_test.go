package caddy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMutateConflictRetry(t *testing.T) {
	writes := 0
	reads := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			reads++
			w.Header().Set("ETag", fmt.Sprintf(`"%d"`, reads))
			w.Write([]byte(`{"n":0}`))
			return
		}
		writes++
		w.WriteHeader(412)
	}))
	defer s.Close()
	c, _ := NewClient(Options{Endpoint: s.URL, ConflictRetries: 5})
	err := c.Mutate(context.Background(), Path{"config"}, func(s Snapshot) (Decision, error) {
		return Decision{Method: "PATCH", Path: Path{"config", "n"}, Body: 1}, nil
	})
	if err == nil || writes != 6 || reads != 6 {
		t.Fatalf("expected 6 guarded attempts, writes=%d reads=%d err=%v", writes, reads, err)
	}
}
func TestMutateLostResponse(t *testing.T) {
	n := 0
	writes := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("ETag", `"e"`)
			json.NewEncoder(w).Encode(map[string]any{"n": n})
			return
		}
		writes++
		n = 1
		conn, _, _ := w.(http.Hijacker).Hijack()
		conn.Close()
	}))
	defer s.Close()
	c, _ := NewClient(Options{Endpoint: s.URL, ConflictRetries: 5})
	err := c.Mutate(context.Background(), Path{"config"}, func(s Snapshot) (Decision, error) {
		if s.Value.(map[string]any)["n"] == json.Number("1") {
			return Decision{Done: true}, nil
		}
		return Decision{Method: "PATCH", Path: Path{"config", "n"}, Body: 1}, nil
	})
	if err != nil || writes != 1 {
		t.Fatalf("lost response: writes %d err %v", writes, err)
	}
}
func TestMutateNoop(t *testing.T) {
	writes := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes++
		}
		w.Header().Set("ETag", `"e"`)
		w.Write([]byte(`{}`))
	}))
	defer s.Close()
	c, _ := NewClient(Options{Endpoint: s.URL})
	if err := c.Mutate(context.Background(), Path{"config"}, func(Snapshot) (Decision, error) { return Decision{Done: true}, nil }); err != nil {
		t.Fatal(err)
	}
	if writes != 0 {
		t.Fatal("noop wrote")
	}
}
func TestMutateDeadline(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"e"`)
		if r.Method == "GET" {
			w.Write([]byte(`{}`))
		} else {
			w.WriteHeader(412)
		}
	}))
	defer s.Close()
	c, _ := NewClient(Options{Endpoint: s.URL, ConflictRetries: 100})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if e := c.Mutate(ctx, Path{"config"}, func(Snapshot) (Decision, error) {
		return Decision{Method: "PATCH", Path: Path{"config", "x"}, Body: 1}, nil
	}); e == nil {
		t.Fatal("deadline ignored")
	}
}

func TestMutateLostResponseNoRetries(t *testing.T) {
	n := 0
	writes := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("ETag", `"e"`)
			json.NewEncoder(w).Encode(map[string]any{"n": n})
			return
		}
		writes++
		n = 1
		conn, _, _ := w.(http.Hijacker).Hijack()
		conn.Close()
	}))
	defer s.Close()
	c, _ := NewClient(Options{Endpoint: s.URL, ConflictRetries: 0})
	err := c.Mutate(context.Background(), Path{"config"}, func(s Snapshot) (Decision, error) {
		if s.Value.(map[string]any)["n"] == json.Number("1") {
			return Decision{Done: true}, nil
		}
		return Decision{Method: "PATCH", Path: Path{"config", "n"}, Body: 1}, nil
	})
	if err != nil || writes != 1 {
		t.Fatalf("lost response: writes %d err %v", writes, err)
	}
}
func TestMutatePostCommitCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("ETag", `"e"`)
			w.Write([]byte(`{}`))
			return
		}
		cancel()
		conn, _, _ := w.(http.Hijacker).Hijack()
		conn.Close()
	}))
	defer s.Close()
	c, _ := NewClient(Options{Endpoint: s.URL, ConflictRetries: 5})
	err := c.Mutate(ctx, Path{"config"}, func(Snapshot) (Decision, error) {
		return Decision{Method: "POST", Path: Path{"config", "apps"}, Body: map[string]any{}}, nil
	})
	if !errors.Is(err, ErrUncertainWrite) || !errors.Is(err, context.Canceled) {
		t.Fatalf("lost uncertainty classification: %v", err)
	}
}
func TestMutateGatewayAfterCommit(t *testing.T) {
	n := 0
	writes := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("ETag", `"e"`)
			json.NewEncoder(w).Encode(map[string]any{"n": n})
			return
		}
		n = 1
		writes++
		w.WriteHeader(502)
	}))
	defer s.Close()
	c, _ := NewClient(Options{Endpoint: s.URL, ConflictRetries: 0})
	err := c.Mutate(context.Background(), Path{"config"}, func(s Snapshot) (Decision, error) {
		if s.Value.(map[string]any)["n"] == json.Number("1") {
			return Decision{Done: true}, nil
		}
		return Decision{Method: "PATCH", Path: Path{"config", "n"}, Body: 1}, nil
	})
	if err != nil || writes != 1 {
		t.Fatalf("gateway commit: %v writes=%d", err, writes)
	}
}
