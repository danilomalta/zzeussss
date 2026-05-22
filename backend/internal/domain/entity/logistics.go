package entity

import (
	"time"
)

// LogisticsSchedule define as janelas de horário permitidas e acordadas que o cliente B2B
// aceita receber frotas/caminhões para entrega ou retirada de mercadorias.
type LogisticsSchedule struct {
	ID           string    `json:"id"`
	ClientID     string    `json:"client_id"`
	DayOfWeek    int       `json:"day_of_week"` // 1 (Segunda) a 7 (Domingo)
	WindowStart  time.Time `json:"window_start"` // Hora de início da janela (ex: 08:00)
	WindowEnd    time.Time `json:"window_end"`   // Hora de fim da janela (ex: 12:00)
	MaxTrucks    int       `json:"max_trucks"`   // Capacidade operacional máxima de caminhões simultâneos
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// FleetRoute representa o plano de rota consolidado e otimizado para despacho de cargas B2B.
type FleetRoute struct {
	ID             string              `json:"id"`
	SupplierID     string              `json:"supplier_id"`
	TruckPlate     string              `json:"truck_plate"`
	DriverName     string              `json:"driver_name"`
	EstimatedStart time.Time           `json:"estimated_start"`
	EstimatedEnd   time.Time           `json:"estimated_end"`
	TotalDistance  float64             `json:"total_distance"` // Em quilômetros
	Status         string              `json:"status"`         // Ex: "DRAFT", "DISPATCHED", "COMPLETED"
	Schedules      []LogisticsSchedule `json:"schedules"`      // Janelas contempladas nesta rota
	CreatedAt      time.Time           `json:"created_at"`
}

// StockAlert representa o gatilho disparado quando o inventário de um cliente B2B atinge níveis críticos.
type StockAlert struct {
	ID          string    `json:"id"`
	ProductID   string    `json:"product_id"`
	SupplierID  string    `json:"supplier_id"`
	ClientID    string    `json:"client_id"`
	CurrentQty  float64   `json:"current_qty"`
	MinLimitQty float64   `json:"min_limit_qty"`
	Severity    string    `json:"severity"`   // Ex: "MEDIUM" (Próximo do limite), "CRITICAL" (Abaixo do limite)
	Status      string    `json:"status"`     // Ex: "PENDING", "RESOLVED", "IGNORED"
	TriggeredAt time.Time `json:"triggered_at"`
}

// RELACIONAMENTO ENTRE ENTIDADES NO FLUXO B2B (Cérebro B2B):
// 1. O ciclo se inicia com a geração de um StockAlert. Quando o monitor de estoque detecta que o
//    inventário de um cliente B2B atingiu níveis abaixo do limite mínimo, um Alerta é gerado.
// 2. Esse StockAlert é a fundação para o emparelhamento automático de suprimentos (Supplier Matching),
//    gerando uma proposta de ordem de ressuprimento inteligente baseada nas necessidades do cliente.
// 3. Com a ordem aceita, o sistema entra na fase logística: cruza-se o LogisticsSchedule definido pelo
//    cliente (janelas em que ele aceita receber mercadorias) com a disponibilidade da frota do fornecedor.
// 4. Como resultado, é consolidada uma FleetRoute otimizada, garantindo que o caminhão despachado
//    chegará ao destino exatamente dentro da janela de horário estipulada no LogisticsSchedule correspondente,
//    evitando ociosidade, gargalos de recebimento e custos adicionais com frete (demurrage).
