package credentials

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testHost(t *testing.T) *Host {
	t.Helper()
	directory, state := t.TempDir(), t.TempDir()
	for _, dir := range []string{directory, state} {
		if err := os.Chmod(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "test-key"), []byte("synthetic-real-key-value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	host, err := NewHost(directory, state)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := NewHost(directory, state)
	if err != nil || restarted.PublicCA != host.PublicCA {
		t.Fatalf("CA was not preserved: %v", err)
	}
	return host
}

func TestVerifiedTLSSubstitutionAndDenials(t *testing.T) {
	for _, encoding := range []string{"bearer", "raw", "basic", "raw-prefixed"} {
		t.Run(encoding, func(t *testing.T) { testVerifiedTLS(t, encoding) })
	}
}

func testVerifiedTLS(t *testing.T, encoding string) {
	host := testHost(t)
	header := "Authorization"
	if encoding == "raw" {
		header = "X-Api-Key"
	}
	binding := Binding{Env: "API_KEY", Secret: "test-key", Host: "provider.example", Header: header, Encoding: encoding, Operations: []Operation{{Method: "POST", Path: "/check"}}}
	if encoding == "raw-prefixed" {
		binding.Encoding = "raw"
		binding.Prefix = "token "
	}
	alternate := binding
	alternate.Env = "OTHER_API_KEY"
	otherHost := binding
	otherHost.Host = "other.example"
	profile := Profile{Credentials: []Binding{binding, alternate, otherHost}, AllowedHosts: []string{"unbound.example"}}
	if err := profile.Validate(); err != nil {
		t.Fatal(err)
	}
	var live atomic.Bool
	live.Store(true)
	authorize := func(_ context.Context, ip string) (Grant, error) {
		if !live.Load() || ip != "127.0.0.2" {
			return Grant{}, ErrUnavailable
		}
		return Grant{ID: "vm-one", Profile: profile}, nil
	}
	var upstreamCalls atomic.Int32
	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		if r.URL.RawQuery == "publiccookie=1" {
			w.Header().Set("Set-Cookie", "session=public-session; Secure")
			w.Write([]byte("public response"))
			return
		}
		if r.URL.RawQuery == "publiccompressed=1" {
			var data bytes.Buffer
			writer := gzip.NewWriter(&data)
			writer.Write([]byte("public compressed content"))
			writer.Close()
			w.Header().Set("Content-Encoding", "gzip")
			w.Write(data.Bytes())
			return
		}
		if r.Header.Get(header) != binding.Value("synthetic-real-key-value") {
			t.Errorf("bad upstream auth")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "body:"+Placeholder(binding.Secret, binding.Env) {
			t.Errorf("body was rewritten")
		}
		if r.URL.RawQuery == "redirect=1" {
			w.Header().Set("Location", "https://other.example/target")
			w.WriteHeader(302)
			return
		}
		if r.URL.RawQuery == "compressed=1" {
			w.Header().Set("Content-Encoding", "gzip")
			w.Write([]byte("not forwarded"))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Echo", "synthetic-real-key-value")
		w.Write([]byte("data: ready\n\n"))
		w.(http.Flusher).Flush()
		// Force a reflected credential to cross upstream reads.
		w.Write([]byte("synthetic-real-"))
		w.(http.Flusher).Flush()
		time.Sleep(10 * time.Millisecond)
		w.Write([]byte("key-value"))
	}))
	cert, err := host.certificate(binding.Host)
	if err != nil {
		t.Fatal(err)
	}
	upstream.TLS = &tls.Config{Certificates: []tls.Certificate{*cert}, MinVersion: tls.VersionTLS12, GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if hello.ServerName == otherHost.Host {
			return host.certificate(hello.ServerName)
		}
		return cert, nil
	}}
	upstream.StartTLS()
	defer upstream.Close()
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM([]byte(host.PublicCA))
	proxy := NewProxy(host, authorize)
	var untrustedUpstream atomic.Bool
	proxy.transport = func() *http.Transport {
		upstreamRoots := roots
		if untrustedUpstream.Load() {
			upstreamRoots = x509.NewCertPool()
		}
		return &http.Transport{Proxy: nil, DisableCompression: true, TLSClientConfig: &tls.Config{RootCAs: upstreamRoots}, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if address != "provider.example:443" && address != "other.example:443" {
				return nil, errors.New("unexpected upstream")
			}
			return (&net.Dialer{}).DialContext(ctx, "tcp", upstream.Listener.Addr().String())
		}}
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := proxy.Server()
	defer server.Close()
	go server.ServeTLS(listener, "", "")
	clientTransport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots}, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.2")}}).DialContext(ctx, "tcp", listener.Addr().String())
	}}
	client := &http.Client{Transport: clientTransport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer clientTransport.CloseIdleConnections()
	request := func(path string, headers http.Header) *http.Request {
		r, _ := http.NewRequest("POST", "https://provider.example"+path, strings.NewReader("body:"+Placeholder(binding.Secret, binding.Env)))
		r.Header = headers
		return r
	}
	auth := binding.Value(Placeholder(binding.Secret, binding.Env))
	response, err := client.Do(request("/check", http.Header{header: []string{auth}}))
	if err != nil {
		t.Fatal(err)
	}
	// A normal small SSE frame must arrive without waiting for a key-length buffer.
	first := make([]byte, len("data: ready\n\n"))
	if _, err := io.ReadFull(response.Body, first); err != nil || string(first) != "data: ready\n\n" {
		t.Fatalf("SSE frame: %q, %v", first, err)
	}
	rest, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || strings.Contains(string(rest), "synthetic-real-key-value") || response.Header.Get("X-Echo") != "[REDACTED]" {
		t.Fatalf("credential reflection: %q, %v", rest, err)
	}
	if response.StatusCode != 200 || upstreamCalls.Load() != 1 {
		t.Fatal("request did not reach upstream")
	}
	redirect, e := client.Do(request("/check?redirect=1", http.Header{header: []string{auth}}))
	if e != nil {
		t.Fatal(e)
	}
	redirect.Body.Close()
	if redirect.StatusCode != 302 || upstreamCalls.Load() != 2 {
		t.Fatal("proxy followed redirect")
	}
	compressed, e := client.Do(request("/check?compressed=1", http.Header{header: []string{auth}}))
	if e != nil {
		t.Fatal(e)
	}
	compressed.Body.Close()
	if compressed.StatusCode != 502 || upstreamCalls.Load() != 3 {
		t.Fatal("compressed response bypassed filtering")
	}
	public, e := client.Do(request("/check?publiccookie=1", http.Header{}))
	if e != nil {
		t.Fatal(e)
	}
	public.Body.Close()
	if public.Header.Get("Set-Cookie") != "session=public-session; Secure" {
		t.Fatal("public browsing cookies were removed")
	}
	public, e = client.Do(request("/check?publiccompressed=1", http.Header{}))
	if e != nil {
		t.Fatal(e)
	}
	content, e := io.ReadAll(public.Body)
	public.Body.Close()
	if e != nil || string(content) != "public compressed content" {
		t.Fatal("public compressed content was not preserved")
	}
	alternateResponse, e := client.Do(request("/check", http.Header{header: []string{alternate.Value(Placeholder(alternate.Secret, alternate.Env))}}))
	if e != nil {
		t.Fatal(e)
	}
	io.Copy(io.Discard, alternateResponse.Body)
	alternateResponse.Body.Close()
	if alternateResponse.StatusCode != 200 {
		t.Fatal("alternate reference at same auth slot rejected")
	}
	otherRequest := request("/check", http.Header{header: []string{auth}})
	otherRequest.URL.Host = "other.example"
	otherRequest.Host = "other.example"
	otherResponse, e := client.Do(otherRequest)
	if e != nil {
		t.Fatal(e)
	}
	io.Copy(io.Discard, otherResponse.Body)
	otherResponse.Body.Close()
	if otherResponse.StatusCode != 200 {
		t.Fatal("one env reference at another bound host rejected")
	}
	legitimateCalls := int32(7)
	for _, test := range []struct {
		name, path string
		headers    http.Header
		host       string
	}{
		{name: "wrong reference", path: "/check", headers: http.Header{header: []string{"wrong"}}},
		{name: "duplicate auth", path: "/check", headers: http.Header{header: []string{auth, auth}}},
		{name: "wrong route", path: "/other", headers: http.Header{header: []string{auth}}},
		{name: "wrong slot", path: "/check", headers: http.Header{"X-Debug": []string{auth}}},
		{name: "authority mismatch", path: "/check", headers: http.Header{header: []string{auth}}, host: "other.example"},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := request(test.path, test.headers)
			if test.host != "" {
				r.Host = test.host
			}
			result, e := client.Do(r)
			if e != nil {
				t.Fatal(e)
			}
			result.Body.Close()
			if result.StatusCode != 403 {
				t.Fatalf("status %d", result.StatusCode)
			}
		})
	}
	wrongDestination := request("/check", http.Header{header: []string{auth}})
	wrongDestination.URL.Host = "unbound.example"
	wrongDestination.Host = "unbound.example"
	other, e := client.Do(wrongDestination)
	if e != nil {
		t.Fatal(e)
	}
	other.Body.Close()
	if other.StatusCode != 403 {
		t.Fatal("credential used at another network-permitted host")
	}
	if upstreamCalls.Load() != legitimateCalls {
		t.Fatal("denied request reached upstream")
	}
	untrustedUpstream.Store(true)
	badUpstream, e := client.Do(request("/check", http.Header{header: []string{auth}}))
	if e != nil {
		t.Fatal(e)
	}
	badUpstream.Body.Close()
	if badUpstream.StatusCode != 502 {
		t.Fatal("untrusted upstream certificate accepted")
	}
	untrustedUpstream.Store(false)
	untrustedClient := clientTransport.Clone()
	untrustedClient.TLSClientConfig = &tls.Config{RootCAs: x509.NewCertPool()}
	rejected, e := (&http.Client{Transport: untrustedClient, Timeout: 5 * time.Second}).Do(request("/check", http.Header{header: []string{auth}}))
	untrustedClient.CloseIdleConnections()
	if e == nil {
		rejected.Body.Close()
		t.Fatal("guest accepted an untrusted proxy CA")
	}
	live.Store(false)
	result, err := client.Do(request("/check", http.Header{header: []string{auth}}))
	if err == nil {
		result.Body.Close()
		if result.StatusCode != 403 {
			t.Fatal("revoked VM accepted")
		}
	}
	if upstreamCalls.Load() != legitimateCalls {
		t.Fatal("revocation reached upstream")
	}
}

