import React from 'react';

/*
CATÁLOGO DE PRODUTOS / SUPPLY CHAIN (ESTADO VAZIO REAL)
=======================================================
Interface limpa livre de mocks, representando o estado inicial do catálogo corporativo.
*/

const SupplyChain: React.FC = () => {
  const handleCreateProduct = () => {
    alert('Função de cadastro de produto acionada.');
  };

  return (
    <div className="min-h-screen bg-slate-900 text-slate-100 font-sans p-6">
      <div className="max-w-7xl mx-auto space-y-6">
        
        {/* Cabeçalho da página com título e botão de ação */}
        <div className="flex items-center justify-between border-b border-slate-800 pb-5">
          <div>
            <h1 className="text-xl font-extrabold tracking-tight text-white">Catálogo de Produtos</h1>
            <p className="text-xs text-slate-400 mt-1">Gerencie seu inventário e ressuprimento B2B</p>
          </div>
          <button
            onClick={handleCreateProduct}
            className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 active:scale-[0.98] text-white text-xs font-bold rounded-lg shadow-md shadow-indigo-600/10 transition duration-200"
          >
            + Novo Produto
          </button>
        </div>

        {/* Bloco Central de Empty State Corporativo */}
        <div className="flex flex-col items-center justify-center min-h-[400px] border border-slate-800 bg-slate-800/40 rounded-2xl p-8 text-center shadow-sm">
          {/* Ícone de Caixa/Inventário Suave */}
          <div className="w-16 h-16 rounded-2xl bg-indigo-600/10 flex items-center justify-center text-indigo-400 mb-4 border border-indigo-500/20 shadow-inner">
            <svg className="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
            </svg>
          </div>
          
          <h2 className="text-lg font-bold text-slate-200">Seu catálogo está vazio</h2>
          <p className="text-xs text-slate-400 mt-1 max-w-sm">
            Cadastre seu primeiro produto para começar a vender
          </p>

          <button
            onClick={handleCreateProduct}
            className="mt-6 px-4 py-2 border border-slate-700 hover:bg-slate-800 text-slate-300 text-xs font-semibold rounded-lg transition"
          >
            Começar cadastro
          </button>
        </div>

      </div>
    </div>
  );
};

export default SupplyChain;
