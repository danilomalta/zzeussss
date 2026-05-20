package websocket

import (
	"log"
	"strings"
	"time"

	ws "github.com/gofiber/websocket/v2"
)

const (
	prazoEscrita  = 10 * time.Second
	prazoLeitura  = 60 * time.Second
	periodoPing   = 30 * time.Second
	tamanhoMaxMsg = 64 * 1024
)

// HandlerChat é um handler de WebSocket preparado para chat em tempo real.
//
// Regras de robustez:
// - define limite de tamanho de mensagem
// - controla prazos de leitura e escrita
// - envia ping periódico e renova prazo via pong
func HandlerChat(c *ws.Conn) {
	c.SetReadLimit(tamanhoMaxMsg)
	_ = c.SetReadDeadline(time.Now().Add(prazoLeitura))
	c.SetPongHandler(func(string) error {
		return c.SetReadDeadline(time.Now().Add(prazoLeitura))
	})

	ticker := time.NewTicker(periodoPing)
	defer ticker.Stop()

	fechar := make(chan struct{})
	defer close(fechar)

	go func() {
		for {
			select {
			case <-ticker.C:
				_ = c.SetWriteDeadline(time.Now().Add(prazoEscrita))
				if err := c.WriteMessage(ws.PingMessage, []byte("ping")); err != nil {
					return
				}
			case <-fechar:
				return
			}
		}
	}()

	for {
		tipo, msg, err := c.ReadMessage()
		if err != nil {
			return
		}

		texto := strings.TrimSpace(string(msg))
		if texto == "" {
			continue
		}

		// “ping” textual é aceito por compatibilidade simples; respondemos “pong”.
		if strings.EqualFold(texto, "ping") {
			_ = c.SetWriteDeadline(time.Now().Add(prazoEscrita))
			_ = c.WriteMessage(ws.TextMessage, []byte("pong"))
			continue
		}

		// Implementação inicial: eco controlado (serve como prova do upgrade e do canal).
		// O próximo passo é publicar no “hub” e persistir `MensagemChat` no banco com auditoria.
		_ = c.SetWriteDeadline(time.Now().Add(prazoEscrita))
		if err := c.WriteMessage(tipo, []byte("eco: "+texto)); err != nil {
			log.Printf("falha ao escrever no websocket: %v", err)
			return
		}
	}
}