func TestProfileValidationAndEncodedAuth(t *testing.T) {
	for _, encoding := range []string{"bearer", "raw", "basic"} {
		binding := Binding{Env: "API_KEY", Secret: "test-key", Host: "api.example.com", Header: "X-Api-Key", Encoding: encoding}
		p := Profile{Credentials: []Binding{binding}}
		if p.Validate() != nil {
			t.Fatal(encoding)
		}
		parsed, err := Parse([]byte(p.JSON()))
		if err != nil || parsed.Environment() != p.Environment() {
			t.Fatal("profile round trip")
		}
		if binding.Value("value") == "" {
			t.Fatal("empty encoding")
		}
	}
	for _, body := range []string{`{"credentials":[]}`, `{"credentials":[],"value":"secret"}`, `{} {}`, `{"credentials":[{"env":"TOKEN","secret":"../key","host":"api.example.com","header":"Authorization","encoding":"bearer"}]}`} {
		if _, err := Parse([]byte(body)); err == nil {
			t.Fatal("accepted invalid profile")
		}
	}
	b := Binding{Env: "TOKEN", Secret: "key", Host: "api.example.com", Header: "Authorization", Encoding: "bearer"}
	if (Profile{Credentials: []Binding{b, b}}).Validate() == nil {
		t.Fatal("duplicate bindings")
	}
}

