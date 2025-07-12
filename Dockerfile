# Build stage
FROM golang:1.24.3-alpine AS builder

# Instala apenas dependências essenciais para build
RUN apk add --no-cache git ca-certificates

# Define o diretório de trabalho
WORKDIR /app

# Copia apenas arquivos de dependências primeiro (para cache)
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

# Compila os binários com Go puro (CGO desabilitado)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o api-gateway ./cmd/api-gateway && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o payment-orchestrator ./cmd/payment-orchestrator && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o summary-service ./cmd/summary-service

# Runtime stage - imagem mínima
FROM alpine:latest AS runtime

# Instala apenas dependências essenciais para runtime
RUN apk add --no-cache ca-certificates libc6-compat && \
    addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Define o diretório de trabalho
WORKDIR /app

# Copia apenas os binários e arquivos necessários
COPY --from=builder /app/api-gateway .
COPY --from=builder /app/payment-orchestrator .
COPY --from=builder /app/summary-service .
COPY --from=builder /app/certs ./certs
COPY --from=builder /app/config ./config

# Muda para usuário não-root
# USER appuser

# Expõe as portas
EXPOSE 8443 8444 8445

# Comando padrão
CMD ["./api-gateway"] 