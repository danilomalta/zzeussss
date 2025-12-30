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
