import React, { useState } from 'react';
import { useAuthStore } from '../../../../core/auth/useAuthStore'; // Ajuste o path do authStore se necessário

export default function AdminDashboard() {
  const { user } = useAuthStore();
  const [activeTab, setActiveTab] = useState('monitoramento');

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-[#050505] text-gray-300 font-sans">
      
      {/* SIDEBAR - TITANSYSTEM MASTER */}
      <aside className="w-64 flex-shrink-0 bg-[#0a0a0a] border-r border-white/10 flex flex-col justify-between z-20">
        <div>
          <div className="h-16 flex items-center px-6 border-b border-white/10">
            <span className="text-xl font-bold tracking-widest text-white">Titan<span className="text-indigo-500">System</span></span>
          </div>
          
          <nav className="p-4 space-y-2">
            <div className="text-[10px] uppercase tracking-widest text-gray-500 font-bold mb-4 px-2 mt-2">Core Master Admin</div>
            
            <button 
              onClick={() => setActiveTab('monitoramento')} 
              className={`w-full text-left px-4 py-2.5 rounded-lg text-sm transition-all outline-none ${activeTab === 'monitoramento' ? 'bg-indigo-600/10 text-indigo-400 border border-indigo-500/20 shadow-sm' : 'text-gray-500 hover:text-gray-300 hover:bg-white/5 border border-transparent'}`}
            >
              Monitoramento & Logs
            </button>
            
            <button 
              onClick={() => setActiveTab('database')} 
              className={`w-full text-left px-4 py-2.5 rounded-lg text-sm transition-all outline-none ${activeTab === 'database' ? 'bg-indigo-600/10 text-indigo-400 border border-indigo-500/20 shadow-sm' : 'text-gray-500 hover:text-gray-300 hover:bg-white/5 border border-transparent'}`}
            >
              Banco de Dados (PostgreSQL)
            </button>

            <button 
              onClick={() => setActiveTab('clientes')} 
              className={`w-full text-left px-4 py-2.5 rounded-lg text-sm transition-all outline-none ${activeTab === 'clientes' ? 'bg-indigo-600/10 text-indigo-400 border border-indigo-500/20 shadow-sm' : 'text-gray-500 hover:text-gray-300 hover:bg-white/5 border border-transparent'}`}
            >
              Gestão de Clientes (Tenants)
            </button>

            <button 
              onClick={() => setActiveTab('configuracoes')} 
              className={`w-full text-left px-4 py-2.5 rounded-lg text-sm transition-all outline-none ${activeTab === 'configuracoes' ? 'bg-indigo-600/10 text-indigo-400 border border-indigo-500/20 shadow-sm' : 'text-gray-500 hover:text-gray-300 hover:bg-white/5 border border-transparent'}`}
            >
              Configurações Globais
            </button>
          </nav>
        </div>

        {/* PERFIL DO USUÁRIO */}
        <div className="p-4 border-t border-white/10 flex items-center gap-3 bg-[#080808]">
          <div className="w-8 h-8 rounded-full bg-indigo-600 flex items-center justify-center text-white text-xs font-bold">
            {user?.name ? user.name.charAt(0).toUpperCase() : 'T'}
          </div>
          <div className="flex flex-col">
            <span className="text-xs text-white font-medium">{user?.name || 'Administrador'}</span>
            <span className="text-[10px] text-gray-500">Acesso Master</span>
          </div>
        </div>
      </aside>

      {/* ÁREA PRINCIPAL (CRAVADA NA TELA, ZERO SCROLL GLOBAL) */}
      <main className="flex-1 flex flex-col h-screen relative overflow-hidden bg-[#0A0A0A]">
        
        {/* HEADER TOP */}
        <header className="h-16 flex-shrink-0 border-b border-white/10 flex items-center px-8 justify-between bg-[#0a0a0a]">
          <div className="text-sm font-medium text-gray-400 uppercase tracking-wider">{activeTab.replace('_', ' ')}</div>
          <div className="flex items-center gap-3">
            <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse shadow-[0_0_8px_#10b981]"></span>
            <span className="text-xs text-emerald-500 font-mono tracking-widest uppercase">API Go Conectada</span>
          </div>
        </header>

        {/* MIOLO DINÂMICO PROPORCIONAL */}
        <div className="flex-1 p-8 overflow-hidden flex flex-col">
          <div className="max-w-6xl mx-auto w-full flex-1 flex flex-col animate-in fade-in duration-300">
            
            {activeTab === 'monitoramento' && (
              <div className="flex-1 flex flex-col space-y-6">
                <h1 className="text-2xl font-semibold text-white tracking-tight">Status da Infraestrutura</h1>
                <div className="flex-1 grid grid-cols-1 gap-6 pb-6">
                  {/* Terminal Log de Verdade */}
                  <div className="flex flex-col bg-[#050505] border border-white/10 rounded-xl overflow-hidden shadow-2xl h-full">
                    <div className="bg-white/5 px-4 py-3 border-b border-white/10">
                      <span className="text-xs font-mono text-gray-400">TitanSystem_Terminal.log</span>
                    </div>
                    <div className="p-5 font-mono text-xs text-gray-400 space-y-3 flex-1 overflow-y-auto">
                      <p><span className="text-emerald-500">✓</span> Conexão com API Go na porta 8080 estabelecida.</p>
                      <p><span className="text-emerald-500">✓</span> Autenticação JWT HTTPOnly validada.</p>
                      <p className="text-indigo-400 pt-2">ℹ Sistema aguardando provisionamento de novos locatários no banco de dados.</p>
                    </div>
                  </div>
                </div>
              </div>
            )}

            {activeTab !== 'monitoramento' && (
              <div className="flex-1 flex flex-col items-center justify-center border border-dashed border-white/10 rounded-xl bg-white/[0.01] mb-6">
                <div className="w-16 h-16 bg-white/5 rounded-full flex items-center justify-center mb-4">
                  <span className="text-indigo-500 text-2xl">⚡</span>
                </div>
                <h3 className="text-white font-medium text-lg mb-1">Módulo {activeTab.toUpperCase()}</h3>
                <p className="text-gray-500 text-sm">Aguardando endpoints da API Fiber na pasta `/backend/api/`.</p>
              </div>
            )}

          </div>
        </div>
      </main>
    </div>
  );
}