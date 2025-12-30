#!/bin/bash

echo "🚀 ATUALIZANDO TITAN PARA MODO HÍBRIDO (LOCAL/CLOUD/OFFLINE)..."

cd TitanSystem/titan-backend

# ==============================================================================
# 1. SISTEMA DE CONFIGURAÇÃO (A ESCOLHA DO MODO)
# ==============================================================================
mkdir -p configs
cat > configs/config.yaml <<EOF
system:
  # Modos disponíveis: "STANDALONE" (Local), "CLOUD_MASTER" (Nuvem), "EDGE_NODE" (Filial/Caixa)
  mode: "EDGE_NODE" 
  node_id: "loja_centro_01"

network:
  port: 8080
  master_url: "https://api.titansystem.com.br" # Se for local, deixa vazio
  sync_interval: 60 # Segundos

database:
  local_path: "storage/titan_local.db"
  cloud_dsn: "postgres://user:pass@cloud-aws/titan_db"
EOF

# ==============================================================================
# 2. MOTOR DE SINCRONIZAÇÃO (OFFLINE FIRST)
# ==============================================================================
# Esta é a pasta que faltava para gerenciar a "conversa" entre sistemas
mkdir -p pkg/sync_engine
mkdir -p pkg/sync_engine/queue

cat > pkg/sync_engine/manager.go <<EOF
package sync_engine

import (
	"log"
	"time"
)

// SyncManager decide se guarda no banco local ou manda para a nuvem
type SyncManager struct {
	Mode string
}

func (s *SyncManager) Start() {
	log.Println("🔄 Motor de Sincronização Iniciado...")
	
	go func() {
		for {
			if s.CheckInternet() {
				s.PushDataToMaster()
			} else {
				log.Println("⚠️ Sem conexão. Operando em Modo Offline (Cache Local).")
			}
			time.Sleep(30 * time.Second)
		}
	}()
}

func (s *SyncManager) CheckInternet() bool {
	// Aqui virá o ping para o Google ou Cloud
	return true 
}

func (s *SyncManager) PushDataToMaster() {
	// Pega dados da pasta 'queue' e envia para a API do Master
	log.Println("☁️ Enviando pacotes pendentes para o Servidor Principal...")
}
EOF

# ==============================================================================
# 3. COMUNICAÇÃO ENTRE SISTEMAS (CLUSTER)
# ==============================================================================
mkdir -p pkg/cluster
cat > pkg/cluster/discovery.go <<EOF
package cluster

import "fmt"

// Discovery permite que um caixa ache o outro na rede local
func FindNodes() {
	fmt.Println("📡 Procurando outros sistemas Titan na rede local...")
}
EOF

# ==============================================================================
# 4. EXECUTÁVEIS DIFERENTES (MASTER vs AGENT)
# ==============================================================================
# O sistema principal (Nuvem) roda um código, o caixa (Agente) roda outro
mkdir -p cmd/master_server
mkdir -p cmd/edge_agent

cat > cmd/master_server/main.go <<EOF
package main
import "fmt"
func main() { fmt.Println("👑 INICIANDO SERVIDOR MASTER (NUVEM/DEDICADO)") }
EOF

cat > cmd/edge_agent/main.go <<EOF
package main
import (
	"fmt"
	"github.com/titan/backend/pkg/sync_engine"
)
func main() { 
	fmt.Println("💻 INICIANDO AGENTE LOCAL (CAIXA/FILIAL)") 
	sync := sync_engine.SyncManager{Mode: "EDGE"}
	sync.Start()
	// Mantém rodando
	select {}
}
EOF

# ==============================================================================
# 5. AJUSTANDO A ESTRUTURA GERAL
# ==============================================================================
# Criando pastas utilitárias que você sentiu falta
mkdir -p pkg/utils
mkdir -p pkg/logger
mkdir -p internal/middleware

echo "✅ ESTRUTURA HÍBRIDA CRIADA!"
echo "------------------------------------------------"
echo "📂 Novas pastas:"
echo "   - configs/ (Onde o cliente escolhe se é Nuvem ou Local)"
echo "   - pkg/sync_engine/ (O Robô que sincroniza dados)"
echo "   - cmd/master_server/ (Executável para Nuvem)"
echo "   - cmd/edge_agent/ (Executável para PC Local)"
echo "------------------------------------------------"
