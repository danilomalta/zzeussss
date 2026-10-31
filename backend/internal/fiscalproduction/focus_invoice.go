package fiscalproduction

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
)

// Field names come from the official schema linked by emitir_nfce:
// https://campos.focusnfe.com.br/nfe/NotaFiscalXML.html
// https://campos.focusnfe.com.br/nfe/ItemNotaFiscalXML.html
// No tax rates or tax codes are inferred. Unknown groups fail closed.
func focusTaxFields(dst map[string]any, nodes []fiscalcore.TaxNode) error {
	mappings := map[string]map[string]string{
		"ICMS":   {"orig": "icms_origem", "CST": "icms_situacao_tributaria", "CSOSN": "icms_situacao_tributaria", "modBC": "icms_modalidade_base_calculo", "vBC": "icms_base_calculo", "pICMS": "icms_aliquota", "vICMS": "icms_valor", "pRedBC": "icms_reducao_base_calculo", "vBCSTRet": "icms_base_calculo_retido_st", "vICMSSTRet": "icms_valor_retido_st", "pST": "icms_aliquota_final", "vICMSSubstituto": "icms_valor_substituto", "pICMSST": "icms_aliquota_st", "vICMSST": "icms_valor_st", "pCredSN": "icms_aliquota_credito_simples", "vCredICMSSN": "icms_valor_credito_simples", "pRedBCEfet": "icms_reducao_base_calculo_efetiva", "vBCEfet": "icms_base_calculo_efetiva", "pICMSEfet": "icms_aliquota_efetiva", "vICMSEfet": "icms_valor_efetivo", "vICMSOp": "icms_valor_operacao", "pDif": "icms_percentual_diferimento", "vICMSDif": "icms_valor_diferido", "vBCFCP": "fcp_base_calculo", "pFCP": "fcp_percentual", "vFCP": "fcp_valor", "vBCFCPST": "fcp_base_calculo_st", "pFCPST": "fcp_percentual_st", "vFCPST": "fcp_valor_st", "vBCFCPSTRet": "fcp_base_calculo_retido_st", "pFCPSTRet": "fcp_percentual_retido_st", "vFCPSTRet": "fcp_valor_retido_st"},
		"PIS":    {"CST": "pis_situacao_tributaria", "vBC": "pis_base_calculo", "pPIS": "pis_aliquota_porcentual", "qBCProd": "pis_quantidade_vendida", "vAliqProd": "pis_aliquota_valor", "vPIS": "pis_valor"},
		"COFINS": {"CST": "cofins_situacao_tributaria", "vBC": "cofins_base_calculo", "pCOFINS": "cofins_aliquota_porcentual", "qBCProd": "cofins_quantidade_vendida", "vAliqProd": "cofins_aliquota_valor", "vCOFINS": "cofins_valor"},
	}
	for _, group := range nodes {
		fields, ok := mappings[group.Name]
		if !ok {
			return fiscalcore.ErrUnsupported
		}
		var walk func([]fiscalcore.TaxNode, int) error
		walk = func(nodes []fiscalcore.TaxNode, depth int) error {
			if depth > 8 {
				return fiscalcore.ErrUnsupported
			}
			for _, n := range nodes {
				if len(n.Children) > 0 {
					if n.Value != "" {
						return fiscalcore.ErrUnsupported
					}
					if err := walk(n.Children, depth+1); err != nil {
						return err
					}
					continue
				}
				name, ok := fields[n.Name]
				if !ok || n.Value == "" {
					return fiscalcore.ErrUnsupported
				}
				if _, exists := dst[name]; exists {
					return fiscalcore.ErrUnsupported
				}
				dst[name] = n.Value
			}
			return nil
		}
		if err := walk(group.Children, 0); err != nil {
			return err
		}
	}
	for _, key := range []string{"icms_situacao_tributaria", "pis_situacao_tributaria", "cofins_situacao_tributaria"} {
		if _, ok := dst[key]; !ok {
			return fiscalcore.ErrUnsupported
		}
	}
	return nil
}
func focusMoney(v int64) string { return fmt.Sprintf("%d.%02d", v/100, v%100) }
func focusInvoice(ctx context.Context, a RequestContext, intent fiscal.Intent, raw json.RawMessage) ([]byte, error) {
	return focusInvoiceRequest(ctx, a, intent, DocumentRequest{Invoice: raw})
}

