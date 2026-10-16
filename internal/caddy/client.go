// Package caddy implements bounded, conditional access to Caddy's JSON API.
package caddy

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Value = any
type Path []string
type Options struct {
	Endpoint, CAFile, ClientCertificateFile, ClientKeyFile string
	RequestTimeout                                         time.Duration
	ConflictRetries                                        int
}
type Snapshot struct {
	AfterWrite bool
	Value      Value
	ETag       string
	Path       Path
}
type Client struct {
	options  Options
	http     *http.Client
	endpoint *url.URL
}
type HTTPError struct {
	Status int
	Method string
	Path   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("Caddy %s %s returned HTTP %d; check Caddy logs for details", e.Method, e.Path, e.Status)
}
func IsStatus(err error, status int) bool {
	var e *HTTPError
	return errors.As(err, &e) && e.Status == status
}
func NewClient(o Options) (*Client, error) {
	u, err := url.Parse(o.Endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("endpoint must be an HTTP or HTTPS origin without credentials, path, query or fragment")
	}
	if (o.ClientCertificateFile == "") != (o.ClientKeyFile == "") {
		return nil, errors.New("client certificate and key must be provided together")
	}
	if o.RequestTimeout == 0 {
		o.RequestTimeout = 30 * time.Second
	}
	if o.RequestTimeout < 0 || o.ConflictRetries < 0 {
		return nil, errors.New("timeout must be positive and conflict retries nonnegative")
	}
	tc := &tls.Config{MinVersion: tls.VersionTLS12}
	if o.CAFile != "" {
		data, e := os.ReadFile(o.CAFile)
		if e != nil {
			return nil, errors.New("cannot read CA file")
		}
		pool, e := x509.SystemCertPool()
		if e != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(data) {
			return nil, errors.New("CA file has no valid certificates")
		}
		tc.RootCAs = pool
	}
	if o.ClientCertificateFile != "" {
		cert, e := tls.LoadX509KeyPair(o.ClientCertificateFile, o.ClientKeyFile)
		if e != nil {
			return nil, errors.New("cannot load client certificate/key pair")
		}
		tc.Certificates = []tls.Certificate{cert}
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = tc
	return &Client{options: o, endpoint: u, http: &http.Client{Timeout: o.RequestTimeout, Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) request(ctx context.Context, method string, p Path, v Value, etag string) (*http.Response, error) {
	parts := make([]string, len(p))
	for i, s := range p {
		if s == "" || s == "." || s == ".." || strings.ContainsAny(s, "/\\") {
			return nil, errors.New("invalid API path segment")
		}
		parts[i] = url.PathEscape(s)
	}
	suffix := ""
	if len(p) == 1 && p[0] == "config" {
		suffix = "/"
	}
	var body io.Reader
	if v != nil {
		data, err := json.Marshal(v)
		if err != nil {
			return nil, errors.New("cannot encode configuration")
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.endpoint.String(), "/")+"/"+strings.Join(parts, "/")+suffix, body)
	if err != nil {
		return nil, errors.New("cannot construct API request")
	}
	if method != "GET" {
		if etag == "" {
			return nil, errors.New("refusing configuration mutation without ETag")
		}
		req.Header.Set("If-Match", etag)
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Caddy %s request failed: %w", method, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		return nil, &HTTPError{Status: resp.StatusCode, Method: method, Path: req.URL.EscapedPath()}
	}
	return resp, nil
}
func (c *Client) Get(ctx context.Context, p Path) (Snapshot, error) {
	resp, err := c.request(ctx, "GET", p, nil, "")
	if err != nil {
		return Snapshot{}, err
	}
	defer resp.Body.Close()
	var v Value
	dec := json.NewDecoder(io.LimitReader(resp.Body, 16<<20))
	dec.UseNumber()
	if err = dec.Decode(&v); err != nil {
		return Snapshot{}, errors.New("Caddy returned invalid JSON")
	}
	if dec.Decode(new(any)) != io.EOF {
		return Snapshot{}, errors.New("Caddy returned trailing or oversized JSON")
	}
	return Snapshot{Value: v, ETag: resp.Header.Get("ETag"), Path: p}, nil
}
func (c *Client) Write(ctx context.Context, method string, p Path, v Value, etag string) error {
	if method != "POST" && method != "PUT" && method != "PATCH" && method != "DELETE" {
		return errors.New("unsupported mutation method")
	}
	if len(p) == 0 || (p[0] != "config" && p[0] != "id") || (p[0] == "config" && len(p) == 1) {
		return errors.New("refusing root or non-configuration mutation")
	}
	resp, err := c.request(ctx, method, p, v, etag)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return err
}
