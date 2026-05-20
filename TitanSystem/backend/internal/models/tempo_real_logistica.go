package models

import (
	"time"

	"gorm.io/gorm"
)

// MensagemChat registra uma mensagem do chat em tempo real.
type MensagemChat struct {
	gorm.Model

	// Identificador externo para idempotência (reenvios em reconexões).
	IDExterno string `json:"id_externo" gorm:"uniqueIndex"`

	// Escopo da conversa (sala/canal).
	Sala string `json:"sala" gorm:"index"`

	// Quem enviou.
	RemetenteID     *uint  `json:"remetente_id" gorm:"index"`
	RemetentePerfil string `json:"remetente_perfil" gorm:"index"` // Dono, Vendedor, Cliente, Contador

	Conteudo string `json:"conteudo" gorm:"type:text"`

	EnviadaEm time.Time  `json:"enviada_em" gorm:"index"`
	LidaEm    *time.Time `json:"lida_em"`
}

// StatusFunilLogistico registra a etapa atual de uma entidade no funil logístico.
// Ele é a base persistida do kanban, e pode ser propagado em tempo real via WebSocket.
type StatusFunilLogistico struct {
	gorm.Model

	// Ex.: "venda", "pedido", "entrega"
	TipoEntidade string `json:"tipo_entidade" gorm:"index"`
	IDEntidade   string `json:"id_entidade" gorm:"index"`

	EtapaAtual string `json:"etapa_atual" gorm:"index"` // Ex.: "separacao", "em_rota", "entregue"

	Observacao string `json:"observacao" gorm:"type:text"`

	AtualizadoEm time.Time `json:"atualizado_em" gorm:"index"`
	Origem       string    `json:"origem" gorm:"index"` // Ex.: "computador", "mobile", "web"
}

