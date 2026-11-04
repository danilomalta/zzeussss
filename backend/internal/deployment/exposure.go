package deployment

import (
	"bytes"
	"crypto/ed25519"
	"net"
)

// CheckExposure uses the installation's already authenticated device secret;
// no vendor password, generated fallback or additional shared secret exists.
// A declared TLS reverse proxy must connect over loopback. Forwarded headers
// never change network admission or authenticate an operator.
func (p Profile) CheckExposure(stationKey []byte, reverseProxyTLS bool) error {
	if err := p.CheckTransportExposure(reverseProxyTLS); err != nil {
		return err
	}
	host, _, _ := net.SplitHostPort(p.Listen)
	if !net.ParseIP(host).IsLoopback() || reverseProxyTLS {
		if len(stationKey) != ed25519.PrivateKeySize || bytes.Equal(stationKey[:ed25519.SeedSize], make([]byte, ed25519.SeedSize)) {
			return ErrProfile
		}
		derived := ed25519.NewKeyFromSeed(stationKey[:ed25519.SeedSize])
		defer clear(derived)
		if !bytes.Equal(derived, stationKey) {
			return ErrProfile
		}
	}
	return nil
}

// CheckTransportExposure shares only network policy. Callers must independently
// require their own credentials; the local station's CheckExposure still does so.
func (p Profile) CheckTransportExposure(reverseProxyTLS bool) error {
	if err := p.Validate(); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(p.Listen)
	ip := net.ParseIP(host)
	if err != nil || ip == nil {
		return ErrProfile
	}
	if reverseProxyTLS && !ip.IsLoopback() {
		return ErrProfile
	}
	if !ip.IsLoopback() && (p.TLSCert == "" || p.TLSKey == "") {
		return ErrProfile
	}
	return nil
}
