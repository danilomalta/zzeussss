package delivery

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/core/security"
	recompensas "titansystem-backend/internal/modules/tenant/domain"
)

type RequisicaoCriarIndicacao struct {
	IDExterno string `json:"id_externo"`
	EmailIndicado string `json:"email_indicado"`
	TelefoneIndicado string `json:"telefone_indicado"`
	IndicadorPerfil string `json:"indicador_perfil"` // Dono, Vendedor, Cliente, Contador
}

// CriarIndicacao registra uma indicação. A regra financeira (R$ 50 / R$ 10) é aplicada
// por eventos posteriores (ex.: conta aprovada). Aqui registramos o vínculo e garantimos idempotência.
func CriarIndicacao(c *fiber.Ctx) error {
	var req RequisicaoCriarIndicacao
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "json inválido"})
	}

	req.IDExterno = strings.TrimSpace(req.IDExterno)
	req.EmailIndicado = strings.TrimSpace(strings.ToLower(req.EmailIndicado))
	req.TelefoneIndicado = strings.TrimSpace(req.TelefoneIndicado)
	req.IndicadorPerfil = strings.TrimSpace(req.IndicadorPerfil)

	if req.IDExterno == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id_externo é obrigatório"})
	}
	if req.EmailIndicado == "" && req.TelefoneIndicado == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "email_indicado ou telefone_indicado é obrigatório"})
	}
	if req.IndicadorPerfil == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "indicador_perfil é obrigatório"})
	}

	indicacao := recompensas.Indicacao{
		IDExterno: req.IDExterno,
		IndicadorPerfil: req.IndicadorPerfil,
		EmailIndicado: req.EmailIndicado,
		TelefoneIndicado: req.TelefoneIndicado,
		Status: "registrada",
	}

	if err := database.DB.Create(&indicacao).Error; err != nil {
		// Pode ser duplicidade por reenvio (idempotência).
		var existente recompensas.Indicacao
		if e2 := database.DB.Where("id_externo = ?", req.IDExterno).First(&existente).Error; e2 == nil {
			return c.Status(fiber.StatusOK).JSON(existente)
		}
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": "não foi possível registrar a indicação"})
	}

	_ = database.DB.Create(&security.AuditLog{
		Perfil:      req.IndicadorPerfil,
		Acao:        "indicacao_registrada",
		Recurso:     "recompensas",
		EnderecoIP:  c.IP(),
		Agente:      c.Get("User-Agent"),
		Sucesso:     true,
		Detalhes:    "indicação registrada; recompensa será concedida por evento posterior",
		IDCorrelacao: strings.TrimSpace(c.Get("X-Correlacao-Id")),
	}).Error

	return c.Status(fiber.StatusCreated).JSON(indicacao)
}

type RequisicaoConcederRecompensa struct {
	IDExterno string `json:"id_externo"`
	IndicacaoID uint `json:"indicacao_id"`
	Motivo string `json:"motivo"` // "conta_aprovada" ou "acao_menor"
}

// ConcederRecompensaIndicacao concede crédito financeiro por indicação.
// Implementação inicial: aplica valores fixos (R$ 50 / R$ 10) e garante idempotência por IDExterno.
func ConcederRecompensaIndicacao(c *fiber.Ctx) error {
	var req RequisicaoConcederRecompensa
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "json inválido"})
	}

	req.IDExterno = strings.TrimSpace(req.IDExterno)
	req.Motivo = strings.TrimSpace(req.Motivo)
	if req.IDExterno == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id_externo é obrigatório"})
	}
	if req.IndicacaoID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "indicacao_id é obrigatório"})
	}
	if req.Motivo != "conta_aprovada" && req.Motivo != "acao_menor" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "motivo inválido"})
	}

	valor := recompensas.ValorIndicacaoAcaoMenorCentavos
	if req.Motivo == "conta_aprovada" {
		valor = recompensas.ValorIndicacaoContaAprovadaCentavos
	}

	agora := time.Now()
	r := recompensas.RecompensaIndicacao{
		IDExterno:     req.IDExterno,
		IndicacaoID:   req.IndicacaoID,
		ValorCentavos: int64(valor),
		Moeda:         recompensas.MoedaPadrao,
		Status:        recompensas.StatusRecompensaAprovada,
		Motivo:        req.Motivo,
		ConcedidaEm:   &agora,
	}

	if err := database.DB.Create(&r).Error; err != nil {
		var existente recompensas.RecompensaIndicacao
		if e2 := database.DB.Where("id_externo = ?", req.IDExterno).First(&existente).Error; e2 == nil {
			return c.Status(fiber.StatusOK).JSON(existente)
		}
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": "não foi possível conceder recompensa"})
	}

	_ = database.DB.Create(&security.AuditLog{
		Acao:        "recompensa_concedida",
		Recurso:     "recompensas",
		EnderecoIP:  c.IP(),
		Agente:      c.Get("User-Agent"),
		Sucesso:     true,
		Detalhes:    "recompensa concedida por indicação; motivo=" + req.Motivo,
		IDCorrelacao: strings.TrimSpace(c.Get("X-Correlacao-Id")),
	}).Error

	return c.Status(fiber.StatusCreated).JSON(r)
}

// ConsultarSaldoIndicador retorna o saldo total aprovado (em centavos) de um indicador.
// Implementação inicial: filtra por perfil e soma valores. No futuro, deve usar IndicadorID real.
func ConsultarSaldoIndicador(c *fiber.Ctx) error {
	perfil := strings.TrimSpace(c.Query("perfil"))
	if perfil == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "parâmetro 'perfil' é obrigatório"})
	}

	var total int64
	err := database.DB.
		Table("recompensa_indicacaos r").
		Select("COALESCE(SUM(r.valor_centavos), 0)").
		Joins("JOIN indicacaos i ON i.id = r.indicacao_id").
		Where("i.indicador_perfil = ? AND r.status = ?", perfil, recompensas.StatusRecompensaAprovada).
		Scan(&total).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "falha ao consultar saldo"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"perfil": perfil,
		"moeda": recompensas.MoedaPadrao,
		"saldo_centavos": total,
	})
}

