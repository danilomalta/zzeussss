import { useState, useEffect } from 'react';

// ─── Mock Data ────────────────────────────────────────────────────────────────

interface Client {
  id: number;
  name: string;
  plan: string;
  mrr: number;
  status: string;
  lastPayment: string;
  nextPayment: string;
  daysOverdue: number;
  issue: string | null;
  city: string;
}

const MOCK_CLIENTS: Client[] = [
  { id: 1, name: "Restaurante Bella Vista", plan: "Pro", mrr: 299, status: "active", lastPayment: "2026-05-01", nextPayment: "2026-06-01", daysOverdue: 0, issue: null, city: "São Paulo" },
  { id: 2, name: "Padaria do Zé", plan: "Starter", mrr: 99, status: "overdue", lastPayment: "2026-03-15", nextPayment: "2026-04-15", daysOverdue: 35, issue: "Pagamento atrasado", city: "Campinas" },
  { id: 3, name: "Lanchonete Top", plan: "Starter", mrr: 99, status: "active", lastPayment: "2026-05-10", nextPayment: "2026-06-10", daysOverdue: 0, issue: null, city: "Curitiba" },
  { id: 4, name: "Churrascaria Gaúcha", plan: "Enterprise", mrr: 599, status: "active", lastPayment: "2026-05-05", nextPayment: "2026-06-05", daysOverdue: 0, issue: null, city: "Porto Alegre" },
  { id: 5, name: "Bar do Marcos", plan: "Starter", mrr: 99, status: "suspended", lastPayment: "2026-02-01", nextPayment: "2026-03-01", daysOverdue: 80, issue: "Conta suspensa — 3 pagamentos perdidos", city: "Rio de Janeiro" },
  { id: 6, name: "Sushi Kyoto", plan: "Pro", mrr: 299, status: "active", lastPayment: "2026-05-15", nextPayment: "2026-06-15", daysOverdue: 0, issue: null, city: "São Paulo" },
  { id: 7, name: "Pizzaria Roma", plan: "Pro", mrr: 299, status: "overdue", lastPayment: "2026-04-10", nextPayment: "2026-05-10", daysOverdue: 10, issue: "Cartão recusado", city: "Belo Horizonte" },
  { id: 8, name: "Café Central", plan: "Starter", mrr: 99, status: "active", lastPayment: "2026-05-18", nextPayment: "2026-06-18", daysOverdue: 0, issue: null, city: "Brasília" },
];

interface Service {
  name: string;
  status: string;
  latency: number;
  uptime: number;
  region: string;
}

const MOCK_SERVICES: Service[] = [
  { name: "API Backend (Go)", status: "online", latency: 42, uptime: 99.97, region: "sa-east-1" },
  { name: "Frontend Web (React)", status: "online", latency: 18, uptime: 99.99, region: "CDN" },
  { name: "PostgreSQL", status: "online", latency: 8, uptime: 99.95, region: "sa-east-1" },
  { name: "Redis Cache", status: "online", latency: 1, uptime: 100, region: "sa-east-1" },
  { name: "Mobile API (RN)", status: "degraded", latency: 340, uptime: 98.2, region: "sa-east-1" },
  { name: "Electron Desktop", status: "online", latency: 0, uptime: 99.9, region: "local" },
  { name: "Webhook / Events", status: "offline", latency: 0, uptime: 89.1, region: "sa-east-1" },
  { name: "Auth Service (JWT)", status: "online", latency: 12, uptime: 99.98, region: "sa-east-1" },
];

interface Issue {
  id: number;
  severity: string;
  title: string;
  area: string;
  detail: string;
  fix: string;
}

