package app

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/yeixio/toskar-core/internal/config"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

// Access from anywhere (#456, docs/remote-access.md): the listener for paired
// devices outside the home network, started and stopped with its setting.

// applyRemoteAccess starts the remote listener when access from anywhere is
// on, on every address, or stops it and drops its connections when it's off.
func (a *App) applyRemoteAccess() error {
	if a.API == nil || a.Config == nil {
		return nil
	}
	cfg := a.Config.Get()
	if !cfg.RemoteAccess.Enabled {
		a.API.StopRemote()
		return nil
	}
	return a.API.ServeRemote(fmt.Sprintf(":%d", cfg.RemotePort()))
}

// Errors from the remote access settings.
var (
	errRemotePort    = contracts.NewError("REMOTE_PORT_INVALID", nil, errors.New("the port for access from anywhere is 1024 to 65535, and not the API's or the computers' port"))
	errRemoteAddress = contracts.NewError("REMOTE_ADDRESS_INVALID", nil, errors.New("the address is a host name or IP, with a port when it differs, such as home.example.com or 203.0.113.7:7333"))
)

// validRemotePort reports a port the remote listener can take.
func validRemotePort(cfg config.Config, port int) bool {
	return port >= 1024 && port <= 65535 && port != cfg.APIPort && port != cfg.InternalPort
}

// cleanRemoteAddress checks a forwarded address, such as
// "home.example.com" or "203.0.113.7:7333"; "" clears it.
func cleanRemoteAddress(in string) (string, error) {
	addr := strings.TrimSpace(in)
	if addr == "" {
		return "", nil
	}
	if strings.Contains(addr, "://") || strings.ContainsAny(addr, " /?#@") || len(addr) > 260 {
		return "", errRemoteAddress
	}
	host := addr
	if h, port, err := net.SplitHostPort(addr); err == nil {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", errRemoteAddress
		}
		host = h
	} else if strings.Count(addr, ":") == 1 {
		return "", errRemoteAddress
	}
	host = strings.Trim(host, "[]")
	if host == "" || (net.ParseIP(host) == nil && !validHostName(host)) {
		return "", errRemoteAddress
	}
	return addr, nil
}

func validHostName(h string) bool {
	if len(h) > 253 {
		return false
	}
	for _, label := range strings.Split(strings.TrimSuffix(h, "."), ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}
