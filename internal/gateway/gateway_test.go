package gateway

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nishantdania/outpost/internal/outpost"
)

type resolverFunc func(context.Context, string) (outpost.Host, error)

func (f resolverFunc) Host(ctx context.Context, name string) (outpost.Host, error) {
	return f(ctx, name)
}

func targetHost(t *testing.T, server *httptest.Server) outpost.Host {
	t.Helper()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	ip, port, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	value, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	return outpost.Host{Hostname: "app.example.com", GuestIP: ip, Port: value, Status: outpost.StatusRunning, DesiredState: outpost.DesiredRunning}
}

func TestGatewayPreservesHTTPAndUpdatesRoutesWithoutRestart(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Host != "app.example.com" || r.URL.EscapedPath() != "/git/a%2Fb" || r.URL.RawQuery != "service=git-upload-pack" || string(body) != "payload" {
			t.Errorf("upstream request = host %q path %q query %q body %q", r.Host, r.URL.EscapedPath(), r.URL.RawQuery, body)
		}
		if r.Header.Get("X-Forwarded-Proto") != "https" || r.Header.Get("X-Forwarded-Host") != "app.example.com" {
			t.Errorf("forwarded headers = %v", r.Header)
		}
		if r.Header.Get("Authorization") != "Bearer application-token" {
			t.Errorf("app authorization missing")
		}
		if strings.Contains(r.Header.Get("X-Forwarded-For"), "bad-header") {
			t.Errorf("invalid forwarded metadata copied")
		}
		fmt.Fprint(w, "first")
	}))
	defer upstream.Close()
	var current atomic.Value
	current.Store(targetHost(t, upstream))
	handler := New(resolverFunc(func(_ context.Context, name string) (outpost.Host, error) {
		if name != "app.example.com" {
			return outpost.Host{}, outpost.ErrHostNotFound
		}
		return current.Load().(outpost.Host), nil
	}))
	defer handler.Close()
	server := httptest.NewServer(handler)
	defer server.Close()
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/git/a%2Fb?service=git-upload-pack", strings.NewReader("payload"))
	request.Host = "app.example.com"
	request.Header.Set("Authorization", "Bearer application-token")
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Forwarded-Host", "attacker.example.com")
	request.Header.Set("X-Forwarded-For", "bad-header")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || string(body) != "first" {
		t.Fatalf("response = %d %q", response.StatusCode, body)
	}

	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "second") }))
	defer second.Close()
	current.Store(targetHost(t, second))
	request, _ = http.NewRequest(http.MethodGet, server.URL, nil)
	request.Host = "APP.EXAMPLE.COM:443"
	response, err = server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(response.Body)
	response.Body.Close()
	if string(body) != "second" {
		t.Fatalf("route did not update: %q", body)
	}
	stopped := current.Load().(outpost.Host)
	stopped.Status = outpost.StatusStopped
	current.Store(stopped)
	response, err = server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("stopped VM = %d", response.StatusCode)
	}
}

func TestGatewayRejectsUnknownHostsAndDoesNotTrustRemoteForwardedHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Forwarded-Proto") != "http" || strings.Contains(r.Header.Get("X-Forwarded-For"), "203.0.113.99") {
			t.Errorf("remote forwarded headers trusted")
		}
		w.WriteHeader(204)
	}))
	defer upstream.Close()
	host := targetHost(t, upstream)
	calls := 0
	handler := New(resolverFunc(func(_ context.Context, name string) (outpost.Host, error) {
		calls++
		if name != host.Hostname {
			return outpost.Host{}, outpost.ErrHostNotFound
		}
		return host, nil
	}))
	defer handler.Close()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Host = host.Hostname
	request.RemoteAddr = "192.0.2.1:1234"
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Forwarded-For", "203.0.113.99")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != 204 {
		t.Fatalf("status = %d", recorder.Code)
	}
	request.Host = "unknown.example.com"
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != 404 {
		t.Fatalf("unknown host = %d", recorder.Code)
	}
	before := calls
	request = httptest.NewRequest(http.MethodConnect, "/", nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != 400 || calls != before {
		t.Fatal("CONNECT was not rejected before resolution")
	}
}

func TestGatewayStreamsSSEBeforeUpstreamCompletes(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: ready\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer upstream.Close()
	handler := New(resolverFunc(func(context.Context, string) (outpost.Host, error) { return targetHost(t, upstream), nil }))
	defer handler.Close()
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	request.Host = "app.example.com"
	response, err := server.Client().Do(request)
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	line, err := bufio.NewReader(response.Body).ReadString('\n')
	close(release)
	response.Body.Close()
	if err != nil || line != "data: ready\n" {
		t.Fatalf("SSE not streamed: %q %v", line, err)
	}
}

func TestGatewaySupportsBidirectionalProtocolUpgrade(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		connection, reader, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer connection.Close()
		connection.SetDeadline(time.Now().Add(3 * time.Second))
		fmt.Fprint(reader, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: outpost-echo\r\n\r\n")
		reader.Flush()
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Error(err)
			return
		}
		fmt.Fprint(reader, line)
		reader.Flush()
	}))
	defer upstream.Close()
	handler := New(resolverFunc(func(context.Context, string) (outpost.Host, error) { return targetHost(t, upstream), nil }))
	defer handler.Close()
	server := httptest.NewServer(handler)
	defer server.Close()
	connection, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	connection.SetDeadline(time.Now().Add(3 * time.Second))
	fmt.Fprint(connection, "GET / HTTP/1.1\r\nHost: app.example.com\r\nConnection: Upgrade\r\nUpgrade: outpost-echo\r\n\r\n")
	reader := bufio.NewReader(connection)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodGet})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 101 {
		t.Fatalf("upgrade = %d", response.StatusCode)
	}
	fmt.Fprint(connection, "ping\n")
	line, err := reader.ReadString('\n')
	if err != nil || line != "ping\n" {
		t.Fatalf("upgrade stream = %q, %v", line, err)
	}
}