const MOCK_ISSUES: Issue[] = [
  { id: 1, severity: "critical", title: "⚠️ CRÍTICO: Token JWT expira em 1h e não há refresh automático", area: "Segurança", detail: "O backend não implementa rotação de refresh tokens. Se o JWT expirar com o usuário logado, ele não consegue renovar sem logout manual.", fix: "Implementar /auth/refresh com refresh token em httpOnly cookie." },
  { id: 2, severity: "critical", title: "🔥 Senha de DB hardcoded no docker-compose.yml", area: "Segurança", detail: "`DB_PASSWORD=titanpass` está em texto puro no repositório público.", fix: "Usar .env com docker secrets. Nunca comitar credenciais." },
  { id: 3, severity: "high", title: "SQLite → PostgreSQL: migração incompleta", area: "Banco de Dados", detail: "O README menciona SQLite, mas o docker-compose já usa PostgreSQL. Há desalinhamento.", fix: "Remover toda referência a SQLite. Padronizar em PostgreSQL com GORM migrations." },
  { id: 4, severity: "high", title: "Sem rate limiting nas rotas de auth", area: "Segurança", detail: "A rota /auth/login não tem limite de tentativas, exposta a brute force.", fix: "Adicionar middleware de rate limit (ex: golang.org/x/time/rate ou redis-based)." },
  { id: 5, severity: "high", title: "Frontend em JS puro, sem TypeScript", area: "Código", detail: "O IMPROVEMENTS.md menciona TypeScript como próximo passo, mas o código ainda usa .js.", fix: "Migrar para .tsx/.ts em todo o frontend-web e mobile." },
  { id: 6, severity: "medium", title: "Sem testes unitários ou de integração", area: "Qualidade", detail: "Zero cobertura de testes no backend Go e no frontend React.", fix: "Adicionar testes com testify (Go) e Vitest/RTL (React)." },
  { id: 7, severity: "medium", title: "Webhook / Events offline", area: "Infraestrutura", detail: "Serviço de eventos caído — notificações de pedidos não chegam.", fix: "Verificar configuração de serviço e implementar health check + restart automático." },
  { id: 8, severity: "medium", title: "Mobile API com latência alta (340ms)", area: "Performance", detail: "Endpoint mobile respondendo 8x mais lento que o backend web.", fix: "Verificar queries N+1 no GORM. Adicionar índices nas tabelas de pedidos." },
  { id: 9, severity: "low", title: "Sem CI/CD configurado", area: "DevOps", detail: "Pasta .github/workflows existe mas sem pipelines ativos.", fix: "Adicionar GitHub Actions para build, test e deploy automático." },
  { id: 10, severity: "low", title: "CORS permissivo (*) em produção", area: "Segurança", detail: "Se o Fiber estiver configurado com CORS *, qualquer origem pode chamar a API.", fix: "Restringir CORS para domínios específicos em produção." },
];

interface MonthlyRevenue {
  month: string;
  revenue: number;
  costs: number;
  clients: number;
}

const MONTHLY_REVENUE: MonthlyRevenue[] = [
  { month: "Dez", revenue: 2100, costs: 800, clients: 5 },
  { month: "Jan", revenue: 2500, costs: 850, clients: 6 },
  { month: "Fev", revenue: 2800, costs: 900, clients: 7 },
  { month: "Mar", revenue: 3100, costs: 920, clients: 7 },
  { month: "Abr", revenue: 3400, costs: 980, clients: 8 },
  { month: "Mai", revenue: 3494, costs: 1020, clients: 8 },
];

interface Cost {
  label: string;
  monthly: number;
}

const COSTS: Cost[] = [
  { label: "Servidor (VPS/Cloud)", monthly: 320 },
  { label: "PostgreSQL (RDS)", monthly: 180 },
  { label: "Redis Cloud", monthly: 60 },
  { label: "CDN / Storage", monthly: 90 },
  { label: "Serviços de Email", monthly: 40 },
  { label: "Monitoramento", monthly: 80 },
  { label: "Certificados SSL", monthly: 20 },
  { label: "Backup automático", monthly: 30 },
  { label: "Desenvolvimento (hora)", monthly: 200 },
];

// ─── Helpers ──────────────────────────────────────────────────────────────────

const statusColor = (s: string): string => 
  (({ active: "#00d4a1", overdue: "#f5a623", suspended: "#e74c3c", online: "#00d4a1", degraded: "#f5a623", offline: "#e74c3c" } as Record<string, string>)[s] || "#888");

const severityColor = (s: string): string => 
  (({ critical: "#e74c3c", high: "#f5a623", medium: "#3498db", low: "#95a5a6" } as Record<string, string>)[s] || "#888");

const severityBg = (s: string): string => 
  (({ critical: "rgba(231,76,60,0.12)", high: "rgba(245,166,35,0.12)", medium: "rgba(52,152,219,0.12)", low: "rgba(149,165,166,0.1)" } as Record<string, string>)[s] || "transparent");

const fmt = (n: number) => `R$ ${n.toLocaleString("pt-BR")}`;
const totalMRR = MOCK_CLIENTS.filter(c => c.status === "active").reduce((a, c) => a + c.mrr, 0);
const totalOverdue = MOCK_CLIENTS.filter(c => c.status !== "active").reduce((a, c) => a + c.mrr, 0);
const totalCosts = COSTS.reduce((a, c) => a + c.monthly, 0);
const netProfit = totalMRR - totalCosts;

// ─── Mini Bar Chart ───────────────────────────────────────────────────────────

interface BarChartProps {
  data: MonthlyRevenue[];
}

