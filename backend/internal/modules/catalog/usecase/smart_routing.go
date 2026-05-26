package usecase

import "titansystem-backend/internal/modules/catalog/domain"

// SmartRoutingUseCase define a interface para cálculo inteligente de roteamento e otimização de frotas.
type SmartRoutingUseCase interface {
	// OptimizeRoute recebe os parâmetros da frota e gera um plano de rotas ideal.
	OptimizeRoute(supplierID string, truckPlates []string) ([]*domain.FleetRoute, error)
}

// LOGÍSTICA B2B E REGRAS DE NEGÓCIO DO SMART ROUTING (Roteamento Inteligente):
// 1. Cruzamento de Janelas Operacionais (Logistics Schedules): O motor inteligente de rotas analisa as agendas
//    específicas de cada cliente B2B da fila de entrega (dias da semana e horas permitidas para recebimento).
// 2. Otimização de Recursos da Frota: Cruza a cubagem, peso de carga e limite de veículos do fornecedor com
//    as prioridades geográficas e de janelas para maximizar a taxa de ocupação dos caminhões.
// 3. Algoritmo de Roteirização (futuro): O motor implementará heurísticas e algoritmos matemáticos avançados
//    (como o Problema de Roteamento de Veículos com Janelas de Tempo - VRPTW) para calcular:
//    - A sequência exata de paradas de entrega.
//    - Os horários de chegada projetados para que o motorista estacione estritamente dentro da janela LogisticsSchedule.
// 4. Prevenção de Ineficiências: Evita atrasos severos, ociosidade na fila de descarregamento das docas dos clientes B2B,
//    e multas por janelas estouradas, promovendo redução drástica de emissões e custos operacionais de transporte.
