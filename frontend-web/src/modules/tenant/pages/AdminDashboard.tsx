import React, { useState } from 'react';
import { useAuthStore } from '../../../core/auth/useAuthStore';

type Tab = 'painel' | 'pdv' | 'database' | 'clientes' | 'relatorios';

const NAV_ITEMS: { id: Tab; label: string }[] = [
  { id: 'painel',     label: 'Painel'        },
  { id: 'pdv',        label: 'PDV'           },
  { id: 'database',   label: 'Banco de Dados'},
  { id: 'clientes',   label: 'Clientes'      },
  { id: 'relatorios', label: 'Relatórios'    },
];

const METRICS = [
  { label: 'MÓDULOS',  value: '4',          sub: 'ativos'     },
  { label: 'DATABASE', value: 'PostgreSQL',  sub: 'conectado'  },
  { label: 'UPTIME',   value: '99.9%',       sub: 'últimas 24h'},
];

export default function AdminDashboard() {
  const { user } = useAuthStore();
  const [activeTab, setActiveTab] = useState<Tab>('painel');

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-[#060606] text-gray-300">

      {/* ── SIDEBAR ── */}
      <aside className="w-40 flex-shrink-0 flex flex-col border-r border-white/[0.06] bg-[#060606]">

        {/* Logo */}
        <div className="h-14 flex items-center px-6 border-b border-white/[0.06]">
          <span className="text-[11px] font-bold tracking-[0.3em] text-white">TITAN</span>
          <span className="text-[11px] font-light tracking-[0.3em] text-blue-500 ml-1">SYS</span>
        </div>

        {/* Nav */}
        <nav className="flex-1 py-6 px-3 space-y-0.5">
          <p className="text-[9px] tracking-[0.2em] font-medium text-white/20 px-3 mb-4 uppercase">Sistema</p>

          {NAV_ITEMS.map(({ id, label }) => {
            const active = activeTab === id;
            return (
              <button
                key={id}
                onClick={() => setActiveTab(id)}
                className={[
                  'w-full text-left px-3 py-2 rounded-md text-[11px] transition-colors outline-none relative',
                  active
                    ? 'text-blue-400 bg-blue-950/40'
                    : 'text-white/20 hover:text-white/50 hover:bg-white/[0.03]',
                ].join(' ')}
              >
                {active && (
                  <span className="absolute left-0 top-1.5 bottom-1.5 w-[2px] bg-blue-500 rounded-r-full" />
                )}
                {label}
              </button>
            );
          })}
        </nav>

        {/* User */}
        <div className="px-4 py-4 border-t border-white/[0.06] flex items-center gap-3">
          <div className="w-7 h-7 rounded-full bg-blue-950 border border-blue-900/50 flex items-center justify-center text-[10px] font-semibold text-blue-400">
            {user?.name?.charAt(0).toUpperCase() ?? 'T'}
          </div>
          <div className="min-w-0">
            <p className="text-[10px] text-white/70 truncate">{user?.name ?? 'Admin'}</p>
            <p className="text-[9px] text-white/20">Master</p>
          </div>
        </div>
      </aside>

      {/* ── MAIN ── */}
      <main className="flex-1 flex flex-col overflow-hidden bg-[#080808]">

        {/* Topbar */}
        <header className="h-14 flex-shrink-0 flex items-center justify-between px-8 border-b border-white/[0.06] bg-[#060606]">
          <span className="text-[10px] tracking-[0.2em] font-medium text-white/20 uppercase">
            {activeTab}
          </span>
          <div className="flex items-center gap-2">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-500" />
            <span className="text-[9px] tracking-[0.15em] text-emerald-600 font-mono uppercase">
              Online
            </span>
          </div>
        </header>

        {/* Content — scroll interno */}
        <div className="flex-1 overflow-y-auto p-8">
          <div className="max-w-5xl mx-auto space-y-8">

            {activeTab === 'painel' ? (
              <>
                {/* Heading */}
                <div>
                  <h1 className="text-lg font-medium text-white">Status da Operação</h1>
                  <p className="text-[11px] text-white/20 mt-1">Titan System · v1.0</p>
                </div>

                {/* Metrics */}
                <div className="grid grid-cols-3 gap-4">
                  {METRICS.map(({ label, value, sub }) => (
                    <div
                      key={label}
                      className="bg-[#0a0a0a] border border-white/[0.06] rounded-lg px-5 py-4"
                    >
                      <p className="text-[9px] tracking-[0.15em] text-white/20 mb-3">{label}</p>
                      <p className="text-2xl font-medium text-white leading-none">{value}</p>
                      <p className="text-[10px] text-white/20 mt-1">{sub}</p>
                    </div>
                  ))}
                </div>

                {/* Terminal */}
                <div className="bg-[#040404] border border-white/[0.06] rounded-xl overflow-hidden">
                  <div className="flex items-center gap-2 px-5 py-3 border-b border-white/[0.04]">
                    <span className="w-2.5 h-2.5 rounded-full bg-white/[0.06]" />
                    <span className="w-2.5 h-2.5 rounded-full bg-white/[0.06]" />
                    <span className="w-2.5 h-2.5 rounded-full bg-white/[0.06]" />
                    <span className="ml-3 text-[9px] font-mono text-white/20">titan_core.log</span>
                  </div>
                  <div className="p-5 font-mono text-[11px] space-y-2.5 h-52 overflow-y-auto">
                    <p><span className="text-emerald-600">✓</span><span className="text-white/20 ml-2">PostgreSQL :8080 — conectado</span></p>
                    <p><span className="text-emerald-600">✓</span><span className="text-white/20 ml-2">Operador autenticado com sucesso</span></p>
                    <p><span className="text-emerald-600">✓</span><span className="text-white/20 ml-2">Módulos carregados: 4/4</span></p>
                    <p className="text-blue-600">› Aguardando comandos<span className="inline-block w-1.5 h-3 bg-blue-600/60 ml-1 align-middle animate-pulse" /></p>
                  </div>
                </div>
              </>
            ) : (
              /* Placeholder para outros módulos */
              <div className="flex flex-col items-center justify-center h-64 border border-dashed border-white/[0.06] rounded-xl">
                <p className="text-[10px] tracking-[0.2em] text-white/20 uppercase mb-2">
                  {activeTab}
                </p>
                <p className="text-[11px] text-white/10">Módulo em desenvolvimento</p>
              </div>
            )}

          </div>
        </div>
      </main>
    </div>
  );
}