function BarChart({ data }: BarChartProps) {
  const maxRev = Math.max(...data.map(d => d.revenue));
  return (
    <div style={{ display: "flex", alignItems: "flex-end", gap: 8, height: 120, padding: "0 4px" }}>
      {data.map((d, i) => (
        <div key={i} style={{ flex: 1, display: "flex", flexDirection: "column", alignItems: "center", gap: 4 }}>
          <div style={{ display: "flex", gap: 2, alignItems: "flex-end", height: 90 }}>
            <div title={`Receita: ${fmt(d.revenue)}`} style={{ width: 14, height: `${(d.revenue / maxRev) * 90}px`, background: "linear-gradient(180deg,#00d4a1,#00967a)", borderRadius: "3px 3px 0 0", transition: "height .4s" }} />
            <div title={`Custos: ${fmt(d.costs)}`} style={{ width: 14, height: `${(d.costs / maxRev) * 90}px`, background: "rgba(231,76,60,0.7)", borderRadius: "3px 3px 0 0", transition: "height .4s" }} />
          </div>
          <span style={{ fontSize: 10, color: "#888", fontFamily: "monospace" }}>{d.month}</span>
        </div>
      ))}
      <div style={{ display: "flex", gap: 12, alignSelf: "flex-end", paddingLeft: 8 }}>
        <span style={{ fontSize: 10, color: "#00d4a1" }}>▮ Receita</span>
        <span style={{ fontSize: 10, color: "#e74c3c" }}>▮ Custos</span>
      </div>
    </div>
  );
}

// ─── Stat Card ────────────────────────────────────────────────────────────────

interface StatCardProps {
  label: string;
  value: string | number;
  sub?: string;
  color?: string;
  icon?: string;
}

function StatCard({ label, value, sub, color = "#00d4a1", icon }: StatCardProps) {
  return (
    <div style={{ background: "#161b22", border: "1px solid #21262d", borderRadius: 12, padding: "20px 24px", display: "flex", flexDirection: "column", gap: 6, position: "relative", overflow: "hidden" }}>
      <div style={{ position: "absolute", top: 12, right: 16, fontSize: 28, opacity: .15 }}>{icon}</div>
      <span style={{ fontSize: 11, color: "#8b949e", textTransform: "uppercase", letterSpacing: 1, fontFamily: "'DM Mono', monospace" }}>{label}</span>
      <span style={{ fontSize: 28, fontWeight: 700, color, fontFamily: "'Syne', sans-serif", lineHeight: 1 }}>{value}</span>
      {sub && <span style={{ fontSize: 12, color: "#8b949e" }}>{sub}</span>}
    </div>
  );
}

// ─── Main ─────────────────────────────────────────────────────────────────────

