package launcher

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Exercise the actual Linux rules in an unprivileged user/network namespace.
// A veth substitutes for the TAP's guest Ethernet peer; no host rules or VMs are
// modified. TLS substitution itself is exercised separately in credentials tests.
func TestManagedNetworkNamespace(t *testing.T) {
	if os.Getenv("OUTPOST_NETWORK_CHILD") != "1" {
		for _, name := range []string{"unshare", "nft", "ip", "nsenter"} {
			if _, err := exec.LookPath(name); err != nil {
				t.Skip(name + " unavailable")
			}
		}
		probe := exec.Command("unshare", "-Urn", "sh", "-c", "nft add table inet probe; nft delete table inet probe")
		if output, err := probe.CombinedOutput(); err != nil {
			t.Skipf("isolated net-admin unavailable: %v (%s)", err, output)
		}
		command := exec.Command("unshare", "-Urn", os.Args[0], "-test.run=^TestManagedNetworkNamespace$", "-test.v")
		command.Env = []string{"PATH=" + os.Getenv("PATH"), "OUTPOST_NETWORK_CHILD=1"}
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated network checks: %v\n%s", err, output)
		}
		t.Log(string(output))
		return
	}
	run := func(name string, args ...string) {
		t.Helper()
		output, err := exec.Command(name, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v (%s)", name, args, err, output)
		}
	}
	run("ip", "link", "set", "lo", "up")
	if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	guest := exec.Command("unshare", "-n", "sleep", "120")
	if err := guest.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { guest.Process.Kill(); guest.Wait() }()
	// Wait until the child has actually entered its network namespace.
	var ready bool
	for i := 0; i < 100; i++ {
		a, _ := os.Readlink(fmt.Sprintf("/proc/%d/ns/net", guest.Process.Pid))
		b, _ := os.Readlink("/proc/self/ns/net")
		if a != "" && a != b {
			ready = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatal("guest namespace not ready")
	}
	pid := strconv.Itoa(guest.Process.Pid)
	guestRun := func(args ...string) {
		t.Helper()
		argv := append([]string{"-t", pid, "-n", "ip"}, args...)
		run("nsenter", argv...)
	}
	r := testRuntime(t, nil)
	m := seedVM(t, r, testSpec(), true)
	run("ip", "link", "add", m.Tap, "type", "veth", "peer", "name", "guest0")
	run("ip", "link", "set", "guest0", "netns", pid)
	run("ip", "addr", "add", m.Gateway+"/30", "dev", m.Tap)
	run("ip", "link", "set", m.Tap, "up")
	guestRun("link", "set", "lo", "up")
	guestRun("addr", "add", m.GuestIP+"/30", "dev", "guest0")
	guestRun("link", "set", "guest0", "up")
	guestRun("route", "add", "default", "via", m.Gateway)
	// Put a live synthetic upstream in a separate namespace, so bypass tests
	// exercise FORWARD as well as INPUT rather than failing for lack of a route.
	outside := exec.Command("unshare", "-n", "sleep", "120")
	if err := outside.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { outside.Process.Kill(); outside.Wait() }()
	outsidePID := strconv.Itoa(outside.Process.Pid)
	for i := 0; i < 100; i++ {
		a, _ := os.Readlink("/proc/" + outsidePID + "/ns/net")
		b, _ := os.Readlink("/proc/self/ns/net")
		if a != "" && a != b {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	run("ip", "link", "add", "uplink0", "type", "veth", "peer", "name", "outside0")
	run("ip", "link", "set", "outside0", "netns", outsidePID)
	run("ip", "addr", "add", "198.18.0.1/30", "dev", "uplink0")
	run("ip", "link", "set", "uplink0", "up")
	outsideRun := func(args ...string) {
		t.Helper()
		run("nsenter", append([]string{"-t", outsidePID, "-n", "ip"}, args...)...)
	}
	outsideRun("link", "set", "lo", "up")
	outsideRun("addr", "add", "198.18.0.2/30", "dev", "outside0")
	outsideRun("link", "set", "outside0", "up")
	outsideRun("addr", "add", "203.0.113.8/32", "dev", "lo")
	outsideRun("route", "add", "default", "via", "198.18.0.1")
	run("ip", "route", "add", "203.0.113.8/32", "via", "198.18.0.2")
	run("ip", "-6", "addr", "add", "fc00::1/64", "dev", m.Tap, "nodad")
	guestRun("-6", "addr", "add", "fc00::2/64", "dev", "guest0", "nodad")
	// A live original destination would accept the guest if mediation were bypassed.
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readyRead.Close()
	callsRead, callsWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer callsRead.Close()
	upstream := exec.Command("nsenter", "-t", outsidePID, "-n", os.Args[0], "-test.run=^TestManagedNetworkUpstream$")
	upstream.Env = []string{"PATH=" + os.Getenv("PATH"), "OUTPOST_NETWORK_UPSTREAM=1"}
	upstream.ExtraFiles = []*os.File{readyWrite, callsWrite}
	if err := upstream.Start(); err != nil {
		readyWrite.Close()
		callsWrite.Close()
		t.Fatal(err)
	}
	readyWrite.Close()
	callsWrite.Close()
	defer func() { upstream.Process.Kill(); upstream.Wait() }()
	readyRead.SetReadDeadline(time.Now().Add(5 * time.Second))
	signal := make([]byte, 1)
	if _, err := io.ReadFull(readyRead, signal); err != nil || signal[0] != 'R' {
		t.Fatalf("upstream fixture startup: %v", err)
	}
	proxy, err := net.Listen("tcp4", m.Gateway+":18443")
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	go func() {
		for {
			c, e := proxy.Accept()
			if e != nil {
				return
			}
			c.Write([]byte("mediated"))
			c.Close()
		}
	}()
	service, err := net.Listen("tcp4", m.Gateway+":18444")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	go func() {
		for {
			c, e := service.Accept()
			if e != nil {
				return
			}
			c.Write([]byte("host-service"))
			c.Close()
		}
	}()
	v6, err := net.Listen("tcp6", "[fc00::1]:18444")
	if err != nil {
		t.Fatal(err)
	}
	defer v6.Close()
	go func() {
		for {
			c, e := v6.Accept()
			if e != nil {
				return
			}
			c.Write([]byte("ipv6-bypass"))
			c.Close()
		}
	}()
	resolver, err := net.ListenPacket("udp4", m.Gateway+":53")
	if err != nil {
		t.Fatal(err)
	}
	defer resolver.Close()
	go func() {
		buffer := make([]byte, 512)
		for {
			_, addr, err := resolver.ReadFrom(buffer)
			if err != nil {
				return
			}
			resolver.WriteTo([]byte("dns-fixture"), addr)
		}
	}()
	quic, err := net.ListenPacket("udp4", m.Gateway+":443")
	if err != nil {
		t.Fatal(err)
	}
	defer quic.Close()
	go func() {
		buffer := make([]byte, 512)
		for {
			_, addr, err := quic.ReadFrom(buffer)
			if err != nil {
				return
			}
			quic.WriteTo([]byte("udp-bypass"), addr)
		}
	}()
	// Explicitly configured local resolvers are the one host UDP exception.
	r.config.DNS = m.Gateway
	if err := r.managedNetworkUp(t.Context(), m); err != nil {
		t.Fatal(err)
	}
	dial := func(label, address, source, expected string) {
		t.Helper()
		cmd := exec.Command("nsenter", "-t", pid, "-n", os.Args[0], "-test.run=^TestManagedNetworkDial$", "-test.v")
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "OUTPOST_NETWORK_DIAL=" + address, "OUTPOST_NETWORK_SOURCE=" + source, "OUTPOST_NETWORK_EXPECT=" + expected}
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", label, err, output)
		}
		t.Log("PASS: " + label)
	}
	dial("TCP/443 transparently redirected", "203.0.113.8:443", "", "mediated")
	dial("own proxy reachable", m.Gateway+":18443", "", "mediated")
	dial("configured resolver UDP allowed", "udp:"+m.Gateway+":53", "", "dns-fixture")
	dial("other UDP/QUIC blocked", "udp:"+m.Gateway+":443", "", "denied")
	dial("host service blocked", m.Gateway+":18444", "", "denied")
	dial("alternate port blocked", "203.0.113.8:80", "", "denied")
	dial("IPv6 host access blocked", "[fc00::1]:18444", "", "denied")
	// Spoofed source on the correct ingress interface cannot get a proxy grant.
	guestRun("addr", "add", "172.29.0.2/32", "dev", "guest0")
	dial("spoofed source blocked", m.Gateway+":18443", "172.29.0.2", "denied")
	// A second interface models another VM attempting to reach this gateway.
	run("ip", "link", "add", "other0", "type", "veth", "peer", "name", "otherguest")
	run("ip", "link", "set", "otherguest", "netns", pid)
	run("ip", "addr", "add", "172.29.1.1/30", "dev", "other0")
	run("ip", "link", "set", "other0", "up")
	guestRun("addr", "add", "172.29.1.2/30", "dev", "otherguest")
	guestRun("link", "set", "otherguest", "up")
	guestRun("route", "add", m.Gateway+"/32", "via", "172.29.1.1", "dev", "otherguest")
	dial("cross-interface proxy access blocked", m.Gateway+":18443", "172.29.1.2", "denied")
	guestRun("route", "del", m.Gateway+"/32")
	proxy.Close()
	dial("proxy death does not fall back to upstream", "203.0.113.8:443", "", "denied")
	callsRead.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if n, _ := callsRead.Read(signal); n != 0 {
		t.Fatal("request bypassed mediation")
	}
	if err := r.networkDown(t.Context(), m); err != nil {
		t.Fatal(err)
	}
}

