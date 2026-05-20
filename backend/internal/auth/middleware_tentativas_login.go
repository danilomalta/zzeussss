package auth

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/database"
	"titansystem-backend/internal/models"
)

type ConfiguracaoTentativasLogin struct {
	CaminhoLogin              string
	MaxTentativas             int
	Janela                    time.Duration
	BloqueioInicial            time.Duration
	FatorCrescimentoBloqueio   float64
}

func ConfiguracaoTentativasLoginPadrao() ConfiguracaoTentativasLogin {
	return ConfiguracaoTentativasLogin{
		CaminhoLogin:            "/api/v1/autenticacao/login",
		MaxTentativas:           5,
		Janela:                  10 * time.Minute,
		BloqueioInicial:         30 * time.Second,
		FatorCrescimentoBloqueio: 2.0,
	}
}

type estadoIP struct {
	PrimeiraFalhaNaJanela time.Time
	Falhas                int
	BloqueadoAte          time.Time
	NivelBloqueio         int
}

type LimitadorTentativasLogin struct {
	cfg ConfiguracaoTentativasLogin
	mu  sync.Mutex
	porIP map[string]*estadoIP
}

func NovoLimitadorTentativasLogin(cfg ConfiguracaoTentativasLogin) *LimitadorTentativasLogin {
	if cfg.MaxTentativas <= 0 {
		cfg.MaxTentativas = 5
	}
	if cfg.Janela <= 0 {
		cfg.Janela = 10 * time.Minute
	}
	if cfg.BloqueioInicial <= 0 {
		cfg.BloqueioInicial = 30 * time.Second
	}
	if cfg.FatorCrescimentoBloqueio < 1 {
		cfg.FatorCrescimentoBloqueio = 2.0
	}
	if cfg.CaminhoLogin == "" {
		cfg.CaminhoLogin = "/api/v1/autenticacao/login"
	}
	return &LimitadorTentativasLogin{
		cfg:  cfg,
		porIP: make(map[string]*estadoIP),
	}
}

func (l *LimitadorTentativasLogin) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Path() != l.cfg.CaminhoLogin {
			return c.Next()
		}

		ip := c.IP()
		agora := time.Now()

		l.mu.Lock()
		estado := l.obterEstado(ip, agora)
		if agora.Before(estado.BloqueadoAte) {
			restante := int(time.Until(estado.BloqueadoAte).Seconds())
			l.mu.Unlock()
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"erro":                 "muitas tentativas de login; aguarde antes de tentar novamente",
				"segundos_para_tentar": restante,
			})
		}
		l.mu.Unlock()

		err := c.Next()

		// Se o handler de login retornar 401, tratamos como falha.
		if c.Response().StatusCode() == fiber.StatusUnauthorized {
			l.registrarFalha(ip, agora, c)
		}
		return err
	}
}

func (l *LimitadorTentativasLogin) obterEstado(ip string, agora time.Time) *estadoIP {
	estado, ok := l.porIP[ip]
	if !ok {
		estado = &estadoIP{}
		l.porIP[ip] = estado
	}
	if estado.PrimeiraFalhaNaJanela.IsZero() || agora.Sub(estado.PrimeiraFalhaNaJanela) > l.cfg.Janela {
		estado.PrimeiraFalhaNaJanela = agora
		estado.Falhas = 0
	}
	return estado
}

func (l *LimitadorTentativasLogin) registrarFalha(ip string, agora time.Time, c *fiber.Ctx) {
	l.mu.Lock()
	estado := l.obterEstado(ip, agora)
	estado.Falhas++

	bloqueouAgora := false
	ate := time.Time{}

	if estado.Falhas >= l.cfg.MaxTentativas {
		estado.NivelBloqueio++
		duracao := float64(l.cfg.BloqueioInicial) * pow(l.cfg.FatorCrescimentoBloqueio, float64(estado.NivelBloqueio-1))
		estado.BloqueadoAte = agora.Add(time.Duration(duracao))
		bloqueouAgora = true
		ate = estado.BloqueadoAte
	}
	l.mu.Unlock()

	// Auditoria: registrar a falha (e se houve bloqueio).
	detalhes := "login falho"
	if bloqueouAgora {
		detalhes = "login falho; bloqueio aplicado até " + ate.Format(time.RFC3339)
	}

	_ = database.DB.Create(&models.AuditLog{
		Perfil:      "desconhecido",
		Acao:        "login_falho",
		Recurso:     "autenticacao",
		EnderecoIP:  ip,
		Agente:      limitarAgente(c.Get("User-Agent"), 512),
		Sucesso:     false,
		Detalhes:    detalhes,
		IDCorrelacao: gerarIDCorrelacaoBasico(c),
	}).Error
}

func limitarAgente(agente string, max int) string {
	agente = strings.TrimSpace(agente)
	if max <= 0 || len(agente) <= max {
		return agente
	}
	return agente[:max]
}

func gerarIDCorrelacaoBasico(c *fiber.Ctx) string {
	// Estratégia simples: reaproveitar um cabeçalho se existir; caso contrário, criar um identificador pobre
	// usando timestamp+comprimento do caminho. Em produção, ideal é gerar um UUID e propagar.
	id := strings.TrimSpace(c.Get("X-Correlacao-Id"))
	if id != "" {
		return id
	}
	return strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + strconv.Itoa(len(c.Path()))
}

func pow(base float64, exp float64) float64 {
	if exp == 0 {
		return 1
	}
	resultado := 1.0
	for i := 0; i < int(exp); i++ {
		resultado *= base
	}
	return resultado
}