func focusInvoiceRequest(ctx context.Context, a RequestContext, intent fiscal.Intent, in DocumentRequest) ([]byte, error) {
	raw := in.Invoice
	var template fiscalcore.Invoice
	if fiscalcore.DecodeInput(raw, &template) != nil || template.Environment != 2 || template.Key != (fiscalcore.KeyInput{}) || !template.ContingencyAt.IsZero() || template.ContingencyReason != "" {
		return nil, fiscalcore.ErrUnsupported
	}
	if template.Presence != "1" && template.Presence != "4" {
		return nil, fiscalcore.ErrUnsupported
	}
	snapshot := intent.Snapshot
	p := snapshot.Profile
	if (p.Model != "65" && p.Model != "55") || p.Environment != "homologation" || len(snapshot.Items) == 0 || len(snapshot.Items) > 500 {
		return nil, fiscalcore.ErrUnsupported
	}
	if p.Model == "55" {
		if err := validateFocusNFeInput(template, in, p); err != nil {
			return nil, err
		}
	}
	items := make([]map[string]any, 0, len(snapshot.Items))
	for i, item := range snapshot.Items {
		if item.Tax == nil || item.QuantityMilli <= 0 || item.UnitPriceCents < 0 {
			return nil, fiscalcore.ErrUnsupported
		}
		quote, err := fiscal.ReadTaxQuote(ctx, a.DB, a.Actor, a.Device, item.Tax.QuoteOperationID)
		if err != nil {
			return nil, err
		}
		if quote.Input.Context.OriginUF != p.State || quote.Input.Context.DestinationUF != p.State {
			return nil, fiscalcore.ErrUnsupported
		}
		if quote.Input.Case.FinalConsumer != "yes" || quote.Input.Case.RecipientStatus != "noncontributor" {
			return nil, fiscalcore.ErrUnsupported
		}
		if quote.ResultSHA256 != item.Tax.ResultSHA256 || quote.Input.Item.ProductID != item.ProductID || item.Tax.ProfileID != p.ProfileID || item.Tax.ProfileRevision != intent.Snapshot.ProfileRevision {
			return nil, fiscal.ErrConflict
		}
		var name string
		if err = a.DB.QueryRowContext(ctx, `SELECT name FROM products WHERE tenant_id=? AND id=?`, a.Actor.TenantID, item.ProductID).Scan(&name); err != nil {
			return nil, err
		}
		row := map[string]any{"numero_item": i + 1, "codigo_produto": item.ProductID, "descricao": name, "codigo_ncm": quote.Result.NCM, "cfop": quote.Result.CFOP, "unidade_comercial": item.Tax.CatalogUnit, "quantidade_comercial": fmt.Sprintf("%d.%03d", item.QuantityMilli/1000, item.QuantityMilli%1000), "valor_unitario_comercial": focusMoney(item.UnitPriceCents), "valor_bruto": quote.Result.Gross, "valor_desconto": quote.Input.Item.Discount, "valor_frete": quote.Input.Item.Freight, "valor_seguro": quote.Input.Item.Insurance, "valor_outras_despesas": quote.Input.Item.Other}
		if err = focusTaxFields(row, item.Tax.Taxes); err != nil {
			return nil, err
		}
		if quote.Result.GoodsXML != nil && (quote.Result.GoodsXML.Classification.BenefitCode != "" || quote.Result.GoodsXML.Classification.FCI != "") {
			return nil, fiscalcore.ErrUnsupported
		}
		items = append(items, row)
	}
	payments := make([]map[string]string, 0, len(snapshot.Payments))
	for _, payment := range snapshot.Payments {
		if payment.Method != "cash" || payment.Status != "confirmed" {
			return nil, fiscalcore.ErrUnsupported
		}
		payments = append(payments, map[string]string{"forma_pagamento": "01", "valor_pagamento": focusMoney(payment.AmountCents)})
	}
	payload := map[string]any{"cnpj_emitente": p.CNPJ, "serie": fmt.Sprint(p.Series), "inscricao_estadual_emitente": p.StateRegistration, "regime_tributario_emitente": p.CRT, "data_emissao": time.Now().UTC().Format(time.RFC3339), "natureza_operacao": template.Nature, "tipo_documento": "1", "finalidade_emissao": "1", "local_destino": "1", "consumidor_final": "1", "presenca_comprador": template.Presence, "indicador_inscricao_estadual_destinatario": "9", "modalidade_frete": "9", "valor_total": focusMoney(snapshot.TotalCents), "items": items, "formas_pagamento": payments}
	if p.Model == "55" {
		payload["finalidade_emissao"] = in.Purpose
		payload["modalidade_frete"] = in.FreightMode
		payload["tipo_documento"] = template.Direction
		payload["local_destino"] = template.Destination
		payload["consumidor_final"] = template.FinalConsumer
		// Issuer registration is authoritative; addresses are configured in Focus.
		payload["nome_emitente"] = p.LegalName
		payload["uf_emitente"] = p.State
		for _, row := range items {
			if row["valor_frete"] != "0.00" {
				return nil, fiscalcore.ErrUnsupported
			}
		}
	}
	return focusRecipientPayload(template, p, payload)
}

func focusRecipientPayload(template fiscalcore.Invoice, profile fiscal.ProfileInput, payload map[string]any) ([]byte, error) {
	if r := template.Recipient; r != nil {
		if (r.CNPJ != "" && r.CPF != "") || (r.IEDestination != "" && r.IEDestination != "9") || (r.Address.UF != "" && r.Address.UF != profile.State) {
			return nil, fiscalcore.ErrUnsupported
		}
		values := map[string]string{
			"cnpj_destinatario": r.CNPJ, "cpf_destinatario": r.CPF, "nome_destinatario": r.Name,
			"inscricao_estadual_destinatario": r.IE, "logradouro_destinatario": r.Address.Street,
			"numero_destinatario": r.Address.Number, "bairro_destinatario": r.Address.District,
			"codigo_municipio_destinatario": r.Address.CityCode, "municipio_destinatario": r.Address.City,
			"uf_destinatario": r.Address.UF, "cep_destinatario": r.Address.PostalCode,
		}
		for field, value := range values {
			if value != "" {
				payload[field] = value
			}
		}
	}
	return json.Marshal(payload)
}
