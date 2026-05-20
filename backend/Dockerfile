# ==========================================
# STAGE 1: BUILDER
# ==========================================
FROM golang:1.22-alpine AS builder

# Instala certificados atualizados e dependências para build seguro
RUN apk add --no-cache ca-certificates git tzdata

WORKDIR /app

# Copia dependências isoladamente para cache
COPY go.mod go.sum ./
RUN go mod download

# Copia código fonte
COPY . .

# Compila de forma estática (segurança contra bibliotecas dinâmicas injetadas)
# Retira symbol tables e DWARF debugging info (-s -w) dificultando reverse engineering
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /go/bin/api cmd/api/main.go

# ==========================================
# STAGE 2: PRODUCTION RUNNER
# ==========================================
# Usar "scratch" é a prática mais segura, pois não há shell (sh/bash) 
# nem ferramentas instaladas para um invasor escalar privilégios.
FROM scratch

# Importa as chaves SSL/TLS do builder para HTTPS seguro
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Importa fusos horários globais
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Importa o binário protegido
COPY --from=builder /go/bin/api /api

# Roda a aplicação (Atenção: rodar como usuário não-root é ideal,
# adicione 'USER 1000' se for suportado na infra)
ENTRYPOINT ["/api"]
