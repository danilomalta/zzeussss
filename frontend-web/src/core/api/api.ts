import axios, { AxiosResponse, AxiosError, InternalAxiosRequestConfig } from 'axios';
import { useAuthStore } from '../auth/useAuthStore';

/*
API CLIENT (AXIOS)
==================
Instância central de rede configurada para comunicação com o backend Go (TitanSystem).

Segurança & Desempenho:
1. `withCredentials: true`: Permite tráfego seguro de cookies HttpOnly.
2. Interceptor de Requisição: Injeta automaticamente o token de acesso (JWT) no cabeçalho Authorization.
3. Interceptor de Resposta: Em caso de erro 401 (Unauthorized), limpa a store do Zustand e realiza o logout.
*/

export const api = axios.create({
  baseURL: 'http://localhost:8080/api/v1',
  timeout: 15000,
  withCredentials: true,
  headers: {
    'Content-Type': 'application/json',
    'X-Client-Platform': 'web',
  },
});

// Interceptor de Requisição: Injeta o JWT no Header de Autorização
api.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = useAuthStore.getState().accessToken;
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error: AxiosError) => {
    return Promise.reject(error);
  }
);

// Interceptor de Resposta: Trata a invalidação de tokens e redirecionamento de segurança
api.interceptors.response.use(
  (response: AxiosResponse) => response,
  (error: AxiosError) => {
    if (error.response?.status === 401) {
      console.warn('Sessão expirada ou não autorizada. Limpando credenciais locais...');
      useAuthStore.getState().logout();
    }
    return Promise.reject(error);
  }
);
