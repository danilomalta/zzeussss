import axios from 'axios';

// MUDAR ISTO: Se estiver testando no emulador Android, use 10.0.2.2
// Se estiver no iPhone ou celular físico, use o IP do seu PC (ex: 192.168.1.x)
const API_URL = 'http://10.0.2.2:8080/api'; 

const apiClient = axios.create({
  baseURL: API_URL,
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Interceptor para injetar o Token de Segurança automaticamente
apiClient.interceptors.request.use(async (config) => {
  // const token = await getTokenFromStorage();
  // if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

export default apiClient;
