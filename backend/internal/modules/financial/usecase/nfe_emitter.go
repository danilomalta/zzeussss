package usecase

import "time"

// NFeInput define os parâmetros necessários para solicitar a emissão de uma Nota Fiscal Eletrônica.
type NFeInput struct {
	OrderID    string    `json:"order_id"`
	ClientID   string    `json:"client_id"`
	TotalValue float64   `json:"total_value"`
	IssuedAt   time.Time `json:"issued_at"`
}

// NFeOutput representa os dados retornados após o processamento da emissão da Nota Fiscal.
type NFeOutput struct {
	NFeKey     string    `json:"nfe_key"`     // Chave de acesso única da NFe (44 dígitos)
	XMLContent []byte    `json:"xml_content"` // Conteúdo XML assinado digitalmente
	Status     string    `json:"status"`      // Ex: "AUTHORIZED", "REJECTED", "PENDING"
	Protocol   string    `json:"protocol"`    // Protocolo de autorização da SEFAZ
	IssuedAt   time.Time `json:"issued_at"`
}

// NFeEmitterUseCase define a assinatura da interface do caso de uso de emissão de NFe.
type NFeEmitterUseCase interface {
	// QueueNFeRequest coloca o pedido de emissão na fila de processamento assíncrono.
	QueueNFeRequest(input NFeInput) (string, error) // Retorna o JobID
	// GetNFeStatus consulta o status atual de uma solicitação de NFe.
	GetNFeStatus(jobID string) (*NFeOutput, error)
}

// REGRAS DE NEGÓCIO E DE SEGURANÇA (SecOps) - FATURAMENTO (NFe):
// 1. Processamento Assíncrono Orientado a Eventos (Background Queue): Devido à latência inerente das comunicações
//    com os servidores externos da SEFAZ, o processo de emissão da NFe deve rodar em background. Após a confirmação
//    de pagamento de um pedido, o sistema enfileira o Job de emissão, liberando a requisição HTTP do usuário imediatamente.
// 2. Integração Resiliente com APIs Governamentais (SEFAZ): O serviço em background consome o Job, monta o layout XML,
//    assina com certificado digital A1/A3 ICP-Brasil, e transmite para os web services da SEFAZ estadual aplicável.
// 3. Estratégias de Retry com Exponential Backoff (Resiliência): Em cenários de oscilação da SEFAZ (timeout, erros 5xx),
//    o sistema de filas de background deve aplicar políticas automáticas de retentativa com retardo exponencial e tratamento
//    de contingências (como emissão em modo contingência SCAN/SVC-AN).
// 4. Integridade e Proteção de Dados de Faturamento: Toda chave de acesso e XML emitidos devem ser salvos criptografados
//    no banco de dados ou no storage de objetos seguro (S3), bloqueando qualquer acesso público sem token assinado de download.