export default function TitanAdminDashboard() {
  const [tab, setTab] = useState("overview");
  const [clientFilter, setClientFilter] = useState("all");
  const [issueFilter, setIssueFilter] = useState("all");
  const [, setPulse] = useState(0);

  useEffect(() => {
    const t = setInterval(() => setPulse(p => p + 1), 3000);
    return () => clearInterval(t);
  }, []);

  const tabs = [
    { id: "overview", label: "📊 Visão Geral" },
    { id: "clients", label: "👥 Clientes" },
    { id: "finance", label: "💰 Financeiro" },
    { id: "health", label: "🖥️ Saúde do Sistema" },
    { id: "issues", label: `🔴 Problemas (${MOCK_ISSUES.length})` },
    { id: "architecture", label: "🏗️ Arquitetura" },
  ];

  const filteredClients = clientFilter === "all" ? MOCK_CLIENTS : MOCK_CLIENTS.filter(c => c.status === clientFilter);
  const filteredIssues = issueFilter === "all" ? MOCK_ISSUES : MOCK_ISSUES.filter(i => i.severity === issueFilter);

  return (
    <div style={{ minHeight: "100vh", background: "#0d1117", color: "#e6edf3", fontFamily: "'Inter', 'Segoe UI', sans-serif" }}>
      {/* Load fonts */}
      <style>{`
        @import url('https://fonts.googleapis.com/css2?family=Syne:wght@700;800&family=DM+Mono:wght@400;500&family=Inter:wght@400;500;600&display=swap');
        * { box-sizing: border-box; margin: 0; padding: 0; }
        ::-webkit-scrollbar { width: 4px; } ::-webkit-scrollbar-track { background: #161b22; } ::-webkit-scrollbar-thumb { background: #30363d; border-radius: 4px; }
        .tab-btn { cursor: pointer; padding: 8px 16px; border-radius: 8px; border: 1px solid transparent; background: transparent; color: #8b949e; font-size: 13px; font-family: inherit; transition: all .2s; white-space: nowrap; }
        .tab-btn:hover { background: #21262d; color: #e6edf3; }
        .tab-btn.active { background: #21262d; border-color: #30363d; color: #e6edf3; }
        .client-row:hover { background: #21262d !important; }
        .issue-card:hover { transform: translateY(-1px); box-shadow: 0 4px 20px rgba(0,0,0,.3); }
        @keyframes blink { 0%,100%{opacity:1} 50%{opacity:.3} }
        .pulse { animation: blink 2s infinite; }
      `}</style>

      {/* Header */}
      <div style={{ borderBottom: "1px solid #21262d", padding: "16px 32px", display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <div style={{ width: 36, height: 36, background: "linear-gradient(135deg,#00d4a1,#0096ff)", borderRadius: 10, display: "flex", alignItems: "center", justifyContent: "center", fontSize: 18 }}>⚡</div>
          <div>
            <div style={{ fontFamily: "'Syne', sans-serif", fontWeight: 800, fontSize: 18, letterSpacing: -.5 }}>TitanSystem</div>
            <div style={{ fontSize: 11, color: "#8b949e", fontFamily: "'DM Mono', monospace" }}>Admin Operations Panel</div>
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <div className="pulse" style={{ width: 8, height: 8, borderRadius: "50%", background: "#00d4a1" }} />
          <span style={{ fontSize: 12, color: "#8b949e", fontFamily: "'DM Mono', monospace" }}>LIVE • {new Date().toLocaleTimeString("pt-BR")}</span>
        </div>
      </div>

      {/* Tabs */}
      <div style={{ borderBottom: "1px solid #21262d", padding: "0 32px", display: "flex", gap: 4, overflowX: "auto" }}>
        {tabs.map(t => (
          <button key={t.id} className={`tab-btn ${tab === t.id ? "active" : ""}`} onClick={() => setTab(t.id)}>{t.label}</button>
        ))}
      </div>

      <div style={{ padding: "24px 32px", maxWidth: 1400, margin: "0 auto" }}>

        {/* ── OVERVIEW ── */}
        {tab === "overview" && (
          <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px,1fr))", gap: 16 }}>
              <StatCard label="MRR Ativo" value={fmt(totalMRR)} sub={`${MOCK_CLIENTS.filter(c=>c.status==="active").length} clientes`} icon="💚" color="#00d4a1" />
              <StatCard label="Inadimplente" value={fmt(totalOverdue)} sub={`${MOCK_CLIENTS.filter(c=>c.status!=="active").length} clientes`} icon="🔴" color="#e74c3c" />
              <StatCard label="Lucro Líquido" value={fmt(netProfit)} sub={`margem ${Math.round((netProfit/totalMRR)*100)}%`} icon="📈" color="#3498db" />
              <StatCard label="Custos/mês" value={fmt(totalCosts)} sub="infra + dev" icon="💸" color="#f5a623" />
              <StatCard label="Problemas Críticos" value={MOCK_ISSUES.filter(i=>i.severity==="critical").length} sub="requerem ação urgente" icon="⚠️" color="#e74c3c" />
              <StatCard label="Uptime Médio" value="97.8%" sub="últimos 30 dias" icon="🟢" color="#00d4a1" />
            </div>

            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
              <div style={{ background: "#161b22", border: "1px solid #21262d", borderRadius: 12, padding: 20 }}>
                <div style={{ fontWeight: 600, marginBottom: 16, fontSize: 14 }}>Receita vs Custos (6 meses)</div>
                <BarChart data={MONTHLY_REVENUE} />
              </div>
              <div style={{ background: "#161b22", border: "1px solid #21262d", borderRadius: 12, padding: 20 }}>
                <div style={{ fontWeight: 600, marginBottom: 16, fontSize: 14 }}>Alertas Urgentes</div>
                <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                  {MOCK_CLIENTS.filter(c => c.status !== "active").map(c => (
                    <div key={c.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "10px 12px", background: "rgba(231,76,60,0.08)", borderRadius: 8, border: "1px solid rgba(231,76,60,0.2)" }}>
                      <div>
                        <div style={{ fontSize: 13, fontWeight: 600 }}>{c.name}</div>
                        <div style={{ fontSize: 11, color: "#8b949e" }}>{c.issue}</div>
                      </div>
                      <div style={{ textAlign: "right" }}>
                        <div style={{ fontSize: 12, color: "#e74c3c", fontFamily: "'DM Mono',monospace" }}>{c.daysOverdue}d atraso</div>
                        <div style={{ fontSize: 11, color: "#8b949e" }}>{fmt(c.mrr)}/mês</div>
                      </div>
                    </div>
                  ))}
                  {MOCK_SERVICES.filter(s => s.status !== "online").map(s => (
                    <div key={s.name} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "10px 12px", background: `${s.status === "offline" ? "rgba(231,76,60,0.08)" : "rgba(245,166,35,0.08)"}`, borderRadius: 8, border: `1px solid ${s.status === "offline" ? "rgba(231,76,60,0.2)" : "rgba(245,166,35,0.2)"}` }}>
                      <div style={{ fontSize: 13, fontWeight: 600 }}>{s.name}</div>
                      <span style={{ fontSize: 11, color: statusColor(s.status), background: `${statusColor(s.status)}22`, padding: "2px 8px", borderRadius: 4, fontFamily: "'DM Mono',monospace" }}>{s.status.toUpperCase()}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>
        )}

        {/* ── CLIENTS ── */}
        {tab === "clients" && (
          <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
              {["all","active","overdue","suspended"].map(f => (
                <button key={f} className={`tab-btn ${clientFilter === f ? "active" : ""}`} onClick={() => setClientFilter(f)}>
                  {f === "all" ? "Todos" : f === "active" ? "✅ Ativos" : f === "overdue" ? "⚠️ Atrasados" : "🚫 Suspensos"}
                  {" "}({f === "all" ? MOCK_CLIENTS.length : MOCK_CLIENTS.filter(c=>c.status===f).length})
                </button>
              ))}
            </div>
            <div style={{ background: "#161b22", border: "1px solid #21262d", borderRadius: 12, overflow: "hidden" }}>
              <table style={{ width: "100%", borderCollapse: "collapse" }}>
                <thead>
                  <tr style={{ borderBottom: "1px solid #21262d" }}>
                    {["Cliente","Cidade","Plano","MRR","Status","Último Pag.","Próx. Pag.","Problema"].map(h => (
                      <th key={h} style={{ padding: "12px 16px", textAlign: "left", fontSize: 11, color: "#8b949e", fontWeight: 500, textTransform: "uppercase", letterSpacing: .8, fontFamily: "'DM Mono',monospace", whiteSpace: "nowrap" }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {filteredClients.map((c, i) => (
                    <tr key={c.id} className="client-row" style={{ borderBottom: i < filteredClients.length-1 ? "1px solid #21262d" : "none", transition: "background .15s" }}>
                      <td style={{ padding: "14px 16px", fontWeight: 600, fontSize: 13 }}>{c.name}</td>
                      <td style={{ padding: "14px 16px", fontSize: 12, color: "#8b949e" }}>{c.city}</td>
                      <td style={{ padding: "14px 16px" }}>
                        <span style={{ fontSize: 11, padding: "3px 8px", borderRadius: 4, background: c.plan === "Enterprise" ? "rgba(52,152,219,.2)" : c.plan === "Pro" ? "rgba(0,212,161,.15)" : "#21262d", color: c.plan === "Enterprise" ? "#3498db" : c.plan === "Pro" ? "#00d4a1" : "#8b949e", fontFamily: "'DM Mono',monospace" }}>{c.plan}</span>
                      </td>
                      <td style={{ padding: "14px 16px", fontFamily: "'DM Mono',monospace", fontSize: 13, color: "#e6edf3" }}>{fmt(c.mrr)}</td>
                      <td style={{ padding: "14px 16px" }}>
                        <span style={{ fontSize: 11, padding: "3px 10px", borderRadius: 20, background: `${statusColor(c.status)}22`, color: statusColor(c.status), fontFamily: "'DM Mono',monospace", fontWeight: 600 }}>
                          {c.status === "active" ? "ATIVO" : c.status === "overdue" ? "ATRASADO" : "SUSPENSO"}
                        </span>
                      </td>
                      <td style={{ padding: "14px 16px", fontSize: 12, color: "#8b949e", fontFamily: "'DM Mono',monospace" }}>{c.lastPayment}</td>
                      <td style={{ padding: "14px 16px", fontSize: 12, color: c.daysOverdue > 0 ? "#e74c3c" : "#8b949e", fontFamily: "'DM Mono',monospace" }}>{c.nextPayment}{c.daysOverdue > 0 && <span style={{ color: "#e74c3c" }}> (+{c.daysOverdue}d)</span>}</td>
                      <td style={{ padding: "14px 16px", fontSize: 12, color: c.issue ? "#e74c3c" : "#8b949e" }}>{c.issue || "—"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* ── FINANCE ── */}
        {tab === "finance" && (
          <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(180px,1fr))", gap: 16 }}>
              <StatCard label="MRR Total" value={fmt(totalMRR)} sub="clientes ativos" icon="💰" color="#00d4a1" />
              <StatCard label="Potencial Perdido" value={fmt(totalOverdue)} sub="inadimplência" icon="❌" color="#e74c3c" />
              <StatCard label="Custos Totais" value={fmt(totalCosts)} sub="mensal" icon="💸" color="#f5a623" />
              <StatCard label="Lucro Líquido" value={fmt(netProfit)} sub={`${Math.round((netProfit/totalMRR)*100)}% margem`} icon="📈" color="#3498db" />
              <StatCard label="ARR Estimado" value={fmt(totalMRR * 12)} sub="receita anual" icon="📅" color="#9b59b6" />
            </div>

            <div style={{ display: "grid", gridTemplateColumns: "1.4fr 1fr", gap: 16 }}>
              <div style={{ background: "#161b22", border: "1px solid #21262d", borderRadius: 12, padding: 20 }}>
                <div style={{ fontWeight: 600, marginBottom: 16, fontSize: 14 }}>Evolução 6 meses</div>
                <BarChart data={MONTHLY_REVENUE} />
                <div style={{ marginTop: 16, display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 12 }}>
                  {MONTHLY_REVENUE.slice(-3).map(m => (
                    <div key={m.month} style={{ background: "#0d1117", borderRadius: 8, padding: 12 }}>
                      <div style={{ fontSize: 11, color: "#8b949e", marginBottom: 4 }}>{m.month}</div>
                      <div style={{ fontSize: 14, color: "#00d4a1", fontFamily: "'DM Mono',monospace" }}>{fmt(m.revenue)}</div>
                      <div style={{ fontSize: 11, color: "#e74c3c", fontFamily: "'DM Mono',monospace" }}>-{fmt(m.costs)}</div>
                    </div>
                  ))}
                </div>
              </div>

              <div style={{ background: "#161b22", border: "1px solid #21262d", borderRadius: 12, padding: 20 }}>
                <div style={{ fontWeight: 600, marginBottom: 16, fontSize: 14 }}>Breakdown de Custos</div>
                <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                  {COSTS.map(c => {
                    const pct = Math.round((c.monthly / totalCosts) * 100);
                    return (
                      <div key={c.label}>
                        <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 3 }}>
                          <span style={{ fontSize: 12 }}>{c.label}</span>
                          <span style={{ fontSize: 12, fontFamily: "'DM Mono',monospace", color: "#f5a623" }}>{fmt(c.monthly)}</span>
                        </div>
                        <div style={{ height: 4, background: "#21262d", borderRadius: 2 }}>
                          <div style={{ width: `${pct}%`, height: "100%", background: "linear-gradient(90deg,#f5a623,#e74c3c)", borderRadius: 2 }} />
                        </div>
                      </div>
                    );
                  })}
                  <div style={{ borderTop: "1px solid #21262d", paddingTop: 10, display: "flex", justifyContent: "space-between", fontWeight: 600 }}>
                    <span>Total Mensal</span>
                    <span style={{ fontFamily: "'DM Mono',monospace", color: "#e74c3c" }}>{fmt(totalCosts)}</span>
                  </div>
                </div>
              </div>
            </div>

            <div style={{ background: "#161b22", border: "1px solid #21262d", borderRadius: 12, padding: 20 }}>
              <div style={{ fontWeight: 600, marginBottom: 12, fontSize: 14 }}>Receita por Plano</div>
              <div style={{ display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 16 }}>
                {["Starter","Pro","Enterprise"].map(plan => {
                  const clients = MOCK_CLIENTS.filter(c => c.plan === plan && c.status === "active");
                  const rev = clients.reduce((a,c) => a+c.mrr, 0);
                  return (
                    <div key={plan} style={{ background: "#0d1117", borderRadius: 10, padding: 16 }}>
                      <div style={{ fontSize: 12, color: "#8b949e", marginBottom: 4 }}>{plan}</div>
                      <div style={{ fontSize: 22, fontWeight: 700, fontFamily: "'Syne',sans-serif", color: plan === "Enterprise" ? "#3498db" : plan === "Pro" ? "#00d4a1" : "#e6edf3" }}>{fmt(rev)}</div>
                      <div style={{ fontSize: 11, color: "#8b949e", marginTop: 2 }}>{clients.length} clientes ativos · {fmt(clients[0]?.mrr || 0)}/cliente</div>
                    </div>
                  );
                })}
              </div>
            </div>
          </div>
        )}

        {/* ── HEALTH ── */}
        {tab === "health" && (
          <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(300px,1fr))", gap: 16 }}>
              {MOCK_SERVICES.map(s => (
                <div key={s.name} style={{ background: "#161b22", border: `1px solid ${s.status === "online" ? "#21262d" : s.status === "degraded" ? "rgba(245,166,35,.3)" : "rgba(231,76,60,.3)"}`, borderRadius: 12, padding: 20, position: "relative", overflow: "hidden" }}>
                  <div style={{ position: "absolute", top: 0, left: 0, right: 0, height: 3, background: s.status === "online" ? "#00d4a1" : s.status === "degraded" ? "#f5a623" : "#e74c3c" }} />
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
                    <div>
                      <div style={{ fontWeight: 600, fontSize: 14 }}>{s.name}</div>
                      <div style={{ fontSize: 11, color: "#8b949e", fontFamily: "'DM Mono',monospace", marginTop: 2 }}>{s.region}</div>
                    </div>
                    <span className={s.status !== "online" ? "pulse" : ""} style={{ fontSize: 11, padding: "3px 10px", borderRadius: 20, background: `${statusColor(s.status)}22`, color: statusColor(s.status), fontFamily: "'DM Mono',monospace", fontWeight: 600 }}>
                      {s.status.toUpperCase()}
                    </span>
                  </div>
                  <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12, marginTop: 16 }}>
                    <div style={{ background: "#0d1117", borderRadius: 8, padding: 12 }}>
                      <div style={{ fontSize: 10, color: "#8b949e", marginBottom: 2 }}>LATÊNCIA</div>
                      <div style={{ fontSize: 20, fontWeight: 700, fontFamily: "'DM Mono',monospace", color: s.latency > 200 ? "#e74c3c" : s.latency > 50 ? "#f5a623" : "#00d4a1" }}>{s.latency > 0 ? `${s.latency}ms` : "—"}</div>
                    </div>
                    <div style={{ background: "#0d1117", borderRadius: 8, padding: 12 }}>
                      <div style={{ fontSize: 10, color: "#8b949e", marginBottom: 2 }}>UPTIME</div>
                      <div style={{ fontSize: 20, fontWeight: 700, fontFamily: "'DM Mono',monospace", color: s.uptime > 99 ? "#00d4a1" : s.uptime > 95 ? "#f5a623" : "#e74c3c" }}>{s.uptime}%</div>
                    </div>
                  </div>
                  {s.status !== "online" && (
                    <div style={{ marginTop: 12, padding: "8px 12px", background: "rgba(231,76,60,.1)", borderRadius: 6, fontSize: 12, color: "#e74c3c" }}>
                      ⚠️ {s.status === "offline" ? "Serviço inacessível — verificar logs" : "Latência elevada — possível gargalo"}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* ── ISSUES ── */}
        {tab === "issues" && (
          <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
              <span style={{ fontSize: 13, color: "#8b949e", marginRight: 4 }}>Filtrar:</span>
              {["all","critical","high","medium","low"].map(f => (
                <button key={f} className={`tab-btn ${issueFilter === f ? "active" : ""}`} onClick={() => setIssueFilter(f)} style={{ borderColor: f !== "all" ? severityColor(f) : undefined }}>
                  {f === "all" ? "Todos" : f.charAt(0).toUpperCase()+f.slice(1)} ({f === "all" ? MOCK_ISSUES.length : MOCK_ISSUES.filter(i=>i.severity===f).length})
                </button>
              ))}
            </div>
            <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
              {filteredIssues.map(issue => (
                <div key={issue.id} className="issue-card" style={{ background: severityBg(issue.severity), border: `1px solid ${severityColor(issue.severity)}44`, borderLeft: `4px solid ${severityColor(issue.severity)}`, borderRadius: 10, padding: 18, transition: "all .2s" }}>
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 12 }}>
                    <div style={{ flex: 1 }}>
                      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 6 }}>
                        <span style={{ fontSize: 10, padding: "2px 8px", borderRadius: 4, background: `${severityColor(issue.severity)}33`, color: severityColor(issue.severity), fontFamily: "'DM Mono',monospace", fontWeight: 600, textTransform: "uppercase" }}>{issue.severity}</span>
                        <span style={{ fontSize: 11, color: "#8b949e" }}>{issue.area}</span>
                      </div>
                      <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 8 }}>{issue.title}</div>
                      <div style={{ fontSize: 12, color: "#8b949e", lineHeight: 1.6, marginBottom: 8 }}>{issue.detail}</div>
                      <div style={{ fontSize: 12, padding: "8px 12px", background: "rgba(0,212,161,.08)", border: "1px solid rgba(0,212,161,.2)", borderRadius: 6, color: "#00d4a1" }}>
                        <strong>✅ Fix:</strong> {issue.fix}
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* ── ARCHITECTURE ── */}
        {tab === "architecture" && (
          <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
            {/* Melhorias de Arquitetura */}
            {[
              {
                cat: "🗄️ Banco de Dados",
                color: "#3498db",
                items: [
                  { title: "PostgreSQL consolidado", desc: "Remover toda referência a SQLite. O docker-compose já usa PG15, mas o README ainda cita SQLite — inconsistência grave para novos devs." },
                  { title: "GORM Migrations versionadas", desc: "Usar golang-migrate para versionamento. Nunca deixar o AutoMigrate do GORM rodar em produção — risco de perda de dados." },
                  { title: "Índices nas tabelas críticas", desc: "Adicionar índices em orders.created_at, orders.client_id, payments.status para queries de relatório não travar." },
                  { title: "Connection pooling", desc: "Configurar pgxpool no backend Go com max_open_conns, max_idle_conns. Default do GORM é muito conservador para PDV em produção." },
                ]
              },
              {
                cat: "🔐 Segurança",
                color: "#e74c3c",
                items: [
                  { title: "CRÍTICO: Credenciais fora do repo", desc: "DB_PASSWORD, JWT_SECRET nunca em docker-compose.yml. Usar .env + Docker Secrets + GitHub Secrets. Já estão expostas no repositório público!" },
                  { title: "CRÍTICO: Refresh Token", desc: "Implementar rotação de JWT com refresh token em httpOnly cookie. O sistema atual não renova tokens — usuários perdem sessão abruptamente." },
                  { title: "Rate Limiting nas rotas de auth", desc: "Brute force em /login é trivial sem limitação. Usar middleware com Redis para limitar tentativas por IP (ex: 5/min)." },
                  { title: "CORS restrito por ambiente", desc: "Em produção, CORS deve aceitar apenas os domínios do app. Nunca usar * em produção." },
                  { title: "HTTPS obrigatório", desc: "Forçar TLS em produção. Adicionar HSTS header. Certificado via Let's Encrypt + Traefik ou Nginx." },
                  { title: "Sanitização de inputs", desc: "Validar e sanitizar todos os inputs no backend Go, não só no frontend. O frontend pode ser bypassado." },
                ]
              },
              {
                cat: "🏗️ Arquitetura Geral",
                color: "#9b59b6",
                items: [
                  { title: "Clean Architecture no Go", desc: "Separar em layers: handler → usecase → repository. Evitar business logic nos handlers do Fiber. Facilita testes." },
                  { title: "TypeScript em todo o frontend", desc: "Migrar frontend-web e mobile de .js para .ts/.tsx. O projeto já tem TypeScript no package mas não usa — risco de bugs silenciosos." },
                  { title: "Monorepo tools: Turborepo ou Nx", desc: "Com 4 sub-projetos (backend, web, desktop, mobile), adicionar Turborepo para builds incrementais e compartilhamento de tipos." },
                  { title: "Event-driven para pedidos", desc: "Usar Redis Streams (já tem Redis) para comunicação assíncrona: pedido criado → notifica cozinha → atualiza status → notifica garçom." },
                  { title: "Separar schema por tenant", desc: "Para clientes multi-tenant, considerar schema por cliente no PostgreSQL. Garante isolamento total de dados." },
                ]
              },
              {
                cat: "⚡ Performance",
                color: "#f5a623",
                items: [
                  { title: "Cache de cardápio com Redis", desc: "O cardápio raramente muda. Cachear com TTL de 5min no Redis — reduz 80% das queries mais frequentes do mobile." },
                  { title: "Paginação em todas as listagens", desc: "Rotas de GET /orders sem limite podem retornar milhares de registros e derrubar o app. Adicionar cursor-based pagination." },
                  { title: "Lazy loading no Electron", desc: "Electron carregando o frontend completo na inicialização pode ser lento. Usar lazy() + Suspense para rotas secundárias." },
                  { title: "Compressão gzip no Fiber", desc: "Adicionar middleware de compressão no Go — reduz payload ~70% e melhora latência do mobile." },
                ]
              },
              {
                cat: "🧪 Qualidade e DevOps",
                color: "#00d4a1",
                items: [
                  { title: "CI/CD no GitHub Actions", desc: "A pasta .github/workflows existe mas vazia. Adicionar: lint → test → build → deploy em cada PR." },
                  { title: "Testes: Go (testify) + React (Vitest)", desc: "Zero testes é risco crítico para um PDV. Priorizar testes de integração nas rotas de pagamento e pedidos." },
                  { title: "Healthcheck nos containers", desc: "Adicionar HEALTHCHECK no Dockerfile do backend e no docker-compose para o orchestrator saber quando reiniciar." },
                  { title: "Logging estruturado (zerolog/zap)", desc: "Substituir fmt.Println por logger estruturado em JSON. Facilita integração com Datadog, Grafana Loki, etc." },
                  { title: "Monitoramento com Prometheus + Grafana", desc: "Expor métricas /metrics no Fiber. Dashboards de latência, taxa de erro e uso de conexões DB." },
                ]
              },
              {
                cat: "📱 Mobile (React Native)",
                color: "#e67e22",
                items: [
                  { title: "Modo offline com SQLite local", desc: "PDV precisa funcionar sem internet. Usar WatermelonDB ou MMKV para persistência local + sync quando reconectar." },
                  { title: "Push Notifications (Expo + FCM)", desc: "Notificar garçom quando pedido está pronto. Expo Notifications + Firebase Cloud Messaging já suportados pelo Expo." },
                  { title: "Autenticação biométrica", desc: "Para troca rápida de turno, adicionar biometria (expo-local-authentication) em vez de digitar senha a cada login." },
                ]
              },
            ].map(section => (
              <div key={section.cat} style={{ background: "#161b22", border: "1px solid #21262d", borderRadius: 12, overflow: "hidden" }}>
                <div style={{ padding: "14px 20px", borderBottom: "1px solid #21262d", background: `${section.color}11`, display: "flex", alignItems: "center", gap: 8 }}>
                  <div style={{ width: 4, height: 20, background: section.color, borderRadius: 2 }} />
                  <span style={{ fontWeight: 700, fontSize: 14, color: section.color }}>{section.cat}</span>
                </div>
                <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(300px,1fr))", gap: 0 }}>
                  {section.items.map((item, i) => (
                    <div key={i} style={{ padding: "16px 20px", borderBottom: "1px solid #0d1117", borderRight: "1px solid #0d1117" }}>
                      <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 4, color: "#e6edf3" }}>{item.title}</div>
                      <div style={{ fontSize: 12, color: "#8b949e", lineHeight: 1.6 }}>{item.desc}</div>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