func TestPrivateCredentialsAndPublicDestinations(t *testing.T) {
	host := testHost(t)
	if err := os.Symlink(filepath.Join(host.directory, "test-key"), filepath.Join(host.directory, "link")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../test-key", "link", "missing"} {
		if _, err := host.Secret(name); err == nil {
			t.Fatal("read unsafe credential")
		}
	}
	os.Chmod(filepath.Join(host.directory, "test-key"), 0644)
	if _, err := host.Secret("test-key"); err == nil {
		t.Fatal("read public credential")
	}
	for _, value := range []string{"127.0.0.1", "169.254.169.254", "10.0.0.1", "100.64.0.1", "::1", "::ffff:127.0.0.1", "192.0.2.1", "198.18.0.1"} {
		if publicIP(netip.MustParseAddr(value)) {
			t.Fatal("public:", value)
		}
	}
	if !publicIP(netip.MustParseAddr("1.1.1.1")) {
		t.Fatal("public address denied")
	}
}

func TestOneEnvironmentReferenceSupportsMultipleHosts(t *testing.T) {
	api := Binding{Env: "GITHUB_TOKEN", Secret: "github", Host: "api.github.com", Header: "Authorization", Encoding: "bearer"}
	git := api
	git.Host = "github.com"
	git.Encoding = "basic"
	git.Username = "x-access-token"
	p := Profile{Credentials: []Binding{api, git}}
	parsed, err := Parse([]byte(p.JSON()))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(parsed.Environment(), "GITHUB_TOKEN=") != 1 {
		t.Fatal("duplicated environment variable")
	}
	conflicting := git
	conflicting.Secret = "other-key"
	if (Profile{Credentials: []Binding{api, conflicting}}).Validate() == nil {
		t.Fatal("one env name mapped to conflicting secrets")
	}
}

func TestNetworkAllowlistSeparateFromCredentialScopes(t *testing.T) {
	binding := Binding{Env: "TOKEN", Secret: "key", Host: "api.example.com", Header: "Authorization", Encoding: "bearer"}
	for _, test := range []struct {
		hosts   []string
		allowed bool
	}{{hosts: nil, allowed: true}, {hosts: []string{}, allowed: false}, {hosts: []string{"cdn.example.com"}, allowed: true}} {
		p := Profile{Credentials: []Binding{binding}, AllowedHosts: test.hosts}
		parsed, err := Parse([]byte(p.JSON()))
		if err != nil {
			t.Fatal(err)
		}
		if parsed.hostAllowed("cdn.example.com") != test.allowed || !parsed.hostAllowed(binding.Host) {
			t.Fatal("network scope changed during persistence")
		}
	}
}

func TestRedactorSuffix(t *testing.T) {
	if secretSuffix([]byte("data: ready\n\n"), []string{"secret"}) != 0 {
		t.Fatal("ordinary SSE retained")
	}
	if secretSuffix([]byte("data: sec"), []string{"secret"}) != 3 {
		t.Fatal("split secret not retained")
	}
	if gatewayPeer("172.30.0.6", "172.30.0.1:18443") {
		t.Fatal("cross VM gateway accepted")
	}
	if !gatewayPeer("172.30.0.6", "172.30.0.5:18443") {
		t.Fatal("own gateway rejected")
	}
}
