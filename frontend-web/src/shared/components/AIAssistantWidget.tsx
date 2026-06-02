import { useState } from 'react';
import { useAuthStore } from '../../core/auth/useAuthStore';

export default function AIAssistantWidget() {
  const [isOpen, setIsOpen] = useState(false);
  const { user } = useAuthStore();

  if (!isOpen) {
    return (
      <button 
        onClick={() => setIsOpen(true)}
        className="fixed bottom-6 right-6 w-14 h-14 bg-indigo-600 hover:bg-indigo-500 text-white rounded-full shadow-[0_0_15px_rgba(79,70,229,0.5)] flex items-center justify-center transition-all z-50 border-2 border-indigo-400"
      >
        <span className="text-2xl">✨</span>
      </button>
    );
  }

  return (
    <div className="fixed bottom-6 right-6 w-80 h-[26rem] bg-slate-800 border border-slate-600 rounded-2xl shadow-2xl flex flex-col overflow-hidden z-50">
      {/* Topo do Chat */}
      <div className="bg-slate-900 p-4 border-b border-slate-700 flex justify-between items-center">
        <div className="flex items-center gap-2">
          <span className="text-xl">⚡</span>
          <h3 className="font-semibold text-white">Assistente ZZEUS</h3>
        </div>
        <button onClick={() => setIsOpen(false)} className="text-slate-400 hover:text-white">
          ✖
        </button>
      </div>

      {/* Corpo do Chat */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4 bg-slate-800">
        <div className="bg-slate-700/50 p-3 rounded-2xl rounded-tl-sm border border-slate-600 text-slate-200 text-sm">
          Olá, <span className="font-bold text-indigo-300">{user?.name}</span>! Sou o assistente do seu sistema. Como posso ajudar na sua operação hoje?
        </div>
      </div>

      {/* Rodapé */}
      <div className="p-3 bg-slate-900 border-t border-slate-700">
        <input 
          type="text" 
          placeholder="Digite sua pergunta..." 
          className="w-full bg-slate-800 text-white border border-slate-600 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-indigo-500"
        />
      </div>
    </div>
  );
}
