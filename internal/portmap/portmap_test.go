package portmap

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeRouter answers PCP, or only NAT-PMP, on a UDP port on this computer.
type fakeRouter struct {
	conn    *net.UDPConn
	pcp     bool
	mu      sync.Mutex
	removed int // mappings asked away (lifetime 0)
}

func newFakeRouter(t *testing.T, pcp bool) *fakeRouter {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeRouter{conn: conn, pcp: pcp}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1100)
		for {
			n, from, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if resp := f.answer(buf[:n]); resp != nil {
				_, _ = conn.WriteToUDP(resp, from)
			}
		}
	}()
	t.Cleanup(func() { conn.Close(); <-done })
	return f
}

func (f *fakeRouter) addr() netip.AddrPort { return f.conn.LocalAddr().(*net.UDPAddr).AddrPort() }

func (f *fakeRouter) answer(req []byte) []byte {
	switch {
	case len(req) == 60 && req[0] == 2 && req[1] == 1:
		if !f.pcp {
			// A NAT-PMP router: "unsupported version", in its own format.
			return []byte{0, 129, 0, 1, 0, 0, 0, 0}
		}
		resp := make([]byte, 60)
		resp[0], resp[1] = 2, 0x81
		copy(resp[4:8], req[4:8]) // the lifetime asked
		copy(resp[24:44], req[24:44])
		ext := mapped4(netip.MustParseAddr("203.0.113.9"))
		copy(resp[44:60], ext)
		if binary.BigEndian.Uint32(req[4:8]) == 0 {
			f.mu.Lock()
			f.removed++
			f.mu.Unlock()
		}
		return resp
	case len(req) == 2 && req[0] == 0 && req[1] == 0:
		return []byte{0, 128, 0, 0, 0, 0, 0, 1, 198, 51, 100, 4}
	case len(req) == 12 && req[0] == 0 && req[1] == 2:
		resp := make([]byte, 16)
		resp[1] = 130
		copy(resp[8:10], req[4:6])
		copy(resp[10:12], req[6:8])
		copy(resp[12:16], req[8:12])
		if binary.BigEndian.Uint32(req[8:12]) == 0 {
			f.mu.Lock()
			f.removed++
			f.mu.Unlock()
		}
		return resp
	}
	return nil
}

// closedPort is a UDP port on this computer that nothing listens on, so
// PCP and NAT-PMP fail at once instead of reaching a real router.
func closedPort(t *testing.T) netip.AddrPort {
	t.Helper()
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	a := c.LocalAddr().(*net.UDPAddr).AddrPort()
	c.Close()
	return a
}

func TestPCP(t *testing.T) {
	f := newFakeRouter(t, true)
	m, err := Router{Gateway: f.addr(), SkipUPnP: true}.Map(context.Background(), 7333, 0)
	if err != nil {
		t.Fatal(err)
	}
	if m.Method != MethodPCP || m.External != netip.MustParseAddrPort("203.0.113.9:7333") || m.Lifetime != Lifetime {
		t.Fatalf("mapping: %+v", m)
	}
	if err := m.Unmap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.removed != 1 {
		t.Fatalf("removed: %d", f.removed)
	}
}

func TestNATPMPAfterPCPIsRefused(t *testing.T) {
	f := newFakeRouter(t, false)
	m, err := Router{Gateway: f.addr(), SkipUPnP: true}.Map(context.Background(), 7333, 0)
	if err != nil {
		t.Fatal(err)
	}
	if m.Method != MethodNATPMP || m.External != netip.MustParseAddrPort("198.51.100.4:7333") {
		t.Fatalf("mapping: %+v", m)
	}
	if err := m.Unmap(context.Background()); err != nil || f.removed != 1 {
		t.Fatalf("unmap: %v %d", err, f.removed)
	}
}

