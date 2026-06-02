import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '../../../core/auth/useAuthStore';

/*
PÁGINA DE LOGIN - TITANSYSTEM (SaaS)
=====================================
Interface corporativa moderna de nível SaaS, com visual minimalista e de alta fidelidade.
- Fundo Escuro: slate-900
- Card Centralizado: slate-800 com bordas finas e efeito de elevação
- Inputs: Limpos com foco em azul/índigo
- Botão: Estado de loading (desativado) e feedback visual
- Tratamento de Erros: Erros específicos (401, 429) destacados acima do botão
*/

const Login: React.FC = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const login = useAuthStore((state) => state.login);
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      // Chama a ação de autenticação global no Zustand
      await login({ email, password });
      // Redireciona para a raiz do painel em caso de sucesso
      navigate('/');
    } catch (err: any) {
      console.error('Falha na autenticação:', err);

      // Tratamento estrito de erros do backend (SecOps)
      if (err.response?.status === 429) {
        setError('Limite de tentativas excedido. Por favor, aguarde 1 minuto antes de tentar novamente.');
      } else if (err.response?.status === 401) {
        setError('Email ou palavra-passe incorretos. Por favor, verifique suas credenciais.');
      } else if (err.response?.data?.error) {
        setError(err.response.data.error);
      } else {
        setError('Não foi possível conectar ao servidor. Tente novamente mais tarde.');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-slate-900 text-slate-100 flex flex-col justify-center items-center p-4 relative overflow-hidden font-sans">
      {/* Detalhes estéticos sutis no fundo */}
      <div className="absolute top-0 right-0 w-96 h-96 bg-indigo-600/5 rounded-full blur-3xl -z-10 pointer-events-none"></div>
      <div className="absolute bottom-0 left-0 w-96 h-96 bg-blue-600/5 rounded-full blur-3xl -z-10 pointer-events-none"></div>

      {/* Card Centralizado Elegante (slate-800) */}
      <div className="w-full max-w-md p-8 bg-slate-800 border border-slate-700/60 rounded-2xl shadow-2xl relative">
        
        {/* Logo / Header */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-12 h-12 rounded-xl bg-gradient-to-br from-indigo-500 to-blue-600 shadow-md shadow-indigo-500/20 mb-4">
            <span className="text-lg font-black text-white tracking-wider">TS</span>
          </div>
          <h1 className="text-2xl font-bold tracking-tight text-white">
            Acesso ao TitanSystem
          </h1>
          <p className="text-xs text-slate-400 mt-1.5 font-medium">
            Entre com suas credenciais corporativas
          </p>
        </div>

        {/* Formulário */}
        <form onSubmit={handleSubmit} className="space-y-5">
          {/* Campo Email */}
          <div>
            <label htmlFor="email-input" className="block text-xs font-bold uppercase tracking-wider text-slate-400 mb-1.5">
              Email
            </label>
            <div className="relative">
              <span className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-500">
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M16 12a4 4 0 10-8 0 4 4 0 008 0zm0 0v1.5a2.5 2.5 0 005 0V12a9 9 0 10-9 9m4.5-1.206a8.959 8.959 0 01-4.5 1.206" />
                </svg>
              </span>
              <input
                id="email-input"
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="exemplo@corporativo.com"
                className="w-full pl-10 pr-4 py-2.5 bg-slate-900 border border-slate-700/80 focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 rounded-xl outline-none transition text-sm text-slate-100 placeholder-slate-500"
              />
            </div>
          </div>

          {/* Campo Palavra-passe */}
          <div>
            <div className="flex justify-between items-center mb-1.5">
              <label htmlFor="password-input" className="block text-xs font-bold uppercase tracking-wider text-slate-400">
                Palavra-passe
              </label>
              <a href="#recuperar" className="text-[11px] font-semibold text-indigo-400 hover:text-indigo-300 transition">
                Esqueceu?
              </a>
            </div>
            <div className="relative">
              <span className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-500">
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                </svg>
              </span>
              <input
                id="password-input"
                type="password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••••••"
                className="w-full pl-10 pr-4 py-2.5 bg-slate-900 border border-slate-700/80 focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 rounded-xl outline-none transition text-sm text-slate-100 placeholder-slate-500"
              />
            </div>
          </div>

          {/* Mensagem de Erro SecOps (Exibida exatamente acima do botão) */}
          {error && (
            <div className="p-3.5 bg-red-950/30 border border-red-800/50 rounded-xl flex items-start space-x-2.5 text-red-300 text-xs animate-fade-in shadow-md">
              <svg className="w-4.5 h-4.5 flex-shrink-0 mt-0.5 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              <span>{error}</span>
            </div>
          )}

          {/* Botão de Submissão com Estado de Loading */}
          <button
            type="submit"
            disabled={loading}
            className="w-full py-3 px-4 bg-gradient-to-r from-indigo-600 to-blue-600 hover:from-indigo-500 hover:to-blue-500 disabled:from-slate-700 disabled:to-slate-700 disabled:opacity-50 disabled:cursor-not-allowed text-white font-bold rounded-xl shadow-lg shadow-indigo-600/10 hover:shadow-indigo-600/20 transition duration-300 transform active:scale-[0.99] flex items-center justify-center space-x-2 text-sm"
          >
            {loading ? (
              <>
                <svg className="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                </svg>
                <span>A autenticar...</span>
              </>
            ) : (
              <span>Entrar</span>
            )}
          </button>
        </form>

        {/* Footer Seguro */}
        <div className="mt-6 pt-5 border-t border-slate-700/50 flex items-center justify-center space-x-1.5 text-[11px] text-slate-500">
          <svg className="w-3.5 h-3.5 text-indigo-500/80" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
          </svg>
          <span>Conexão Criptografada SSL</span>
        </div>
      </div>
    </div>
  );
};

export default Login;
