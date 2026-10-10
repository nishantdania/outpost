// Package gateway serves registered VM hostnames behind a trusted local tunnel
// connector. It has no Cloudflare/ngrok dependency and never exposes the control API.
package gateway

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nishantdania/outpost/internal/outpost"
)

type Resolver interface {
	Host(context.Context, string) (outpost.Host, error)
}

type Handler struct {
	resolver  Resolver
	transport *http.Transport
}

func New(resolver Resolver) *Handler {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Guest traffic must not go through a desktop's environment HTTP proxy.
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.ResponseHeaderTimeout = 30 * time.Second
	return &Handler{resolver: resolver, transport: transport}
}

func (h *Handler) Close() {
	h.transport.CloseIdleConnections()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect || r.URL.IsAbs() {
		http.Error(w, "proxy requests are not supported", http.StatusBadRequest)
		return
	}
	hostname := r.Host
	if strings.Contains(hostname, ":") {
		var err error
		hostname, _, err = net.SplitHostPort(hostname)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	hostname, err := outpost.CanonicalHostname(hostname)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// Only resolution has a short deadline. Uploads, SSE and WebSockets retain
	// their original context and are not subject to a whole-request timeout.
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	host, err := h.resolver.Host(ctx, hostname)
	cancel()
	if errors.Is(err, outpost.ErrHostNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "host registry unavailable", http.StatusServiceUnavailable)
		return
	}
	ip := net.ParseIP(host.GuestIP)
	if host.Status != outpost.StatusRunning || host.DesiredState != outpost.DesiredRunning || ip == nil || host.Port < 1 || host.Port > 65535 {
		http.Error(w, "VM is not available", http.StatusServiceUnavailable)
		return
	}
	target := &url.URL{Scheme: "http", Host: net.JoinHostPort(ip.String(), strconv.Itoa(host.Port))}
	proxy := &httputil.ReverseProxy{
		Transport:     h.transport,
		FlushInterval: -1,
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetURL(target)
			request.Out.Host = request.In.Host
			// Rewrite removes inbound Forwarded/X-Forwarded headers. Preserve
			// provider metadata only from the local connector, never remote peers.
			trusted := loopbackPeer(request.In.RemoteAddr)
			if trusted && validForwardedFor(request.In.Header.Get("X-Forwarded-For")) {
				request.Out.Header.Set("X-Forwarded-For", request.In.Header.Get("X-Forwarded-For"))
			}
			request.SetXForwarded()
			request.Out.Header.Del("X-Forwarded-Port")
			if trusted && request.In.Header.Get("X-Forwarded-Proto") == "https" {
				request.Out.Header.Set("X-Forwarded-Proto", "https")
			}
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			// No upstream addresses, URLs, query strings or credentials in public errors.
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)
}

func loopbackPeer(address string) bool {
	host, _, err := net.SplitHostPort(address)
	ip := net.ParseIP(host)
	return err == nil && ip != nil && ip.IsLoopback()
}

func validForwardedFor(value string) bool {
	if value == "" || len(value) > 1024 {
		return false
	}
	addresses := strings.Split(value, ",")
	if len(addresses) > 16 {
		return false
	}
	for _, address := range addresses {
		if net.ParseIP(strings.TrimSpace(address)) == nil {
			return false
		}
	}
	return true
}
