package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/client"
	"github.com/nishantdania/outpost/internal/gateway"
	"github.com/nishantdania/outpost/internal/outpost"
)

func TestHostAPIRoutesRequireAuthenticationAndRejectInvalidOrConflictingRequests(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.Create(context.Background(), "dev"); err != nil {
		t.Fatal(err)
	}
	router := newRouter(testHandler(t, store).service, "control-token")
	for _, input := range []struct {
		method, path, body string
		status             int
		authenticated      bool
	}{
		{"GET", "/v1/hosts", "", 401, false},
		{"PUT", "/v1/hosts/app.example.com", `{"outpost_name":"dev","port":3000}`, 401, false},
		{"GET", "/v1/hosts/app.example.com", "", 401, false},
		{"DELETE", "/v1/hosts/app.example.com", "", 401, false},
		{"PUT", "/v1/hosts/app.example.com", `{"outpost_name":"dev","port":3000}`, 200, true},
		{"PUT", "/v1/hosts/APP.example.com", `{"outpost_name":"dev","port":3000}`, 200, true},
		{"PUT", "/v1/hosts/app.example.com", `{"outpost_name":"dev","port":3101}`, 409, true},
		{"PUT", "/v1/hosts/other.example.com", `{"outpost_name":"missing","port":3000}`, 404, true},
		{"PUT", "/v1/hosts/other.example.com", `{"outpost_name":"dev","port":0}`, 400, true},
		{"PUT", "/v1/hosts/other.example.com", `{"outpost_name":"dev","port":3000,"extra":true}`, 400, true},
		{"PUT", "/v1/hosts/other.example.com", `{"outpost_name":"dev","port":3000} {}`, 400, true},
		{"PUT", "/v1/hosts/*.example.com", `{"outpost_name":"dev","port":3000}`, 400, true},
		{"GET", "/v1/hosts/app.example.com", "", 200, true},
		{"GET", "/v1/hosts", "", 200, true},
		{"DELETE", "/v1/hosts/app.example.com", "", 204, true},
		{"GET", "/v1/hosts/app.example.com", "", 404, true},
	} {
		request := httptest.NewRequest(input.method, input.path, strings.NewReader(input.body))
		if input.authenticated {
			request.Header.Set("Authorization", "Bearer control-token")
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != input.status {
			t.Fatalf("%s %s: %d %s, want %d", input.method, input.path, recorder.Code, recorder.Body.String(), input.status)
		}
	}
}

func TestHostClientAndGatewayThroughRealHTTP(t *testing.T) {
	store := newTestStore(t)
	vm, err := store.Create(context.Background(), "dev")
	if err != nil {
		t.Fatal(err)
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "app.example.com" {
			t.Errorf("host rewritten: %q", r.Host)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("control API token leaked to guest")
		}
		fmt.Fprint(w, "guest response")
	}))
	defer backend.Close()
	parsed, _ := url.Parse(backend.URL)
	ip, port, _ := net.SplitHostPort(parsed.Host)
	value, _ := strconv.Atoi(port)
	if _, err := store.SetState(context.Background(), vm.ID, outpost.DesiredRunning, outpost.StatusRunning, ip, ""); err != nil {
		t.Fatal(err)
	}
	control := httptest.NewServer(newRouter(testHandler(t, store).service, "private-control-token"))
	defer control.Close()
	application, err := client.New(control.URL, "private-control-token")
	if err != nil {
		t.Fatal(err)
	}
	registered, err := application.SetHost(context.Background(), "app.example.com", "dev", value)
	if err != nil || registered.OutpostId != vm.ID {
		t.Fatalf("registered = %#v, %v", registered, err)
	}
	hosts, err := application.Hosts(context.Background())
	if err != nil || len(hosts) != 1 {
		t.Fatalf("hosts = %#v, %v", hosts, err)
	}
	handler := gateway.New(application)
	defer handler.Close()
	public := httptest.NewServer(handler)
	defer public.Close()
	request, _ := http.NewRequest(http.MethodGet, public.URL, nil)
	request.Host = "app.example.com"
	response, err := public.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || string(body) != "guest response" {
		t.Fatalf("public = %d %q", response.StatusCode, body)
	}
	if err := application.Unhost(context.Background(), "app.example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := application.Host(context.Background(), "app.example.com"); !errors.Is(err, outpost.ErrHostNotFound) {
		t.Fatalf("removed host = %v", err)
	}
	response, err = public.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 404 {
		t.Fatalf("removed host still exposed: %d", response.StatusCode)
	}
}

func TestHostsListJSONContainsNoSSHOrControlCredentials(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.Create(context.Background(), "dev"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetHost(context.Background(), "app.example.com", "dev", 3000); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/v1/hosts", nil)
	request.Header.Set("Authorization", "Bearer secret-control-token")
	recorder := httptest.NewRecorder()
	newRouter(testHandler(t, store).service, "secret-control-token").ServeHTTP(recorder, request)
	var hosts []api.Host
	if err := json.Unmarshal(recorder.Body.Bytes(), &hosts); err != nil || len(hosts) != 1 {
		t.Fatalf("hosts JSON: %v", err)
	}
	if strings.Contains(recorder.Body.String(), "secret-control-token") || strings.Contains(recorder.Body.String(), "ssh_public_key") {
		t.Fatal("credential data included in host listing")
	}
}
