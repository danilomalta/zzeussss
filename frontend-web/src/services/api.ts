import axios from 'axios';

/*
API CLIENT (AXIOS INTERCEPTORS)
===============================
Configuração mestre de saída de rede do React/Vite.

Regras de Segurança:
1. `withCredentials: true`: CRÍTICO. Instrui o navegador a enviar silenciosamente
   o Cookie HttpOnly gerado no login em toda requisição subsequente. Sem isso,
   o AuthGuard no Go vai recusar os acessos.
2. Interceptor de Resposta: Capta ataques expirados (401) ou tentativas de acesso 
   não autorizadas por Role (403) para ejetar o usuário instantaneamente para a tela de Login.
*/

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1',
  timeout: 10000,
  withCredentials: true, 
  headers: {
    'Content-Type': 'application/json',
    'X-Client-Platform': 'web'
  }
});

// Interceptor de Resposta
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // Token expirou ou cookie sumiu. Disparar Logout Global do Zustand.
      console.warn("Sessão Inválida - Acesso Negado");
    }
    if (error.response?.status === 403) {
      console.warn("Falta de Privilégios RBAC");
    }
    return Promise.reject(error);
  }
);
