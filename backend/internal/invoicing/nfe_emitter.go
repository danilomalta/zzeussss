package invoicing

/*
NFE EMITTER SERVICE
===================
Este componente gerencia a comunicação direta com a Sefaz/órgãos regulamentadores para emissão online 
de Notas Fiscais Eletrônicas (NFe e NFCe).

Regra de Negócio:
1. Integrado diretamente ao checkout do PDV e faturamento do ERP.
2. Transforma as entidades internas de Venda (Orders/Invoices) para o padrão XML exigido pela Sefaz.
3. Processa o envio, assina digitalmente com certificado A1 e lida com retornos e contingência offline (se for NFCe).
4. Trabalha lado-a-lado com o `sped_service.go` para geração de relatórios mensais.
*/

// NFEEmitter assina o comportamento de um emissor fiscal.
type NFEEmitter interface {
	GenerateXML(orderID string) (string, error)
	SignAndTransmit(xmlContent string, certificateData []byte) (NFEResponse, error)
	HandleContingency(orderID string) error
}

// NFEResponse encapsula o retorno governamental.
type NFEResponse struct {
	Success       bool
	ReceiptNumber string
	AccessKey     string
	ErrorMessage  string
}
