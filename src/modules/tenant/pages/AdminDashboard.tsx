import React, { useState } from 'react';
import { useAuthStore } from '../../../core/auth/useAuthStore';

export default function AdminDashboard() {
  const { user } = useAuthStore();
  const [activeTab, setActiveTab] = useState('painel');

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-[#050505] text-gray-300 font-sans">
      
      {/* SIDEBAR - TITAN SYSTEM */}
      <aside className="w-64 flex-shrink-0 bg-[#0a0a0a] border-r border-white/10 flex flex-col z-20">
        <div className="h-16 flex items-center px-6 border-b border-white/10">
          <span className="text-xl font-bold tracking-widest text-white">TITAN <span className="text-blue-500 font-light">SYSTEM</span></span>
        </div>
        
        <nav className="flex-1 p-4 space-y-2">
          <div className="text-[10px] uppercase tracking-widest text-gray-500 font-bold mb-4 px-2 mt-2">Navegação Principal</div>
          
          {/* BOTÕES REAIS (SEM LINKS ROXOS) */}
          <button 
            onClick={() => setActiveTab('painel')} 
            className={`w-full text-left px-4 py-2.5 rounded-lg text-sm transition-all outline-none ${activeTab === 'painel' ? 'bg-blue-600/10 text-blue-400 border border-blue-500/20 shadow-sm' : 'text-gray-500 hover:text-gray-300 hover:bg-white/5 border border-transparent'}`}
          >
            Painel de Controle
          </button>
          
          <button 
            onClick={() => setActiveTab('pdv')} 
            className={`w-full text-left px-4 py-2.5 rounded-lg text-sm transition-all outline-none ${activeTab === 'pdv' ? 'bg-blue-600/10 text-blue-400 border border-blue-500/20 shadow-sm' : 'text-gray-500 hover:text-gray-300 hover:bg-white/5 border border-transparent'}`}
          >
            Frente de Caixa (PDV)
          </button>

          <button 
            onClick={() => setActiveTab('database')} 
            className={`w-full text-left px-4 py-2.5 rounded-lg text-sm transition-all outline-none ${activeTab === 'database' ? 'bg-blue-600/10 text-blue-400 border border-blue-500/20 shadow-sm' : 'text-gray-500 hover:text-gray-300 hover:bg-white/5 border border-transparent'}`}
          >
            Banco de Dados
          </button>

          <button 
            onClick={() => setActiveTab('clientes')} 
            className={`w-full text-left px-4 py-2.5 rounded-lg text-sm transition-all outline-none ${activeTab === 'clientes' ? 'bg-blue-600/10 text-blue-400 border border-blue-500/20 shadow-sm' : 'text-gray-500 hover:text-gray-300 hover:bg-white/5 border border-transparent'}`}
          >
            Gestão de Clientes
          </button>
        </nav>

        {/* PERFIL DO USUÁRIO */}
        <div className="p-4 border-t border-white/10 flex items-center gap-3 bg-[#080808]">
          <div className="w-8 h-8 rounded-full bg-blue-600 flex items-center justify-center text-white text-xs font-bold">
            {user?.name ? user.name.charAt(0).toUpperCase() : 'T'}
          </div>
          <div className="flex flex-col">
            <span className="text-xs text-white font-medium">{user?.name || 'Administrador Titan'}</span>
            <span className="text-[10px] text-gray-500">Acesso Master</span>
          </div>
        </div>
      </aside>

      {/* ÁREA PRINCIPAL (NÃO ROLA A TELA INTEIRA) */}
      <main className="flex-1 flex flex-col h-screen relative overflow-hidden bg-[#0A0A0A]">
        
        {/* HEADER TOP */}
        <header className="h-16 flex-shrink-0 border-b border-white/10 flex items-center px-8 justify-between bg-[#0a0a0a]">
          <div className="text-sm font-medium text-gray-400 uppercase tracking-wider">{activeTab}</div>
          <div className="flex items-center gap-3">
            <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse shadow-[0_0_8px_#10b981]"></span>
            <span className="text-xs text-emerald-500 font-mono tracking-widest uppercase">Titan Core Online</span>
          </div>
        </header>

        {/* MIOLO DINÂMICO (COM ROLAGEM APENAS INTERNA) */}
        <div className="flex-1 overflow-y-auto p-8">
          <div className="max-w-6xl mx-auto animate-in fade-in duration-300">
            
            {activeTab === 'painel' && (
              <div className="space-y-6">
                <h1 className="text-2xl font-semibold text-white tracking-tight">Status da Operação</h1>
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                  {/* Terminal Log */}
                  <div className="bg-[#050505] border border-white/10 rounded-xl overflow-hidden shadow-2xl">
                    <div className="bg-white/5 px-4 py-3 border-b border-white/10">
                      <span className="text-xs font-mono text-gray-400">Titan_Terminal.log</span>
                    </div>
                    <div className="p-5 font-mono text-xs text-gray-400 space-y-3 h-64 overflow-y-auto">
                      <p><span className="text-emerald-500">✓</span> Conexão com PostgreSQL estabelecida na porta 8080.</p>
                      <p><span className="text-emerald-500">✓</span> Autenticação de Operador validada.</p>
                      <p className="text-blue-400 pt-2">ℹ Aguardando instruções da interface cliente.</p>
                    </div>
                  </div>
                </div>
              </div>
            )}

            {activeTab !== 'painel' && (
              <div className="flex flex-col items-center justify-center h-80 border border-dashed border-white/10 rounded-xl bg-white/[0.01]">
                <div className="w-16 h-16 bg-white/5 rounded-full flex items-center justify-center mb-4">
                  <span className="text-blue-500 text-2xl">⚡</span>
                </div>
                <h3 className="text-white font-medium text-lg mb-1">Módulo {activeTab.toUpperCase()}</h3>
                <p className="text-gray-500 text-sm">O ambiente está limpo, minimalista e pronto para integração de dados.</p>
              </div>
            )}

          </div>
        </div>
      </main>
    </div>
  );
}
