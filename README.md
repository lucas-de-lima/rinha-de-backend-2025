# Rinha de Backend 2025

Solução de processamento de pagamentos com arquitetura distribuída e otimizações de performance para o desafio Rinha de Backend 2025.

## Arquitetura

```
Load Balancer (9999) → API Gateway 1 → Payment Orchestrator → Payment Processors
                    → API Gateway 2 → Summary Service → BBolt Database
```

## Sobre a Solução

Sistema de microsserviços desenvolvido em Go com foco em performance e baixa latência. Utiliza gRPC para comunicação interna, BBolt para persistência local e implementa estratégias de fallback entre processadores de pagamento.

### Balanceamento de Carga

- **Load Balancer**: O balanceamento de carga é realizado por um proxy reverso customizado em Go

### Tecnologias

- **Go 1.24.3**: Linguagem principal
- **gRPC**: Comunicação entre microsserviços
- **BBolt**: Database embedded
- **Docker**: Containerização

### Otimizações de Performance

- Connection Pooling
- Cache em Memória
- Circuit Breaker
- Timeouts Ultra-Agressivos (300-500ms)
- Deduplicação
- Buffer Pools

## Execução

```bash
# 1. Iniciar payment-processor oficial
docker-compose up -d

# 2. Iniciar microsserviços
docker-compose up -d
```

---

*Projeto desenvolvido para Rinha de Backend 2025*