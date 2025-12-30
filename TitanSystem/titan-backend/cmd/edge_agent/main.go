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
