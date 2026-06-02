import React, { useState } from 'react';
import { useAuthStore } from '../../../core/auth/useAuthStore';

export default function AdminDashboard() {
  const { user } = useAuthStore();
  const [activeTab, setActiveTab] = useState('monitoramento');

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-[#050505] text-gray-300 font-sans">
      
      {/* SIDEBAR - TITANSYSTEM */}
      <aside className="w-64 flex-shrink-0 bg-[#0a0a0a] border-r border-white/10 flex flex-col z-20">
        <div className="h-16 flex items-center px-6 border-b border-white/10">
          <span className="text-xl font-bold tracking-widest text-white">Titan<span className="text-blue-500">System</span></span>
        </div>
        
        <nav className="p-4 space-y-2">
          <div className="text-[10px] uppercase tracking-widest text-gray-500 font-bold mb-4 px-2 mt-2">Core Master Admin</div>
          
          <button onClick={() => setActiveTab('monitoramento')} className={`w-full text-left px-4 py-2.5 rounded-lg text-sm transition-all ${activeTab === 'monitoramento' ? 'bg-blue-600/10 text-blue-400 border border-blue-500/20' : 'text-gray-500 hover:bg-white/5'}`}>Monitoramento & Logs</button>
          <button onClick={() => setActiveTab('database')} className={`w-full text-left px-4 py-2.5 rounded-lg text-sm ${activeTab === 'database' ? 'bg-blue-600/10 text-blue-400 border border-blue-500/20' : 'text-gray-500 hover:bg-white/5'}`}>Banco de Dados</button>
          <button onClick={() => setActiveTab('clientes')} className={`w-full text-left px-4 py-2.5 rounded-lg text-sm ${activeTab === 'clientes' ? 'bg-blue-600/10 text-blue-400 border border-blue-500/20' : 'text-gray-500 hover:bg-white/5'}`}>Gestão de Clientes</button>
          <button onClick={() => setActiveTab('configuracoes')} className={`w-full text-left px-4 py-2.5 rounded-lg text-sm ${activeTab === 'configuracoes' ? 'bg-blue-600/10 text-blue-400 border border-blue-500/20' : 'text-gray-500 hover:bg-white/5'}`}>Configurações</button>
        </nav>
      </aside>

      {/* ÁREA PRINCIPAL FIXA */}
      <main className="flex-1 flex flex-col h-screen overflow-hidden bg-[#0A0A0A]">
        <header className="h-16 flex-shrink-0 border-b border-white/10 flex items-center px-8 justify-between">
          <span className="text-xs text-gray-400 uppercase tracking-widest">{activeTab.toUpperCase()}</span>
          <span className="text-xs text-emerald-500 font-mono">TITANSYSTEM ONLINE</span>
        </header>

        <div className="flex-1 p-8 overflow-hidden">
          <div className="h-full w-full rounded-xl border border-white/10 bg-white/[0.01] flex flex-col items-center justify-center">
             <h2 className="text-lg text-white font-medium">Painel {activeTab.toUpperCase()}</h2>
             <p className="text-gray-500 text-sm mt-2">Ambiente operacional restrito.</p>
          </div>
        </div>
      </main>
    </div>
  );
}