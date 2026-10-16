package caddy

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClientConditionalWrite(t *testing.T) {
	seen := false
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("ETag", `"/config/foo abc"`)
			w.Write([]byte(`{"number":9007199254740993}`))
			return
		}
		seen = true
		if r.URL.Path != "/id/my-proxy/upstreams/0/dial" || r.Header.Get("If-Match") != `"/config/foo abc"` || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("bad request: %s %v", r.URL, r.Header)
		}
		var dial string
		json.NewDecoder(r.Body).Decode(&dial)
		if dial != "backend:80" {
			t.Errorf("dial %q", dial)
		}
	}))
	defer s.Close()
	c, err := NewClient(Options{Endpoint: s.URL})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := c.Get(context.Background(), Path{"config", "foo"})
	if err != nil {
		t.Fatal(err)
	}
	if snap.ETag != `"/config/foo abc"` {
		t.Fatalf("missing ETag: %+v", snap)
	}
	if snap.Value.(map[string]any)["number"] != json.Number("9007199254740993") {
		t.Fatal("lost precision")
	}
	if err = c.Write(context.Background(), "PATCH", Path{"id", "my-proxy", "upstreams", "0", "dial"}, "backend:80", snap.ETag); err != nil {
		t.Fatal(err)
	}
	if !seen {
		t.Fatal("no write")
	}
	if err = c.Write(context.Background(), "PATCH", Path{"config", "foo"}, true, ""); err == nil {
		t.Fatal("unguarded write accepted")
	}
}
func TestClientTransport(t *testing.T) {
	for _, endpoint := range []string{"", "ftp://localhost", "http://user:secret@localhost", "http://localhost?x=y", "http://localhost/extra"} {
		if _, err := NewClient(Options{Endpoint: endpoint}); err == nil {
			t.Errorf("accepted %q", endpoint)
		}
	}
	if _, err := NewClient(Options{Endpoint: "http://localhost", ClientKeyFile: "key"}); err == nil {
		t.Fatal("unpaired key")
	}
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("ETag", `"e"`); w.Write([]byte(`{}`)) }))
	defer s.Close()
	c, _ := NewClient(Options{Endpoint: s.URL})
	if _, err := c.Get(context.Background(), Path{"config"}); err == nil {
		t.Fatal("untrusted TLS accepted")
	}
	ca := filepath.Join(t.TempDir(), "ca.pem")
	os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.Certificate().Raw}), 0600)
	c, err := NewClient(Options{Endpoint: s.URL, CAFile: ca})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Get(context.Background(), Path{"config"}); err != nil {
		t.Fatal(err)
	}
	// Require a real client certificate, using the test certificate only in this fixture.
	pool := x509.NewCertPool()
	pool.AddCert(s.Certificate())
	s.TLS.ClientCAs = pool
	s.TLS.ClientAuth = tls.RequireAndVerifyClientCert
	cert := filepath.Join(t.TempDir(), "client.pem")
	key := filepath.Join(t.TempDir(), "client.key")
	template := &x509.Certificate{SerialNumber: big.NewInt(42), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	clientDER, err := x509.CreateCertificate(rand.Reader, template, template, s.Certificate().PublicKey, s.TLS.Certificates[0].PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	clientCert, _ := x509.ParseCertificate(clientDER)
	pool.AddCert(clientCert)
	os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER}), 0600)
	keyDER, _ := x509.MarshalPKCS8PrivateKey(s.TLS.Certificates[0].PrivateKey)
	os.WriteFile(key, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600)
	c, err = NewClient(Options{Endpoint: s.URL, CAFile: ca, ClientCertificateFile: cert, ClientKeyFile: key})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Get(context.Background(), Path{"config"}); err != nil {
		t.Fatal(err)
	}
}
func TestClientDeadline(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer s.Close()
	c, _ := NewClient(Options{Endpoint: s.URL, RequestTimeout: 10 * time.Millisecond})
	if _, err := c.Get(context.Background(), Path{"config"}); err == nil {
		t.Fatal("no timeout")
	}
}
func TestClientErrors(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/config/redirect" {
			http.Redirect(w, r, "https://example.test", 302)
			return
		}
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"password supersecret rejected"}`))
	}))
	defer s.Close()
	c, _ := NewClient(Options{Endpoint: s.URL})
	_, err := c.Get(context.Background(), Path{"config"})
	if err == nil || strings.Contains(err.Error(), "supersecret") {
		t.Fatalf("unsafe diagnostic: %v", err)
	}
	if _, err = c.Get(context.Background(), Path{"config", "redirect"}); err == nil {
		t.Fatal("followed redirect")
	}
}
