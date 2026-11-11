import type {AxiosRequestConfig} from 'axios';
export function onlineLogoutRequest(): AxiosRequestConfig;
export function interpretOnlineLogout(status: number): 'closed' | 'uncertain';
