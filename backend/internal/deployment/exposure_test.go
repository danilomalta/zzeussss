package deployment

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func TestExposureRequiresCustomerSecretAndNeverTrustsRemoteProxy(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	server := Profile{Version: 1, Mode: "server", Listen: "192.168.1.5:8443", TLSCert: "/customer/cert", TLSKey: "/customer/key", AllowedNetworks: []string{"192.168.1.0/24"}}
	cloud := server
	cloud.Version = 2
	cloud.Mode = "cloud"
	for _, p := range []Profile{server, cloud} {
		if p.CheckExposure(nil, false) == nil || p.CheckExposure(make([]byte, 64), false) == nil {
			t.Fatal("missing customer secret accepted")
		}
		if err := p.CheckExposure(key, false); err != nil {
			t.Fatal(err)
		}
		if p.CheckExposure(key, true) == nil {
			t.Fatal("plaintext remote proxy accepted")
		}
		p.TLSKey = ""
		if p.CheckExposure(key, false) == nil {
			t.Fatal("remote without TLS")
		}
	}
	if Local(8181).CheckExposure(nil, false) != nil {
		t.Fatal("loopback default")
	}
	if Local(8181).CheckExposure(nil, true) == nil {
		t.Fatal("proxy without customer secret")
	}
	if Local(8181).CheckExposure(key, true) != nil {
		t.Fatal("explicit loopback proxy")
	}
	key[63] ^= 1
	if server.CheckExposure(key, false) == nil {
		t.Fatal("corrupt device secret")
	}
}
