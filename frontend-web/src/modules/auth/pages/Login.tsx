import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '../../../core/auth/useAuthStore';

/*
TELA DE LOGIN SAAS DE ALTO DESEMPENHO
=====================================
Interface corporativa de alto padrão visual otimizada para baixo consumo de memória.
- Fundo: Slate-900
- Card: Slate-800 com elevação elegante e bordas sutis com brilho discreto
- Inputs: Foco em Indigo-500
- Botão: Prevenção de duplo clique (disabled e label dinâmico)
- Tratamento de Erros: Mensagens claras exibidas acima do botão de submissão
*/

const Login: React.FC = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  const login = useAuthStore((state) => state.login);
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setIsLoading(true);

    try {
      // Chama o login global passando e-mail e senha diretamente
      await login(email, password);
      // Redireciona para a raiz após sucesso
      navigate('/');
    } catch (err: any) {
      console.error('Erro de autenticação:', err);

      // Tratamento de erros detalhado do backend (SecOps)
      if (err.response?.status === 429) {
        setError('Limite de tentativas excedido. Por favor, tente novamente em 1 minuto.');
      } else if (err.response?.status === 401) {
        setError('Email ou senha incorretos. Por favor, verifique suas credenciais.');
      } else if (err.response?.data?.error) {
        setError(err.response.data.error);
      } else {
        setError('Erro de conexão com o servidor de autenticação. Tente novamente mais tarde.');
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-slate-900 text-slate-100 flex flex-col justify-center items-center p-4 font-sans relative">
      {/* Brilho de fundo sutil de baixa GPU */}
      <div className="absolute top-10 right-10 w-80 h-80 bg-indigo-500/5 rounded-full blur-3xl pointer-events-none"></div>
      
      {/* Card Centralizado Slate-800 */}
      <div className="w-full max-w-md p-8 bg-slate-800 border border-slate-700/50 rounded-2xl shadow-xl relative">
        {/* Borda superior de brilho sutil */}
        <div className="absolute top-0 left-0 right-0 h-0.5 bg-gradient-to-r from-indigo-500/30 to-blue-500/30 rounded-t-2xl"></div>

        {/* Cabeçalho */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-11 h-11 rounded-lg bg-indigo-600 mb-3 shadow-md shadow-indigo-600/10">
            <span className="text-sm font-black text-white">TS</span>
          </div>
          <h1 className="text-xl font-bold tracking-tight text-white">
            Painel Administrativo
          </h1>
          <p className="text-xs text-slate-400 mt-1">
            Entre para acessar o caixa e controle geral
          </p>
        </div>

        {/* Formulário */}
        <form onSubmit={handleSubmit} className="space-y-5">
          {/* Email */}
          <div>
            <label htmlFor="email-field" className="block text-xs font-bold uppercase tracking-wider text-slate-400 mb-1.5">
              Email
            </label>
            <input
              id="email-field"
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="seuemail@empresa.com"
              className="w-full px-4 py-2.5 bg-slate-900 border border-slate-700/80 focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 rounded-xl outline-none transition text-sm text-slate-100 placeholder-slate-500"
            />
          </div>

          {/* Senha */}
          <div>
            <div className="flex justify-between items-center mb-1.5">
              <label htmlFor="password-field" className="block text-xs font-bold uppercase tracking-wider text-slate-400">
                Senha
              </label>
              <a href="#recuperar" className="text-[11px] font-semibold text-indigo-400 hover:text-indigo-300 transition">
                Esqueceu a senha?
              </a>
            </div>
            <input
              id="password-field"
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              className="w-full px-4 py-2.5 bg-slate-900 border border-slate-700/80 focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 rounded-xl outline-none transition text-sm text-slate-100 placeholder-slate-500"
            />
          </div>

          {/* Mensagem de Erro (Renderizada logo acima do botão) */}
          {error && (
            <div className="p-3 bg-red-950/30 border border-red-800/40 rounded-xl flex items-start space-x-2 text-red-300 text-xs animate-fade-in shadow-md">
              <svg className="w-4 h-4 flex-shrink-0 mt-0.5 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              <span>{error}</span>
            </div>
          )}

          {/* Botão de Envio */}
          <button
            type="submit"
            disabled={isLoading}
            className="w-full py-2.5 px-4 bg-indigo-600 hover:bg-indigo-500 disabled:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed text-white font-bold rounded-xl shadow-md hover:shadow-indigo-600/10 transition duration-200 flex items-center justify-center space-x-2 text-sm"
          >
            {isLoading ? (
              <>
                <svg className="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                </svg>
                <span>Entrando...</span>
              </>
            ) : (
              <span>Entrar</span>
            )}
          </button>
        </form>

        {/* Rodapé Seguro */}
        <div className="mt-6 pt-4 border-t border-slate-700/40 flex items-center justify-center space-x-1 text-[10px] text-slate-500">
          <svg className="w-3 h-3 text-indigo-500/70" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
          </svg>
          <span>Ambiente Autenticado via HTTPS</span>
        </div>
      </div>
    </div>
  );
};

export default Login;
