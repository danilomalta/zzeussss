# Makefile - Automação local do Backend do TitanSystem
# Tech Lead SecOps - TitanSystem Team

.PHONY: run test test-integration docker-up

# Inicializa o servidor HTTP principal na porta 8080
run:
	go run cmd/api/main.go

# Executa toda a suíte de testes unitários
test:
	go test -v ./internal/...

# Executa especificamente a suíte de testes de integração HTTP E2E
test-integration:
	go test -v ./tests/integration/...

# Levanta os contêineres de infraestrutura na raiz do projeto (PostgreSQL)
docker-up:
	docker-compose -f ../docker-compose.yml up -d