func TestManagedNetworkUpstream(t *testing.T) {
	if os.Getenv("OUTPOST_NETWORK_UPSTREAM") != "1" {
		t.Skip("isolated upstream test helper")
	}
	ready, calls := os.NewFile(3, "ready"), os.NewFile(4, "calls")
	for _, address := range []string{"203.0.113.8:443", "203.0.113.8:80"} {
		listener, err := net.Listen("tcp4", address)
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				calls.Write([]byte("A"))
				conn.Write([]byte("bypassed"))
				conn.Close()
			}
		}()
	}
	ready.Write([]byte("R"))
	ready.Close()
	select {} // terminated by the parent after the network checks
}

func TestManagedNetworkDial(t *testing.T) {
	address := os.Getenv("OUTPOST_NETWORK_DIAL")
	if address == "" {
		t.Skip("isolated network test helper")
	}
	dialer := net.Dialer{Timeout: 750 * time.Millisecond}
	if source := os.Getenv("OUTPOST_NETWORK_SOURCE"); source != "" {
		dialer.LocalAddr = &net.TCPAddr{IP: net.ParseIP(source)}
	}
	network := "tcp"
	if strings.HasPrefix(address, "udp:") {
		network = "udp"
		address = strings.TrimPrefix(address, "udp:")
	}
	conn, err := dialer.Dial(network, address)
	expected := os.Getenv("OUTPOST_NETWORK_EXPECT")
	if network == "udp" && err == nil {
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(750 * time.Millisecond))
		conn.Write([]byte("probe"))
		buffer := make([]byte, 512)
		n, readErr := conn.Read(buffer)
		if expected == "denied" {
			if readErr == nil {
				t.Fatal("unexpected UDP response")
			}
			return
		}
		if readErr != nil || string(buffer[:n]) != expected {
			t.Fatalf("UDP response %q: %v", buffer[:n], readErr)
		}
		return
	}
	if expected == "denied" {
		if err == nil {
			conn.Close()
			t.Fatal("unexpected connection")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(time.Second))
	data, err := io.ReadAll(conn)
	if err != nil || strings.TrimSpace(string(data)) != expected {
		t.Fatalf("response %q: %v", data, err)
	}
}
