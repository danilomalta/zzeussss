package main

import (
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/apicontract"
)

func onlineServerConfig() fiber.Config {
	return fiber.Config{
		AppName:               "TitanSystem Backend API (PostgreSQL Cores)",
		BodyLimit:             64 * 1024,
		ReadBufferSize:        8192,
		Concurrency:           256,
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          30 * time.Second,
		IdleTimeout:           60 * time.Second,
		DisableStartupMessage: true,
		ErrorHandler:          apicontract.FrameworkError,
		// Forwarded IP headers are not trusted. Proxy deployments require an
		// explicit trust/network policy before IP-based limits can use them.
		ProxyHeader: "",
	}
}

func onlineListenAddress(host, port string) (string, error) {
	if host == "" {
		host = "0.0.0.0"
	}
	if port == "" {
		port = "8080"
	}
	if net.ParseIP(host) == nil || len(port) > 5 {
		return "", errors.New("endereço ou porta online inválidos")
	}
	for _, r := range port {
		if r < '0' || r > '9' {
			return "", errors.New("endereço ou porta online inválidos")
		}
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return "", errors.New("endereço ou porta online inválidos")
	}
	return net.JoinHostPort(host, strconv.Itoa(number)), nil
}
