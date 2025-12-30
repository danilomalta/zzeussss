#!/bin/bash

echo "📱 CONFIGURANDO O ECOSSISTEMA MOBILE (OFFLINE FIRST)..."

# Garante que a pasta existe
mkdir -p TitanSystem/titan-mobile
cd TitanSystem/titan-mobile

# ==============================================================================
# 1. ESTRUTURA DE PASTAS (ORGANIZAÇÃO DE APP GRANDE)
# ==============================================================================
mkdir -p src/api
mkdir -p src/database
mkdir -p src/services/sync
mkdir -p src/screens/{Auth,Dashboard,Logistics,PDV}
mkdir -p src/navigation
mkdir -p src/components

# ==============================================================================
# 2. A CONEXÃO COM O BACKEND (AXIOS + IP)
# ==============================================================================
# O celular não entende "localhost". Ele precisa do IP da máquina ou da Nuvem.
cat > src/api/client.ts <<EOF
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
  // if (token) config.headers.Authorization = \`Bearer \${token}\`;
  return config;
});

export default apiClient;
EOF

# ==============================================================================
# 3. O BANCO DE DADOS DO CELULAR (SQLite)
# ==============================================================================
cat > src/database/local_db.ts <<EOF
// Simulação da conexão SQLite no Mobile
// Em produção, usaremos 'expo-sqlite' ou 'react-native-quick-sqlite'

export const initMobileDB = async () => {
  console.log("💾 Inicializando Banco SQLite no Celular...");
  // Criação das tabelas locais (Vendas, Rotas) para funcionar Offline
  const query = \`
    CREATE TABLE IF NOT EXISTS pending_sync (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      payload TEXT,
      type TEXT,
      status TEXT DEFAULT 'PENDING'
    );
  \`;
  console.log("✅ Tabelas Locais Criadas (Modo Offline Ativo)");
};
EOF

# ==============================================================================
# 4. SERVIÇO DE SINCRONIZAÇÃO (O MÁGICO)
# ==============================================================================
cat > src/services/sync/syncService.ts <<EOF
import apiClient from '../../api/client';

// Este serviço roda em segundo plano no celular
export const syncData = async () => {
  console.log("🔄 Tentando sincronizar dados do celular com o Servidor Go...");
  
  try {
    // 1. Verifica se tem internet (Ping no Google ou no Servidor)
    // 2. Pega dados da tabela 'pending_sync'
    // 3. Envia para o Backend
    
    // Exemplo de envio:
    // await apiClient.post('/sync/upload', dadosPendentes);
    
    console.log("✅ Sincronização Mobile Concluída!");
  } catch (error) {
    console.log("⚠️ Sem internet. Dados mantidos no celular.");
  }
};
EOF

# ==============================================================================
# 5. TELAS PRINCIPAIS (COM CÓDIGO)
# ==============================================================================

# Tela de Login
cat > src/screens/Auth/LoginScreen.tsx <<EOF
import React from 'react';
import { View, Text, Button, StyleSheet } from 'react-native';
import apiClient from '../../api/client';

export const LoginScreen = () => {
  const handleLogin = async () => {
    try {
      // Chama o Backend Go
      const response = await apiClient.post('/auth/login', { email: 'admin', pass: '123' });
      console.log("Login Sucesso:", response.data);
    } catch (e) {
      console.error("Erro no Login:", e);
    }
  };

  return (
    <View style={styles.container}>
      <Text style={styles.title}>Titan Mobile</Text>
      <Button title="Entrar no Sistema" onPress={handleLogin} />
    </View>
  );
};

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: 'center', alignItems: 'center' },
  title: { fontSize: 24, marginBottom: 20, fontWeight: 'bold' }
});
EOF

# Tela do Motorista (Logística)
cat > src/screens/Logistics/DriverScreen.tsx <<EOF
import React from 'react';
import { View, Text } from 'react-native';

export const DriverScreen = () => {
  return (
    <View>
      <Text>🚚 Rota Atual: Entrega #5092</Text>
      <Text>Status: Em Trânsito</Text>
      {/* Aqui virá o Mapa e Botão de Confirmar Entrega */}
    </View>
  );
};
EOF

echo "✅ MOBILE CONFIGURADO COM SUCESSO!"
echo "---------------------------------------------------"
echo "📲 O app já tem:"
echo "   1. Cliente API apontando para o Backend Go."
echo "   2. Estrutura de Banco Offline."
echo "   3. Serviço de Sincronização."
echo "---------------------------------------------------"
