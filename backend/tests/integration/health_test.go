package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/core/routes"
)

/*
TESTE DE INTEGRAÇÃO DE SAÚDE DA API (E2E)
=========================================
Este teste valida o ponto de extremidade público de verificação de integridade da API (/api/v1/saude).
Ele inicializa o servidor de forma real sobre uma porta dedicada de testes (:9876)
e realiza uma chamada HTTP cliente real utilizando a biblioteca nativa, garantindo que o ciclo de vida
da requisição/resposta esteja em total conformidade técnica com o esperado em ambiente de produção.
*/

func TestHealthCheckIntegration(t *testing.T) {
	// 1. Instancia o servidor real Fiber
	app := fiber.New()

	// 2. Mapeia a rota de verificação da saúde da API
	app.Get("/api/v1/saude", routes.Saude)

	// 3. Inicializa a escuta HTTP em segundo plano em uma porta dedicada (:9876)
	go func() {
		// O Fiber irá escutar na porta :9876 para os testes de integridade E2E
		if err := app.Listen(":9876"); err != nil {
			// Ignora falhas de fechamento ao final do ciclo de vida
		}
	}()

	// 4. Aguarda pequeno delay de propagação do socket de rede
	time.Sleep(100 * time.Millisecond)

	// 5. Constrói a URL real local do servidor temporário
	url := "http://localhost:9876/api/v1/saude"

	// 6. Dispara a requisição HTTP cliente real de rede
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Falha de comunicação: impossível enviar requisição GET à rota %s: %v", url, err)
	}
	defer resp.Body.Close()

	// 7. Asserções de conformidade de integridade
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Falha de assertiva: esperava status HTTP 200 OK, porém recebeu: %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Falha de decodificação: corpo da resposta JSON corrompido: %v", err)
	}

	if result["status"] != "ativo" {
		t.Errorf("Falha de integridade: esperava status 'ativo', porém recebeu: %q", result["status"])
	}

	// 8. Shutdown limpo para devolução de recursos de rede ao Kernel
	if err := app.Shutdown(); err != nil {
		t.Logf("Aviso: erro ao desmontar listener temporário do Fiber: %v", err)
	}
}
