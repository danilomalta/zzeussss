package utils

import (
	"net"
)

// ObterIPLocal busca o primeiro endereço IPv4 local não-loopback (ex: 192.168.x.x ou 10.x.x.x).
func ObterIPLocal() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, address := range addrs {
		// Verifica se o IP não é loopback
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}
