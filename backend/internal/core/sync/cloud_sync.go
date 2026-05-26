package sync

import (
	"context"
	"log"
	"net/http"
	"time"
)

// SyncWorker gerencia o ciclo de vida do worker de sincronização Local-First em background.
type SyncWorker struct {
	stopChan chan struct{}
}

// NovoSyncWorker cria uma nova instância de SyncWorker.
func NovoSyncWorker() *SyncWorker {
	return &SyncWorker{
		stopChan: make(chan struct{}),
	}
}

// Start inicia o worker de sincronização em uma Goroutine contínua usando o ticker informado.
func (w *SyncWorker) Start(interval time.Duration) {
	ticker := time.NewTicker(interval)
	log.Printf("[SYNC] SyncWorker inicializado com sucesso. Intervalo: %v", interval)

	go func() {
		for {
			select {
			case <-ticker.C:
				w.checkAndSync()
			case <-w.stopChan:
				ticker.Stop()
				log.Println("[SYNC] SyncWorker finalizado com sucesso.")
				return
			}
		}
	}()
}

// Stop finaliza a execução do worker de sincronização.
func (w *SyncWorker) Stop() {
	close(w.stopChan)
}

// checkAndSync verifica se há internet e realiza o sync mockado.
func (w *SyncWorker) checkAndSync() {
	// Cria um cliente HTTP leve com timeout estrito de 3 segundos para evitar travamento
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	// Criando um contexto com timeout para a requisição de ping
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Utiliza uma requisição HEAD leve para o serviço de validação do Google (gerador de status 204)
	req, err := http.NewRequestWithContext(ctx, "HEAD", "https://clients3.google.com/generate_204", nil)
	if err != nil {
		log.Printf("[SYNC] Erro ao criar request de ping: %v", err)
		log.Println("[SYNC] Offline. Operando em modo de rede local exclusiva.")
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		// Sem conexão externa
		log.Println("[SYNC] Offline. Operando em modo de rede local exclusiva.")
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Com internet activa
		log.Println("[SYNC] Internet detectada. Sincronizando dados com a nuvem...")
		// Lógica de sincronização real ficará mockada aqui por enquanto
	} else {
		log.Println("[SYNC] Offline. Operando em modo de rede local exclusiva.")
	}
}
