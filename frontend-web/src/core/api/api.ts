import axios, { AxiosResponse, AxiosError, InternalAxiosRequestConfig } from 'axios';
import { useAuthStore } from '../auth/useAuthStore';

/*
API CLIENT (AXIOS INTERCEPTORS)
===============================
Configuração da camada de rede utilizando Axios com suporte nativo a cookies seguros.

Segurança & Protocolo:
1. `withCredentials: true`: OBRIGATÓRIO para tráfego seguro de cookies HttpOnly (Refresh Token).
2. Interceptor de Requisição: Injeta dinamicamente o accessToken no cabeçalho Authorization.
3. Interceptor de Resposta: Trata a expiração da sessão (401) limpando o estado global.
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

// Interceptor de requisições para injetar o Access Token JWT
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

// Interceptor de respostas para captura global de falhas de autenticação
api.interceptors.response.use(
  (response: AxiosResponse) => response,
  (error: AxiosError) => {
    if (error.response?.status === 401) {
      console.warn('Sessão expirada ou inválida. Limpando credenciais locais...');
      useAuthStore.getState().logout();
    }
    return Promise.reject(error);
  }
);
