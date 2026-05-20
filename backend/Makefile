# Makefile para orquestração do backend Go TitanSystem

.PHONY: run build test migrate-up migrate-down

run:
	go run cmd/api/main.go

build:
	go build -o bin/api cmd/api/main.go

test:
	go test -v ./...

migrate-up:
	@echo "Executando migrations UP..."
	# Implementar lógica com golang-migrate ou similar

migrate-down:
	@echo "Executando migrations DOWN..."
	# Implementar lógica com golang-migrate ou similar
