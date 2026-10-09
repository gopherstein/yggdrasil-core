package portmap

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net/netip"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ErrNoGateway is a network with no default IPv4 route to ask.
var ErrNoGateway = errors.New("no default gateway")

// Gateway is the default IPv4 gateway: the router PCP and NAT-PMP talk to.
func Gateway(ctx context.Context) (netip.Addr, error) {
	switch runtime.GOOS {
	case "linux", "android":
		return linuxGateway()
	case "windows":
		out, err := run(ctx, "route", "print", "-4", "0.0.0.0")
		if err != nil {
			return netip.Addr{}, err
		}
		return windowsGateway(out)
	default: // darwin and the BSDs
		out, err := run(ctx, "route", "-n", "get", "default")
		if err != nil {
			return netip.Addr{}, err
		}
		return bsdGateway(out)
	}
}

func run(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return string(out), err
}

// bsdGateway reads `route -n get default`: "    gateway: 192.168.1.1".
func bsdGateway(out string) (netip.Addr, error) {
	for _, line := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), ":"); ok && k == "gateway" {
			if a, err := netip.ParseAddr(strings.TrimSpace(v)); err == nil && a.Is4() {
				return a, nil
			}
		}
	}
	return netip.Addr{}, ErrNoGateway
}

// windowsGateway reads `route print -4 0.0.0.0`: a row of
// "0.0.0.0  0.0.0.0  <gateway>  <interface>  <metric>", the lowest metric.
func windowsGateway(out string) (netip.Addr, error) {
	best, bestMetric := netip.Addr{}, -1
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || f[0] != "0.0.0.0" || f[1] != "0.0.0.0" {
			continue
		}
		a, err := netip.ParseAddr(f[2])
		if err != nil || !a.Is4() {
			continue
		}
		metric := 0
		for _, c := range f[4] {
			if c < '0' || c > '9' {
				metric = -1
				break
			}
			metric = metric*10 + int(c-'0')
		}
		if metric >= 0 && (bestMetric < 0 || metric < bestMetric) {
			best, bestMetric = a, metric
		}
	}
	if !best.IsValid() {
		return netip.Addr{}, ErrNoGateway
	}
	return best, nil
}

// linuxGateway reads /proc/net/route for the default route's gateway.
func linuxGateway() (netip.Addr, error) {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return netip.Addr{}, err
	}
	defer f.Close()
	return procRouteGateway(bufio.NewScanner(f))
}

// procRouteGateway reads /proc/net/route lines: the destination and the
// gateway are little-endian hex, and the default route has destination 0.
func procRouteGateway(sc *bufio.Scanner) (netip.Addr, error) {
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 3 || f[1] != "00000000" {
			continue
		}
		raw, err := hex.DecodeString(f[2])
		if err != nil || len(raw) != 4 {
			continue
		}
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], binary.BigEndian.Uint32(raw))
		if a := netip.AddrFrom4(b); a.IsValid() && !a.IsUnspecified() {
			return a, nil
		}
	}
	return netip.Addr{}, ErrNoGateway
}
