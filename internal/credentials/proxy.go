package credentials

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

type Grant struct {
	Epoch   string
	ID      string
	Profile Profile
}
type Authorize func(context.Context, string) (Grant, error)
type Proxy struct {
	Host      *Host
	Authorize Authorize
	slots     chan struct{}
	transport func() *http.Transport // private test seam; production always uses publicDial
}

func NewProxy(host *Host, authorize Authorize) *Proxy {
	return &Proxy{Host: host, Authorize: authorize, slots: make(chan struct{}, 64), transport: func() *http.Transport {
		return &http.Transport{Proxy: nil, DisableCompression: true, DisableKeepAlives: true, ForceAttemptHTTP2: false, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 30 * time.Second, DialContext: publicDial}
	}}
}

// Server accepts TLS directly: the launcher redirects guest TCP/443 here. There
// is intentionally no CONNECT/SOCKS/raw tunnel or guest administration endpoint.
func (p *Proxy) Server() *http.Server {
	return &http.Server{Handler: p, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 64 << 10, ErrorLog: log.New(io.Discard, "", 0), TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){}, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}, GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		select {
		case p.slots <- struct{}{}:
			defer func() { <-p.slots }()
		default:
			return nil, ErrUnavailable
		}
		ip, _, err := net.SplitHostPort(hello.Conn.RemoteAddr().String())
		if err != nil {
			return nil, ErrUnavailable
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		grant, err := p.Authorize(ctx, ip)
		if err != nil || !grant.Profile.hostAllowed(strings.ToLower(hello.ServerName)) || !gatewayPeer(ip, hello.Conn.LocalAddr().String()) {
			return nil, ErrUnavailable
		}
		return p.Host.certificate(strings.ToLower(hello.ServerName))
	}}}
}
func gatewayPeer(ip, local string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is4() {
		return false
	}
	host, _, err := net.SplitHostPort(local)
	if err != nil {
		return false
	}
	value := addr.As4()
	if value[3] == 0 {
		return false
	}
	value[3]--
	return netip.AddrFrom4(value).String() == host
}
func (p Profile) hostAllowed(host string) bool {
	if p.AllowedHosts == nil {
		return validHost(host)
	}
	for _, h := range p.AllowedHosts {
		if h == host {
			return true
		}
	}
	for _, b := range p.Credentials {
		if b.Host == host {
			return true
		}
	}
	return false
}
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	select {
	case p.slots <- struct{}{}:
		defer func() { <-p.slots }()
	default:
		http.Error(w, "egress busy", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	deny := func() { http.Error(w, "credential egress denied", 403) }
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		deny()
		return
	}
	grant, err := p.Authorize(ctx, ip)
	host := strings.ToLower(r.Host)
	if strings.HasSuffix(host, ":443") {
		host = strings.TrimSuffix(host, ":443")
	}
	if err != nil || r.TLS == nil || strings.ToLower(r.TLS.ServerName) != host || !grant.Profile.hostAllowed(host) || r.Method == "CONNECT" || r.URL.IsAbs() || r.URL.User != nil || r.Header.Get("Upgrade") != "" {
		deny()
		return
	}
	local, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok || !gatewayPeer(ip, local.String()) {
		deny()
		return
	}
	request := r.Clone(ctx)
	request.Header = r.Header.Clone()
	request.RequestURI = ""
	request.URL.Scheme = "https"
	request.URL.Host = host
	request.Host = host
	slots := map[string][]Binding{}
	for _, binding := range grant.Profile.Credentials {
		if binding.Host == host {
			header := http.CanonicalHeaderKey(binding.Header)
			slots[header] = append(slots[header], binding)
		}
	}
	var selected []Binding
	for header, bindings := range slots {
		values := request.Header.Values(header)
		if len(values) == 0 {
			continue
		}
		if len(values) != 1 {
			deny()
			return
		}
		var chosen *Binding
		for i := range bindings {
			b := &bindings[i]
			if values[0] == b.Value(Placeholder(b.Secret, b.Env)) && b.Allows(r) {
				chosen = b
				break
			}
		}
		if chosen == nil {
			deny()
			return
		}
		for _, value := range request.Header.Values("Connection") {
			for _, name := range strings.Split(value, ",") {
				if strings.EqualFold(strings.TrimSpace(name), header) {
					deny()
					return
				}
			}
		}
		selected = append(selected, *chosen)
		// Validate all other locations before resolving any host secrets.
		request.Header.Del(header)
	}
	// No recognized reference may escape in an unbound location. Also detect Basic
	// encoding of a reference, not just plaintext header bytes.
	for _, values := range request.Header {
		for _, value := range values {
			decoded := []byte{}
			if strings.HasPrefix(value, "Basic ") {
				decoded, _ = base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "Basic "))
			}
			if strings.Contains(value, "outpost_ref_") || strings.Contains(string(decoded), "outpost_ref_") {
				deny()
				return
			}
			for _, b := range grant.Profile.Credentials {
				if value == b.Value(Placeholder(b.Secret, b.Env)) {
					deny()
					return
				}
			}
		}
	}
	body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
	if e != nil {
		http.Error(w, "request too large", 413)
		return
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.ContentLength = int64(len(body))
	var redactions []string
	for _, binding := range selected {
		secret, e := p.Host.Secret(binding.Secret)
		if e != nil {
			http.Error(w, "credential unavailable", 503)
			return
		}
		actual := binding.Value(secret)
		request.Header.Set(binding.Header, actual)
		redactions = append(redactions, secret, actual, base64.StdEncoding.EncodeToString([]byte(secret)))
		if binding.Encoding == "basic" {
			redactions = append(redactions, strings.TrimPrefix(actual, "Basic "))
		}
	}
	stripHop(request.Header)
	request.Header.Set("Accept-Encoding", "identity")
	transport := p.transport()
	defer transport.CloseIdleConnections()
	response, e := transport.RoundTrip(request)
	if e != nil {
		http.Error(w, "upstream unavailable", 502)
		return
	}
	defer response.Body.Close()
	if enc := response.Header.Get("Content-Encoding"); len(redactions) > 0 && enc != "" && enc != "identity" {
		http.Error(w, "unsupported upstream encoding", 502)
		return
	}
	stripHop(response.Header)
	if len(redactions) > 0 {
		for _, name := range []string{"Content-Length", "ETag", "Content-Md5", "Content-Encoding", "Set-Cookie"} {
			response.Header.Del(name)
		}
	}
	if response.ContentLength > 64<<20 {
		http.Error(w, "upstream response too large", 502)
		return
	}
	for name, values := range response.Header {
		for _, v := range values {
			w.Header().Add(name, redact(v, redactions))
		}
	}
	w.WriteHeader(response.StatusCode)
	// Keep a small suffix so literal secret echoes split across network chunks are
	// removed. Arbitrary transformations/encodings by a malicious upstream are not
	// a security guarantee; credential destinations must be trusted.
	pending := []byte{}
	buffer := make([]byte, 32<<10)
	var total int64
	for {
		current, e := p.Authorize(ctx, ip)
		if e != nil || current.ID != grant.ID || current.Epoch != grant.Epoch || ctx.Err() != nil {
			return
		}
		n, readErr := response.Body.Read(buffer)
		current, e = p.Authorize(ctx, ip)
		if e != nil || current.ID != grant.ID || current.Epoch != grant.Epoch || ctx.Err() != nil {
			return
		}
		total += int64(n)
		if total > 64<<20 {
			return
		}
		pending = append(pending, buffer[:n]...)
		// Redact complete occurrences before releasing a prefix; keep replacements
		// length-preserving here to avoid splitting a replacement boundary.
		for _, secret := range redactions {
			if secret != "" {
				pending = []byte(strings.ReplaceAll(string(pending), secret, strings.Repeat("*", len(secret))))
			}
		}
		emit := len(pending) - secretSuffix(pending, redactions)
		if readErr != nil {
			emit = len(pending)
		}
		if emit > 0 {
			if _, e = w.Write(pending[:emit]); e != nil {
				return
			}
			pending = append([]byte(nil), pending[emit:]...)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		if readErr != nil {
			return
		}
	}
}

// Retain only a suffix that could become a secret when the next chunk arrives.
// Ordinary short SSE frames flush immediately rather than waiting for a full key length.
func secretSuffix(data []byte, secrets []string) int {
	retain := 0
	for _, secret := range secrets {
		limit := len(secret) - 1
		if len(data) < limit {
			limit = len(data)
		}
		for n := limit; n > retain; n-- {
			if string(data[len(data)-n:]) == secret[:n] {
				retain = n
				break
			}
		}
	}
	return retain
}

func redact(value string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
}
func stripHop(h http.Header) {
	for _, value := range h.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			h.Del(strings.TrimSpace(name))
		}
	}
	for _, name := range []string{"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailer", "Transfer-Encoding", "Upgrade"} {
		h.Del(name)
	}
}

var blocked = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("192.168.0.0/16"), netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("224.0.0.0/3"),
}

func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	// IPv4-only upstream initially: avoids IPv6 translation/metadata edge cases.
	if !ip.Is4() {
		return false
	}
	for _, prefix := range blocked {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
func publicDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != strconv.Itoa(443) {
		return nil, ErrUnavailable
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, ErrUnavailable
	}
	// Reject mixed public/private answers; dial the checked IP, not the hostname.
	var v4 []netip.Addr
	for _, ip := range ips {
		ip = ip.Unmap()
		if !ip.Is4() {
			if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				return nil, ErrUnavailable
			}
			continue
		}
		if !publicIP(ip) {
			return nil, ErrUnavailable
		}
		v4 = append(v4, ip)
	}
	for _, ip := range v4 {
		conn, e := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), port))
		if e == nil {
			return conn, nil
		}
	}
	return nil, errors.New("upstream connection unavailable")
}
