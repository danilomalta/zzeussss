FROM golang:1.25-alpine AS builder
RUN apk add --no-cache ca-certificates git
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/cmd ./cmd
COPY backend/internal ./internal
COPY backend/pkg ./pkg
COPY backend/db ./db
RUN CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/api && CGO_ENABLED=0 go build -trimpath -o /out/titan-online ./cmd/titan-online

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/ /usr/local/bin/
COPY deploy/lab52/migrate.sh /usr/local/bin/titan-lab-migrate
RUN chmod 755 /usr/local/bin/titan-lab-migrate
USER 65532:65532
WORKDIR /tmp
CMD ["api"]