func TestUPnP(t *testing.T) {
	var mu sync.Mutex
	var actions []string
	mux := http.NewServeMux()
	mux.HandleFunc("/desc.xml", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?><root><device><deviceList><device><deviceList><device><serviceList><service>
<serviceType>urn:schemas-upnp-org:service:WANIPConnection:1</serviceType><controlURL>/ctl/IPConn</controlURL>
</service></serviceList></device></deviceList></device></deviceList></device></root>`)
	})
	mux.HandleFunc("/ctl/IPConn", func(w http.ResponseWriter, r *http.Request) {
		action := strings.Trim(strings.SplitN(r.Header.Get("SOAPAction"), "#", 2)[1], `"`)
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		actions = append(actions, action)
		mu.Unlock()
		if action == "AddPortMapping" && !strings.Contains(string(body), "<NewInternalClient>127.0.0.1</NewInternalClient>") {
			http.Error(w, "<errorCode>402</errorCode>", http.StatusInternalServerError)
			return
		}
		if action == "GetExternalIPAddress" {
			fmt.Fprint(w, `<NewExternalIPAddress>192.0.2.44</NewExternalIPAddress>`)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// A fake SSDP responder pointing at the description.
	ssdp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1024)
		for {
			n, from, err := ssdp.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if strings.HasPrefix(string(buf[:n]), "M-SEARCH") {
				reply := "HTTP/1.1 200 OK\r\nST: urn:schemas-upnp-org:device:InternetGatewayDevice:1\r\nLOCATION: " + srv.URL + "/desc.xml\r\n\r\n"
				_, _ = ssdp.WriteToUDP([]byte(reply), from)
			}
		}
	}()
	defer func() { ssdp.Close(); <-done }()
	old := ssdpAddr
	ssdpAddr = ssdp.LocalAddr().String()
	defer func() { ssdpAddr = old }()

	m, err := Router{Gateway: closedPort(t)}.Map(context.Background(), 7333, 0)
	if err != nil {
		t.Fatal(err)
	}
	if m.Method != MethodUPnP || m.External != netip.MustParseAddrPort("192.0.2.44:7333") {
		t.Fatalf("mapping: %+v", m)
	}
	if err := m.Unmap(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(actions, ",") != "AddPortMapping,GetExternalIPAddress,DeletePortMapping" {
		t.Fatalf("actions: %v", actions)
	}
}

// A description elsewhere than the device that answered isn't followed.
func TestUPnPOnlyTheDeviceThatAnswered(t *testing.T) {
	if sameHost("http://203.0.113.5/desc.xml", netip.MustParseAddr("192.168.1.1")) {
		t.Fatal("followed another host")
	}
	if sameHost("http://203.0.113.5/desc.xml", netip.MustParseAddr("203.0.113.5")) {
		t.Fatal("followed a public address")
	}
	if !sameHost("http://192.168.1.1:5000/desc.xml", netip.MustParseAddr("192.168.1.1")) {
		t.Fatal("refused the router")
	}
}

func TestNoRouter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := (Router{Gateway: closedPort(t), SkipUPnP: true}).Map(ctx, 7333, 0); err == nil || !strings.Contains(err.Error(), "didn't open a port") {
		t.Fatalf("err: %v", err)
	}
}

// The keeper maps, says so, and removes the mapping when stopped.
func TestKeeper(t *testing.T) {
	f := newFakeRouter(t, true)
	k := &Keeper{Router: Router{Gateway: f.addr(), SkipUPnP: true}}
	k.Start(7333)
	var m Mapping
	for i := 0; i < 100 && m.Method == ""; i++ {
		m, _ = k.Status()
		time.Sleep(10 * time.Millisecond)
	}
	if m.Method != MethodPCP {
		t.Fatalf("status: %+v", m)
	}
	k.Stop()
	if m, err := k.Status(); m.Method != "" || err != nil {
		t.Fatalf("after stop: %+v %v", m, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.removed != 1 {
		t.Fatalf("removed: %d", f.removed)
	}
}

func TestGatewayParsing(t *testing.T) {
	if a, err := bsdGateway("   route to: default\ndestination: default\n    gateway: 192.168.50.1\n  interface: en0\n"); err != nil || a != netip.MustParseAddr("192.168.50.1") {
		t.Fatalf("bsd: %v %v", a, err)
	}
	win := "Network Destination        Netmask          Gateway       Interface  Metric\n          0.0.0.0          0.0.0.0      10.0.0.1     10.0.0.20     35\n          0.0.0.0          0.0.0.0   192.168.1.1  192.168.1.20     25\n"
	if a, err := windowsGateway(win); err != nil || a != netip.MustParseAddr("192.168.1.1") {
		t.Fatalf("windows: %v %v", a, err)
	}
	proc := "Iface\tDestination\tGateway \tFlags\nwlan0\t0001A8C0\t00000000\t0001\nwlan0\t00000000\t0101A8C0\t0003\n"
	if a, err := procRouteGateway(bufio.NewScanner(strings.NewReader(proc))); err != nil || a != netip.MustParseAddr("192.168.1.1") {
		t.Fatalf("linux: %v %v", a, err)
	}
	if _, err := bsdGateway("nothing"); err != ErrNoGateway {
		t.Fatal(err)
	}
}

func TestPublicAndCarrierNAT(t *testing.T) {
	for addr, public := range map[string]bool{
		"203.0.113.9": true, "2001:db8::1": true, "192.168.1.1": false, "10.0.0.1": false,
		"100.72.0.1": false, "127.0.0.1": false, "fd00::1": false, "fe80::1": false,
	} {
		if Public(netip.MustParseAddr(addr)) != public {
			t.Errorf("%s: public = %v", addr, !public)
		}
	}
	if !CarrierNAT(netip.MustParseAddr("100.64.3.4")) || CarrierNAT(netip.MustParseAddr("203.0.113.9")) {
		t.Fatal("carrier NAT")
	}
}
