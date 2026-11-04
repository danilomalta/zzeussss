package main

import (
	"crypto/tls"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/deployment"
)

var errOnlineExposure = errors.New("exposição online recusada: configure perfil privado, TLS válido e origens explícitas; proxy TLS somente em loopback com IPs confiáveis explícitos")

type onlineExposure struct {
	profile deployment.Profile
	tls     *tls.Config
	proxies map[netip.Addr]bool
}

// Configuration is validated before database startup. No private paths, input
// values or certificate diagnostics escape through the returned error.
func readOnlineExposure(getenv func(string) string, now time.Time) (onlineExposure, error) {
	var out onlineExposure
	address, err := onlineListenAddress(getenv("TITAN_API_HOST"), getenv("PORT"))
	if err != nil {
		return out, errOnlineExposure
	}
	host, port, _ := net.SplitHostPort(address)
	n, _ := strconv.Atoi(port)
	p := deployment.Local(n)
	p.Listen = address
	if path := getenv("TITAN_API_DEPLOYMENT"); path != "" {
		p, err = deployment.Load(path)
		if err != nil {
			return out, errOnlineExposure
		}
		ph, pp, e := net.SplitHostPort(p.Listen)
		canonical, e2 := onlineListenAddress(ph, pp)
		if e != nil || e2 != nil || canonical != p.Listen ||
			(getenv("TITAN_API_HOST") != "" && !net.ParseIP(host).Equal(net.ParseIP(ph))) ||
			(getenv("PORT") != "" && port != pp) {
			return out, errOnlineExposure
		}
	}
	proxy := false
	switch getenv("TITAN_API_REVERSE_PROXY_TLS") {
	case "", "false":
	case "true":
		proxy = true
	default:
		return out, errOnlineExposure
	}
	if p.CheckTransportExposure(proxy) != nil || !p.Describe().Available {
		return out, errOnlineExposure
	}
	// Proxy mode has a single clear TLS terminator, and cannot silently replace
	// direct TLS or accept HTTP connections from a remote reverse proxy.
	if proxy && p.Mode != "local" {
		return out, errOnlineExposure
	}
	peers := getenv("TITAN_API_TRUSTED_PROXY_IPS")
	if proxy {
		if peers == "" || len(peers) > 512 {
			return out, errOnlineExposure
		}
		out.proxies = map[netip.Addr]bool{}
		for _, raw := range strings.Split(peers, ",") {
			ip, e := netip.ParseAddr(raw)
			if e != nil || ip.Zone() != "" || !ip.IsLoopback() || out.proxies[ip.Unmap()] || len(out.proxies) >= 16 {
				return onlineExposure{}, errOnlineExposure
			}
			out.proxies[ip.Unmap()] = true
		}
	} else if peers != "" {
		return out, errOnlineExposure
	}
	out.tls, err = p.TLS(now)
	if err != nil {
		return onlineExposure{}, errOnlineExposure
	}
	if out.tls != nil || proxy {
		if !onlineSecureOrigins(getenv("ALLOWED_ORIGINS")) || strings.EqualFold(strings.TrimSpace(getenv("COOKIE_SECURE")), "false") {
			return onlineExposure{}, errOnlineExposure
		}
	}
	out.profile = p
	return out, nil
}

func onlineSecureOrigins(raw string) bool {
	if raw == "" || len(raw) > 4096 {
		return false
	}
	seen := map[string]bool{}
	for _, origin := range strings.Split(raw, ",") {
		u, err := url.Parse(origin)
		if err != nil || seen[origin] || origin != strings.TrimSpace(origin) ||
			u.User != nil || u.Hostname() == "" || u.Path != "" || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || origin != u.Scheme+"://"+u.Host || strings.HasSuffix(u.Host, ":") ||
			(u.Scheme != "https" && origin != "titan://app") || strings.ContainsAny(origin, "*\r\n\t") {
			return false
		}
		if p := u.Port(); p != "" {
			n, e := strconv.Atoi(p)
			if e != nil || n < 1 || n > 65535 || strconv.Itoa(n) != p {
				return false
			}
		}
		seen[origin] = true
	}
	return true
}

func (p onlineExposure) allowsPeer(remote string) bool {
	if !p.profile.Allows(remote) {
		return false
	}
	if p.proxies != nil {
		ip, err := netip.ParseAddr(remote)
		return err == nil && p.proxies[ip.Unmap()]
	}
	return true
}

func (p onlineExposure) gate(c *fiber.Ctx) error {
	// Always the socket peer: no Forwarded/X-Forwarded-For/X-Real-IP trust.
	if !p.allowsPeer(c.Context().RemoteIP().String()) {
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.SendStatus(fiber.StatusForbidden)
	}
	return c.Next()
}

func (p onlineExposure) listen() (net.Listener, error) {
	if p.profile.CheckTransportExposure(p.proxies != nil) != nil || (p.profile.Mode != "local" && p.tls == nil) {
		return nil, errOnlineExposure
	}
	ln, err := net.Listen("tcp", p.profile.Listen)
	if err != nil {
		return nil, errOnlineExposure
	}
	if p.tls != nil {
		return tls.NewListener(ln, p.tls.Clone()), nil
	}
	return ln, nil
}
