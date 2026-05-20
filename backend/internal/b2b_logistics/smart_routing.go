package b2blogistics

import "time"

/*
SMART ROUTING SERVICE
=====================
Arquivo que carrega o motor de roteirização inteligente do ecosistema de logística B2B TitanSystem.

Regra de Negócio:
1. Após um pedido de ressuprimento ser aceito, a entrega física precisa acontecer.
2. Para evitar gargalos e caminhões parados nas docas, o lojista define uma "Janela de Recebimento" (no app).
3. Este serviço calcula a melhor rota para o caminhão (se comunicando possivelmente com APIs de mapas/trânsito) e aloca o caminhão na janela exata definida, agendando a doca e o horário de desembarque.
*/

// DeliveryWindow representa a janela de tempo autorizada pelo cliente para receber a mercadoria.
type DeliveryWindow struct {
	StartTime time.Time
	EndTime   time.Time
}

// RouteEngine assina os métodos necessários para a roteirização do caminhão.
type RouteEngine interface {
	CalculateOptimalRoute(origin, destination string) (RouteResult, error)
	ScheduleDocking(deliveryID string, window DeliveryWindow) error
}

// RouteResult guarda os metadados do trajeto estimado.
type RouteResult struct {
	EstimatedTimeMinutes int
	DistanceKM           float64
	SuggestedWindow      DeliveryWindow
}
