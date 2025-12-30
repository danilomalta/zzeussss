#!/bin/bash

# 1. Cria a pasta do Backend (Se não existir)
mkdir -p TitanSystem/titan-backend
cd TitanSystem/titan-backend

# 2. Inicia o Go (Cria o arquivo go.mod)
if [ ! -f go.mod ]; then
    go mod init github.com/titan/backend
fi

# 3. Cria a estrutura de pastas dos 12 Módulos
mkdir -p cmd/api
mkdir -p pkg/database
# Cria as pastas internas dos módulos
for module in auth user financial fiscal stock sales_pdv logistics production legal office communication marketplace; do
    mkdir -p internal/$module/domain
    mkdir -p internal/$module/service
    mkdir -p internal/$module/repository
    mkdir -p internal/$module/handler
done

# 4. CRIA O ARQUIVO PRINCIPAL (main.go)
cat > cmd/api/main.go <<EOF
package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	fmt.Println("🚀 TITAN BACKEND: O Motor em Go está LIGADO!")
	fmt.Println("✅ Módulos carregados: Estoque, Fiscal, PDV, Financeiro...")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Sistema Titan Operacional"))
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
EOF

# 5. CRIA O CONECTOR DE BANCO DE DADOS (sqlite.go)
cat > pkg/database/sqlite.go <<EOF
package database

import "gorm.io/gorm"

func Connect() *gorm.DB {
    // Configuração do SQLite WAL virá aqui
    return nil
}
EOF

echo "---------------------------------------------------"
echo "✅ SUCESSO! A pasta 'titan-backend' foi criada."
echo "📄 Verifique se o arquivo 'go.mod' apareceu."
echo "---------------------------------------------------"
