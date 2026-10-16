package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type Request struct {
	Method, Path, IfMatch string
	Body                  any
}
type Fixture struct {
	Endpoint string
	Direct   string
	mu       sync.Mutex
	requests []Request
	Fault    func(http.ResponseWriter, *http.Request) bool
}

func FreePort(t *testing.T) int {
	t.Helper()
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
func StartCaddy(t *testing.T) *Fixture {
	t.Helper()
	bin := os.Getenv("CADDY_BIN")
	if bin == "" {
		t.Skip("set CADDY_BIN to run real Caddy tests")
	}
	dir := t.TempDir()
	port := FreePort(t)
	direct := fmt.Sprintf("http://127.0.0.1:%d", port)
	config := fmt.Sprintf(`{"admin":{"listen":"127.0.0.1:%d"},"apps":{}}`, port)
	conf := filepath.Join(dir, "config.json")
	os.WriteFile(conf, []byte(config), 0600)
	cmd := exec.Command(bin, "run", "--config", conf)
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+dir, "XDG_DATA_HOME="+dir, "HOME="+dir)
	var log bytes.Buffer
	cmd.Stdout = &log
	cmd.Stderr = &log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		cmd.Process.Signal(os.Interrupt)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			cmd.Process.Kill()
			<-done
		}
	})
	ready := false
	client := &http.Client{Timeout: 100 * time.Millisecond}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		resp, err := client.Get(direct + "/config/")
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		select {
		case err := <-done:
			t.Fatalf("Caddy exited: %v %s", err, log.String())
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		t.Fatal("Caddy did not start")
	}
	f := &Fixture{Direct: direct}
	target, _ := url.Parse(direct)
	proxy := httputil.NewSingleHostReverseProxy(target)
	// Caddy verifies Host on loopback admin connections; preserve its configured origin.
	proxy.Director = func(r *http.Request) { r.URL.Scheme = target.Scheme; r.URL.Host = target.Host; r.Host = target.Host }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte
		if r.Body != nil {
			var b bytes.Buffer
			b.ReadFrom(r.Body)
			body = b.Bytes()
			r.Body.Close()
			r.Body = ioBody(body)
		}
		var v any
		json.Unmarshal(body, &v)
		f.mu.Lock()
		f.requests = append(f.requests, Request{r.Method, r.URL.Path, r.Header.Get("If-Match"), v})
		fault := f.Fault
		f.mu.Unlock()
		if fault != nil && fault(w, r) {
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	f.Endpoint = server.URL
	return f
}
func (f *Fixture) Requests() []Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Request(nil), f.requests...)
}
func (f *Fixture) ResetRequests() { f.mu.Lock(); defer f.mu.Unlock(); f.requests = nil }

func (f *Fixture) SetFault(fn func(http.ResponseWriter, *http.Request) bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Fault = fn
}
