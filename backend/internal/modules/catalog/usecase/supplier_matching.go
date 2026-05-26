package usecase

import "titansystem-backend/internal/modules/catalog/domain"

// SupplierMatchingUseCase define a interface para emparelhamento inteligente entre alertas de estoque e ofertas de fornecedores.
type SupplierMatchingUseCase interface {
	// ProposeOrder analisa o alerta de estoque e gera uma sugestão de proposta comercial de fornecimento.
	ProposeOrder(alert *domain.StockAlert) (string, error) // Retorna o ID da proposta criada
	// AcceptProposedOrder lida com o aceite do cliente B2B da proposta sugerida.
	AcceptProposedOrder(proposalID string) (bool, error)
}

// LOGÍSTICA B2B E REGRAS DE NEGÓCIO DO SUPPLIER MATCHING (Emparelhamento de Suprimentos):
// 1. Ponte Invisível e Proativa: Em vez do cliente B2B perceber a falta de produto e manualmente entrar
//    em contato com o fornecedor para orçar e comprar, o sistema atua de forma proativa baseado nos alertas de estoque baixo.
// 2. Recomendação de Compra Inteligente: Cruzando a quantidade em falta, a velocidade de consumo histórico
//    do produto e os dados de lote mínimo de venda do fornecedor, o sistema gera uma recomendação otimizada de ordem de ressuprimento.
// 3. Facilidade de Decisão (One-Click Acceptance): O cliente B2B visualiza em seu painel uma proposta pré-aprovada
//    e já agendada logisticamente. Ao clicar em "Aceitar", toda a burocracia comercial é resolvida instantaneamente,
//    gerando o pedido no sistema e encaminhando para faturamento e despacho de entrega.
