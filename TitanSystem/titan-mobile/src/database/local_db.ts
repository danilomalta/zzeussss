// Simulação da conexão SQLite no Mobile
// Em produção, usaremos 'expo-sqlite' ou 'react-native-quick-sqlite'

export const initMobileDB = async () => {
  console.log("💾 Inicializando Banco SQLite no Celular...");
  // Criação das tabelas locais (Vendas, Rotas) para funcionar Offline
  const query = `
    CREATE TABLE IF NOT EXISTS pending_sync (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      payload TEXT,
      type TEXT,
      status TEXT DEFAULT 'PENDING'
    );
  `;
  console.log("✅ Tabelas Locais Criadas (Modo Offline Ativo)");
};
