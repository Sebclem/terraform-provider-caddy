package acceptance

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/testutil"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func certificate(t *testing.T, dir, name string) (string, string) {
	return certificateHosts(t, dir, name, []string{name})
}
func certificateHosts(t *testing.T, dir, name string, hosts []string) (string, string) {
	t.Helper()
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name}, DNSNames: hosts, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, e := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	kb, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		t.Fatal(e)
	}
	cp, kp := filepath.Join(dir, name+".pem"), filepath.Join(dir, name+".key")
	os.WriteFile(cp, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600)
	os.WriteFile(kp, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kb}), 0600)
	return cp, kp
}
func TestAccTLS(t *testing.T) {
	r := newRunner(t)
	httpPort := testutil.FreePort(t)
	bootstrap, e := http.Post(r.f.Direct+"/config/apps/http", "application/json", strings.NewReader(fmt.Sprintf(`{"http_port":%d,"https_port":%d}`, httpPort, r.port)))
	if e != nil {
		t.Fatal(e)
	}
	bootstrap.Body.Close()
	if bootstrap.StatusCode != 200 {
		t.Fatal("bootstrap HTTP app ports", bootstrap.StatusCode)
	}
	b := backend(t, "secure")
	dir := t.TempDir()
	cp, kp := certificate(t, dir, "app.test")
	cfg := fmt.Sprintf(`resource "caddy_tls_certificate" "app" {
 certificate = %q
 key = %q
}
resource "caddy_http_server" "main" {
 name = "main"
 listen = ["127.0.0.1:%d"]
 https = true
 depends_on = [caddy_tls_certificate.app]
}
resource "caddy_http_route" "app" {
 server_id = caddy_http_server.main.id
 match = [{hosts = ["app.test"]}]
}
resource "caddy_handler_reverse_proxy" "app" {
 route_id = caddy_http_route.app.id
 upstream {dial = %q}
}
`, cp, kp, r.port, strings.TrimPrefix(b.URL, "http://"))
	r.write(cfg)
	r.run(0, "init", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: "app.test"}}}
	defer client.CloseIdleConnections()
	resp, e := client.Get(fmt.Sprintf("https://127.0.0.1:%d", r.port))
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.TLS.PeerCertificates[0].Subject.CommonName != "app.test" {
		t.Fatal("wrong certificate")
	}
	redirectClient := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	q, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/path?q=1", httpPort), nil)
	q.Host = "app.test"
	rd, e := redirectClient.Do(q)
	if e != nil {
		t.Fatal(e)
	}
	rd.Body.Close()
	if rd.StatusCode != 308 || rd.Header.Get("Location") != "https://app.test/path?q=1" {
		t.Fatalf("redirect %d %s", rd.StatusCode, rd.Header.Get("Location"))
	}
	r.f.ResetRequests()
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.noWrites()
	id := r.values()["caddy_tls_certificate.app"]["id"].(string)
	r.run(0, "state", "rm", "caddy_tls_certificate.app")
	r.run(0, "import", "-input=false", "-no-color", "caddy_tls_certificate.app", id)
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	// Same-path renewal is external: it must not cause a provider write/reload.
	originalSerial := resp.TLS.PeerCertificates[0].SerialNumber.String()
	certificate(t, dir, "app.test")
	r.f.ResetRequests()
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	r.noWrites()
	client.CloseIdleConnections()
	renewed, e := client.Get(fmt.Sprintf("https://127.0.0.1:%d", r.port))
	if e != nil {
		t.Fatal(e)
	}
	renewed.Body.Close()
	if renewed.TLS.PeerCertificates[0].SerialNumber.String() != originalSerial {
		t.Fatal("same-path renewal unexpectedly reloaded certificate")
	}
	bad := strings.Replace(cfg, cp, filepath.Join(dir, "missing.pem"), 1)
	r.write(bad)
	r.run(1, "apply", "-auto-approve", "-input=false", "-no-color")
	r.write(cfg)
	r.run(0, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	cp2, kp2 := certificate(t, dir, "second.test")
	r.write(strings.ReplaceAll(strings.ReplaceAll(cfg, cp, cp2), kp, kp2))
	r.run(0, "apply", "-auto-approve", "-input=false", "-no-color")
	if r.values()["caddy_tls_certificate.app"]["id"] != id {
		t.Fatal("certificate ID changed")
	}
	r.run(0, "destroy", "-auto-approve", "-input=false", "-no-color")
}
