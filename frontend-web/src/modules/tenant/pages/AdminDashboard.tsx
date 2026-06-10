import React, { useState } from 'react';
import { useAuthStore } from '../../../core/auth/useAuthStore';

export default function AdminDashboard() {
  const { user } = useAuthStore();
  const [activeTab, setActiveTab] = useState('pdv');

  return (
    /* ROOT: Travado em 100% da tela */
    <div className="flex h-screen w-screen overflow-hidden bg-[#050505] text-gray-300 font-sans">
      
      {/* SIDEBAR */}
      <aside className="w-20 md:w-64 flex-shrink-0 flex flex-col h-full bg-[#0a0a0a] border-r border-white/10 z-20">
        <div className="h-16 flex items-center justify-center md:justify-start md:px-6 border-b border-white/10 flex-shrink-0">
          <span className="hidden md:block text-xl font-bold tracking-widest text-white">Titan<span className="text-indigo-500">System</span></span>
          <span className="block md:hidden text-xl font-bold text-indigo-500">T</span>
        </div>
        
        <nav className="flex-1 p-3 md:p-4 space-y-2 flex flex-col min-h-0 overflow-y-auto">
          <div className="hidden md:block text-[10px] uppercase tracking-widest text-gray-500 font-bold mb-4 px-2 mt-2">Módulos do Sistema</div>
          
          <button onClick={() => setActiveTab('pdv')} className={`w-full flex items-center justify-center md:justify-start px-0 md:px-4 py-3 rounded-lg text-sm transition-all outline-none ${activeTab === 'pdv' ? 'bg-indigo-600/10 text-indigo-400 border border-indigo-500/20' : 'text-gray-500 hover:bg-white/5 border border-transparent'}`}>
            <span className="md:hidden text-lg">🛒</span>
            <span className="hidden md:block truncate">Frente de Caixa (PDV)</span>
          </button>
          
          <button onClick={() => setActiveTab('monitoramento')} className={`w-full flex items-center justify-center md:justify-start px-0 md:px-4 py-3 rounded-lg text-sm transition-all outline-none ${activeTab === 'monitoramento' ? 'bg-indigo-600/10 text-indigo-400 border border-indigo-500/20' : 'text-gray-500 hover:bg-white/5 border border-transparent'}`}>
            <span className="md:hidden text-lg">📊</span>
            <span className="hidden md:block truncate">Visão Geral & Logs</span>
          </button>

          <button onClick={() => setActiveTab('estoque')} className={`w-full flex items-center justify-center md:justify-start px-0 md:px-4 py-3 rounded-lg text-sm transition-all outline-none ${activeTab === 'estoque' ? 'bg-indigo-600/10 text-indigo-400 border border-indigo-500/20' : 'text-gray-500 hover:bg-white/5 border border-transparent'}`}>
             <span className="md:hidden text-lg">📦</span>
             <span className="hidden md:block truncate">Estoque & Financeiro</span>
          </button>
        </nav>

        {/* PERFIL (Rodapé Fixo) */}
        <div className="p-4 border-t border-white/10 flex items-center justify-center md:justify-start gap-3 bg-[#080808] flex-shrink-0">
          <div className="w-8 h-8 rounded-full bg-indigo-600 flex items-center justify-center text-white text-xs font-bold flex-shrink-0">
            {user?.name ? user.name.charAt(0).toUpperCase() : 'T'}
          </div>
          <div className="hidden md:flex flex-col min-w-0">
            <span className="text-xs text-white font-medium truncate">{user?.name || 'Administrador'}</span>
            <span className="text-[10px] text-gray-500 truncate">SaaS Operator</span>
          </div>
        </div>
      </aside>

      {/* ÁREA PRINCIPAL: min-w-0 forca os filhos a nao vazarem para os lados */}
      <main className="flex-1 flex flex-col h-full min-w-0 overflow-hidden bg-[#0A0A0A]">
        
        {/* HEADER */}
        <header className="h-16 flex-shrink-0 w-full border-b border-white/10 flex items-center px-4 md:px-8 justify-between bg-[#0a0a0a]">
          <div className="text-sm font-medium text-gray-400 uppercase tracking-wider truncate">{activeTab.replace('_', ' ')}</div>
          <div className="flex items-center gap-2 flex-shrink-0">
            <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse shadow-[0_0_8px_#10b981]"></span>
            <span className="text-[10px] text-emerald-500 font-mono tracking-widest uppercase">Go API Integrada</span>
          </div>
        </header>

        {/* WRAPPER DE CONTEÚDO (min-h-0 forca o conteudo a caber na altura restante) */}
        <div className="flex-1 p-4 md:p-8 flex flex-col min-h-0 w-full">
          
          <div className="flex-1 flex flex-col bg-[#050505] border border-white/10 rounded-xl min-h-0 w-full overflow-hidden animate-in fade-in duration-300">
             
             <div className="h-14 flex-shrink-0 border-b border-white/10 flex items-center px-6 bg-white/[0.02]">
                <h2 className="text-white font-medium text-sm truncate">
                  {activeTab === 'pdv' ? 'Operação de Caixa Nativa' : `Módulo: ${activeTab.toUpperCase()}`}
                </h2>
             </div>

             {/* MIOLO: Redimensiona proporcionalmente, sem barras de scroll globais */}
             <div className="flex-1 p-6 flex flex-col items-center justify-center min-h-0">
                {activeTab === 'pdv' ? (
                  <div className="text-center w-full max-w-2xl">
                    <div className="w-16 h-16 bg-white/5 rounded-full flex items-center justify-center mx-auto mb-4">
                      <span className="text-indigo-500 text-2xl">🛒</span>
                    </div>
                    <h3 className="text-lg font-medium text-white mb-2">Frente de Caixa Totalmente Integrada</h3>
                    <p className="text-gray-400 text-sm leading-relaxed mb-4">
                      As vendas registradas aqui não dependem de marketplaces ou APIs externas. Toda operação atualiza o estoque, caixa, financeiro e os relatórios do TitanSystem de forma nativa e imediata via API Go.
                    </p>
                    <div className="inline-flex items-center px-3 py-1 rounded-full border border-emerald-500/30 bg-emerald-500/10 text-emerald-400 text-xs font-mono">
                      Status: Pronto para Operação Nativa
                    </div>
                  </div>
                ) : (
                  <div className="text-center w-full max-w-md">
                    <div className="w-16 h-16 bg-white/5 rounded-full flex items-center justify-center mx-auto mb-4">
                      <span className="text-gray-500 text-2xl">⚙️</span>
                    </div>
                    <p className="text-gray-400 text-sm leading-relaxed">
                      Painel fluído e responsivo ativado. O CSS Flexbox agora utiliza as propriedades corretas para evitar overflows indesejados.
                    </p>
                  </div>
                )}
             </div>

          </div>
        </div>
      </main>
    </div>
  );
}