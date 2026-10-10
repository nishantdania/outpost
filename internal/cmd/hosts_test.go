package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nishantdania/outpost/internal/api"
)

func TestHostCommandsUsePositionalVMPortAndAuthenticatedAPI(t *testing.T) {
	t.Setenv("OUTPOST_TOKEN", "test-control-token")
	host := api.Host{Hostname: "main-dev.example.com", OutpostId: "vm-id", OutpostName: "dev", Port: 3000, Status: "stopped", DesiredState: "stopped"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-control-token" {
			t.Errorf("missing authentication")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "PUT" && r.URL.Path == "/v1/hosts/main-dev.example.com":
			var input api.SetHostRequest
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.OutpostName != "dev" || input.Port != 3000 {
				t.Errorf("request = %#v %v", input, err)
			}
			json.NewEncoder(w).Encode(host)
		case r.Method == "GET" && r.URL.Path == "/v1/hosts":
			json.NewEncoder(w).Encode([]api.Host{host})
		case r.Method == "DELETE" && r.URL.Path == "/v1/hosts/main-dev.example.com":
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected API request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	for _, args := range [][]string{{"host", "MAIN-DEV.example.com", "dev:3000"}, {"hosts"}, {"unhost", "main-dev.example.com"}} {
		root := newRootCmd()
		root.SetArgs(append([]string{"--server", server.URL}, args...))
		var output bytes.Buffer
		root.SetOut(&output)
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), host.Hostname) || strings.Contains(output.String(), "test-control-token") {
			t.Fatalf("output = %q", output.String())
		}
	}
}

func TestHostCommandRejectsUnsafeOrInvalidTargetsBeforeCallingServer(t *testing.T) {
	for _, args := range [][]string{
		{"host", "*.example.com", "dev:3000"},
		{"host", "app.example.com", "127.0.0.1:3000"},
		{"host", "app.example.com", "dev:0"},
		{"host", "app.example.com", "dev:65536"},
		{"host", "app.example.com", "dev"},
		{"gateway", "--listen", "0.0.0.0:17891"},
	} {
		root := newRootCmd()
		root.SetArgs(append([]string{"--server", "http://127.0.0.1:1"}, args...))
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		if err := root.Execute(); err == nil {
			t.Errorf("accepted invalid arguments %v", args)
		}
	}
}

type gatewayReadyWriter struct{ ready chan string }

func (w gatewayReadyWriter) Write(body []byte) (int, error) {
	if text, ok := strings.CutPrefix(string(body), "Gateway listening on http://"); ok {
		address, _, _ := strings.Cut(text, ";")
		w.ready <- address
	}
	return len(body), nil
}

func TestGatewayCommandServesOnLoopbackAndShutsDownOnCancellation(t *testing.T) {
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "guest") }))
	defer guest.Close()
	ip, text, _ := net.SplitHostPort(guest.Listener.Addr().String())
	port, _ := strconv.Atoi(text)
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-control-token" {
			t.Errorf("missing control credentials")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.Host{Hostname: "app.example.com", OutpostId: "vm-id", OutpostName: "dev", Port: port, GuestIp: ip, Status: "running", DesiredState: "running"})
	}))
	defer control.Close()
	cmd := newGatewayCmd(&rootOptions{serverURL: control.URL, token: "test-control-token"})
	cmd.SetArgs([]string{"--listen", "127.0.0.1:0"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd.SetContext(ctx)
	ready := make(chan string, 1)
	cmd.SetOut(gatewayReadyWriter{ready: ready})
	finished := make(chan error, 1)
	go func() { finished <- cmd.Execute() }()
	var address string
	select {
	case address = <-ready:
	case err := <-finished:
		t.Fatalf("early exit: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("gateway did not start")
	}
	request, _ := http.NewRequest("GET", "http://"+address, nil)
	request.Host = "app.example.com"
	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || string(body) != "guest" {
		t.Fatalf("response = %d %q", response.StatusCode, body)
	}
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("gateway did not stop")
	}
}
