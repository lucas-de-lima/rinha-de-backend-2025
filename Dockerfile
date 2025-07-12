# Build stage - OTIMIZADO PARA PERFORMANCE
FROM golang:1.24.3-alpine AS builder

# Instala apenas dependências essenciais para build
RUN apk add --no-cache git ca-certificates

# Define o diretório de trabalho
WORKDIR /app

# Copia apenas arquivos de dependências primeiro (para cache otimizado)
COPY go.mod go.sum ./
RUN go mod download

# Copia o código fonte
COPY . .

# Instala buf e plugins protobuf
RUN go install github.com/bufbuild/buf/cmd/buf@latest && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Gera os stubs do protobuf
RUN buf generate

# Compila os binários com flags OTIMIZADOS para performance
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -gcflags="-l=4" \
    -trimpath \
    -o api-gateway ./cmd/api-gateway && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -gcflags="-l=4" \
    -trimpath \
    -o payment-orchestrator ./cmd/payment-orchestrator && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -gcflags="-l=4" \
    -trimpath \
    -o summary-service ./cmd/summary-service && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -gcflags="-l=4" \
    -trimpath \
    -o load-balancer ./cmd/load-balancer

# Runtime stage - IMAGEM MÍNIMA COMPATÍVEL
FROM alpine:latest AS runtime

# Instala apenas dependências essenciais para runtime
RUN apk add --no-cache ca-certificates libc6-compat && \
    addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Define o diretório de trabalho
WORKDIR /app

# Copia apenas os binários (certificados e config serão montados como volumes)
COPY --from=builder /app/api-gateway .
COPY --from=builder /app/payment-orchestrator .
COPY --from=builder /app/summary-service .
COPY --from=builder /app/load-balancer .

# Cria diretórios para volumes
RUN mkdir -p /app/certs /app/config /app/data

# Muda para usuário não-root (segurança)
USER appuser

# Expõe as portas
EXPOSE 8443 8444 8445 9999

# Comando padrão
CMD ["./api-gateway"] 