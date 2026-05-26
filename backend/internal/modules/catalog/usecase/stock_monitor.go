package usecase

import "titansystem-backend/internal/modules/catalog/domain"

// StockMonitorUseCase define a interface para monitoramento contínuo de inventário B2B.
type StockMonitorUseCase interface {
	// CheckStockLevels avalia os níveis de estoque atuais de um cliente contra o limite mínimo.
	CheckStockLevels(clientID string) ([]*domain.StockAlert, error)
	// ProcessStockAlert trata o fluxo de tomada de decisão do alerta (ex: notificar fornecedor ou auto-resolver).
	ProcessStockAlert(alertID string) error
}

// LOGÍSTICA B2B E REGRAS DE NEGÓCIO DO STOCK MONITOR:
// 1. Verificação Automatizada e Integrada: O serviço analisa periodicamente (ou via eventos/hooks de venda)
//    a quantidade de itens disponíveis nos canais de venda física ou digital do cliente B2B.
// 2. Detecção de Cruzamento de Limites (Threshold Crossing): Se a quantidade atual de estoque cruzar para
//    baixo do limite de segurança preestabelecido pelo cliente (MinLimitQty), o sistema aciona o motor de alertas.
// 3. Classificação de Severidade:
//    - "MEDIUM": O estoque atingiu a margem de segurança planejada (ideal para alerta prévio).
//    - "CRITICAL": O estoque está em risco iminente de ruptura (ruptura de estoque/falta de produto na gôndola).
// 4. Disparo Automático para o Produtor: Um evento assíncrono deve ser despachado, criando um registro de alerta
//    e notificando imediatamente a central do fornecedor/produtor associado com a quantidade sugerida de reposição,
//    iniciando instantaneamente o processo de "Supplier Matching" sem intervenção manual demorada.
