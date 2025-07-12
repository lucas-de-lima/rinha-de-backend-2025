#!/bin/bash

# Script de Build Otimizado para Performance BRUTO
# Rinha de Backend 2025

set -e

echo "🚀 Build Otimizado BRUTO iniciando..."

# Configurações de performance para o build
export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1
export DOCKER_CLI_EXPERIMENTAL=enabled

# Limpa containers e imagens antigas
echo "🧹 Limpando containers antigos..."
docker-compose down --remove-orphans
docker system prune -f

# Build com cache otimizado
echo "🔨 Build com cache otimizado..."
docker-compose build --parallel --no-cache

# Inicia os serviços
echo "🚀 Iniciando serviços otimizados..."
docker-compose up -d

# Aguarda os serviços estarem prontos
echo "⏳ Aguardando serviços..."
sleep 10

# Verifica se os serviços estão rodando
echo "🔍 Verificando saúde dos serviços..."
docker-compose ps

echo "✅ Build otimizado concluído!"
echo "📊 Para testar: k6 run rinha-de-backend-2025-main/rinha-test/rinha.js" 