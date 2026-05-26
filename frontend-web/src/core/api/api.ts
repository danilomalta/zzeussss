import axios, { AxiosResponse, AxiosError } from 'axios';

/*
API CLIENT (AXIOS INTERCEPTORS)
===============================
Configuração mestre de saída de rede do React/Vite.

Regras de Segurança:
1. `withCredentials: true`: [CRÍTICO & OBRIGATÓRIO]
   Esta diretiva é ABSOLUTAMENTE OBRIGATÓRIA para que o navegador aceite, armazene
   e envie silenciosamente o "Refresh Token" trafegado em cookie seguro com a flag HttpOnly.
   Sem esta configuração ativada no Axios, o navegador descartará o cookie de autenticação
   e as requisições protegidas subsequentes falharão no AuthGuard do backend.
2. Interceptor de Resposta: Capta ataques expirados (401) ou de privilégios RBAC (403)
   para redirecionar ou ejetar o usuário.
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
  (response: AxiosResponse) => response,
  (error: AxiosError) => {
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
