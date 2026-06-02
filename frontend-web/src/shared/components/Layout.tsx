import React from 'react';
import { NavLink } from 'react-router-dom';
import { useAuthStore } from '../../core/auth/useAuthStore';
import AIAssistantWidget from './AIAssistantWidget';

export default function Layout({ children }: { children: React.ReactNode }) {
  const { user } = useAuthStore();

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-slate-900 text-slate-100 font-sans">
      {/* BARRA LATERAL FIXA À ESQUERDA */}
      <aside className="w-64 flex-shrink-0 bg-slate-950 border-r border-slate-800 flex flex-col z-20">
        <div className="h-16 flex items-center px-6 border-b border-slate-800">
          <span className="text-2xl font-black tracking-widest text-white">ZZEUS</span>
        </div>
        <nav className="flex-1 overflow-y-auto p-4 flex flex-col gap-2">
          <NavLink
            to="/"
            end
            className={({ isActive }) =>
              isActive
                ? 'text-left w-full px-4 py-3 rounded-lg bg-indigo-600 text-white font-medium shadow-md'
                : 'text-left w-full px-4 py-3 rounded-lg text-slate-400 hover:bg-slate-800 hover:text-white transition-colors'
            }
          >
            Visão Geral
          </NavLink>
          <NavLink
            to="/pos"
            className={({ isActive }) =>
              isActive
                ? 'text-left w-full px-4 py-3 rounded-lg bg-indigo-600 text-white font-medium shadow-md'
                : 'text-left w-full px-4 py-3 rounded-lg text-slate-400 hover:bg-slate-800 hover:text-white transition-colors'
            }
          >
            Frente de Caixa
          </NavLink>
          <NavLink
            to="/logistics"
            className={({ isActive }) =>
              isActive
                ? 'text-left w-full px-4 py-3 rounded-lg bg-indigo-600 text-white font-medium shadow-md'
                : 'text-left w-full px-4 py-3 rounded-lg text-slate-400 hover:bg-slate-800 hover:text-white transition-colors'
            }
          >
            Catálogo
          </NavLink>
          <NavLink
            to="/settings"
            className={({ isActive }) =>
              isActive
                ? 'text-left w-full px-4 py-3 rounded-lg bg-indigo-600 text-white font-medium shadow-md'
                : 'text-left w-full px-4 py-3 rounded-lg text-slate-400 hover:bg-slate-800 hover:text-white transition-colors'
            }
          >
            Configurações
          </NavLink>
        </nav>
      </aside>

      {/* ÁREA DIREITA (TOPO + MIOLO) */}
      <div className="flex-1 flex flex-col h-screen overflow-hidden bg-slate-900 relative">
        {/* TOPO FIXO */}
        <header className="h-16 flex-shrink-0 bg-slate-900/90 border-b border-slate-800 flex items-center justify-between px-8 z-10 backdrop-blur-sm">
          <div className="text-slate-400 text-sm font-medium">Operações ZZEUS</div>
          <div className="flex items-center gap-4">
            <div className="text-right">
              <p className="text-sm font-bold text-white">{user?.name || 'Administrador'}</p>
              <p className="text-xs text-indigo-400">Operador mestre</p>
            </div>
            <div className="w-10 h-10 bg-indigo-600 rounded-full flex items-center justify-center text-white font-bold shadow-lg">
              {user?.name ? user.name.charAt(0).toUpperCase() : 'Z'}
            </div>
          </div>
        </header>

        {/* MIOLO ROLÁVEL (SÓ ESTA PARTE ROLA) */}
        <main className="flex-1 overflow-y-auto p-8">
          <div className="max-w-6xl mx-auto">{children}</div>
        </main>

        {/* Assistente deve existir apenas uma vez no aplicativo */}
        <AIAssistantWidget />
      </div>
    </div>
  );
}
